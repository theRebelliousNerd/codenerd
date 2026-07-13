# retrieval — Internal Architecture

> Last verified: **2026-07-13**  
> Sources: `sparse.go`, `tiered_context.go`, `scanner_*.go`

## 1. Component diagram

```
┌──────────────────────────────────────────────────────────────┐
│                    Package retrieval                         │
│                                                              │
│  ExtractKeywords(text) ──► IssueKeywords                     │
│         │                                                    │
│         ▼                                                    │
│  SparseRetriever                                             │
│    ├─ KeywordHitCache (LRU + TTL)                            │
│    ├─ SearchKeywords ── parallel per keyword                 │
│    │      └─ searchSingleKeyword                             │
│    │            ├─ WalkDir + excludePatterns                 │
│    │            ├─ workers ── ReadFile                       │
│    │            ├─ ScanBuffer (generic | amd64+simd)         │
│    │            └─ word boundary + line map                  │
│    ├─ RankFiles / determineTier                              │
│    └─ FindRelevantFiles (extract∘search∘rank)                │
│                                                              │
│  TieredContextBuilder                                        │
│    ├─ T1 extractMentionedFiles / findFile                    │
│    ├─ T2 searchKeywordFiles ──► SparseRetriever              │
│    ├─ T3 expandImportGraph (Python)                          │
│    └─ T4 semanticExpansion (symbol heuristic)                │
│         └─ TieredContext { Files[], stats }                  │
└──────────────────────────────────────────────────────────────┘
```

## 2. Data flow — single keyword search

```
keyword
  → cache.Get?
       hit: return clone
       miss:
         ctx timeout = searchTimeout
         files chan ← WalkDir(workDir)
         workers:
           data = ReadFile
           lower = ToLower(data)
           offsets = ScanBuffer(lower, lowerKeyword)
           for offset in offsets:
             if !isWordBoundary: continue
             map to line/col/context → KeywordHit
         ctx.Err()? return hits+error
         cache.Set(keyword, hits)
```

## 3. Data flow — tiered build

```
issueText
  → ExtractKeywords
  → addedFiles map
  → T1 mentioned resolve (stat / walk suffix)
  → T2 SearchKeywords + RankFiles → ContextFile{Tier:2}
  → T3 for each existing file: extractImports → resolveImport
  → T4 for each MentionedSymbol: findSymbolDefinitions
  → TieredContext with counts
```

## 4. Key types (ownership)

| Type | File | Kind |
|------|------|------|
| `SparseRetriever` | sparse.go | Stateful service |
| `SparseRetrieverConfig` | sparse.go | Value config |
| `IssueKeywords` | sparse.go | Extract result |
| `KeywordHit` | sparse.go | Match atom |
| `CandidateFile` | sparse.go | Ranked file |
| `KeywordHitCache` | sparse.go | LRU store |
| `cacheEntry` | sparse.go | unexported |
| `TieredContextBuilder` | tiered_context.go | Stateful service |
| `TieredContextConfig` | tiered_context.go | Value config |
| `ContextFile` | tiered_context.go | Selected file |
| `TieredContext` | tiered_context.go | Full pack |

## 5. State machines

### 5.1 Cache entry lifecycle

```
absent --Set--> live --Get(valid)--> live (promoted)
live --Get(expired)--> deleted
live --evictOldest--> deleted
any --Clear--> empty
```

### 5.2 Search cancellation

```
running
  ├─ files drained + workers done + ctx ok → success (hits, nil)
  ├─ ctx DeadlineExceeded → (partial hits, timeout error)
  └─ ctx Canceled → (partial hits, cancel error)
```

Workers check `ctx.Done()` between files; in-flight `ReadFile` is not interruptible mid-read.

### 5.3 Tier selection (rank path)

```
mentioned path match? → 1
score ≥ 2.0? → 2
score ≥ 1.0? → 3
else → 4
```

## 6. Parallelism topology

```
SearchKeywords
  └── up to P concurrent searchSingleKeyword
         └── each: P file workers on shared files channel
```

Worst case ~`P²` goroutines across keywords if all cache-miss; semaphore on keyword level limits concurrent keyword searches to `P`, but each still starts `P` file workers → **P×P file workers** possible. Callers should treat this as a scale concern for high `Parallelism`.

## 7. Exclusion model

`filepath.Match(pattern, d.Name())` only — not full gitignore semantics.

- Directory match → `SkipDir`
- File match → skip file

Patterns are basename-oriented (`node_modules`, `*.pyc`).

## 8. Dual scanner architecture

```
//go:build amd64 && simd     → scanner_amd64.go
//go:build !amd64 || !simd   → scanner_generic.go
```

Both define `ScanBuffer`. Tests call the active build’s implementation.

Generic advances by full keyword length after match (non-overlapping).  
AMD64 path walks 16-byte windows checking first char then `HasPrefix` (can report denser offsets when overlapping is possible depending on keyword).

## 9. Error propagation policy

| API | Error behavior |
|-----|----------------|
| `ExtractKeywords` | Never errors |
| `SearchKeywords` | Logs per-keyword errors; returns aggregated hits, `nil` error overall (partial failure soft) |
| `searchSingleKeyword` | Returns timeout/cancel errors |
| `FindRelevantFiles` | Propagates `SearchKeywords` error (usually nil) |
| `BuildContext` | Tier2 errors logged; still returns partial `TieredContext`, `nil` error |
| `LoadContent` | Swallows per-file read errors; returns nil |

Soft-failure design prefers **partial context over hard fail** — important for agent resilience, dangerous for silent quality loss.

## 10. Extension points (practical)

| Need | Where to extend |
|------|-----------------|
| New file extensions in extract | `filePathPattern` |
| New stopwords | `isCommonWord` map |
| Language import graph | new helpers parallel to `extractImports` |
| Embedding T4 | replace body of `semanticExpansion` |
| Alternate backend (rg) | interface behind `searchSingleKeyword` |
