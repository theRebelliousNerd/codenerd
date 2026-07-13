# sqlpragmas — Current State

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/sqlpragmas/` (complete internal coverage)
> **Implementation: `internal/sqlpragmas/` — 1 non-test .go, 2 tests, 0 .mg**


## 1. Source location

- Primary package: `internal/sqlpragmas/` (exists; 1 non-test Go files)
- 1:1 mapping: `Docs/architecture/sqlpragmas/` ↔ `internal/sqlpragmas/`

## 2. Largest source files

| Path | Lines | Kind |
|------|------:|------|
| `internal/sqlpragmas/pragmas.go` | 124 | source |

## 3. Complete source inventory (capped)

| Path | Lines |
|------|------:|
| `internal/sqlpragmas/pragmas.go` | 124 |

## 4. Tests (sample)

| Path | Lines |
|------|------:|
| `internal/sqlpragmas/pragma_integration_test.go` | 170 |
| `internal/sqlpragmas/pragmas_test.go` | 116 |

## 5. Behavior summary

Package **sqlpragmas** is a living codeNERD subsystem: SQLite pragma helpers for safe DB open.

APIs and control flow are defined by the cited source files — this corpus does not invent signatures.

## 6. Known limitations

- Inventory/type extraction is automated; deepen narrative when this package is the active design target.
- Completeness heuristic (90%) is not go cover %.
- Cross-package wiring claims require grep of registration hubs before treating as live.
