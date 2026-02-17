package mcp

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/mlawd/m-cli/internal/state"
)

func TestBuildContextSnapshotStacklessLinkedWorktree(t *testing.T) {
	repoRoot := initGitRepoWithMainCommit(t)

	mappedWorktree := filepath.Join(t.TempDir(), "mapped-worktree")
	stacklessWorktree := filepath.Join(t.TempDir(), "stackless-worktree")

	runGit(t, repoRoot, "worktree", "add", "-b", "feature/mapped", mappedWorktree)
	runGit(t, repoRoot, "worktree", "add", "-b", "feature/stackless", stacklessWorktree)

	stacks := &state.Stacks{
		Version: 1,
		Stacks: []state.Stack{
			{
				Name:         "checkout",
				Type:         "feat",
				PlanFile:     "plan.md",
				CurrentStage: "foundation",
				Stages: []state.Stage{
					{ID: "foundation", Worktree: mappedWorktree},
				},
			},
		},
	}

	if err := state.SaveStacks(repoRoot, stacks); err != nil {
		t.Fatalf("SaveStacks: %v", err)
	}

	t.Run("mapped linked worktree reports stack and stage", func(t *testing.T) {
		snapshot, err := BuildContextSnapshot(mappedWorktree, false)
		if err != nil {
			t.Fatalf("BuildContextSnapshot returned error: %v", err)
		}

		if snapshot.CurrentStack != "checkout" {
			t.Fatalf("CurrentStack = %q, want %q", snapshot.CurrentStack, "checkout")
		}
		if snapshot.CurrentStage != "foundation" {
			t.Fatalf("CurrentStage = %q, want %q", snapshot.CurrentStage, "foundation")
		}
		if snapshot.CurrentStackType != "feat" {
			t.Fatalf("CurrentStackType = %q, want %q", snapshot.CurrentStackType, "feat")
		}
	})

	t.Run("stackless linked worktree reports empty stack and stage", func(t *testing.T) {
		snapshot, err := BuildContextSnapshot(stacklessWorktree, false)
		if err != nil {
			t.Fatalf("BuildContextSnapshot returned error: %v", err)
		}

		if snapshot.CurrentStack != "" {
			t.Fatalf("CurrentStack = %q, want empty", snapshot.CurrentStack)
		}
		if snapshot.CurrentStage != "" {
			t.Fatalf("CurrentStage = %q, want empty", snapshot.CurrentStage)
		}
	})
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test",
		"GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test",
		"GIT_COMMITTER_EMAIL=test@example.com",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(out))
	}
}

func initGitRepoWithMainCommit(t *testing.T) string {
	t.Helper()
	repoRoot := t.TempDir()
	runGit(t, repoRoot, "init", "-b", "main")

	readmePath := filepath.Join(repoRoot, "README.md")
	if err := os.WriteFile(readmePath, []byte("init\n"), 0o644); err != nil {
		t.Fatalf("write README.md: %v", err)
	}

	runGit(t, repoRoot, "add", "README.md")
	runGit(t, repoRoot, "commit", "-m", "init")
	return repoRoot
}
