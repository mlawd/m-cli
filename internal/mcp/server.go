package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	mmcp "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
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
	registerPrompts(srv)

	stdio := mcpserver.NewStdioServer(srv)
	return stdio.Listen(ctx, in, out)
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
			mmcp.WithResourceDescription("JSON snapshot of repo-local m state, including selected stack and stage"),
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
			mmcp.WithDescription("Return current repository context for m, including selected stack and stage"),
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

			steps := []string{
				"Read `m://state/context` or call `get_m_context`.",
				"If no stack is selected, run `m stack list` then `m stack select <stack-name>`.",
				"If no stage is selected, run `m stage list` then `m stage select <stage-id>`.",
				"Execute implementation tasks for the selected stage only.",
				"Before handoff, re-check with `m stage current` and summarize stage-scoped changes.",
			}

			headline := fmt.Sprintf("Goal: %s", goal)
			if snapshot.CurrentStack != "" {
				headline += fmt.Sprintf(" | current stack: %s", snapshot.CurrentStack)
			}
			if snapshot.CurrentStage != "" {
				headline += fmt.Sprintf(" | current stage: %s", snapshot.CurrentStage)
			}

			var textBuilder strings.Builder
			textBuilder.WriteString(headline)
			textBuilder.WriteString("\n\nSuggested plan:\n")
			for i, step := range steps {
				textBuilder.WriteString(fmt.Sprintf("%d. %s\n", i+1, step))
			}

			structured := map[string]interface{}{
				"goal":  goal,
				"steps": steps,
				"context": map[string]interface{}{
					"current_stack": snapshot.CurrentStack,
					"current_stage": snapshot.CurrentStage,
				},
			}

			return mmcp.NewToolResultStructured(structured, strings.TrimSpace(textBuilder.String())), nil
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

			promptText := fmt.Sprintf("Goal: %s\n\nUse the m context and workflow resources before planning. Propose a stage-aligned plan and identify the exact m commands needed to confirm or update stack/stage context.", goal)

			return mmcp.NewGetPromptResult(
				"Stage-aware planning prompt for m",
				[]mmcp.PromptMessage{mmcp.NewPromptMessage(mmcp.RoleUser, mmcp.NewTextContent(promptText))},
			), nil
		},
	)
}
