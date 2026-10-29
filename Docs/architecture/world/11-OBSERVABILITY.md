# world — Observability

> Last verified: **2026-07-13**

## Logging category

| Constant | Value | Package |
|----------|-------|---------|
| `logging.CategoryWorld` | `"world"` | `internal/logging/logger.go` |

Convenience wrappers (`internal/logging/logger_convenience.go`):

- `logging.World` — Info
- `logging.WorldDebug` — Debug
- `logging.WorldWarn` — Warn
- Errors often via `logging.Get(CategoryWorld).Error|Warn`

## Timed operations

| Name (typical) | Where |
|----------------|-------|
| `ScanWorkspace` | `fs.go` |
| `ScanDirectory` | `fs.go` |
| `FileScope.Open` | `scope.go` |

Use `logging.StartTimer(logging.CategoryWorld, name)` → `Stop` / `StopWithInfo`.

## High-value log fields (conceptual)

Scans already log:

- root path
- file/dir counts, skipped dirs
- cache hits vs misses
- language breakdown
- fact counts
- duration
- parse failures per file (warn)

Deep scan: `parsed`, fact count, duration (debug).

Holographic: siblings/signatures counts; impact query fallbacks.

LSP: projected definition/reference/diagnostic counts.

## Metrics surface

**No first-class metrics registry in-package.** Operators rely on:

1. CategoryWorld log lines
2. `ScanResult` / `IncrementalResult` / `DeepResult` returned to callers
3. `DataFlowCache` / `CacheStats.HitRate` when dataflow cache used

## Debug artifacts

- `debug_program_ERROR.mg` in package root is a **crash dump artifact**, not observability pipeline.
- FileCache disk: `.nerd/cache/manifest.json` inspectable for hash state.
- LocalStore world tables: per-path `fast`/`deep` fact blobs.

## Recommended operator playbook

| Symptom | Where to look |
|---------|----------------|
| Stale symbols | CategoryWorld scan + FileCache mtimes + ApplyIncremental logs in chat helpers |
| Slow boot | ScanDirectory duration + language breakdown + MaxConcurrency |
| Missing code_defines | EnsureDeepFacts logs; confirm `.go` only |
| Holographic empty callers | Kernel query `context_priority_file` existence |
| LSP empty | Manager indexed flag + IndexWorkspace errors |

## Gaps

- No structured span IDs correlating scan → kernel load → policy query
- No exported Prometheus counters for cache hit rate at scanner level (only dataflow cache stats type)
