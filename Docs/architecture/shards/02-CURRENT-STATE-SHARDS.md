# shards — Current State

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/shards/` (complete internal coverage)
> **Implementation: `internal/shards/` — 18 non-test .go, 24 tests, 1 .mg**


## 1. Source location

- Primary package: `internal/shards/` (exists; 18 non-test Go files)
- 1:1 mapping: `Docs/architecture/shards/` ↔ `internal/shards/`

## 2. Largest source files

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
| `internal/shards/system/campaign_runner.go` | 471 | source |
| `internal/shards/system/legislator.go` | 457 | source |
| `internal/shards/consultation.go` | 406 | source |
| `internal/shards/system/executive_autopoiesis.go` | 269 | source |
| `internal/shards/requirements_interrogator.go` | 190 | source |
| `internal/shards/system/payloads.go` | 69 | source |

## 3. Complete source inventory (capped)

| Path | Lines |
|------|------:|
| `internal/shards/consultation.go` | 406 |
| `internal/shards/matching.go` | 649 |
| `internal/shards/observer_manager.go` | 542 |
| `internal/shards/registration.go` | 534 |
| `internal/shards/requirements_interrogator.go` | 190 |
| `internal/shards/system/base.go` | 779 |
| `internal/shards/system/campaign_runner.go` | 471 |
| `internal/shards/system/constitution.go` | 1038 |
| `internal/shards/system/executive.go` | 660 |
| `internal/shards/system/executive_autopoiesis.go` | 269 |
| `internal/shards/system/executive_intent.go` | 563 |
| `internal/shards/system/legislator.go` | 457 |
| `internal/shards/system/mangle_repair.go` | 1061 |
| `internal/shards/system/payloads.go` | 69 |
| `internal/shards/system/perception.go` | 953 |
| `internal/shards/system/planner.go` | 1108 |
| `internal/shards/system/router.go` | 1030 |
| `internal/shards/system/world_model.go` | 748 |

## 4. Tests (sample)

| Path | Lines |
|------|------:|
| `internal/shards/system/base_coverage_test.go` | 1053 |
| `internal/shards/system/constitution_coverage_test.go` | 941 |
| `internal/shards/system/executive_coverage_test.go` | 819 |
| `internal/shards/consultation_test.go` | 268 |
| `internal/shards/system/mangle_repair_test.go` | 263 |
| `internal/shards/observer_integration_test.go` | 248 |
| `internal/shards/system/learning_test.go` | 248 |
| `internal/shards/system/planner_test.go` | 244 |
| `internal/shards/observer_manager_test.go` | 194 |
| `internal/shards/matching_test.go` | 168 |

## 5. Behavior summary

Package **shards** is a living codeNERD subsystem: Domain and system shard implementations + registration.

APIs and control flow are defined by the cited source files — this corpus does not invent signatures.

## 6. Known limitations

- Inventory/type extraction is automated; deepen narrative when this package is the active design target.
- Completeness heuristic (90%) is not go cover %.
- Cross-package wiring claims require grep of registration hubs before treating as live.
