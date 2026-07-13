# observability — Architecture Corpus (`internal/observability`)

> Last verified against codebase: 2026-07-13  
> Status: Living Reference Document — code-grounded  
> Language: Go (module `codenerd`, `go 1.26.0`)  
> Primary package: `internal/observability/`  
> Scale: **2** non-test Go files ≈ **326** lines; **3** test files ≈ **369** lines; **0** `.mg`

## Scope

This corpus documents the **process-level runtime observability leaf package**:

1. **Startup metrics** — one-shot `runtime/metrics` + GOMAXPROCS/CPU/Go version + Green Tea GC status, emitted on the boot logger.
2. **Flight recorder** — process-wide `runtime/trace.FlightRecorder` ring buffer (Go 1.25+), started at binary entry when the feature flag is on, dumped to `.nerd/traces/` on panic.

It is **not**:

- The categorized log sink (`internal/logging/`) — observability only *emits* via `logging.CategoryBoot`.
- Operator glass-box / transparency / reflection (CLI + `internal/transparency`).
- Prompt-compiler “Flight Recorder” (`internal/prompt/manifest.go` — different concept, shared metaphor only).
- Continuous APM, OpenTelemetry exporters, or Mangle fact-stream telemetry.

## Document map

| Doc | Role |
|-----|------|
| [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md) | Authoritative living architecture + inventory |
| [00-ALIGNMENT-VISION-REVIEW.md](00-ALIGNMENT-VISION-REVIEW.md) | North-star alignment scores with evidence |
| [01-VISION.md](01-VISION.md) | Target product/architecture vision for this package |
| [02-CURRENT-STATE.md](02-CURRENT-STATE.md) | Precise on-disk inventory and hotspots |
| [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md) | Gaps vs vision, priorities, non-gaps |
| [04-ARCHITECTURAL-PRINCIPLES.md](04-ARCHITECTURAL-PRINCIPLES.md) | Binding design principles |
| [05-INTERNAL-ARCHITECTURE.md](05-INTERNAL-ARCHITECTURE.md) | Components, data flow, state machine |
| [06-PUBLIC-API-AND-TYPES.md](06-PUBLIC-API-AND-TYPES.md) | Exported funcs and unexported helpers |
| [07-DEPENDENCY-MAP.md](07-DEPENDENCY-MAP.md) | Upstream/downstream packages with evidence |
| [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md) | Boot path in `cmd/nerd/main.go` + feature flags |
| [09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md) | Concurrency, panic dump, default-deny of side effects |
| [10-TESTING-ALIGNMENT.md](10-TESTING-ALIGNMENT.md) | Tests, gaps, commands |
| [11-OBSERVABILITY.md](11-OBSERVABILITY.md) | Logging categories, metric paths, trace artifacts |
| [12-FAILURE-MODES.md](12-FAILURE-MODES.md) | Concrete failure modes + mitigations |
| [TODO.md](TODO.md) / [OPEN-QUESTIONS.md](OPEN-QUESTIONS.md) / [_progress.md](_progress.md) | Governance |

## Role in fact-flow

```
user_intent → kernel → next_action → VirtualStore → articulation
         ▲
         │  (observability does NOT sit on this path)
         │
  nerd process boot ──► LogStartupMetrics()
                    ──► StartFlightRecorder() [if features.IsFlightRecorderEnabled]
                    ──► defer DumpFlightRecord on panic
```

Observability is **host infrastructure**, not executive logic. It never asserts Mangle facts, never routes actions, and never participates in `permitted(...)`.

## Verify

```powershell
# Unit + lifecycle tests (flight recorder needs real wall-clock; not synctest)
go test ./internal/observability/...

# Race (singleton mutex paths)
go test -race ./internal/observability/...

# Feature gate (optional — env wins over config)
$env:NERD_FLIGHTREC = "0"   # disable ring buffer at boot
$env:NERD_FLIGHTREC = "1"   # force on

# Post-mortem of a dumped trace (after a panic dump or test)
go tool trace .nerd/traces/flight_YYYYMMDDTHHMMSSZ.trace
```

## Related corpora

| Corpus | Relationship |
|--------|----------------|
| [`Docs/architecture/logging/`](../logging/) | Sink for boot INFO/WARN/structured events |
| [`Docs/architecture/features/`](../features/) | `IsFlightRecorderEnabled`, `NERD_FLIGHTREC` |
| [`Docs/architecture/cli/`](../cli/) | Sole production importer (`cmd/nerd/main.go`); see also CLI [12-TELEMETRY-OBSERVABILITY.md](../cli/12-TELEMETRY-OBSERVABILITY.md) |
| [`Docs/architecture/config/`](../config/) | Loads `.nerd/config.json` → `features.SetActive` before flight-recorder gate |

## Quality bar

Modeled on `Docs/architecture/cli/`: real inventories, control-flow diagrams, wiring journal, honest gaps — **not** auto-generated inventory stubs.
