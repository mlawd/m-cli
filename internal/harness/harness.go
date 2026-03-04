// Package harness provides abstractions for spawning AI agent processes
// (opencode, claude, etc.) as part of the m stack run pipeline.
package harness

import "context"

// AgentOpts contains the parameters passed to a spawned agent.
type AgentOpts struct {
	// WorktreePath is the directory where the agent should run.
	WorktreePath string
	// StageContext is the freeform body of the ## Stage: <id> section from the plan.
	StageContext string
	// StackName identifies the parent stack.
	StackName string
	// StageID identifies the stage being worked on.
	StageID string
	// Phase is "implementing" or "ai_review".
	Phase string
	// SystemPrompt is injected before stage context.
	SystemPrompt string
	// MCPServerAddr is the address (or socket path) of the running m MCP server,
	// included in the prompt so the agent knows how to call report_stage_done.
	MCPServerAddr string
	// DiffBase is the git ref the review agent should diff from (used in ai_review).
	DiffBase string
}

// Harness can spawn build and review agents using a particular AI CLI tool.
type Harness interface {
	SpawnBuildAgent(ctx context.Context, opts AgentOpts) error
	SpawnReviewAgent(ctx context.Context, opts AgentOpts) error
}

// New returns a Harness for the given harness name ("opencode" or "claude").
func New(harnessName string) (Harness, error) {
	switch harnessName {
	case "opencode":
		return &opencodeHarness{}, nil
	case "claude":
		return &claudeHarness{}, nil
	default:
		return &opencodeHarness{}, nil
	}
}
