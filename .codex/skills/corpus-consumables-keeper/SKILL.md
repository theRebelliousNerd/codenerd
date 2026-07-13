---
name: corpus-consumables-keeper
description: >
  Keeps codeNERD generated and consumed artifacts synchronized: prompt atoms,
  embedded schemas and policies, skill mirrors, registries, generated manifests,
  and command-facing assets. Use when runtime code changed but a generated,
  embedded, or mirrored consumer may drift.
metadata:
  version: 2.0.0
  author: codeNERD
  last-verified: 2026-07-13
---

# Corpus Consumables Keeper

Audit producer-to-consumer parity for the packet.

Typical pairs:

- `internal/prompt/atoms/` -> prompt compiler selection and embedded atom data
- `internal/core/defaults/schemas*.mg` -> declarations loaded by the kernel
- `internal/core/defaults/policy/` -> policy corpus and embedded defaults
- `.claude/skills/`, `.agents/skills/`, and governed `.codex/skills/`
  mirrors when the repository intentionally maintains them
- agent TOMLs -> `.codex/config.toml` registrations
- generated manifests/indexes -> their source inputs
- CLI/MCP tool definitions -> runtime dispatch handlers

Run `scripts/consumables_parity.py --root .` for a path/reference audit, then
verify every reported mismatch manually. Regenerate with the owning tool when
one exists; do not hand-edit generated output unless its contract explicitly
allows it.

Report producer, consumer, command used, and parity verdict.

