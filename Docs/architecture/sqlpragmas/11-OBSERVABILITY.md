# sqlpragmas — Observability

> Last verified: **2026-08-15**

## Logging

| Aspect | Value |
|--------|-------|
| Category | `logging.CategoryStore` (`"store"`) |
| API | `logging.Get(logging.CategoryStore)` |
| Success path | **No log** |
| Failure path | `Debug("pragma %q failed (profile %s): %v", p, profile, err)` |
| File | `internal/sqlpragmas/pragmas.go` (`ApplyDefaultPragmas`), `connector.go` (`applyToDriverConn`, suffixed "on new connection") |

### Why Debug, not Warn/Error

A pragma a driver refuses must not become a failed open, and rejections were assumed common enough that Error would spam every open on pure-Go paths. Debug keeps the signal available when store debugging is enabled without frightening operators.

Note that the premise has now been measured and is weaker than assumed: at modernc.org/sqlite v1.50.1 the reject set is empty (see `modernc_integration_test.go`). Debug is retained because read-only handles and unusual filesystems still produce expected rejections, and because the metrics below now give the regression signal that Debug alone could not.

### What is logged

- Full PRAGMA statement string (constant content — not user DSN)  
- Profile name (`Hot`, `BulkBuild`, `Query`, `ReadOnly`)  
- Underlying `error` from `db.Exec`  

### What is not logged

- DSN / file path  
- Which call site applied  
- Anything on success  

## Metrics

Off by default. Debug-only failure reporting means a real regression — a pragma that started failing on the primary driver, or a whole preset being rejected — is invisible unless someone happens to be running with store Debug on. These counters close that without adding noise.

| Symbol | Returns |
|--------|---------|
| `PragmaFailureTotal()` | `uint64` total failed PRAGMA statements |
| `PragmaFailuresByProfile()` | `map[string]uint64` keyed by `PragmaProfile.String()` |
| `PragmaFailuresByStatement()` | `map[string]uint64` — the **driver reject-set view** |
| `FailingPragmas()` | sorted distinct failed statements |
| `ResetPragmaMetrics()` | zeroes the counters |

Enable with `SetMetricsEnabled(true)` (the inversion-of-control seam for an observability config — this leaf must not import one) or `NERD_SQL_PRAGMA_METRICS=1` at process start. When disabled, recording is a single atomic load and a return.

Still no counters for pragmas applied or per-profile open counts: those would be pure volume, with no failure mode to detect.

## Tracing / spans

**None.**

## Debug workflow for operators

1. Enable store-category Debug logging (via logging system config / CLI flags as documented in `internal/logging` architecture), or set `NERD_SQL_PRAGMA_METRICS=1` for a countable answer.  
2. Reproduce open of the suspect DB.  
3. Grep logs for `pragma "` failures — the line names the profile.  
4. Or read `FailingPragmas()` / `PragmaFailuresByStatement()` for the exact reject set.  
5. Manually query live DB: `PRAGMA journal_mode; PRAGMA cache_size; PRAGMA mmap_size;`  
6. If the values are right on one connection and wrong on another, the handle needs `OpenWithPragmas` rather than `sql.Open` + `ApplyDefaultPragmas` — pragmas are per-connection.  

## Test observability

Tests do not assert log output. Failures surface as test `Errorf` / `require` on read-back PRAGMA values.

## Hook status

| Hook | Status |
|------|--------|
| Debug include profile name | **Implemented** |
| Optional metric `pragma_fail_total` | **Implemented** as `PragmaFailureTotal` + per-profile / per-statement breakdowns |
| Open-site name parameter | Not implemented — would change the signature at 32 call sites for a value a stack trace already carries |

Implemented hooks stay optional and quiet by default (P10 silent success).
