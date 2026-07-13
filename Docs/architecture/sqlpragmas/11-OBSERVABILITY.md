# sqlpragmas — Observability

> Last verified: **2026-07-13**

## Logging

| Aspect | Value |
|--------|-------|
| Category | `logging.CategoryStore` (`"store"`) |
| API | `logging.Get(logging.CategoryStore)` |
| Success path | **No log** |
| Failure path | `Debug("pragma %q failed: %v", p, err)` |
| File | `internal/sqlpragmas/pragmas.go` in `ApplyDefaultPragmas` |

### Why Debug, not Warn/Error

Package comment: modernc rejects some pragmas mattn accepts; treating that as Error would spam every open on pure-Go paths. Debug keeps signal available when store debugging is enabled without frightening operators on expected rejects.

### What is logged

- Full PRAGMA statement string (constant content — not user DSN)  
- Underlying `error` from `db.Exec`  

### What is not logged

- Profile name / numeric profile  
- DSN / file path  
- Which call site applied  
- Success metrics  

## Metrics

**None** in this package.

No counters for:

- pragmas applied  
- pragmas failed  
- per-profile open counts  

## Tracing / spans

**None.**

## Debug workflow for operators

1. Enable store-category Debug logging (via logging system config / CLI flags as documented in `internal/logging` architecture).  
2. Reproduce open of the suspect DB.  
3. Grep logs for `pragma "` failures.  
4. Cross-check profile at call site (code read — not in log).  
5. Manually query live DB: `PRAGMA journal_mode; PRAGMA cache_size; PRAGMA mmap_size;`  

## Test observability

Tests do not assert log output. Failures surface as test `Errorf` / `require` on read-back PRAGMA values.

## Recommended future hooks (not implemented)

| Hook | Benefit |
|------|---------|
| Debug include profile name | Faster correlation |
| Optional metric `pragma_fail_total` | CI / prod regression signal |
| Open-site name parameter | Trace which subsystem opened |

Any of these must stay optional and quiet by default (P10 silent success).
