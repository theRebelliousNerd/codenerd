# Vectryx Porter Self-Run

## Scope

Run the newly ported `claude-to-codex-porter` against Vectryx's project-local
Claude workspace surfaces. The source `.claude/` tree remained read-only.

## Surface Ledger

| Source | Role | Target | Classification | Status |
|---|---|---|---|---|
| `.claude/skills/` | Reusable workflows | Existing owner; governed packages under `.codex/skills/` | `DIRECT_TRANSLATION` / `REHOME` | Applied for eight missing packages and `corpus-build` v2 |
| `.claude/agents/` | Custom roles | `.codex/agents/*.toml` plus governed registry | `DIRECT_TRANSLATION` | Six missing specialist agents added; six `packet-*` sources mapped to existing `corpus-*` aliases |
| Claude model labels | Role-tier hints | Codex model fields | `DIRECT_TRANSLATION` | 133 TOMLs normalized: 124 Sol, 9 Terra, no live Haiku/Luna source |
| `.claude/commands/` | Shared command workflows | Existing source-command skills in active roots | `REHOME` | Intentionally unchanged; already runtime-visible |
| `.claude/rules/` | Path-scoped guidance | Root/nested `AGENTS.md`, explicit shared references, or command policy | `REHOME` | No blind bulk move; 69-rule consolidation remains a separately scoped audit |
| `.claude/hooks/` and settings hook blocks | Lifecycle guards | `.codex/hooks.json`, inline config, or plugin hooks | `UNSUPPORTED_GAP` / human checkpoint | Existing Codex PostToolUse hook preserved; no enforcement hooks enabled without confirmation |
| `.claude/plugins/` | Claude plugin bundles | Codex plugin package plus marketplace | `WRAP_AS_PLUGIN` / human checkpoint | Not installed or published; requires plugin-by-plugin scope and confirmation |
| `.claude/prompts/` | User-local prompt workflows | Owning skills or deprecated prompt exception | `REHOME` | Preserved source; no automatic promotion without invocation/ownership decision |
| `.claude/settings*.json` | Permissions, plugins, hooks, local config | Project config, user/admin config, or gap | Mixed | Secrets/permissions and user-local settings were not copied into repo config |
| `.claude/mcp.json` | MCP config | `.codex/config.toml` or plugin config | `NOT APPLICABLE` | Source file absent |

## Applied Package Set

- `corpus-build` v2 plus `PLAN-corpus-build.md` and its wiring checklist
- `corpus-comms-plumber`
- `corpus-consumables-keeper`
- `corpus-critic`
- `corpus-defense-auditor`
- `corpus-doc-auditor`
- `corpus-jules-dispatcher`
- `crucible`
- `socratic-innovation-partner`

The six corpus specialists include matching registered TOML agents and skill
attachments. The existing Socratic agent now attaches its ported skill and uses
the shared `.claude/agent-memory/` root instead of the stale `.Codex` path.

## Intentional Non-Parity

- Claude agent-local hooks have no matching per-agent Codex hook attachment in
  this runtime. The six new TOMLs state the gap and require root/corpus guard
  contracts.
- Generated `__pycache__`, `.pyc`, and `.pyo` files were excluded.
- Preserved changelogs, journals, and problem-solving threads retain historical
  Claude paths and model labels when rewriting would falsify evidence.
- Hook enablement, plugin installation/marketplace publication, root deletion,
  and permission migration remain behind explicit human checkpoints.

## Validation

See the final run output for package validation, TOML/config parsing, model-map
parity, Python compilation/tests, runtime skill discovery, collision scanning,
support-closure checks, and `git diff --check`.
