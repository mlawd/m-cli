package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type AgentConfig struct {
	Agent string `json:"agent"`
	Model string `json:"model,omitempty"`
}

// AgentEntry supports both string shorthand ("build") and full AgentConfig objects.
type AgentEntry struct {
	AgentConfig
}

func (e *AgentEntry) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		e.Agent = s
		return nil
	}

	var ac AgentConfig
	if err := json.Unmarshal(data, &ac); err != nil {
		return fmt.Errorf("agent entry must be a string or {agent, model} object: %w", err)
	}
	e.AgentConfig = ac
	return nil
}

func (e AgentEntry) MarshalJSON() ([]byte, error) {
	if e.Model == "" {
		return json.Marshal(e.Agent)
	}
	return json.Marshal(e.AgentConfig)
}

var validHarnesses = map[string]struct{}{
	"opencode": {},
	"claude":   {},
}

type Config struct {
	AgentHarness string                `json:"agent_harness"`
	Agents       map[string]AgentEntry `json:"agents"`
}

func DefaultConfig() *Config {
	return &Config{
		AgentHarness: "opencode",
		Agents: map[string]AgentEntry{
			"build":  {AgentConfig{Agent: "build"}},
			"review": {AgentConfig{Agent: "review"}},
		},
	}
}

func ConfigPath() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "m", "config.json")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".config", "m", "config.json")
	}
	return filepath.Join(home, ".config", "m", "config.json")
}

func Load() (*Config, error) {
	cfg := DefaultConfig()
	path := ConfigPath()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}

	var fileCfg Config
	if err := json.Unmarshal(data, &fileCfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}

	if strings.TrimSpace(fileCfg.AgentHarness) != "" {
		cfg.AgentHarness = fileCfg.AgentHarness
	}
	if fileCfg.Agents != nil {
		for k, v := range fileCfg.Agents {
			cfg.Agents[k] = v
		}
	}

	return cfg, nil
}

func Save(cfg *Config) error {
	path := ConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	return os.WriteFile(path, data, 0o644)
}

func IsValidHarness(name string) bool {
	_, ok := validHarnesses[strings.TrimSpace(strings.ToLower(name))]
	return ok
}

func ValidateConfig(cfg *Config) error {
	if !IsValidHarness(cfg.AgentHarness) {
		return fmt.Errorf("invalid agent_harness %q; valid values: opencode, claude", cfg.AgentHarness)
	}
	for key, entry := range cfg.Agents {
		if strings.TrimSpace(entry.Agent) == "" {
			return fmt.Errorf("agent entry %q has empty agent name", key)
		}
	}
	return nil
}
