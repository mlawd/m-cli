package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestAgentEntryUnmarshalString(t *testing.T) {
	var e AgentEntry
	if err := json.Unmarshal([]byte(`"build"`), &e); err != nil {
		t.Fatal(err)
	}
	if e.Agent != "build" {
		t.Errorf("got %q, want build", e.Agent)
	}
	if e.Model != "" {
		t.Errorf("got model %q, want empty", e.Model)
	}
}

func TestAgentEntryUnmarshalObject(t *testing.T) {
	var e AgentEntry
	if err := json.Unmarshal([]byte(`{"agent":"review","model":"gpt-4"}`), &e); err != nil {
		t.Fatal(err)
	}
	if e.Agent != "review" {
		t.Errorf("got %q, want review", e.Agent)
	}
	if e.Model != "gpt-4" {
		t.Errorf("got %q, want gpt-4", e.Model)
	}
}

func TestAgentEntryMarshalStringShorthand(t *testing.T) {
	e := AgentEntry{AgentConfig{Agent: "build"}}
	data, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `"build"` {
		t.Errorf("got %s, want \"build\"", data)
	}
}

func TestAgentEntryMarshalObjectWithModel(t *testing.T) {
	e := AgentEntry{AgentConfig{Agent: "review", Model: "gpt-4"}}
	data, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	if m["agent"] != "review" || m["model"] != "gpt-4" {
		t.Errorf("unexpected: %s", data)
	}
}

func TestLoadDefaults(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AgentHarness != "opencode" {
		t.Errorf("got %q, want opencode", cfg.AgentHarness)
	}
	if len(cfg.Agents) != 2 {
		t.Errorf("got %d agents, want 2", len(cfg.Agents))
	}
}

func TestLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	cfgDir := filepath.Join(dir, "m")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.json"), []byte(`{"agent_harness":"claude","agents":{"deploy":"deploy"}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AgentHarness != "claude" {
		t.Errorf("got %q, want claude", cfg.AgentHarness)
	}
	if cfg.Agents["deploy"].Agent != "deploy" {
		t.Errorf("missing deploy agent")
	}
	// defaults should still be present
	if cfg.Agents["build"].Agent != "build" {
		t.Errorf("default build agent missing after merge")
	}
}

func TestValidateConfig(t *testing.T) {
	cfg := DefaultConfig()
	if err := ValidateConfig(cfg); err != nil {
		t.Fatal(err)
	}

	cfg.AgentHarness = "invalid"
	if err := ValidateConfig(cfg); err == nil {
		t.Error("expected error for invalid harness")
	}
}

func TestIsValidHarness(t *testing.T) {
	if !IsValidHarness("opencode") {
		t.Error("opencode should be valid")
	}
	if !IsValidHarness("claude") {
		t.Error("claude should be valid")
	}
	if IsValidHarness("other") {
		t.Error("other should not be valid")
	}
}
