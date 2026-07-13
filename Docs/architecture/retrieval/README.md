# retrieval — Architecture Corpus (`internal/retrieval`)

> Last verified against codebase: **2026-07-13**  
> Status: Living reference (code-grounded rewrite)  
> Language: Go (module `codenerd`)  
> Primary package: `internal/retrieval/`  
> Scale: **4** non-test Go sources (~1,488 lines) · **6** test files · **0** Mangle sources

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
  → chat.seedIssueFacts → retrieval.ExtractKeywords
  → kernel LoadFacts(issue_text | issue_keyword | file_mentioned | tiered_context_file)
  → context compressor / activation / prompt atoms
  → next_action → VirtualStore → articulation
```

The package itself does **not** assert facts or call the kernel. Callers (today: chat seed path) own transduction into Mangle EDB.

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

**Keyword extraction is production-wired into chat issue seeding; full sparse search + tiered context builder are implemented and tested but largely unwired from the live OODA loop** (`Model.Retriever` is constructed at boot and never invoked for search).
