// Package agentfiles embeds the default agent definition files for the
// m stack run pipeline and provides helpers to write them to the repo.
package agentfiles

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed opencode_review.md
var opencodeReview []byte

//go:embed claude_review.md
var claudeReview []byte

// EnsureOpenCode writes the opencode review agent definition to
// <repoRoot>/.opencode/agents/review.md if it does not already exist.
func EnsureOpenCode(repoRoot string) error {
	return ensureFile(filepath.Join(repoRoot, ".opencode", "agents", "review.md"), opencodeReview)
}

// EnsureClaude writes the claude review agent definition to
// <repoRoot>/.claude/agents/review.md if it does not already exist.
func EnsureClaude(repoRoot string) error {
	return ensureFile(filepath.Join(repoRoot, ".claude", "agents", "review.md"), claudeReview)
}

func ensureFile(path string, content []byte) error {
	if _, err := os.Stat(path); err == nil {
		return nil // already exists
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create directory for %s: %w", path, err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
