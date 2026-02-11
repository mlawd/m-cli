package cmd

import (
	"reflect"
	"strings"
	"testing"

	"github.com/mlawd/m-cli/internal/state"
)

func TestStageIndexesToPush(t *testing.T) {
	stack := &state.Stack{
		Name: "test-stack",
		Stages: []state.Stage{
			{ID: "stage-1", Branch: "test-stack/1/stage-1"},
			{ID: "stage-2", Branch: "test-stack/2/stage-2"},
			{ID: "stage-3", Branch: "test-stack/3/stage-3"},
		},
	}

	tests := []struct {
		name         string
		currentIndex int
		remoteExists map[string]bool
		want         []int
	}{
		{
			name:         "pushes missing stage 1 before stage 2",
			currentIndex: 1,
			remoteExists: map[string]bool{},
			want:         []int{0},
		},
		{
			name:         "skips stage 1 when already on remote",
			currentIndex: 1,
			remoteExists: map[string]bool{"test-stack/1/stage-1": true},
			want:         []int{},
		},
		{
			name:         "pushes only missing earlier stages",
			currentIndex: 2,
			remoteExists: map[string]bool{"test-stack/2/stage-2": true},
			want:         []int{0},
		},
		{
			name:         "pushes multiple missing earlier stages",
			currentIndex: 2,
			remoteExists: map[string]bool{},
			want:         []int{0, 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := stageIndexesToPush(stack, tt.currentIndex, func(branch string) bool {
				return tt.remoteExists[branch]
			})
			if err != nil {
				t.Fatalf("stageIndexesToPush returned error: %v", err)
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("stageIndexesToPush() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStageIndexesToPushErrors(t *testing.T) {
	t.Run("nil stack", func(t *testing.T) {
		_, err := stageIndexesToPush(nil, 0, func(string) bool { return false })
		if err == nil {
			t.Fatal("expected error for nil stack")
		}
	})

	t.Run("out of range index", func(t *testing.T) {
		stack := &state.Stack{Name: "test-stack", Stages: []state.Stage{{ID: "stage-1"}}}
		_, err := stageIndexesToPush(stack, 2, func(string) bool { return false })
		if err == nil {
			t.Fatal("expected error for out-of-range index")
		}
	})
}

func TestStagePRBodyIncludesStackLinks(t *testing.T) {
	stack := &state.Stack{
		Name: "test-stack",
		Stages: []state.Stage{
			{ID: "stage-1", Outcome: "First outcome"},
			{
				ID:             "stage-2",
				Outcome:        "Second outcome",
				Implementation: []string{"Create service interface", "Wire checkout handler"},
				Validation:     []string{"go test ./...", "exercise checkout quote endpoint"},
				Risks: []state.StageRisk{
					{Risk: "Payment gateway latency spikes", Mitigation: "Add retry with backoff and timeout metrics"},
				},
			},
			{ID: "stage-3", Outcome: "Third outcome"},
		},
	}

	prURLs := map[int]string{
		0: "https://github.com/org/repo/pull/10",
		2: "https://github.com/org/repo/pull/12",
	}

	body := stagePRBody(stack, 1, prURLs)
	checks := []string{
		"Stage: stage-2",
		"## Outcome",
		"Second outcome",
		"## Implementation",
		"- Create service interface",
		"## Validation",
		"- go test ./...",
		"## Risks",
		"Risk: Payment gateway latency spikes",
		"Mitigation: Add retry with backoff and timeout metrics",
		"## Stack PRs",
		"### Upstream",
		"- stage-1: https://github.com/org/repo/pull/10",
		"### Downstream",
		"- stage-3: https://github.com/org/repo/pull/12",
	}

	for _, check := range checks {
		if !strings.Contains(body, check) {
			t.Fatalf("expected body to contain %q; got:\n%s", check, body)
		}
	}
}

func TestStagePRBodyUsesNotCreatedPlaceholder(t *testing.T) {
	stack := &state.Stack{
		Name: "test-stack",
		Stages: []state.Stage{
			{ID: "stage-1"},
			{ID: "stage-2"},
		},
	}

	body := stagePRBody(stack, 1, map[int]string{})
	if !strings.Contains(body, "- stage-1: (not created)") {
		t.Fatalf("expected placeholder for missing upstream PR; got:\n%s", body)
	}
	if !strings.Contains(body, "No implementation details found for this stage.") {
		t.Fatalf("expected fallback details message; got:\n%s", body)
	}
	if !strings.Contains(body, "### Downstream\n- None") {
		t.Fatalf("expected downstream none section; got:\n%s", body)
	}
}

func TestPluralSuffix(t *testing.T) {
	if got := pluralSuffix(1); got != "" {
		t.Fatalf("pluralSuffix(1) = %q, want empty string", got)
	}
	if got := pluralSuffix(2); got != "s" {
		t.Fatalf("pluralSuffix(2) = %q, want s", got)
	}
}
