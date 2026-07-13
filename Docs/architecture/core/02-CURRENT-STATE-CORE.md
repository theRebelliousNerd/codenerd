# core — Current State

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded corpus**
> Mode: dark-factory autonomous generation via arch-propose/corpus-build port
> **Implementation: present under `internal/core/` (78 non-test .go, 107 tests, 129 .mg)**


## 1. Source location

- Primary package: `internal/core/` (**exists** with 78 non-test Go files)
- Supporting global surfaces: `internal/core/defaults/` when schemas/policy apply

## 2. File inventory (largest sources)

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

## 3. Test inventory (sample)

| Path | Lines |
|------|------:|
| `internal/core/virtual_store_workflows_coverage_test.go` | 1378 |
| `internal/core/virtual_store_actions_coverage_test.go` | 1355 |
| `internal/core/virtual_store_codedom_coverage_test.go` | 1265 |
| `internal/core/coverage_boost_test.go` | 1092 |
| `internal/core/kernel_facts_gaps_test.go` | 944 |
| `internal/core/transaction_manager_test.go` | 858 |

## 4. Current behavior (summary)

Package **core** is a living codeNERD subsystem: Mangle kernel, VirtualStore, Dreamer, fact store, shard manager plumbing.

Behavior is defined by the source files above. This corpus does **not** invent APIs —
consult the cited paths for signatures and control flow.

## 5. Known limitations (honest)

- Corpus generated in dark-factory mode from inventory + lightweight type extraction; deep behavioral narrative may lag micro-refactors.
- Completeness heuristic (88%) is not coverage % — run `go test` for truth.
- Cross-package wiring must be validated against `internal/shards/registration.go` and VirtualStore routes when relevant.
