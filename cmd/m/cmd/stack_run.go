package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/mlawd/m-cli/internal/config"
	"github.com/mlawd/m-cli/internal/harness"
	"github.com/mlawd/m-cli/internal/state"
	"github.com/spf13/cobra"
)

func newStackRunCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "run",
		Short: "Start the automated implement -> review pipeline for a stack",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := discoverRepoContext()
			if err != nil {
				return err
			}

			stacksFile, err := loadState(repo)
			if err != nil {
				return err
			}

			stack, err := requireCurrentStackWithPlan(stacksFile, repo, stackNameFromFlag(cmd))
			if err != nil {
				return err
			}

			// Must have at least one pending stage
			firstPending := state.NextPendingStage(stack)
			if firstPending == nil {
				return fmt.Errorf("no pending stages in stack %q; all stages are in progress or complete", stack.Name)
			}

			// Load global config
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			if err := config.ValidateConfig(cfg); err != nil {
				return fmt.Errorf("invalid config: %w", err)
			}

			// Verify harness binary is available
			harnessName := strings.ToLower(cfg.AgentHarness)
			if _, lookErr := exec.LookPath(harnessName); lookErr != nil {
				return fmt.Errorf("%s not found in PATH; install it or run: m config set agent_harness <opencode|claude>", harnessName)
			}

			// Ensure agent definition files exist
			if err := ensureAgentDefinitions(repo.rootPath, cfg); err != nil {
				outWarn(cmd.OutOrStdout(), "Could not write agent definitions: %v", err)
			}

			// Ensure the first pending stage has a worktree
			if err := ensureStageWorktree(repo, stacksFile, stack, firstPending); err != nil {
				return fmt.Errorf("prepare stage worktree: %w", err)
			}

			// Transition first pending stage to implementing
			if err := state.TransitionStage(stacksFile, stack.Name, firstPending.ID, state.StatusImplementing); err != nil {
				return err
			}
			if err := state.SaveStacks(repo.rootPath, stacksFile); err != nil {
				return err
			}

			// Spawn build agent
			h, err := harness.ForConfig(cfg)
			if err != nil {
				return err
			}

			worktreePath := firstPending.Worktree
			if worktreePath == "" {
				worktreePath = repo.rootPath
			}

			opts := harness.AgentOpts{
				WorktreePath: worktreePath,
				StageContext: firstPending.Context,
				StackName:    stack.Name,
				StageID:      firstPending.ID,
				Phase:        "implementing",
			}
			opts.SystemPrompt = harness.BuildSystemPrompt(opts)

			if err := h.SpawnBuildAgent(cmd.Context(), opts); err != nil {
				return fmt.Errorf("spawn build agent: %w", err)
			}

			outSuccess(cmd.OutOrStdout(), "Stack %q started. Stage %q is now implementing.", formatStackDisplayName(*stack), firstPending.ID)
			outInfo(cmd.OutOrStdout(), "Run `m stack watch` to follow progress.")

			return nil
		},
	}
}

func ensureStageWorktree(repo *repoContext, stacksFile *state.Stacks, stack *state.Stack, stage *state.Stage) error {
	if strings.TrimSpace(stage.Worktree) != "" {
		if _, err := os.Stat(stage.Worktree); err == nil {
			return nil
		}
	}

	_, stageIndex := state.FindStage(stack, stage.ID)
	if stageIndex < 0 {
		return fmt.Errorf("stage %q not found", stage.ID)
	}

	branch := strings.TrimSpace(stage.Branch)
	if branch == "" {
		branch = stageBranchName(stack.Name, stageIndex, stage.ID)
	}

	parentBranch, err := parentBranchForStage(repo.rootPath, stack, stageIndex)
	if err != nil {
		return err
	}

	return startStageWorktreeOnly(repo, stacksFile, stack, stageIndex, branch, parentBranch)
}
