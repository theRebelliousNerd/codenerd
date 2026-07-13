# browser — Current State

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/browser/` (complete internal coverage)
> **Implementation: `internal/browser/` — 3 non-test .go, 6 tests, 0 .mg**


## 1. Source location

- Primary package: `internal/browser/` (exists; 3 non-test Go files)
- 1:1 mapping: `Docs/architecture/browser/` ↔ `internal/browser/`

## 2. Largest source files

| Path | Lines | Kind |
|------|------:|------|
| `internal/browser/session_manager.go` | 809 | source |
| `internal/browser/session_manager_dom.go` | 677 | source |
| `internal/browser/honeypot.go` | 412 | source |

## 3. Complete source inventory (capped)

| Path | Lines |
|------|------:|
| `internal/browser/honeypot.go` | 412 |
| `internal/browser/session_manager.go` | 809 |
| `internal/browser/session_manager_dom.go` | 677 |

## 4. Tests (sample)

| Path | Lines |
|------|------:|
| `internal/browser/session_manager_coverage_test.go` | 1102 |
| `internal/browser/honeypot_coverage_test.go` | 479 |
| `internal/browser/start_coverage_test.go` | 378 |
| `internal/browser/lifecycle_coverage_test.go` | 280 |
| `internal/browser/browser_integration_test.go` | 186 |
| `internal/browser/honeypot_test.go` | 145 |

## 5. Behavior summary

Package **browser** is a living codeNERD subsystem: Browser automation / Rod session management and honeypot surfaces.

APIs and control flow are defined by the cited source files — this corpus does not invent signatures.

## 6. Known limitations

- Inventory/type extraction is automated; deepen narrative when this package is the active design target.
- Completeness heuristic (90%) is not go cover %.
- Cross-package wiring claims require grep of registration hubs before treating as live.
