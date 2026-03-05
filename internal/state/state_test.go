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

func TestNormalizeStackType(t *testing.T) {
	if got := NormalizeStackType("  FEAT "); got != "feat" {
		t.Fatalf("NormalizeStackType() = %q, want %q", got, "feat")
	}
}

func TestIsValidStackType(t *testing.T) {
	if !IsValidStackType("fix") {
		t.Fatal("IsValidStackType() = false, want true for fix")
	}
	if IsValidStackType("feature") {
		t.Fatal("IsValidStackType() = true, want false for feature")
	}
}

func TestEffectiveStatus(t *testing.T) {
	if got := EffectiveStatus(nil); got != StatusPending {
		t.Fatalf("EffectiveStatus(nil) = %q, want %q", got, StatusPending)
	}
	if got := EffectiveStatus(&Stage{}); got != StatusPending {
		t.Fatalf("EffectiveStatus(empty) = %q, want %q", got, StatusPending)
	}
	if got := EffectiveStatus(&Stage{Status: StatusImplementing}); got != StatusImplementing {
		t.Fatalf("EffectiveStatus(implementing) = %q, want %q", got, StatusImplementing)
	}
}

func TestValidTransition(t *testing.T) {
	valid := []struct{ from, to string }{
		{StatusPending, StatusImplementing},
		{StatusImplementing, StatusAIReview},
		{StatusAIReview, StatusHumanReview},
		{StatusHumanReview, StatusDone},
	}
	for _, tc := range valid {
		if !ValidTransition(tc.from, tc.to) {
			t.Errorf("ValidTransition(%q, %q) = false, want true", tc.from, tc.to)
		}
	}

	invalid := []struct{ from, to string }{
		{StatusPending, StatusAIReview},
		{StatusPending, StatusDone},
		{StatusImplementing, StatusDone},
		{StatusDone, StatusPending},
	}
	for _, tc := range invalid {
		if ValidTransition(tc.from, tc.to) {
			t.Errorf("ValidTransition(%q, %q) = true, want false", tc.from, tc.to)
		}
	}
}

func TestTransitionStage(t *testing.T) {
	stacks := &Stacks{
		Stacks: []Stack{
			{
				Name: "test-stack",
				Stages: []Stage{
					{ID: "stage-1"},
					{ID: "stage-2"},
				},
			},
		},
	}

	// pending → implementing
	if err := TransitionStage(stacks, "test-stack", "stage-1", StatusImplementing); err != nil {
		t.Fatal(err)
	}
	stage, _ := FindStage(&stacks.Stacks[0], "stage-1")
	if stage.Status != StatusImplementing {
		t.Fatalf("got %q, want %q", stage.Status, StatusImplementing)
	}
	if stage.StartedAt == "" {
		t.Fatal("StartedAt should be set")
	}

	// implementing → ai-review
	if err := TransitionStage(stacks, "test-stack", "stage-1", StatusAIReview); err != nil {
		t.Fatal(err)
	}

	// ai-review → human-review
	if err := TransitionStage(stacks, "test-stack", "stage-1", StatusHumanReview); err != nil {
		t.Fatal(err)
	}
	if stage.ReviewedAt == "" {
		t.Fatal("ReviewedAt should be set")
	}

	// Invalid: pending → done
	if err := TransitionStage(stacks, "test-stack", "stage-2", StatusDone); err == nil {
		t.Fatal("expected error for invalid transition")
	}

	// Not found
	if err := TransitionStage(stacks, "test-stack", "nope", StatusImplementing); err == nil {
		t.Fatal("expected error for missing stage")
	}
	if err := TransitionStage(stacks, "nope", "stage-1", StatusImplementing); err == nil {
		t.Fatal("expected error for missing stack")
	}
}

func TestNextPendingStage(t *testing.T) {
	stack := &Stack{
		Stages: []Stage{
			{ID: "s1", Status: StatusHumanReview},
			{ID: "s2", Status: ""},
			{ID: "s3", Status: StatusPending},
		},
	}

	next := NextPendingStage(stack)
	if next == nil || next.ID != "s2" {
		t.Fatalf("NextPendingStage() = %v, want s2", next)
	}
}

func TestAllStagesComplete(t *testing.T) {
	complete := &Stack{
		Stages: []Stage{
			{ID: "s1", Status: StatusHumanReview},
			{ID: "s2", Status: StatusDone},
		},
	}
	if !AllStagesComplete(complete) {
		t.Fatal("expected all complete")
	}

	incomplete := &Stack{
		Stages: []Stage{
			{ID: "s1", Status: StatusHumanReview},
			{ID: "s2", Status: StatusImplementing},
		},
	}
	if AllStagesComplete(incomplete) {
		t.Fatal("expected not all complete")
	}
}
