package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func newPromptRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "prompt",
		Short: "Print built-in prompts",
	}

	cmd.AddCommand(newPromptDefaultCmd())

	return cmd
}

func newPromptDefaultCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "default",
		Short: "Print the default MCP prompt",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := discoverRepoContext()
			if err != nil {
				return err
			}

			prompt, err := readDefaultPrompt(repo.rootPath)
			if err != nil {
				return err
			}

			fmt.Fprint(cmd.OutOrStdout(), prompt)
			return nil
		},
	}
}

func readDefaultPrompt(repoRoot string) (string, error) {
	promptPath := filepath.Join(repoRoot, "MCP_PROMPT.md")
	data, err := os.ReadFile(promptPath)
	if err != nil {
		return "", fmt.Errorf("read default prompt: %w", err)
	}

	return string(data), nil
}
