package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mlawd/m-cli/internal/paths"
	"github.com/mlawd/m-cli/internal/plan"
	"github.com/mlawd/m-cli/internal/state"
	"github.com/spf13/cobra"
)

func newStackRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stack",
		Short: "Manage stacks",
	}

	cmd.AddCommand(
		newStackNewCmd(),
		newStackListCmd(),
		newStackSelectCmd(),
		newStackCurrentCmd(),
	)

	return cmd
}

func newStackNewCmd() *cobra.Command {
	var planFile string

	cmd := &cobra.Command{
		Use:   "new <stack-name>",
		Short: "Create a stack from a YAML plan file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			stackName := strings.TrimSpace(args[0])
			if err := paths.EnsureValidStackName(stackName); err != nil {
				return err
			}

			if strings.TrimSpace(planFile) == "" {
				return fmt.Errorf("--plan-file is required")
			}

			repo, err := discoverRepoContext()
			if err != nil {
				return err
			}

			config, stacksFile, err := loadState(repo)
			if err != nil {
				return err
			}

			if existing, _ := state.FindStack(stacksFile, stackName); existing != nil {
				return fmt.Errorf("stack %q already exists", stackName)
			}

			absolutePlanFile, err := filepath.Abs(planFile)
			if err != nil {
				return err
			}

			parsedPlan, err := plan.ParseFile(absolutePlanFile)
			if err != nil {
				return err
			}

			stages := make([]state.Stage, 0, len(parsedPlan.Stages))
			for _, stage := range parsedPlan.Stages {
				stages = append(stages, state.Stage{
					ID:          stage.ID,
					Title:       stage.Title,
					Description: stage.Description,
				})
			}

			stacksFile.Stacks = append(stacksFile.Stacks, state.NewStack(stackName, absolutePlanFile, stages))
			if err := state.SaveStacks(repo.rootPath, stacksFile); err != nil {
				return err
			}

			config.CurrentStack = stackName
			if err := state.SaveConfig(repo.rootPath, config); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Created stack %q with %d stages\n", stackName, len(stages))
			fmt.Fprintf(cmd.OutOrStdout(), "Current stack: %s\n", stackName)
			return nil
		},
	}

	cmd.Flags().StringVar(&planFile, "plan-file", "", "YAML plan file path")
	_ = cmd.MarkFlagRequired("plan-file")

	return cmd
}

func newStackListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List stacks",
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

			if len(stacksFile.Stacks) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No stacks")
				return nil
			}

			for _, stack := range stacksFile.Stacks {
				marker := " "
				if stack.Name == config.CurrentStack {
					marker = "*"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s %s (%d stages)\n", marker, stack.Name, len(stack.Stages))
			}

			return nil
		},
	}
}

func newStackSelectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "select <stack-name>",
		Short: "Set current stack context",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			stackName := strings.TrimSpace(args[0])

			repo, err := discoverRepoContext()
			if err != nil {
				return err
			}

			config, stacksFile, err := loadState(repo)
			if err != nil {
				return err
			}

			if existing, _ := state.FindStack(stacksFile, stackName); existing == nil {
				return fmt.Errorf("stack %q not found", stackName)
			}

			config.CurrentStack = stackName
			if err := state.SaveConfig(repo.rootPath, config); err != nil {
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), stackName)
			return nil
		},
	}
}

func newStackCurrentCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "current",
		Short: "Print current stack",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := discoverRepoContext()
			if err != nil {
				return err
			}

			config, err := state.LoadConfig(repo.rootPath)
			if err != nil {
				return err
			}

			if strings.TrimSpace(config.CurrentStack) == "" {
				return nil
			}

			fmt.Fprintln(cmd.OutOrStdout(), config.CurrentStack)
			return nil
		},
	}
}
