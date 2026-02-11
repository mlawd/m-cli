package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func NewRootCmd(version string) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "m",
		Short: "A tiny starter CLI",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Hello from m. Try: m help")
		},
	}

	rootCmd.AddCommand(
		newCloneCmd(),
		newNewCmd(),
		newStackCmd(),
		newGreetCmd(),
		newTimeCmd(),
		newVersionCmd(version),
	)

	return rootCmd
}
