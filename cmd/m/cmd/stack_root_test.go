package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mlawd/m-cli/internal/state"
)

func TestParseStagesFromPlanFileRequiresMarkdownExtension(t *testing.T) {
	tempDir := t.TempDir()
	planPath := filepath.Join(tempDir, "plan.yaml")

	content := `---
version: 2
stages:
  - id: foundation
    title: Foundation
    outcome: Done
    implementation:
      - build it
    validation:
      - test it
    risks:
      - risk: drift
        mitigation: review
---
`
	if err := os.WriteFile(planPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write plan: %v", err)
	}

	_, _, err := parseStagesFromPlanFile(planPath)
	if err == nil {
		t.Fatal("expected error for non-markdown plan file")
	}
	if !strings.Contains(err.Error(), "must use .md extension") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStartedStageIndexes(t *testing.T) {
	stack := &state.Stack{
		Name: "test-stack",
		Stages: []state.Stage{
			{ID: "stage-1", Branch: "test-stack/1/stage-1"},
			{ID: "stage-2", Branch: "test-stack/2/stage-2"},
			{ID: "stage-3", Branch: "test-stack/3/stage-3"},
		},
	}

	tests := []struct {
		name        string
		exists      map[string]bool
		wantIndexes []int
	}{
		{
			name:        "none started",
			exists:      map[string]bool{},
			wantIndexes: []int{},
		},
		{
			name: "subset started",
			exists: map[string]bool{
				"test-stack/1/stage-1": true,
				"test-stack/3/stage-3": true,
			},
			wantIndexes: []int{0, 2},
		},
		{
			name: "all started",
			exists: map[string]bool{
				"test-stack/1/stage-1": true,
				"test-stack/2/stage-2": true,
				"test-stack/3/stage-3": true,
			},
			wantIndexes: []int{0, 1, 2},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := startedStageIndexes(stack, func(branch string) bool {
				return tt.exists[branch]
			})
			if err != nil {
				t.Fatalf("startedStageIndexes returned error: %v", err)
			}

			if !reflect.DeepEqual(got, tt.wantIndexes) {
				t.Fatalf("startedStageIndexes() = %v, want %v", got, tt.wantIndexes)
			}
		})
	}
}

func TestStartedStageIndexesNilStack(t *testing.T) {
	if _, err := startedStageIndexes(nil, func(string) bool { return false }); err == nil {
		t.Fatal("expected error for nil stack")
	}
}

func TestRemoveStackByIndexReturnsRemovedName(t *testing.T) {
	stacks := []state.Stack{
		{Name: "first"},
		{Name: "second"},
	}

	updated, removedName := removeStackByIndex(stacks, 0)
	if removedName != "first" {
		t.Fatalf("removedName = %q, want %q", removedName, "first")
	}
	if len(updated) != 1 {
		t.Fatalf("len(updated) = %d, want %d", len(updated), 1)
	}
	if updated[0].Name != "second" {
		t.Fatalf("updated[0].Name = %q, want %q", updated[0].Name, "second")
	}
}

func TestParseStagesFromPlanFileV3Context(t *testing.T) {
	tempDir := t.TempDir()
	planPath := filepath.Join(tempDir, "plan.md")

	content := `---
version: 3
title: Feature rollout
stages:
  - id: foundation
    title: Foundation
---

## Stage: foundation
Preserve existing default values and keep behavior compatible with legacy checkout.
`

	if err := os.WriteFile(planPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write plan: %v", err)
	}

	_, stages, err := parseStagesFromPlanFile(planPath)
	if err != nil {
		t.Fatalf("parseStagesFromPlanFile returned error: %v", err)
	}
	if len(stages) != 1 {
		t.Fatalf("len(stages) = %d, want 1", len(stages))
	}
	if got := strings.TrimSpace(stages[0].Context); got == "" {
		t.Fatal("expected stage context to be populated")
	}
}

