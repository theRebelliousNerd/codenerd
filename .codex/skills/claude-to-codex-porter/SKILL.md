---
name: claude-to-codex-porter
description: >
  Ports or repairs project-local Claude Code workspace surfaces as native Codex
  skills, custom agents, hooks, AGENTS.md guidance, rules, config, plugins, and
  support directories. Use for cross-repository Claude-to-Codex migration,
  parity repair, or syncing newer Claude-side behavior into Codex. Do not use
  for greenfield skill design or product-code language migrations.
version: 3.0.0
last-verified: 2026-07-13
---

# Claude to Codex Porter

Port behavior, not paths. The source tree explains what must survive; current
Codex runtime semantics decide where it belongs.

## Operating contract

- Treat the source repository and requested `.claude/` surfaces as read-only.
- Build a surface ledger before edits.
- Work single-threaded unless the user or an applicable skill explicitly asks
  for subagents.
- Preserve unrelated dirty-tree changes and historical evidence.
- Use official Codex docs plus local CLI/runtime probes for current surfaces.
- Keep identity in custom-agent TOML and workflow detail in skills.
- Validate activation, not only syntax.

## Surface ledger

Record one row per operational source surface:

| Source | Behavior | Activation | Codex target | Classification | Validation | Status |
|---|---|---|---|---|---|---|

Classifications:

- `DIRECT_TRANSLATION`
- `REHOME`
- `WRAP_AS_PLUGIN`
- `UNSUPPORTED_GAP`
- `PRESERVED_EVIDENCE`

## Iron rules

1. Classify behavior and activation before selecting a target.
2. Include the full skill support closure: references, scripts, assets,
   metadata, and active runtime folders.
3. Rehome Claude slash commands into skills by default.
4. Rehome durable path-scoped instructions into the nearest `AGENTS.md`.
5. Never put Markdown guidance in `.codex/rules/`; that surface is command
   execution policy.
6. Convert Claude agents to standalone `.codex/agents/<name>.toml` with
   `name`, `description`, and `developer_instructions`.
7. Register governed codeNERD agents in `.codex/config.toml`.
8. Use current documented models. Treat Claude labels as workload hints:
   demanding roles normally start at `gpt-5.6`; read-heavy support roles at
   `gpt-5.6-terra`. Do not copy Claude model names.
9. Preserve the shared memory owner at `.claude/agent-memory/`; do not
   fabricate history or a second journal tree.
10. A hook is ported only when one owning representation, its handler, trust,
    command path, and JSON-stdin behavior are validated.
11. Command hooks are the supported handler type unless current docs/runtime
    prove otherwise.
12. Hooks and `PreToolUse` are guardrails, not complete security boundaries.
13. Do not copy credentials, user-level providers, auth, telemetry routing, or
    machine-local secrets into project config.
14. Do not delete or consolidate an active skill root without explicit
    confirmation.
15. Account for every source/target difference in the ledger.

## codeNERD target policy

- `.agents/skills/` is the official auto-discovered repository skill root.
- `.codex/skills/` is also exposed in this environment and is used for
  governed packages attached by custom-agent TOMLs.
- Do not create same-name packages in both roots by accident. When explicit
  compatibility requires both, keep them synchronized or document why they
  diverge and validate actual discovery.
- Custom agents live in `.codex/agents/` and governed roles are registered in
  `.codex/config.toml`.
- Repo hooks are owned by `.codex/hooks.json`.
- Shared agent memory remains under `.claude/agent-memory/`.
- Root and scoped conventions live in `AGENTS.md`.

## Phase pipeline

At each phase, read the matching file under `references/phases/`.

1. Scope: classify every in-scope surface.
2. Inventory: find active instructions, registrations, support closure,
   collisions, runtime directories, and preserved evidence.
3. Transform: make the smallest faithful native change.
4. Validate: parse, probe, discover, and execute surface-specific checks.
5. Report: tie every change and residual to a ledger row.

Rebuild the ledger if the reason for an edit becomes unclear.

## Validation map

Use only checks matching touched surfaces:

- Markdown/frontmatter and support-closure readback for skills
- `tomllib` parse for agent TOMLs and `.codex/config.toml`
- required custom-agent field and registry checks
- attached-skill path existence
- `python -m py_compile` and focused unit tests for Python helpers
- PowerShell parser and self-tests for PowerShell helpers
- JSON parse, command-path existence, trust review, and behavior probes for hooks
- `codex debug prompt-input` or the closest available local discovery probe
- positive and negative skill trigger checks
- stale Claude-tool, source-repo path, model-label, and journal-root scans
- `git diff --check`

## Required references

- `references/phases/README.md`
- `references/codex-surface-decision-tree.md`
- `references/repo-migration-pattern.md`
- `references/migration-surface-checklist.md`
- `references/agent-conversion-checklist.md`
- `references/repo-codex-config-patterns.md`
- `references/discovery-vs-registration-policy.md`
- `references/custom-agent-option-matrix.md`

codeNERD journal entries and source-parity records remain preserved provenance.
They are not active codeNERD instructions.

## Start

Run:

```powershell
python scripts/inventory_workspace.py --root .
python -m unittest discover -s scripts -p "test_*.py" -v
```

Then read Phase 01 and build the ledger before editing.


