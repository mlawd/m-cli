package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mlawd/m-cli/internal/gitx"
	"github.com/mlawd/m-cli/internal/paths"
	"github.com/spf13/cobra"
)

func newCloneCmd() *cobra.Command {
	var defaultBranchOverride string

	cmd := &cobra.Command{
		Use:   "clone <repo-url> [name]",
		Short: "Clone a repository into worktree-first layout",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			repoURL := args[0]
			targetName := ""
			if len(args) == 2 {
				targetName = args[1]
			} else {
				targetName = paths.RepoNameFromURL(repoURL)
			}

			targetDir, err := filepath.Abs(targetName)
			if err != nil {
				return err
			}

			if _, err := os.Stat(targetDir); err == nil {
				return fmt.Errorf("target path already exists: %s", targetDir)
			} else if !os.IsNotExist(err) {
				return err
			}

			if err := os.MkdirAll(targetDir, 0o755); err != nil {
				return err
			}

			gitDir := filepath.Join(targetDir, ".git")
			if _, err := gitx.Run("", "clone", "--bare", repoURL, gitDir); err != nil {
				return err
			}

			defaultBranch := strings.TrimSpace(defaultBranchOverride)
			if defaultBranch == "" {
				if out, err := gitx.Run(targetDir, "--git-dir", gitDir, "symbolic-ref", "--short", "refs/remotes/origin/HEAD"); err == nil {
					defaultBranch = strings.TrimPrefix(out, "origin/")
				} else {
					defaultBranch = "main"
				}
			}

			if defaultBranch == "" {
				defaultBranch = "main"
			}

			worktreePath := filepath.Join(targetDir, "worktrees", filepath.FromSlash(defaultBranch))
			if err := os.MkdirAll(filepath.Dir(worktreePath), 0o755); err != nil {
				return err
			}

			if gitx.BranchExists(targetDir, defaultBranch) {
				if err := gitx.AddWorktree(targetDir, worktreePath, defaultBranch); err != nil {
					return err
				}
			} else {
				remoteBranch := "origin/" + defaultBranch
				if err := gitx.AddWorktreeFromRemote(targetDir, worktreePath, defaultBranch, remoteBranch); err != nil {
					return err
				}
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Initialized: %s\n", targetDir)
			fmt.Fprintf(cmd.OutOrStdout(), "Main worktree: %s\n", worktreePath)
			fmt.Fprintf(cmd.OutOrStdout(), "Next: cd %s\n", worktreePath)
			return nil
		},
	}

	cmd.Flags().StringVar(&defaultBranchOverride, "default-branch", "", "Override detected default branch")

	return cmd
}
