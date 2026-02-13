package cmd

import (
	"github.com/spf13/cobra"
)

func NewRootCmd(version string) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:           "m",
		Short:         "A local orchestration CLI for stacked workflows",
		SilenceUsage:  true,
		SilenceErrors: true,
		Run: func(cmd *cobra.Command, args []string) {
			outInfo(cmd.OutOrStdout(), "Welcome to m. Run `m --help` to get started.")
		},
	}

	rootCmd.AddCommand(
		newInitCmd(),
		newStackRootCmd(),
		newStageRootCmd(),
		newWorktreeRootCmd(),
		newMCPRootCmd(version),
		newVersionCmd(version),
	)

	return rootCmd
}
