---
phase: "02_inventory"
next: "../03_transform/phase.md"
---

# Phase 02: Inventory and Compare

## Objective

Read enough source and target reality to know what must change, what must remain
unchanged, and what would be false to rewrite.

## Steps

1. Read `references/repo-migration-pattern.md`.
2. Run `python scripts/inventory_workspace.py --root .` and record active skill
   roots, duplicate names, config entries, and parse errors.
3. For each source skill package in the ledger:
   - read `SKILL.md`
   - read `CHANGELOG.md` and `CLAUDE.md` when present
   - enumerate package-root docs, `references/`, `assets/`, `scripts/`,
     `agents/openai.yaml`, metadata files, and skill-local runtime dirs
   - read only the support files needed to classify behavior, but keep every
     present support path in the ledger until it is ported, preserved as
     evidence, intentionally unchanged, or declared an unsupported gap
   - search for agent names, path roots, runtime output dirs, and validation
     requirements
4. For each source agent in the ledger:
   - read the full source body
   - capture description, model hints, skills, tools, and memory/journal roots
   - search the owning skill to confirm the agent is actually required
5. For root systems:
   - read the registration site as well as the file body
   - hooks require `.claude/settings*.json` or plugin manifest context
   - commands and prompts require discovery of how users invoke them
   - rules require deciding whether they are instruction, workflow, or command
     policy
6. Compare target surfaces:
   - existing `.agents/skills/<name>/` and `.codex/skills/<name>/`
   - existing `.codex/agents/<name>.toml`
   - `.codex/config.toml` registrations
   - `.codex/hooks.json`
   - relevant `AGENTS.md` files
   - plugin manifests
7. Mark preserved evidence explicitly when rewriting old paths would falsify
   historical research, changelogs, or interrogation transcripts.

## Gate

Proceed only when the ledger has enough evidence to distinguish active stale
instructions from preserved historical evidence.

## Failure Modes

- Copying inactive runtime history just because it exists.
- Letting the existing target package override newer source behavior.
- Converting agents that the requested skill does not actually dispatch.
- Missing support closure: comparing `SKILL.md` and `references/` while
  skipping package `scripts/` or `assets/` that the workflow reads.
- Treating duplicate skill roots as merged or assuming config enablement proves
  runtime discovery.

## Next

Read `../03_transform/phase.md`.