func TestPruneMergedStagesRemovesMergedAndAdvancesCurrent(t *testing.T) {
	stack := &state.Stack{
		Name:         "test-stack",
		CurrentStage: "stage-2",
		Stages: []state.Stage{
			{ID: "stage-1", Branch: "test-stack/1/stage-1", Worktree: "/tmp/wt1"},
			{ID: "stage-2", Branch: "test-stack/2/stage-2", Worktree: "/tmp/wt2"},
			{ID: "stage-3", Branch: "test-stack/3/stage-3", Worktree: "/tmp/wt3"},
		},
	}

	merged := map[string]bool{
		"test-stack/2/stage-2": true,
	}

	cleaned := []string{}
	pruned, mutated, err := pruneMergedStages(
		stack,
		func(branch string) (bool, error) {
			return merged[branch], nil
		},
		func(stage state.Stage, branch string) error {
			cleaned = append(cleaned, fmt.Sprintf("%s:%s", stage.ID, branch))
			return nil
		},
	)
	if err != nil {
		t.Fatalf("pruneMergedStages returned error: %v", err)
	}
	if !mutated {
		t.Fatal("expected mutated=true")
	}
	if pruned != 1 {
		t.Fatalf("pruned = %d, want 1", pruned)
	}

	if got := []string{stack.Stages[0].ID, stack.Stages[1].ID}; !reflect.DeepEqual(got, []string{"stage-1", "stage-3"}) {
		t.Fatalf("remaining stage IDs = %v, want [stage-1 stage-3]", got)
	}
	if stack.CurrentStage != "stage-3" {
		t.Fatalf("CurrentStage = %q, want %q", stack.CurrentStage, "stage-3")
	}
	if !reflect.DeepEqual(cleaned, []string{"stage-2:test-stack/2/stage-2"}) {
		t.Fatalf("cleanup calls = %v, want one stage-2 call", cleaned)
	}
}

func TestPruneMergedStagesNoMerged(t *testing.T) {
	stack := &state.Stack{
		Name:         "test-stack",
		CurrentStage: "stage-1",
		Stages: []state.Stage{
			{ID: "stage-1", Branch: "test-stack/1/stage-1"},
			{ID: "stage-2", Branch: "test-stack/2/stage-2"},
		},
	}

	pruned, mutated, err := pruneMergedStages(
		stack,
		func(string) (bool, error) { return false, nil },
		func(state.Stage, string) error { return nil },
	)
	if err != nil {
		t.Fatalf("pruneMergedStages returned error: %v", err)
	}
	if mutated {
		t.Fatal("expected mutated=false")
	}
	if pruned != 0 {
		t.Fatalf("pruned = %d, want 0", pruned)
	}
	if stack.CurrentStage != "stage-1" {
		t.Fatalf("CurrentStage = %q, want stage-1", stack.CurrentStage)
	}
}

func TestPruneMergedStagesErrors(t *testing.T) {
	t.Run("nil stack", func(t *testing.T) {
		_, _, err := pruneMergedStages(nil, func(string) (bool, error) { return false, nil }, func(state.Stage, string) error { return nil })
		if err == nil {
			t.Fatal("expected error for nil stack")
		}
	})

	t.Run("merge checker error", func(t *testing.T) {
		stack := &state.Stack{Name: "test-stack", Stages: []state.Stage{{ID: "stage-1", Branch: "test-stack/1/stage-1"}}}
		_, _, err := pruneMergedStages(
			stack,
			func(string) (bool, error) { return false, fmt.Errorf("gh failed") },
			func(state.Stage, string) error { return nil },
		)
		if err == nil || !strings.Contains(err.Error(), "gh failed") {
			t.Fatalf("expected checker error, got: %v", err)
		}
	})
}

func TestBuildStageSyncInfos(t *testing.T) {
	stack := &state.Stack{
		Name: "test-stack",
		Stages: []state.Stage{
			{ID: "stage-1", Branch: "test-stack/1/stage-1"},
			{ID: "stage-2"},
			{ID: "stage-3", Branch: "custom/branch-3"},
		},
	}

	infos := buildStageSyncInfos(stack, "main")
	if len(infos) != 3 {
		t.Fatalf("len(infos) = %d, want 3", len(infos))
	}

	if infos[0].OldParent != "main" {
		t.Fatalf("infos[0].OldParent = %q, want main", infos[0].OldParent)
	}
	if infos[1].Branch != "test-stack/2/stage-2" {
		t.Fatalf("infos[1].Branch = %q, want generated stage branch", infos[1].Branch)
	}
	if infos[1].OldParent != "test-stack/1/stage-1" {
		t.Fatalf("infos[1].OldParent = %q, want previous stage branch", infos[1].OldParent)
	}
	if infos[2].OldParent != "test-stack/2/stage-2" {
		t.Fatalf("infos[2].OldParent = %q, want generated stage-2 branch", infos[2].OldParent)
	}
}

