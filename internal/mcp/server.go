package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	mmcp "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/mlawd/m-cli/internal/config"
	"github.com/mlawd/m-cli/internal/harness"
	"github.com/mlawd/m-cli/internal/state"
)

func ServeStdio(ctx context.Context, in io.Reader, out io.Writer, version string) error {
	trimmedVersion := strings.TrimSpace(version)
	if trimmedVersion == "" {
		trimmedVersion = "dev"
	}

	srv := mcpserver.NewMCPServer(
		"m-cli-mcp",
		trimmedVersion,
		mcpserver.WithInstructions("Use resources for m workflow guidance and tools to inspect current stack/stage context."),
		mcpserver.WithResourceCapabilities(false, false),
		mcpserver.WithToolCapabilities(false),
		mcpserver.WithPromptCapabilities(false),
		mcpserver.WithRecovery(),
	)

	registerResources(srv)
	registerTools(srv)
	registerOrchestrationTools(srv)
	registerPrompts(srv)

	stdio := mcpserver.NewStdioServer(srv)
	return stdio.Listen(ctx, in, out)
}

func loadGlobalConfig() (*config.Config, error) {
	return config.Load()
}

func registerResources(srv *mcpserver.MCPServer) {
	srv.AddResource(
		mmcp.NewResource(
			"m://plan/format",
			"m Plan File Format",
			mmcp.WithResourceDescription("YAML schema and validation rules for `m stack new --plan-file`"),
			mmcp.WithMIMEType("text/markdown"),
		),
		func(ctx context.Context, request mmcp.ReadResourceRequest) ([]mmcp.ResourceContents, error) {
			return readResource(strings.TrimSpace(request.Params.URI))
		},
	)

	srv.AddResource(
		mmcp.NewResource(
			"m://guide/workflow",
			"m Workflow Guide",
			mmcp.WithResourceDescription("How to plan and execute work using m stack and stage commands"),
			mmcp.WithMIMEType("text/markdown"),
		),
		func(ctx context.Context, request mmcp.ReadResourceRequest) ([]mmcp.ResourceContents, error) {
			return readResource(strings.TrimSpace(request.Params.URI))
		},
	)

	srv.AddResource(
		mmcp.NewResource(
			"m://commands/reference",
			"m Command Reference",
			mmcp.WithResourceDescription("Quick command reference for m init/stack/stage workflows"),
			mmcp.WithMIMEType("text/markdown"),
		),
		func(ctx context.Context, request mmcp.ReadResourceRequest) ([]mmcp.ResourceContents, error) {
			return readResource(strings.TrimSpace(request.Params.URI))
		},
	)

	srv.AddResource(
		mmcp.NewResource(
			"m://state/context",
			"Current m Context",
			mmcp.WithResourceDescription("JSON snapshot of repo-local m state, including selected stack, stack type, and stage"),
			mmcp.WithMIMEType("application/json"),
		),
		func(ctx context.Context, request mmcp.ReadResourceRequest) ([]mmcp.ResourceContents, error) {
			return readResource(strings.TrimSpace(request.Params.URI))
		},
	)
}

func readResource(uri string) ([]mmcp.ResourceContents, error) {
	switch uri {
	case "m://plan/format":
		return []mmcp.ResourceContents{
			mmcp.TextResourceContents{URI: uri, MIMEType: "text/markdown", Text: planFormatGuide()},
		}, nil

	case "m://guide/workflow":
		return []mmcp.ResourceContents{
			mmcp.TextResourceContents{URI: uri, MIMEType: "text/markdown", Text: planningGuide()},
		}, nil

	case "m://commands/reference":
		return []mmcp.ResourceContents{
			mmcp.TextResourceContents{URI: uri, MIMEType: "text/markdown", Text: commandReference()},
		}, nil

	case "m://state/context":
		snapshot, err := BuildContextSnapshot(".", true)
		if err != nil {
			return nil, err
		}
		data, err := json.MarshalIndent(snapshot, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("failed to encode context: %w", err)
		}
		return []mmcp.ResourceContents{
			mmcp.TextResourceContents{URI: uri, MIMEType: "application/json", Text: string(data)},
		}, nil

	default:
		return nil, fmt.Errorf("unknown resource uri")
	}
}

