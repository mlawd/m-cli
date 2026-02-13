package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mlawd/m-cli/internal/agent"
	"github.com/mlawd/m-cli/internal/gitx"
	"github.com/mlawd/m-cli/internal/state"
	"github.com/spf13/cobra"
)

func newWorktreeRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "worktree",
		Short: "Manage ad-hoc worktrees without stage plans",
	}

	cmd.AddCommand(
		newWorktreeOpenCmd(),
		newWorktreeListCmd(),
		newWorktreePruneCmd(),
	)

	return cmd
}

func newWorktreeListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List linked git worktrees with m metadata",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := discoverRepoContext()
			if err != nil {
				return err
			}

			worktrees, err := listGitWorktrees(repo.rootPath)
			if err != nil {
				return err
			}

			stacksFile, err := state.LoadStacks(repo.rootPath)
			if err != nil {
				return err
			}

			stageByWorktree := map[string]string{}
			for _, stack := range stacksFile.Stacks {
				for _, stage := range stack.Stages {
					normalized := normalizeCmdPath(stage.Worktree)
					if normalized == "" {
						continue
					}
					stageByWorktree[normalized] = fmt.Sprintf("%s/%s", stack.Name, stage.ID)
				}
			}

			managedRoot := filepath.Join(state.Dir(repo.rootPath), "worktrees")
			currentWorktree := normalizeCmdPath(repo.worktreePath)

			for _, wt := range worktrees {
				entryPath := normalizeCmdPath(wt.Path)
				currentMarker := " "
				if entryPath == currentWorktree {
					currentMarker = "*"
				}

				kind := "external"
				if isWithinDir(entryPath, managedRoot) {
					kind = "managed"
				}

				branch := wt.Branch
				if wt.Detached || strings.TrimSpace(branch) == "" {
					branch = "(detached)"
				}

				line := fmt.Sprintf("%s %s  [%s]  %s", currentMarker, wt.Path, kind, branch)
				if owner := strings.TrimSpace(stageByWorktree[entryPath]); owner != "" {
					line = fmt.Sprintf("%s  ·  %s", line, owner)
				}

				fmt.Fprintln(cmd.OutOrStdout(), line)
			}

			outInfo(cmd.OutOrStdout(), "Total worktrees: %d", len(worktrees))
			return nil
		},
	}
}

func newWorktreePruneCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "prune",
		Short: "Prune stale git worktrees and remove orphan managed directories",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := discoverRepoContext()
			if err != nil {
				return err
			}

			if _, err := gitx.Run(repo.rootPath, "worktree", "prune"); err != nil {
				return err
			}

			worktrees, err := listGitWorktrees(repo.rootPath)
			if err != nil {
				return err
			}

			active := map[string]struct{}{}
			for _, wt := range worktrees {
				normalized := normalizeCmdPath(wt.Path)
				if normalized == "" {
					continue
				}
				active[normalized] = struct{}{}
			}

			removedDirs, err := removeOrphanManagedWorktrees(repo.rootPath, active)
			if err != nil {
				return err
			}

			stacksFile, err := state.LoadStacks(repo.rootPath)
			if err != nil {
				return err
			}

			clearedRefs := clearMissingStageWorktrees(stacksFile, func(path string) bool {
				normalized := normalizeCmdPath(path)
				if normalized == "" {
					return false
				}

				if _, ok := active[normalized]; ok {
					return true
				}

				_, err := os.Stat(normalized)
				return err == nil
			})

			if clearedRefs > 0 {
				if err := state.SaveStacks(repo.rootPath, stacksFile); err != nil {
					return err
				}
			}

			outSuccess(cmd.OutOrStdout(), "Pruned git worktrees")
			outInfo(cmd.OutOrStdout(), "Removed orphan managed directories: %d", removedDirs)
			outInfo(cmd.OutOrStdout(), "Cleared stale stage worktree references: %d", clearedRefs)
			return nil
		},
	}
}

