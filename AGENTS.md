# AGENTS

This repository provides an MCP server for `m` workflows. Agents should use MCP resources/tools to stay aligned with stack/stage context and plan format rules.

## Default MCP Prompt

The default prompt is maintained in `MCP_PROMPT.md`.

@MCP_PROMPT.md

## Workflow Policy

- Prefer `m` for stack/stage context and stack-managed branch workflows.
- Before planning or edits, check context via MCP (`get_m_context`) or `m stack current` / `m stage current`.
- Plan files are markdown documents with YAML frontmatter and must follow the v2 plan format from `m://plan/format`.
- Do not create manual branch/worktree structures when equivalent `m` commands exist.
- Use raw git only when the action is outside `m` capabilities.

## Maintenance Instructions

- Keep `internal/mcp/server.go` resources/tools/prompts in sync with `MCP_PROMPT.md`.
- Update both `MCP_PROMPT.md` and this `AGENTS.md` when:
  - MCP resources, tools, or prompt names change.
  - `m` command behavior changes for init/stack/stage workflows.
  - Plan format validation rules change.
- Ensure examples and command names remain accurate and runnable.
