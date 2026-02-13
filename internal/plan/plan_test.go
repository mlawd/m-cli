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

func TestParseFileV3StageContext(t *testing.T) {
	tempDir := t.TempDir()
	planPath := filepath.Join(tempDir, "plan.md")

	content := `---
version: 3
title: Checkout rollout
stages:
  - id: foundation
    title: Foundation setup
  - id: api-wiring
    title: API wiring
---

## Stage: foundation
Carry forward existing order defaults and keep pricing behavior identical.

## Stage: api-wiring
Endpoints should reuse foundation interfaces. Keep request validation strict.
`

	if err := os.WriteFile(planPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write plan: %v", err)
	}

	parsed, err := ParseFile(planPath)
	if err != nil {
		t.Fatalf("ParseFile returned error: %v", err)
	}

	if parsed.Version != 3 {
		t.Fatalf("version = %d, want 3", parsed.Version)
	}
	if got := strings.TrimSpace(parsed.Stages[0].Context); got == "" {
		t.Fatal("expected context for stage foundation")
	}
	if got := strings.TrimSpace(parsed.Stages[1].Context); !strings.Contains(got, "request validation") {
		t.Fatalf("unexpected stage context: %q", got)
	}
}

func TestValidateV3RequiresContext(t *testing.T) {
	plan := &File{
		Version: 3,
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
	if !strings.Contains(err.Error(), "missing context section") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseFileV3UnknownStageContext(t *testing.T) {
	tempDir := t.TempDir()
	planPath := filepath.Join(tempDir, "plan.md")

	content := `---
version: 3
title: Checkout rollout
stages:
  - id: foundation
    title: Foundation setup
---

## Stage: unknown-stage
This should fail because the stage id is not declared in frontmatter.
`

	if err := os.WriteFile(planPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write plan: %v", err)
	}

	_, err := ParseFile(planPath)
	if err == nil {
		t.Fatal("expected ParseFile to fail for unknown stage context")
	}
	if !strings.Contains(err.Error(), "unknown stage") {
		t.Fatalf("unexpected error: %v", err)
	}
}
