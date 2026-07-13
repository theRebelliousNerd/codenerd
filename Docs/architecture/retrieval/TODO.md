# retrieval — TODO

> Last verified: **2026-07-13**  
> Priority: P0 must ship for north-star integration; P1 quality; P2 scale/polish.

## P0 — Wiring

- [ ] Call `Model.Retriever.FindRelevantFiles` or `TieredContextBuilder.BuildContext` from `seedIssueFacts` (or session observe phase) under timeout.
- [ ] Assert `candidate_file` / `keyword_hit` / multi-tier `tiered_context_file` / `issue_context` into kernel EDB.
- [ ] Resolve paths before asserting `file_mentioned` / tier facts (reuse `findFile` logic).
- [ ] Update glass-box or context logs when sparse search runs (prove liveness).

## P1 — Correctness & hybrid

- [ ] Remove dead `FindRelevantFiles(ctx, "", …)` call in `searchKeywordFiles`.
- [ ] Fix T4 definition search to not treat regex anchors as literals.
- [ ] Inject optional embedding query for real semantic T4 with heuristic fallback.
- [ ] Add Go import expander for T3.
- [ ] Max file size + binary skip in `searchSingleKeyword`.
- [ ] Cap max hits per keyword before ranking.

## P2 — Scale & maintainability

- [ ] Shared worker pool across keywords (avoid P×P goroutines).
- [ ] Invalidate cache on workspace file writes / session hooks.
- [ ] Either implement real `rg` backend behind interface **or** delete/rename `parseRipgrepOutput` + update comments/tests (`RealRg` → `NativeScan`).
- [ ] Structured metrics (latency, cache hit rate, files walked).
- [ ] Use or remove unused `SparseRetriever.mu`.
- [ ] Expand `filePathPattern` extensions as needed (`.tsx`, `.vue`, `.kt`, …).

## P3 — Docs / tests

- [ ] Cross-package test: seed fact arity vs `schemas_knowledge.mg`.
- [ ] SIMD-tagged CI job optional.
- [ ] Keep this corpus updated when wire lands (date stamp).

## Done (historical anchors)

- Keyword extract + weights + path normalize  
- Native parallel search + word boundary  
- LRU+TTL cache with clone safety  
- Tiered builder T1–T3 + placeholder T4  
- Chat extract seed + partial T1 tier facts  
- Substantial unit/integration/race-oriented tests  
