package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mlawd/m-cli/internal/agent"
	"github.com/mlawd/m-cli/internal/gitx"
	"github.com/mlawd/m-cli/internal/state"
	"github.com/spf13/cobra"
)

func newStageRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stage",
		Short: "Manage stages in the current stack",
	}

	cmd.AddCommand(
		newStageListCmd(),
		newStageSelectCmd(),
		newStageCurrentCmd(),
		newStageStartNextCmd(),
		newStagePushCmd(),
	)

	return cmd
}

func newStagePushCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "push",
		Short: "Push current stage branch and create PR if missing",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := exec.LookPath("gh"); err != nil {
				return fmt.Errorf("gh CLI is required for stage push")
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

			currentStageID := state.EffectiveCurrentStage(stack, repo.worktreePath)
			if currentStageID == "" {
				return fmt.Errorf("no stage selected; run: m stage select <stage-id>")
			}

			stage, stageIndex := state.FindStage(stack, currentStageID)
			if stage == nil {
				return fmt.Errorf("current stage %q not found in stack %q", currentStageID, stack.Name)
			}

			branch := strings.TrimSpace(stage.Branch)
			if branch == "" {
				branch = stageBranchName(stack.Name, stageIndex, stage.ID)
			}
			if !gitx.BranchExists(repo.rootPath, branch) {
				return fmt.Errorf("stage branch %q does not exist; run: m stage start-next", branch)
			}

			if _, err := gitx.Run(repo.rootPath, "push", "-u", "origin", branch); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Pushed %s\n", branch)

			prURL, err := findOpenPRURL(repo.rootPath, branch)
			if err != nil {
				return err
			}
			if strings.TrimSpace(prURL) != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Existing PR: %s\n", prURL)
				return nil
			}

			baseBranch, err := parentBranchForStage(repo.rootPath, stack, stageIndex)
			if err != nil {
				return err
			}

			title := fmt.Sprintf("%s: %s", stack.Name, stage.Title)
			if strings.TrimSpace(stage.Title) == "" {
				title = fmt.Sprintf("%s: %s", stack.Name, stage.ID)
			}
			body := fmt.Sprintf("Stage: %s\n\n%s", stage.ID, strings.TrimSpace(stage.Description))

			if _, err := runGH(repo.rootPath, "pr", "create", "--head", branch, "--base", baseBranch, "--title", title, "--body", body); err != nil {
				return err
			}

			prURL, err = findOpenPRURL(repo.rootPath, branch)
			if err != nil {
				return err
			}
			if strings.TrimSpace(prURL) == "" {
				return fmt.Errorf("failed to determine PR URL after creation")
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Created PR: %s\n", prURL)
			return nil
		},
	}
}

func newStageStartNextCmd() *cobra.Command {
	var noOpen bool

	cmd := &cobra.Command{
		Use:   "start-next",
		Short: "Start and select the next stage in order",
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

			nextIndex, err := nextStageIndex(stack)
			if err != nil {
				return err
			}

			target := &stack.Stages[nextIndex]
			branch := target.Branch
			if strings.TrimSpace(branch) == "" {
				branch = stageBranchName(stack.Name, nextIndex, target.ID)
			}

			parentBranch, err := parentBranchForStage(repo.rootPath, stack, nextIndex)
			if err != nil {
				return err
			}

			if !gitx.BranchExists(repo.rootPath, branch) {
				if err := gitx.CreateBranch(repo.rootPath, branch, parentBranch); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Created %s (from %s)\n", branch, parentBranch)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Reusing %s\n", branch)
			}

			worktree := target.Worktree
			if strings.TrimSpace(worktree) == "" {
				worktree = filepath.Join(state.Dir(repo.rootPath), "worktrees", filepath.FromSlash(branch))
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
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Reusing worktree %s\n", worktree)
			}

			target.Branch = branch
			target.Worktree = worktree
			target.Parent = parentBranch
			stack.CurrentStage = target.ID

			if err := state.SaveStacks(repo.rootPath, stacksFile); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Current stage: %s\n", target.ID)
			if noOpen {
				return nil
			}

			return agent.StartOpenCodeWithArgs(worktree, "--prompt", stageStartPrompt(target))
		},
	}

	cmd.Flags().BoolVar(&noOpen, "no-open", false, "Skip launching opencode")

	return cmd
}

func newStageListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List stages for the current stack",
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

			effectiveCurrentStage := state.EffectiveCurrentStage(stack, repo.worktreePath)

			if len(stack.Stages) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No stages")
				return nil
			}

			for idx, stage := range stack.Stages {
				marker := " "
				if stage.ID == effectiveCurrentStage {
					marker = "*"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s %d. %s - %s\n", marker, idx+1, stage.ID, stage.Title)
			}

			return nil
		},
	}
}

func newStageSelectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "select <stage>",
		Short: "Select a stage in the current stack",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			selectedStage := strings.TrimSpace(args[0])

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

			if stage, _ := state.FindStage(stack, selectedStage); stage == nil {
				return fmt.Errorf("stage %q not found in stack %q", selectedStage, stack.Name)
			}

			stack.CurrentStage = selectedStage
			if err := state.SaveStacks(repo.rootPath, stacksFile); err != nil {
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), selectedStage)
			return nil
		},
	}
}

func newStageCurrentCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "current",
		Short: "Print current stage for current stack",
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

			currentStageID := state.EffectiveCurrentStage(stack, repo.worktreePath)
			if currentStageID == "" {
				return nil
			}

			fmt.Fprintln(cmd.OutOrStdout(), currentStageID)
			return nil
		},
	}
}

func requireCurrentStackWithPlan(config *state.Config, stacksFile *state.Stacks) (*state.Stack, error) {
	if strings.TrimSpace(config.CurrentStack) == "" {
		return nil, fmt.Errorf("no stack selected; run: m stack select <stack-name>")
	}

	stack, _ := state.FindStack(stacksFile, config.CurrentStack)
	if stack == nil {
		return nil, fmt.Errorf("current stack %q not found", config.CurrentStack)
	}

	if strings.TrimSpace(stack.PlanFile) == "" || len(stack.Stages) == 0 {
		return nil, fmt.Errorf("no plan attached to current stack; run: m stack attach-plan <plan-file>")
	}

	return stack, nil
}

func nextStageIndex(stack *state.Stack) (int, error) {
	if strings.TrimSpace(stack.CurrentStage) == "" {
		return 0, nil
	}

	_, currentIndex := state.FindStage(stack, stack.CurrentStage)
	if currentIndex < 0 {
		return 0, fmt.Errorf("current stage %q not found in stack %q", stack.CurrentStage, stack.Name)
	}

	nextIndex := currentIndex + 1
	if nextIndex >= len(stack.Stages) {
		return 0, fmt.Errorf("already at final stage %q", stack.CurrentStage)
	}

	return nextIndex, nil
}

func parentBranchForStage(repoRoot string, stack *state.Stack, stageIndex int) (string, error) {
	if stageIndex == 0 {
		repo, err := gitx.DiscoverRepo(repoRoot)
		if err != nil {
			return "", err
		}
		return repo.DefaultBranch, nil
	}

	previous := stack.Stages[stageIndex-1]
	branch := strings.TrimSpace(previous.Branch)
	if branch == "" {
		branch = stageBranchName(stack.Name, stageIndex-1, previous.ID)
	}

	if !gitx.BranchExists(repoRoot, branch) {
		return "", fmt.Errorf("previous stage branch %q does not exist; start stage %q first", branch, previous.ID)
	}

	return branch, nil
}

func stageBranchName(stackName string, stageIndex int, stageID string) string {
	return fmt.Sprintf("%s/%d/%s", strings.Trim(stackName, "/"), stageIndex+1, stageID)
}

func stageStartPrompt(stage *state.Stage) string {
	prompt := fmt.Sprintf("Implement stage %s", stage.ID)
	if title := strings.TrimSpace(stage.Title); title != "" {
		prompt = fmt.Sprintf("%s: %s", prompt, title)
	}

	return prompt
}

func findOpenPRURL(repoRoot, headBranch string) (string, error) {
	out, err := runGH(repoRoot, "pr", "list", "--state", "open", "--head", headBranch, "--json", "url", "--limit", "1")
	if err != nil {
		return "", err
	}

	var prs []struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal([]byte(out), &prs); err != nil {
		return "", fmt.Errorf("parse gh pr list output: %w", err)
	}
	if len(prs) == 0 {
		return "", nil
	}

	return strings.TrimSpace(prs[0].URL), nil
}

func runGH(dir string, args ...string) (string, error) {
	cmd := exec.Command("gh", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	trimmed := strings.TrimSpace(string(out))
	if err != nil {
		if trimmed == "" {
			trimmed = err.Error()
		}
		return "", fmt.Errorf("gh %s: %s", strings.Join(args, " "), trimmed)
	}

	return trimmed, nil
}
