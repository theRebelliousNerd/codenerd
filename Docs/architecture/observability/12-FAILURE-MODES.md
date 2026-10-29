# 12 — Failure Modes: observability

> Last verified against codebase: 2026-07-14  
> Package: `internal/observability/`

## FM1 — Flight recorder fails to start

| | |
|--|--|
| **Symptoms** | stderr: `Warning: flight recorder failed to start: …`; no start log line |
| **Causes** | Runtime rejects second process-wide recorder; unsupported platform edge; Start error from `trace.FlightRecorder` |
| **Impact** | No ring; no panic dump defer installed |
| **Mitigation** | Process continues; investigate runtime error; ensure no other code starts a raw `trace.FlightRecorder` |
| **Detection** | Missing boot Info `flight recorder started`; `FlightRecorderEnabled()==false` with flag on |

## FM2 — Dump without active recorder

| | |
|--|--|
| **Symptoms** | `DumpFlightRecord` returns `flight recorder not started` or `not enabled` |
| **Causes** | Flag off; start failed; Stop already called; dump from tests without Start |
| **Impact** | No trace file; panic path prints nothing |
| **Mitigation** | Ensure Start success before relying on dump; for ops enable `NERD_FLIGHTREC=1` |

## FM3 — Empty or unwritable workspace path

| | |
|--|--|
| **Symptoms** | Error `nerdDir must be non-empty` or `create traces dir: …` / `write flight trace: …` |
| **Causes** | Empty string; permission denied; read-only FS; disk full |
| **Impact** | No artifact; panic still re-raised |
| **Mitigation** | Run from writable workspace; check disk free space (disk-guard floors on constrained hosts) |

## FM4 — Snapshot succeeds, disk write fails

| | |
|--|--|
| **Symptoms** | `write flight trace: …` after successful `WriteTo` |
| **Causes** | Disk full; ACL change mid-write |
| **Impact** | No file; **recorder not torn** (buffer-first design) |
| **Mitigation** | Free disk; retry Dump |

## FM5 — Panic recovered inside chat never dumps

| | |
|--|--|
| **Symptoms** | User sees recovered error in TUI; no `Flight trace dumped` stderr; no new file under `.nerd/traces` |
| **Causes** | `chat/process.go` (and similar) recover without re-panicking to main |
| **Impact** | Forensic gap for non-fatal panics |
| **Mitigation** | Treat as design; for hard crashes force process exit; future: optional dump from chat recover |
| **Honesty rule** | Do not claim “all panics dump” |

## FM6 — Dump lands under wrong directory with `--workspace`

| | |
|--|--|
| **Symptoms** | Trace under CWD `.nerd/traces` while agent used another workspace tree |
| **Causes** | main captures `Getwd` before `Chdir` in interactive RunE |
| **Impact** | Operator looks in wrong tree |
| **Mitigation** | Search both CWD and `--workspace`; future fix: pass effective workspace into dump |

## FM7 — Metric path retired by Go upgrade

| | |
|--|--|
| **Symptoms** | CI red on `TestStartupMetricPaths_AllSupported`; runtime shows `n/a` for keys if test skipped |
| **Causes** | Go renames/removes `runtime/metrics` paths |
| **Impact** | Incomplete boot snapshot |
| **Mitigation** | Update `startupMetricPaths` to supported replacements; keep test mandatory |

## FM8 — Green Tea disabled unexpectedly

| | |
|--|--|
| **Symptoms** | Boot WARN about Green Tea; `greentea_gc=false` in structured fields |
| **Causes** | `GOEXPERIMENT=nogreenteagc` env or build |
| **Impact** | Different GC behavior/perf than Go 1.26 default |
| **Mitigation** | Clear experiment if unintended; treat warn as actionable on prod hosts |

## FM9 — Ring window empty or tiny

| | |
|--|--|
| **Symptoms** | Dump file near header-only (~16+ bytes) |
| **Causes** | Dump immediately after Start with no scheduling activity; virtualized test time (avoided in prod tests) |
| **Impact** | Trace not useful for post-mortem |
| **Mitigation** | Keep MinAge and process activity; dump after meaningful work |

