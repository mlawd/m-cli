package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/manifoldco/promptui"
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

	cmd.PersistentFlags().String("stack", "", "Use this stack instead of inferring from workspace")

	cmd.AddCommand(
		newStageListCmd(),
		newStageSelectCmd(),
		newStageCurrentCmd(),
		newStageOpenCmd(),
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

			stacksFile, err := loadState(repo)
			if err != nil {
				return err
			}

			stack, err := requireCurrentStackWithPlan(stacksFile, repo, stackNameFromFlag(cmd))
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

			stageIndexes, err := stageIndexesToPush(stack, stageIndex, func(branch string) bool {
				return gitx.RemoteBranchExists(repo.rootPath, "origin", branch)
			})
			if err != nil {
				return err
			}

			for _, idx := range stageIndexes {
				if err := pushStageAndEnsurePR(cmd, repo.rootPath, stack, idx); err != nil {
					return err
				}
			}

			if err := pushStageAndEnsurePR(cmd, repo.rootPath, stack, stageIndex); err != nil {
				return err
			}

			updatedIndexes := append(stageIndexes, stageIndex)
			if err := syncStackPRDescriptions(cmd, repo.rootPath, stack, updatedIndexes, ""); err != nil {
				return err
			}

			return nil
		},
	}
}

func newStageOpenCmd() *cobra.Command {
	var noOpen bool
	var next bool
	var stageID string

	cmd := &cobra.Command{
		Use:   "open",
		Short: "Open a stage worktree (interactive by default)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if next && strings.TrimSpace(stageID) != "" {
				return fmt.Errorf("--next and --stage cannot be used together")
			}

			repo, err := discoverRepoContext()
			if err != nil {
				return err
			}

			stacksFile, err := loadState(repo)
			if err != nil {
				return err
			}

			if next {
				stack, err := requireCurrentStackWithPlan(stacksFile, repo, stackNameFromFlag(cmd))
				if err != nil {
					return err
				}

				nextIndex, err := nextStageIndex(stack)
				if err != nil {
					return err
				}

				return startStageAtIndex(cmd, repo, stacksFile, stack, nextIndex, true, !noOpen)
			}

			if trimmedStageID := strings.TrimSpace(stageID); trimmedStageID != "" {
				stack, err := requireCurrentStackWithPlan(stacksFile, repo, stackNameFromFlag(cmd))
				if err != nil {
					return err
				}

				_, stageIndex := state.FindStage(stack, trimmedStageID)
				if stageIndex < 0 {
					return fmt.Errorf("stage %q not found in stack %q", trimmedStageID, stack.Name)
				}

				return startStageAtIndex(cmd, repo, stacksFile, stack, stageIndex, false, !noOpen)
			}

			if len(stacksFile.Stacks) == 0 {
				return fmt.Errorf("no stacks found; run: m stack new <stack-name>")
			}

			overrideStack := stackNameFromFlag(cmd)
			if strings.TrimSpace(overrideStack) != "" {
				stack, err := requireCurrentStackWithPlan(stacksFile, repo, overrideStack)
				if err != nil {
					return err
				}

				stageOptions := make([]string, 0, len(stack.Stages))
				for _, stage := range stack.Stages {
					stageOptions = append(stageOptions, fmt.Sprintf("%s - %s", stage.ID, stage.Title))
				}

				stageChoice, err := promptSelectIndex(fmt.Sprintf("Select stage for %s", stack.Name), stageOptions)
				if err != nil {
					return err
				}

				return startStageAtIndex(cmd, repo, stacksFile, stack, stageChoice, false, !noOpen)
			}

			stackIndexes := []int{}
			stackOptions := []string{}
			for idx, stack := range stacksFile.Stacks {
				if strings.TrimSpace(stack.PlanFile) == "" || len(stack.Stages) == 0 {
					continue
				}
				stackIndexes = append(stackIndexes, idx)
				stackOptions = append(stackOptions, fmt.Sprintf("%s  (%d stage%s)", stack.Name, len(stack.Stages), pluralSuffix(len(stack.Stages))))
			}

			if len(stackOptions) == 0 {
				return fmt.Errorf("no stacks with attached plans found; run: m stack attach-plan <plan-file>")
			}

			stackChoice, err := promptSelectIndex("Select stack", stackOptions)
			if err != nil {
				return err
			}

			stack := &stacksFile.Stacks[stackIndexes[stackChoice]]
			stageOptions := make([]string, 0, len(stack.Stages))
			for _, stage := range stack.Stages {
				stageOptions = append(stageOptions, fmt.Sprintf("%s - %s", stage.ID, stage.Title))
			}

			stageChoice, err := promptSelectIndex(fmt.Sprintf("Select stage for %s", stack.Name), stageOptions)
			if err != nil {
				return err
			}

			return startStageAtIndex(cmd, repo, stacksFile, stack, stageChoice, false, !noOpen)
		},
	}

	cmd.Flags().BoolVar(&noOpen, "no-open", false, "Skip launching opencode")
	cmd.Flags().BoolVar(&next, "next", false, "Start and open the next stage in the current stack")
	cmd.Flags().StringVar(&stageID, "stage", "", "Start and open the specified stage id in the current stack")

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

			stacksFile, err := loadState(repo)
			if err != nil {
				return err
			}

			stack, err := requireCurrentStackWithPlan(stacksFile, repo, stackNameFromFlag(cmd))
			if err != nil {
				return err
			}

			effectiveCurrentStage := state.EffectiveCurrentStage(stack, repo.worktreePath)

			if len(stack.Stages) == 0 {
				outInfo(cmd.OutOrStdout(), "No stages found in current stack plan")
				return nil
			}

			for idx, stage := range stack.Stages {
				if stage.ID == effectiveCurrentStage {
					outCurrent(cmd.OutOrStdout(), "%d. %s - %s", idx+1, stage.ID, stage.Title)
					continue
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  %d. %s - %s\n", idx+1, stage.ID, stage.Title)
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

			stacksFile, err := loadState(repo)
			if err != nil {
				return err
			}

			stack, err := requireCurrentStackWithPlan(stacksFile, repo, stackNameFromFlag(cmd))
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

			outCurrent(cmd.OutOrStdout(), "Current stage: %s", selectedStage)
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

			stacksFile, err := loadState(repo)
			if err != nil {
				return err
			}

			_, workspaceStage := state.CurrentWorkspaceStackStage(stacksFile, repo.worktreePath)
			if workspaceStage != "" {
				outCurrent(cmd.OutOrStdout(), "Current stage: %s", workspaceStage)
				return nil
			}

			if state.IsLinkedWorktree(repo.worktreePath, repo.rootPath) {
				return nil
			}

			stack, err := requireCurrentStackWithPlan(stacksFile, repo, stackNameFromFlag(cmd))
			if err != nil {
				return err
			}

			currentStageID := state.EffectiveCurrentStage(stack, repo.worktreePath)
			if currentStageID == "" {
				return nil
			}

			outCurrent(cmd.OutOrStdout(), "Current stage: %s", currentStageID)
			return nil
		},
	}
}

