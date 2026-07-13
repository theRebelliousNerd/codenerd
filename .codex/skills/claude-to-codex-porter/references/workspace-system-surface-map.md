# Workspace System Surface Map

Use this reference after `codex-surface-decision-tree.md` when a migration
touches more than a single skill package or agent definition.

The goal is to stop treating Claude root systems as invisible leftovers.

## Classification Labels

Assign one label to every source workspace surface:

- `DIRECT_TRANSLATION`: move into an equivalent Codex-native surface
- `REHOME`: preserve behavior, but move it into a different owning package
- `WRAP_AS_PLUGIN`: port the behavior into a repo-local plugin bundle
- `UNSUPPORTED_GAP`: no faithful Codex destination exists yet; preserve source and
  report the gap explicitly

Do not leave any touched source system unclassified.

Record the classification in the surface ledger before editing. A migration that
copies files first and classifies later is incomplete.

## Source-to-Target Map

| Claude source surface | Preferred Codex destination | Classification guidance |
|---|---|---|
| `.claude/settings.json` | project `.codex/config.toml`, user/admin config, or unsupported gap | `DIRECT_TRANSLATION` only when the setting maps cleanly at the same ownership scope |
| `.claude/settings.local.json` | `.codex/config.toml` or intentional local-only omission | `DIRECT_TRANSLATION` only for stable repo-relevant settings; otherwise `UNSUPPORTED_GAP` or intentionally unchanged |
| `.claude/mcp.json` | `.codex/config.toml` and/or `plugins/<name>/.mcp.json` | Use direct config for repo-wide MCP servers; use plugin-local MCP for plugin-shaped systems |
| `.claude/hooks/*` | command handler plus one `hooks.json`, inline `[hooks]`, or plugin-hook representation | Usually `DIRECT_TRANSLATION`; preserve a portable helper language and existing config-layer style |
| `.claude/plugins/<name>/` | `plugins/<name>/` with `.codex-plugin/plugin.json` | Usually `WRAP_AS_PLUGIN` |
| `.claude/commands/` | owning active skill/plugin package | Usually `REHOME`; slash commands import to skills |
| `.claude/rules/` | `AGENTS.md`, owning skill instructions, agent `developer_instructions`, or hook enforcement logic | Usually `REHOME`; official Codex `.rules` files are command policy only |
| `.claude/prompts/` | owning skill reference or explicit user-local custom prompt exception | Usually `REHOME`; custom prompts are deprecated |

## Hook Migration Rules

Root Claude hooks are system surfaces, not loose scripts.

When a hook is in scope:

1. Read the hook script.
2. Read the registration site in `.claude/settings.json` or the owning plugin.
3. Identify its real role:
   - blocking write guard
   - advisory warning
   - telemetry/metrics logging
   - post-run automation
   - MCP/bootstrap helper
4. Decide whether the behavior belongs in:
   - a command handler plus `.codex/hooks.json`
   - inline `[hooks]` in `.codex/config.toml`
   - `.codex/config.toml` as direct config
   - a repo-local plugin bundle
   - a migrated skill or agent instruction surface
5. Port both the script and the registration/wiring, not just the file body.

Never claim a hook is ported if only the helper moved but the activation path did
not. Review project trust, resolve repo-local scripts from the Git root, and do
not treat `PreToolUse` as a complete security boundary.

## Plugin Bundle Rules

When the source behavior is bundle-shaped, create or update:

- `plugins/<name>/.codex-plugin/plugin.json`
- optional `hooks/`
- optional `scripts/`
- optional `skills/`
- optional `assets/`
- optional `.mcp.json`
- optional `.app.json`

For a repo-distributed installable plugin, add or update
`.agents/plugins/marketplace.json` unless an existing marketplace already owns
discovery. If installation is intentionally out of scope, report the plugin as
shaped but unwired rather than calling it complete.

Use `plugin-creator` conventions for target shape if you need a local Codex plugin
layout reference.

## Command and Rule Rehoming Rules

Claude commands and rules are not automatically equivalent to a Codex root feature.
In particular, Codex `.rules` files are Starlark command execution policy files,
not path-scoped instruction Markdown.

For each command or rule:

1. Identify whether it is:
   - reusable prompt content
   - policy text
   - procedural workflow guidance
   - deterministic enforcement logic
2. Move it into the surface that will actually execute or expose it:
   - a skill package (`SKILL.md`, `references/`, `scripts/`)
   - a documented deprecated custom prompt exception when explicitly requested
   - an agent TOML `developer_instructions`
   - an `AGENTS.md` file for persistent directory-local guidance
   - a Python hook for deterministic tool-event enforcement
3. Rewrite references so the moved surface is reachable from the owning workflow.

## Validation Requirements

For root-system migrations, validate all touched surfaces that apply:

- parse `.codex/config.toml` with Python `tomllib`
- parse JSON manifests (`plugin.json`, `.mcp.json`, marketplace files)
- run `bash -n` on touched shell helpers that intentionally remain shell
- run the narrow syntax or compile check for every touched hook helper language
- confirm the project is trusted before relying on project config, hooks, or rules
- probe hook activation from a non-root working directory when command paths are
  repository-relative
- search touched Codex surfaces for stale `.claude/` paths
- verify plugin companion surfaces exist when the migration claims they were ported
- verify plugin marketplace/install wiring or document the intentional omission
- verify direct config and plugin wiring are not both missing

## Return Contract

Report root-system work explicitly:

- which Claude root surfaces were in scope
- how each one was classified
- where each one landed on the Codex side
- what remains unsupported or intentionally unchanged
- what validation ran for each migrated system surface
