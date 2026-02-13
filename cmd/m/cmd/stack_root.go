package cmd

import (
	"encoding/json"
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
		newStackSyncCmd(),
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
				outStyled(cmd.OutOrStdout(), ansiBlue, "🧹", "Removed worktrees: %s", stackWorktreesDir)
			}

			var removedStackName string
			stacksFile.Stacks, removedStackName = removeStackByIndex(stacksFile.Stacks, stackIdx)
			if err := state.SaveStacks(repo.rootPath, stacksFile); err != nil {
				return err
			}

			if config.CurrentStack == removedStackName {
				config.CurrentStack = ""
				if err := state.SaveConfig(repo.rootPath, config); err != nil {
					return err
				}
			}

			outSuccess(cmd.OutOrStdout(), "Removed stack %q", removedStackName)
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Allow removing stack with started stages")
	cmd.Flags().BoolVar(&deleteWorktrees, "delete-worktrees", false, "Also remove .m/worktrees for this stack")

	return cmd
}

func removeStackByIndex(stacks []state.Stack, index int) ([]state.Stack, string) {
	removedName := stacks[index].Name
	return append(stacks[:index], stacks[index+1:]...), removedName
}

func stackHasStartedStages(stack *state.Stack) bool {
	for _, stage := range stack.Stages {
		if strings.TrimSpace(stage.Branch) != "" {
			return true
		}
	}

	return false
}

func newStackSyncCmd() *cobra.Command {
	var noPrune bool

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Prune merged stages, remove local resources, and rebase remaining stages",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStackSync(cmd, noPrune)
		},
	}

	cmd.Flags().BoolVar(&noPrune, "no-prune", false, "Keep merged stages in state and only rebase started branches")

	return cmd
}

func runStackSync(cmd *cobra.Command, noPrune bool) error {
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

	pruneMerged := !noPrune
	if pruneMerged {
		if _, err := exec.LookPath("gh"); err != nil {
			return fmt.Errorf("gh CLI is required for stack sync prune mode; rerun with --no-prune to skip merged-stage pruning")
		}
	}

	stageInfos := buildStageSyncInfos(stack, repoInfo.DefaultBranch)
	mergedByBranch := map[string]bool{}
	if pruneMerged {
		for _, info := range stageInfos {
			merged, err := stagePRMerged(repo.rootPath, info.Branch)
			if err != nil {
				return err
			}
			mergedByBranch[info.Branch] = merged
		}
	}

	mutated := false
	prunedCount := 0

	parentBranch := repoInfo.DefaultBranch
	rebasedCount := 0

	for _, info := range stageInfos {
		if pruneMerged && mergedByBranch[info.Branch] {
			continue
		}

		stage := &stack.Stages[info.Index]
		branch := info.Branch

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
			outSuccess(cmd.OutOrStdout(), "Created worktree: %s", worktree)
		} else if err != nil {
			return err
		}

		rebaseArgs := []string{"rebase", parentBranch}
		rebaseMode := "plain"
		if shouldTransplantRebase(info, pruneMerged, mergedByBranch, parentBranch) {
			upstream, err := resolveTransplantUpstream(repo.rootPath, branch, info.OldParent)
			if err != nil {
				return err
			}
			if strings.TrimSpace(upstream) == "" {
				outWarn(cmd.OutOrStdout(), "Could not resolve upstream %s for %s; falling back to plain rebase onto %s", info.OldParent, branch, parentBranch)
			} else {
				rebaseArgs = []string{"rebase", "--onto", parentBranch, upstream}
				rebaseMode = "transplant"
				outStyled(cmd.OutOrStdout(), ansiBlue, "🔄", "Transplant rebasing %s onto %s (from %s)", branch, parentBranch, upstream)
			}
		} else {
			outStyled(cmd.OutOrStdout(), ansiBlue, "🔄", "Rebasing %s onto %s", branch, parentBranch)
		}

		if err := runRebaseWithAbort(func(dir string, args ...string) (string, error) {
			return gitx.Run(dir, args...)
		}, worktree, rebaseArgs, stage.ID, branch, rebaseMode); err != nil {
			return err
		}

		stage.Branch = branch
		stage.Worktree = worktree
		stage.Parent = parentBranch
		parentBranch = branch
		rebasedCount++
		mutated = true
	}

	if pruneMerged {
		removed, changed, err := pruneMergedStages(stack,
			func(branch string) (bool, error) {
				return mergedByBranch[branch], nil
			},
			func(stage state.Stage, branch string) error {
				if err := removeStageWorktree(repo.rootPath, stage.Worktree); err != nil {
					return err
				}
				if err := removeLocalStageBranch(repo.rootPath, branch); err != nil {
					return err
				}
				return nil
			},
		)
		if err != nil {
			return err
		}
		prunedCount = removed
		mutated = mutated || changed
	}

	if mutated {
		if err := state.SaveStacks(repo.rootPath, stacksFile); err != nil {
			return err
		}
	}

	if pruneMerged {
		if prunedCount == 0 {
			outInfo(cmd.OutOrStdout(), "No merged stage PRs found to prune")
		} else {
			outSuccess(cmd.OutOrStdout(), "Pruned %d merged stage(s)", prunedCount)
		}
	}

	if rebasedCount == 0 {
		outInfo(cmd.OutOrStdout(), "Nothing to rebase (no started stage branches)")
		return nil
	}

	outSuccess(cmd.OutOrStdout(), "Synced stack: rebased %d stage branch(es)", rebasedCount)
	return nil
}

