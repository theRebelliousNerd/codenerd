# TODO — observability

> Last verified against codebase: 2026-09-25 (lane B wave 3)  
> Package: `internal/observability/`  
> Scope: backlog for package + its host wiring (docs-only corpus; items are proposals)

## Status 2026-09-25

| ID | Status | Evidence |
|----|--------|----------|
| T0.1 | Closed | `internal/features/features.go` now names `/flightrec`, which exists |
| T0.2 | Standing | panic dump is still main()'s defer only; see T1.3 |
| T1.1 | Closed | chat `/flightrec` (`cmd/nerd/chat/diagnostics.go` `handleCmdFlightrec`) calls `DumpFlightRecord` and prints the path; `/status` reports `FlightRecorderEnabled` |
| T1.2 | Already done | `cmd/nerd/main.go` passes `ws` (the `--workspace`-resolved root) to the panic dump |
| T1.3 | Declined | a recovered chat panic is handled state; an operator who wants the ring runs `/flightrec` |
| T2.1 | Declined | no reported need; 64 MiB / 30 s stay constants in main |
| T2.2 | Closed (stop, not dump) | main defers `StopFlightRecorder` so the recorder and its watchdog stop on a normal return, ordered after the panic dump |
| T3.1 | Closed | `DumpFlightRecord` keeps the newest `maxFlightTraces` (10) under `.nerd/traces/`; `TestDumpFlightRecord_WhenTracesAccumulate_ShouldKeepOnlyTheNewest` |
| T3.2 | Already done | `flightDumpSeq` suffix on every dump name |
| T3.3 | Closed via /status | the Diagnostics block is the mid-session surface |
| T4.1 | Declined | a real panic dump test needs a subprocess harness; the lifecycle tests cover the dump path |
| T4.2 | Open | no structured-field assertions on the dump log line |

## P0 — Trust / documentation correctness

| ID | Item | Notes |
|----|------|-------|
| T0.1 | Align `features.go` comment with reality on `/diag flightrec` | Comment claims dump path that is not wired |
| T0.2 | Keep panic-scope honesty in CLI/ops docs | Main defer only |

## P1 — Product wiring

| ID | Item | Notes |
|----|------|-------|
| T1.1 | Implement on-demand dump CLI or slash command | Call existing `DumpFlightRecord`; print path |
| T1.2 | Pass effective workspace into panic dump | Fix `--workspace` chdir skew |
| T1.3 | Optional dump from chat recover path | Only if product wants forensics for recovered panics |

## P2 — Configurability

| ID | Item | Notes |
|----|------|-------|
| T2.1 | Config keys for ring `MaxBytes` / `MinAge` | Defaults remain 64 MiB / 30 s |
| T2.2 | Graceful-shutdown optional dump | `PersistentPostRun` or signal handler |

## P3 — Ops hygiene

| ID | Item | Notes |
|----|------|-------|
| T3.1 | Trace retention / rotation under `.nerd/traces/` | Age or count based |
| T3.2 | Nanosecond or counter suffix on filenames | Avoid second-resolution overwrite |
| T3.3 | Mid-session metrics helper | Only if operators need status integration |

## P4 — Testing

| ID | Item | Notes |
|----|------|-------|
| T4.1 | Optional integration test for main panic dump | Build-tag / manual stress |
| T4.2 | Assert structured field keys if logging test hooks allow | |

## Explicit non-TODO

- OpenTelemetry exporter in this package  
- Mangle predicates for GC stats  
- Importing core/session into observability  
- Replacing glass box or prompt manifest  

## Done (living baseline — do not re-open as greenfield)

- [x] `LogStartupMetrics`  
- [x] Green Tea detection + warn  
- [x] Metric path support test  
- [x] Flight recorder singleton Start/Stop/Enabled/Dump  
- [x] Buffer-then-write dump  
- [x] Panic dump wiring in `cmd/nerd/main.go`  
- [x] Feature flag + env override  
- [x] Lifecycle + double-dump tests  
