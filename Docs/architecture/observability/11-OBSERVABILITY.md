# 11 — Observability of observability

> Last verified against codebase: 2026-07-13  
> Meta-doc: how this package itself is observed and what it emits  
> Package: `internal/observability/`

## 1. Emit surface

This package **is** an observability producer. Its outputs are:

| Channel | Artifact |
|---------|----------|
| Boot logger (human) | One-line metrics summary; start/dump notices; optional Green Tea warn |
| Boot logger (structured) | Event `runtime_metrics_startup` + field map |
| Filesystem | `.nerd/traces/flight_<UTC>.trace` |
| stderr (host) | Start failure warning; successful panic dump path |

It does **not** expose HTTP metrics ports, statsd, or OpenTelemetry exporters.

## 2. Logging categories

| Category | Constant | Usage in package |
|----------|----------|------------------|
| Boot | `logging.CategoryBoot` | All Info/Warn/StructuredLog calls |

No other logging categories are used.

## 3. Structured event contract

**Event name:** `runtime_metrics_startup`  
**Level:** `info` (via `StructuredLog`)

| Field key | Type (typical) | Source |
|-----------|----------------|--------|
| `go_version` | string | `runtime.Version()` |
| `gomaxprocs` | int | `GOMAXPROCS(0)` |
| `num_cpu` | int | `NumCPU()` |
| `sched_goroutines` | uint64 | `/sched/goroutines:goroutines` |
| `sched_gomaxprocs` | uint64 | `/sched/gomaxprocs:threads` |
| `gc_heap_goal` | uint64 | `/gc/heap/goal:bytes` |
| `gc_heap_live` | uint64 | `/gc/heap/live:bytes` |
| `gc_cycles_total` | uint64 | `/gc/cycles/total:gc-cycles` |
| `cpu_classes_gc_total` | float64 or uint64 | `/cpu/classes/gc/total:cpu-seconds` |
| `memory_classes_total` | uint64 | `/memory/classes/total:bytes` |
| `greentea_gc` | bool | detection |
| `greentea_detail` | string | detection |

Keys for metric paths are produced by `metricFieldKey` (unit suffix stripped). If KindBad, values may be nil / `"n/a"` display in the human line.

## 4. Human summary line

Pattern:

```text
runtime metrics snapshot: go=<ver> GOMAXPROCS=<n>/<cpu> <short>=<val> … greentea=<bool>
```

Short labels from `shortMetric` (e.g. `goroutines`, `heap_goal`, `gc_total`).

## 5. Flight recorder log lines

```text
flight recorder started (max_bytes=<int> min_age=<duration>)
flight recorder dumped: path=<abs> bytes=<int>
```

Both Info on CategoryBoot.

## 6. Host stderr messages (outside package)

From `cmd/nerd/main.go`:

```text
Warning: flight recorder failed to start: …
Flight trace dumped to <path>
```

## 7. Trace file observability

| Property | Value |
|----------|-------|
| Directory | `<workspace>/.nerd/traces/` |
| Filename | `flight_20060102T150405Z.trace` (UTC) |
| Format | Go execution trace (`go tool trace`) |
| Magic | ASCII prefix `go 1.` |

Debug workflow:

```powershell
go tool trace .nerd\traces\flight_YYYYMMDDTHHMMSSZ.trace
```

## 8. Debug hooks for developers

| Hook | How |
|------|-----|
| Is recorder live? | `observability.FlightRecorderEnabled()` |
| Force dump in debugger/REPL tests | `DumpFlightRecord(tmp)` after Start |
| Disable ring | `NERD_FLIGHTREC=0` |
| Inspect metric paths | Read `startupMetricPaths` or fail the support test on upgrade |

## 9. What this package does **not** observe

| Domain | Owner |
|--------|-------|
| LLM I/O payloads | `internal/logging` LLM I/O logger |
| Kernel eval timing | core / logging categories |
| Shard lifecycle | shards manager + logging |
| Prompt atom selection | `internal/prompt` manifest |
| User-visible agent steps | glass box / transparency |
| Campaign assault artifacts | `.nerd/campaigns/…` |

## 10. Relationship to CLI telemetry doc

CLI corpus `Docs/architecture/cli/12-TELEMETRY-OBSERVABILITY.md` points here for runtime metrics and flight recorder. When extending CLI hot-path counters, prefer logging categories or glass box; only add to this package for **process-wide Go runtime** instruments.
