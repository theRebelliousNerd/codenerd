# retrieval — Architecture Corpus (`internal/retrieval`)

> Last verified against codebase: **2026-08-15**  
> Status: Living reference (code-grounded rewrite)  
> Language: Go (module `codenerd`)  
> Primary package: `internal/retrieval/`  
> Scale: **9** non-test Go sources · **8** test files · **0** Mangle sources
> (the package consumes the Decls in `internal/core/defaults/schemas_knowledge.mg` §52)

## Scope

This corpus documents **issue-driven sparse file discovery and tiered context assembly**: keyword extraction from problem text, parallel filesystem keyword scan, hit ranking into context tiers, and optional multi-tier context packing.

It is **not**:

- Vector / embedding retrieval (`internal/embedding/`, corpus DBs)
- Context compression / activation (`internal/context/`)
- The Mangle knowledge schemas that *consume* issue facts (`internal/core/defaults/schemas_knowledge.mg`)
- Product Spec templates under `Docs/Spec/`

### Role in fact-flow

```
user input → perception Intent
  → chat.seedIssueFacts → retrieval.SeedIssueFacts (bounded budget)
      → ExtractKeywords → SparseRetriever.SearchKeywords → RankFiles
      → TieredContextBuilder.BuildContext (T1 mentions, T2 keywords,
        T3 imports, T4 semantic)
  → kernel LoadFacts(issue_text | issue_keyword | keyword_weight |
      file_mentioned | candidate_file | keyword_hit | context_tier |
      tiered_context_file | issue_context)
  → context compressor / activation / prompt atoms
  → next_action → VirtualStore → articulation
```

`internal/retrieval/facts.go` owns the transduction into Mangle EDB; it takes a
narrow `FactSink` interface (`LoadFacts([]types.Fact) error`) rather than
importing `internal/core`. Callers supply the kernel and a glass-box bus.

Fact arguments are typed against their Decl bounds, not against what is natural
in Go: ratios go through `types.PercentFromRatio` because a `/number` slot is
int64 and the kernel rejects a fractional float outright, tier constants are
`types.MangleAtom`, and name-shaped strings in `/string` slots are wrapped in
`types.MangleString`.

## Document map

| Doc | Role |
|-----|------|
| [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md) | Flagship living architecture + inventory + flows |
| [00-ALIGNMENT-VISION-REVIEW.md](00-ALIGNMENT-VISION-REVIEW.md) | North-star alignment scores with evidence |
| [01-VISION.md](01-VISION.md) | Target product/architecture vision |
| [02-CURRENT-STATE.md](02-CURRENT-STATE.md) | Precise on-disk inventory and hotspots |
| [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md) | Spec vs reality, priorities, non-gaps |
| [04-ARCHITECTURAL-PRINCIPLES.md](04-ARCHITECTURAL-PRINCIPLES.md) | Binding package principles |
| [05-INTERNAL-ARCHITECTURE.md](05-INTERNAL-ARCHITECTURE.md) | Components, data flow, state machines |
| [06-PUBLIC-API-AND-TYPES.md](06-PUBLIC-API-AND-TYPES.md) | Exported types/funcs with file refs |
| [07-DEPENDENCY-MAP.md](07-DEPENDENCY-MAP.md) | Upstream/downstream with evidence |
| [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md) | Boot, chat seed, dormant hooks |
| [09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md) | Concurrency, path safety, budgets |
| [10-TESTING-ALIGNMENT.md](10-TESTING-ALIGNMENT.md) | Tests, tags, gaps, commands |
| [11-OBSERVABILITY.md](11-OBSERVABILITY.md) | Logging categories and debug hooks |
| [12-FAILURE-MODES.md](12-FAILURE-MODES.md) | Concrete failures + mitigations |
| [TODO.md](TODO.md) | Prioritized backlog |
| [OPEN-QUESTIONS.md](OPEN-QUESTIONS.md) | Unresolved design questions |
| [_progress.md](_progress.md) | Rebuild log |

## Verify

```powershell
# One retrieval pass from the CLI, with the exact facts it asserts
nerd retrieve --facts --stats "panic in internal/core/kernel.go calling Evaluate()"

# Unit + package tests (default tags; excludes //go:build integration)
go test ./internal/retrieval/...

# Race on concurrent cache/search paths
go test -race ./internal/retrieval/...

# Integration suite (real temp trees; requires integration build tag)
go test -tags=integration ./internal/retrieval/...

# Keyword extraction micro-benchmark
go test -bench=BenchmarkExtractKeywords -benchmem ./internal/retrieval/
```

No sqlite-vec / CGO flags required for this package alone.

## Honest one-liner

**Sparse search and the four-tier context builder are production-wired into the
chat issue seed under a bounded budget, and every tier now lands in the kernel
EDB as section-52 facts** — verified by querying them back out of a real kernel
in `facts_test.go`, which is the only proof that survives Decl-bound rejection.

Known gaps: Tier 3 resolves Go and Python imports only; Tier 4 uses the
definition-scan fallback unless a `SemanticSearcher` is injected (nothing wires
one yet); there is no VirtualStore `search_code` action, so an agent cannot ask
for a retrieval pass mid-turn — only the seed path triggers one.
