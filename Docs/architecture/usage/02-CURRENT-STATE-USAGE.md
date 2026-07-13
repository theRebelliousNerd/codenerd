# usage — Current State

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/usage/` (complete internal coverage)
> **Implementation: `internal/usage/` — 2 non-test .go, 4 tests, 0 .mg**


## 1. Source location

- Primary package: `internal/usage/` (exists; 2 non-test Go files)
- 1:1 mapping: `Docs/architecture/usage/` ↔ `internal/usage/`

## 2. Largest source files

| Path | Lines | Kind |
|------|------:|------|
| `internal/usage/usage_tracker.go` | 210 | source |
| `internal/usage/usage_types.go` | 47 | source |

## 3. Complete source inventory (capped)

| Path | Lines |
|------|------:|
| `internal/usage/usage_tracker.go` | 210 |
| `internal/usage/usage_types.go` | 47 |

## 4. Tests (sample)

| Path | Lines |
|------|------:|
| `internal/usage/usage_comprehensive_test.go` | 354 |
| `internal/usage/usage_tracker_test.go` | 299 |
| `internal/usage/usage_types_test.go` | 78 |
| `internal/usage/usage_tracker_context_test.go` | 34 |

## 5. Behavior summary

Package **usage** is a living codeNERD subsystem: Usage / token accounting helpers.

APIs and control flow are defined by the cited source files — this corpus does not invent signatures.

## 6. Known limitations

- Inventory/type extraction is automated; deepen narrative when this package is the active design target.
- Completeness heuristic (90%) is not go cover %.
- Cross-package wiring claims require grep of registration hubs before treating as live.
