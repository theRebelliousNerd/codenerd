# codeNERD Migration Pattern

## Source and target map

| Source behavior | Target |
|---|---|
| Claude skill | existing owner; governed Codex package under `.codex/skills/` when explicitly attached |
| Claude agent | `.codex/agents/<name>.toml` plus governed config registration |
| Claude slash command | owning Codex skill or reference |
| path-scoped rule | nearest `AGENTS.md` |
| command policy | trusted `.codex/rules/*.rules` |
| lifecycle hook | handler plus the single `.codex/hooks.json` owner |
| shared agent memory | preserve `.claude/agent-memory/` |
| plugin bundle | Codex plugin with manifest and deliberate discovery |

## Repository facts

- codeNERD is Go plus Mangle.
- New LLM-facing behavior is JIT prompt-atom first.
- The constitutional permission surface is
  `permitted(ActionType, Target, Payload)`.
- Important wiring surfaces are the kernel, VirtualStore, session executor,
  shard manager/registration, prompt compiler, articulation assembler, CLI,
  MCP, and tools.
- `.agents/skills/` is the official auto-discovered root.
- This environment also exposes governed `.codex/skills/` packages.
- Agents are explicitly registered in `.codex/config.toml`.
- Hooks are owned by `.codex/hooks.json`.
- Shared memory stays under `.claude/agent-memory/`.

## Model translation

Do not mechanically map Claude labels to private-looking slugs.

- demanding judgment or implementation -> `gpt-5.6`, high/xhigh
- read-heavy exploration and structured support -> `gpt-5.6-terra`,
  medium/high
- omit a pin only when inheriting the repository default is deliberate

## Compatibility duplicates

A same-name `.agents/skills` and `.codex/skills` package is allowed only for
an explicit compatibility reason. Compare support closure and validate
discovery; Codex does not merge duplicate skill packages.

