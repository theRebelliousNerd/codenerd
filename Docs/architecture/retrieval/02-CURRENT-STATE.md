# retrieval — Current State

> Last verified: **2026-07-13**  
> Package: `internal/retrieval/`  
> Mode: complete internal coverage of on-disk sources

## 1. Inventory summary

| Class | Count | Notes |
|-------|------:|-------|
| Non-test `.go` | 4 | sparse, tiered_context, scanner_generic, scanner_amd64 |
| Test `.go` | 6 | unit, coverage, bench, integration-tagged |
| `.mg` | 0 | Schema lives in core defaults, not here |
| Package docs in-tree | 0 | No package README/agents.md |

Approx. non-test lines: **~1,488**.

## 2. File roles

### `sparse.go` (~814 lines) — hotspot

Owns:

- Package comment + `SparseRetriever` / config / defaults / constructor
- `IssueKeywords`, `ExtractKeywords`, `AllKeywords`
- Regex extractors + `isCommonWord` stopword map
- `KeywordHit`, `CandidateFile`
- `SearchKeywords` (fan-out + cache)
- `searchSingleKeyword` (walk + workers + ScanBuffer)
- `parseRipgrepOutput` (legacy format parser)
- `isWordBoundary` / `isAlphanumeric`
- `RankFiles` / `determineTier`
- `FindRelevantFiles`
- `KeywordHitCache` LRU+TTL
- Path normalize helper

### `tiered_context.go` (~546 lines)

Owns:

- `TieredContextBuilder` + config/defaults
- `ContextFile`, `TieredContext`
- `BuildContext` orchestration
- T1 `extractMentionedFiles` / `findFile`
- T2 `searchKeywordFiles`
- T3 `expandImportGraph` / `extractImports` / `resolveImport` (Python)
- T4 `semanticExpansion` / `findSymbolDefinitions` (placeholder)
- Result helpers: `GetFilesByTier`, `GetTopFiles`, `GetFilePaths`, `LoadContent`

### `scanner_generic.go` (~29 lines)

Default `ScanBuffer` via `bytes.Index` non-overlapping search.

### `scanner_amd64.go` (~99 lines)

Optional SIMD-assisted first-byte filtering; build tags `amd64 && simd`.

## 3. Hotspots & complexity

| Hotspot | Why it matters |
|---------|----------------|
| `searchSingleKeyword` | Full-tree IO; concurrency; timeout semantics |
| `ExtractKeywords` | Production wire; false positive rate drives EDB noise |
| `RankFiles` / `determineTier` | Relevance quality |
| `searchKeywordFiles` | Dead empty-string call + dual rank path |
| `KeywordHitCache` | Shared concurrent state |
| Build-tag scanners | Must stay behaviorally compatible |

## 4. Runtime configuration defaults

From `DefaultSparseRetrieverConfig`:

```
MaxResults=100, SearchTimeout=30s, Parallelism=4,
Exclude: *.pyc, __pycache__, .git, node_modules, *.egg-info,
         .tox, .pytest_cache, *.min.js, vendor, dist, build, .venv, venv
CacheSize=1000, CacheTTL=5m
```

From `DefaultTieredContextConfig`:

```
Tier budgets 0.30/0.40/0.20/0.10, MaxTotal=50
```

## 5. Concurrency model (as implemented)

| Structure | Sync |
|-----------|------|
| `SparseRetriever.mu` | Present (`sync.RWMutex`) — **currently unused** by methods |
| Keyword fan-out | `sync.WaitGroup` + semaphore channel |
| Per-keyword workers | WaitGroup + `files` channel + mutex on hit append |
| Cache | Mutex on Get/Set/Clear (Get takes write lock for TTL delete + LRU move) |

Go 1.22+ style `wg.Go(func(){...})` used for workers.

## 6. What is live vs library-only

| Capability | Live in agent | Library/tests |
|------------|:-------------:|:-------------:|
| ExtractKeywords | yes | yes |
| SparseRetriever construct | yes (boot) | yes |
| SearchKeywords / FindRelevantFiles | no | yes |
| RankFiles | no (except via tests) | yes |
| TieredContextBuilder.BuildContext | no | yes |
| parseRipgrepOutput | no | yes |
| ScanBuffer | via search tests | yes |

## 7. External schema surface (not in package)

`internal/core/defaults/schemas_knowledge.mg` §52 defines:

- `issue_text`, `issue_keyword`, `keyword_weight`
- `keyword_hit`, `candidate_file`, `file_mentioned`
- `context_tier`, `tiered_context_file`, `issue_context`

Producer coverage today is incomplete (see gap analysis).

## 8. Known drift

1. Package comments: “ripgrep” — implementation is native Go scan.
2. Integration test names (`TestSearchKeywords_RealRg`) imply `rg` binary; they exercise the native path.
3. `Model.Retriever` field exists without consumers.
4. T4 docs say “vector similarity”; code does symbol heuristic.
