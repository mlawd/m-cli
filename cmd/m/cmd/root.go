package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func NewRootCmd(version string) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "m",
		Short: "A local orchestration CLI for stacked workflows",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Hello from m. Try: m help")
		},
	}

	rootCmd.AddCommand(
		newInitCmd(),
		newStackRootCmd(),
		newStageRootCmd(),
		newVersionCmd(version),
	)

	return rootCmd
}
