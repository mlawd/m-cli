package harness

import (
	"context"
	"fmt"
	"os/exec"
)

type claudeHarness struct{}

func (h *claudeHarness) SpawnBuildAgent(ctx context.Context, opts AgentOpts) error {
	path, err := exec.LookPath("claude")
	if err != nil {
		return fmt.Errorf("claude not found in PATH; install Claude Code or set agent_harness to a supported value")
	}

	prompt := buildAgentPrompt(opts)
	return runDetached(ctx, path, opts.WorktreePath, "--print", prompt)
}

func (h *claudeHarness) SpawnReviewAgent(ctx context.Context, opts AgentOpts) error {
	path, err := exec.LookPath("claude")
	if err != nil {
		return fmt.Errorf("claude not found in PATH; install Claude Code or set agent_harness to a supported value")
	}

	prompt := reviewAgentPrompt(opts)
	return runDetached(ctx, path, opts.WorktreePath, "--print", prompt)
}