type stackSyncStageInfo struct {
	Index        int
	Branch       string
	OldParent    string
	ParentMerged bool
}

func buildStageSyncInfos(stack *state.Stack, defaultBranch string) []stackSyncStageInfo {
	infos := make([]stackSyncStageInfo, 0, len(stack.Stages))
	for idx := range stack.Stages {
		branch := stageBranchFor(stack, idx)
		oldParent := strings.TrimSpace(defaultBranch)
		parentMerged := false
		if idx > 0 {
			oldParent = stageBranchFor(stack, idx-1)
			parentMerged = true
		}

		infos = append(infos, stackSyncStageInfo{
			Index:        idx,
			Branch:       branch,
			OldParent:    oldParent,
			ParentMerged: parentMerged,
		})
	}

	return infos
}

func shouldTransplantRebase(info stackSyncStageInfo, pruneMerged bool, mergedByBranch map[string]bool, currentParent string) bool {
	if !pruneMerged || !info.ParentMerged {
		return false
	}
	if strings.TrimSpace(info.OldParent) == "" {
		return false
	}
	if strings.TrimSpace(currentParent) == strings.TrimSpace(info.OldParent) {
		return false
	}

	return mergedByBranch[info.OldParent]
}

func runRebaseWithAbort(
	runGit func(dir string, args ...string) (string, error),
	worktree string,
	rebaseArgs []string,
	stageID string,
	branch string,
	mode string,
) error {
	if _, err := runGit(worktree, rebaseArgs...); err != nil {
		if _, abortErr := runGit(worktree, "rebase", "--abort"); abortErr != nil {
			return fmt.Errorf("rebase failed for stage %q (%s) [%s]: %w\nRebase abort also failed in %s: %v\nResolve manually in %s (`git rebase --abort`), then rerun `m stack sync`", stageID, branch, mode, err, worktree, abortErr, worktree)
		}
		return fmt.Errorf("rebase failed for stage %q (%s) [%s]: %w\nAborted rebase in %s; resolve issues and rerun `m stack sync`", stageID, branch, mode, err, worktree)
	}

	return nil
}

func resolveTransplantUpstream(repoRoot, stageBranch, oldParent string) (string, error) {
	parent := strings.TrimSpace(oldParent)
	if parent == "" {
		return "", nil
	}

	candidates := []string{parent, "origin/" + parent}
	for _, candidate := range candidates {
		exists, err := gitCommitRefExists(repoRoot, candidate)
		if err != nil {
			return "", err
		}
		if !exists {
			continue
		}

		isAncestor, err := gitIsAncestor(repoRoot, candidate, stageBranch)
		if err != nil {
			return "", err
		}
		if isAncestor {
			return candidate, nil
		}

		mergeBase, err := gitMergeBase(repoRoot, candidate, stageBranch)
		if err != nil {
			continue
		}
		if strings.TrimSpace(mergeBase) != "" {
			return mergeBase, nil
		}
	}

	return "", nil
}

