# build — Current State

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/build/` (complete internal coverage)
> **Implementation: `internal/build/` — 1 non-test .go, 2 tests, 0 .mg**


## 1. Source location

- Primary package: `internal/build/` (exists; 1 non-test Go files)
- 1:1 mapping: `Docs/architecture/build/` ↔ `internal/build/`

## 2. Largest source files

| Path | Lines | Kind |
|------|------:|------|
| `internal/build/env.go` | 312 | source |

## 3. Complete source inventory (capped)

| Path | Lines |
|------|------:|
| `internal/build/env.go` | 312 |

## 4. Tests (sample)

| Path | Lines |
|------|------:|
| `internal/build/env_gaps_test.go` | 427 |
| `internal/build/env_test.go` | 211 |

## 5. Behavior summary

Package **build** is a living codeNERD subsystem: Build-time environment helpers (CGO/sqlite flags, build env).

APIs and control flow are defined by the cited source files — this corpus does not invent signatures.

## 6. Known limitations

- Inventory/type extraction is automated; deepen narrative when this package is the active design target.
- Completeness heuristic (90%) is not go cover %.
- Cross-package wiring claims require grep of registration hubs before treating as live.
