package harness

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/mlawd/m-cli/internal/config"
)

type ClaudeHarness struct {
	Config *config.Config
}

func (h *ClaudeHarness) SpawnBuildAgent(ctx context.Context, opts AgentOpts) error {
	agentName := "build"
	if entry, ok := h.Config.Agents["build"]; ok {
		agentName = entry.Agent
	}

	return h.spawn(ctx, agentName, opts)
}

func (h *ClaudeHarness) SpawnReviewAgent(ctx context.Context, opts AgentOpts) error {
	agentName := "review"
	if entry, ok := h.Config.Agents["review"]; ok {
		agentName = entry.Agent
	}

	return h.spawn(ctx, agentName, opts)
}

func (h *ClaudeHarness) spawn(ctx context.Context, agentName string, opts AgentOpts) error {
	path, err := exec.LookPath("claude")
	if err != nil {
		return fmt.Errorf("claude not found in PATH; install it or switch agent_harness to opencode")
	}

	args := []string{"--agent", agentName, "--prompt", opts.SystemPrompt}

	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Dir = opts.WorktreePath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("spawn claude %s agent: %w", agentName, err)
	}

	go func() {
		_ = cmd.Wait()
	}()

	return nil
}
