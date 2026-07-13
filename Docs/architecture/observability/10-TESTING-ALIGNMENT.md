# 10 — Testing Alignment: observability

> Last verified against codebase: 2026-07-13  
> Package: `internal/observability/`

## 1. Commands

```powershell
go test ./internal/observability/...
go test -race ./internal/observability/...
go test -count=1 ./internal/observability/...   # avoid cache when validating flakes
```

**Note:** lifecycle tests intentionally avoid `testing/synctest`. Comments in `flight_recorder_lifecycle_test.go` explain that virtual time starves `runtime/trace` emission and yields near-empty dumps. Real wall-clock and OS-thread scheduling are required.

## 2. Test inventory

### 2.1 `runtime_metrics_test.go`

| Test | Asserts |
|------|---------|
| `TestLogStartupMetrics_NoPanic` | Boot metrics entry does not panic without full workspace config |
| `TestStartupMetricPaths_AllSupported` | Every `startupMetricPaths` entry ∈ `runtime/metrics.All()` |
| `TestGOMAXPROCSReported` | `GOMAXPROCS(0) >= 1` |
| `TestFormatSample_Uint64` | Live goroutine sample formats as uint64 + non-empty display |
| `TestMetricFieldKey` | Path → snake_case table |
| `TestShortMetric` | Compact labels + generic leaf disambiguation |
| `TestGreenTeaStatus` | Detail non-empty; disabled implies `nogreenteagc` in detail |

### 2.2 `flight_recorder_test.go`

| Test | Asserts |
|------|---------|
| `TestStartFlightRecorder_StartStop` | Disabled before start; enabled after; second start ok |
| `TestDumpFlightRecord_WritesNonEmptyFile` | Absolute path; non-empty; ≥16 bytes; dir `.nerd/traces` |
| `TestDumpFlightRecord_WithoutStart` | Error when idle |
| `TestStopFlightRecorder_Idempotent` | Stop without start is fine |
| `TestDumpFlightRecord_EmptyNerdDir` | Error for `""` |

Helpers: `resetFlightRecorder` stops singleton between tests.

### 2.3 `flight_recorder_lifecycle_test.go`

| Test | Asserts |
|------|---------|
| `TestFlightRecorder_PanicCaptureLifecycle` | Start → panicing goroutine recover → dump → magic header `"go 1."` → path under `.nerd/traces` / `flight_*` |
| `TestFlightRecorder_DoubleDumpKeepsRecorderRunning` | Two dumps, distinct paths, both valid magic, recorder still enabled |

Helpers: `hasGoTraceMagic`, `strHasPrefix`.

**Timing:** double-dump sleeps ~1.1s so second-resolution timestamps differ.

## 3. Coverage vs risk

| Risk | Covered? |
|------|----------|
| Go upgrade retires metric path | **Yes** — support test |
| Field key regressions | **Yes** — table tests |
| Dump without start | **Yes** |
| Empty nerdDir | **Yes** |
| Panic-adjacent dump still valid trace | **Yes** — lifecycle |
| Double dump uniqueness + keep running | **Yes** |
| main.go panic defer integration | **No** — package-level only |
| Feature flag env matrix | **No** here (covered partly in `internal/features` tests) |
| Concurrent Start from two packages | **N/A** single importer |
| Disk full during dump | **No** |
| Green Tea force-on/off env matrix | **Partial** — cannot always control process GOEXPERIMENT |

## 4. Gaps and recommendations

| Gap | Priority | Suggestion |
|-----|----------|------------|
| No `cmd/nerd` integration test for panic dump | P2 | Optional build-tag stress test; expensive |
| No assert on structured log payload keys | P3 | Capture logger if test hooks exist in logging package |
| Double-dump 1.1s sleep | P3 | Acceptable; document as intentional |
| Race with Stop during Dump | P3 | Add targeted race test if Stop is wired to shutdown |

## 5. Alignment with package principles

| Principle | Test support |
|-----------|--------------|
| P6 Singleton honesty | Start twice no-op |
| P7 Dump non-destructive | Double dump |
| P10 Pin metric names | AllSupported |
| Fail-open / no panic metrics | NoPanic |

## 6. CI expectations

Package is small and should always run in full `go test ./...`. Do not skip lifecycle tests in short mode without an alternate validation of dump magic — they are the only end-to-end proof that traces are real Go execution traces.
