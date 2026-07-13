# sqlpragmas — TODO

> Last verified: **2026-07-13**  
> Prioritized backlog. Docs-only rebuild does not implement these.

## P0 — Keep true

- [ ] When changing `pragmasFor`, update unit + integration tests in the same PR.  
- [ ] When adding a profile, update this corpus (`IMPLEMENTED_SPEC`, API, failure modes).  
- [ ] Never add mid-layer imports to this package.

## P1 — Process / hygiene

- [ ] Periodic audit: product `sql.Open` sites without `ApplyDefaultPragmas` (or explicit exception comment).  
- [ ] Prefer `sqlpragmas` import in new mid-layer packages that must not touch `store`.

## P2 — Product quality

- [ ] Optional modernc.org/sqlite integration test (build tag) to catch reject sets early.  
- [ ] Document multi-conn pool guidance next to major store openers (or shared helper).  
- [ ] Consider Debug log including profile name on failure only.

## P3 — Ergonomics

- [ ] `EnableForeignKeys(db *sql.DB)` helper for schemas ready to enforce FKs.  
- [ ] Idempotency tests for BulkBuild / Query / ReadOnly.  
- [ ] Named Go constants for cache/mmap sizes (readability) without changing values.

## P4 — Aspirational

- [ ] Config/env overrides for host class (laptop vs workstation).  
- [ ] `database/sql` connector hook helper for per-connection apply.  
- [ ] Metrics counter for pragma failures (behind observability flag).

## Explicitly not planned

- Returning `error` from `ApplyDefaultPragmas` without a repo-wide migration.  
- Mangle predicates for pragma application.  
- Automatic profile selection from file path heuristics.
