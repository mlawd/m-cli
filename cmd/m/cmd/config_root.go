package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mlawd/m-cli/internal/config"
	"github.com/spf13/cobra"
)

func newConfigRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "View and modify m global config",
	}

	cmd.AddCommand(
		newConfigShowCmd(),
		newConfigSetCmd(),
	)

	return cmd
}

func newConfigShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Print resolved config as JSON",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			data, err := json.MarshalIndent(cfg, "", "  ")
			if err != nil {
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return nil
		},
	}
}

func newConfigSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a config value (e.g. agent_harness opencode, agents.review review)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := strings.TrimSpace(args[0])
			value := strings.TrimSpace(args[1])

			cfg, err := config.Load()
			if err != nil {
				return err
			}

			switch {
			case key == "agent_harness":
				if !config.IsValidHarness(value) {
					return fmt.Errorf("invalid agent_harness %q; valid values: opencode, claude", value)
				}
				cfg.AgentHarness = value

			case strings.HasPrefix(key, "agents."):
				agentKey := strings.TrimPrefix(key, "agents.")
				if strings.TrimSpace(agentKey) == "" {
					return fmt.Errorf("agent key must be non-empty")
				}
				if strings.TrimSpace(value) == "" {
					return fmt.Errorf("agent value must be non-empty")
				}
				if cfg.Agents == nil {
					cfg.Agents = map[string]config.AgentEntry{}
				}
				cfg.Agents[agentKey] = config.AgentEntry{AgentConfig: config.AgentConfig{Agent: value}}

			default:
				return fmt.Errorf("unknown config key %q; supported: agent_harness, agents.<name>", key)
			}

			if err := config.ValidateConfig(cfg); err != nil {
				return err
			}

			if err := config.Save(cfg); err != nil {
				return err
			}

			outSuccess(cmd.OutOrStdout(), "Set %s = %s", key, value)
			outInfo(cmd.OutOrStdout(), "Config file: %s", config.ConfigPath())
			return nil
		},
	}
}
