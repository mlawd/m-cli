# m

<!-- README TEST MARKER: Stage 1 -->

`m` is a local orchestration CLI for single-stack, multi-branch workflows.

## Prerequisites

- Go 1.25+

## Run locally

```bash
go run ./cmd/m
go run ./cmd/m help
go run ./cmd/m version
go run ./cmd/m init
go run ./cmd/m stack new my-stack
go run ./cmd/m stack attach-plan ./plan.yaml
go run ./cmd/m stack remove my-stack
go run ./cmd/m stack list
go run ./cmd/m stack select my-stack
go run ./cmd/m stack current
go run ./cmd/m stage list
go run ./cmd/m stage select foundation
go run ./cmd/m stage start-next
go run ./cmd/m stack rebase
go run ./cmd/m stack push
go run ./cmd/m stage push
go run ./cmd/m stage current
```

With Makefile args forwarding:

```bash
make run ARGS="init"
make run ARGS="stack new my-stack"
make run ARGS="stack attach-plan ./plan.yaml"
make run ARGS="stage list"
```

## Initialization

- `m init` creates repo-local orchestration state in `.m/`
- `m init` also appends `.m/` to `.git/info/exclude` so it stays local-only and untracked

## Stack + Stage workflow

- `m stack new <stack-name> [--plan-file <plan.yaml>]` creates a stack and auto-selects it
- `m stack attach-plan <plan.yaml>` attaches a plan to the current stack (fails if one is already attached)
- `m stack list` lists stacks and marks the selected one
- `m stack remove <stack-name> [--force] [--delete-worktrees]` removes a stack from local state
- `m stack select <stack-name>` sets current stack context
- `m stack current` prints the current stack name
- `m stage list` lists stages for the current stack (requires attached plan)
- `m stage select <stage-id>` selects a stage in the current stack
- `m stage current` prints the current stage id (empty if none)
- `m stage start-next` creates/reuses the next stage branch and worktree under `.m/worktrees/`, selects it, and opens `opencode` in that worktree with an initial prompt like `Implement stage <id>: <title>` (use `--no-open` to skip)
- `m stack rebase` rebases started stage branches in order (first onto default branch, then each onto the previous stage)
- `m stack push` pushes started stage branches in order with `--force-with-lease` and creates missing PRs
- `m stage push` pushes the current stage branch and creates a PR if one does not already exist

### Plan file format

```yaml
version: 1
title: Example rollout
stages:
  - id: foundation
    title: Foundation setup
    description: Optional details
  - id: api-wiring
    title: Wire API endpoints
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
  - `m://plan/format` (plan YAML format + validation rules)
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
- `internal/plan/` - YAML plan parser/validator
- `internal/state/` - repo-local state model + persistence
- `go.mod` - Go module metadata
