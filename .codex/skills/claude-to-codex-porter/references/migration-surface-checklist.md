# Migration Surface Checklist

Use this checklist at the start and end of every migration. The goal is to prevent
“good enough” ports that miss one of the real integration surfaces.

## Required Review Surfaces

Mark each one as:

- `DONE`
- `INTENTIONALLY UNCHANGED`
- `NOT APPLICABLE`

Do not leave any surface unreviewed.

## Surface Ledger

Before editing, record one row per in-scope source surface:

- Source path
- Role
- Activation event
- Target Codex surface
- Classification: `DIRECT_TRANSLATION`, `REHOME`, `WRAP_AS_PLUGIN`,
  `UNSUPPORTED_GAP`, or `PRESERVED_EVIDENCE`
- Validation command or inspection
- Status

If you cannot fill every column, stop and read
`codex-surface-decision-tree.md` before touching files.

## Source/Target Boundary

- Source `.claude/` reviewed
- Target `.codex/` updated
- No Codex-only fixes accidentally applied to `.claude/`
- Any intentional `.claude/` edits explicitly approved by the user

## Skill Package Surface

- `SKILL.md`
- `CHANGELOG.md`
- `CLAUDE.md`
- `references/`
- `assets/`
- `scripts/`
- `agents/openai.yaml`
- package metadata files

Question:

- Did every source `scripts/` and `assets/` path either land under the target
  skill package or receive an explicit preserved-evidence, intentionally
  unchanged, or unsupported-gap ledger entry?

## Root Workspace Surface

- `.claude/settings.json`
- `.claude/settings.local.json`
- `.claude/mcp.json`
- `.claude/hooks/`
- `.claude/commands/`
- `.claude/rules/`
- `.claude/prompts/`
- `.claude/plugins/`
- `.codex/rules/*.rules` only when creating experimental Starlark command policy
  files in a trusted project

Question:

- Which of these are in scope for this migration, and where does each one land on
  the Codex side?
- Is any old Claude path-scoped instruction being incorrectly placed in
  `.codex/rules/*.md`?
- Should this content be `AGENTS.md`, a hook, a skill, an agent instruction, a
  deprecated custom prompt exception, config, plugin, command policy, preserved
  evidence, or an unsupported gap?

## Runtime Working Surface

- `interrogations/`
- `research/`
- `revision-journal/`
- any other skill-local runtime artifact directories

Question:

- Does the source skill read or write this directory at runtime?

## Agent Surface

- required source `.codex/agents/*.md`
- target `.codex/agents/*.toml`
- source-provenance comments
- model and reasoning-effort mapping
- sandbox selection
- `[[skills.config]]`
- `mcp_servers`
- shared memory root handling

## Config Surface

- `.codex/config.toml` registration blocks
- description strings shown in the registry
- config parse validation
- MCP server declarations moved from Claude root config
- prompt or plugin registry files updated as part of the migration

Question:

- Is this repo hybrid and registration-complete for agents?

## Journal Surface

- `.claude/agent-memory/<agent>/`
- `.claude/agent-memory/MEMORY.md`
- real seed notes copied forward only when they actually exist
- empty roots created with `.gitkeep` where needed

## Plugin Surface

- `plugins/<name>/.codex-plugin/plugin.json`
- `plugins/<name>/hooks/`
- `plugins/<name>/scripts/`
- `plugins/<name>/skills/`
- `plugins/<name>/assets/`
- `plugins/<name>/.mcp.json`
- `plugins/<name>/.app.json`
- `.agents/plugins/marketplace.json` or an existing marketplace when repo
  distribution is part of the migration

Question:

- Is the migrated behavior actually plugin-shaped, or is it just direct config?

## Evidence Preservation Surface

- historical changelog citations
- research briefings
- interrogation transcripts
- preserved source references that would become false if rewritten

Question:

- Is the old path a stale instruction, or preserved evidence?

## Validation Surface

- markdown read-back
- TOML parse
- JSON parse
- `codex execpolicy check` for touched `.codex/rules/*.rules` files
- hook JSON path existence check
- hook behavior probe for blocking hooks
- helper-language syntax checks for every touched hook command
- hook trust and Git-root path-resolution check
- explicit note that `PreToolUse` is not a complete security boundary
- Python compile
- package asset path resolution for touched scripts or instructions
- fresh-run `AGENTS.md` / `AGENTS.override.md` discovery check from the intended CWD
- stale `.claude` search
- stale Claude-style `.codex/rules/*.md` search
- standalone-agent discovery or governed config-registration coverage
- skill-root discovery and duplicate-name coverage
- plugin manifest plus marketplace/install reachability coverage
- intentional-exception list

## Return Surface

Report all of these explicitly:

- skills touched
- agents touched
- hooks, commands, rules, prompts, or plugins touched
- config entries touched
- memory roots repointed to `.claude/agent-memory/<agent>/`
- validation commands run
- intentional exceptions
