package cmd

import (
	"os"
	"path/filepath"
)

const reviewAgentMD = `---
name: review
description: AI code reviewer for m stage pipelines. Invoked automatically by m after implementation.
mode: subagent
hidden: true
permission:
  bash:
    "git commit *": allow
    "git push *": deny
    "git *": allow
---

You are an AI code reviewer operating as part of an automated m stack pipeline.

You will receive:
1. The stage's plan context describing what should have been implemented
2. A git diff of the committed changes for this stage

Your job:
- Review the diff against the plan context
- Fix any issues directly (wrong scope, bugs, missing validation, style)
- Commit any fixes with message prefix "review: "
- If no fixes are needed, do NOT create an empty commit
- When complete, call the report_stage_done MCP tool with phase "ai_review"

Do not implement new features. Do not modify files outside the stage's scope.
`

func writeAgentDefinitions(repoRoot, harnessName string) error {
	var agentDir string
	switch harnessName {
	case "claude":
		agentDir = filepath.Join(repoRoot, ".claude", "agents")
	default:
		agentDir = filepath.Join(repoRoot, ".opencode", "agents")
	}

	reviewPath := filepath.Join(agentDir, "review.md")
	if _, err := os.Stat(reviewPath); err == nil {
		return nil // already exists
	}

	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		return err
	}

	return os.WriteFile(reviewPath, []byte(reviewAgentMD), 0o644)
}
