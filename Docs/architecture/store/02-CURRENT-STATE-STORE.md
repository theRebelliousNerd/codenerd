# store — Current State

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded corpus**
> Mode: dark-factory autonomous generation via arch-propose/corpus-build port
> **Implementation: present under `internal/store/` (39 non-test .go, 44 tests, 0 .mg)**


## 1. Source location

- Primary package: `internal/store/` (**exists** with 39 non-test Go files)
- Supporting global surfaces: `internal/core/defaults/` when schemas/policy apply

## 2. File inventory (largest sources)

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

## 3. Test inventory (sample)

| Path | Lines |
|------|------:|
| `internal/store/trace_store_test.go` | 571 |
| `internal/store/vector_store_search_test.go` | 524 |
| `internal/store/vector_store_batch_test.go` | 393 |
| `internal/store/vector_store_test.go` | 370 |
| `internal/store/archival_test.go` | 328 |
| `internal/store/trace_store_integration_test.go` | 300 |

## 4. Current behavior (summary)

Package **store** is a living codeNERD subsystem: Memory tiers / persistence stores.

Behavior is defined by the source files above. This corpus does **not** invent APIs —
consult the cited paths for signatures and control flow.

## 5. Known limitations (honest)

- Corpus generated in dark-factory mode from inventory + lightweight type extraction; deep behavioral narrative may lag micro-refactors.
- Completeness heuristic (90%) is not coverage % — run `go test` for truth.
- Cross-package wiring must be validated against `internal/shards/registration.go` and VirtualStore routes when relevant.
