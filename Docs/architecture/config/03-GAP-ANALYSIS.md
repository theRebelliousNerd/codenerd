# 03 — Gap analysis

> Bridge from [02-CURRENT-STATE.md](02-CURRENT-STATE.md) to
> [01-VISION.md](01-VISION.md). Cards are authoritative only in [TODO.md](TODO.md).

## Reality-to-target matrix

| Target | Current proof | Gap | Card |
|---|---|---|---|
| Secret-safe transactional persistence | **PARTIAL** tested wizard preservation, failure-safe rename and Unix `0600` exist; conflict/backup/Windows ACL/secret-reference contracts remain. | P0 | `config-safe-persistence-v1` |
| One strict boot snapshot | **PARTIAL** strict syntax and fail-closed shared boot are tested; semantic validation, immutable identity and secondary-consumer parity remain. | P0 | `config-strict-snapshot-v1` |
| Same execution bounds on every effect path | **PARTIAL** shared Cortex projects/contains all fields; campaigns copy binaries/env/directory without shared timeout/containment and dormant legacy code bypasses projection. | P0 | `config-execution-projection-v1` |
| Explain effective values without exposure | **PARTIAL** feature/provider logs exist; no field provenance, snapshot ID, retention or redacted receipt. | P1 | `config-provenance-receipt-v1` |
| Safe evolution of schemas/defaults | **PARTIAL** two live aggregates and tests exist; no no-effect migration comparison. | P3 | `config-migration-shadow-lab-v1` |

## Causal order

1. **PROPOSED UPLIFT:** verify and finish persistence first, including conflict,
   failure injection, platform modes and preservation receipts.
2. **PROPOSED UPLIFT:** finish one strict immutable snapshot and extend shared
   syntax fail-closed behavior to semantic invalidity and every consumer.
3. **PROPOSED UPLIFT:** project that snapshot into every effect path and prove
   permission remains independent.
4. **PROPOSED UPLIFT:** add provenance only after values and consumers agree.
5. **PROPOSED UPLIFT:** use receipts and redacted fixtures for the deferred lab.

## Do not mistake these for fixes

- **REJECTED:** treating a `0600` mode request as a cross-platform credential
  vault overstates what the local file abstraction proves.
- **REJECTED:** adding more `Get*` defaults does not validate hostile or
  contradictory input.
- **REJECTED:** treating `AllowedBinaries` as `permitted/3` merges resource bounds
  with constitutional authority.
- **REJECTED:** provider environment fallback after a present invalid file
  violates explicit workspace intent.
- **REJECTED:** duplicating validation in system/perception/chat would create a
  third configuration authority.

## Resolved observations

**VERIFIED CURRENT.** Missing config is an intentional first-run state; it is not
the same as malformed config. Pointer fields are appropriate where explicit
false must differ from omission. Config has no reason to own Mangle rules or
prompt atoms. These are boundaries, not backlog items.
