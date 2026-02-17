package mcp

import (
	"fmt"
	"os"
	"strings"

	"github.com/mlawd/m-cli/internal/gitx"
	"github.com/mlawd/m-cli/internal/state"
)

type ContextSnapshot struct {
	RepoRoot         string        `json:"repo_root,omitempty"`
	IsGitRepo        bool          `json:"is_git_repo"`
	Initialized      bool          `json:"initialized"`
	CurrentStack     string        `json:"current_stack,omitempty"`
	CurrentStackType string        `json:"current_stack_type,omitempty"`
	CurrentStage     string        `json:"current_stage,omitempty"`
	Stacks           []state.Stack `json:"stacks,omitempty"`
	Notes            []string      `json:"notes,omitempty"`
}

func BuildContextSnapshot(cwd string, includeStacks bool) (*ContextSnapshot, error) {
	repo, err := gitx.DiscoverRepo(cwd)
	if err != nil {
		return &ContextSnapshot{
			IsGitRepo: false,
			Notes: []string{
				"Current directory is not inside a git repository.",
				"Run the MCP server from a git repo to expose m stack/stage context.",
			},
		}, nil
	}

	snapshot := &ContextSnapshot{
		RepoRoot:  gitx.SharedRoot(repo.TopLevel, repo.CommonDir),
		IsGitRepo: true,
	}

	stacksFile, err := state.LoadStacks(snapshot.RepoRoot)
	if err != nil {
		return nil, fmt.Errorf("load stacks: %w", err)
	}

	if _, err := os.Stat(state.Dir(snapshot.RepoRoot)); err == nil {
		snapshot.Initialized = true
	}

	snapshot.CurrentStack, snapshot.CurrentStage = state.CurrentWorkspaceStackStageByPath(snapshot.RepoRoot, repo.TopLevel)
	if snapshot.CurrentStack == "" {
		snapshot.CurrentStack, snapshot.CurrentStage = state.CurrentWorkspaceStackStage(stacksFile, repo.TopLevel)
	}

	if snapshot.CurrentStack == "" && !state.IsLinkedWorktree(repo.TopLevel, snapshot.RepoRoot) && len(stacksFile.Stacks) == 1 {
		snapshot.CurrentStack = strings.TrimSpace(stacksFile.Stacks[0].Name)
		snapshot.CurrentStackType = state.NormalizeStackType(stacksFile.Stacks[0].Type)
		snapshot.CurrentStage = state.EffectiveCurrentStage(&stacksFile.Stacks[0], repo.TopLevel)
	}

	if snapshot.CurrentStack != "" && snapshot.CurrentStackType == "" {
		if stack, _ := state.FindStack(stacksFile, snapshot.CurrentStack); stack != nil {
			snapshot.CurrentStackType = state.NormalizeStackType(stack.Type)
		}
	}

	if includeStacks {
		snapshot.Stacks = stacksFile.Stacks
	}

	if !snapshot.Initialized {
		snapshot.Notes = append(snapshot.Notes, "No local m state found yet; run `m init` and create/select a stack.")
	}

	return snapshot, nil
}

func planningGuide() string {
	return strings.TrimSpace(`Use this workflow to plan and execute work with m:

1) Initialize local state once per repo:
   - m init
   - m status

2) Create a stack when starting a new effort:
   - m stack new <stack-name> [--type feat|fix|chore] [--plan-file ./plan.md]
   - Set --type when known to improve defaults for branch/PR/changelog semantics.
   - Stack context is inferred from workspace location.

3) If the stack was created without a plan, either:
   - attach one before stage commands: m stack attach-plan ./plan.md
   - or start ad-hoc work without stages: m worktree open <branch>
   - inspect all linked worktrees: m worktree list
   - prune stale entries/orphans: m worktree prune

4) Confirm current inferred stack:
   - m stack current
   - Inference source: .m/stacks/<stack>/<stage> workspace, or the only stack in repo root.

5) Break work into ordered stages and select one:
   - m stage list
   - m stage select <stage-id>
   - m stage open
   - m stage open --next [--no-open]
   - m stage open --stage <stage-id> [--no-open]
   - m stage current

6) Keep stack branches synchronized as upstream changes land:
   - m stack sync
   - m stack sync --no-prune  (rebase-only)

7) Publish a full stack (force-with-lease) when needed:
   - m stack push

8) Publish the active stage branch and open/update review:
   - m stage push

9) While planning agent work:
    - Prefer one stage-focused goal at a time.
    - Keep changes scoped to the selected stage.
    - For new plans, prefer version 3 and capture prompt-like stage context under "## Stage: <id>".
    - If no stage is selected, pick the earliest incomplete stage.

10) Useful guardrails for agents:
   - Read inferred stack/stage before proposing edits.
   - Mention which stage a change belongs to.
   - If changing stage scope, update selection first.
`)
}

