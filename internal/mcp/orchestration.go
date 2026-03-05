package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mlawd/m-cli/internal/config"
	"github.com/mlawd/m-cli/internal/gitx"
	"github.com/mlawd/m-cli/internal/harness"
	"github.com/mlawd/m-cli/internal/state"

	mmcp "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

func registerOrchestrationTools(srv *mcpserver.MCPServer) {
	srv.AddTool(
		mmcp.NewTool(
			"report_stage_done",
			mmcp.WithDescription("Report that an agent phase (implementing or ai_review) is complete for a stage"),
			mmcp.WithString("stack_name", mmcp.Description("Name of the stack"), mmcp.Required()),
			mmcp.WithString("stage_id", mmcp.Description("ID of the stage"), mmcp.Required()),
			mmcp.WithString("phase", mmcp.Description("Phase that completed: implementing or ai_review"), mmcp.Required()),
			mmcp.WithString("summary", mmcp.Description("Optional summary of work done")),
		),
		handleReportStageDone,
	)

	srv.AddTool(
		mmcp.NewTool(
			"get_stack_run_status",
			mmcp.WithDescription("Get the current run status of a stack including per-stage status"),
			mmcp.WithString("stack_name", mmcp.Description("Name of the stack"), mmcp.Required()),
		),
		handleGetStackRunStatus,
	)
}

func handleReportStageDone(ctx context.Context, request mmcp.CallToolRequest) (*mmcp.CallToolResult, error) {
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

	stackName = strings.TrimSpace(stackName)
	stageID = strings.TrimSpace(stageID)
	phase = strings.TrimSpace(phase)

	if phase != "implementing" && phase != "ai_review" {
		return nil, fmt.Errorf("phase must be \"implementing\" or \"ai_review\", got %q", phase)
	}

	repo, err := gitx.DiscoverRepo(".")
	if err != nil {
		return nil, fmt.Errorf("discover repo: %w", err)
	}
	repoRoot := gitx.SharedRoot(repo.TopLevel, repo.CommonDir)

	stacks, err := state.LoadStacks(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("load stacks: %w", err)
	}

	switch phase {
	case "implementing":
		if err := state.TransitionStage(stacks, stackName, stageID, state.StatusAIReview); err != nil {
			return nil, err
		}

		if err := state.SaveStacks(repoRoot, stacks); err != nil {
			return nil, fmt.Errorf("save stacks: %w", err)
		}

		// Spawn review agent
		if err := spawnReviewAgent(ctx, repoRoot, stacks, stackName, stageID); err != nil {
			return mmcp.NewToolResultText(fmt.Sprintf("Stage transitioned to ai-review but failed to spawn review agent: %v", err)), nil
		}

		return mmcp.NewToolResultText(fmt.Sprintf("Stage %q transitioned to ai-review. Review agent spawned.", stageID)), nil

	case "ai_review":
		if err := state.TransitionStage(stacks, stackName, stageID, state.StatusHumanReview); err != nil {
			return nil, err
		}

		if err := state.SaveStacks(repoRoot, stacks); err != nil {
			return nil, fmt.Errorf("save stacks: %w", err)
		}

		stack, _ := state.FindStack(stacks, stackName)
		if stack == nil {
			return mmcp.NewToolResultText(fmt.Sprintf("Stage %q transitioned to human-review.", stageID)), nil
		}

		// Find next pending stage and start it
		next := state.NextPendingStage(stack)
		if next == nil {
			if state.AllStagesComplete(stack) {
				return mmcp.NewToolResultText(fmt.Sprintf("Stage %q transitioned to human-review. All stages complete.", stageID)), nil
			}
			return mmcp.NewToolResultText(fmt.Sprintf("Stage %q transitioned to human-review. No more pending stages.", stageID)), nil
		}

		if err := state.TransitionStage(stacks, stackName, next.ID, state.StatusImplementing); err != nil {
			return mmcp.NewToolResultText(fmt.Sprintf("Stage %q transitioned to human-review but failed to start next stage: %v", stageID, err)), nil
		}

		if err := state.SaveStacks(repoRoot, stacks); err != nil {
			return nil, fmt.Errorf("save stacks: %w", err)
		}

		if err := spawnBuildAgent(ctx, repoRoot, stacks, stackName, next.ID); err != nil {
			return mmcp.NewToolResultText(fmt.Sprintf("Stage %q -> human-review. Next stage %q -> implementing but failed to spawn build agent: %v", stageID, next.ID, err)), nil
		}

		return mmcp.NewToolResultText(fmt.Sprintf("Stage %q -> human-review. Next stage %q -> implementing. Build agent spawned.", stageID, next.ID)), nil
	}

	return nil, fmt.Errorf("unexpected phase: %s", phase)
}

