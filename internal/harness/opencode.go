package harness

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type opencodeHarness struct{}

func (h *opencodeHarness) SpawnBuildAgent(ctx context.Context, opts AgentOpts) error {
	path, err := exec.LookPath("opencode")
	if err != nil {
		return fmt.Errorf("opencode not found in PATH; install it or set agent_harness to a supported value")
	}

	prompt := buildAgentPrompt(opts)
	return runDetached(ctx, path, opts.WorktreePath, "--agent", "build", "--message", prompt)
}

func (h *opencodeHarness) SpawnReviewAgent(ctx context.Context, opts AgentOpts) error {
	path, err := exec.LookPath("opencode")
	if err != nil {
		return fmt.Errorf("opencode not found in PATH; install it or set agent_harness to a supported value")
	}

	prompt := reviewAgentPrompt(opts)
	return runDetached(ctx, path, opts.WorktreePath, "--agent", "review", "--message", prompt)
}

// buildAgentPrompt constructs the prompt injected for the build (implementing) phase.
func buildAgentPrompt(opts AgentOpts) string {
	var b strings.Builder
	if opts.SystemPrompt != "" {
		b.WriteString(opts.SystemPrompt)
		b.WriteString("\n\n")
	}
	fmt.Fprintf(&b, "Stack: %s\nStage: %s\nPhase: implementing\n\n", opts.StackName, opts.StageID)
	if opts.StageContext != "" {
		b.WriteString("## Stage Context\n\n")
		b.WriteString(opts.StageContext)
		b.WriteString("\n\n")
	}
	b.WriteString("When your implementation work is complete, call the `report_stage_done` MCP tool with:\n")
	b.WriteString("  stack_name: " + opts.StackName + "\n")
	b.WriteString("  stage_id: " + opts.StageID + "\n")
	b.WriteString("  phase: implementing\n")
	return strings.TrimSpace(b.String())
}

// reviewAgentPrompt constructs the prompt injected for the review (ai_review) phase.
func reviewAgentPrompt(opts AgentOpts) string {
	var b strings.Builder
	if opts.SystemPrompt != "" {
		b.WriteString(opts.SystemPrompt)
		b.WriteString("\n\n")
	}
	fmt.Fprintf(&b, "Stack: %s\nStage: %s\nPhase: ai_review\n\n", opts.StackName, opts.StageID)
	if opts.StageContext != "" {
		b.WriteString("## Stage Context\n\n")
		b.WriteString(opts.StageContext)
		b.WriteString("\n\n")
	}
	if opts.DiffBase != "" {
		b.WriteString("## Diff Base\n\n")
		fmt.Fprintf(&b, "Review the changes since: %s\n\n", opts.DiffBase)
	}
	b.WriteString("When your review work is complete, call the `report_stage_done` MCP tool with:\n")
	b.WriteString("  stack_name: " + opts.StackName + "\n")
	b.WriteString("  stage_id: " + opts.StageID + "\n")
	b.WriteString("  phase: ai_review\n")
	return strings.TrimSpace(b.String())
}

// runDetached spawns cmd in a new process group so it survives the parent process exiting.
func runDetached(_ context.Context, path, dir string, args ...string) error {
	cmd := exec.Command(path, args...)
	cmd.Dir = dir
	cmd.Stdin = nil
	cmd.Stdout = os.Stderr // redirect to stderr so watch output stays clean
	cmd.Stderr = os.Stderr

	setSysProcAttr(cmd) // platform-specific: create new process group

	return cmd.Start()
}
