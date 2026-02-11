package plan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseFileMarkdownFrontmatter(t *testing.T) {
	tempDir := t.TempDir()
	planPath := filepath.Join(tempDir, "plan.md")

	content := `---
version: 2
title: Checkout rollout
stages:
  - id: foundation
    title: Foundation setup
    outcome: Core checkout contracts are in place.
    implementation:
      - Add order and pricing interfaces.
    validation:
      - go test ./...
    risks:
      - risk: API shape drift
        mitigation: Add contract fixtures in tests.
---

## Notes
This is optional narrative content.
`

	if err := os.WriteFile(planPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write plan: %v", err)
	}

	parsed, err := ParseFile(planPath)
	if err != nil {
		t.Fatalf("ParseFile returned error: %v", err)
	}

	if parsed.Version != 2 {
		t.Fatalf("version = %d, want 2", parsed.Version)
	}
	if len(parsed.Stages) != 1 {
		t.Fatalf("stages length = %d, want 1", len(parsed.Stages))
	}
	if got := parsed.Stages[0].Outcome; got != "Core checkout contracts are in place." {
		t.Fatalf("outcome = %q", got)
	}
}

func TestParseFileRequiresFrontmatter(t *testing.T) {
	tempDir := t.TempDir()
	planPath := filepath.Join(tempDir, "plan.md")

	content := `# Plan

No frontmatter here.
`

	if err := os.WriteFile(planPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write plan: %v", err)
	}

	_, err := ParseFile(planPath)
	if err == nil {
		t.Fatal("expected ParseFile to fail without frontmatter")
	}

	if !strings.Contains(err.Error(), "must start with YAML frontmatter") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateRequiresDetailedFields(t *testing.T) {
	plan := &File{
		Version: 2,
		Stages: []FileStage{
			{
				ID:    "foundation",
				Title: "Foundation",
			},
		},
	}

	err := Validate(plan)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "missing outcome") {
		t.Fatalf("unexpected error: %v", err)
	}
}
