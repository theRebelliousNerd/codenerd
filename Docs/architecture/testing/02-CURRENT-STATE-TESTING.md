# testing — Current State

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/testing/` (complete internal coverage)
> **Implementation: `internal/testing/` — 21 non-test .go, 8 tests, 0 .mg**


## 1. Source location

- Primary package: `internal/testing/` (exists; 21 non-test Go files)
- 1:1 mapping: `Docs/architecture/testing/` ↔ `internal/testing/`

## 2. Largest source files

| Path | Lines | Kind |
|------|------:|------|
| `internal/testing/context_harness/simulator.go` | 1096 | source |
| `internal/testing/context_harness/scenarios.go` | 818 | source |
| `internal/testing/context_harness/scenarios_integration.go` | 493 | source |
| `internal/testing/context_harness/real_engine.go` | 484 | source |
| `internal/testing/context_harness/inspector.go` | 406 | source |
| `internal/testing/context_harness/mock_engine.go` | 395 | source |
| `internal/testing/context_harness/activation_tracer.go` | 320 | source |
| `internal/testing/context_harness/file_logger.go` | 293 | source |
| `internal/testing/context_harness/jit_tracer.go` | 281 | source |
| `internal/testing/context_harness/compression_viz.go` | 222 | source |
| `internal/testing/context_harness/feedback_tracer.go` | 212 | source |
| `internal/testing/context_harness/test_kernel_factory.go` | 178 | source |
| `internal/testing/context_harness/engine_interface.go` | 176 | source |
| `internal/testing/context_harness/reporter.go` | 171 | source |
| `internal/testing/context_harness/types.go` | 163 | source |
| `internal/testing/context_harness/piggyback_tracer.go` | 162 | source |
| `internal/testing/context_harness/harness.go` | 160 | source |
| `internal/testing/context_harness/fact_seeder.go` | 158 | source |
| `internal/testing/context_harness/metrics.go` | 135 | source |
| `internal/testing/harness_subsystem.go` | 3 | source |

## 3. Complete source inventory (capped)

| Path | Lines |
|------|------:|
| `internal/testing/context_harness/activation_tracer.go` | 320 |
| `internal/testing/context_harness/compression_viz.go` | 222 |
| `internal/testing/context_harness/engine_interface.go` | 176 |
| `internal/testing/context_harness/fact_seeder.go` | 158 |
| `internal/testing/context_harness/feedback_tracer.go` | 212 |
| `internal/testing/context_harness/file_logger.go` | 293 |
| `internal/testing/context_harness/harness.go` | 160 |
| `internal/testing/context_harness/inspector.go` | 406 |
| `internal/testing/context_harness/jit_tracer.go` | 281 |
| `internal/testing/context_harness/metrics.go` | 135 |
| `internal/testing/context_harness/mock_engine.go` | 395 |
| `internal/testing/context_harness/piggyback_tracer.go` | 162 |
| `internal/testing/context_harness/real_engine.go` | 484 |
| `internal/testing/context_harness/reporter.go` | 171 |
| `internal/testing/context_harness/scenarios.go` | 818 |
| `internal/testing/context_harness/scenarios_integration.go` | 493 |
| `internal/testing/context_harness/simulator.go` | 1096 |
| `internal/testing/context_harness/test_kernel_factory.go` | 178 |
| `internal/testing/context_harness/types.go` | 163 |
| `internal/testing/doc.go` | 2 |
| `internal/testing/harness_subsystem.go` | 3 |

## 4. Tests (sample)

| Path | Lines |
|------|------:|
| `internal/testing/context_harness/feedback_test.go` | 286 |
| `internal/testing/context_harness/simulator_test.go` | 168 |
| `internal/testing/context_harness/tracer_helpers_test.go` | 97 |
| `internal/testing/context_harness/seeder_logger_test.go` | 89 |
| `internal/testing/context_harness/reporter_test.go` | 68 |
| `internal/testing/context_harness/file_logger_test.go` | 64 |
| `internal/testing/context_harness/metrics_test.go` | 57 |
| `internal/testing/context_harness/helpers_extra_test.go` | 35 |

## 5. Behavior summary

Package **testing** is a living codeNERD subsystem: Internal test helpers and harness utilities.

APIs and control flow are defined by the cited source files — this corpus does not invent signatures.

## 6. Known limitations

- Inventory/type extraction is automated; deepen narrative when this package is the active design target.
- Completeness heuristic (70%) is not go cover %.
- Cross-package wiring claims require grep of registration hubs before treating as live.
