# observability — Domain Model

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/observability/` (complete internal coverage)
> **Implementation: `internal/observability/` — 2 non-test .go, 3 tests, 0 .mg**


## Package

`internal/observability/`

## Exported types (sampled, up to 40)

| Type | Location |
|------|----------|
| — | — |

## Exported functions/methods (sampled, up to 30)

| Symbol | Location |
|--------|----------|
| `StartFlightRecorder` | `internal/observability/flight_recorder.go:36` |
| `StopFlightRecorder` | `internal/observability/flight_recorder.go:73` |
| `FlightRecorderEnabled` | `internal/observability/flight_recorder.go:87` |
| `DumpFlightRecord` | `internal/observability/flight_recorder.go:101` |
| `LogStartupMetrics` | `internal/observability/runtime_metrics.go:48` |

## Mangle surface

| Artifact | Count |
|----------|------:|
| Package-local `.mg` | 0 |

| Path | Lines |
|------|------:|
| — | 0 |

Global kernel schemas/policy (when this package participates):

- `internal/core/defaults/schemas.mg`
- `internal/core/defaults/policy/`

## Fact-flow placement

```
user input → perception → user_intent → kernel(core/mangle) → next_action
  → VirtualStore / shards / tools → articulation
```

This package: **Flight recorder and runtime metrics**
