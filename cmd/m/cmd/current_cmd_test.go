package cmd

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mlawd/m-cli/internal/state"
)

func TestCurrentCommandsInLinkedWorktrees(t *testing.T) {
	repoRoot := initGitRepoForCurrentCmdTests(t)

	mappedWorktree := filepath.Join(t.TempDir(), "mapped-worktree")
	stacklessWorktree := filepath.Join(t.TempDir(), "stackless-worktree")

	runGitForCurrentCmdTests(t, repoRoot, "worktree", "add", "-b", "feature/mapped", mappedWorktree)
	runGitForCurrentCmdTests(t, repoRoot, "worktree", "add", "-b", "feature/stackless", stacklessWorktree)

	if err := state.SaveConfig(repoRoot, &state.Config{Version: 1, CurrentStack: "checkout"}); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	stacks := &state.Stacks{
		Version: 1,
		Stacks: []state.Stack{
			{
				Name:         "checkout",
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

	t.Run("stackless linked worktree prints no current stack or stage", func(t *testing.T) {
		stackOut, err := runRootCmdInDir(stacklessWorktree, "stack", "current")
		if err != nil {
			t.Fatalf("stack current returned error: %v", err)
		}
		if strings.TrimSpace(stackOut) != "" {
			t.Fatalf("stack current output = %q, want empty", stackOut)
		}

		stageOut, err := runRootCmdInDir(stacklessWorktree, "stage", "current")
		if err != nil {
			t.Fatalf("stage current returned error: %v", err)
		}
		if strings.TrimSpace(stageOut) != "" {
			t.Fatalf("stage current output = %q, want empty", stageOut)
		}
	})

	t.Run("mapped linked worktree prints owning stack and stage", func(t *testing.T) {
		stackOut, err := runRootCmdInDir(mappedWorktree, "stack", "current")
		if err != nil {
			t.Fatalf("stack current returned error: %v", err)
		}
		if !strings.Contains(stackOut, "Current stack: checkout") {
			t.Fatalf("stack current output = %q, expected current stack", stackOut)
		}

		stageOut, err := runRootCmdInDir(mappedWorktree, "stage", "current")
		if err != nil {
			t.Fatalf("stage current returned error: %v", err)
		}
		if !strings.Contains(stageOut, "Current stage: foundation") {
			t.Fatalf("stage current output = %q, expected current stage", stageOut)
		}
	})
}

func runRootCmdInDir(dir string, args ...string) (string, error) {
	buf := &bytes.Buffer{}
	root := NewRootCmd("test")
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)

	originalDir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	defer func() { _ = os.Chdir(originalDir) }()

	if err := os.Chdir(dir); err != nil {
		return "", err
	}

	err = root.Execute()
	return buf.String(), err
}

func initGitRepoForCurrentCmdTests(t *testing.T) string {
	t.Helper()
	repoRoot := t.TempDir()
	runGitForCurrentCmdTests(t, repoRoot, "init", "-b", "main")

	readmePath := filepath.Join(repoRoot, "README.md")
	if err := os.WriteFile(readmePath, []byte("init\n"), 0o644); err != nil {
		t.Fatalf("write README.md: %v", err)
	}

	runGitForCurrentCmdTests(t, repoRoot, "add", "README.md")
	runGitForCurrentCmdTests(t, repoRoot, "commit", "-m", "init")
	return repoRoot
}

func runGitForCurrentCmdTests(t *testing.T, dir string, args ...string) {
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
