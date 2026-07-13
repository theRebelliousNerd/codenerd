# TODO — observability

> Last verified against codebase: 2026-07-13  
> Package: `internal/observability/`  
> Scope: backlog for package + its host wiring (docs-only corpus; items are proposals)

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
