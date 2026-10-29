# retrieval — Observability

> Last verified: **2026-07-13**

## 1. Logging

The package uses `codenerd/internal/logging` convenience helpers, primarily:

```go
logging.Context("SparseRetriever: ...")
logging.Context("TieredContextBuilder: ...")
```

These map to `CategoryContext` (`"context"`) via `internal/logging/logger_convenience.go` (`Context` → Info on CategoryContext; related Debug/Warn/Error helpers exist but are not heavily used here).

### Log call sites (behavioral)

| Message pattern | When |
|-----------------|------|
| `SparseRetriever: searching %d keywords` | SearchKeywords start |
| `SparseRetriever: search error: %v` | Per-keyword errors drained |
| `SparseRetriever: found %d total hits` | SearchKeywords end |
| `SparseRetriever: extracted keywords - primary=%d, secondary=%d, tertiary=%d, files=%d` | FindRelevantFiles |
| `TieredContextBuilder: Tier 1 - %d explicitly mentioned files` | After T1 |
| `TieredContextBuilder: Tier 2 search error: %v` | T2 failure |
| `TieredContextBuilder: Tier 2 - %d keyword match files` | After T2 |
| `TieredContextBuilder: Tier 3 - %d import neighbor files` | After T3 |
| `TieredContextBuilder: Tier 4 - %d semantic expansion files` | After T4 |

Boot path (outside package) logs:

```
Initializing sparse retriever...
```

via chat boot `logStep` / CategoryBoot — **does not prove search activity**.

## 2. Metrics

**None implemented** (no counters, histograms, or prometheus hooks in package).

Recommended future metrics:

| Metric | Labels |
|--------|--------|
| `retrieval_search_seconds` | outcome=ok\|timeout\|cancel |
| `retrieval_hits_total` | |
| `retrieval_cache_hit_ratio` | |
| `retrieval_files_walked_total` | |
| `retrieval_tier_files` | tier=1..4 |

## 3. Tracing / glass box

No Glass Box events emitted from retrieval. Selection reasons live on `ContextFile.SelectionReason` only if builders are used and results inspected in-process.

Vision: emit transparency events when files enter context for glass-box UI (`internal/transparency`).

## 4. Debug techniques

1. **Unit isolation:** construct retriever on a tiny temp dir; call `FindRelevantFiles` with known tokens.  
2. **Logging:** enable context category logs in workspace logging config.  
3. **Inspect EDB:** after chat seed, `nerd query` / kernel query for `issue_keyword` / `file_mentioned` (proves extract wire).  
4. **Absence proof for search:** if only extract facts exist and no `candidate_file` / multi-tier files, sparse search did not run.  
5. **Integration tag:** `go test -tags=integration ./internal/retrieval/ -v`.

## 5. Structured data already available

`TieredContext` JSON tags support dumping:

```json
{
  "issue_text": "...",
  "keywords": {...},
  "files": [{"file_path":"...","tier":2,"relevance_score":1.2,"selection_reason":"..."}],
  "tier1_count": 1,
  "tier2_count": 4,
  "total_files": 5
}
```

Useful for campaign artifacts if callers serialize it (not automatic today).

## 6. Observability gaps

- No correlation ID with chat turn / issue ID in log lines  
- Soft search errors only logged, not returned  
- Idle retriever produces **zero** runtime logs after boot  
- No cache hit/miss logging  
