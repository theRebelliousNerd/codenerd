# retrieval — Alignment & Vision Review

> Last verified: **2026-07-13**  
> Evidence base: `internal/retrieval/*`, `cmd/nerd/chat/{process_seed,session_boot,session_shared_boot,model_types}.go`, `internal/core/defaults/schemas_knowledge.mg`, `internal/context/compressor.go`

Scoring: **1–5** (5 = fully realized and on north star). Scores require code evidence, not aspiration.

## Dimension scores

| # | Dimension | Score | Evidence |
|---|-----------|------:|----------|
| 1 | North-star split (LLM creative / logic executive) | **3** | Extraction is heuristic (not LLM). Facts can feed kernel EDB. Full search/ranking does not yet drive `next_action` loop. |
| 2 | Transduction quality (NL → formal atoms) | **4** | `ExtractKeywords` → weighted tokens + files; chat seeds `issue_*` / `file_mentioned` / partial `tiered_context_file`. |
| 3 | Production wiring completeness | **2** | Boot constructs `SparseRetriever`; **no** post-boot call sites. Only extract path is live. |
| 4 | Scale readiness (large repos) | **3** | Parallel walk + exclude list + cache + timeouts exist; still full-tree walk per keyword (no index); can thrash huge trees. |
| 5 | Multi-language fidelity | **2** | File extensions multi-lang; symbol/import heuristics Python-skewed; T3 Python-only. |
| 6 | Semantic / hybrid retrieval | **1** | T4 placeholder; no embedding query integration. |
| 7 | Observability | **3** | `logging.Context` lines for search/tier counts; no metrics/histograms/traces. |
| 8 | Safety & resource bounds | **3** | Timeouts, cancel, cache caps, tier budgets; no max file size filter; walks can be heavy; partial results on timeout. |
| 9 | Test depth | **4** | Unit, coverage, integration tag, race-minded cache tests, cancel hang test, bench. |
| 10 | Schema / fact completeness | **2** | Rich Decls in `schemas_knowledge.mg`; producers cover a subset only. |
| 11 | Documentation honesty | **5** | This rebuild records dormant `Model.Retriever` and ripgrep drift explicitly. |
| 12 | Constitutional safety surface | **4** | Read-only FS; no action permissions; does not bypass `permitted(...)`. |

**Composite (mean): ~3.0** — solid library substrate, incomplete agent integration.

## Strengths

- Clear separation: pure package + logging only; easy to test.
- Keyword tiers + weights map cleanly onto Mangle `issue_keyword`.
- Context budgets encode an intentional relevance economy (30/40/20/10).
- Cancellation and timeout paths return errors (not silent empty).
- Cache clones prevent caller mutation of shared slices.

## Weaknesses (alignment-critical)

1. **Executive path incomplete:** Kernel never sees ranked candidates from disk search.
2. **Boot sinks memory for an unused retriever** — looks integrated, isn’t.
3. **Comment debt (“ripgrep”)** misleads operators and audits.
4. **T4 naming lies** relative to embedding-backed vision.

## Alignment actions (priority)

| Priority | Action | Expected score lift |
|----------|--------|---------------------|
| P0 | Wire `FindRelevantFiles` or `BuildContext` into issue seed / session path; assert `candidate_file` / multi-tier facts | Wiring 2→4 |
| P0 | Either use `Model.Retriever` or stop constructing it (document either choice) | Honesty / maintainability |
| P1 | Bridge T4 to embedding corpus query | Semantic 1→3+ |
| P1 | Go/TS import expanders for T3 | Multi-lang 2→3 |
| P2 | Fix empty-string T2 pre-call; rename comments away from ripgrep | Hygiene |
| P2 | File-size / binary skip filters | Scale safety |

## Verdict

Fits the **transduction** half of codeNERD’s architecture when used. Today it is **half-plugged**: extraction yes, sparse discovery and tiered packs mostly library-only. Do not score as “unused dead code” — schemas, compressor, and boot field prove intent; finish the wire.
