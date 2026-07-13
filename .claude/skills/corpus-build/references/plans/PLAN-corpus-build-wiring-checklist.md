# Corpus-build wiring plan

The authoritative candidate list is `../surfaces.yaml`; the operational explanation is `../02-integration-surface-checklist.md`.

## Procedure

1. Create a run manifest with `subsystem`, `integration_points`, `touched_paths`, and requirement IDs.
2. Run `scripts/verify_surfaces.py --manifest <manifest> --json`.
3. Have `corpus-wiring-auditor` classify candidates as REQUIRED/OPTIONAL/N-A/BLOCKED.
4. Gather registration citations and executable oracles for REQUIRED candidates.
5. Apply worker intents serially to contested files.
6. Re-run the surface report and the targeted packages.
7. Attach repo-level gate failures without laundering unrelated failures into a pass.

## Required invariants

- All Mangle predicates are declared before use.
- Dangerous actions derive `permitted(...)`; default deny remains intact.
- LLM-facing behavior is JIT prompt atoms plus selection logic.
- External effects route through VirtualStore or an explicitly governed runtime boundary.
- Shards are registered and lifecycle-safe, including cancellation and result ownership.
- CLI/MCP/tool surfaces call the same core capability rather than parallel implementations.
- Generated, embedded, mirrored, and registry consumers are synchronized.
- Architecture status is reconciled only after test evidence.

## Evidence row

```json
{
  "surface_id": "A6-shard-registration",
  "verdict": "REQUIRED_PASS",
  "citations": ["internal/shards/registration.go:42"],
  "commands": ["go test -count=1 ./internal/shards/..."],
  "residuals": []
}
```
