# regression — Current State

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/regression/` (complete internal coverage)
> **Implementation: `internal/regression/` — 1 non-test .go, 1 tests, 0 .mg**


## 1. Source location

- Primary package: `internal/regression/` (exists; 1 non-test Go files)
- 1:1 mapping: `Docs/architecture/regression/` ↔ `internal/regression/`

## 2. Largest source files

| Path | Lines | Kind |
|------|------:|------|
| `internal/regression/battery.go` | 138 | source |

## 3. Complete source inventory (capped)

| Path | Lines |
|------|------:|
| `internal/regression/battery.go` | 138 |

## 4. Tests (sample)

| Path | Lines |
|------|------:|
| `internal/regression/battery_test.go` | 102 |

## 5. Behavior summary

Package **regression** is a living codeNERD subsystem: Regression harness utilities.

APIs and control flow are defined by the cited source files — this corpus does not invent signatures.

## 6. Known limitations

- Inventory/type extraction is automated; deepen narrative when this package is the active design target.
- Completeness heuristic (90%) is not go cover %.
- Cross-package wiring claims require grep of registration hubs before treating as live.
