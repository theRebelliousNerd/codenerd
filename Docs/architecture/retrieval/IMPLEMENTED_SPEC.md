# retrieval — Implemented Spec

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/retrieval/` (complete internal coverage)
> **Implementation: `internal/retrieval/` — 4 non-test .go, 6 tests, 0 .mg**


## 1. Purpose

Retrieval / knowledge lookup helpers

## 2. Source paths (1:1)

| Path | Role |
|------|------|
| `internal/retrieval/` | Primary implementation |
| `Docs/architecture/retrieval/` | This full corpus |

## 3. Implementation Status

> Living code status — **not** pre-implementation zeroing.

| Component | Status | Completion |
|-----------|--------|------------|
| Source package tree | Implemented | **90%** |
| Exported types (sampled) | Implemented | **80%** |
| Tests | Implemented | **90%** |
| Mangle local sources | N/A or global-only | **n/a** |
| Full architecture corpus | Implemented | **100%** |

**Overall (heuristic): 90%** as living package (4 src / 6 tests)

## 4. Public surface inventory

### Largest files

| Path | Lines |
|------|------:|
| `internal/retrieval/sparse.go` | 814 | source |
| `internal/retrieval/tiered_context.go` | 546 | source |
| `internal/retrieval/scanner_amd64.go` | 99 | source |
| `internal/retrieval/scanner_generic.go` | 29 | source |

### Types (sampled)

| Type | Location |
|------|----------|
| `SparseRetriever` | `internal/retrieval/sparse.go:31` |
| `SparseRetrieverConfig` | `internal/retrieval/sparse.go:44` |
| `IssueKeywords` | `internal/retrieval/sparse.go:97` |
| `KeywordHit` | `internal/retrieval/sparse.go:235` |
| `CandidateFile` | `internal/retrieval/sparse.go:245` |
| `KeywordHitCache` | `internal/retrieval/sparse.go:636` |
| `TieredContextBuilder` | `internal/retrieval/tiered_context.go:28` |
| `TieredContextConfig` | `internal/retrieval/tiered_context.go:47` |
| `ContextFile` | `internal/retrieval/tiered_context.go:104` |
| `TieredContext` | `internal/retrieval/tiered_context.go:115` |

### Functions (sampled)

| Symbol | Location |
|--------|----------|
| `ScanBuffer` | `internal/retrieval/scanner_amd64.go:12` |
| `ScanBuffer` | `internal/retrieval/scanner_generic.go:11` |
| `DefaultSparseRetrieverConfig` | `internal/retrieval/sparse.go:55` |
| `NewSparseRetriever` | `internal/retrieval/sparse.go:72` |
| `ExtractKeywords` | `internal/retrieval/sparse.go:130` |
| `AllKeywords` | `internal/retrieval/sparse.go:222` |
| `SearchKeywords` | `internal/retrieval/sparse.go:260` |
| `RankFiles` | `internal/retrieval/sparse.go:522` |
| `FindRelevantFiles` | `internal/retrieval/sparse.go:613` |
| `NewKeywordHitCache` | `internal/retrieval/sparse.go:652` |
| `Get` | `internal/retrieval/sparse.go:662` |
| `Set` | `internal/retrieval/sparse.go:688` |
| `Clear` | `internal/retrieval/sparse.go:731` |
| `DefaultTieredContextConfig` | `internal/retrieval/tiered_context.go:58` |
| `NewTieredContextBuilder` | `internal/retrieval/tiered_context.go:70` |
| `BuildContext` | `internal/retrieval/tiered_context.go:133` |
| `GetFilesByTier` | `internal/retrieval/tiered_context.go:493` |
| `GetTopFiles` | `internal/retrieval/tiered_context.go:504` |
| `GetFilePaths` | `internal/retrieval/tiered_context.go:519` |
| `LoadContent` | `internal/retrieval/tiered_context.go:528` |

## 5. Integration relevance

| Surface | Relevance |
|---------|-----------|
| Kernel | Related |
| VirtualStore | Consumer if effectful |
| Shards | Related |
| Prompt JIT | Optional |
| CLI | Related via `cmd/nerd` |
| Config | Reader |

## 6. Non-goals of this corpus revision

- Full prose rewrite of every function body
- Docs/Spec 18-file product templates (`spec-doc-sprint`)
- Implementing missing features (corpus-build implementation mode)
