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
go run ./cmd/m stack new my-stack --plan-file ./plan.yaml
go run ./cmd/m stack list
go run ./cmd/m stack select my-stack
go run ./cmd/m stack current
go run ./cmd/m stage list
go run ./cmd/m stage select foundation
go run ./cmd/m stage current
```

With Makefile args forwarding:

```bash
make run ARGS="init"
make run ARGS="stack new my-stack --plan-file ./plan.yaml"
make run ARGS="stage list"
```

## Initialization

- `m init` creates repo-local orchestration state in `.m/`
- `m init` also appends `.m/` to `.git/info/exclude` so it stays local-only and untracked

## Stack + Stage workflow

- `m stack new <stack-name> --plan-file <plan.yaml>` creates a stack from YAML and auto-selects it
- `m stack list` lists stacks and marks the selected one
- `m stack select <stack-name>` sets current stack context
- `m stack current` prints the current stack name
- `m stage list` lists stages for the current stack
- `m stage select <stage-id>` selects a stage in the current stack
- `m stage current` prints the current stage id (empty if none)

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
