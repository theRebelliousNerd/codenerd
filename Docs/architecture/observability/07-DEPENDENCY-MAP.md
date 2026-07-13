# 07 — Dependency Map: observability

> Last verified against codebase: 2026-07-13  
> Package: `internal/observability/`

## 1. Inbound (who imports this package)

Evidence: repository grep for `codenerd/internal/observability` in non-test `.go` files.

| Importer | Path | Symbols used |
|----------|------|--------------|
| codeNERD CLI main | `cmd/nerd/main.go` | `LogStartupMetrics`, `StartFlightRecorder`, `DumpFlightRecord` |

**No** other production packages import observability. Tests live inside the package itself.

```
cmd/nerd/main.go
       │
       ▼
internal/observability
```

## 2. Outbound (what this package imports)

### 2.1 Internal

| Package | Used for |
|---------|----------|
| `codenerd/internal/logging` | `Get(CategoryBoot)`, `Info`, `Warn`, `StructuredLog` |

### 2.2 Standard library

| Package | Used for |
|---------|----------|
| `runtime` | GOMAXPROCS, NumCPU, Version, Gosched (tests) |
| `runtime/metrics` | Sample paths, Read |
| `runtime/debug` | ReadBuildInfo for GOEXPERIMENT |
| `runtime/trace` | FlightRecorder, FlightRecorderConfig |
| `sync` | Mutex |
| `os` | Env, MkdirAll, WriteFile |
| `bytes` | Dump buffer |
| `fmt` | Errors and formatting |
| `path/filepath` | Trace paths |
| `time` | MinAge, dump timestamps |
| `strings` | Field key transforms, Green Tea parse |

### 2.3 Third-party

| Package | Used for |
|---------|----------|
| `github.com/stretchr/testify/require` | **Tests only** (`flight_recorder_lifecycle_test.go`) |

## 3. Sibling packages (conceptual, not imports)

These packages interact at the **system** level without Go import edges into observability:

| Package | Relationship |
|---------|----------------|
| `internal/features` | Main gates Start via `IsFlightRecorderEnabled` |
| `internal/config` | Main loads config so features registry is populated |
| `internal/prompt` | Homonymous “flight recorder” (manifest) — **no code coupling** |
| `cmd/nerd/chat` | May recover panics without dumping — **coverage gap** |

## 4. Layering diagram

```
                    ┌──────────────────┐
                    │   cmd/nerd       │
                    │   (binary host)  │
                    └────┬───┬───┬─────┘
                         │   │   │
           ┌─────────────┘   │   └──────────────┐
           ▼                 ▼                  ▼
   internal/config    internal/features   internal/observability
           │                 ▲                  │
           └─────────────────┘                  │
                                                ▼
                                       internal/logging
                                                │
                                                ▼
                                           filesystem
                                         (.nerd/logs,…)
                                                │
                                    .nerd/traces (from Dump)
```

**Invariant:** arrows into `observability` only from host main (today).  
**Invariant:** observability never arrows into config/features/core.

## 5. Reverse-dep maintenance commands

```powershell
# Production importers
rg "codenerd/internal/observability" -g "*.go" --glob "!*_test.go"

# Feature gate surface
rg "IsFlightRecorderEnabled|StartFlightRecorder|DumpFlightRecord|LogStartupMetrics" -g "*.go"
```

## 6. Cycle risk

| Risk | Status |
|------|--------|
| observability → config → … → observability | **Blocked** by not importing config |
| observability → features | **Blocked** by design (P8) |
| logging → observability | Must not happen; logging is lower leaf |

If a future mid-session API is needed from core, **do not** import observability into core. Either keep calls in host/CLI or introduce a tiny callback registered from main.
