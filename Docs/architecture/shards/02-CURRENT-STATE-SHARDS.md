# shards — Current State

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded corpus**
> Mode: dark-factory autonomous generation via arch-propose/corpus-build port
> **Implementation: present under `internal/shards/` (18 non-test .go, 24 tests, 1 .mg)**


## 1. Source location

- Primary package: `internal/shards/` (**exists** with 18 non-test Go files)
- Supporting global surfaces: `internal/core/defaults/` when schemas/policy apply

## 2. File inventory (largest sources)

| Path | Lines | Kind |
|------|------:|------|
| `internal/shards/system/planner.go` | 1108 | source |
| `internal/shards/system/mangle_repair.go` | 1061 | source |
| `internal/shards/system/constitution.go` | 1038 | source |
| `internal/shards/system/router.go` | 1030 | source |
| `internal/shards/system/perception.go` | 953 | source |
| `internal/shards/system/base.go` | 779 | source |
| `internal/shards/system/world_model.go` | 748 | source |
| `internal/shards/system/executive.go` | 660 | source |
| `internal/shards/matching.go` | 649 | source |
| `internal/shards/system/executive_intent.go` | 563 | source |
| `internal/shards/observer_manager.go` | 542 | source |
| `internal/shards/registration.go` | 534 | source |

## 3. Test inventory (sample)

| Path | Lines |
|------|------:|
| `internal/shards/system/base_coverage_test.go` | 1053 |
| `internal/shards/system/constitution_coverage_test.go` | 941 |
| `internal/shards/system/executive_coverage_test.go` | 819 |
| `internal/shards/consultation_test.go` | 268 |
| `internal/shards/system/mangle_repair_test.go` | 263 |
| `internal/shards/observer_integration_test.go` | 248 |

## 4. Current behavior (summary)

Package **shards** is a living codeNERD subsystem: Domain/system shard implementations and registration.

Behavior is defined by the source files above. This corpus does **not** invent APIs —
consult the cited paths for signatures and control flow.

## 5. Known limitations (honest)

- Corpus generated in dark-factory mode from inventory + lightweight type extraction; deep behavioral narrative may lag micro-refactors.
- Completeness heuristic (90%) is not coverage % — run `go test` for truth.
- Cross-package wiring must be validated against `internal/shards/registration.go` and VirtualStore routes when relevant.
