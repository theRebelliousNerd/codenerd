# session — Current State

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded corpus**
> Mode: dark-factory autonomous generation via arch-propose/corpus-build port
> **Implementation: present under `internal/session/` (6 non-test .go, 14 tests, 0 .mg)**


## 1. Source location

- Primary package: `internal/session/` (**exists** with 6 non-test Go files)
- Supporting global surfaces: `internal/core/defaults/` when schemas/policy apply

## 2. File inventory (largest sources)

| Path | Lines | Kind |
|------|------:|------|
| `internal/session/executor.go` | 1057 | source |
| `internal/session/executor_tools.go` | 587 | source |
| `internal/session/spawner.go` | 516 | source |
| `internal/session/subagent.go` | 470 | source |
| `internal/session/task_executor.go` | 415 | source |
| `internal/session/semantic_compressor.go` | 104 | source |

## 3. Test inventory (sample)

| Path | Lines |
|------|------:|
| `internal/session/executor_process_test.go` | 835 |
| `internal/session/task_executor_test.go` | 563 |
| `internal/session/executor_test.go` | 306 |
| `internal/session/subagent_test.go` | 279 |
| `internal/session/spawner_test.go` | 278 |
| `internal/session/mocks_test.go` | 269 |

## 4. Current behavior (summary)

Package **session** is a living codeNERD subsystem: Clean execution loop / session executor.

Behavior is defined by the source files above. This corpus does **not** invent APIs —
consult the cited paths for signatures and control flow.

## 5. Known limitations (honest)

- Corpus generated in dark-factory mode from inventory + lightweight type extraction; deep behavioral narrative may lag micro-refactors.
- Completeness heuristic (90%) is not coverage % — run `go test` for truth.
- Cross-package wiring must be validated against `internal/shards/registration.go` and VirtualStore routes when relevant.
