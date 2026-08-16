# retrieval — TODO

> Last verified: **2026-08-16**  
> Priority: P0 must ship for north-star integration; P1 quality; P2 scale/polish.

## P0 — Wiring *(landed 2026-08-15)*

- [x] Call `TieredContextBuilder.BuildContext` from `seedIssueFacts` under timeout
      (`retrieval.SeedIssueFacts`, budget `DefaultSeedTimeout` = 5s, independent
      of the LLM timeouts).
- [x] Assert `candidate_file` / `keyword_hit` / multi-tier `tiered_context_file` /
      `issue_context` into kernel EDB — plus `context_tier` and `keyword_weight`,
      which were declared and unproduced.
- [x] Resolve paths before asserting `file_mentioned` / tier facts (`findFile`
      results are carried on `TieredContext.ResolvedMentions` and normalized to
      workspace-relative form).
- [x] Update glass-box or context logs when sparse search runs — every pass logs
      a `SeedReport.Summary()` line and emits a `CategoryKernel` glass-box event
      on the caller's bus.

## P1 — Correctness & hybrid

- [x] Remove dead `FindRelevantFiles(ctx, "", …)` call in `searchKeywordFiles`.
- [x] Fix T4 definition search to not treat regex anchors as literals.
- [x] Inject optional embedding query for real semantic T4 with heuristic fallback
      (`SemanticSearcher` / `EmbeddingSemanticSearcher`; nil falls back to the
      definition scan).
- [x] Add Go import expander for T3 (`go_imports.go`; module-local imports only).
- [x] Max file size + binary skip in `searchSingleKeyword`.
- [x] Cap max hits per keyword before ranking.

## P2 — Scale & maintainability

- [x] Shared worker pool across keywords (avoid P×P goroutines).
- [x] Invalidate cache on workspace file writes / session hooks — driven off the
      kernel's `file_written` / `file_modified_externally` facts
      (`InvalidateFromKernel`), so no writer has to be re-plumbed.
- [x] Implement real `rg` backend behind `ScanBackend`; `parseRipgrepOutput` is
      now live code. Native remains the default (bounds live in code, no external
      binary); ripgrep is opt-in via `SparseRetrieverConfig.Backend` or
      `nerd retrieve --ripgrep`.
- [x] Structured metrics (`RetrieverMetrics`: latency, cache hit rate, files
      walked/scanned/skipped, timeouts) surfaced by `nerd retrieve --stats`.
- [x] `SparseRetriever.mu` now guards the kernel-write invalidation cursor;
      `TieredContextBuilder.mu` guards the `findFile` memo.
- [x] Expanded `filePathPattern` extensions (`.tsx`, `.jsx`, `.vue`, `.svelte`,
      `.kt`, `.kts`, `.swift`, `.cs`, `.scala`, `.mg`, …) and allowed `-`/`.` in
      path bodies.

## P3 — Docs / tests

- [x] Cross-package test: seed fact arity vs `schemas_knowledge.mg`
      (`TestSeedFacts_ShouldMatchSchemaDeclArity`).
- [ ] SIMD-tagged CI job optional — **blocked**: CI now exists (.github/workflows/ci.yml), so "no pipeline" is no longer the blocker. Adding the SIMD job was attempted and it cannot pass, because the `simd`-tagged build does not compile. Without GOEXPERIMENT=simd, `go test -tags "sqlite_vec simd" ./internal/retrieval/...` fails with `imports simd/archsimd: build constraints exclude all Go files`. With GOEXPERIMENT=simd set so archsimd resolves, it fails with `internal/mangle/simd_intersect_amd64.go:18: cannot index vA (variable of struct type archsimd.Uint64x4)`.
      The cause: Go 1.26's archsimd exposes vector types as opaque structs rather than indexable arrays, so this code has never compiled against the toolchain pinned in go.mod. The scope: three subsystems carry `simd`-tagged files - internal/embedding (math_amd64.go), internal/mangle (simd_intersect_amd64.go) and internal/retrieval (scanner_amd64.go) - and each has a generic fallback, which is what actually ships and what CI covers today. The real prerequisite is now fixing those intrinsics against the current archsimd API; the CI job is a one-line addition once the tagged build compiles. Note that a permanently red job was deliberately not added, because it would train people to ignore CI, and that this reasoning is recorded in the workflow file itself.
- [x] Keep this corpus updated when wire lands (date stamp).

## Open follow-ups

- `transparency.SetProcessBus` is never called in production, so
  `transparency.ProcessBus()` is nil in a real session. `SeedRequest.GlassBox`
  works around it for this path; `ReportDeny`'s glass-box mirror is still dark.
- Tier 3 import expansion covers Go and Python. TypeScript/Rust module
  resolution is unimplemented.

## Done (historical anchors)

- Keyword extract + weights + path normalize  
- Native parallel search + word boundary  
- LRU+TTL cache with clone safety  
- Tiered builder T1–T3 + placeholder T4  
- Chat extract seed + partial T1 tier facts  
- Substantial unit/integration/race-oriented tests  
