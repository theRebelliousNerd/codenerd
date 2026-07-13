# logging — Current State

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/logging/` (complete internal coverage)
> **Implementation: `internal/logging/` — 4 non-test .go, 5 tests, 0 .mg**


## 1. Source location

- Primary package: `internal/logging/` (exists; 4 non-test Go files)
- 1:1 mapping: `Docs/architecture/logging/` ↔ `internal/logging/`

## 2. Largest source files

| Path | Lines | Kind |
|------|------:|------|
| `internal/logging/logger.go` | 680 | source |
| `internal/logging/audit.go` | 617 | source |
| `internal/logging/logger_convenience.go` | 530 | source |
| `internal/logging/llm_io_logger.go` | 206 | source |

## 3. Complete source inventory (capped)

| Path | Lines |
|------|------:|
| `internal/logging/audit.go` | 617 |
| `internal/logging/llm_io_logger.go` | 206 |
| `internal/logging/logger.go` | 680 |
| `internal/logging/logger_convenience.go` | 530 |

## 4. Tests (sample)

| Path | Lines |
|------|------:|
| `internal/logging/coverage_boost_test.go` | 1208 |
| `internal/logging/logging_comprehensive_test.go` | 743 |
| `internal/logging/logger_test.go` | 451 |
| `internal/logging/audit_coverage_test.go` | 326 |
| `internal/logging/audit_benchmark_test.go` | 30 |

## 5. Behavior summary

Package **logging** is a living codeNERD subsystem: Categorized logging system for debug/diagnostics.

APIs and control flow are defined by the cited source files — this corpus does not invent signatures.

## 6. Known limitations

- Inventory/type extraction is automated; deepen narrative when this package is the active design target.
- Completeness heuristic (90%) is not go cover %.
- Cross-package wiring claims require grep of registration hubs before treating as live.
