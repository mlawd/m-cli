package cmd

import (
	"fmt"
	"os"
	"path/filepath"
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

	cmd.AddCommand(newWorktreeOpenCmd())

	return cmd
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