func newWorktreeOpenCmd() *cobra.Command {
	var baseBranch string
	var worktreePath string
	var noOpen bool

	cmd := &cobra.Command{
		Use:   "open <branch>",
		Short: "Create or open a branch worktree without plan/stage",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := discoverRepoContext()
			if err != nil {
				return err
			}

			branch, err := normalizeBranchName(args[0])
			if err != nil {
				return err
			}

			if err := state.EnsureInitialized(repo.rootPath); err != nil {
				return err
			}

			repoInfo, err := gitx.DiscoverRepo(repo.rootPath)
			if err != nil {
				return err
			}

			fromBranch := resolveBaseBranch(baseBranch, repoInfo.DefaultBranch)
			if !gitx.BranchExists(repo.rootPath, branch) {
				if err := gitx.CreateBranch(repo.rootPath, branch, fromBranch); err != nil {
					return err
				}
				outSuccess(cmd.OutOrStdout(), "Created branch %s from %s", branch, fromBranch)
			} else {
				outReuse(cmd.OutOrStdout(), "Reusing branch %s", branch)
			}

			resolvedPath, err := resolveWorktreePath(repo.rootPath, branch, worktreePath)
			if err != nil {
				return err
			}

			if _, err := os.Stat(resolvedPath); os.IsNotExist(err) {
				if err := os.MkdirAll(filepath.Dir(resolvedPath), 0o755); err != nil {
					return err
				}
				if err := gitx.AddWorktree(repo.rootPath, resolvedPath, branch); err != nil {
					return err
				}
				outSuccess(cmd.OutOrStdout(), "Created worktree: %s", resolvedPath)
			} else if err != nil {
				return err
			} else {
				outReuse(cmd.OutOrStdout(), "Reusing worktree: %s", resolvedPath)
			}

			if noOpen {
				return nil
			}

			return agent.StartOpenCode(resolvedPath)
		},
	}

	cmd.Flags().StringVar(&baseBranch, "base", "", "Base branch when creating a new branch (defaults to repo default branch)")
	cmd.Flags().StringVar(&worktreePath, "path", "", "Custom worktree path (defaults to .m/worktrees/<branch>)")
	cmd.Flags().BoolVar(&noOpen, "no-open", false, "Skip launching opencode")

	return cmd
}

func normalizeBranchName(raw string) (string, error) {
	branch := strings.TrimSpace(raw)
	if branch == "" {
		return "", fmt.Errorf("branch is required")
	}

	return branch, nil
}

func resolveBaseBranch(override, defaultBranch string) string {
	base := strings.TrimSpace(override)
	if base != "" {
		return base
	}

	return strings.TrimSpace(defaultBranch)
}

func resolveWorktreePath(repoRoot, branch, override string) (string, error) {
	trimmed := strings.TrimSpace(override)
	if trimmed != "" {
		abs, err := filepath.Abs(trimmed)
		if err != nil {
			return "", err
		}
		return filepath.Clean(abs), nil
	}

	return filepath.Join(state.Dir(repoRoot), "worktrees", filepath.FromSlash(branch)), nil
}

type gitWorktree struct {
	Path     string
	Branch   string
	Detached bool
}

func listGitWorktrees(repoRoot string) ([]gitWorktree, error) {
	out, err := gitx.Run(repoRoot, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}

	worktrees := parseGitWorktreePorcelain(out)
	sort.Slice(worktrees, func(i, j int) bool {
		return strings.TrimSpace(worktrees[i].Path) < strings.TrimSpace(worktrees[j].Path)
	})

	return worktrees, nil
}

