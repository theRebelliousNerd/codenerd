# TODO — `internal/diff`

> Last verified against codebase: 2026-08-15  
> Prioritized backlog for the package and its immediate integration. Docs-only session;
> items are recommendations, not scheduled work.

## P0 / P1 — Correctness & resource safety

- [x] Deep-copy `Hunks`/`Lines` on cache hit (or store immutable snapshots)  
- [x] Bound cache size (LRU / max entries / max total bytes)  
- [x] Optional content verification on cache hit. Key now carries two hashes + length per side unconditionally; `Options.VerifyCacheContent` adds exact byte comparison, with rejected hits counted in `Stats.Collisions`.

## P2 — API polish

- [x] `DiffOptions{ContextLines, DisableCache, ...}` with zero-value defaults  
- [x] Word-level spans as codeNERD types — `WordSpan`/`SpanType`; the UI now paints them instead of ignoring an `any`.  
- [x] `LineHeader` decided: UI-owned enum member, engine never emits it, enforced by test and documented at the declaration.  
- [x] `CreateDiffFromStrings` and every `DiffApprovalView` share one `uiDiffEngine`; `DiffEngineStats()` exposes its counters.

## P3 — Observability & tests

- [x] `Engine.Stats()` counters (hits, misses, binary, computes)  
- [x] Test: shallow-cache mutation fail-closed after deep-copy fix  
- [x] Test: ClearCache concurrent with ComputeDiff under `-race`  
- [x] Test: assert DiffTimeout behavior on synthetic pathological input  
- [x] Test: trailing-newline-only change representation precision  
- [x] Benchmark CI smoke — `TestBenchmarks_WhenRunAsSmoke_ShouldCompleteAndDoWork` runs every benchmark in the normal test pass (skipped under `-short`).

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
