package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadDefaultPrompt(t *testing.T) {
	repoRoot := t.TempDir()
	promptPath := filepath.Join(repoRoot, "MCP_PROMPT.md")

	const want = "# default prompt\n\nUse this prompt.\n"
	if err := os.WriteFile(promptPath, []byte(want), 0o644); err != nil {
		t.Fatalf("write prompt: %v", err)
	}

	got, err := readDefaultPrompt(repoRoot)
	if err != nil {
		t.Fatalf("readDefaultPrompt returned error: %v", err)
	}
	if got != want {
		t.Fatalf("readDefaultPrompt() = %q, want %q", got, want)
	}
}

func TestReadDefaultPromptMissingFile(t *testing.T) {
	_, err := readDefaultPrompt(t.TempDir())
	if err == nil {
		t.Fatal("expected error for missing MCP_PROMPT.md")
	}
	if !strings.Contains(err.Error(), "read default prompt") {
		t.Fatalf("unexpected error: %v", err)
	}
}
