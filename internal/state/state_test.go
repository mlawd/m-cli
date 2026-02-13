package state

import (
	"path/filepath"
	"testing"
)

func TestCurrentWorkspaceStackStage(t *testing.T) {
	worktreePath := filepath.Join(string(filepath.Separator), "tmp", "repo", ".m", "worktrees", "feature", "one")

	stacks := &Stacks{
		Stacks: []Stack{
			{
				Name: "checkout",
				Stages: []Stage{
					{ID: "foundation", Worktree: worktreePath},
				},
			},
		},
	}

	gotStack, gotStage := CurrentWorkspaceStackStage(stacks, worktreePath)
	if gotStack != "checkout" {
		t.Fatalf("CurrentWorkspaceStackStage() stack = %q, want %q", gotStack, "checkout")
	}
	if gotStage != "foundation" {
		t.Fatalf("CurrentWorkspaceStackStage() stage = %q, want %q", gotStage, "foundation")
	}
}

func TestCurrentWorkspaceStackStageReturnsEmptyForUnknownPath(t *testing.T) {
	stacks := &Stacks{
		Stacks: []Stack{
			{
				Name:   "checkout",
				Stages: []Stage{{ID: "foundation", Worktree: filepath.Join(string(filepath.Separator), "tmp", "repo", "known")}},
			},
		},
	}

	gotStack, gotStage := CurrentWorkspaceStackStage(stacks, filepath.Join(string(filepath.Separator), "tmp", "repo", "unknown"))
	if gotStack != "" || gotStage != "" {
		t.Fatalf("CurrentWorkspaceStackStage() = (%q, %q), want empty values", gotStack, gotStage)
	}
}

func TestIsLinkedWorktree(t *testing.T) {
	repoRoot := filepath.Join(string(filepath.Separator), "tmp", "repo")
	if IsLinkedWorktree(repoRoot, repoRoot) {
		t.Fatal("IsLinkedWorktree() = true for primary repo root, want false")
	}

	linked := filepath.Join(repoRoot, ".m", "worktrees", "feature", "one")
	if !IsLinkedWorktree(linked, repoRoot) {
		t.Fatal("IsLinkedWorktree() = false for linked worktree, want true")
	}
}
