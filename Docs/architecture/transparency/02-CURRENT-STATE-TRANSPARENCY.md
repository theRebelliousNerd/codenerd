# transparency — Current State

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/transparency/` (complete internal coverage)
> **Implementation: `internal/transparency/` — 8 non-test .go, 9 tests, 0 .mg**


## 1. Source location

- Primary package: `internal/transparency/` (exists; 8 non-test Go files)
- 1:1 mapping: `Docs/architecture/transparency/` ↔ `internal/transparency/`

## 2. Largest source files

| Path | Lines | Kind |
|------|------:|------|
| `internal/transparency/explainer.go` | 327 | source |
| `internal/transparency/safety_reporter.go` | 306 | source |
| `internal/transparency/shard_observer.go` | 305 | source |
| `internal/transparency/event_bus.go` | 286 | source |
| `internal/transparency/error_classifier.go` | 266 | source |
| `internal/transparency/transparency.go` | 213 | source |
| `internal/transparency/glass_box_events.go` | 140 | source |
| `internal/transparency/doc.go` | 20 | source |

## 3. Complete source inventory (capped)

| Path | Lines |
|------|------:|
| `internal/transparency/doc.go` | 20 |
| `internal/transparency/error_classifier.go` | 266 |
| `internal/transparency/event_bus.go` | 286 |
| `internal/transparency/explainer.go` | 327 |
| `internal/transparency/glass_box_events.go` | 140 |
| `internal/transparency/safety_reporter.go` | 306 |
| `internal/transparency/shard_observer.go` | 305 |
| `internal/transparency/transparency.go` | 213 |

## 4. Tests (sample)

| Path | Lines |
|------|------:|
| `internal/transparency/transparency_comprehensive_test.go` | 901 |
| `internal/transparency/event_bus_test.go` | 140 |
| `internal/transparency/explainer_test.go` | 94 |
| `internal/transparency/shard_observer_test.go` | 90 |
| `internal/transparency/glass_box_helpers_test.go` | 63 |
| `internal/transparency/transparency_test.go` | 56 |
| `internal/transparency/glass_box_events_test.go` | 54 |
| `internal/transparency/safety_reporter_test.go` | 49 |
| `internal/transparency/error_classifier_test.go` | 42 |

## 5. Behavior summary

Package **transparency** is a living codeNERD subsystem: Transparency event bus / glass-box observability.

APIs and control flow are defined by the cited source files — this corpus does not invent signatures.

## 6. Known limitations

- Inventory/type extraction is automated; deepen narrative when this package is the active design target.
- Completeness heuristic (90%) is not go cover %.
- Cross-package wiring claims require grep of registration hubs before treating as live.
