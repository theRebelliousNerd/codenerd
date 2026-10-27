# Goleak Findings (Task #9 Wiring)

Generated during the wiring of `go.uber.org/goleak` into `TestMain` for
`internal/campaign`, `internal/core`, and `internal/mangle`.

## Configured ignores

| Package | Ignore | Justification |
|---|---|---|
| `internal/core` | `goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start")` | OpenCensus stats worker has no clean shutdown hook; process-lifetime by design (pre-existing ignore, preserved). |
| `internal/core` | `goleak.IgnoreTopFunction("database/sql.(*DB).connectionOpener")` | sql.DB connection opener lives until *DB is GC'd; tests don't always Close (pre-existing, preserved). |
| `internal/core` | `goleak.IgnoreAnyFunction("github.com/fsnotify/fsnotify.*")` | fsnotify spawns multiple internal goroutines (readEvents/recv) that vary by OS; pre-existing ignore, preserved. |
| `internal/mangle` | `goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start")` | Same as core (pre-existing). |
| `internal/mangle` | `goleak.IgnoreTopFunction("database/sql.(*DB).connectionOpener")` | Same as core (pre-existing). |
| `internal/campaign` | `goleak.IgnoreTopFunction("codenerd/internal/perception.(*ConsolidationWorker).Start.func1")` | `internal/perception` package init() instantiates `SharedTaxonomy`, which starts a ConsolidationWorker goroutine with no shutdown hook reachable from test code. Process-lifetime by design. |
| `internal/campaign` | `goleak.IgnoreTopFunction("database/sql.(*DB).connectionOpener")` | Same rationale as core. |

## Discovered leaks (follow-up tasks)

### 1. `codenerd/internal/perception.(*ConsolidationWorker)` started by package init()

- **File**: `internal/perception/taxonomy.go:83-89` (the `init()` block creates
  `SharedTaxonomy` which calls `worker.Start()`).
- **Severity**: Low. The worker is process-lifetime by design and `Stop()` is
  idempotent, but no code path in tests can call it (the singleton has no
  exported `Shutdown` and the worker is unexported).
- **Recommendation (NOT in scope for Task #9)**: Add an exported
  `perception.ShutdownSharedTaxonomy()` so test cleanup paths and the chat
  shutdown hook can stop the worker cleanly. Until then it remains ignored
  in `internal/campaign/main_test.go`.

### 2. `database/sql.(*DB).connectionOpener` instances

- **Source**: Tests in `internal/campaign` and `internal/core` open multiple
  `*sql.DB` handles (likely via `internal/store` and `internal/embedding`
  paths) without explicit `Close()`. Each leaked goroutine corresponds to
  one un-closed `*sql.DB`.
- **Severity**: Test-hygiene issue; not a production leak. `database/sql`
  documents that callers must Close the DB or accept the connectionOpener
  goroutine living until GC.
- **Recommendation (NOT in scope for Task #9)**: Audit test fixtures in
  `internal/store`, `internal/embedding`, and the campaign orchestrator
  test mocks to ensure `defer db.Close()` is wired in `t.Cleanup` blocks.

## Pre-existing build blocker (NOT a goleak issue, NOT introduced by Task #9)

`go test ./internal/campaign/...` and `go test ./internal/core/...` currently
fail at the *compile* stage with:

```
package codenerd/internal/config
    imports codenerd/internal/mcp from integrations.go
    imports codenerd/internal/store from store.go
    imports codenerd/internal/config from learning.go: import cycle not allowed
```

The cycle is `internal/config -> internal/mcp -> internal/store -> internal/config`.
It was introduced by recent commits to `internal/store/learning.go` (adds
`import "codenerd/internal/config"`) combined with
`internal/config/integrations.go` (already imports `internal/mcp`) and
`internal/mcp/store.go` (already imports `internal/store`).

The goleak TestMain wiring is correct; this cycle must be resolved by another
task before the goleak verification can run end-to-end on those packages.
`internal/mangle` is unaffected and its tests pass cleanly with the existing
`VerifyTestMain` setup.
