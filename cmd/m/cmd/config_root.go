package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	mcfg "github.com/mlawd/m-cli/internal/config"
	"github.com/spf13/cobra"
)

func newConfigRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "View and edit global m configuration",
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
		Short: "Print resolved global config as JSON",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := mcfg.Load()
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

			cfg, err := mcfg.Load()
			if err != nil {
				return err
			}

			if err := applyConfigSet(cfg, key, value); err != nil {
				return err
			}

			if err := mcfg.Save(cfg); err != nil {
				return err
			}

			outSuccess(cmd.OutOrStdout(), "Set %s = %s", key, value)
			outInfo(cmd.OutOrStdout(), "Config file: %s", mcfg.ConfigPath())
			return nil
		},
	}
}

func applyConfigSet(cfg *mcfg.Config, key, value string) error {
	switch key {
	case "agent_harness":
		cfg.AgentHarness = value
	default:
		// Handle agents.<name> key pattern.
		if strings.HasPrefix(key, "agents.") {
			agentKey := strings.TrimPrefix(key, "agents.")
			if strings.TrimSpace(agentKey) == "" {
				return fmt.Errorf("invalid agents key %q: agent name is empty", key)
			}
			if cfg.Agents == nil {
				cfg.Agents = make(map[string]mcfg.AgentEntry)
			}
			cfg.Agents[agentKey] = mcfg.AgentEntry{AgentConfig: mcfg.AgentConfig{Agent: value}}
			return nil
		}
		return fmt.Errorf("unknown config key %q; supported keys: agent_harness, agents.<name>", key)
	}
	return nil
}
