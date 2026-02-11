package cmd

import (
	"fmt"
	"strings"

	"github.com/mlawd/m-cli/internal/gitx"
	"github.com/mlawd/m-cli/internal/paths"
	"github.com/mlawd/m-cli/internal/stacks"
	"github.com/spf13/cobra"
)

func newNewCmd() *cobra.Command {
	var baseOverride string

	cmd := &cobra.Command{
		Use:   "new <stack>",
		Short: "Create a new stack rooted on the default branch",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			stackName := strings.TrimSpace(args[0])
			if err := paths.EnsureValidStackName(stackName); err != nil {
				return err
			}

			repo, err := gitx.DiscoverRepo(".")
			if err != nil {
				return fmt.Errorf("discover repo: %w", err)
			}

			statePath := stacks.StatePath(repo.CommonDir)
			state, err := stacks.Load(statePath)
			if err != nil {
				return err
			}

			baseBranch := strings.TrimSpace(baseOverride)
			if baseBranch == "" {
				baseBranch = repo.DefaultBranch
			}

			created := stacks.EnsureStack(state, stackName, baseBranch)
			if err := stacks.Save(statePath, state); err != nil {
				return err
			}

			if created {
				fmt.Fprintf(cmd.OutOrStdout(), "Created stack %q (base: %s)\n", stackName, baseBranch)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Stack %q already exists\n", stackName)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "State: %s\n", statePath)
			return nil
		},
	}

	cmd.Flags().StringVar(&baseOverride, "base", "", "Override stack base branch")

	return cmd
}
