# Importing from Codex (sibling agent) — Grok policy

**`.codex/` is Codex CLI’s workspace. Grok must not edit it.**

Codex already ported high-quality `arch-propose` and `corpus-build` ecosystems there.
Grok’s job is a **one-way import** into Grok/Claude-facing trees:

| Direction | Path |
|-----------|------|
| **Source (read-only)** | `.codex/skills/{arch-propose,corpus-build,corpus-*,requirements-interrogator}/` |
| **Source (read-only)** | `.codex/agents/*.toml` (for understanding fleet roles) |
| **Destination (ours)** | `.agents/skills/` (shared discovery) |
| **Destination (ours)** | `.grok/skills/` (Grok-local discovery) |
| **Destination (ours)** | `.claude/skills/` (Claude discovery mirror) |
| **Destination (ours)** | `.grok/agents/*.md` (Grok subagent definitions) |
| **Destination (ours)** | `.grok/hooks/corpus-build/` (Grok hooks; do not touch `.codex/hooks`) |

## Import rule

1. Read Codex trees only.
2. Copy into `.agents/skills` / `.claude/skills`.
3. Never `git add` Codex-owned fleet paths under `.codex/` except pre-existing shared mirrors (if any).
4. Never modify files under `.codex/`.

## “app-discover”

No skill by that name. Closest Codex surface: `claude-to-codex-porter` discovery-vs-registration policy (also under `.codex/skills/` — read-only).