## FM10 — Timestamp collision on dumps

| | |
|--|--|
| **Symptoms** | Second dump same second overwrites? **WriteFile overwrites same path** if timestamps equal |
| **Causes** | Filename second resolution `20060102T150405Z` |
| **Impact** | Lost earlier dump if two dumps same second |
| **Mitigation** | Tests sleep >1s; production panic usually single dump; future: monotonic suffix / nanoseconds |

## FM11 — Memory pressure from 64 MiB ring

| | |
|--|--|
| **Symptoms** | Higher RSS; constrained VMs OOM edge |
| **Causes** | Production MaxBytes 64 MiB always reserved behavior per runtime |
| **Impact** | Host stress |
| **Mitigation** | `NERD_FLIGHTREC=0` or future smaller config; prefer disable over deleting package. Distinct from the *fatal* runaway in FM13 — this row is steady-state RSS, not the tracer's unbounded region growth. |

## FM12 — Naming confusion with prompt “Flight Recorder”

| | |
|--|--|
| **Symptoms** | Operator opens prompt manifest expecting `go tool trace` file |
| **Causes** | Shared metaphor in docs/comments |
| **Impact** | Wasted triage time |
| **Mitigation** | This corpus; distinguish execution-trace vs PromptManifest |

## FM13 — Fatal `traceRegion: out of memory` under heavy/long load

| | |
|--|--|
| **Symptoms** | Whole `nerd` process dies with `fatal error: traceRegion: out of memory` (exit 2). Stack is entirely in the Go runtime tracer: `runtime.(*traceRegionAlloc).alloc` → `runtime.(*traceMap).put` → `runtime.traceStack`. First seen on a heavy `campaign start … --type audit` run (2026-07-13). |
| **Causes** | The execution tracer keeps interning tables (stacks/strings) and generation buffers for the life of the trace. A long run with massive subprocess spawning + goroutine churn (many deep unique stacks) grows the tracer's region allocator until `sysAlloc` fails and the runtime aborts. It is a `fatal error`, **not** a panic — `defer`/`recover` cannot catch it, so the panic-dump defer in `main.go` never runs. |
| **Impact** | Total loss of the workload (contract-#9 no-crash violation). Because a *debug/observability* feature caused it, the blast radius is the entire agent run. |
| **Mitigation** | Two layers: (1) the compile-time default is now **off** — `IsFlightRecorderEnabled()` and `DefaultFeaturesConfig().FlightRecorder` default `false`; the recorder is opt-in via `NERD_FLIGHTREC=1` / `features.flight_recorder: true`. (2) When enabled, `StartFlightRecorder` runs a **memory watchdog** (`flightMemWatchdog`) that samples `/memory/classes/other:bytes` every `flightWatchdogInterval` (2 s) and, once trace-attributed growth passes the cap (`max(256 MiB, 4× ring)`), stops the recorder and logs a boot WARN. The recorder degrades to *off* instead of crashing the host; it stays stopped for the rest of the run. |
| **Detection** | Boot WARN `flight recorder auto-stopped: trace memory grew N bytes (> M guard)`; `FlightRecorderEnabled()` flips to `false` mid-run. If the recorder is on for a known-heavy campaign, prefer disabling it up front (`NERD_FLIGHTREC=0`) rather than relying on the watchdog. |
| **Tests** | `flight_recorder_watchdog_test.go`: `TestFlightWatchdog_StopsRecorderOnMemoryGrowth` (cap 0 → trips on first growth), `TestFlightWatchdog_NoTripUnderCap` (no false-fire), `TestFlightWatchdog_ExternalStopExitsWatchdog` (clean generation handoff). |

## Incident quick checklist

1. Is `features.flight_recorder` / `NERD_FLIGHTREC` on?  
2. Does boot log show `flight recorder started`?  
3. Did the panic reach main, or was it recovered in chat?  
4. Search `.nerd/traces/` under CWD and `--workspace`.  
5. `go tool trace` the file; confirm magic `go 1.`.  
6. Review boot `runtime_metrics_startup` for GOMAXPROCS / greentea / heap.  
7. If disk/full errors, free space and consider manual dump path once on-demand wiring exists.
