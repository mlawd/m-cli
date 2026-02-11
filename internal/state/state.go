package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const schemaVersion = 1

type Config struct {
	Version      int    `json:"version"`
	CurrentStack string `json:"current_stack,omitempty"`
}

type Stacks struct {
	Version int     `json:"version"`
	Stacks  []Stack `json:"stacks"`
}

type Stack struct {
	Name         string  `json:"name"`
	PlanFile     string  `json:"plan_file"`
	CreatedAt    string  `json:"created_at"`
	CurrentStage string  `json:"current_stage,omitempty"`
	Stages       []Stage `json:"stages"`
}

type Stage struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Branch      string `json:"branch,omitempty"`
	Worktree    string `json:"worktree,omitempty"`
	Parent      string `json:"parent_branch,omitempty"`
}

func Dir(repoRoot string) string {
	return filepath.Join(repoRoot, ".m")
}

func ConfigPath(repoRoot string) string {
	return filepath.Join(Dir(repoRoot), "config.json")
}

func StacksPath(repoRoot string) string {
	return filepath.Join(Dir(repoRoot), "stacks.json")
}

func EnsureInitialized(repoRoot string) error {
	dir := Dir(repoRoot)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	configPath := ConfigPath(repoRoot)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		if err := writeJSONAtomic(configPath, defaultConfig()); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	stacksPath := StacksPath(repoRoot)
	if _, err := os.Stat(stacksPath); os.IsNotExist(err) {
		if err := writeJSONAtomic(stacksPath, defaultStacks()); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	return nil
}

func LoadConfig(repoRoot string) (*Config, error) {
	path := ConfigPath(repoRoot)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultConfig(), nil
		}
		return nil, err
	}

	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if c.Version == 0 {
		c.Version = schemaVersion
	}

	return &c, nil
}

func SaveConfig(repoRoot string, c *Config) error {
	if c.Version == 0 {
		c.Version = schemaVersion
	}
	return writeJSONAtomic(ConfigPath(repoRoot), c)
}

func LoadStacks(repoRoot string) (*Stacks, error) {
	path := StacksPath(repoRoot)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultStacks(), nil
		}
		return nil, err
	}

	var s Stacks
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse stacks: %w", err)
	}
	if s.Version == 0 {
		s.Version = schemaVersion
	}
	if s.Stacks == nil {
		s.Stacks = []Stack{}
	}

	return &s, nil
}

func SaveStacks(repoRoot string, s *Stacks) error {
	if s.Version == 0 {
		s.Version = schemaVersion
	}
	if s.Stacks == nil {
		s.Stacks = []Stack{}
	}
	return writeJSONAtomic(StacksPath(repoRoot), s)
}

func FindStack(s *Stacks, name string) (*Stack, int) {
	for i := range s.Stacks {
		if s.Stacks[i].Name == name {
			return &s.Stacks[i], i
		}
	}

	return nil, -1
}

func FindStage(stack *Stack, id string) (*Stage, int) {
	for i := range stack.Stages {
		if stack.Stages[i].ID == id {
			return &stack.Stages[i], i
		}
	}

	return nil, -1
}

func NewStack(name, planFile string, stages []Stage) Stack {
	return Stack{
		Name:      name,
		PlanFile:  planFile,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		Stages:    stages,
	}
}

func EffectiveCurrentStage(stack *Stack, workspaceRoot string) string {
	if stack == nil {
		return ""
	}

	workspace := normalizePath(workspaceRoot)
	if workspace != "" {
		for _, stage := range stack.Stages {
			if pathEqual(stage.Worktree, workspace) {
				return strings.TrimSpace(stage.ID)
			}
		}
	}

	return strings.TrimSpace(stack.CurrentStage)
}

func defaultConfig() *Config {
	return &Config{Version: schemaVersion}
}

func defaultStacks() *Stacks {
	return &Stacks{Version: schemaVersion, Stacks: []Stack{}}
}

func writeJSONAtomic(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return err
	}

	return os.Rename(tmpPath, path)
}

func pathEqual(a, b string) bool {
	aPath := normalizePath(a)
	bPath := normalizePath(b)

	if aPath == "" || bPath == "" {
		return false
	}

	return aPath == bPath
}

func normalizePath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}

	abs, err := filepath.Abs(trimmed)
	if err == nil {
		trimmed = abs
	}

	return filepath.Clean(trimmed)
}