func handleGetStackRunStatus(ctx context.Context, request mmcp.CallToolRequest) (*mmcp.CallToolResult, error) {
	stackName, err := request.RequireString("stack_name")
	if err != nil {
		return nil, err
	}
	stackName = strings.TrimSpace(stackName)

	repo, err := gitx.DiscoverRepo(".")
	if err != nil {
		return nil, fmt.Errorf("discover repo: %w", err)
	}
	repoRoot := gitx.SharedRoot(repo.TopLevel, repo.CommonDir)

	stacks, err := state.LoadStacks(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("load stacks: %w", err)
	}

	stack, _ := state.FindStack(stacks, stackName)
	if stack == nil {
		return nil, fmt.Errorf("stack %q not found", stackName)
	}

	type stageStatus struct {
		ID     string `json:"id"`
		Title  string `json:"title"`
		Status string `json:"status"`
		Elapsed string `json:"elapsed,omitempty"`
	}

	stages := make([]stageStatus, 0, len(stack.Stages))
	activeStageID := ""
	allDone := true

	for i := range stack.Stages {
		s := &stack.Stages[i]
		status := state.EffectiveStatus(s)
		elapsed := ""

		if status == state.StatusImplementing || status == state.StatusAIReview {
			activeStageID = s.ID
			if s.StartedAt != "" {
				if t, err := time.Parse(time.RFC3339, s.StartedAt); err == nil {
					elapsed = time.Since(t).Truncate(time.Second).String()
				}
			}
		}

		if status != state.StatusHumanReview && status != state.StatusDone {
			allDone = false
		}

		stages = append(stages, stageStatus{
			ID:      s.ID,
			Title:   s.Title,
			Status:  status,
			Elapsed: elapsed,
		})
	}

	stackStatus := "running"
	if allDone {
		stackStatus = "complete"
	}
	if activeStageID == "" && !allDone {
		stackStatus = "idle"
	}

	result := map[string]interface{}{
		"stack_name":     stackName,
		"stack_type":     stack.Type,
		"stack_status":   stackStatus,
		"active_stage":   activeStageID,
		"total_stages":   len(stack.Stages),
		"stages":         stages,
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, err
	}

	return mmcp.NewToolResultStructured(result, string(data)), nil
}

func spawnBuildAgent(ctx context.Context, repoRoot string, stacks *state.Stacks, stackName, stageID string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	h, err := harness.ForConfig(cfg)
	if err != nil {
		return err
	}

	stack, _ := state.FindStack(stacks, stackName)
	if stack == nil {
		return fmt.Errorf("stack %q not found", stackName)
	}

	stage, _ := state.FindStage(stack, stageID)
	if stage == nil {
		return fmt.Errorf("stage %q not found", stageID)
	}

	worktreePath := stage.Worktree
	if worktreePath == "" {
		worktreePath = repoRoot
	}

	opts := harness.AgentOpts{
		WorktreePath: worktreePath,
		StageContext: stage.Context,
		StackName:    stackName,
		StageID:      stageID,
		Phase:        "implementing",
	}
	opts.SystemPrompt = harness.BuildSystemPrompt(opts)

	return h.SpawnBuildAgent(ctx, opts)
}

func spawnReviewAgent(ctx context.Context, repoRoot string, stacks *state.Stacks, stackName, stageID string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	h, err := harness.ForConfig(cfg)
	if err != nil {
		return err
	}

	stack, _ := state.FindStack(stacks, stackName)
	if stack == nil {
		return fmt.Errorf("stack %q not found", stackName)
	}

	stage, _ := state.FindStage(stack, stageID)
	if stage == nil {
		return fmt.Errorf("stage %q not found", stageID)
	}

	worktreePath := stage.Worktree
	if worktreePath == "" {
		worktreePath = repoRoot
	}

	opts := harness.AgentOpts{
		WorktreePath: worktreePath,
		StageContext: stage.Context,
		StackName:    stackName,
		StageID:      stageID,
		Phase:        "ai_review",
	}
	opts.SystemPrompt = harness.BuildSystemPrompt(opts)

	return h.SpawnReviewAgent(ctx, opts)
}
