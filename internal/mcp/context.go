package mcp

import (
	"fmt"
	"os"
	"strings"

	"github.com/mlawd/m-cli/internal/gitx"
	"github.com/mlawd/m-cli/internal/state"
)

type ContextSnapshot struct {
	RepoRoot     string        `json:"repo_root,omitempty"`
	IsGitRepo    bool          `json:"is_git_repo"`
	Initialized  bool          `json:"initialized"`
	CurrentStack string        `json:"current_stack,omitempty"`
	CurrentStage string        `json:"current_stage,omitempty"`
	Stacks       []state.Stack `json:"stacks,omitempty"`
	Notes        []string      `json:"notes,omitempty"`
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

	config, err := state.LoadConfig(snapshot.RepoRoot)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	stacksFile, err := state.LoadStacks(snapshot.RepoRoot)
	if err != nil {
		return nil, fmt.Errorf("load stacks: %w", err)
	}

	if _, err := os.Stat(state.Dir(snapshot.RepoRoot)); err == nil {
		snapshot.Initialized = true
	}
	snapshot.CurrentStack = strings.TrimSpace(config.CurrentStack)

	if snapshot.CurrentStack != "" {
		if stack, _ := state.FindStack(stacksFile, snapshot.CurrentStack); stack != nil {
			snapshot.CurrentStage = state.EffectiveCurrentStage(stack, repo.TopLevel)
		} else {
			snapshot.Notes = append(snapshot.Notes, fmt.Sprintf("Current stack %q is missing from stacks state.", snapshot.CurrentStack))
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

2) Create a stack when starting a new effort:
   - m stack new <stack-name> [--plan-file ./plan.yaml]
   - This auto-selects the stack.

3) If the stack was created without a plan, attach one before stage commands:
   - m stack attach-plan ./plan.yaml

4) Confirm or switch current stack:
   - m stack current
   - m stack select <stack-name>

5) Break work into ordered stages and select one:
   - m stage list
   - m stage select <stage-id>
   - m stage start-next [--no-open]
   - m stage current

6) Keep stack branches synchronized as upstream changes land:
   - m stack rebase

7) Publish the active stage branch and open/update review:
   - m stage push

8) While planning agent work:
   - Prefer one stage-focused goal at a time.
   - Keep changes scoped to the selected stage.
   - If no stage is selected, pick the earliest incomplete stage.

9) Useful guardrails for agents:
   - Read current stack/stage before proposing edits.
   - Mention which stage a change belongs to.
   - If changing stage scope, update selection first.
`)
}

func commandReference() string {
	return strings.TrimSpace(`m command reference:

- m init
  Initialize repo-local .m state for this repository.

- m stack new <stack-name> [--plan-file <file>]
  Create a stack, optionally from a YAML plan file, and select it.

- m stack attach-plan <file>
  Attach a YAML plan file to the current stack (fails if a plan is already attached).

- m stack list
  List stacks and indicate which one is current.

- m stack remove <stack-name> [--force] [--delete-worktrees]
  Remove a stack from local m state.

- m stack select <stack-name>
  Set the active stack context.

- m stack current
  Print the active stack name.

- m stack rebase
  Rebase started stage branches in order for the current stack.

- m stage list
  List stages for the current stack (requires an attached plan).

- m stage select <stage-id>
  Set the active stage for the current stack.

- m stage current
  Print the active stage id for the current stack.

- m stage start-next
  Start the next stage by creating/reusing its branch and worktree under .m/worktrees/, selecting it, and opening opencode in that worktree (use --no-open to skip).

- m stage push
  Push the current stage branch and create a PR if an open one does not exist.
`)
}

func planFormatGuide() string {
	return strings.TrimSpace(`m plan file format (YAML):

Required top-level fields:
- version: must be 1
- stages: non-empty list

Optional top-level fields:
- title: free-form string

Each stage entry:
- id: required, unique, kebab-case letters/numbers only
      regex: ^[a-z0-9]+(?:-[a-z0-9]+)*$
- title: required, non-empty string
- description: optional string

Validation rules enforced by m:
- plan version must be exactly 1
- at least one stage is required
- stage ids must match the regex above
- stage ids must be unique
- each stage must include a title

Example:

version: 1
title: Example rollout
stages:
  - id: foundation
    title: Foundation setup
    description: Optional details
  - id: api-wiring
    title: Wire API endpoints
`)
}
