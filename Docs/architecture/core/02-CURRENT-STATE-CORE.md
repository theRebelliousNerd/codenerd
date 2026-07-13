# core — Current State

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/core/` (complete internal coverage)
> **Implementation: `internal/core/` — 78 non-test .go, 107 tests, 129 .mg**


## 1. Source location

- Primary package: `internal/core/` (exists; 78 non-test Go files)
- 1:1 mapping: `Docs/architecture/core/` ↔ `internal/core/`

## 2. Largest source files

| Path | Lines | Kind |
|------|------:|------|
| `internal/core/kernel_facts.go` | 1255 | source |
| `internal/core/virtual_store.go` | 1077 | source |
| `internal/core/virtual_store_workflows.go` | 1029 | source |
| `internal/core/virtual_store_actions.go` | 1008 | source |
| `internal/core/virtual_store_predicates.go` | 993 | source |
| `internal/core/api_scheduler.go` | 871 | source |
| `internal/core/scheduled_llm_client.go` | 854 | source |
| `internal/core/tdd_loop.go` | 833 | source |
| `internal/core/dreamer.go` | 753 | source |
| `internal/core/cortex_kernel.go` | 731 | source |
| `internal/core/kernel_eval.go` | 730 | source |
| `internal/core/virtual_store_codedom.go` | 706 | source |
| `internal/core/predicate_corpus.go` | 636 | source |
| `internal/core/shards/manager_spawn.go` | 622 | source |
| `internal/core/kernel_query.go` | 595 | source |
| `internal/core/kernel_init.go` | 591 | source |
| `internal/core/dream_learning.go` | 585 | source |
| `internal/core/tool_registry.go` | 582 | source |
| `internal/core/shards/spawn_queue.go` | 581 | source |
| `internal/core/virtual_store_python.go` | 573 | source |

## 3. Complete source inventory (capped)

| Path | Lines |
|------|------:|
| `internal/core/action_validator.go` | 437 |
| `internal/core/api_scheduler.go` | 871 |
| `internal/core/cortex_kernel.go` | 731 |
| `internal/core/defaults/intent_corpus.go` | 40 |
| `internal/core/defaults/predicate_corpus.go` | 42 |
| `internal/core/defaults/prompt_corpus.go` | 39 |
| `internal/core/dream_learning.go` | 585 |
| `internal/core/dream_plan.go` | 179 |
| `internal/core/dream_plan_extractor.go` | 414 |
| `internal/core/dream_plan_manager.go` | 366 |
| `internal/core/dream_router.go` | 356 |
| `internal/core/dreamer.go` | 753 |
| `internal/core/external_predicates.go` | 154 |
| `internal/core/fact_categories.go` | 183 |
| `internal/core/fact_event_bus.go` | 149 |
| `internal/core/hybrid_loader.go` | 233 |
| `internal/core/intent_defaults.go` | 65 |
| `internal/core/intent_inference.go` | 122 |
| `internal/core/intent_loader.go` | 33 |
| `internal/core/kernel.go` | 16 |
| `internal/core/kernel_accessors.go` | 21 |
| `internal/core/kernel_eval.go` | 730 |
| `internal/core/kernel_facts.go` | 1255 |
| `internal/core/kernel_facts_intern.go` | 64 |
| `internal/core/kernel_init.go` | 591 |
| `internal/core/kernel_policy.go` | 487 |
| `internal/core/kernel_provenance.go` | 116 |
| `internal/core/kernel_query.go` | 595 |
| `internal/core/kernel_shard.go` | 437 |
| `internal/core/kernel_transactions.go` | 239 |
| `internal/core/kernel_types.go` | 140 |
| `internal/core/kernel_utils.go` | 188 |
| `internal/core/kernel_validation.go` | 470 |
| `internal/core/kernel_virtual.go` | 21 |
| `internal/core/learning.go` | 14 |
| `internal/core/limits.go` | 342 |
| `internal/core/llm_client.go` | 73 |
| `internal/core/mangle_updates.go` | 154 |
| `internal/core/mangle_watcher.go` | 422 |
| `internal/core/parse_serial.go` | 24 |
| `internal/core/predicate_corpus.go` | 636 |
| `internal/core/rule_court.go` | 110 |
| `internal/core/scheduled_llm_client.go` | 854 |
| `internal/core/self_healing.go` | 360 |
| `internal/core/shadow_mode.go` | 559 |
| `internal/core/shard_fact_router.go` | 209 |
| `internal/core/shards/agents.go` | 427 |
| `internal/core/shards/config.go` | 74 |
| `internal/core/shards/manager.go` | 419 |
| `internal/core/shards/manager_spawn.go` | 622 |
| `internal/core/shards/manager_tools.go` | 273 |
| `internal/core/shards/spawn_queue.go` | 581 |
| `internal/core/tdd_loop.go` | 833 |
| `internal/core/tool_registry.go` | 582 |
| `internal/core/trace.go` | 181 |
| `internal/core/transaction_manager.go` | 527 |
| `internal/core/validator_codedom.go` | 314 |
| `internal/core/validator_dir.go` | 88 |
| `internal/core/validator_edit_enhanced.go` | 318 |
| `internal/core/validator_exec.go` | 388 |
| `internal/core/validator_file.go` | 315 |
| `internal/core/validator_paranoid.go` | 343 |
| `internal/core/validator_registry.go` | 59 |
| `internal/core/validator_syntax.go` | 391 |
| `internal/core/virtual_store.go` | 1077 |
| `internal/core/virtual_store_actions.go` | 1008 |
| `internal/core/virtual_store_codedom.go` | 706 |
| `internal/core/virtual_store_constitution.go` | 144 |
| `internal/core/virtual_store_file_actions.go` | 419 |
| `internal/core/virtual_store_graph.go` | 99 |
| `internal/core/virtual_store_interactive_gate.go` | 196 |
| `internal/core/virtual_store_mcp_proxy.go` | 132 |
| `internal/core/virtual_store_predicates.go` | 993 |
| `internal/core/virtual_store_python.go` | 573 |
| `internal/core/virtual_store_routing.go` | 506 |
| `internal/core/virtual_store_tools.go` | 244 |
| `internal/core/virtual_store_types.go` | 310 |
| `internal/core/virtual_store_workflows.go` | 1029 |

## 4. Tests (sample)

| Path | Lines |
|------|------:|
| `internal/core/virtual_store_workflows_coverage_test.go` | 1378 |
| `internal/core/virtual_store_actions_coverage_test.go` | 1355 |
| `internal/core/virtual_store_codedom_coverage_test.go` | 1265 |
| `internal/core/coverage_boost_test.go` | 1092 |
| `internal/core/kernel_facts_gaps_test.go` | 944 |
| `internal/core/transaction_manager_test.go` | 858 |
| `internal/core/api_scheduler_test.go` | 767 |
| `internal/core/transaction_manager_gaps_test.go` | 715 |
| `internal/core/validator_paranoid_test.go` | 682 |
| `internal/core/virtual_store_python_test.go` | 677 |

## 5. Behavior summary

Package **core** is a living codeNERD subsystem: Mangle kernel, VirtualStore, Dreamer, facts, API scheduler, shard manager plumbing.

APIs and control flow are defined by the cited source files — this corpus does not invent signatures.

## 6. Known limitations

- Inventory/type extraction is automated; deepen narrative when this package is the active design target.
- Completeness heuristic (88%) is not go cover %.
- Cross-package wiring claims require grep of registration hubs before treating as live.
