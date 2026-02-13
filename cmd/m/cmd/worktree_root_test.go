package cmd

import (
	"path/filepath"
	"testing"
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
