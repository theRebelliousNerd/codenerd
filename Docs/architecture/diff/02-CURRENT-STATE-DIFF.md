# diff — Current State

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/diff/` (complete internal coverage)
> **Implementation: `internal/diff/` — 1 non-test .go, 2 tests, 0 .mg**


## 1. Source location

- Primary package: `internal/diff/` (exists; 1 non-test Go files)
- 1:1 mapping: `Docs/architecture/diff/` ↔ `internal/diff/`

## 2. Largest source files

| Path | Lines | Kind |
|------|------:|------|
| `internal/diff/diff.go` | 378 | source |

## 3. Complete source inventory (capped)

| Path | Lines |
|------|------:|
| `internal/diff/diff.go` | 378 |

## 4. Tests (sample)

| Path | Lines |
|------|------:|
| `internal/diff/diff_test.go` | 483 |
| `internal/diff/diff_comprehensive_test.go` | 465 |

## 5. Behavior summary

Package **diff** is a living codeNERD subsystem: Diff utilities for code change analysis.

APIs and control flow are defined by the cited source files — this corpus does not invent signatures.

## 6. Known limitations

- Inventory/type extraction is automated; deepen narrative when this package is the active design target.
- Completeness heuristic (90%) is not go cover %.
- Cross-package wiring claims require grep of registration hubs before treating as live.
