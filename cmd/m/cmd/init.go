package cmd

import (
	"github.com/mlawd/m-cli/internal/localignore"
	"github.com/mlawd/m-cli/internal/state"
	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize local orchestration state for this repo",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := discoverRepoContext()
			if err != nil {
				return err
			}

			if err := state.EnsureInitialized(repo.rootPath); err != nil {
				return err
			}

			if err := localignore.EnsurePattern(repo.common, ".m/"); err != nil {
				return err
			}

			outSuccess(cmd.OutOrStdout(), "Initialized m state at %s", state.Dir(repo.rootPath))
			return nil
		},
	}
}
