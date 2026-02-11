package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mlawd/m-cli/internal/agent"
	"github.com/mlawd/m-cli/internal/gitx"
	"github.com/mlawd/m-cli/internal/paths"
	"github.com/mlawd/m-cli/internal/stacks"
	"github.com/spf13/cobra"
)

func newStackCmd() *cobra.Command {
	var noOpen bool
	var printPath bool

	cmd := &cobra.Command{
		Use:   "stack <stack> <part>",
		Short: "Create or reopen a part in a stack",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			stackName := strings.TrimSpace(args[0])
			partLabel := strings.TrimSpace(args[1])

			if err := paths.EnsureValidStackName(stackName); err != nil {
				return err
			}

			partSlug := stacks.SlugPart(partLabel)
			if partSlug == "" {
				return fmt.Errorf("part label produced empty slug")
			}

			repo, err := gitx.DiscoverRepo(".")
			if err != nil {
				return fmt.Errorf("discover repo: %w", err)
			}
			repoRoot := filepath.Dir(repo.CommonDir)

			statePath := stacks.StatePath(repo.CommonDir)
			state, err := stacks.Load(statePath)
			if err != nil {
				return err
			}

			stack, _ := stacks.FindStack(state, stackName)
			if stack == nil {
				return fmt.Errorf("stack %q not found; run: m new %s", stackName, stackName)
			}

			part := stacks.FindPartBySlug(stack, partSlug)
			if part == nil {
				index := stacks.NextIndex(stack)
				parentBranch := stack.BaseBranch
				if last := stacks.LastPart(stack); last != nil {
					parentBranch = last.Branch
				}

				branch := stacks.BuildBranch(stackName, index, partSlug)
				worktreePath := stacks.BuildWorktreePath(repoRoot, branch)

				if !gitx.BranchExists(repo.TopLevel, branch) {
					if err := gitx.CreateBranch(repo.TopLevel, branch, parentBranch); err != nil {
						return err
					}
				}

				if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
					if err := os.MkdirAll(filepath.Dir(worktreePath), 0o755); err != nil {
						return err
					}
					if err := gitx.AddWorktree(repo.TopLevel, worktreePath, branch); err != nil {
						return err
					}
				} else if err != nil {
					return err
				}

				newPart := stacks.NewPart(index, partLabel, partSlug, branch, parentBranch, worktreePath)
				stack.Parts = append(stack.Parts, newPart)
				if err := stacks.Save(statePath, state); err != nil {
					return err
				}

				part = &stack.Parts[len(stack.Parts)-1]
				fmt.Fprintf(cmd.OutOrStdout(), "Created %s (from %s)\n", part.Branch, part.ParentBranch)
			} else {
				if _, err := os.Stat(part.Worktree); os.IsNotExist(err) {
					if err := os.MkdirAll(filepath.Dir(part.Worktree), 0o755); err != nil {
						return err
					}
					if err := gitx.AddWorktree(repo.TopLevel, part.Worktree, part.Branch); err != nil {
						return err
					}
				} else if err != nil {
					return err
				}

				fmt.Fprintf(cmd.OutOrStdout(), "Reusing %s\n", part.Branch)
			}

			if printPath {
				fmt.Fprintln(cmd.OutOrStdout(), part.Worktree)
			}

			if noOpen {
				return nil
			}

			return agent.StartOpenCode(part.Worktree)
		},
	}

	cmd.Flags().BoolVar(&noOpen, "no-open", false, "Skip launching opencode")
	cmd.Flags().BoolVar(&printPath, "print-path", false, "Print resolved worktree path")

	return cmd
}
