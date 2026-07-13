# store — Current State

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/store/` (complete internal coverage)
> **Implementation: `internal/store/` — 39 non-test .go, 44 tests, 0 .mg**


## 1. Source location

- Primary package: `internal/store/` (exists; 39 non-test Go files)
- 1:1 mapping: `Docs/architecture/store/` ↔ `internal/store/`

## 2. Largest source files

| Path | Lines | Kind |
|------|------:|------|
| `internal/store/vector_store.go` | 1009 | source |
| `internal/store/migrations.go` | 811 | source |
| `internal/store/trace_store.go` | 710 | source |
| `internal/store/local_core.go` | 689 | source |
| `internal/store/reflection_worker.go` | 651 | source |
| `internal/store/learned_store.go` | 571 | source |
| `internal/store/local_cold.go` | 544 | source |
| `internal/store/tool_cleanup.go` | 464 | source |
| `internal/store/embedded_store.go` | 444 | source |
| `internal/store/local_knowledge.go` | 426 | source |
| `internal/store/reflection_search.go` | 405 | source |
| `internal/store/learning.go` | 386 | source |
| `internal/store/tool_store.go` | 373 | source |
| `internal/store/vector_store_reembed.go` | 344 | source |
| `internal/store/local_prompt.go` | 334 | source |
| `internal/store/trace_reflection.go` | 324 | source |
| `internal/store/learning_reflection.go` | 317 | source |
| `internal/store/local_world.go` | 313 | source |
| `internal/store/local_verification.go` | 264 | source |
| `internal/store/vector_store_bruteforce.go` | 257 | source |

## 3. Complete source inventory (capped)

| Path | Lines |
|------|------:|
| `internal/store/embedded_store.go` | 444 |
| `internal/store/fact_codec.go` | 111 |
| `internal/store/indexes.go` | 31 |
| `internal/store/init_sqlite.go` | 3 |
| `internal/store/init_vec.go` | 13 |
| `internal/store/learned_store.go` | 571 |
| `internal/store/learning.go` | 386 |
| `internal/store/learning_candidates.go` | 179 |
| `internal/store/learning_reflection.go` | 317 |
| `internal/store/local.go` | 18 |
| `internal/store/local_cold.go` | 544 |
| `internal/store/local_core.go` | 689 |
| `internal/store/local_graph.go` | 238 |
| `internal/store/local_graph_query.go` | 102 |
| `internal/store/local_knowledge.go` | 426 |
| `internal/store/local_prompt.go` | 334 |
| `internal/store/local_review.go` | 43 |
| `internal/store/local_session.go` | 235 |
| `internal/store/local_vector.go` | 107 |
| `internal/store/local_verification.go` | 264 |
| `internal/store/local_world.go` | 313 |
| `internal/store/migrations.go` | 811 |
| `internal/store/pragmas.go` | 32 |
| `internal/store/prompt_reembed.go` | 166 |
| `internal/store/reembed_all.go` | 185 |
| `internal/store/reflection_reembed.go` | 116 |
| `internal/store/reflection_search.go` | 405 |
| `internal/store/reflection_utils.go` | 92 |
| `internal/store/reflection_worker.go` | 651 |
| `internal/store/tool_cleanup.go` | 464 |
| `internal/store/tool_store.go` | 373 |
| `internal/store/trace_reflection.go` | 324 |
| `internal/store/trace_store.go` | 710 |
| `internal/store/vec_support_disabled.go` | 6 |
| `internal/store/vec_support_enabled.go` | 6 |
| `internal/store/vector_store.go` | 1009 |
| `internal/store/vector_store_bruteforce.go` | 257 |
| `internal/store/vector_store_reembed.go` | 344 |
| `internal/store/vector_utils.go` | 77 |

## 4. Tests (sample)

| Path | Lines |
|------|------:|
| `internal/store/trace_store_test.go` | 571 |
| `internal/store/vector_store_search_test.go` | 524 |
| `internal/store/vector_store_batch_test.go` | 393 |
| `internal/store/vector_store_test.go` | 370 |
| `internal/store/archival_test.go` | 328 |
| `internal/store/trace_store_integration_test.go` | 300 |
| `internal/store/cold_storage_integration_test.go` | 247 |
| `internal/store/local_graph_test.go` | 224 |
| `internal/store/tool_cleanup_extra_test.go` | 223 |
| `internal/store/local_session_integration_test.go` | 173 |

## 5. Behavior summary

Package **store** is a living codeNERD subsystem: Memory tiers and durable store implementations.

APIs and control flow are defined by the cited source files — this corpus does not invent signatures.

## 6. Known limitations

- Inventory/type extraction is automated; deepen narrative when this package is the active design target.
- Completeness heuristic (90%) is not go cover %.
- Cross-package wiring claims require grep of registration hubs before treating as live.
