package cmd

import (
	"fmt"
	"strings"

	"github.com/mlawd/m-cli/internal/state"
	"github.com/spf13/cobra"
)

func newStageRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stage",
		Short: "Manage stages in the current stack",
	}

	cmd.AddCommand(
		newStageListCmd(),
		newStageSelectCmd(),
		newStageCurrentCmd(),
	)

	return cmd
}

func newStageListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List stages for the current stack",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := discoverRepoContext()
			if err != nil {
				return err
			}

			config, stacksFile, err := loadState(repo)
			if err != nil {
				return err
			}

			if strings.TrimSpace(config.CurrentStack) == "" {
				return fmt.Errorf("no stack selected; run: m stack select <stack-name>")
			}

			stack, _ := state.FindStack(stacksFile, config.CurrentStack)
			if stack == nil {
				return fmt.Errorf("current stack %q not found", config.CurrentStack)
			}

			if len(stack.Stages) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No stages")
				return nil
			}

			for idx, stage := range stack.Stages {
				marker := " "
				if stage.ID == stack.CurrentStage {
					marker = "*"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s %d. %s - %s\n", marker, idx+1, stage.ID, stage.Title)
			}

			return nil
		},
	}
}

func newStageSelectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "select <stage>",
		Short: "Select a stage in the current stack",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			selectedStage := strings.TrimSpace(args[0])

			repo, err := discoverRepoContext()
			if err != nil {
				return err
			}

			config, stacksFile, err := loadState(repo)
			if err != nil {
				return err
			}

			if strings.TrimSpace(config.CurrentStack) == "" {
				return fmt.Errorf("no stack selected; run: m stack select <stack-name>")
			}

			stack, _ := state.FindStack(stacksFile, config.CurrentStack)
			if stack == nil {
				return fmt.Errorf("current stack %q not found", config.CurrentStack)
			}

			if stage, _ := state.FindStage(stack, selectedStage); stage == nil {
				return fmt.Errorf("stage %q not found in stack %q", selectedStage, stack.Name)
			}

			stack.CurrentStage = selectedStage
			if err := state.SaveStacks(repo.rootPath, stacksFile); err != nil {
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), selectedStage)
			return nil
		},
	}
}

func newStageCurrentCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "current",
		Short: "Print current stage for current stack",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := discoverRepoContext()
			if err != nil {
				return err
			}

			config, stacksFile, err := loadState(repo)
			if err != nil {
				return err
			}

			if strings.TrimSpace(config.CurrentStack) == "" {
				return nil
			}

			stack, _ := state.FindStack(stacksFile, config.CurrentStack)
			if stack == nil {
				return nil
			}

			if strings.TrimSpace(stack.CurrentStage) == "" {
				return nil
			}

			fmt.Fprintln(cmd.OutOrStdout(), stack.CurrentStage)
			return nil
		},
	}
}
