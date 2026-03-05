# AGENTS

This repository provides an MCP server for `m` workflows. Agents should use MCP resources/tools to stay aligned with stack/stage context and plan format rules.

## Default MCP Prompt

The default prompt is maintained in `MCP_PROMPT.md`.

@MCP_PROMPT.md

## Workflow Policy

- Prefer `m` for stack/stage context and stack-managed branch workflows.
- When creating stacks, prefer setting an explicit stack type when known (`m stack new <name> --type feat|fix|chore`).
- For stack maintenance, prefer `m stack sync` (or `m stack sync --no-prune` for rebase-only behavior).
- For ad-hoc work that does not use plan stages, prefer `m worktree open <branch>`.
- Before planning or edits, check context via MCP (`get_m_context`) or `m stack current` / `m stage current`.
- Plan files are markdown documents with YAML frontmatter and must follow the versioned format documented in `m://plan/format`.
- Prefer plan version 3 (hybrid) for new plans so each stage has freeform markdown context under `## Stage: <id>`.
- Do not create manual branch/worktree structures when equivalent `m` commands exist.
- Use raw git only when the action is outside `m` capabilities.

## Automated Stack Pipeline

`m stack run` starts an automated implement → review → human-review pipeline.

### Stage status lifecycle

```
pending → implementing → ai-review → human-review → done
```

- `pending`: default; stage has not been started yet
- `implementing`: build agent is running in the stage worktree
- `ai-review`: review agent is running
- `human-review`: both agents finished; humans can inspect
- `done`: explicitly marked complete

### Agent callback: `report_stage_done`

When an agent finishes its phase, it **must** call the `report_stage_done` MCP tool:

```
tool: report_stage_done
params:
  stack_name: <string>  # required
  stage_id:   <string>  # required
  phase:      "implementing" | "ai_review"  # required
  summary:    <string>  # optional
```

- `phase: implementing` → transitions stage to `ai-review` and spawns the review agent
- `phase: ai_review` → transitions stage to `human-review` and spawns the build agent for the next pending stage (or marks the stack complete)

### Agent configuration

Global config lives at `~/.config/m/config.json`:

```json
{
  "agent_harness": "opencode",
  "agents": {
    "build":  "build",
    "review": "review"
  }
}
```

Manage with `m config show` / `m config set <key> <value>`.

Valid `agent_harness` values: `opencode`, `claude`.

### Watching a run

```
m stack run    # kick off pipeline
m stack watch  # follow progress (Ctrl-C detaches, does not stop the run)
```

## Maintenance Instructions

- Keep `internal/mcp/server.go` resources/tools/prompts in sync with `MCP_PROMPT.md`.
- Update both `MCP_PROMPT.md` and this `AGENTS.md` when:
  - MCP resources, tools, or prompt names change.
  - `m` command behavior changes for init/stack/stage workflows.
  - Plan format validation rules change.
  - The stage status lifecycle changes.
- Ensure examples and command names remain accurate and runnable.
