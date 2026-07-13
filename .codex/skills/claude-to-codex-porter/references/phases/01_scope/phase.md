---
phase: "01_scope"
next: "../02_inventory/phase.md"
---

# Phase 01: Scope and Surface Ledger

## Objective

Define the migration boundary and classify every in-scope source surface before
editing.

## Steps

1. Read `references/codex-surface-decision-tree.md`.
2. Identify the exact user request:
   - skill package sync
   - agent conversion
   - root workspace migration
   - plugin migration
   - mixed migration
3. List every source surface in scope. Include root systems when mentioned or
   implied:
   - `.claude/settings*.json`
   - `.claude/mcp.json`
   - `.claude/hooks/`
   - `.claude/commands/`
   - `.claude/rules/`
   - `.claude/prompts/`
   - `.claude/plugins/`
   - source skills, agents, memory, prompts, scripts, and runtime folders
   - for every source skill package, its support closure: package-root docs,
     `references/`, `scripts/`, `assets/`, `agents/openai.yaml`, metadata files,
     and skill-local runtime directories
4. Build the surface ledger:

| Source | Role | Activation event | Target | Classification | Validation | Status |
|---|---|---|---|---|---|---|

5. Classify each row as `DIRECT_TRANSLATION`, `REHOME`, `WRAP_AS_PLUGIN`,
   `UNSUPPORTED_GAP`, or `PRESERVED_EVIDENCE`.

## Gate

Proceed only when every source surface has a target, classification, validation
plan, and edit boundary.

## Failure Modes

- Source-path bias: treating `.claude/rules/` as if it belongs in `.codex/rules/`.
- Fake completion: moving a script without wiring the activation path.
- Partial skill port: updating `SKILL.md` while leaving active `scripts/` or
  `assets/` source-only.
- Silent unsupported gap: creating inert files for Codex events that do not exist.

## Next

Read `../02_inventory/phase.md`.
