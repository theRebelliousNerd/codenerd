# 08 — Wiring and Integration: observability

> Last verified against codebase: 2026-07-13  
> Package: `internal/observability/`  
> Production wire site: `cmd/nerd/main.go` only

## 1. Boot sequence (authoritative)

Source: `cmd/nerd/main.go` function `main()` (approx lines 319–370).

```
1. logging.Initialize(os.Getwd())
     // idempotent; enables CategoryBoot for metrics line

2. config.GlobalConfig()
     // errors → stderr warning; features fall back to defaults
     // side effect: features.SetActive from .nerd/config.json when present

3. observability.LogStartupMetrics()
     // always; not feature-gated

4. if features.IsFlightRecorderEnabled():
     4a. observability.StartFlightRecorder(64<<20, 30*time.Second)
         - on error: stderr "Warning: flight recorder failed to start"
         - on success: install defer recover:
              recover → DumpFlightRecord(ws) → print path → re-panic
5. rootCmd.Execute()
```

Order matters: metrics without logging init lose file sinks; flight gate without config load ignores user `features.flight_recorder` until env override.

## 2. Feature gate resolution path

```
NERD_FLIGHTREC env (1/true/0/false)
        │ if set meaningfully
        ▼
features.IsFlightRecorderEnabled()
        │ else
active FeaturesConfig.FlightRecorder  (from config.SetActive)
        │ else
compile default true  (DefaultFeaturesConfig / resolveBool def)
```

Related files:

| File | Role |
|------|------|
| `internal/features/features.go` | `IsFlightRecorderEnabled`, defaults |
| `internal/config/user_config.go` | Loads features into registry (comments reference observability) |
| `.nerd/config.json` | Runtime `features.flight_recorder` key (workspace-local) |

## 3. What is **not** wired

| Claim / temptation | Status |
|--------------------|--------|
| Chat slash `/diag flightrec` | **Not present** (features comment only) |
| Cobra subcommand dumping traces | **Not present** |
| `StopFlightRecorder` on process exit | **Not called** (runtime stop sufficient) |
| Dump on SIGTERM / graceful shutdown | **Not present** |
| Session boot (`chat/session_boot.go`) re-start | **Not present** — single start at main |
| Cortex / VirtualStore hooks | **Not present** |

## 4. Interactive vs non-interactive

Both paths share `main()` before Cobra:

| Mode | Metrics | Flight recorder |
|------|---------|-----------------|
| Interactive chat (`rootCmd.RunE` → chat) | Yes | If flag on |
| Non-interactive Cobra verbs | Yes | If flag on |
| `PersistentPreRunE` logging init | Secondary; main already initialized | N/A |

Interactive chat may `Chdir` to `--workspace` **after** main’s `ws` snapshot used for panic dump. See gap: dump directory vs workspace flag.

## 5. Panic dump integration detail

```go
defer func() {
    if r := recover(); r != nil {
        nerdDir := ws // from Getwd at start of main
        if nerdDir == "" {
            nerdDir, _ = os.Getwd()
        }
        if path, err := observability.DumpFlightRecord(nerdDir); err == nil {
            fmt.Fprintf(os.Stderr, "Flight trace dumped to %s\n", path)
        }
        panic(r)
    }
}()
```

| Property | Behavior |
|----------|----------|
| Install condition | Only if Start succeeded |
| Failed dump | Silent (no stderr path); original panic continues |
| Successful dump | Path printed to stderr |
| Panic value | Always re-raised |

## 6. Integration with logging categories

| Event | Category | Level | Message / event |
|-------|----------|-------|-----------------|
| Metrics human line | Boot | Info | `runtime metrics snapshot: …` |
| Metrics structured | Boot | info structured | event name `runtime_metrics_startup` |
| Green Tea disabled | Boot | Warn | GOEXPERIMENT guidance |
| Recorder started | Boot | Info | `flight recorder started (max_bytes=… min_age=…)` |
| Recorder dumped | Boot | Info | `flight recorder dumped: path=… bytes=…` |

No dedicated `CategoryObservability` exists; boot category is deliberate (startup-adjacent).

## 7. Wiring audit checklist (for future changes)

Before claiming “flight recorder unused”:

1. Grep `StartFlightRecorder` / `DumpFlightRecord` in `cmd/`.  
2. Grep `IsFlightRecorderEnabled` in `internal/features` + call sites.  
3. Confirm panic defer still present after main refactors.  
4. Confirm tests still force Stop (process-wide singleton).  

Do **not** delete the package based on low import count — one strategic importer is correct for a leaf host utility.

## 8. Suggested future wiring (not implemented)

| Wire | Where | Call |
|------|-------|------|
| On-demand dump | `cmd/nerd/chat` slash handler or `cmd_diag.go` | `DumpFlightRecord(effectiveWorkspace)` |
| Graceful dump | `PersistentPostRun` or signal handler | optional Dump + Stop |
| Status line | `status` command | `FlightRecorderEnabled()` |

Any of these keep the leaf pure; only host/CLI grows.
