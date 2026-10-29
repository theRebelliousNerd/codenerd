# Codex Surface Decision Tree

Use this reference before moving any Claude workspace surface. The source path is
only a clue. The target is determined by what the content must do at runtime.

## Ground Truth

Codex surfaces are not one-for-one replacements for Claude surfaces:

- `AGENTS.md` or `AGENTS.override.md`: persistent project or directory-local
  instructions. Codex discovers them once per run from the project root to the
  launch CWD; the closest active file wins, override files take precedence in a
  directory, and the combined project-document limit defaults to 32 KiB.
- Hook command handlers plus one owning `hooks.json`, inline `[hooks]`, or plugin
  representation: lifecycle checks, telemetry, and mechanical reminders. Keep
  one representation per config layer to avoid merge warnings. Non-managed
  hooks require trust review; only command handlers run today.
- `.codex/rules/*.rules`: experimental Starlark command execution policy consumed
  by `codex execpolicy` from a trusted project config layer. Use only for shell
  command allow/ask/deny policy. Do not put Markdown instructions here.
- Skill package in the active repository root: codeNERD's governed packages and
  attachments currently use `.codex/skills/<name>/`, while current Codex also
  discovers `.agents/skills/<name>/`. Duplicate names are not merged.
- `.codex/agents/<name>.toml`: custom agent identity and execution stance. Keep
  operational workflow in the companion skill unless the agent must carry a small,
  role-specific mandate.
- Codex skills: preferred target for reusable slash-command workflows and shared
  prompt-driven procedures.
- Custom prompts: deprecated, user-local, explicit-invocation prompt templates.
  Use only when the user explicitly asks for that exception.
- `.codex/config.toml`: Codex runtime configuration, feature flags, MCP servers,
  hooks, and repo agent registry entries. Project config is ignored until the
  repository is trusted, and user/admin-owned settings must not be forced into
  project config.
- `plugins/<name>/`: bundle-shaped systems with their own manifest, MCP server,
  hooks, prompts, or skills.

Current hook events from the 2026-06-27 manual refresh:

- `SessionStart`
- `PreToolUse`
- `PermissionRequest`
- `PostToolUse`
- `PreCompact`
- `PostCompact`
- `UserPromptSubmit`
- `SubagentStart`
- `SubagentStop`
- `Stop`

Matcher behavior is event-specific. `UserPromptSubmit` and `Stop` ignore
matchers; `SessionStart` uses startup/resume/clear/compact; compact events use
manual/auto; tool events match tool names.

## Classification Algorithm

For every source file or directory, answer these questions in order:

1. What behavior must survive: instruction, workflow, identity, command policy,
   lifecycle script, runtime config, prompt template, or bundled extension?
2. What event activates it: session start, directory scope, skill invocation,
   subagent dispatch, shell command check, hook event, slash command, MCP call, or
   explicit human use?
3. Is deterministic enforcement required, or is advisory guidance enough?
4. Does Codex expose the activation surface today? If not, classify it as an
   `UNSUPPORTED_GAP` and preserve/document the source rather than pretending it
   was ported.
5. Which validation proves the target works?

## Decision Table

| Source content role | Codex target | Validation |
|---|---|---|
| Directory-local instructions | nearest active `AGENTS.md` or `AGENTS.override.md` on the launch chain | fresh run from the intended CWD; keep under `project_doc_max_bytes` |
| Universal repo guidance | root `AGENTS.md` | verify no duplication with narrower `AGENTS.md` files |
| Procedural workflow | owning active skill root; `.codex/skills/<name>/` for new governed codeNERD packages | read-back, `codex debug prompt-input`, and trigger/negative-trigger check |
| Agent identity or stance | `.codex/agents/<name>.toml` | `tomllib` parse and config registration check |
| Shell command allow/deny policy | `.codex/rules/*.rules` beside the active project config | trust check plus `codex execpolicy check --rules <file> -- <command>` |
| Tool lifecycle guard | command handler plus one JSON, inline TOML, or plugin representation | language syntax check; command path exists; trust and behavior probe |
| Claude slash command prompt | owning active skill package or reference; deprecated user-local prompt only by explicit exception | skill discovery and trigger/negative-trigger check, or documented prompt exception |
| MCP/runtime config | `.codex/config.toml` or plugin `.mcp.json` | `tomllib` or JSON parse; server name referenced correctly |
| Plugin-shaped bundle | `plugins/<name>/` | plugin manifest parse plus marketplace/install reachability decision |
| Historical evidence | preserve in place or copy with provenance | mark as preserved evidence, not stale instruction |

## Hard No Mappings

- Do not migrate Claude path-scoped Markdown rules into `.codex/rules/*.md`.
- Do not change a working hook helper language merely to force Python. Validate
  the retained executable on every supported platform.
- Do not claim a hook is migrated unless the handler and one owning JSON, inline
  TOML, or plugin registration exist and activate, or the missing event is an
  explicit unsupported gap.
- Do not claim security parity from `PreToolUse` alone; it is a guardrail, not a
  complete enforcement boundary.
- Do not copy Claude settings wholesale into `.codex/config.toml`; translate only
  keys with a known Codex equivalent.
- Do not convert a shared Claude command into a custom prompt by default. Codex
  imports slash commands to skills; prompts are deprecated user-local exceptions.

## Surface Ledger

Before editing, create a ledger in your notes with one row per source surface:

| Source | Role | Activation event | Target | Classification | Validation | Status |
|---|---|---|---|---|---|---|

Allowed classifications:

- `DIRECT_TRANSLATION`: faithful native Codex target exists.
- `REHOME`: behavior survives in a different owning surface.
- `WRAP_AS_PLUGIN`: behavior is bundle-shaped and needs a plugin package.
- `UNSUPPORTED_GAP`: no faithful Codex event or target exists yet.
- `PRESERVED_EVIDENCE`: old path is historical evidence, not active instruction.

Do not edit until every in-scope source surface has a target and validation plan.

## Routing Examples

- Claude `.claude/rules/frontend.md` with `paths:` and style guidance:
  `REHOME` to `web/AGENTS.md` or the owning frontend skill, not `.rules`.
- Claude hook blocking `cargo test`: `DIRECT_TRANSLATION` to
  a validated handler plus the repository's active hook representation.
- Claude rule saying "never run rm -rf": command policy belongs in a `.rules`
  file under `.codex/rules/` and must be trust-reviewed and tested with
  `codex execpolicy check`.
- Claude command that expands a long reusable review prompt: `REHOME` to a
  focused skill package or owning skill reference. Use a deprecated custom
  prompt only when the user explicitly asks for a one-off prompt surface.
- Claude plugin with hooks, MCP, and commands: `WRAP_AS_PLUGIN`.
