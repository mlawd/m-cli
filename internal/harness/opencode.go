package harness

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/mlawd/m-cli/internal/config"
)

type OpenCodeHarness struct {
	Config *config.Config
}

func (h *OpenCodeHarness) SpawnBuildAgent(ctx context.Context, opts AgentOpts) error {
	agentName := "build"
	if entry, ok := h.Config.Agents["build"]; ok {
		agentName = entry.Agent
	}

	return h.spawn(ctx, agentName, opts)
}

func (h *OpenCodeHarness) SpawnReviewAgent(ctx context.Context, opts AgentOpts) error {
	agentName := "review"
	if entry, ok := h.Config.Agents["review"]; ok {
		agentName = entry.Agent
	}

	return h.spawn(ctx, agentName, opts)
}

func (h *OpenCodeHarness) spawn(ctx context.Context, agentName string, opts AgentOpts) error {
	path, err := exec.LookPath("opencode")
	if err != nil {
		return fmt.Errorf("opencode not found in PATH; install it or switch agent_harness to claude")
	}

	args := []string{"--agent", agentName, "--prompt", opts.SystemPrompt}

	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Dir = opts.WorktreePath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("spawn opencode %s agent: %w", agentName, err)
	}

	go func() {
		_ = cmd.Wait()
	}()

	return nil
}

func BuildSystemPrompt(opts AgentOpts) string {
	var b strings.Builder

	if opts.Phase == "implementing" {
		b.WriteString(fmt.Sprintf("You are implementing stage %q of stack %q.\n\n", opts.StageID, opts.StackName))
	} else {
		b.WriteString(fmt.Sprintf("You are reviewing stage %q of stack %q.\n\n", opts.StageID, opts.StackName))
	}

	if opts.StageContext != "" {
		b.WriteString("## Stage Context\n\n")
		b.WriteString(opts.StageContext)
		b.WriteString("\n\n")
	}

	b.WriteString("## Completion\n\n")
	b.WriteString("When your work is complete, call the report_stage_done MCP tool with:\n")
	b.WriteString(fmt.Sprintf("- stack_name: %q\n", opts.StackName))
	b.WriteString(fmt.Sprintf("- stage_id: %q\n", opts.StageID))
	b.WriteString(fmt.Sprintf("- phase: %q\n", opts.Phase))
	b.WriteString("- summary: a brief summary of what you did\n")

	return b.String()
}