func startStageAtIndex(cmd *cobra.Command, repo *repoContext, stacksFile *state.Stacks, stack *state.Stack, stageIndex int, withPrompt bool, openAgent bool) error {
	if stack == nil {
		return fmt.Errorf("stack is required")
	}
	if stageIndex < 0 || stageIndex >= len(stack.Stages) {
		return fmt.Errorf("stage index %d out of range", stageIndex)
	}

	target := &stack.Stages[stageIndex]
	branch := strings.TrimSpace(target.Branch)
	if branch == "" {
		branch = stageBranchName(stack.Name, stageIndex, target.ID)
	}

	parentBranch, err := parentBranchForStage(repo.rootPath, stack, stageIndex)
	if err != nil {
		return err
	}

	if !gitx.BranchExists(repo.rootPath, branch) {
		if err := gitx.CreateBranch(repo.rootPath, branch, parentBranch); err != nil {
			return err
		}
		outSuccess(cmd.OutOrStdout(), "Created branch %s from %s", branch, parentBranch)
	} else {
		outReuse(cmd.OutOrStdout(), "Reusing branch %s", branch)
	}

	worktree := strings.TrimSpace(target.Worktree)
	if worktree == "" {
		worktree = filepath.Join(state.StacksDir(repo.rootPath), filepath.FromSlash(stack.Name), filepath.FromSlash(target.ID))
	}

	if _, err := os.Stat(worktree); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(worktree), 0o755); err != nil {
			return err
		}
		if err := gitx.AddWorktree(repo.rootPath, worktree, branch); err != nil {
			return err
		}
		outSuccess(cmd.OutOrStdout(), "Created worktree: %s", worktree)
	} else if err != nil {
		return err
	} else {
		outReuse(cmd.OutOrStdout(), "Reusing worktree: %s", worktree)
	}

	target.Branch = branch
	target.Worktree = worktree
	target.Parent = parentBranch
	stack.CurrentStage = target.ID

	if err := state.SaveStacks(repo.rootPath, stacksFile); err != nil {
		return err
	}

	outCurrent(cmd.OutOrStdout(), "Current stack: %s", stack.Name)
	outCurrent(cmd.OutOrStdout(), "Current stage: %s", target.ID)
	if !openAgent {
		return nil
	}

	if withPrompt {
		return agent.StartOpenCodeWithArgs(worktree, "--prompt", stageStartPrompt(target))
	}

	return agent.StartOpenCode(worktree)
}

