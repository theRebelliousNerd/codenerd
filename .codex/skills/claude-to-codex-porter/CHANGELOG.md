# Claude to Codex Porter Changelog

## v3.0.0 (2026-07-13)

- Ported the Vectryx migration package into codeNERD and replaced Vectryx-specific architecture, model, path, and governance assumptions with live codeNERD evidence.
- Added a codeNERD cross-repository migration pattern covering official `.agents/skills` discovery, governed `.codex/skills` agent attachments, current custom-agent TOMLs, and command lifecycle hooks.
- Self-ran the porter to build the corpus-build and arch-propose Codex fleets, supporting micro-skills, registry wiring, subagent memory injection, and lifecycle telemetry.
- Preserved Claude sources and historical porter evidence; rejected stale write-scope/build hooks that conflicted with codeNERD's current root test/build contract.

## v2.4.2 (2026-07-11)

- Added the validated global-memory migration pattern: Claude global agent and
  project memory records rehome through Codex's native user-level memory intake,
  with a user-level `SessionStart` hook for idempotent synchronization.
- Preserved project and agent scope without source-vendor labels, excluded
  transcripts and credentials, and documented append-only deletion semantics.

## v2.4.1 (2026-07-11)

- Self-ran against the upgraded Vectryx Claude workspace and synchronized the complete post-2026-07-09 operational delta.
- Refreshed `corpus-build`, `arch-propose`, and `arch-templates`, including support references, Storyworld weave coverage, and active evolution evidence.
- Ported and registered eight native `arch-propose-*` Codex agents with the Vectryx Sol/Terra role mapping.
- Added Codex-native todo migration and corpus fleet lifecycle hooks while recording exact per-subagent token metering as an unsupported runtime gap.
- Preserved `.claude/agent-memory/` as the shared corpus and left unchanged Claude rules, prompts, commands, plugin bundles, and local settings in their existing classified state.

## v2.4.0 (2026-07-09)

- Ported the complete 23-file NeuroLog package into Vectryx's governed
  `.codex/skills` surface and registered both the package and `SKILL.md` paths
- Replaced NeuroLog-specific root, model, registration, hook, plugin, and agent
  exemplar assumptions with Vectryx runtime and governance evidence
- Added the requested Vectryx Claude-label baseline: `opus` -> `gpt-5.6-sol`,
  `sonnet` -> `gpt-5.6-terra`, and `haiku` -> `gpt-5.6-luna`
- Ran the porter against Vectryx: synchronized `corpus-build` v2, added six
  missing specialist skill+agent pairs plus `crucible` and
  `socratic-innovation-partner`, and normalized 133 governed agent TOMLs to 124
  Sol / 9 Terra from their Claude source roles
- Refreshed active guidance against current official Codex docs for skill roots,
  custom agents, hooks, rules, plugins, project trust, and `AGENTS.md` discovery
- Added activation, safety, human-checkpoint, evaluation, and self-improvement
  contracts
- Added deterministic mixed-root workspace inventory with unit tests and
  machine-readable migration eval cases
- Recorded two late-arriving, untracked NeuroLog fleet-maintenance scripts as
  intentional non-parity rather than copying destructive repo-specific helpers
- Preserved all prior changelog and flat-journal content as historical evidence

## v2.3.1 (2026-06-28)

- Restored `.agents/skills` as the canonical repo-local Codex skill tree after
  `.codex/skills` consolidation
- Made skill package support closure explicit: ports must inventory, merge, and
  validate `scripts/`, `assets/`, `references/`, metadata, and active runtime
  support paths, not just `SKILL.md`
- Removed stale guidance and config blocks that disabled or blocked
  `.agents/skills`

## v2.3.0 (2026-06-27)

- Refreshed target-selection guidance against the current Codex manual
- Replaced stale hook guidance with current behavior: hooks are enabled by
  default under `[features].hooks`, Windows command overrides use
  `commandWindows`/`command_windows`, and only command hook handlers run today
- Updated Claude slash-command routing: shared reusable commands import to
  Codex skills; deprecated custom prompts are explicit user-local exceptions
- Updated skill and subagent reference notes for current package shape, model
  guidance, optional custom-agent fields, and plugin packaging
