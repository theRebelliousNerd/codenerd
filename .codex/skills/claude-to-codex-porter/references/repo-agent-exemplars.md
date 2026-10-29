# Repo Agent Exemplars

Use these local agents as concrete patterns when deciding how sophisticated a migrated
Codex agent should be.

## Read-Only Explorer Archetype

Files:

- `.codex/agents/arch-monitor.toml`
- `.codex/agents/roadmap-grinder-explorer.toml`

Why it matters:

- Uses `sandbox_mode = "read-only"`
- Keeps the mission narrow and evidence-driven
- Good template for agents that should inspect but not mutate

Use it when migrating:

- architecture explorers
- reviewers
- diagnostics lanes that should not write outside a transcript or report

## Journaled Writer With Skill Attachment

File: `.codex/agents/skill-research-scout.toml`

Why it matters:

- Uses `sandbox_mode = "workspace-write"`
- Includes a strong journal protocol
- Uses `[[skills.config]]` to preload a specific skill package

Use it when migrating:

- transcript writers
- specialist interrogators
- planners that always depend on one skill package

## Deep Implementation Worker

File: `.codex/agents/corpus-integration-worker.toml`

Why it matters:

- Strong write-scope rules
- Clear output contract
- Good example of a high-agency worker that still has explicit boundaries

Use it when migrating:

- bounded worker agents
- agents that produce code, results, or structured artifacts

## Source-Provenance Port Pattern

File: the actual source `.claude/agents/<name>.md` plus its destination TOML.

Why it matters:

- Preserve source provenance in comments when it materially helps maintenance;
  codeNERD does not require a provenance header on every agent
- Keep the migrated body close to the original prompt
- Good baseline pattern for `.md -> .toml` ports

Use it when migrating:

- Claude agents that already have mature prompts and only need Codex wrapping

## Registration Pattern

File: `.codex/config.toml`

Why it matters:

- This repo explicitly registers many governed custom agents in config
- Standalone TOML discovery is also valid; state which ownership model applies

Use it when migrating:

- any agent that joins the governed codeNERD fleet
