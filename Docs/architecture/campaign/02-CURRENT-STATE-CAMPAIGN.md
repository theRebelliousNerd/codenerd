# campaign — Current State

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded corpus**
> Mode: dark-factory autonomous generation via arch-propose/corpus-build port
> **Implementation: present under `internal/campaign/` (44 non-test .go, 29 tests, 1 .mg)**


## 1. Source location

- Primary package: `internal/campaign/` (**exists** with 44 non-test Go files)
- Supporting global surfaces: `internal/core/defaults/` when schemas/policy apply

## 2. File inventory (largest sources)

| Path | Lines | Kind |
|------|------:|------|
| `internal/campaign/assault_tasks.go` | 1157 | source |
| `internal/campaign/prompts.go` | 1072 | source |
| `internal/campaign/decomposer.go` | 1059 | source |
| `internal/campaign/edge_case_detector.go` | 1057 | source |
| `internal/campaign/risk_scoring.go` | 1051 | source |
| `internal/campaign/replan.go` | 997 | source |
| `internal/campaign/types.go` | 916 | source |
| `internal/campaign/orchestrator_task_handlers.go` | 718 | source |
| `internal/campaign/shard_advisory_board.go` | 671 | source |
| `internal/campaign/tool_pregenerator.go` | 656 | source |
| `internal/campaign/intelligence_gatherer.go` | 652 | source |
| `internal/campaign/decomposer_requirements.go` | 591 | source |

## 3. Test inventory (sample)

| Path | Lines |
|------|------:|
| `internal/campaign/context_pager_test.go` | 1260 |
| `internal/campaign/decomposer_test.go` | 1058 |
| `internal/campaign/shard_advisory_board_test.go` | 722 |
| `internal/campaign/replan_test.go` | 653 |
| `internal/campaign/orchestrator_task_handlers_test.go` | 629 |
| `internal/campaign/risk_scoring_test.go` | 611 |

## 4. Current behavior (summary)

Package **campaign** is a living codeNERD subsystem: Multi-phase goal orchestration and context paging.

Behavior is defined by the source files above. This corpus does **not** invent APIs —
consult the cited paths for signatures and control flow.

## 5. Known limitations (honest)

- Corpus generated in dark-factory mode from inventory + lightweight type extraction; deep behavioral narrative may lag micro-refactors.
- Completeness heuristic (85%) is not coverage % — run `go test` for truth.
- Cross-package wiring must be validated against `internal/shards/registration.go` and VirtualStore routes when relevant.
