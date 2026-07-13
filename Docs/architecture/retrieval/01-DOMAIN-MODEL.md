# retrieval — Domain Model

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/retrieval/` (complete internal coverage)
> **Implementation: `internal/retrieval/` — 4 non-test .go, 6 tests, 0 .mg**


## Package

`internal/retrieval/`

## Exported types (sampled, up to 40)

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

## Exported functions/methods (sampled, up to 30)

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

## Mangle surface

| Artifact | Count |
|----------|------:|
| Package-local `.mg` | 0 |

| Path | Lines |
|------|------:|
| — | 0 |

Global kernel schemas/policy (when this package participates):

- `internal/core/defaults/schemas.mg`
- `internal/core/defaults/policy/`

## Fact-flow placement

```
user input → perception → user_intent → kernel(core/mangle) → next_action
  → VirtualStore / shards / tools → articulation
```

This package: **Retrieval / knowledge lookup helpers**
