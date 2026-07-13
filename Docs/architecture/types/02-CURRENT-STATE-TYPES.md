# types — Current State

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/types/` (complete internal coverage)
> **Implementation: `internal/types/` — 5 non-test .go, 4 tests, 0 .mg**


## 1. Source location

- Primary package: `internal/types/` (exists; 5 non-test Go files)
- 1:1 mapping: `Docs/architecture/types/` ↔ `internal/types/`

## 2. Largest source files

| Path | Lines | Kind |
|------|------:|------|
| `internal/types/types.go` | 455 | source |
| `internal/types/interfaces.go` | 379 | source |
| `internal/types/extract.go` | 210 | source |
| `internal/types/shard.go` | 157 | source |
| `internal/types/transaction.go` | 85 | source |

## 3. Complete source inventory (capped)

| Path | Lines |
|------|------:|
| `internal/types/extract.go` | 210 |
| `internal/types/interfaces.go` | 379 |
| `internal/types/shard.go` | 157 |
| `internal/types/transaction.go` | 85 |
| `internal/types/types.go` | 455 |

## 4. Tests (sample)

| Path | Lines |
|------|------:|
| `internal/types/types_comprehensive_test.go` | 528 |
| `internal/types/extract_test.go` | 221 |
| `internal/types/types_test.go` | 174 |
| `internal/types/shard_test.go` | 19 |

## 5. Behavior summary

Package **types** is a living codeNERD subsystem: Shared type definitions used across packages.

APIs and control flow are defined by the cited source files — this corpus does not invent signatures.

## 6. Known limitations

- Inventory/type extraction is automated; deepen narrative when this package is the active design target.
- Completeness heuristic (85%) is not go cover %.
- Cross-package wiring claims require grep of registration hubs before treating as live.
