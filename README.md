# m

`m` is a tiny starter CLI app written in Go, using Cobra.

Stage 1 now includes stack/worktree commands for AI-agent workflows.

## Prerequisites

- Go 1.25+

## Run locally

```bash
go run ./cmd/m
go run ./cmd/m help
go run ./cmd/m version
go run ./cmd/m greet
go run ./cmd/m greet "Go learner"
go run ./cmd/m greet --caps "Go learner"
go run ./cmd/m time
go run ./cmd/m time utc
go run ./cmd/m greet --help
go run ./cmd/m new feat/example
go run ./cmd/m stack feat/example do-thing --no-open --print-path
go run ./cmd/m clone git@github.com:org/repo.git
```

With Makefile args forwarding:

```bash
make run ARGS="greet --caps Go learner"
make run ARGS="new feat/example"
make run ARGS="stack feat/example do-thing --no-open --print-path"
```

## Stack workflow

- `m new <stack>` registers a stack (for example `feat/whatever`)
- `m stack <stack> <part>` creates/reuses the next part branch
- Part branches follow: `<stack>/<index>/<part-slug>`
- Worktree paths follow: `worktrees/<stack>/<index>/<part-slug>`
- Stack state is stored locally in `<git-common-dir>/m/stacks.json`

`m stack` opens `opencode` by default. Use `--no-open` to skip launching it.

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
- `cmd/m/cmd/` - Cobra commands (`root`, `clone`, `new`, `stack`, `greet`, `time`, `version`)
- `internal/gitx/` - git command helpers
- `internal/stacks/` - stack state + naming logic
- `internal/agent/` - `opencode` launcher
- `go.mod` - Go module metadata
