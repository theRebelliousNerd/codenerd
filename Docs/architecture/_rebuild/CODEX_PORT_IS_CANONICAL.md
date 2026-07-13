# Codex port is the canonical ecosystem

As of 2026-07-13, the **codeNERD-native** ports under `.codex/` are authoritative for:

- `arch-propose` (v3)
- `corpus-build` (v3)
- fleet micro-skills (`corpus-critic`, `corpus-comms-plumber`, …)
- `requirements-interrogator`
- Codex agent TOMLs under `.codex/agents/`
- Codex hooks under `.codex/hooks/` + `.codex/hooks.json`
- registry in `.codex/config.toml`

These were rewritten for codeNERD (fact-flow, Mangle, JIT, registration, verification ladder), not search-replaced from Vectryx.

## Multi-harness layout

| Harness | Skill root | Agent definitions |
|---------|------------|-------------------|
| Codex | `.codex/skills/` | `.codex/agents/*.toml` + `config.toml` registry |
| Grok / shared | `.agents/skills/` (synced from Codex) | `.grok/agents/*.md` (full bodies) + skills frontmatter |
| Claude | `.claude/skills/` (synced) | `.claude/agents/*.md` |

When updating the ecosystem: edit **Codex** first, then re-sync to `.agents` / `.claude`.

## Not found as a skill

There is **no** skill named `app-discover`. Closest related surfaces:

- `.codex/skills/claude-to-codex-porter/` (migration + discovery vs registration policy)
- `references/discovery-vs-registration-policy.md`

If “app-discover” meant a different package, it is not in this tree under that name.