func registerTools(srv *mcpserver.MCPServer) {
	srv.AddTool(
		mmcp.NewTool(
			"get_m_context",
			mmcp.WithDescription("Return current repository context for m, including selected stack, stack type, and stage"),
			mmcp.WithBoolean(
				"include_stacks",
				mmcp.Description("When true, include full stack/stage objects in response"),
			),
		),
		func(ctx context.Context, request mmcp.CallToolRequest) (*mmcp.CallToolResult, error) {
			includeStacks := request.GetBool("include_stacks", true)

			snapshot, err := BuildContextSnapshot(".", includeStacks)
			if err != nil {
				return nil, err
			}

			data, err := json.MarshalIndent(snapshot, "", "  ")
			if err != nil {
				return nil, fmt.Errorf("failed to encode context: %w", err)
			}

			return mmcp.NewToolResultStructured(snapshot, string(data)), nil
		},
	)

	srv.AddTool(
		mmcp.NewTool(
			"suggest_m_plan",
			mmcp.WithDescription("Generate an m-aware execution plan for an agent goal"),
			mmcp.WithString("goal", mmcp.Description("What the agent is trying to accomplish"), mmcp.Required()),
		),
		func(ctx context.Context, request mmcp.CallToolRequest) (*mmcp.CallToolResult, error) {
			goal, err := request.RequireString("goal")
			if err != nil {
				return nil, err
			}
			goal = strings.TrimSpace(goal)
			if goal == "" {
				return nil, fmt.Errorf("goal is required")
			}

			snapshot, err := BuildContextSnapshot(".", false)
			if err != nil {
				return nil, err
			}

			guidance := map[string][]string{
				"format": {
					"Prefer plan version 3 for new plans.",
					"Keep YAML frontmatter focused on stage identity and ordering.",
					"Store stage-specific prompt-like detail under markdown headings: ## Stage: <stage-id>.",
				},
				"stage_context": {
					"Describe defaults and assumptions that must remain true.",
					"Describe how this stage should interact with existing systems and adjacent stages.",
					"Call out invariants, compatibility constraints, and explicit non-goals.",
					"Include edge-case expectations and rollback/safety notes where relevant.",
				},
				"execution": {
					"For each stage, keep a crisp outcome plus concrete implementation and validation bullets.",
					"Make stage boundaries explicit so reviewers can reason about ordering and risk.",
				},
			}

			headline := fmt.Sprintf("Goal: %s", goal)
			if snapshot.CurrentStack != "" {
				headline += fmt.Sprintf(" | current stack: %s", snapshot.CurrentStack)
			}
			if snapshot.CurrentStage != "" {
				headline += fmt.Sprintf(" | current stage: %s", snapshot.CurrentStage)
			}
			if snapshot.CurrentStackType != "" {
				headline += fmt.Sprintf(" | stack type: %s", snapshot.CurrentStackType)
			}

			var textBuilder strings.Builder
			textBuilder.WriteString(headline)
			textBuilder.WriteString("\n\nSuggested planning guidance:\n")
			textBuilder.WriteString("\nFormat:\n")
			for _, item := range guidance["format"] {
				textBuilder.WriteString(fmt.Sprintf("- %s\n", item))
			}
			textBuilder.WriteString("\nStage context quality:\n")
			for _, item := range guidance["stage_context"] {
				textBuilder.WriteString(fmt.Sprintf("- %s\n", item))
			}
			textBuilder.WriteString("\nExecution quality:\n")
			for _, item := range guidance["execution"] {
				textBuilder.WriteString(fmt.Sprintf("- %s\n", item))
			}

			textBuilder.WriteString("\nVersion 3 plan skeleton:\n")
			textBuilder.WriteString("```markdown\n")
			textBuilder.WriteString("---\n")
			textBuilder.WriteString("version: 3\n")
			textBuilder.WriteString("title: <goal title>\n")
			textBuilder.WriteString("stages:\n")
			textBuilder.WriteString("  - id: stage-1\n")
			textBuilder.WriteString("    title: <stage title>\n")
			textBuilder.WriteString("  - id: stage-2\n")
			textBuilder.WriteString("    title: <stage title>\n")
			textBuilder.WriteString("---\n\n")
			textBuilder.WriteString("## Stage: stage-1\n")
			textBuilder.WriteString("<freeform stage context: defaults, interactions, assumptions, constraints>\n\n")
			textBuilder.WriteString("## Stage: stage-2\n")
			textBuilder.WriteString("<freeform stage context: defaults, interactions, assumptions, constraints>\n")
			textBuilder.WriteString("```\n")

			textBuilder.WriteString("\nContext checks:\n")
			textBuilder.WriteString("1. Read `m://state/context` or call `get_m_context`.\n")
			textBuilder.WriteString("2. If stack context is not inferred, run `m stack list` then `m stage open` for interactive selection.\n")
			textBuilder.WriteString("3. If no stage is selected, run `m stage list` then `m stage select <stage-id>`.\n")
			textBuilder.WriteString("4. Before handoff, run `m stage current` and summarize stage-scoped changes.\n")

			structured := map[string]interface{}{
				"goal":                   goal,
				"guidance":               guidance,
				"preferred_plan_version": 3,
				"plan_skeleton": map[string]interface{}{
					"frontmatter": map[string]interface{}{
						"version": 3,
						"title":   "<goal title>",
						"stages":  []map[string]string{{"id": "stage-1", "title": "<stage title>"}},
					},
					"markdown_sections": []string{"## Stage: stage-1", "<freeform stage context>"},
				},
				"context_checks": []string{
					"Read `m://state/context` or call `get_m_context`.",
					"If stack context is not inferred, run `m stack list` then `m stage open` for interactive selection.",
					"If no stage is selected, run `m stage list` then `m stage select <stage-id>`.",
					"Before handoff, run `m stage current` and summarize stage-scoped changes.",
				},
				"context": map[string]interface{}{
					"current_stack":      snapshot.CurrentStack,
					"current_stack_type": snapshot.CurrentStackType,
					"current_stage":      snapshot.CurrentStage,
				},
			}

			return mmcp.NewToolResultStructured(structured, strings.TrimSpace(textBuilder.String())), nil
		},
	)
}