func promptSelectIndex(label string, options []string) (int, error) {
	if len(options) == 0 {
		return 0, fmt.Errorf("no options available")
	}

	selectPrompt := promptui.Select{
		Label: label,
		Items: options,
		Size:  10,
	}

	idx, _, err := selectPrompt.Run()
	if err != nil {
		return 0, fmt.Errorf("selection cancelled: %w", err)
	}

	return idx, nil
}

func pluralSuffix(count int) string {
	if count == 1 {
		return ""
	}

	return "s"
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
	if context := strings.TrimSpace(stage.Context); context != "" {
		prompt = fmt.Sprintf("%s\n\nStage context:\n%s", prompt, context)
	}

	return prompt
}

func stageBranchFor(stack *state.Stack, stageIndex int) string {
	stage := stack.Stages[stageIndex]
	branch := strings.TrimSpace(stage.Branch)
	if branch == "" {
		branch = stageBranchName(stack.Name, stageIndex, stage.ID)
	}

	return branch
}

func stageIndexesToPush(stack *state.Stack, currentStageIndex int, remoteBranchExists func(branch string) bool) ([]int, error) {
	if stack == nil {
		return nil, fmt.Errorf("stack is required")
	}
	if currentStageIndex < 0 || currentStageIndex >= len(stack.Stages) {
		return nil, fmt.Errorf("current stage index %d out of range", currentStageIndex)
	}

	indexes := make([]int, 0, currentStageIndex)
	for idx := 0; idx < currentStageIndex; idx++ {
		branch := stageBranchFor(stack, idx)
		if remoteBranchExists(branch) {
			continue
		}
		indexes = append(indexes, idx)
	}

	return indexes, nil
}

func pushStageAndEnsurePR(cmd *cobra.Command, repoRoot string, stack *state.Stack, stageIndex int) error {
	return pushStageAndEnsurePROpts(cmd, repoRoot, stack, stageIndex, false, "")
}

