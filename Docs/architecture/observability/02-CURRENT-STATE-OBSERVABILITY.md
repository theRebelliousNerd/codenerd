# observability — Current State

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/observability/` (complete internal coverage)
> **Implementation: `internal/observability/` — 2 non-test .go, 3 tests, 0 .mg**


## 1. Source location

- Primary package: `internal/observability/` (exists; 2 non-test Go files)
- 1:1 mapping: `Docs/architecture/observability/` ↔ `internal/observability/`

## 2. Largest source files

| Path | Lines | Kind |
|------|------:|------|
| `internal/observability/runtime_metrics.go` | 188 | source |
| `internal/observability/flight_recorder.go` | 138 | source |

## 3. Complete source inventory (capped)

| Path | Lines |
|------|------:|
| `internal/observability/flight_recorder.go` | 138 |
| `internal/observability/runtime_metrics.go` | 188 |

## 4. Tests (sample)

| Path | Lines |
|------|------:|
| `internal/observability/flight_recorder_lifecycle_test.go` | 147 |
| `internal/observability/flight_recorder_test.go` | 121 |
| `internal/observability/runtime_metrics_test.go` | 101 |

## 5. Behavior summary

Package **observability** is a living codeNERD subsystem: Flight recorder and runtime metrics.

APIs and control flow are defined by the cited source files — this corpus does not invent signatures.

## 6. Known limitations

- Inventory/type extraction is automated; deepen narrative when this package is the active design target.
- Completeness heuristic (90%) is not go cover %.
- Cross-package wiring claims require grep of registration hubs before treating as live.
