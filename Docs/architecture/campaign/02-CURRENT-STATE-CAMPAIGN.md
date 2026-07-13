# campaign — Current State

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/campaign/` (complete internal coverage)
> **Implementation: `internal/campaign/` — 44 non-test .go, 29 tests, 1 .mg**


## 1. Source location

- Primary package: `internal/campaign/` (exists; 44 non-test Go files)
- 1:1 mapping: `Docs/architecture/campaign/` ↔ `internal/campaign/`

## 2. Largest source files

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
| `internal/campaign/intelligence_gathering_methods.go` | 586 | source |
| `internal/campaign/decomposer_planning.go` | 575 | source |
| `internal/campaign/orchestrator_tasks.go` | 542 | source |
| `internal/campaign/context_pager.go` | 506 | source |
| `internal/campaign/checkpoint.go` | 477 | source |
| `internal/campaign/orchestrator_failure.go` | 434 | source |
| `internal/campaign/orchestrator_init.go` | 381 | source |
| `internal/campaign/decomposer_documents.go` | 363 | source |

## 3. Complete source inventory (capped)

| Path | Lines |
|------|------:|
| `internal/campaign/assault_campaign.go` | 184 |
| `internal/campaign/assault_prompts.go` | 55 |
| `internal/campaign/assault_tasks.go` | 1157 |
| `internal/campaign/assault_types.go` | 116 |
| `internal/campaign/campaign_fact_sync.go` | 243 |
| `internal/campaign/campaign_prompts.go` | 124 |
| `internal/campaign/checkpoint.go` | 477 |
| `internal/campaign/context_pager.go` | 506 |
| `internal/campaign/decomposer.go` | 1059 |
| `internal/campaign/decomposer_documents.go` | 363 |
| `internal/campaign/decomposer_planning.go` | 575 |
| `internal/campaign/decomposer_requirements.go` | 591 |
| `internal/campaign/document_ingestor.go` | 91 |
| `internal/campaign/edge_case_detector.go` | 1057 |
| `internal/campaign/errors.go` | 38 |
| `internal/campaign/intelligence_formatting.go` | 119 |
| `internal/campaign/intelligence_gatherer.go` | 652 |
| `internal/campaign/intelligence_gathering_methods.go` | 586 |
| `internal/campaign/micro_checkpoint.go` | 69 |
| `internal/campaign/normalization.go` | 101 |
| `internal/campaign/orchestrator.go` | 20 |
| `internal/campaign/orchestrator_control.go` | 140 |
| `internal/campaign/orchestrator_execution.go` | 231 |
| `internal/campaign/orchestrator_failure.go` | 434 |
| `internal/campaign/orchestrator_init.go` | 381 |
| `internal/campaign/orchestrator_journal.go` | 302 |
| `internal/campaign/orchestrator_lifecycle.go` | 173 |
| `internal/campaign/orchestrator_phases.go` | 296 |
| `internal/campaign/orchestrator_task_handlers.go` | 718 |
| `internal/campaign/orchestrator_task_results.go` | 149 |
| `internal/campaign/orchestrator_task_transaction.go` | 279 |
| `internal/campaign/orchestrator_tasks.go` | 542 |
| `internal/campaign/orchestrator_types.go` | 157 |
| `internal/campaign/orchestrator_utils.go` | 295 |
| `internal/campaign/prompts.go` | 1072 |
| `internal/campaign/replan.go` | 997 |
| `internal/campaign/risk_scoring.go` | 1051 |
| `internal/campaign/shard_advisory_board.go` | 671 |
| `internal/campaign/specialist_knowledge.go` | 174 |
| `internal/campaign/task_mutation_types.go` | 21 |
| `internal/campaign/tool_pregenerator.go` | 656 |
| `internal/campaign/types.go` | 916 |
| `internal/campaign/utils.go` | 121 |
| `internal/campaign/write_set_lock_manager.go` | 251 |

## 4. Tests (sample)

| Path | Lines |
|------|------:|
| `internal/campaign/context_pager_test.go` | 1260 |
| `internal/campaign/decomposer_test.go` | 1058 |
| `internal/campaign/shard_advisory_board_test.go` | 722 |
| `internal/campaign/replan_test.go` | 653 |
| `internal/campaign/orchestrator_task_handlers_test.go` | 629 |
| `internal/campaign/risk_scoring_test.go` | 611 |
| `internal/campaign/types_test.go` | 600 |
| `internal/campaign/assault_tasks_test.go` | 543 |
| `internal/campaign/edge_case_detector_test.go` | 461 |
| `internal/campaign/tool_pregenerator_test.go` | 416 |

## 5. Behavior summary

Package **campaign** is a living codeNERD subsystem: Multi-phase goal orchestration, decomposition, context paging.

APIs and control flow are defined by the cited source files — this corpus does not invent signatures.

## 6. Known limitations

- Inventory/type extraction is automated; deepen narrative when this package is the active design target.
- Completeness heuristic (85%) is not go cover %.
- Cross-package wiring claims require grep of registration hubs before treating as live.
