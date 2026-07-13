---
phase: "04_validate"
next: "../05_report/phase.md"
---

# Phase 04: Validate by Target Surface

## Objective

Prove the migrated surfaces are executable, discoverable, or intentionally
documented as gaps.

## Required Checks

Run only the checks that match touched surfaces:

- Skill Markdown: read back touched `SKILL.md`, `CLAUDE.md`, `CHANGELOG.md`, and
  phase/reference files.
- Skill support closure: compare source and target package support paths for
  `references/`, `scripts/`, `assets/`, `agents/openai.yaml`, metadata, and
  active runtime dirs; every omitted source support path needs an intentional
  exception in the ledger/report.
- Agent TOML: parse touched `.codex/agents/*.toml` with `tomllib`.
- Config TOML: parse `.codex/config.toml` with `tomllib`.
- Agent registry: compare `.codex/agents/*.toml` names with
  `[agents.<name>]` blocks through `config_file` when the governed fleet owns the
  role; validate standalone discovery otherwise.
- Python hooks/scripts: run `python -m py_compile`.
- Non-Python package helpers: run the narrow syntax or smoke check available for
  touched shell, PowerShell, NeuroLog, JSON, YAML, or generated fixtures.
- Assets: verify touched scripts or instructions that reference package-local
  assets resolve those paths under the target skill package.
- Hook wiring: verify every selected JSON, inline TOML, or plugin command path
  exists when resolved from the Git root.
- Blocking hooks: run at least one JSON-stdin behavior probe.
- Hook trust: prove the project layer is trusted and do not claim `PreToolUse`
  alone is a complete enforcement boundary.
- `.codex/rules/*.rules`: run `codex execpolicy check --rules <file> -- <command>` with an
  allowed example and a blocked/ask example.
- `AGENTS.md`: launch a fresh run from the intended CWD and verify root-to-CWD
  discovery, override precedence, and project-doc budget.
- Stale references: search touched surfaces for markdown agent references that
  should be TOML, old `.codex/agentJournals/` roots, and Claude-style Markdown
  in `.codex/rules/`. Do not flag active `.claude/agent-memory/` references;
  that is the canonical shared memory corpus.
- Skills: use `codex debug prompt-input` to verify the selected root is visible
  without unexplained duplicate names.
- Plugins: parse manifests and verify companion surfaces plus marketplace/install
  reachability.

## Gate

Proceed only when every ledger row has validation evidence or an explicit
unsupported-gap note.

## Failure Modes

- "Looks right" without tool output.
- Valid file body with missing registry/wiring.
- Passing Markdown read-back but wrong activation surface.
- Passing Markdown read-back while a required script or asset is absent from the
  target skill package.

## Next

Read `../05_report/phase.md`.
