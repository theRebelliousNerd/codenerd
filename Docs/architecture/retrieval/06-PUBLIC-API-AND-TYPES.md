# retrieval — Public API and Types

> Last verified: **2026-07-13**  
> All symbols live in package `retrieval` (`codenerd/internal/retrieval`).

## 1. Constructors and defaults

| Symbol | File | Signature / notes |
|--------|------|-------------------|
| `DefaultSparseRetrieverConfig` | sparse.go | `(workDir string) *SparseRetrieverConfig` |
| `NewSparseRetriever` | sparse.go | `(cfg *SparseRetrieverConfig) *SparseRetriever` — nil cfg → defaults for `.` |
| `DefaultTieredContextConfig` | tiered_context.go | `(workDir string) *TieredContextConfig` |
| `NewTieredContextBuilder` | tiered_context.go | `(cfg *TieredContextConfig) *TieredContextBuilder` — nil cfg/defaults; nil Retriever auto-built |
| `NewKeywordHitCache` | sparse.go | `(maxSize int, ttl time.Duration) *KeywordHitCache` |

## 2. Keyword extraction

| Symbol | Kind | Notes |
|--------|------|-------|
| `IssueKeywords` | struct | Primary/Secondary/Tertiary, Weights, MentionedFiles, MentionedSymbols |
| `ExtractKeywords` | func | `(issueText string) *IssueKeywords` |
| `(*IssueKeywords).AllKeywords` | method | primary→secondary→tertiary order |

## 3. Sparse search

| Symbol | Kind | Notes |
|--------|------|-------|
| `SparseRetriever` | struct | workDir, cache, maxResults, timeout, parallelism, excludes |
| `SparseRetrieverConfig` | struct | config fields listed in IMPLEMENTED_SPEC |
| `KeywordHit` | struct | FilePath, Keyword, Line, Column, Context, Count |
| `CandidateFile` | struct | ranked file with Tier + Hits |
| `(*SparseRetriever).SearchKeywords` | method | `(ctx, *IssueKeywords) ([]KeywordHit, error)` |
| `(*SparseRetriever).RankFiles` | method | `(hits, keywords, limit) []CandidateFile` |
| `(*SparseRetriever).FindRelevantFiles` | method | `(ctx, issueText, limit) ([]CandidateFile, error)` |

Unexported but important: `searchSingleKeyword`, `determineTier`, `parseRipgrepOutput`.

## 4. Cache API

| Symbol | Notes |
|--------|-------|
| `KeywordHitCache` | public type |
| `(*KeywordHitCache).Get` | clone on hit; TTL expire |
| `(*KeywordHitCache).Set` | clone; LRU insert/evict |
| `(*KeywordHitCache).Clear` | reset map + list |

## 5. Tiered context

| Symbol | Kind | Notes |
|--------|------|-------|
| `TieredContextBuilder` | struct | holds retriever + budgets |
| `TieredContextConfig` | struct | budgets + optional Retriever |
| `ContextFile` | struct | JSON tags; Content optional |
| `TieredContext` | struct | IssueText, Keywords, Files, per-tier counts |
| `(*TieredContextBuilder).BuildContext` | method | main assembly |
| `(*TieredContext).GetFilesByTier` | method | filter |
| `(*TieredContext).GetTopFiles` | method | sort by RelevanceScore |
| `(*TieredContext).GetFilePaths` | method | paths only |
| `(*TieredContext).LoadContent` | method | fill Content up to maxBytes |

## 6. Scanner

| Symbol | Notes |
|--------|-------|
| `ScanBuffer` | `(buf, keyword []byte) []int` — dual build-tag implementations |

## 7. Typical caller snippets (illustrative)

### Extract only (production chat pattern)

```go
kw := retrieval.ExtractKeywords(issueText)
// assert issue_keyword / file_mentioned facts from kw
```

### Full sparse discovery (library / intended agent path)

```go
r := retrieval.NewSparseRetriever(retrieval.DefaultSparseRetrieverConfig(workspace))
cands, err := r.FindRelevantFiles(ctx, issueText, 50)
```

### Tiered pack

```go
b := retrieval.NewTieredContextBuilder(retrieval.DefaultTieredContextConfig(workspace))
tc, err := b.BuildContext(ctx, issueText)
_ = tc.LoadContent(256 * 1024)
```

## 8. Stability notes

- Exported types are stable enough for internal `cmd/nerd` and tests; no versioning scheme.
- Changing weight numbers or tier thresholds is a **behavioral** break for activation — coordinate with `internal/context` consumers.
- `parseRipgrepOutput` is unexported; safe to rewrite without external API break.
