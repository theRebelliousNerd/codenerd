# sqlpragmas — Public API and Types

> Last verified: **2026-07-13**  
> Complete exported surface of `codenerd/internal/sqlpragmas`.

## Import path

```go
import "codenerd/internal/sqlpragmas"
```

## Types

### `PragmaProfile`

| Field | Value |
|-------|-------|
| Definition | `type PragmaProfile int` |
| File | `internal/sqlpragmas/pragmas.go` (~line 26) |
| Purpose | Select a PRAGMA preset matched to a workload |

#### Constants

| Name | Value | File ref | Documented workload |
|------|------:|----------|---------------------|
| `ProfileHot` | 0 | `pragmas.go` | Long-lived agent stores (LocalStore, learned, prompt cache, MCP, northstar, …) |
| `ProfileBulkBuild` | 1 | `pragmas.go` | Bulk-write tools (corpus_builder, prompt_builder, predicate_corpus_builder, …) |
| `ProfileQuery` | 2 | `pragmas.go` | Short-lived read-mostly (query-kb, init validation, agent atom count) |
| `ProfileReadOnly` | 3 | `pragmas.go` | `mode=ro` / no writes from this handle |

## Functions

### `ApplyDefaultPragmas`

```go
func ApplyDefaultPragmas(db *sql.DB, profile PragmaProfile)
```

| Aspect | Contract |
|--------|----------|
| File | `internal/sqlpragmas/pragmas.go` (~line 60) |
| nil `db` | No-op |
| Return | None (no error) |
| Closes DB? | Never |
| Failure | `logging.Get(logging.CategoryStore).Debug("pragma %q failed: %v", p, err)` |
| When to call | Once after `sql.Open`, before schema/first query |

### Unexported (for readers of the package)

| Name | Signature | Role |
|------|-----------|------|
| `pragmasFor` | `func pragmasFor(profile PragmaProfile) []string` | Ordered PRAGMA list |

Not part of public API; do not call from other packages (unexported).

## Re-exported surface (`internal/store`)

Documented here because many callers never import `sqlpragmas` by name:

| Store symbol | Kind | Alias of |
|--------------|------|----------|
| `store.PragmaProfile` | type alias | `sqlpragmas.PragmaProfile` |
| `store.ProfileHot` | const | `sqlpragmas.ProfileHot` |
| `store.ProfileBulkBuild` | const | `sqlpragmas.ProfileBulkBuild` |
| `store.ProfileQuery` | const | `sqlpragmas.ProfileQuery` |
| `store.ProfileReadOnly` | const | `sqlpragmas.ProfileReadOnly` |
| `store.ApplyDefaultPragmas` | var func | `sqlpragmas.ApplyDefaultPragmas` |

File: `internal/store/pragmas.go`.

## Usage examples (illustrative of real patterns)

### Hot store open (mid-layer, direct)

```go
db, err := sql.Open("sqlite3", path)
if err != nil {
    return err
}
sqlpragmas.ApplyDefaultPragmas(db, sqlpragmas.ProfileHot)
```

Pattern in: `internal/mcp/store.go`, `internal/northstar/store.go`, …

### Bulk builder (tool)

```go
sqlpragmas.ApplyDefaultPragmas(db, sqlpragmas.ProfileBulkBuild)
```

Pattern in: `cmd/tools/corpus_builder/main.go`.

### Query tool via store façade

```go
store.ApplyDefaultPragmas(db, store.ProfileQuery)
```

Pattern in: `cmd/query-kb/main.go`.

### Read-only corpus

```go
sqlpragmas.ApplyDefaultPragmas(db, sqlpragmas.ProfileReadOnly)
```

Pattern in: `internal/core/predicate_corpus.go`.

## Compatibility commitments

| Change | Compatibility |
|--------|---------------|
| Add new profile const | Safe (iota append) |
| Reorder existing iota | **Breaking** for int-cast callers |
| Change cache/mmap numbers | Behavior change (perf/RAM), not Go compile break |
| Return `error` from Apply | **Breaking** for all call sites |
| Enable `foreign_keys` in defaults | Data behavior break |

## What is not exported

- PRAGMA string lists  
- mmap/cache numeric constants as named Go consts  
- Logger category choice  
- Driver names  
