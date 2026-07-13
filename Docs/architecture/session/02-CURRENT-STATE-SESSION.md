# session — Current State

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/session/` (complete internal coverage)
> **Implementation: `internal/session/` — 6 non-test .go, 14 tests, 0 .mg**


## 1. Source location

- Primary package: `internal/session/` (exists; 6 non-test Go files)
- 1:1 mapping: `Docs/architecture/session/` ↔ `internal/session/`

## 2. Largest source files

| Path | Lines | Kind |
|------|------:|------|
| `internal/session/executor.go` | 1057 | source |
| `internal/session/executor_tools.go` | 587 | source |
| `internal/session/spawner.go` | 516 | source |
| `internal/session/subagent.go` | 470 | source |
| `internal/session/task_executor.go` | 415 | source |
| `internal/session/semantic_compressor.go` | 104 | source |

## 3. Complete source inventory (capped)

| Path | Lines |
|------|------:|
| `internal/session/executor.go` | 1057 |
| `internal/session/executor_tools.go` | 587 |
| `internal/session/semantic_compressor.go` | 104 |
| `internal/session/spawner.go` | 516 |
| `internal/session/subagent.go` | 470 |
| `internal/session/task_executor.go` | 415 |

## 4. Tests (sample)

| Path | Lines |
|------|------:|
| `internal/session/executor_process_test.go` | 835 |
| `internal/session/task_executor_test.go` | 563 |
| `internal/session/executor_test.go` | 306 |
| `internal/session/subagent_test.go` | 279 |
| `internal/session/spawner_test.go` | 278 |
| `internal/session/mocks_test.go` | 269 |
| `internal/session/spawner_improvements_test.go` | 263 |
| `internal/session/spawner_gaps_test.go` | 258 |
| `internal/session/semantic_compressor_test.go` | 181 |
| `internal/session/executor_boundary_test.go` | 178 |

## 5. Behavior summary

Package **session** is a living codeNERD subsystem: Session execution loop and clean executor.

APIs and control flow are defined by the cited source files — this corpus does not invent signatures.

## 6. Known limitations

- Inventory/type extraction is automated; deepen narrative when this package is the active design target.
- Completeness heuristic (90%) is not go cover %.
- Cross-package wiring claims require grep of registration hubs before treating as live.
