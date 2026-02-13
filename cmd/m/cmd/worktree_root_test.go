package cmd

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mlawd/m-cli/internal/state"
)

func TestNormalizeBranchName(t *testing.T) {
	t.Run("trims spaces", func(t *testing.T) {
		got, err := normalizeBranchName("  feature/test  ")
		if err != nil {
			t.Fatalf("normalizeBranchName returned error: %v", err)
		}
		if got != "feature/test" {
			t.Fatalf("normalizeBranchName() = %q, want %q", got, "feature/test")
		}
	})

	t.Run("empty branch", func(t *testing.T) {
		if _, err := normalizeBranchName("   "); err == nil {
			t.Fatal("expected error for empty branch")
		}
	})
}

func TestResolveBaseBranch(t *testing.T) {
	if got := resolveBaseBranch(" release/1.2 ", "main"); got != "release/1.2" {
		t.Fatalf("resolveBaseBranch override = %q, want %q", got, "release/1.2")
	}

	if got := resolveBaseBranch("", " main "); got != "main" {
		t.Fatalf("resolveBaseBranch default = %q, want %q", got, "main")
	}
}

func TestResolveWorktreePath(t *testing.T) {
	repoRoot := filepath.Join(string(filepath.Separator), "tmp", "repo")

	t.Run("default path", func(t *testing.T) {
		got, err := resolveWorktreePath(repoRoot, "stack/1/foundation", "")
		if err != nil {
			t.Fatalf("resolveWorktreePath returned error: %v", err)
		}

		want := filepath.Join(repoRoot, ".m", "worktrees", filepath.FromSlash("stack/1/foundation"))
		if got != want {
			t.Fatalf("resolveWorktreePath() = %q, want %q", got, want)
		}
	})

	t.Run("custom path", func(t *testing.T) {
		custom := filepath.Join(".", "custom", "tree")
		got, err := resolveWorktreePath(repoRoot, "stack/1/foundation", custom)
		if err != nil {
			t.Fatalf("resolveWorktreePath returned error: %v", err)
		}

		want, err := filepath.Abs(custom)
		if err != nil {
			t.Fatalf("filepath.Abs failed: %v", err)
		}
		want = filepath.Clean(want)
		if got != want {
			t.Fatalf("resolveWorktreePath() = %q, want %q", got, want)
		}
	})
}

func TestParseGitWorktreePorcelain(t *testing.T) {
	input := `worktree /tmp/repo
HEAD 5b47d2
branch refs/heads/main

worktree /tmp/repo/.m/worktrees/feature/test
HEAD 8f0acb
branch refs/heads/feature/test

worktree /tmp/repo/.m/worktrees/detached
HEAD 123abc
detached
`

	got := parseGitWorktreePorcelain(input)
	want := []gitWorktree{
		{Path: "/tmp/repo", Branch: "main"},
		{Path: "/tmp/repo/.m/worktrees/feature/test", Branch: "feature/test"},
		{Path: "/tmp/repo/.m/worktrees/detached", Branch: "", Detached: true},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseGitWorktreePorcelain() = %#v, want %#v", got, want)
	}
}

func TestOrphanManagedWorktrees(t *testing.T) {
	managed := []string{
		"/tmp/repo/.m/worktrees/main",
		"/tmp/repo/.m/worktrees/feature/a",
		"/tmp/repo/.m/worktrees/feature/b",
	}
	active := map[string]struct{}{
		normalizeCmdPath("/tmp/repo/.m/worktrees/main"):      {},
		normalizeCmdPath("/tmp/repo/.m/worktrees/feature/b"): {},
	}

	got := orphanManagedWorktrees(managed, active)
	want := []string{normalizeCmdPath("/tmp/repo/.m/worktrees/feature/a")}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("orphanManagedWorktrees() = %#v, want %#v", got, want)
	}
}

func TestClearMissingStageWorktrees(t *testing.T) {
	stacks := &state.Stacks{
		Stacks: []state.Stack{
			{
				Name: "checkout",
				Stages: []state.Stage{
					{ID: "foundation", Worktree: "/tmp/repo/.m/worktrees/checkout/1/foundation"},
					{ID: "api", Worktree: "/tmp/repo/.m/worktrees/checkout/2/api"},
				},
			},
		},
	}

	exists := map[string]bool{
		normalizeCmdPath("/tmp/repo/.m/worktrees/checkout/1/foundation"): true,
	}

	cleared := clearMissingStageWorktrees(stacks, func(path string) bool {
		return exists[normalizeCmdPath(path)]
	})

	if cleared != 1 {
		t.Fatalf("cleared = %d, want 1", cleared)
	}
	if stacks.Stacks[0].Stages[0].Worktree == "" {
		t.Fatal("expected existing stage worktree to remain")
	}
	if stacks.Stacks[0].Stages[1].Worktree != "" {
		t.Fatal("expected missing stage worktree to be cleared")
	}
}
