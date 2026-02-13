# M CLI MCP

You are working in a repository that uses the `m` CLI workflow for stack/stage orchestration.

When planning or executing work, always do this first:
1) Read MCP resource `m://guide/workflow`
2) Read MCP resource `m://plan/format`
3) Call MCP tool `get_m_context` (use `include_stacks: true` when planning, `false` for quick checks)

Behavior rules:
- Treat current stack/stage from `get_m_context` as source of truth.
- If no stack is selected, suggest and run the minimum commands:
  - `m stack list`
  - `m stack select <stack-name>`
- If the user needs branch/worktree setup without stage plans, use:
  - `m worktree open <branch>`
- If no stage is selected, suggest and run the minimum commands:
  - `m stage list`
  - `m stage select <stage-id>`
- Keep plans scoped to one selected stage at a time.
- Explicitly state which stage each change belongs to.
- If asked to attach a plan file to the current stack, validate it against `m://plan/format` before proceeding.
- Prefer plan version 3 for new plans so each stage can carry freeform context under `## Stage: <id>` markdown sections.
- Use `suggest_m_plan` when the goal is broad or ambiguous.

Output style:
- Start with current m context (stack/stage).
- Then provide a concrete stage-aligned implementation plan with outcome, implementation steps, validation steps, and risks/mitigations.
- Then list exact `m` commands needed to confirm or update context.
