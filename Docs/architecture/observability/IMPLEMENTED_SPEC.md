# observability — Implemented Spec

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/observability/` (complete internal coverage)
> **Implementation: `internal/observability/` — 2 non-test .go, 3 tests, 0 .mg**


## 1. Purpose

Flight recorder and runtime metrics

## 2. Source paths (1:1)

| Path | Role |
|------|------|
| `internal/observability/` | Primary implementation |
| `Docs/architecture/observability/` | This full corpus |

## 3. Implementation Status

> Living code status — **not** pre-implementation zeroing.

| Component | Status | Completion |
|-----------|--------|------------|
| Source package tree | Implemented | **90%** |
| Exported types (sampled) | Partial | **30%** |
| Tests | Implemented | **90%** |
| Mangle local sources | N/A or global-only | **n/a** |
| Full architecture corpus | Implemented | **100%** |

**Overall (heuristic): 90%** as living package (2 src / 3 tests)

## 4. Public surface inventory

### Largest files

| Path | Lines |
|------|------:|
| `internal/observability/runtime_metrics.go` | 188 | source |
| `internal/observability/flight_recorder.go` | 138 | source |

### Types (sampled)

| Type | Location |
|------|----------|
| — | — |

### Functions (sampled)

| Symbol | Location |
|--------|----------|
| `StartFlightRecorder` | `internal/observability/flight_recorder.go:36` |
| `StopFlightRecorder` | `internal/observability/flight_recorder.go:73` |
| `FlightRecorderEnabled` | `internal/observability/flight_recorder.go:87` |
| `DumpFlightRecord` | `internal/observability/flight_recorder.go:101` |
| `LogStartupMetrics` | `internal/observability/runtime_metrics.go:48` |

## 5. Integration relevance

| Surface | Relevance |
|---------|-----------|
| Kernel | Related |
| VirtualStore | Consumer if effectful |
| Shards | Related |
| Prompt JIT | Optional |
| CLI | Related via `cmd/nerd` |
| Config | Reader |

## 6. Non-goals of this corpus revision

- Full prose rewrite of every function body
- Docs/Spec 18-file product templates (`spec-doc-sprint`)
- Implementing missing features (corpus-build implementation mode)
