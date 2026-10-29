# retrieval — TODO

> Last verified: **2026-09-25** (lane B wave 2, against the code)
> Priority: P0 must ship for north-star integration; P1 quality; P2 scale/polish.

> **Status, 2026-09-25.** Every item below is resolved. The checked items were
> re-verified against the code; the open follow-ups, and the unwired rows of
> `08-WIRING-AND-INTEGRATION.md` §7, are closed or decided here.
>
> | Item | Resolution | Evidence |
> |---|---|---|
> | Retrieval decided in Go, `nerd fix` never retrieved (08 §7 "session clean-loop executor hooks", "Mangle rules over candidate_file/keyword_hit") | closed `216b818` | `issue_retrieval_wanted` and `retrieval_brief_file` are kernel decisions (`internal/core/defaults/policy/retrieval.mg`, Decls in `schemas_knowledge.mg` 52.5); `retrieval.TaskRetriever` (`internal/retrieval/decisions.go`) runs one scoped pass per task turn and retracts it; `Executor.retrieveForTurn` (`internal/session/issue_retrieval.go`) hands the brief to the working loop's anchor; the factory wires it into the executor, its task clones and the spawner. Tests: `TestProcessWithIntent_FixTaskIsHandedTheKernelsRetrievalBrief`, `TestTaskRetriever_BriefFollowsTheKernelsFloor`, `TestRetrievalDecisions_DeriveInTheDomainCortex`, `TestBoot_WiresIssueRetrieverIntoSessionExecutor` |
> | Chat gate was a Go verb switch; chat issues accumulated turn over turn | closed `216b818` | `seedIssueFacts` asks `retrieval.Wanted(kernel, "/current_intent")` and seeds one live issue (`/chat_issue`) after `SupersedeIssue`; `TestSeedIssueFacts_KernelGatesAndOneIssueStaysLive` |
> | Follow-up: `SetProcessBus` never called, process bus nil | already done (stale) | `transparency.NewGlassBoxEventBus` calls `adoptProcessBus` (first writer wins, `internal/transparency/event_bus.go`), so the first bus a boot builds is the process bus; the `SeedRequest.GlassBox` comment was corrected in `216b818` |
> | Follow-up: Tier 3 TS/Rust | closed `25ec827` | `internal/retrieval/polyglot_imports.go`; `TestImportNeighbors_TypeScriptRelativeSpecifiers`, `TestImportNeighbors_RustModulesAndUsePaths`, `TestBuildContext_TypeScriptIssueFillsTheImportTier` |
> | 08 §7 "Embedding engine into T4: nothing constructs one" | already done (stale) | `SeedRequest.EmbeddingEngine` builds `NewEmbeddingSemanticSearcher` (`internal/retrieval/facts.go`); chat passes `m.embeddingEngine`, the factory passes `bctx.embeddingEngine` to the TaskRetriever |
> | 08 §7 "Campaign assault automatic sparse pass" | closed by `216b818` | campaign tasks run through `session.TaskExecutor` → `CloneForTask` → `ProcessWithIntent` (`internal/session/task_executor.go`), which now runs the kernel-gated pass; the clone inherits the retriever |
> | 08 §7 "VirtualStore action search_code" | declined | retrieval sits on the Observe/Orient edge; the model's search surface is the typed tool catalog (`find_symbol`, `find_text`, `search_code`, gated by `working_search_open`), and the kernel now hands the retrieved files over itself |
> | 08 §7 "Prompt atoms calling retrieval" | declined | a per-task file list is evidence, not instructions: it rides the anchor under a `[harness: ...]` header like the focus view, not a JIT atom |
> | 03 §2 P2.8 per-workDir inverted index | declined | the LRU keyword cache plus kernel-driven invalidation (`InvalidateFromKernel`) serve repeat passes; nothing measures the walk as the constraint. Reopen with a measurement |

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
- [x] SIMD-tagged CI job — Closed 2026-08-16. .github/workflows/ci.yml now has a `simd` job: it builds with `-tags "sqlite_vec simd"` and tests internal/embedding, internal/mangle and internal/retrieval, with GOEXPERIMENT=simd set at the job level because simd/archsimd is gated behind it. Both commands were verified locally before the job was committed.
      Adding the job is what exposed the real problem. The simd-tagged build had never compiled against the toolchain pinned in go.mod. internal/mangle/simd_intersect_amd64.go and internal/retrieval/scanner_amd64.go each constructed an archsimd vector and then read it back element by element - four (or sixteen) scalar comparisons followed by a scalar loop. They performed no vector operation at all, so they were scalar code wearing a SIMD costume, and would have been slower than the generic path because of the construction overhead. Go 1.26 rejects them outright because those vector types are opaque structs rather than indexable arrays.
      Both files were deleted, along with simd_intersect_amd64_test.go, and their generic implementations lost their build tags so IntersectSIMD and ScanBuffer are now unconditional - which is what has actually been shipping all along. Function names were kept because callers depend on them.
      internal/embedding was left untouched: its math_amd64.go is genuine SIMD using the real API (LoadFloat32x8Slice plus a horizontal reduction) and compiles cleanly. That is what the new job now protects, and it is precisely the code path that would have rotted the same way without one.
      Recorded decision: the deleted files were deliberately NOT ported to real intrinsics. Nothing measures a bottleneck here, and speculative hand-written vector code in a hot path needs a benchmark to justify it rather than an assumption. Revisit only with a measurement showing the generic path is the constraint.
- [x] Keep this corpus updated when wire lands (date stamp).

## Open follow-ups

None. The two listed here were resolved 2026-09-25: the process bus was
already adopted by `NewGlassBoxEventBus` (the claim was stale), and Tier 3
follows TS/JS and Rust imports (`25ec827`). See the status table above.

## Done (historical anchors)

- Keyword extract + weights + path normalize  
- Native parallel search + word boundary  
- LRU+TTL cache with clone safety  
- Tiered builder T1–T3 + placeholder T4  
- Chat extract seed + partial T1 tier facts  
- Substantial unit/integration/race-oriented tests  
