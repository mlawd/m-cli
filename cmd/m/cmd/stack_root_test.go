package cmd

import (
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
