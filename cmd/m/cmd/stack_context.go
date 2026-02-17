package cmd

import (
	"fmt"
	"strings"

	"github.com/mlawd/m-cli/internal/state"
	"github.com/spf13/cobra"
)

func stackNameFromFlag(cmd *cobra.Command) string {
	if cmd == nil {
		return ""
	}

	name, err := cmd.Flags().GetString("stack")
	if err != nil {
		return ""
	}

	return strings.TrimSpace(name)
}

func resolveCurrentStack(stacksFile *state.Stacks, repo *repoContext, override string) (*state.Stack, error) {
	if stacksFile == nil {
		return nil, fmt.Errorf("state is required")
	}

	if trimmed := strings.TrimSpace(override); trimmed != "" {
		stack, _ := state.FindStack(stacksFile, trimmed)
		if stack == nil {
			return nil, fmt.Errorf("stack %q not found", trimmed)
		}

		return stack, nil
	}

	if stackName, _ := state.CurrentWorkspaceStackStageByPath(repo.rootPath, repo.worktreePath); stackName != "" {
		if stack, _ := state.FindStack(stacksFile, stackName); stack != nil {
			return stack, nil
		}
	}

	if stackName, _ := state.CurrentWorkspaceStackStage(stacksFile, repo.worktreePath); stackName != "" {
		if stack, _ := state.FindStack(stacksFile, stackName); stack != nil {
			return stack, nil
		}
	}

	if state.IsLinkedWorktree(repo.worktreePath, repo.rootPath) {
		return nil, nil
	}

	if len(stacksFile.Stacks) == 1 {
		return &stacksFile.Stacks[0], nil
	}

	return nil, nil
}

func requireCurrentStack(stacksFile *state.Stacks, repo *repoContext, override string) (*state.Stack, error) {
	stack, err := resolveCurrentStack(stacksFile, repo, override)
	if err != nil {
		return nil, err
	}
	if stack == nil {
		return nil, fmt.Errorf("could not infer stack from workspace; run from .m/stacks/<stack> (or .m/stacks/<stack>/<stage>) or use `m stage open` for interactive selection")
	}

	return stack, nil
}

func requireCurrentStackWithPlan(stacksFile *state.Stacks, repo *repoContext, override string) (*state.Stack, error) {
	stack, err := requireCurrentStack(stacksFile, repo, override)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(stack.PlanFile) == "" || len(stack.Stages) == 0 {
		return nil, fmt.Errorf("no plan attached to current stack; run: m stack attach-plan <plan-file>")
	}

	return stack, nil
}
