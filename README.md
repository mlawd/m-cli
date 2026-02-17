# m

`m` is a local orchestration CLI for single-stack, multi-branch workflows.

## Prerequisites

- Go 1.25+

## Run locally

```bash
go run ./cmd/m
go run ./cmd/m help
go run ./cmd/m version
go run ./cmd/m init
go run ./cmd/m status
go run ./cmd/m stack new my-stack --type feat
go run ./cmd/m stack attach-plan ./plan.md
go run ./cmd/m stack remove my-stack
go run ./cmd/m stack list
go run ./cmd/m stack current
go run ./cmd/m stage list
go run ./cmd/m stage select foundation
go run ./cmd/m stage open
go run ./cmd/m stage open --next
go run ./cmd/m worktree open feature/no-plan --no-open
go run ./cmd/m prompt default
go run ./cmd/m stack sync
go run ./cmd/m stack push
go run ./cmd/m stage push
go run ./cmd/m stage current
go run ./cmd/m worktree list
go run ./cmd/m worktree prune
```

With Makefile args forwarding:

```bash
make run ARGS="init"
make run ARGS="stack new my-stack --type feat"
make run ARGS="stack attach-plan ./plan.md"
make run ARGS="stage list"
```

## Initialization

- `m init` creates repo-local orchestration state in `.m/`
- `m init` also appends `.m/` to `.git/info/exclude` so it stays local-only and untracked
- `m status` prints a quick snapshot of repo/worktree + current m stack/stage context

## Stack + Stage workflow

- `m stack new <stack-name> [--type <feat|fix|chore>] [--plan-file <plan.md>]` creates a stack
- `m stack attach-plan <plan.md>` attaches a plan to the inferred current stack (fails if one is already attached)
- `m stack list` lists stacks and marks the inferred current one when available
- `m stack remove <stack-name> [--force] [--delete-worktrees]` removes a stack from local state
- `m stack current` prints the inferred stack name (from workspace path, or the only stack in repo root)
- most `m stack` and `m stage` commands accept `--stack <stack-name>` to override inference from cwd
- `m stage list` lists stages for the current stack (requires attached plan)
- `m stage select <stage-id>` selects a stage in the current stack
- `m stage current` prints the current stage id (empty if none)
- `m stage open` opens stage worktrees:
  - default: interactively selects stack and stage
  - `--next`: starts/opens the next stage in the current stack (with initial prompt)
  - `--stage <id>`: starts/opens a specific stage id in the current stack
  - `--no-open`: creates/reuses branch/worktree and selects context without launching `opencode`
- stage worktrees are managed under `.m/stacks/<stack>/<stage>`
- `m worktree open <branch> [--base <branch>] [--path <dir>] [--no-open]` creates/reuses a branch worktree without requiring stack plan stages
- `m worktree list` lists linked git worktrees and annotates stack/ad-hoc ownership
- `m worktree prune` runs `git worktree prune`, removes orphan directories under `.m/worktrees/`, and clears stale stage worktree references
- `m stack sync` prunes merged stage PRs from local stack state, removes their worktrees and local branches, then rebases remaining started stage branches in order (`--no-prune` keeps all stages and performs rebase-only behavior)
- `m stack push` pushes started stage branches in order with `--force-with-lease` and creates missing PRs
- `m stage push` pushes the current stage branch and creates a PR if one does not already exist
- `m prompt default` prints the default MCP prompt (`MCP_PROMPT.md`)

### Ad-hoc worktree flow (no plan required)

```bash
# Create/reuse branch from repo default branch and open opencode there
m worktree open feature/no-plan

# Create branch from a specific base branch
m worktree open feature/release-fix --base release/1.2

# Use a custom worktree path and skip opening opencode
m worktree open chore/docs-refresh --path ./worktrees/docs-refresh --no-open
```

### Plan file format

`m` supports two plan versions:
- `version: 2` (legacy detailed schema in frontmatter)
- `version: 3` (hybrid schema with freeform stage context in markdown body)

For `version: 3`, include `## Stage: <stage-id>` sections in markdown. These sections carry prompt-like context for each stage and are preserved into stage state.

```markdown
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
Preserve existing defaults from checkout settings and keep current pricing fallback behavior.
Do not alter API contracts in this stage.

## Stage: api-wiring
Wire handlers through the foundation interfaces. Reuse existing request validation semantics
and keep response shapes backward compatible.
```

## Build a binary

```bash
make build
./bin/m help
```

## MCP server (for agents)

Run an MCP stdio server so AI agents can read `m` workflow docs and live stack/stage context:

```bash
go run ./cmd/m mcp serve
```

The server exposes:

- resources:
  - `m://plan/format` (plan markdown/frontmatter format + validation rules)
  - `m://guide/workflow` (planning guidance)
  - `m://commands/reference` (command quick reference)
  - `m://state/context` (JSON snapshot of current repo stack/stage context)
- tools:
  - `get_m_context`
  - `suggest_m_plan`
- prompt:
  - `plan_with_m`

Use your MCP client config to launch `m mcp serve` as a stdio server.

### Configure OpenCode

Add an `opencode.json` file in this repo (or update your global config at `~/.config/opencode/opencode.json`) with a local MCP entry:

```json
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "m_cli": {
      "type": "local",
      "enabled": true,
      "command": ["m", "mcp", "serve"]
    }
  },
  "instructions": ["AGENTS.md"]
}
```

If you are developing `m` from source and have not run `make install`, use:

```json
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "m_cli": {
      "type": "local",
      "enabled": true,
      "command": ["go", "run", "./cmd/m", "mcp", "serve"]
    }
  },
  "instructions": ["AGENTS.md"]
}
```

After saving config, restart OpenCode. The MCP tools/resources from `m` will be available to agents.

## Install globally

```bash
make install
m help
```

## Project layout

- `cmd/m/main.go` - CLI bootstrap/entrypoint
- `cmd/m/cmd/` - Cobra commands (`root`, `init`, `stack`, `stage`, `version`)
- `internal/gitx/` - git command helpers
- `internal/localignore/` - repo-local ignore helpers (`.git/info/exclude`)
- `internal/plan/` - plan parser/validator (markdown + YAML frontmatter)
- `internal/state/` - repo-local state model + persistence
- `go.mod` - Go module metadata
