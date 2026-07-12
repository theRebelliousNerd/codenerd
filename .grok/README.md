# Grok project harness for codeNERD

This directory is Grok Build's native project surface. Root `AGENTS.md` / `Claude.md` still own
repo-wide product rules. Everything here is **harness wiring**: how Grok should delegate, which
skills to load, and which subagents/personas to use.

## Layout

| Path | Purpose |
|------|---------|
| `rules/` | Always-on operational rules (loaded as project rules) |
| `agents/` | Spawnable subagent definitions (`spawn_subagent` types) |
| `personas/` | Behavioral overlays for subagents |
| `roles/` | Default capability/model for named roles |
| `skills/` | Grok-native project skills (slash-invocable) |

## Already discovered elsewhere (do not duplicate)

Grok also loads these via harness compatibility — leave them where they live:

- **Skills**: `.agents/skills/*`, `.claude/skills/*` (codenerd-builder, mangle-programming, …)
- **Claude agents**: `.claude/agents/*` (mangle-logic-architect, nerd-evolve-*)
- **Root rules**: `AGENTS.md`, `Claude.md`

## Quick use

- Inspect what Grok sees: `grok inspect`
- Slash skills: `/codenerd-builder`, `/mangle-programming`, `/go-architect`, `/check-work`, …
- Subagents: `explore`, `plan`, `mangle-logic-architect`, `go-architect`, `wiring-auditor`, …
- Agents modal: `/config-agents` or `/agents`

## After clone / new machine

1. Open the repo with Grok (`grok` in this directory).
2. Confirm trust if prompted (`/hooks-trust` or launch with `--trust`).
3. Run `grok inspect` and verify Project Instructions, Skills, and Agents sections.
