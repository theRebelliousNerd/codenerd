# ux — Current State

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/ux/` (complete internal coverage)
> **Implementation: `internal/ux/` — 4 non-test .go, 4 tests, 0 .mg**


## 1. Source location

- Primary package: `internal/ux/` (exists; 4 non-test Go files)
- 1:1 mapping: `Docs/architecture/ux/` ↔ `internal/ux/`

## 2. Largest source files

| Path | Lines | Kind |
|------|------:|------|
| `internal/ux/preferences.go` | 363 | source |
| `internal/ux/migration.go` | 281 | source |
| `internal/ux/user_state.go` | 123 | source |
| `internal/ux/doc.go` | 20 | source |

## 3. Complete source inventory (capped)

| Path | Lines |
|------|------:|
| `internal/ux/doc.go` | 20 |
| `internal/ux/migration.go` | 281 |
| `internal/ux/preferences.go` | 363 |
| `internal/ux/user_state.go` | 123 |

## 4. Tests (sample)

| Path | Lines |
|------|------:|
| `internal/ux/migration_test.go` | 156 |
| `internal/ux/preferences_test.go` | 102 |
| `internal/ux/user_state_test.go` | 50 |
| `internal/ux/migration_extra_test.go` | 46 |

## 5. Behavior summary

Package **ux** is a living codeNERD subsystem: UX helpers for CLI presentation.

APIs and control flow are defined by the cited source files — this corpus does not invent signatures.

## 6. Known limitations

- Inventory/type extraction is automated; deepen narrative when this package is the active design target.
- Completeness heuristic (90%) is not go cover %.
- Cross-package wiring claims require grep of registration hubs before treating as live.