func commandReference() string {
	return strings.TrimSpace(`m command reference:

- m init
  Initialize repo-local .m state for this repository.

- m status
  Print repo/worktree context and the effective current m stack/stage.

- m stack new <stack-name> [--type feat|fix|chore] [--plan-file <file>]
  Create a stack, optionally from a markdown plan file (YAML frontmatter).

- m stack attach-plan <file>
  Attach a markdown plan file to the current stack (fails if a plan is already attached).

- m stack list
  List stacks and indicate the inferred current one when available.

- m stack remove <stack-name> [--force] [--delete-worktrees]
  Remove a stack from local m state.

- m stack current
  Print the inferred current stack name.

Global stack override:
- Most m stack and m stage commands accept --stack <stack-name> to override cwd inference.

- m stack sync
  Prune merged stage PRs from local stack state, remove their worktrees and local branches, then rebase remaining started stage branches in order.
  Use --no-prune for rebase-only behavior.

- m stack push
  Push started stage branches in order with --force-with-lease and create PRs when missing.

- m stage list
  List stages for the current stack (requires an attached plan).

- m stage select <stage-id>
  Set the active stage for the current stack.

- m stage current
  Print the inferred stage id for the current stack.

- m stage open
  Open stage worktrees. Default is interactive stack/stage selection; use --next for next-stage flow or --stage <id> for explicit stage selection. Use --no-open to skip launching opencode.
  Stage worktrees are created under .m/stacks/<stack>/<stage>.

- m stage push
  Push the current stage branch and create a PR if an open one does not exist.

- m worktree open <branch> [--base <branch>] [--path <dir>] [--no-open]
  Create/reuse an ad-hoc branch worktree under .m/worktrees/<branch> without requiring stack stage plans.

- m worktree list
  List linked git worktrees and annotate stack/ad-hoc ownership.

- m worktree prune
  Run git worktree prune, delete orphan directories under .m/worktrees/, and clear stale stage worktree references.

- m prompt default
  Print the default MCP prompt from MCP_PROMPT.md.
`)
}

func planFormatGuide() string {
	return strings.TrimSpace(`m plan file format (Markdown + YAML frontmatter):

Required file structure:
- File must start with YAML frontmatter delimited by ---
- Frontmatter must include:
  - version: must be 2 or 3
  - stages: non-empty list
- Markdown body after frontmatter is optional

Optional top-level frontmatter fields:
- title: free-form string

Each stage entry requires:
- id: unique, kebab-case letters/numbers only
      regex: ^[a-z0-9]+(?:-[a-z0-9]+)*$
- title: non-empty string

Version-specific requirements:

- version: 2 (legacy detailed schema)
  - outcome: non-empty string describing done state
  - implementation: non-empty list of non-empty strings
  - validation: non-empty list of non-empty strings
  - risks: non-empty list of objects with:
    - risk: non-empty string
    - mitigation: non-empty string

- version: 3 (hybrid schema)
  - Frontmatter keeps stage identity and ordering.
  - Markdown body must include a section for each stage using:
      ## Stage: <stage-id>
  - The content under each stage heading is treated as freeform stage context
    (prompt-like details such as defaults, interactions, assumptions, constraints).
  - Every declared stage must have a non-empty stage context section.

Validation rules enforced by m:
- plan version must be 2 or 3
- at least one stage is required
- stage ids must match the regex above
- stage ids must be unique
- each stage must include all required fields for its version
- for version 3, markdown stage sections must map to declared stage ids

Example (version 3):

---
version: 3
title: Checkout rollout
stages:
  - id: foundation
    title: Foundation setup
  - id: api-wiring
    title: Wire API endpoints
---

## Stage: foundation
Preserve existing defaults from checkout settings and keep the current
pricing fallback behavior. Do not alter API contracts in this stage.

## Stage: api-wiring
Wire handlers through the foundation interfaces. Reuse existing request
validation semantics and keep response shapes backward compatible.
`)
}