- Added the GLM-formulate migration run to the porter journal as a repeatable
  example of command-hook plus skill/agent/config migration

## v2.2.0 (2026-06-04)

- Changed the repo-local Codex skill target from `.agents/skills/<name>/` to
  `.codex/skills/<name>/`
- Added a hard porter rule that `.agents/skills` folder paths are disabled in
  `.codex/config.toml`
- Added a hard porter rule that `.codex/hooks/block-agents-directory-access.py`
  must remain wired to prevent Codex from reading or writing `.agents/`
- Updated agent skill attachment examples to point at `.codex/skills`

## v2.1.1 (2026-06-01)

- Changed the active agent memory migration paradigm: `.claude/agent-memory/`
  is now the shared Claude Code and Codex memory corpus
- Updated agent-conversion guidance to point Codex TOML agents at
  `.claude/agent-memory/MEMORY.md` and their direct
  `.claude/agent-memory/<name>/` folder
- Added an explicit guardrail against creating or refreshing parallel
  `.codex/agentJournals/` memory copies

## v2.1.0 (2026-04-22)

- Added `codex-surface-decision-tree.md` so migrations classify behavior and
  activation event before choosing a target surface
- Required a surface ledger before root workspace edits
- Added validation gates for `.rules` command policy, Python hook wiring,
  `AGENTS.md` placement, and stale Claude-style rule Markdown
- Reframed root-system migration as surface routing instead of path rewriting
- Refactored `SKILL.md` into a lean phase router and added
  `references/phases/**/phase.md` files with explicit gates

## v2.0.1 (2026-04-22)

- Clarified that official Codex `.rules` files are Starlark command execution
  policies, not Claude-style path-scoped instruction Markdown
- Updated root hook migration guidance to target `.codex/hooks/*.py` plus
  `.codex/hooks.json` for repo-wide hooks
- Redirected Claude rule migrations toward `AGENTS.md`, hooks, skills, or agent
  instructions depending on the enforcement surface

## v2.0.0 (2026-04-13)

Expanded the skill from package-only migration to full-workspace migration.

- Updated the activation contract to cover hooks, commands, rules, prompts,
  plugin bundles, and root Claude config files in addition to skills, agents,
  and journals
- Added `references/workspace-system-surface-map.md` for root-level source/target
  mapping and unsupported-gap classification
- Updated the workflow to require explicit handling of `.claude/settings.json`,
  `.claude/settings.local.json`, `.claude/mcp.json`, `.claude/hooks/`,
  `.claude/commands/`, `.claude/rules/`, `.claude/prompts/`, and
  `.claude/plugins/`
- Added validation requirements for JSON manifests, shell hook syntax, plugin
  bundle surfaces, and stale root-surface references

## v1.2.0 (2026-03-23)

Added anti-underdoing guardrails.

- Added `references/discovery-vs-registration-policy.md`
- Added `references/migration-surface-checklist.md`
- Updated the workflow to require a migration surface matrix before edits
- Updated the workflow to require all applicable surfaces be marked done,
  intentionally unchanged, or not applicable before completion

## v1.1.0 (2026-03-23)

Expanded the reference layer and migration contract.

- Added repo-level Codex config coverage via `references/repo-codex-config-patterns.md`
- Added a custom-agent option matrix covering `sandbox_mode`, `mcp_servers`,
  `skills.config`, and registration behavior
- Added local advanced agent exemplars so ports can match explorer, writer, and
  worker archetypes instead of using a lowest-common-denominator template
- Updated the workflow to require `.codex/config.toml` registration when the repo
  uses explicit agent-registration blocks

## v1.0.0 (2026-03-23)

Initial release.

- New migration skill for porting repository-local `.claude` skill and agent surfaces
  into `.codex`
- Repo-local override documenting the active `.agents/skills` destination
- Agent conversion workflow for `.codex/agents/*.md` to `.codex/agents/*.toml`
- Journal-root rewrite guidance from `.claude/agent-memory/` to
  `.codex/agentJournals/`
- Validation checklist for TOML parsing, Python script compilation, and stale-path
  detection
- Heading-aligned reference notes from the official OpenAI Codex `skills` and
  `subagents` docs
- Repo migration pattern reference based on existing local migration examples and the
  checked-in methodology port prompt
