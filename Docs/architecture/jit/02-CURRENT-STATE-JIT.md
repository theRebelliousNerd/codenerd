# jit — Current State

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/jit/` (complete internal coverage)
> **Implementation: `internal/jit/` — 1 non-test .go, 1 tests, 0 .mg**


## 1. Source location

- Primary package: `internal/jit/` (exists; 1 non-test Go files)
- 1:1 mapping: `Docs/architecture/jit/` ↔ `internal/jit/`

## 2. Largest source files

| Path | Lines | Kind |
|------|------:|------|
| `internal/jit/config/types.go` | 59 | source |

## 3. Complete source inventory (capped)

| Path | Lines |
|------|------:|
| `internal/jit/config/types.go` | 59 |

## 4. Tests (sample)

| Path | Lines |
|------|------:|
| `internal/jit/config/types_test.go` | 67 |

## 5. Behavior summary

Package **jit** is a living codeNERD subsystem: JIT-related config/types supporting prompt compilation.

APIs and control flow are defined by the cited source files — this corpus does not invent signatures.

## 6. Known limitations

- Inventory/type extraction is automated; deepen narrative when this package is the active design target.
- Completeness heuristic (90%) is not go cover %.
- Cross-package wiring claims require grep of registration hubs before treating as live.
