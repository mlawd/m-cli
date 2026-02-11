package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mlawd/m-cli/internal/gitx"
	"github.com/mlawd/m-cli/internal/paths"
	"github.com/mlawd/m-cli/internal/plan"
	"github.com/mlawd/m-cli/internal/state"
	"github.com/spf13/cobra"
)

func newStackRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stack",
		Short: "Manage stacks",
	}

	cmd.AddCommand(
		newStackNewCmd(),
		newStackAttachPlanCmd(),
		newStackRemoveCmd(),
		newStackRebaseCmd(),
		newStackPushCmd(),
		newStackListCmd(),
		newStackSelectCmd(),
		newStackCurrentCmd(),
	)

	return cmd
}

func newStackRemoveCmd() *cobra.Command {
	var force bool
	var deleteWorktrees bool

	cmd := &cobra.Command{
		Use:   "remove <stack-name>",
		Short: "Remove a stack from local m state",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			stackName := strings.TrimSpace(args[0])

			repo, err := discoverRepoContext()
			if err != nil {
				return err
			}

			config, stacksFile, err := loadState(repo)
			if err != nil {
				return err
			}

			stack, stackIdx := state.FindStack(stacksFile, stackName)
			if stack == nil {
				return fmt.Errorf("stack %q not found", stackName)
			}

			if !force && stackHasStartedStages(stack) {
				return fmt.Errorf("stack %q has started stages; rerun with --force", stack.Name)
			}

			if deleteWorktrees {
				stackWorktreesDir := filepath.Join(state.Dir(repo.rootPath), "worktrees", filepath.FromSlash(strings.Trim(stack.Name, "/")))
				if err := os.RemoveAll(stackWorktreesDir); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Removed worktrees: %s\n", stackWorktreesDir)
			}

			stacksFile.Stacks = append(stacksFile.Stacks[:stackIdx], stacksFile.Stacks[stackIdx+1:]...)
			if err := state.SaveStacks(repo.rootPath, stacksFile); err != nil {
				return err
			}

			if config.CurrentStack == stack.Name {
				config.CurrentStack = ""
				if err := state.SaveConfig(repo.rootPath, config); err != nil {
					return err
				}
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Removed stack %q\n", stack.Name)
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Allow removing stack with started stages")
	cmd.Flags().BoolVar(&deleteWorktrees, "delete-worktrees", false, "Also remove .m/worktrees for this stack")

	return cmd
}

func stackHasStartedStages(stack *state.Stack) bool {
	for _, stage := range stack.Stages {
		if strings.TrimSpace(stage.Branch) != "" {
			return true
		}
	}

	return false
}

func newStackRebaseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rebase",
		Short: "Rebase started stage branches in order",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := discoverRepoContext()
			if err != nil {
				return err
			}

			config, stacksFile, err := loadState(repo)
			if err != nil {
				return err
			}

			stack, err := requireCurrentStackWithPlan(config, stacksFile)
			if err != nil {
				return err
			}

			repoInfo, err := gitx.DiscoverRepo(repo.rootPath)
			if err != nil {
				return err
			}

			parentBranch := repoInfo.DefaultBranch
			rebasedCount := 0
			mutated := false

			for i := range stack.Stages {
				stage := &stack.Stages[i]
				branch := strings.TrimSpace(stage.Branch)
				if branch == "" {
					branch = stageBranchName(stack.Name, i, stage.ID)
				}

				if !gitx.BranchExists(repo.rootPath, branch) {
					continue
				}

				worktree := strings.TrimSpace(stage.Worktree)
				if worktree == "" {
					worktree = filepath.Join(state.Dir(repo.rootPath), "worktrees", filepath.FromSlash(branch))
					mutated = true
				}

				if _, err := os.Stat(worktree); os.IsNotExist(err) {
					if err := os.MkdirAll(filepath.Dir(worktree), 0o755); err != nil {
						return err
					}
					if err := gitx.AddWorktree(repo.rootPath, worktree, branch); err != nil {
						return err
					}
					fmt.Fprintf(cmd.OutOrStdout(), "Created worktree %s\n", worktree)
				} else if err != nil {
					return err
				}

				fmt.Fprintf(cmd.OutOrStdout(), "Rebasing %s onto %s\n", branch, parentBranch)
				if _, err := gitx.Run(worktree, "rebase", parentBranch); err != nil {
					return fmt.Errorf("rebase failed for stage %q (%s): %w\nResolve in %s and run `git rebase --continue` or `git rebase --abort`", stage.ID, branch, err, worktree)
				}

				stage.Branch = branch
				stage.Worktree = worktree
				stage.Parent = parentBranch
				parentBranch = branch
				rebasedCount++
				mutated = true
			}

			if mutated {
				if err := state.SaveStacks(repo.rootPath, stacksFile); err != nil {
					return err
				}
			}

			if rebasedCount == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No started stage branches to rebase")
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Rebased %d stage branch(es)\n", rebasedCount)
			return nil
		},
	}
}

func newStackPushCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "push",
		Short: "Push started stage branches with force-with-lease",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := exec.LookPath("gh"); err != nil {
				return fmt.Errorf("gh CLI is required for stack push")
			}

			repo, err := discoverRepoContext()
			if err != nil {
				return err
			}

			config, stacksFile, err := loadState(repo)
			if err != nil {
				return err
			}

			stack, err := requireCurrentStackWithPlan(config, stacksFile)
			if err != nil {
				return err
			}

			startedStageIndexes, err := startedStageIndexes(stack, func(branch string) bool {
				return gitx.BranchExists(repo.rootPath, branch)
			})
			if err != nil {
				return err
			}

			if len(startedStageIndexes) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No started stage branches to push")
				return nil
			}

			for _, stageIndex := range startedStageIndexes {
				if err := pushStageAndEnsurePROpts(cmd, repo.rootPath, stack, stageIndex, true); err != nil {
					return err
				}
			}

			if err := syncStackPRDescriptions(cmd, repo.rootPath, stack, startedStageIndexes); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Pushed %d stage branch(es) with --force-with-lease\n", len(startedStageIndexes))
			return nil
		},
	}
}

func startedStageIndexes(stack *state.Stack, localBranchExists func(branch string) bool) ([]int, error) {
	if stack == nil {
		return nil, fmt.Errorf("stack is required")
	}

	indexes := make([]int, 0, len(stack.Stages))
	for idx := range stack.Stages {
		branch := stageBranchFor(stack, idx)
		if !localBranchExists(branch) {
			continue
		}
		indexes = append(indexes, idx)
	}

	return indexes, nil
}

func newStackNewCmd() *cobra.Command {
	var planFile string

	cmd := &cobra.Command{
		Use:   "new <stack-name>",
		Short: "Create a stack, optionally from a YAML plan file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			stackName := strings.TrimSpace(args[0])
			if err := paths.EnsureValidStackName(stackName); err != nil {
				return err
			}

			repo, err := discoverRepoContext()
			if err != nil {
				return err
			}

			config, stacksFile, err := loadState(repo)
			if err != nil {
				return err
			}

			if existing, _ := state.FindStack(stacksFile, stackName); existing != nil {
				return fmt.Errorf("stack %q already exists", stackName)
			}

			resolvedPlanFile := ""
			stages := []state.Stage{}
			if strings.TrimSpace(planFile) != "" {
				absolutePlanFile, parsedStages, err := parseStagesFromPlanFile(planFile)
				if err != nil {
					return err
				}
				resolvedPlanFile = absolutePlanFile
				stages = parsedStages
			}

			stacksFile.Stacks = append(stacksFile.Stacks, state.NewStack(stackName, resolvedPlanFile, stages))
			if err := state.SaveStacks(repo.rootPath, stacksFile); err != nil {
				return err
			}

			config.CurrentStack = stackName
			if err := state.SaveConfig(repo.rootPath, config); err != nil {
				return err
			}

			if resolvedPlanFile == "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Created stack %q with no attached plan\n", stackName)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Created stack %q with %d stages\n", stackName, len(stages))
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Current stack: %s\n", stackName)
			return nil
		},
	}

	cmd.Flags().StringVar(&planFile, "plan-file", "", "YAML plan file path")

	return cmd
}

func newStackAttachPlanCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "attach-plan <plan-file>",
		Short: "Attach a plan file to the current stack",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := discoverRepoContext()
			if err != nil {
				return err
			}

			config, stacksFile, err := loadState(repo)
			if err != nil {
				return err
			}

			currentStackName := strings.TrimSpace(config.CurrentStack)
			if currentStackName == "" {
				return fmt.Errorf("no stack selected; run: m stack select <stack-name>")
			}

			stack, _ := state.FindStack(stacksFile, currentStackName)
			if stack == nil {
				return fmt.Errorf("current stack %q not found", currentStackName)
			}

			if strings.TrimSpace(stack.PlanFile) != "" {
				return fmt.Errorf("stack %q already has an attached plan: %s", stack.Name, stack.PlanFile)
			}

			absolutePlanFile, parsedStages, err := parseStagesFromPlanFile(args[0])
			if err != nil {
				return err
			}

			stack.PlanFile = absolutePlanFile
			stack.Stages = parsedStages
			stack.CurrentStage = ""

			if err := state.SaveStacks(repo.rootPath, stacksFile); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Attached plan to stack %q with %d stages\n", stack.Name, len(parsedStages))
			fmt.Fprintf(cmd.OutOrStdout(), "Plan file: %s\n", absolutePlanFile)
			return nil
		},
	}
}

func parseStagesFromPlanFile(planFile string) (string, []state.Stage, error) {
	absolutePlanFile, err := filepath.Abs(strings.TrimSpace(planFile))
	if err != nil {
		return "", nil, err
	}

	parsedPlan, err := plan.ParseFile(absolutePlanFile)
	if err != nil {
		return "", nil, err
	}

	stages := make([]state.Stage, 0, len(parsedPlan.Stages))
	for _, stage := range parsedPlan.Stages {
		stages = append(stages, state.Stage{
			ID:          stage.ID,
			Title:       stage.Title,
			Description: stage.Description,
		})
	}

	return absolutePlanFile, stages, nil
}

func newStackListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List stacks",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := discoverRepoContext()
			if err != nil {
				return err
			}

			config, stacksFile, err := loadState(repo)
			if err != nil {
				return err
			}

			if len(stacksFile.Stacks) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No stacks")
				return nil
			}

			for _, stack := range stacksFile.Stacks {
				marker := " "
				if stack.Name == config.CurrentStack {
					marker = "*"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s %s (%d stages)\n", marker, stack.Name, len(stack.Stages))
			}

			return nil
		},
	}
}

func newStackSelectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "select <stack-name>",
		Short: "Set current stack context",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			stackName := strings.TrimSpace(args[0])

			repo, err := discoverRepoContext()
			if err != nil {
				return err
			}

			config, stacksFile, err := loadState(repo)
			if err != nil {
				return err
			}

			if existing, _ := state.FindStack(stacksFile, stackName); existing == nil {
				return fmt.Errorf("stack %q not found", stackName)
			}

			config.CurrentStack = stackName
			if err := state.SaveConfig(repo.rootPath, config); err != nil {
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), stackName)
			return nil
		},
	}
}

func newStackCurrentCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "current",
		Short: "Print current stack",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := discoverRepoContext()
			if err != nil {
				return err
			}

			config, err := state.LoadConfig(repo.rootPath)
			if err != nil {
				return err
			}

			if strings.TrimSpace(config.CurrentStack) == "" {
				return nil
			}

			fmt.Fprintln(cmd.OutOrStdout(), config.CurrentStack)
			return nil
		},
	}
}
