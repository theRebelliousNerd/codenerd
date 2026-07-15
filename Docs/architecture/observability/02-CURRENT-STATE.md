# 02 — Current State: observability

> Last verified against codebase: 2026-07-13  
> Status: Precise inventory  
> Source root: `internal/observability/`

## 1. Scale summary

| Kind | Count | Approx lines |
|------|------:|-------------:|
| Non-test `.go` | 2 | 326 |
| Test `.go` | 3 | 369 |
| `.mg` / Mangle | 0 | 0 |
| Package README / agents.md | 0 | — |

**Verdict:** small, complete leaf package. Living code — not a stub directory.

## 2. File inventory

### 2.1 Sources

| Path | ≈Lines | Responsibility |
|------|-------:|----------------|
| [`internal/observability/runtime_metrics.go`](../../../internal/observability/runtime_metrics.go) | 188 | Package doc; `LogStartupMetrics`; metric path list; field key / short name / sample formatting; Green Tea detection |
| [`internal/observability/flight_recorder.go`](../../../internal/observability/flight_recorder.go) | 278 | Singleton flight recorder Start/Stop/Enabled/Dump; memory watchdog (`flightMemWatchdog`) that auto-stops the recorder before the tracer's region allocator OOM-crashes the process (see FM13) |

### 2.2 Tests

| Path | ≈Lines | Focus |
|------|-------:|-------|
| [`flight_recorder_lifecycle_test.go`](../../../internal/observability/flight_recorder_lifecycle_test.go) | 147 | Panic capture lifecycle; double dump keeps recorder; Go trace magic |
| [`flight_recorder_test.go`](../../../internal/observability/flight_recorder_test.go) | 121 | Start/stop; non-empty dump; dump without start; stop idempotent; empty nerdDir |
| [`flight_recorder_watchdog_test.go`](../../../internal/observability/flight_recorder_watchdog_test.go) | 100 | Memory watchdog: auto-stop on growth past cap; no false-trip below cap; clean generation handoff after external Stop |
| [`runtime_metrics_test.go`](../../../internal/observability/runtime_metrics_test.go) | 101 | No-panic LogStartupMetrics; all metric paths supported; GOMAXPROCS; formatSample; metricFieldKey; shortMetric; greenTeaStatus |

## 3. Behavioral inventory

### 3.1 Always-on at process entry (when main runs)

1. `logging.Initialize(cwd)` — outside this package but required.  
2. `config.GlobalConfig()` — populates features.  
3. `LogStartupMetrics()` — **not** feature-gated.  
4. Conditionally `StartFlightRecorder` + panic dump defer.

### 3.2 Feature-gated

| Behavior | Gate |
|----------|------|
| Start flight recorder | `features.IsFlightRecorderEnabled()` |
| Install panic dump defer | Only if Start succeeds |

### 3.3 Not present on disk

| Expected by some docs/comments | Reality |
|--------------------------------|---------|
| `/diag flightrec` chat command | Not found under `cmd/nerd` |
| Cobra `diag` subcommand dumping traces | Not found |
| Mid-session `LogRuntimeMetrics` | Only startup |
| Config keys for ring size/period | Hardcoded in `main.go` |
| Retention cleaner for traces | None |

## 4. Hotspots

| Hotspot | Why it matters |
|---------|----------------|
| Package-level `flight` singleton | Process-wide; tests must Stop; production double-start is intentional no-op |
| `DumpFlightRecord` buffer-then-write | Correctness under disk full / permission errors |
| `startupMetricPaths` | Breaks silently as KindBad without the support test |
| `main.go` recover scope | Defines real-world dump coverage |

## 5. Cross-package touchpoints (current)

| Package | Touch |
|---------|-------|
| `cmd/nerd` | Sole importer |
| `internal/logging` | Emit target |
| `internal/features` | Gate (not imported by observability; used by main) |
| `internal/config` | Loads features before gate (main) |

## 6. Comparison to sibling “observability-ish” code

| System | Location | Overlap |
|--------|----------|---------|
| Categorized logs | `internal/logging` | Sink / streams |
| Glass box | `cmd/nerd/chat`, transparency | Agent-visible timeline |
| Prompt manifest | `internal/prompt` | Atom selection “flight recorder” metaphor |
| CLI logs command | `cmd/nerd` logs | Aggregates log files |

`internal/observability` is the only package that owns **Go execution-trace ring capture** and the **canonical boot runtime/metrics sample**.
