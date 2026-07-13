# northstar — Current State

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/northstar/` (complete internal coverage)
> **Implementation: `internal/northstar/` — 4 non-test .go, 6 tests, 0 .mg**


## 1. Source location

- Primary package: `internal/northstar/` (exists; 4 non-test Go files)
- 1:1 mapping: `Docs/architecture/northstar/` ↔ `internal/northstar/`

## 2. Largest source files

| Path | Lines | Kind |
|------|------:|------|
| `internal/northstar/store.go` | 732 | source |
| `internal/northstar/guardian.go` | 677 | source |
| `internal/northstar/observer.go` | 482 | source |
| `internal/northstar/types.go` | 305 | source |

## 3. Complete source inventory (capped)

| Path | Lines |
|------|------:|
| `internal/northstar/guardian.go` | 677 |
| `internal/northstar/observer.go` | 482 |
| `internal/northstar/store.go` | 732 |
| `internal/northstar/types.go` | 305 |

## 4. Tests (sample)

| Path | Lines |
|------|------:|
| `internal/northstar/guardian_test.go` | 1103 |
| `internal/northstar/store_test.go` | 710 |
| `internal/northstar/types_test.go` | 623 |
| `internal/northstar/observer_test.go` | 514 |
| `internal/northstar/types_facts_test.go` | 114 |
| `internal/northstar/guardian_warn_test.go` | 71 |

## 5. Behavior summary

Package **northstar** is a living codeNERD subsystem: North-star goal tracking and alignment helpers.

APIs and control flow are defined by the cited source files — this corpus does not invent signatures.

## 6. Known limitations

- Inventory/type extraction is automated; deepen narrative when this package is the active design target.
- Completeness heuristic (90%) is not go cover %.
- Cross-package wiring claims require grep of registration hubs before treating as live.
