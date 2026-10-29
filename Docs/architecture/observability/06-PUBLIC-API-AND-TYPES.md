# 06 — Public API and Types: observability

> Last verified against codebase: 2026-07-13  
> Package: `codenerd/internal/observability`  
> Exported types: **0**  
> Exported functions: **5**

## 1. Package import

```go
import "codenerd/internal/observability"
```

## 2. Exported functions

### 2.1 `LogStartupMetrics`

| | |
|--|--|
| **Signature** | `func LogStartupMetrics()` |
| **File** | `internal/observability/runtime_metrics.go` |
| **When** | Once near process entry after logging init |
| **Errors** | None returned; must not panic |
| **Side effects** | Boot Info + StructuredLog; optional Warn for Green Tea off |

**Caller contract:** Invoke after `logging.Initialize` so CategoryBoot is usable. Safe to call without a `.nerd` workspace (tests do).

---

### 2.2 `StartFlightRecorder`

| | |
|--|--|
| **Signature** | `func StartFlightRecorder(sizeBytes int, period time.Duration) error` |
| **File** | `internal/observability/flight_recorder.go` |
| **When** | Once at process entry when feature enabled |
| **Idempotency** | Second call is no-op success (existing recorder kept) |
| **Parameters** | `sizeBytes`: ring upper bound; `0` → runtime default. `period`: min age; `0` → runtime default |
| **Production** | `StartFlightRecorder(64<<20, 30*time.Second)` from `cmd/nerd/main.go` |

---

### 2.3 `StopFlightRecorder`

| | |
|--|--|
| **Signature** | `func StopFlightRecorder() error` |
| **File** | `internal/observability/flight_recorder.go` |
| **When** | Tests (required cleanup); optional explicit shutdown |
| **Idempotency** | Safe when never started; returns nil |
| **Production** | Not required for process correctness |

---

### 2.4 `FlightRecorderEnabled`

| | |
|--|--|
| **Signature** | `func FlightRecorderEnabled() bool` |
| **File** | `internal/observability/flight_recorder.go` |
| **When** | Diagnostics / tests |
| **Semantics** | `flight != nil && flight.Enabled()` under mutex |

---

### 2.5 `DumpFlightRecord`

| | |
|--|--|
| **Signature** | `func DumpFlightRecord(nerdDir string) (string, error)` |
| **File** | `internal/observability/flight_recorder.go` |
| **When** | Panic path (production); tests; future on-demand diag |
| **Argument** | Workspace **root** (not `.nerd` itself) |
| **Returns** | Absolute path of `…/.nerd/traces/flight_<UTC>.trace` |
| **Postcondition** | Recorder remains running on success |

**Errors (messages as implemented):**

- `flight recorder not started`
- `flight recorder not enabled`
- `nerdDir must be non-empty`
- `create traces dir: …`
- `flight recorder snapshot: …`
- `write flight trace: …`

## 3. Unexported helpers (callers must not depend)

| Symbol | File | Notes |
|--------|------|-------|
| `startupMetricPaths` | `runtime_metrics.go` | Var of metric path strings |
| `metricFieldKey` | same | Path → snake_case field |
| `shortMetric` | same | Compact summary label |
| `formatSample` | same | Kind → value + display |
| `greenTeaStatus` | same | `(enabled bool, detail string)` |
| `flightMu`, `flight` | `flight_recorder.go` | Singleton state |

## 4. Related external API (not in this package)

| Symbol | Package | Role |
|--------|---------|------|
| `features.IsFlightRecorderEnabled` | `internal/features` | Gate for Start |
| `features.FeaturesConfig.FlightRecorder` | `internal/features` | JSON `flight_recorder` |
| `logging.Get(logging.CategoryBoot)` | `internal/logging` | Sink |
| `config.GlobalConfig` | `internal/config` | Loads features before gate |

## 5. API stability notes

| Change | Risk |
|--------|------|
| Adding optional parameters to Start | Prefer new config struct only if multiple knobs appear |
| Changing dump directory layout | Breaks operator scripts; require major doc notice |
| Renaming structured event `runtime_metrics_startup` | Breaks log scrapers |
| Exporting types for metrics samples | Only if mid-session API is added intentionally |

## 6. Minimal usage sketch (production shape)

```go
// after logging.Initialize and config load:
observability.LogStartupMetrics()

if features.IsFlightRecorderEnabled() {
    if err := observability.StartFlightRecorder(64<<20, 30*time.Second); err != nil {
        // warn; continue
    } else {
        defer func() {
            if r := recover(); r != nil {
                _, _ = observability.DumpFlightRecord(workspaceRoot)
                panic(r)
            }
        }()
    }
}
```

This sketch matches `cmd/nerd/main.go` (2026-07-13); prefer the live file for exact control flow.