func parseGitWorktreePorcelain(out string) []gitWorktree {
	lines := strings.Split(out, "\n")
	items := []gitWorktree{}
	current := gitWorktree{}
	hasCurrent := false

	flush := func() {
		if !hasCurrent || strings.TrimSpace(current.Path) == "" {
			return
		}
		items = append(items, current)
		current = gitWorktree{}
		hasCurrent = false
	}

	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			flush()
			continue
		}

		switch {
		case strings.HasPrefix(line, "worktree "):
			flush()
			current.Path = strings.TrimSpace(strings.TrimPrefix(line, "worktree "))
			hasCurrent = true

		case strings.HasPrefix(line, "branch "):
			branchRef := strings.TrimSpace(strings.TrimPrefix(line, "branch "))
			current.Branch = strings.TrimPrefix(branchRef, "refs/heads/")

		case line == "detached":
			current.Detached = true
		}
	}

	flush()
	return items
}

func removeOrphanManagedWorktrees(repoRoot string, active map[string]struct{}) (int, error) {
	managedRoot := filepath.Join(state.Dir(repoRoot), "worktrees")
	if _, err := os.Stat(managedRoot); os.IsNotExist(err) {
		return 0, nil
	} else if err != nil {
		return 0, err
	}

	managedWorktrees, err := collectManagedWorktreeRoots(managedRoot)
	if err != nil {
		return 0, err
	}

	orphans := orphanManagedWorktrees(managedWorktrees, active)
	removed := 0
	for _, orphan := range orphans {
		if err := os.RemoveAll(orphan); err != nil {
			return removed, err
		}
		removed++
	}

	if err := removeEmptySubdirs(managedRoot); err != nil {
		return removed, err
	}

	return removed, nil
}

func collectManagedWorktreeRoots(root string) ([]string, error) {
	collected := []string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if d.Name() != ".git" {
			return nil
		}

		collected = append(collected, filepath.Dir(path))
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Strings(collected)
	return collected, nil
}

func orphanManagedWorktrees(managedWorktrees []string, active map[string]struct{}) []string {
	orphans := make([]string, 0, len(managedWorktrees))
	for _, path := range managedWorktrees {
		normalized := normalizeCmdPath(path)
		if normalized == "" {
			continue
		}
		if _, ok := active[normalized]; ok {
			continue
		}
		orphans = append(orphans, normalized)
	}

	return orphans
}

func clearMissingStageWorktrees(stacksFile *state.Stacks, existsFn func(path string) bool) int {
	if stacksFile == nil {
		return 0
	}

	cleared := 0
	for stackIdx := range stacksFile.Stacks {
		for stageIdx := range stacksFile.Stacks[stackIdx].Stages {
			worktree := strings.TrimSpace(stacksFile.Stacks[stackIdx].Stages[stageIdx].Worktree)
			if worktree == "" {
				continue
			}
			if existsFn(worktree) {
				continue
			}

			stacksFile.Stacks[stackIdx].Stages[stageIdx].Worktree = ""
			cleared++
		}
	}

	return cleared
}

func removeEmptySubdirs(root string) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		child := filepath.Join(root, entry.Name())
		if err := removeEmptySubdirs(child); err != nil {
			return err
		}
		if err := os.Remove(child); err != nil && !os.IsExist(err) && !os.IsNotExist(err) {
			if !isDirNotEmptyError(err) {
				return err
			}
		}
	}

	return nil
}

func isDirNotEmptyError(err error) bool {
	if err == nil {
		return false
	}

	message := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(message, "directory not empty")
}

func normalizeCmdPath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}

	abs, err := filepath.Abs(trimmed)
	if err == nil {
		trimmed = abs
	}

	if resolved, err := filepath.EvalSymlinks(trimmed); err == nil {
		trimmed = resolved
	}

	return filepath.Clean(trimmed)
}

func isWithinDir(path, root string) bool {
	normalizedPath := normalizeCmdPath(path)
	normalizedRoot := normalizeCmdPath(root)
	if normalizedPath == "" || normalizedRoot == "" {
		return false
	}

	rel, err := filepath.Rel(normalizedRoot, normalizedPath)
	if err != nil {
		return false
	}

	if rel == "." {
		return true
	}

	return !strings.HasPrefix(rel, "..")
}
