# TODO — `internal/diff`

> Last verified against codebase: 2026-07-13  
> Prioritized backlog for the package and its immediate integration. Docs-only session;
> items are recommendations, not scheduled work.

## P0 / P1 — Correctness & resource safety

- [x] Deep-copy `Hunks`/`Lines` on cache hit (or store immutable snapshots)  
- [x] Bound cache size (LRU / max entries / max total bytes)  
- [ ] Optional content verification on cache hit (lengths + secondary hash)

## P2 — API polish

- [x] `DiffOptions{ContextLines, DisableCache, ...}` with zero-value defaults  
- [ ] Word-level spans as codeNERD types (stop leaking `diffmatchpatch.Diff` in public API)  
- [ ] Document or deprecate unused `LineHeader` production gap  
- [ ] Align `CreateDiffFromStrings` with view-local engine (avoid dual-cache surprise)

## P3 — Observability & tests

- [x] `Engine.Stats()` counters (hits, misses, binary, computes)  
- [x] Test: shallow-cache mutation fail-closed after deep-copy fix  
- [x] Test: ClearCache concurrent with ComputeDiff under `-race`  
- [x] Test: assert DiffTimeout behavior on synthetic pathological input  
- [x] Test: trailing-newline-only change representation precision  
- [ ] Benchmark CI smoke (optional)

## Explicit non-TODOs

- Do **not** add Mangle Decl surface for its own sake  
- Do **not** add filesystem I/O  
- Do **not** add Vectryx/product-specific fields  
- Do **not** claim package unused without grepping `cmd/nerd/ui`

## Completed / already true

- [x] sergi-backed line-level diffs  
- [x] Binary NUL short-circuit  
- [x] Context clamp  
- [x] Concurrent ComputeDiff coverage  
- [x] Primary consumer DiffApprovalView  