func TestShouldTransplantRebase(t *testing.T) {
	info := stackSyncStageInfo{
		Branch:       "test-stack/2/stage-2",
		OldParent:    "test-stack/1/stage-1",
		ParentMerged: true,
	}

	if !shouldTransplantRebase(info, true, map[string]bool{"test-stack/1/stage-1": true}, "main") {
		t.Fatal("expected transplant rebase when parent stage is merged")
	}

	if shouldTransplantRebase(info, true, map[string]bool{"test-stack/1/stage-1": false}, "main") {
		t.Fatal("did not expect transplant rebase when parent stage is not merged")
	}

	if shouldTransplantRebase(info, false, map[string]bool{"test-stack/1/stage-1": true}, "main") {
		t.Fatal("did not expect transplant rebase when prune mode is disabled")
	}

	if shouldTransplantRebase(info, true, map[string]bool{"test-stack/1/stage-1": true}, "test-stack/1/stage-1") {
		t.Fatal("did not expect transplant rebase when current parent equals old parent")
	}
}

func TestRunRebaseWithAbort(t *testing.T) {
	t.Run("aborts after rebase failure", func(t *testing.T) {
		calls := []string{}
		err := runRebaseWithAbort(
			func(_ string, args ...string) (string, error) {
				calls = append(calls, strings.Join(args, " "))
				if len(args) >= 1 && args[0] == "rebase" && (len(args) < 2 || args[1] != "--abort") {
					return "", fmt.Errorf("conflict")
				}
				return "", nil
			},
			"/tmp/wt",
			[]string{"rebase", "main"},
			"stage-2",
			"test-stack/2/stage-2",
			"plain",
		)
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "Aborted rebase") {
			t.Fatalf("expected aborted message, got: %v", err)
		}
		if !reflect.DeepEqual(calls, []string{"rebase main", "rebase --abort"}) {
			t.Fatalf("unexpected call sequence: %v", calls)
		}
	})

	t.Run("reports abort failure", func(t *testing.T) {
		err := runRebaseWithAbort(
			func(_ string, args ...string) (string, error) {
				if len(args) >= 2 && args[0] == "rebase" && args[1] == "--abort" {
					return "", fmt.Errorf("abort failed")
				}
				return "", fmt.Errorf("conflict")
			},
			"/tmp/wt",
			[]string{"rebase", "main"},
			"stage-2",
			"test-stack/2/stage-2",
			"plain",
		)
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "abort also failed") {
			t.Fatalf("expected abort failure in message, got: %v", err)
		}
	})
}

func TestStackNewPersistsType(t *testing.T) {
	repoRoot := initGitRepoForCurrentCmdTests(t)

	out, err := runRootCmdInDir(repoRoot, "stack", "new", "checkout", "--type", "feat")
	if err != nil {
		t.Fatalf("stack new returned error: %v\noutput: %s", err, out)
	}

	stacks, err := state.LoadStacks(repoRoot)
	if err != nil {
		t.Fatalf("LoadStacks returned error: %v", err)
	}
	if len(stacks.Stacks) != 1 {
		t.Fatalf("len(stacks.Stacks) = %d, want 1", len(stacks.Stacks))
	}
	if stacks.Stacks[0].Type != "feat" {
		t.Fatalf("stack type = %q, want %q", stacks.Stacks[0].Type, "feat")
	}
}

func TestStackNewRejectsInvalidType(t *testing.T) {
	repoRoot := initGitRepoForCurrentCmdTests(t)

	out, err := runRootCmdInDir(repoRoot, "stack", "new", "checkout", "--type", "feature")
	if err == nil {
		t.Fatalf("expected error for invalid type, got none\noutput: %s", out)
	}
	if !strings.Contains(err.Error(), "invalid stack type") {
		t.Fatalf("unexpected error for invalid type: %v", err)
	}
}

func TestFormatStackDisplayName(t *testing.T) {
	if got := formatStackDisplayName(state.Stack{Name: "checkout", Type: "fix"}); got != "checkout (fix)" {
		t.Fatalf("formatStackDisplayName() = %q, want %q", got, "checkout (fix)")
	}
	if got := formatStackDisplayName(state.Stack{Name: "checkout"}); got != "checkout" {
		t.Fatalf("formatStackDisplayName() = %q, want %q", got, "checkout")
	}
}
