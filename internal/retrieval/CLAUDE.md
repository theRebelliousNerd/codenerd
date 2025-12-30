# internal/retrieval - Sparse File Discovery

This package provides efficient file discovery for large codebases using keyword-based search (ripgrep) without loading the entire repository into memory.

**Related Packages:**
- [internal/context](../context/CLAUDE.md) - IssueActivationContext consuming tiered files
- [internal/shards/researcher](../shards/researcher/CLAUDE.md) - Research using sparse retrieval
- [internal/world](../world/CLAUDE.md) - Scanner for deep analysis

## Architecture

The retrieval package provides:
- **SparseRetriever**: Fast keyword-based file discovery via ripgrep
- **TieredContextBuilder**: Progressive context through 4 relevance tiers
- **Designed for 50,000+ file repositories** (SWE-bench scale)

## File Index

| File | Description |
|------|-------------|
| `sparse.go` | `SparseRetriever` using ripgrep for fast keyword-based file discovery. Exports `SparseRetriever`, `SparseRetrieverConfig` (max results, timeout, parallelism, excludes), `KeywordHitCache` for result caching, and `DefaultSparseRetrieverConfig()`. |
| `tiered_context.go` | `TieredContextBuilder` progressively building context through 4 tiers. Exports `TieredContextBuilder`, `TieredContextConfig` with budget allocation (30%/40%/20%/10%), and tier-specific methods for mentioned files, keyword matches, import neighbors, and semantic expansion. |
| `sparse_test.go` | Unit tests for SparseRetriever keyword search and caching. Tests ripgrep integration and result filtering. |

## Key Types

### SparseRetrieverConfig
```go
type SparseRetrieverConfig struct {
    WorkDir         string
    MaxResults      int           // 100 default
    SearchTimeout   time.Duration // 30 seconds
    Parallelism     int           // 4 parallel ripgrep processes
    ExcludePatterns []string      // *.pyc, __pycache__, .git, etc.
    CacheSize       int
    CacheTTL        time.Duration
}
```

### TieredContextBuilder Budgets
```go
// Tier 1 (30%): Explicitly mentioned files from issue text
// Tier 2 (40%): Files matching extracted keywords
// Tier 3 (20%): Import neighbors of Tier 1-2 files
// Tier 4 (10%): Semantic expansion (vector similarity)
```

## Tiered Context Algorithm

| Tier | Budget | Source |
|------|--------|--------|
| 1 | 30% | Files explicitly mentioned in issue text |
| 2 | 40% | Files matching extracted keywords via ripgrep |
| 3 | 20% | Import/dependency neighbors of Tier 1-2 files |
| 4 | 10% | Semantic expansion via vector similarity |

## Dependencies

- `ripgrep` (external CLI tool)
- `internal/logging` - Structured logging

## Testing

```bash
go test ./internal/retrieval/...
```

---

**Remember: Push to GitHub regularly!**
