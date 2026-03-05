package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// AgentConfig holds a named agent reference plus an optional model override.
type AgentConfig struct {
	Agent string `json:"agent"`
	Model string `json:"model,omitempty"`
}

// AgentEntry unmarshals either a plain string like "build" or an object like
// {"agent":"build","model":"gpt-4o"} into an AgentConfig.
type AgentEntry struct {
	AgentConfig
}

func (e *AgentEntry) UnmarshalJSON(data []byte) error {
	// Try plain string first.
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		e.AgentConfig = AgentConfig{Agent: strings.TrimSpace(s)}
		return nil
	}
	// Fall back to full object.
	var cfg AgentConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return err
	}
	e.AgentConfig = cfg
	return nil
}

func (e AgentEntry) MarshalJSON() ([]byte, error) {
	if e.Model == "" {
		return json.Marshal(e.Agent)
	}
	return json.Marshal(e.AgentConfig)
}

// Config is the global m configuration stored in ~/.config/m/config.json.
type Config struct {
	AgentHarness string                `json:"agent_harness"`
	Agents       map[string]AgentEntry `json:"agents"`
}

var validHarnesses = map[string]struct{}{
	"opencode": {},
	"claude":   {},
}

// Validate checks that Config fields contain acceptable values.
func (c *Config) Validate() error {
	harness := strings.TrimSpace(c.AgentHarness)
	if harness != "" {
		if _, ok := validHarnesses[harness]; !ok {
			return fmt.Errorf("invalid agent_harness %q; valid values: opencode, claude", harness)
		}
	}
	for key, entry := range c.Agents {
		if strings.TrimSpace(entry.Agent) == "" {
			return fmt.Errorf("agents[%q]: agent name must not be empty", key)
		}
	}
	return nil
}

// ConfigPath returns the path to the global m config file.
func ConfigPath() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(configDir, "m", "config.json")
}

// defaults returns the built-in config applied when no file exists.
func defaults() *Config {
	return &Config{
		AgentHarness: "opencode",
		Agents: map[string]AgentEntry{
			"build":  {AgentConfig: AgentConfig{Agent: "build"}},
			"review": {AgentConfig: AgentConfig{Agent: "review"}},
		},
	}
}

// Load reads the global config file, merges with defaults, and returns the
// resolved Config. A missing config file is not an error.
func Load() (*Config, error) {
	path := ConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return defaults(), nil
		}
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}

	// Merge with defaults for missing fields.
	d := defaults()
	if strings.TrimSpace(cfg.AgentHarness) == "" {
		cfg.AgentHarness = d.AgentHarness
	}
	if cfg.Agents == nil {
		cfg.Agents = d.Agents
	} else {
		for k, v := range d.Agents {
			if _, exists := cfg.Agents[k]; !exists {
				cfg.Agents[k] = v
			}
		}
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// Save writes the config to the global config file atomically.
func Save(cfg *Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	path := ConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
