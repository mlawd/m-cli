package cmd

import (
	"fmt"
	"os"

	"github.com/mlawd/m-cli/internal/mcp"
	"github.com/spf13/cobra"
)

func newMCPRootCmd(version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "MCP server integration for m",
	}

	cmd.AddCommand(newMCPServeCmd(version))

	return cmd
}

func newMCPServeCmd(version string) *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Run an MCP stdio server with m context",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := mcp.ServeStdio(cmd.Context(), os.Stdin, os.Stdout, version); err != nil {
				return fmt.Errorf("run MCP server: %w", err)
			}
			return nil
		},
	}
}
