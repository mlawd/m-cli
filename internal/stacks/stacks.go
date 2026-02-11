package stacks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type State struct {
	Version int     `json:"version"`
	Stacks  []Stack `json:"stacks"`
}

type Stack struct {
	Name       string `json:"name"`
	BaseBranch string `json:"base_branch"`
	Parts      []Part `json:"parts"`
}

type Part struct {
	Index        int    `json:"index"`
	Label        string `json:"label"`
	Slug         string `json:"slug"`
	Branch       string `json:"branch"`
	Worktree     string `json:"worktree_path"`
	CreatedAt    string `json:"created_at"`
	ParentBranch string `json:"parent_branch"`
}

var sepRegexp = regexp.MustCompile(`[-_\s]+`)
var invalidRegexp = regexp.MustCompile(`[^a-z0-9-]`)
var dashRegexp = regexp.MustCompile(`-+`)

func StatePath(gitCommonDir string) string {
	return filepath.Join(gitCommonDir, "m", "stacks.json")
}

func Load(path string) (*State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &State{Version: 1, Stacks: []Stack{}}, nil
		}
		return nil, err
	}

	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse state: %w", err)
	}

	if s.Version == 0 {
		s.Version = 1
	}
	if s.Stacks == nil {
		s.Stacks = []Stack{}
	}

	return &s, nil
}

func Save(path string, s *State) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(s, "", "  ")
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

func FindStack(s *State, name string) (*Stack, int) {
	for i := range s.Stacks {
		if s.Stacks[i].Name == name {
			return &s.Stacks[i], i
		}
	}

	return nil, -1
}

func EnsureStack(s *State, name, baseBranch string) (created bool) {
	if stack, _ := FindStack(s, name); stack != nil {
		return false
	}

	s.Stacks = append(s.Stacks, Stack{
		Name:       name,
		BaseBranch: baseBranch,
		Parts:      []Part{},
	})

	return true
}

func SlugPart(label string) string {
	slug := strings.ToLower(strings.TrimSpace(label))
	slug = sepRegexp.ReplaceAllString(slug, "-")
	slug = invalidRegexp.ReplaceAllString(slug, "")
	slug = dashRegexp.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-/")
	return slug
}

func FindPartBySlug(stack *Stack, slug string) *Part {
	for i := range stack.Parts {
		if stack.Parts[i].Slug == slug {
			return &stack.Parts[i]
		}
	}

	return nil
}

func NextIndex(stack *Stack) int {
	max := 0
	for _, p := range stack.Parts {
		if p.Index > max {
			max = p.Index
		}
	}

	return max + 1
}

func LastPart(stack *Stack) *Part {
	if len(stack.Parts) == 0 {
		return nil
	}

	parts := make([]Part, len(stack.Parts))
	copy(parts, stack.Parts)
	sort.Slice(parts, func(i, j int) bool {
		return parts[i].Index < parts[j].Index
	})

	last := parts[len(parts)-1]
	return &last
}

func BuildBranch(stackName string, index int, slug string) string {
	return fmt.Sprintf("%s/%d/%s", strings.Trim(stackName, "/"), index, slug)
}

func BuildWorktreePath(repoRoot, branch string) string {
	return filepath.Join(repoRoot, "worktrees", filepath.FromSlash(branch))
}

func NewPart(index int, label, slug, branch, parentBranch, worktree string) Part {
	return Part{
		Index:        index,
		Label:        label,
		Slug:         slug,
		Branch:       branch,
		ParentBranch: parentBranch,
		Worktree:     worktree,
		CreatedAt:    time.Now().UTC().Format(time.RFC3339),
	}
}
