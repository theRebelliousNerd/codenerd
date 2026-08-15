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

### `EnableForeignKeys`

```go
func EnableForeignKeys(db *sql.DB) error
```

Opt-in FK enforcement for a schema whose data is known clean. Deliberately not part of any profile (see IMPLEMENTED_SPEC §6.1). Unlike `ApplyDefaultPragmas` it **returns an error**: a caller asking for enforcement needs to know if it did not happen. It execs `PRAGMA foreign_keys = ON` and then reads the value back, because SQLite silently ignores the pragma on builds without FK support. `nil` db is a no-op returning `nil`.

Per-connection, and a no-op inside a transaction — pair with `OpenWithPragmas` or `SetMaxOpenConns(1)`.

## Connection-pool helpers

`ApplyDefaultPragmas` tunes exactly one pooled connection (whichever `database/sql` hands it). These tune every connection at birth.

### `OpenWithPragmas`

```go
func OpenWithPragmas(driverName, dsn string, profile PragmaProfile) (*sql.DB, error)
```

`sql.Open` plus a connector hook. Every connection the pool creates has the profile applied inside `Connect`, where `database/sql` cannot route around it. Lazy like `sql.Open` — a bad path surfaces on first use.

### `NewConnector`

```go
func NewConnector(drv driver.Driver, dsn string, profile PragmaProfile) (driver.Connector, error)
```

The hook on its own, for callers that already hold a `driver.Driver` or want to compose connectors. Pass the result to `sql.OpenDB`. Uses the driver's native connector when it implements `driver.DriverContext` (mattn and modernc both do), else wraps it in a minimal DSN connector. Errors on a nil driver.

## Host class (memory-budget scaling)

### `HostClass`

```go
type HostClass int

const (
    HostWorkstation HostClass = iota // default; full tuned budgets, divisor 1
    HostLaptop                       // divisor 4
    HostMicro                        // divisor 16 — containers, CI runners
)
```

Scales **only** `cache_size` and `mmap_size`. `journal_mode`, `synchronous`, `temp_store`, `busy_timeout` and `wal_autocheckpoint` are correctness/latency choices and are identical on every host. `HostWorkstation` emits byte-for-byte the values this package has always emitted.

| Symbol | Signature | Role |
|--------|-----------|------|
| `EnvHostClass` | `const = "NERD_SQL_HOST_CLASS"` | Env var read when no `SetHostClass` call was made |
| `SetHostClass` | `func(HostClass)` | Pin the class; outranks the env var |
| `ClearHostClass` | `func()` | Drop the pin, return to env resolution |
| `ActiveHostClass` | `func() HostClass` | Resolve: pin → env → `HostWorkstation` |
| `ParseHostClass` | `func(string) (HostClass, bool)` | Case-insensitive parse with aliases; `ok=false` on typos |

`SetHostClass` is the inversion-of-control seam: configuration is **pushed down** into this leaf rather than the leaf importing a config package, which is what keeps the import graph acyclic. An unparseable env value falls back to `HostWorkstation` — a typo must never be why a database opens untuned.

## Pragma-failure metrics (off by default)

Failures are logged at Debug and nothing else, so a regression is invisible unless store Debug is on. These counters make it observable without adding noise.

| Symbol | Signature | Role |
|--------|-----------|------|
| `EnvMetrics` | `const = "NERD_SQL_PRAGMA_METRICS"` | Enables counting at process start (`strconv.ParseBool` values) |
| `SetMetricsEnabled` | `func(bool)` | Inversion-of-control switch for an observability config |
| `MetricsEnabled` | `func() bool` | Current state |
| `PragmaFailureTotal` | `func() uint64` | Total failed PRAGMA statements |
| `PragmaFailuresByProfile` | `func() map[string]uint64` | Keyed by `PragmaProfile.String()` |
| `PragmaFailuresByStatement` | `func() map[string]uint64` | The driver reject-set view |
| `FailingPragmas` | `func() []string` | Sorted distinct failed statements |
| `ResetPragmaMetrics` | `func()` | Zero the counters |

When disabled, recording is a single atomic load and a return.

### `PragmaProfile.String`

```go
func (p PragmaProfile) String() string  // "Hot" | "BulkBuild" | "Query" | "ReadOnly"
```

Failure logs carry it (`pragma %q failed (profile %s): %v`) so an operator can tell which preset was in play without reading the call site. Undeclared values render as `PragmaProfile(N)`.

### Unexported (for readers of the package)

| Name | Signature | Role |
|------|-----------|------|
| `pragmasFor` | `func pragmasFor(profile PragmaProfile) []string` | Ordered PRAGMA list at the active host class |
| `pragmasForHost` | `func pragmasForHost(profile PragmaProfile, host HostClass) []string` | The preset table proper; golden-tested |

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

### Multi-connection pool (connector hook)

```go
db, err := sqlpragmas.OpenWithPragmas("sqlite3", path, sqlpragmas.ProfileHot)
if err != nil {
    return err
}
db.SetMaxOpenConns(8) // every one of them is tuned
```

Use this instead of `sql.Open` + `ApplyDefaultPragmas` for any handle whose pool may exceed one connection.

### Opting a clean schema into FK enforcement

```go
sqlpragmas.ApplyDefaultPragmas(db, sqlpragmas.ProfileHot)
if err := sqlpragmas.EnableForeignKeys(db); err != nil {
    return fmt.Errorf("northstar store: %w", err)
}
```

### Pushing host class down from config

```go
if hc, ok := sqlpragmas.ParseHostClass(cfg.SQLHostClass); ok {
    sqlpragmas.SetHostClass(hc)
} else {
    log.Warn("unknown sql host class %q, using workstation defaults", cfg.SQLHostClass)
}
```

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
- mmap/cache numeric constants (named Go consts internally: `mmapHot`, `cacheHotKiB`, `busyTimeoutMS`, … — readability only, values unchanged)  
- Logger category choice  
- Driver names  

## Enforcement

This surface is not maintained by discipline. Tests in the package fail on drift:

| Test | Enforces |
|------|----------|
| `TestExportedAPI_WhenSymbolAdded_ShouldAppearInPublicAPIDoc` | Every exported symbol is named in **this file** |
| `TestProfileConstants_WhenProfileAdded_ShouldAppearInCorpus` | New profiles reach IMPLEMENTED_SPEC + this file |
| `TestHostClassConstants_WhenHostClassAdded_ShouldAppearInCorpus` | Same for host classes |
| `TestPragmasFor_WhenPresetsChange_ShouldMatchGolden` | Any preset edit forces a reviewed golden diff |
| `TestPackageImports_WhenNewImportAdded_ShouldStayLeaf` | Leaf invariant (stdlib + `internal/logging` only) |
| `TestSQLOpenSites_WhenOpeningSQLite_ShouldApplyPragmasOrBeExempt` | Repo-wide: no untuned `sql.Open` |
| `TestPragmaSurface_WhenNewPackageAppliesPragmas_ShouldPreferTheLeaf` | New packages import the leaf, not the `store` façade |
