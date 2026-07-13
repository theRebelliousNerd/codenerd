# retrieval — Current State

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/retrieval/` (complete internal coverage)
> **Implementation: `internal/retrieval/` — 4 non-test .go, 6 tests, 0 .mg**


## 1. Source location

- Primary package: `internal/retrieval/` (exists; 4 non-test Go files)
- 1:1 mapping: `Docs/architecture/retrieval/` ↔ `internal/retrieval/`

## 2. Largest source files

| Path | Lines | Kind |
|------|------:|------|
| `internal/retrieval/sparse.go` | 814 | source |
| `internal/retrieval/tiered_context.go` | 546 | source |
| `internal/retrieval/scanner_amd64.go` | 99 | source |
| `internal/retrieval/scanner_generic.go` | 29 | source |

## 3. Complete source inventory (capped)

| Path | Lines |
|------|------:|
| `internal/retrieval/scanner_amd64.go` | 99 |
| `internal/retrieval/scanner_generic.go` | 29 |
| `internal/retrieval/sparse.go` | 814 |
| `internal/retrieval/tiered_context.go` | 546 |

## 4. Tests (sample)

| Path | Lines |
|------|------:|
| `internal/retrieval/tiered_context_coverage_test.go` | 357 |
| `internal/retrieval/sparse_test.go` | 293 |
| `internal/retrieval/sparse_integration_test.go` | 205 |
| `internal/retrieval/sparse_search_test.go` | 205 |
| `internal/retrieval/tiered_context_test.go` | 75 |
| `internal/retrieval/sparse_bench_test.go` | 17 |

## 5. Behavior summary

Package **retrieval** is a living codeNERD subsystem: Retrieval / knowledge lookup helpers.

APIs and control flow are defined by the cited source files — this corpus does not invent signatures.

## 6. Known limitations

- Inventory/type extraction is automated; deepen narrative when this package is the active design target.
- Completeness heuristic (90%) is not go cover %.
- Cross-package wiring claims require grep of registration hubs before treating as live.
