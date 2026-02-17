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

type Stacks struct {
	Version int     `json:"version"`
	Stacks  []Stack `json:"stacks"`
}

type Stack struct {
	Name         string  `json:"name"`
	Type         string  `json:"type,omitempty"`
	PlanFile     string  `json:"plan_file"`
	CreatedAt    string  `json:"created_at"`
	CurrentStage string  `json:"current_stage,omitempty"`
	Stages       []Stage `json:"stages"`
}

type Stage struct {
	ID             string      `json:"id"`
	Title          string      `json:"title"`
	Outcome        string      `json:"outcome,omitempty"`
	Implementation []string    `json:"implementation,omitempty"`
	Validation     []string    `json:"validation,omitempty"`
	Risks          []StageRisk `json:"risks,omitempty"`
	Context        string      `json:"context,omitempty"`
	Branch         string      `json:"branch,omitempty"`
	Worktree       string      `json:"worktree,omitempty"`
	Parent         string      `json:"parent_branch,omitempty"`
}

type StageRisk struct {
	Risk       string `json:"risk"`
	Mitigation string `json:"mitigation"`
}

func Dir(repoRoot string) string {
	return filepath.Join(repoRoot, ".m")
}

func StacksPath(repoRoot string) string {
	return filepath.Join(Dir(repoRoot), "stacks", "index.json")
}

func StacksDir(repoRoot string) string {
	return filepath.Join(Dir(repoRoot), "stacks")
}

func WorktreesDir(repoRoot string) string {
	return filepath.Join(Dir(repoRoot), "worktrees")
}

func EnsureInitialized(repoRoot string) error {
	dir := Dir(repoRoot)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	if err := os.MkdirAll(StacksDir(repoRoot), 0o755); err != nil {
		return err
	}

	if err := os.MkdirAll(WorktreesDir(repoRoot), 0o755); err != nil {
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

var validStackTypes = map[string]struct{}{
	"feat":  {},
	"fix":   {},
	"chore": {},
}

func NormalizeStackType(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

func IsValidStackType(stackType string) bool {
	_, ok := validStackTypes[NormalizeStackType(stackType)]
	return ok
}

func NewStack(name, stackType, planFile string, stages []Stage) Stack {
	return Stack{
		Name:      name,
		Type:      NormalizeStackType(stackType),
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

func CurrentWorkspaceStackStage(stacks *Stacks, workspaceRoot string) (string, string) {
	if stacks == nil {
		return "", ""
	}

	workspace := normalizePath(workspaceRoot)
	if workspace == "" {
		return "", ""
	}

	for _, stack := range stacks.Stacks {
		for _, stage := range stack.Stages {
			if pathEqual(stage.Worktree, workspace) {
				return strings.TrimSpace(stack.Name), strings.TrimSpace(stage.ID)
			}
		}
	}

	return "", ""
}

func CurrentWorkspaceStackStageByPath(repoRoot, workspaceRoot string) (string, string) {
	workspace := normalizePath(workspaceRoot)
	if workspace == "" {
		return "", ""
	}

	stacksRoot := normalizePath(StacksDir(repoRoot))
	if stacksRoot == "" {
		return "", ""
	}

	rel, err := filepath.Rel(stacksRoot, workspace)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return "", ""
	}

	parts := splitPathParts(rel)
	if len(parts) < 1 {
		return "", ""
	}

	stackName := strings.TrimSpace(parts[0])
	if len(parts) >= 2 {
		return stackName, strings.TrimSpace(parts[1])
	}

	return stackName, ""
}

func IsLinkedWorktree(worktreePath, repoRoot string) bool {
	worktree := normalizePath(worktreePath)
	root := normalizePath(repoRoot)

	if worktree == "" || root == "" {
		return false
	}

	return worktree != root
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

	if resolved, err := filepath.EvalSymlinks(trimmed); err == nil {
		trimmed = resolved
	}

	return filepath.Clean(trimmed)
}

func splitPathParts(rel string) []string {
	cleaned := filepath.Clean(strings.TrimSpace(rel))
	if cleaned == "." || cleaned == "" {
		return nil
	}

	raw := strings.Split(cleaned, string(filepath.Separator))
	parts := make([]string, 0, len(raw))
	for _, item := range raw {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" || trimmed == "." {
			continue
		}
		parts = append(parts, trimmed)
	}

	return parts
}