func gitCommitRefExists(repoRoot, ref string) (bool, error) {
	if strings.TrimSpace(ref) == "" {
		return false, nil
	}
	_, err := gitx.Run(repoRoot, "rev-parse", "--verify", "--quiet", ref+"^{commit}")
	if err != nil {
		return false, nil
	}

	return true, nil
}

func gitIsAncestor(repoRoot, ancestor, descendant string) (bool, error) {
	if strings.TrimSpace(ancestor) == "" || strings.TrimSpace(descendant) == "" {
		return false, nil
	}
	_, err := gitx.Run(repoRoot, "merge-base", "--is-ancestor", ancestor, descendant)
	if err != nil {
		return false, nil
	}

	return true, nil
}

func gitMergeBase(repoRoot, a, b string) (string, error) {
	if strings.TrimSpace(a) == "" || strings.TrimSpace(b) == "" {
		return "", nil
	}

	return gitx.Run(repoRoot, "merge-base", a, b)
}

func pruneMergedStages(
	stack *state.Stack,
	isMerged func(branch string) (bool, error),
	cleanup func(stage state.Stage, branch string) error,
) (int, bool, error) {
	if stack == nil {
		return 0, false, fmt.Errorf("stack is required")
	}

	remaining := make([]state.Stage, 0, len(stack.Stages))
	prunedCount := 0
	currentStage := strings.TrimSpace(stack.CurrentStage)
	preferredCurrentIndex := -1
	currentStillExists := false

	for i := range stack.Stages {
		stage := stack.Stages[i]
		branch := strings.TrimSpace(stage.Branch)
		if branch == "" {
			branch = stageBranchName(stack.Name, i, stage.ID)
		}

		merged, err := isMerged(branch)
		if err != nil {
			return 0, false, err
		}

		if merged {
			if err := cleanup(stage, branch); err != nil {
				return 0, false, err
			}
			if currentStage != "" && stage.ID == currentStage {
				preferredCurrentIndex = len(remaining)
			}
			prunedCount++
			continue
		}

		if currentStage != "" && stage.ID == currentStage {
			currentStillExists = true
		}

		remaining = append(remaining, stage)
	}

	if prunedCount == 0 {
		return 0, false, nil
	}

	stack.Stages = remaining
	if currentStillExists {
		return prunedCount, true, nil
	}

	if len(remaining) == 0 {
		stack.CurrentStage = ""
		return prunedCount, true, nil
	}

	if preferredCurrentIndex >= 0 && preferredCurrentIndex < len(remaining) {
		stack.CurrentStage = remaining[preferredCurrentIndex].ID
		return prunedCount, true, nil
	}

	stack.CurrentStage = remaining[len(remaining)-1].ID
	return prunedCount, true, nil
}

func stagePRMerged(repoRoot, headBranch string) (bool, error) {
	out, err := runGH(repoRoot, "pr", "list", "--state", "merged", "--head", headBranch, "--json", "number", "--limit", "1")
	if err != nil {
		return false, err
	}

	var prs []struct {
		Number int `json:"number"`
	}
	if err := json.Unmarshal([]byte(out), &prs); err != nil {
		return false, fmt.Errorf("parse gh pr list output: %w", err)
	}

	return len(prs) > 0, nil
}

func removeStageWorktree(repoRoot, worktree string) error {
	trimmed := strings.TrimSpace(worktree)
	if trimmed == "" {
		return nil
	}

	if _, err := os.Stat(trimmed); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}

	_, err := gitx.Run(repoRoot, "worktree", "remove", "--force", trimmed)
	return err
}