func pushStageAndEnsurePROpts(cmd *cobra.Command, repoRoot string, stack *state.Stack, stageIndex int, forceWithLease bool, linePrefix string) error {
	stage := &stack.Stages[stageIndex]
	branch := stageBranchFor(stack, stageIndex)
	if !gitx.BranchExists(repoRoot, branch) {
		return fmt.Errorf("stage branch %q does not exist; run: m stage open --next", branch)
	}

	pushArgs := []string{"push", "-u", "origin", branch}
	if forceWithLease {
		pushArgs = append(pushArgs, "--force-with-lease")
	}

	if _, err := gitx.Run(repoRoot, pushArgs...); err != nil {
		return err
	}
	if forceWithLease {
		outStyledWithPrefix(cmd.OutOrStdout(), ansiBlue, "🚀", linePrefix, "Force-pushed branch %s (--force-with-lease)", branch)
	} else {
		outStyledWithPrefix(cmd.OutOrStdout(), ansiBlue, "🚀", linePrefix, "Pushed branch %s", branch)
	}

	prURL, err := findOpenPRURL(repoRoot, branch)
	if err != nil {
		return err
	}

	baseBranch, err := parentBranchForStage(repoRoot, stack, stageIndex)
	if err != nil {
		return err
	}

	if stageIndex > 0 && !gitx.RemoteBranchExists(repoRoot, "origin", baseBranch) {
		if _, err := gitx.Run(repoRoot, "push", "-u", "origin", baseBranch); err != nil {
			return err
		}
		outStyledWithPrefix(cmd.OutOrStdout(), ansiYellow, "⚠️", linePrefix, "Base branch was missing remotely; pushed %s", baseBranch)
	}

	stackPRURLs, err := collectStackOpenPRURLs(repoRoot, stack)
	if err != nil {
		return err
	}

	title := fmt.Sprintf("%s: %s", stack.Name, stage.Title)
	if strings.TrimSpace(stage.Title) == "" {
		title = fmt.Sprintf("%s: %s", stack.Name, stage.ID)
	}
	body := stagePRBody(stack, stageIndex, stackPRURLs)

	if strings.TrimSpace(prURL) != "" {
		if _, err := runGH(repoRoot, "pr", "edit", prURL, "--body", body); err != nil {
			return err
		}
		outStyledWithPrefix(cmd.OutOrStdout(), ansiCyan, "🔗", linePrefix, "Found existing PR for %s: %s", stage.ID, prURL)
		outStyledWithPrefix(cmd.OutOrStdout(), ansiGreen, "✅", linePrefix, "Updated PR description for %s: %s", stage.ID, prURL)
		return nil
	}

	if _, err := runGH(repoRoot, "pr", "create", "--head", branch, "--base", baseBranch, "--title", title, "--body", body); err != nil {
		return err
	}

	prURL, err = findOpenPRURL(repoRoot, branch)
	if err != nil {
		return err
	}
	if strings.TrimSpace(prURL) == "" {
		return fmt.Errorf("failed to determine PR URL after creation")
	}

	outStyledWithPrefix(cmd.OutOrStdout(), ansiGreen, "✅", linePrefix, "Created PR for %s: %s", stage.ID, prURL)
	return nil
}

func collectStackOpenPRURLs(repoRoot string, stack *state.Stack) (map[int]string, error) {
	urls := make(map[int]string, len(stack.Stages))
	for idx := range stack.Stages {
		branch := stageBranchFor(stack, idx)
		prURL, err := findOpenPRURL(repoRoot, branch)
		if err != nil {
			return nil, err
		}
		urls[idx] = strings.TrimSpace(prURL)
	}

	return urls, nil
}

