# verification — Current State

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/verification/` (complete internal coverage)
> **Implementation: `internal/verification/` — 1 non-test .go, 3 tests, 0 .mg**


## 1. Source location

- Primary package: `internal/verification/` (exists; 1 non-test Go files)
- 1:1 mapping: `Docs/architecture/verification/` ↔ `internal/verification/`

## 2. Largest source files

| Path | Lines | Kind |
|------|------:|------|
| `internal/verification/verifier.go` | 819 | source |

## 3. Complete source inventory (capped)

| Path | Lines |
|------|------:|
| `internal/verification/verifier.go` | 819 |

## 4. Tests (sample)

| Path | Lines |
|------|------:|
| `internal/verification/verifier_gaps_test.go` | 801 |
| `internal/verification/verifier_test.go` | 103 |
| `internal/verification/verifier_normalize_test.go` | 42 |

## 5. Behavior summary

Package **verification** is a living codeNERD subsystem: Verification utilities for agent outputs.

APIs and control flow are defined by the cited source files — this corpus does not invent signatures.

## 6. Known limitations

- Inventory/type extraction is automated; deepen narrative when this package is the active design target.
- Completeness heuristic (90%) is not go cover %.
- Cross-package wiring claims require grep of registration hubs before treating as live.
