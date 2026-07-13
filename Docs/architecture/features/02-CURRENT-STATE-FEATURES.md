# features — Current State

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/features/` (complete internal coverage)
> **Implementation: `internal/features/` — 1 non-test .go, 3 tests, 0 .mg**


## 1. Source location

- Primary package: `internal/features/` (exists; 1 non-test Go files)
- 1:1 mapping: `Docs/architecture/features/` ↔ `internal/features/`

## 2. Largest source files

| Path | Lines | Kind |
|------|------:|------|
| `internal/features/features.go` | 350 | source |

## 3. Complete source inventory (capped)

| Path | Lines |
|------|------:|
| `internal/features/features.go` | 350 |

## 4. Tests (sample)

| Path | Lines |
|------|------:|
| `internal/features/config_roundtrip_test.go` | 192 |
| `internal/features/features_test.go` | 153 |
| `internal/features/features_defaults_test.go` | 42 |

## 5. Behavior summary

Package **features** is a living codeNERD subsystem: Feature flags and feature configuration defaults.

APIs and control flow are defined by the cited source files — this corpus does not invent signatures.

## 6. Known limitations

- Inventory/type extraction is automated; deepen narrative when this package is the active design target.
- Completeness heuristic (90%) is not go cover %.
- Cross-package wiring claims require grep of registration hubs before treating as live.
