# persist — Current State

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/persist/` (complete internal coverage)
> **Implementation: `internal/persist/` — 1 non-test .go, 4 tests, 0 .mg**


## 1. Source location

- Primary package: `internal/persist/` (exists; 1 non-test Go files)
- 1:1 mapping: `Docs/architecture/persist/` ↔ `internal/persist/`

## 2. Largest source files

| Path | Lines | Kind |
|------|------:|------|
| `internal/persist/factsnap/factsnap.go` | 287 | source |

## 3. Complete source inventory (capped)

| Path | Lines |
|------|------:|
| `internal/persist/factsnap/factsnap.go` | 287 |

## 4. Tests (sample)

| Path | Lines |
|------|------:|
| `internal/persist/factsnap/factsnap_test.go` | 243 |
| `internal/persist/factsnap/codec_parity_test.go` | 120 |
| `internal/persist/factsnap/factsnap_codec_test.go` | 54 |
| `internal/persist/factsnap/legacy_test.go` | 41 |

## 5. Behavior summary

Package **persist** is a living codeNERD subsystem: Persistence helpers bridging stores and runtime.

APIs and control flow are defined by the cited source files — this corpus does not invent signatures.

## 6. Known limitations

- Inventory/type extraction is automated; deepen narrative when this package is the active design target.
- Completeness heuristic (90%) is not go cover %.
- Cross-package wiring claims require grep of registration hubs before treating as live.