func stagePRBody(stack *state.Stack, stageIndex int, stackPRURLs map[int]string) string {
	stage := stack.Stages[stageIndex]
	hasDetails := strings.TrimSpace(stage.Outcome) != "" || len(stage.Implementation) > 0 || len(stage.Validation) > 0 || len(stage.Risks) > 0 || strings.TrimSpace(stage.Context) != ""

	var body strings.Builder
	body.WriteString(fmt.Sprintf("Stage: %s", stage.ID))
	if outcome := strings.TrimSpace(stage.Outcome); outcome != "" {
		body.WriteString("\n\n## Outcome\n")
		body.WriteString(outcome)
	}

	if len(stage.Implementation) > 0 {
		body.WriteString("\n\n## Implementation\n")
		body.WriteString(formatBulletList(stage.Implementation))
	}

	if len(stage.Validation) > 0 {
		body.WriteString("\n\n## Validation\n")
		body.WriteString(formatBulletList(stage.Validation))
	}

	if len(stage.Risks) > 0 {
		body.WriteString("\n\n## Risks\n")
		for i, risk := range stage.Risks {
			body.WriteString(fmt.Sprintf("- Risk: %s\n  Mitigation: %s", strings.TrimSpace(risk.Risk), strings.TrimSpace(risk.Mitigation)))
			if i < len(stage.Risks)-1 {
				body.WriteString("\n")
			}
		}
	}

	if context := strings.TrimSpace(stage.Context); context != "" {
		body.WriteString("\n\n## Context\n")
		body.WriteString(context)
	}

	if !hasDetails {
		body.WriteString("\n\n")
		body.WriteString("No implementation details found for this stage.")
	}

	body.WriteString("\n\n## Stack PRs\n\n### Earlier stages (base chain)\n")
	upstream := stackPRListLines(stack, stageIndex, stackPRURLs, true)
	if len(upstream) == 0 {
		body.WriteString("- None\n")
	} else {
		for _, line := range upstream {
			body.WriteString(line)
			body.WriteString("\n")
		}
	}

	body.WriteString("\n### Later stages (dependent chain)\n")
	downstream := stackPRListLines(stack, stageIndex, stackPRURLs, false)
	if len(downstream) == 0 {
		body.WriteString("- None")
	} else {
		for i, line := range downstream {
			body.WriteString(line)
			if i < len(downstream)-1 {
				body.WriteString("\n")
			}
		}
	}

	return body.String()
}

func formatBulletList(items []string) string {
	var lines []string
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		lines = append(lines, fmt.Sprintf("- %s", trimmed))
	}

	return strings.Join(lines, "\n")
}

func stackPRListLines(stack *state.Stack, stageIndex int, stackPRURLs map[int]string, upstream bool) []string {
	lines := []string{}
	if upstream {
		for idx := 0; idx < stageIndex; idx++ {
			stage := stack.Stages[idx]
			if prURL := strings.TrimSpace(stackPRURLs[idx]); prURL != "" {
				lines = append(lines, fmt.Sprintf("- %s: %s", stage.ID, prURL))
				continue
			}
			lines = append(lines, fmt.Sprintf("- %s: (not created)", stage.ID))
		}
		return lines
	}

	for idx := stageIndex + 1; idx < len(stack.Stages); idx++ {
		stage := stack.Stages[idx]
		if prURL := strings.TrimSpace(stackPRURLs[idx]); prURL != "" {
			lines = append(lines, fmt.Sprintf("- %s: %s", stage.ID, prURL))
			continue
		}
		lines = append(lines, fmt.Sprintf("- %s: (not created)", stage.ID))
	}

	return lines
}

func syncStackPRDescriptions(cmd *cobra.Command, repoRoot string, stack *state.Stack, stageIndexes []int, linePrefix string) error {
	if len(stageIndexes) == 0 {
		return nil
	}

	stackPRURLs, err := collectStackOpenPRURLs(repoRoot, stack)
	if err != nil {
		return err
	}

	updated := map[int]struct{}{}
	for _, stageIndex := range stageIndexes {
		if stageIndex < 0 || stageIndex >= len(stack.Stages) {
			continue
		}
		if _, seen := updated[stageIndex]; seen {
			continue
		}

		prURL := strings.TrimSpace(stackPRURLs[stageIndex])
		if prURL == "" {
			continue
		}

		body := stagePRBody(stack, stageIndex, stackPRURLs)
		if _, err := runGH(repoRoot, "pr", "edit", prURL, "--body", body); err != nil {
			return err
		}
		outStyledWithPrefix(cmd.OutOrStdout(), ansiGreen, "✅", linePrefix, "Synced PR description for %s: %s", stack.Stages[stageIndex].ID, prURL)
		updated[stageIndex] = struct{}{}
	}

	return nil
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
