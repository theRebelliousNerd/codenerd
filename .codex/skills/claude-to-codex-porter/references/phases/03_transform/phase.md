---
phase: "03_transform"
next: "../04_validate/phase.md"
---

# Phase 03: Transform by Target Surface

## Objective

Apply the smallest faithful Codex-native change for each ledger row.

## Surface Rules

### Skills

- Update the ledger-selected owning root in place. Use `.codex/skills/<name>/`
  for a new governed codeNERD package; do not create a duplicate across roots.
- Keep `SKILL.md` as a router; put operational detail in `references/`.
- Merge active support directories with the skill package: `references/`,
  `scripts/`, `assets/`, `agents/openai.yaml`, metadata files, and active
  skill-local runtime dirs. Patch hardcoded source roots inside helpers so they
  read the migrated package from its selected active root.
- Preserve helper language unless the target surface requires a rewrite. Shell,
  PowerShell, Python, NeuroLog, and data assets may remain package helpers when
  they are not repo hooks.
- Preserve `CHANGELOG.md` and `references/journal.md`.
- Do not create a new skill for a one-shot prompt template.

### Agents

- Convert source markdown agents to `.codex/agents/<name>.toml`.
- Keep identity in the agent file and workflow in the companion skill.
- Preserve source provenance comments.
- Update `.codex/config.toml` when the role belongs to codeNERD's governed fleet;
  otherwise preserve standalone discovery and state that decision.

### Hooks

- Preserve or port hook handlers into a suitable cross-platform language.
- Use one owning representation per config layer: existing `.codex/hooks.json`,
  inline `[hooks]`, or plugin hooks.
- Resolve repo-local scripts from the Git root and review project trust.
- Document unsupported event gaps instead of pretending the hook fires.

### Rules and Instructions

- Put persistent guidance in the nearest appropriate `AGENTS.md`.
- Put command execution policy in trusted `.codex/rules/*.rules` Starlark only.
- Put workflow/domain knowledge in the owning skill or agent.
- Do not write Claude path-scoped Markdown rules to `.codex/rules/*.md`.

### Config, Prompts, Plugins

- Translate only known Codex config keys into `.codex/config.toml`.
- Rehome reusable prompt templates and slash commands to an owning skill by
  default. Use a deprecated custom prompt only as an explicit one-off exception.
- Wrap bundle-shaped systems as plugins with manifests, local assets, and an
  explicit marketplace/install discovery decision.

## Gate

Proceed only when every edited file can be traced to a ledger row and every
ledger row is either done, intentionally unchanged, preserved evidence, or an
unsupported gap.

## Failure Modes

- Broad cleanup outside the requested migration.
- Stale path replacement inside historical evidence.
- Unregistered custom agents in this registry-complete repo.
- Dropped support file: a source `scripts/` or `assets/` path disappears from
  the target package without a ledger reason.

## Next

Read `../04_validate/phase.md`.