func registerOrchestrationTools(srv *mcpserver.MCPServer) {
	srv.AddTool(
		mmcp.NewTool(
			"report_stage_done",
			mmcp.WithDescription("Called by agents when their phase is complete. Transitions the stage status and triggers the next pipeline step."),
			mmcp.WithString("stack_name", mmcp.Description("Name of the stack"), mmcp.Required()),
			mmcp.WithString("stage_id", mmcp.Description("ID of the stage"), mmcp.Required()),
			mmcp.WithString("phase", mmcp.Description("Phase that just completed: implementing or ai_review"), mmcp.Required()),
			mmcp.WithString("summary", mmcp.Description("Optional summary of what was done, stored for watch display")),
		),
		func(ctx context.Context, request mmcp.CallToolRequest) (*mmcp.CallToolResult, error) {
			stackName, err := request.RequireString("stack_name")
			if err != nil {
				return nil, err
			}
			stageID, err := request.RequireString("stage_id")
			if err != nil {
				return nil, err
			}
			phase, err := request.RequireString("phase")
			if err != nil {
				return nil, err
			}
			phase = strings.TrimSpace(phase)
			stackName = strings.TrimSpace(stackName)
			stageID = strings.TrimSpace(stageID)

			snapshot, err := BuildContextSnapshot(".", false)
			if err != nil {
				return nil, fmt.Errorf("build context: %w", err)
			}
			repoRoot := snapshot.RepoRoot
			if repoRoot == "" {
				return nil, fmt.Errorf("could not determine repo root")
			}

			stacks, err := state.LoadStacks(repoRoot)
			if err != nil {
				return nil, fmt.Errorf("load stacks: %w", err)
			}

			switch phase {
			case "implementing":
				// Transition: implementing → ai-review
				if err := state.TransitionStage(stacks, repoRoot, stackName, stageID, state.StageStatusAIReview); err != nil {
					return nil, err
				}

				// Reload stacks after save and spawn review agent.
				stacks, err = state.LoadStacks(repoRoot)
				if err != nil {
					return nil, fmt.Errorf("reload stacks: %w", err)
				}
				stack, _ := state.FindStack(stacks, stackName)
				stage, _ := state.FindStage(stack, stageID)

				cfg, err := loadGlobalConfig()
				if err != nil {
					return nil, err
				}
				h, err := harness.New(cfg.AgentHarness)
				if err != nil {
					return nil, err
				}
				opts := harness.AgentOpts{
					WorktreePath: strings.TrimSpace(stage.Worktree),
					StageContext: stage.Context,
					StackName:    stackName,
					StageID:      stageID,
					Phase:        "ai_review",
					DiffBase:     strings.TrimSpace(stage.Branch),
				}
				if opts.WorktreePath == "" {
					opts.WorktreePath = repoRoot
				}
				if err := h.SpawnReviewAgent(ctx, opts); err != nil {
					return nil, fmt.Errorf("spawn review agent: %w", err)
				}

				return mmcp.NewToolResultText(fmt.Sprintf("Stage %q transitioned to ai-review; review agent spawned.", stageID)), nil

			case "ai_review":
				// Transition: ai-review → human-review
				if err := state.TransitionStage(stacks, repoRoot, stackName, stageID, state.StageStatusHumanReview); err != nil {
					return nil, err
				}

				// Find the next pending stage in this stack and start it.
				stacks, err = state.LoadStacks(repoRoot)
				if err != nil {
					return nil, fmt.Errorf("reload stacks: %w", err)
				}
				stack, _ := state.FindStack(stacks, stackName)

				var nextStage *state.Stage
				for i := range stack.Stages {
					s := &stack.Stages[i]
					if state.EffectiveStatus(s) == state.StageStatusPending {
						nextStage = s
						break
					}
				}

				if nextStage == nil {
					// All stages done — mark stack complete via a summary note.
					return mmcp.NewToolResultText(fmt.Sprintf("Stage %q moved to human-review. All stages complete — stack %q is ready for final review.", stageID, stackName)), nil
				}

				// Transition next stage: pending → implementing
				if err := state.TransitionStage(stacks, repoRoot, stackName, nextStage.ID, state.StageStatusImplementing); err != nil {
					return nil, err
				}
				stacks, err = state.LoadStacks(repoRoot)
				if err != nil {
					return nil, fmt.Errorf("reload stacks: %w", err)
				}
				stack, _ = state.FindStack(stacks, stackName)
				nextStagePtr, _ := state.FindStage(stack, nextStage.ID)

				cfg, err := loadGlobalConfig()
				if err != nil {
					return nil, err
				}
				h, err := harness.New(cfg.AgentHarness)
				if err != nil {
					return nil, err
				}
				opts := harness.AgentOpts{
					WorktreePath: strings.TrimSpace(nextStagePtr.Worktree),
					StageContext: nextStagePtr.Context,
					StackName:    stackName,
					StageID:      nextStagePtr.ID,
					Phase:        "implementing",
				}
				if opts.WorktreePath == "" {
					opts.WorktreePath = repoRoot
				}
				if err := h.SpawnBuildAgent(ctx, opts); err != nil {
					return nil, fmt.Errorf("spawn build agent for stage %q: %w", nextStagePtr.ID, err)
				}

				return mmcp.NewToolResultText(fmt.Sprintf(
					"Stage %q moved to human-review. Next stage %q started (implementing).",
					stageID, nextStagePtr.ID,
				)), nil

			default:
				return nil, fmt.Errorf("unknown phase %q; must be implementing or ai_review", phase)
			}
		},
	)

	srv.AddTool(
		mmcp.NewTool(
			"get_stack_run_status",
			mmcp.WithDescription("Returns the current run status for a stack: per-stage status, active stage, and elapsed times."),
			mmcp.WithString("stack_name", mmcp.Description("Name of the stack"), mmcp.Required()),
		),
		func(ctx context.Context, request mmcp.CallToolRequest) (*mmcp.CallToolResult, error) {
			stackName, err := request.RequireString("stack_name")
			if err != nil {
				return nil, err
			}
			stackName = strings.TrimSpace(stackName)

			snapshot, err := BuildContextSnapshot(".", false)
			if err != nil {
				return nil, fmt.Errorf("build context: %w", err)
			}
			repoRoot := snapshot.RepoRoot
			if repoRoot == "" {
				return nil, fmt.Errorf("could not determine repo root")
			}

			stacks, err := state.LoadStacks(repoRoot)
			if err != nil {
				return nil, fmt.Errorf("load stacks: %w", err)
			}

			stack, _ := state.FindStack(stacks, stackName)
			if stack == nil {
				return nil, fmt.Errorf("stack %q not found", stackName)
			}

			now := time.Now().UTC()
			type stageStatus struct {
				ID          string `json:"id"`
				Title       string `json:"title"`
				Status      string `json:"status"`
				StartedAt   string `json:"started_at,omitempty"`
				ReviewedAt  string `json:"reviewed_at,omitempty"`
				ElapsedSecs int64  `json:"elapsed_secs,omitempty"`
			}

			activeStageID := ""
			stages := make([]stageStatus, 0, len(stack.Stages))
			for _, s := range stack.Stages {
				st := state.EffectiveStatus(&s)
				ss := stageStatus{
					ID:         s.ID,
					Title:      s.Title,
					Status:     st,
					StartedAt:  s.StartedAt,
					ReviewedAt: s.ReviewedAt,
				}
				if s.StartedAt != "" {
					if t, err := time.Parse(time.RFC3339, s.StartedAt); err == nil {
						ss.ElapsedSecs = int64(now.Sub(t).Seconds())
					}
				}
				if st == state.StageStatusImplementing || st == state.StageStatusAIReview {
					activeStageID = s.ID
				}
				stages = append(stages, ss)
			}

			result := map[string]interface{}{
				"stack_name":      stack.Name,
				"stack_type":      stack.Type,
				"active_stage_id": activeStageID,
				"stages":          stages,
				"total_stages":    len(stages),
			}

			data, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				return nil, fmt.Errorf("encode result: %w", err)
			}
			return mmcp.NewToolResultStructured(result, string(data)), nil
		},
	)
}

func registerPrompts(srv *mcpserver.MCPServer) {
	srv.AddPrompt(
		mmcp.NewPrompt(
			"plan_with_m",
			mmcp.WithPromptDescription("Prompt template for stage-aware planning with m"),
			mmcp.WithArgument("goal", mmcp.ArgumentDescription("Goal to plan for")),
		),
		func(ctx context.Context, request mmcp.GetPromptRequest) (*mmcp.GetPromptResult, error) {
			goal := strings.TrimSpace(request.Params.Arguments["goal"])
			if goal == "" {
				goal = "Complete the next stage in this repository"
			}

			promptText := fmt.Sprintf("Goal: %s\n\nUse the m context and workflow resources before planning. Propose a concrete stage-aligned implementation plan with outcome, implementation steps, validation, and risks/mitigations, then identify the exact m commands needed to confirm or update stack/stage context.", goal)

			return mmcp.NewGetPromptResult(
				"Stage-aware planning prompt for m",
				[]mmcp.PromptMessage{mmcp.NewPromptMessage(mmcp.RoleUser, mmcp.NewTextContent(promptText))},
			), nil
		},
	)
}
