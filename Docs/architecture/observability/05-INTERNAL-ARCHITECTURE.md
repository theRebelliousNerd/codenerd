# 05 — Internal Architecture: observability

> Last verified against codebase: 2026-07-13  
> Package: `internal/observability/`

## 1. Component map

```
internal/observability
├── Startup Metrics Subsystem          (runtime_metrics.go)
│   ├── startupMetricPaths[]           // canonical names
│   ├── LogStartupMetrics()            // public entry
│   ├── formatSample / metricFieldKey / shortMetric
│   └── greenTeaStatus()
│
└── Flight Recorder Subsystem          (flight_recorder.go)
    ├── flightMu + flight              // process singleton
    ├── StartFlightRecorder
    ├── StopFlightRecorder
    ├── FlightRecorderEnabled
    └── DumpFlightRecord
```

Two subsystems share only the package name and the logging dependency. There is **no shared interface type** between them.

## 2. Data flow — startup metrics

```
LogStartupMetrics()
    │
    ├─ allocate []metrics.Sample from startupMetricPaths
    ├─ metrics.Read(samples)
    ├─ runtime.GOMAXPROCS(0), NumCPU(), Version()
    │
    ├─ for each sample:
    │     metricFieldKey → fields map
    │     shortMetric + formatSample → summaryParts
    │
    ├─ greenTeaStatus() → greentea_gc, greentea_detail
    │
    ├─ logging.Get(CategoryBoot).Info("runtime metrics snapshot: …")
    ├─ StructuredLog("info", "runtime_metrics_startup", fields)
    └─ if !greentea: Warn(…)
```

### Field key transform

```
/cpu/classes/gc/total:cpu-seconds
        │ drop unit after ':'
/cpu/classes/gc/total
        │ trim leading '/'
cpu/classes/gc/total
        │ '/' and '-' → '_'
cpu_classes_gc_total
```

### Short label disambiguation

Generic leaves `total`, `goal`, `live` prepend the previous path segment so `heap_goal` vs `gc_total` do not collide in the human line.

## 3. Data flow — flight recorder

```
StartFlightRecorder(size, period)
    lock flightMu
    if flight != nil → unlock, return nil
    build FlightRecorderConfig (optional MaxBytes, MinAge)
    NewFlightRecorder → Start
    flight = fr
    Boot.Info("flight recorder started …")
    unlock

DumpFlightRecord(nerdDir)
    lock; copy fr pointer; unlock     // avoid holding lock during I/O
    validate fr, Enabled, nerdDir
    MkdirAll(nerdDir/.nerd/traces)
    fr.WriteTo(buffer)
    WriteFile(flight_<UTC>.trace)
    Boot.Info("flight recorder dumped …")
    return path

StopFlightRecorder()
    lock
    if flight != nil: Stop; flight = nil
    unlock
```

### Concurrency notes

| Region | Lock held? |
|--------|------------|
| Start / Stop / Enabled | Yes (`flightMu`) |
| Dump snapshot WriteTo / WriteFile | **No** (uses local `fr` copy) |

Dump releases the mutex before I/O so a slow disk does not block concurrent `Enabled` checks. Race: concurrent `Stop` during dump could stop the recorder underfoot — acceptable for process-shutdown races; production dump runs from panic path without concurrent Stop.

## 4. State machine (flight recorder)

| State | `flight` | `Enabled()` typical |
|-------|----------|---------------------|
| Idle | `nil` | false |
| Running | non-nil, started | true |
| After dump | same non-nil | true |
| After stop | `nil` | false |

Transitions documented in [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md) mermaid diagram.

## 5. Types (internal)

| Type | Kind | Location |
|------|------|----------|
| `*trace.FlightRecorder` | stdlib | held in package var |
| `trace.FlightRecorderConfig` | stdlib | constructed in Start |
| `metrics.Sample` | stdlib | local in LogStartupMetrics |
| `map[string]any` | fields | structured log payload |

No domain structs, no interfaces implemented by the package.

## 6. Error taxonomy

| Function | Error / outcome |
|----------|-----------------|
| `StartFlightRecorder` | wrap `flight recorder start: %w`; or nil (including already-started) |
| `StopFlightRecorder` | always nil today |
| `DumpFlightRecord` | `flight recorder not started` / `not enabled` / empty nerdDir / mkdir / snapshot / write |
| `LogStartupMetrics` | no error return; must not panic (tested) |

## 7. Interaction with process lifecycle

```
process start
    main initializes logging + config
    metrics snapshot (always)
    optional StartFlightRecorder
    optional defer panic dump
    rootCmd.Execute (chat or cobra)
process exit
    runtime stops FlightRecorder if still running
    (package does not register atexit Stop)
```

## 8. What is intentionally absent

- Worker pools / background samplers  
- Channels or event buses  
- Persistence of metrics history  
- Abstraction over alternate tracers (Jaeger, etc.)  

The architecture is **two pure functions + one guarded singleton**, nothing more.
