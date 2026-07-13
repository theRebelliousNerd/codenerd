# context — Current State

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/context/` (complete internal coverage)
> **Implementation: `internal/context/` — 9 non-test .go, 11 tests, 1 .mg**


## 1. Source location

- Primary package: `internal/context/` (exists; 9 non-test Go files)
- 1:1 mapping: `Docs/architecture/context/` ↔ `internal/context/`

## 2. Largest source files

| Path | Lines | Kind |
|------|------:|------|
| `internal/context/compressor.go` | 748 | source |
| `internal/context/activation.go` | 700 | source |
| `internal/context/activation_scoring.go` | 634 | source |
| `internal/context/serializer.go` | 552 | source |
| `internal/context/compressor_metrics.go` | 492 | source |
| `internal/context/compressor_turns.go` | 438 | source |
| `internal/context/feedback_store.go` | 423 | source |
| `internal/context/types.go` | 359 | source |
| `internal/context/tokens.go` | 346 | source |

## 3. Complete source inventory (capped)

| Path | Lines |
|------|------:|
| `internal/context/activation.go` | 700 |
| `internal/context/activation_scoring.go` | 634 |
| `internal/context/compressor.go` | 748 |
| `internal/context/compressor_metrics.go` | 492 |
| `internal/context/compressor_turns.go` | 438 |
| `internal/context/feedback_store.go` | 423 |
| `internal/context/serializer.go` | 552 |
| `internal/context/tokens.go` | 346 |
| `internal/context/types.go` | 359 |

## 4. Tests (sample)

| Path | Lines |
|------|------:|
| `internal/context/activation_test.go` | 678 |
| `internal/context/compressor_test.go` | 264 |
| `internal/context/serializer_test.go` | 202 |
| `internal/context/budget_helpers_test.go` | 152 |
| `internal/context/compressor_accessors_test.go` | 115 |
| `internal/context/token_counter_extra_test.go` | 108 |
| `internal/context/feedback_store_test.go` | 71 |
| `internal/context/activation_setters_test.go` | 63 |
| `internal/context/feedback_store_scoring_test.go` | 55 |
| `internal/context/mocks_test.go` | 49 |

## 5. Behavior summary

Package **context** is a living codeNERD subsystem: Context activation, scoring, and window management.

APIs and control flow are defined by the cited source files — this corpus does not invent signatures.

## 6. Known limitations

- Inventory/type extraction is automated; deepen narrative when this package is the active design target.
- Completeness heuristic (90%) is not go cover %.
- Cross-package wiring claims require grep of registration hubs before treating as live.
