# 04 — Architectural Principles: observability

> Last verified against codebase: 2026-07-13  
> Status: Binding principles for this package  
> Package: `internal/observability/`

These principles are package-specific. Violating them usually means the feature belongs in `logging`, `transparency`, `prompt`, or a future APM package — not here.

## P1 — Leaf purity

**Statement:** Depend only on the standard library and `codenerd/internal/logging`. Never import `core`, `session`, `shards`, `config`, `features`, `tools`, or CLI packages.

**Evidence:** package header and import blocks in `runtime_metrics.go` / `flight_recorder.go`.

**Rationale:** Host diagnostics must boot before Cortex and must not create import cycles with config → store → core.

## P2 — Host wiring, not agent wiring

**Statement:** Only the process entrypoint (and future thin CLI diag verbs) start/dump. Kernel evaluation paths must not call into this package.

**Evidence:** sole non-test importer is `cmd/nerd/main.go`.

**Rationale:** Keeps executive control in Mangle; avoids metric collection becoming action routing.

## P3 — Prefer Go-native instruments

**Statement:** Use `runtime/metrics` and `runtime/trace.FlightRecorder` rather than inventing proprietary binary formats.

**Evidence:** `metrics.Read`, `trace.NewFlightRecorder`, dump files consumable by `go tool trace`.

**Rationale:** Forward-compat with the toolchain operators already know; less custom viewer debt.

## P4 — One-shot metrics, continuous ring

**Statement:** Startup metrics are a **snapshot**; the flight recorder is the **time window**. Do not blur them into a single API that samples everything every turn.

**Evidence:** `LogStartupMetrics` docs (“not monitoring”); ring buffer API separate.

## P5 — Fail open for the agent

**Statement:** Observability failures must not prevent `rootCmd.Execute`. Start errors print a warning; dump errors are swallowed on the panic path except re-panic of the original value.

**Evidence:** `main.go` warns on start failure; dump errors ignored when printing path; original panic rethrown.

**Rationale:** A coding agent that refuses to start because GC metrics failed is worse than one that starts without a ring.

## P6 — Singleton honesty

**Statement:** Model the Go runtime’s single FlightRecorder constraint explicitly (mutex + package var). Second Start is success-preserving no-op, not an error.

**Evidence:** `StartFlightRecorder` early return when `flight != nil`.

## P7 — Dump must not stop or tear recording

**Statement:** Snapshots buffer in memory first, write second; recorder remains enabled after dump.

**Evidence:** `bytes.Buffer` + `WriteTo` then `WriteFile`; double-dump test.

## P8 — Feature flag outside the leaf

**Statement:** This package does not read env or config itself for enablement. Callers gate `StartFlightRecorder` via `internal/features`.

**Evidence:** no `features` import; main checks `IsFlightRecorderEnabled`.

**Rationale:** Preserves leaf purity; keeps toggle policy with other modernization flags.

## P9 — Structured + human dual emit

**Statement:** Boot metrics emit a human summary line **and** a structured event with stable snake_case keys.

**Evidence:** `logger.Info` + `logger.StructuredLog("info", "runtime_metrics_startup", fields)`.

## P10 — Pin runtime metric names with tests

**Statement:** Every hard-coded metric path must appear in `runtime/metrics.All()` or tests fail.

**Evidence:** `TestStartupMetricPaths_AllSupported`.

## P11 — Workspace root semantics for dumps

**Statement:** `DumpFlightRecord` takes the **workspace root**, not the `.nerd` directory; it creates `.nerd/traces/` underneath.

**Evidence:** function doc and `filepath.Join(nerdDir, ".nerd", "traces")`.

## P12 — Do not absorb product telemetry

**Statement:** Glass box, transparency, LLM I/O logs, campaign artifacts, and prompt manifests stay out of this package even if they are “observability” in English.

**Rationale:** Scope creep would destroy leaf purity and couple host crash tooling to agent UX.
