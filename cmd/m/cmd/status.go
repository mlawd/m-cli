package cmd

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/mlawd/m-cli/internal/gitx"
	"github.com/mlawd/m-cli/internal/state"
	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show current repo and m workflow status",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := discoverRepoContext()
			if err != nil {
				return err
			}

			branch, err := gitx.Run(repo.worktreePath, "rev-parse", "--abbrev-ref", "HEAD")
			if err != nil {
				return err
			}

			initialized := false
			if _, err := os.Stat(state.Dir(repo.rootPath)); err == nil {
				initialized = true
			} else if !os.IsNotExist(err) {
				return err
			}

			config, err := state.LoadConfig(repo.rootPath)
			if err != nil {
				return err
			}

			stacksFile, err := state.LoadStacks(repo.rootPath)
			if err != nil {
				return err
			}

			currentStack := ""
			currentStage := ""

			workspaceStack, workspaceStage := state.CurrentWorkspaceStackStage(stacksFile, repo.worktreePath)
			if workspaceStack != "" {
				currentStack = workspaceStack
				currentStage = workspaceStage
			} else {
				currentStack = strings.TrimSpace(config.CurrentStack)
				if currentStack != "" {
					if stack, _ := state.FindStack(stacksFile, currentStack); stack != nil {
						currentStage = state.EffectiveCurrentStage(stack, repo.worktreePath)
					}
				}
			}

			outInfo(cmd.OutOrStdout(), "Repo root: %s", repo.rootPath)
			outInfo(cmd.OutOrStdout(), "Worktree: %s", repo.worktreePath)
			outInfo(cmd.OutOrStdout(), "Branch: %s", strings.TrimSpace(branch))
			outInfo(cmd.OutOrStdout(), "m state: %s", boolWord(initialized))

			managedRoot := filepath.Join(state.Dir(repo.rootPath), "worktrees")
			outInfo(cmd.OutOrStdout(), "Managed worktrees dir: %s", managedRoot)
			outInfo(cmd.OutOrStdout(), "Stacks: %d", len(stacksFile.Stacks))

			if currentStack == "" {
				outInfo(cmd.OutOrStdout(), "Current stack: (none)")
				outInfo(cmd.OutOrStdout(), "Next: m stack list && m stack select <stack-name>")
			} else {
				outCurrent(cmd.OutOrStdout(), "Current stack: %s", currentStack)
				if stack, _ := state.FindStack(stacksFile, currentStack); stack != nil {
					outInfo(cmd.OutOrStdout(), "Stages in current stack: %d", len(stack.Stages))
				}
			}

			if currentStage == "" {
				outInfo(cmd.OutOrStdout(), "Current stage: (none)")
				if currentStack != "" {
					outInfo(cmd.OutOrStdout(), "Next: m stage list && m stage select <stage-id>")
				}
			} else {
				outCurrent(cmd.OutOrStdout(), "Current stage: %s", currentStage)
			}

			if state.IsLinkedWorktree(repo.worktreePath, repo.rootPath) && workspaceStack == "" {
				outInfo(cmd.OutOrStdout(), "Linked worktree is not mapped to a stack stage")
			}

			return nil
		},
	}
}

func boolWord(v bool) string {
	if v {
		return "yes"
	}

	return "no"
}
