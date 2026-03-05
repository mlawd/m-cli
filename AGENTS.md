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

## Orchestration Pipeline

m supports an automated implement -> review -> human-review pipeline:

### Stage Status Lifecycle

```
pending → implementing → ai-review → human-review → done
```

- **pending**: default for all stages on plan attach
- **implementing**: build agent is running
- **ai-review**: review agent is running
- **human-review**: both agents finished; humans can inspect
- **done**: human explicitly marked complete

### MCP Orchestration Tools

- **`report_stage_done`**: Called by agents when their phase is complete.
  - Parameters: `stack_name`, `stage_id`, `phase` ("implementing" or "ai_review"), `summary` (optional)
  - When `phase == "implementing"`: transitions stage to ai-review and spawns review agent
  - When `phase == "ai_review"`: transitions stage to human-review, finds next pending stage, and spawns build agent for it

- **`get_stack_run_status`**: Returns current run status of a stack including per-stage status, active stage, and elapsed time.

### Agent Harness Configuration

Global config at `~/.config/m/config.json`:
```json
{
  "agent_harness": "opencode",
  "agents": {
    "build": "build",
    "review": "review"
  }
}
```

Manage with `m config show` and `m config set`.

### Agent Definitions

Review agent definitions are shipped in `.opencode/agents/review.md` (or `.claude/agents/review.md`) and are written automatically on `m stack run` if missing.

## Maintenance Instructions

- Keep `internal/mcp/server.go` resources/tools/prompts in sync with `MCP_PROMPT.md`.
- Update both `MCP_PROMPT.md` and this `AGENTS.md` when:
  - MCP resources, tools, or prompt names change.
  - `m` command behavior changes for init/stack/stage workflows.
  - Plan format validation rules change.
  - Stage status lifecycle or orchestration tools change.
- Ensure examples and command names remain accurate and runnable.