func removeLocalStageBranch(repoRoot, branch string) error {
	trimmed := strings.TrimSpace(branch)
	if trimmed == "" {
		return nil
	}

	if !gitx.BranchExists(repoRoot, trimmed) {
		return nil
	}

	_, err := gitx.Run(repoRoot, "branch", "-D", trimmed)
	return err
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
				outInfo(cmd.OutOrStdout(), "Nothing to push (no started stage branches)")
				return nil
			}

			for _, stageIndex := range startedStageIndexes {
				stage := stack.Stages[stageIndex]
				outAction(cmd.OutOrStdout(), "%s PR", stage.ID)
				if err := pushStageAndEnsurePROpts(cmd, repo.rootPath, stack, stageIndex, true, "  "); err != nil {
					return err
				}
			}

			if err := syncStackPRDescriptions(cmd, repo.rootPath, stack, startedStageIndexes, "  "); err != nil {
				return err
			}

			outAction(cmd.OutOrStdout(), "Pushed %d stage branch(es) with --force-with-lease", len(startedStageIndexes))
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
		Short: "Create a stack, optionally from a markdown plan file",
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
				outSuccess(cmd.OutOrStdout(), "Created stack %q (no plan attached yet)", stackName)
			} else {
				outSuccess(cmd.OutOrStdout(), "Created stack %q with %d stage(s)", stackName, len(stages))
			}
			outCurrent(cmd.OutOrStdout(), "Current stack: %s", stackName)
			return nil
		},
	}

	cmd.Flags().StringVar(&planFile, "plan-file", "", "Markdown plan file path")

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

			outSuccess(cmd.OutOrStdout(), "Attached plan to stack %q with %d stage(s)", stack.Name, len(parsedStages))
			outInfo(cmd.OutOrStdout(), "Plan file: %s", absolutePlanFile)
			return nil
		},
	}
}

func parseStagesFromPlanFile(planFile string) (string, []state.Stage, error) {
	absolutePlanFile, err := filepath.Abs(strings.TrimSpace(planFile))
	if err != nil {
		return "", nil, err
	}
	if ext := strings.ToLower(filepath.Ext(absolutePlanFile)); ext != ".md" {
		return "", nil, fmt.Errorf("plan file must use .md extension (markdown with YAML frontmatter)")
	}

	parsedPlan, err := plan.ParseFile(absolutePlanFile)
	if err != nil {
		return "", nil, err
	}

	stages := make([]state.Stage, 0, len(parsedPlan.Stages))
	for _, stage := range parsedPlan.Stages {
		risks := make([]state.StageRisk, 0, len(stage.Risks))
		for _, risk := range stage.Risks {
			risks = append(risks, state.StageRisk{
				Risk:       risk.Risk,
				Mitigation: risk.Mitigation,
			})
		}

		stages = append(stages, state.Stage{
			ID:             stage.ID,
			Title:          stage.Title,
			Outcome:        stage.Outcome,
			Implementation: append([]string(nil), stage.Implementation...),
			Validation:     append([]string(nil), stage.Validation...),
			Risks:          risks,
			Context:        stage.Context,
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
				outInfo(cmd.OutOrStdout(), "No stacks found. Create one with `m stack new <name>`")
				return nil
			}

			for _, stack := range stacksFile.Stacks {
				if stack.Name == config.CurrentStack {
					outCurrent(cmd.OutOrStdout(), "%s  ·  %d stage(s)", stack.Name, len(stack.Stages))
					continue
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  %s  ·  %d stage(s)\n", stack.Name, len(stack.Stages))
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

			outCurrent(cmd.OutOrStdout(), "Current stack: %s", stackName)
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

			stacksFile, err := state.LoadStacks(repo.rootPath)
			if err != nil {
				return err
			}

			workspaceStack, _ := state.CurrentWorkspaceStackStage(stacksFile, repo.worktreePath)
			if workspaceStack != "" {
				outCurrent(cmd.OutOrStdout(), "Current stack: %s", workspaceStack)
				return nil
			}

			if state.IsLinkedWorktree(repo.worktreePath, repo.rootPath) {
				return nil
			}

			if strings.TrimSpace(config.CurrentStack) == "" {
				return nil
			}

			outCurrent(cmd.OutOrStdout(), "Current stack: %s", config.CurrentStack)
			return nil
		},
	}
}
