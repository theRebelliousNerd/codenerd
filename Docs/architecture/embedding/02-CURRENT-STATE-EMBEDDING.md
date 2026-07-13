# embedding — Current State

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/embedding/` (complete internal coverage)
> **Implementation: `internal/embedding/` — 6 non-test .go, 7 tests, 0 .mg**


## 1. Source location

- Primary package: `internal/embedding/` (exists; 6 non-test Go files)
- 1:1 mapping: `Docs/architecture/embedding/` ↔ `internal/embedding/`

## 2. Largest source files

| Path | Lines | Kind |
|------|------:|------|
| `internal/embedding/ollama.go` | 611 | source |
| `internal/embedding/genai.go` | 373 | source |
| `internal/embedding/engine.go` | 216 | source |
| `internal/embedding/task_selector.go` | 198 | source |
| `internal/embedding/math_amd64.go` | 57 | source |
| `internal/embedding/math_generic.go` | 37 | source |

## 3. Complete source inventory (capped)

| Path | Lines |
|------|------:|
| `internal/embedding/engine.go` | 216 |
| `internal/embedding/genai.go` | 373 |
| `internal/embedding/math_amd64.go` | 57 |
| `internal/embedding/math_generic.go` | 37 |
| `internal/embedding/ollama.go` | 611 |
| `internal/embedding/task_selector.go` | 198 |

## 4. Tests (sample)

| Path | Lines |
|------|------:|
| `internal/embedding/ollama_coverage_test.go` | 486 |
| `internal/embedding/engine_coverage_test.go` | 448 |
| `internal/embedding/task_selector_coverage_test.go` | 409 |
| `internal/embedding/ollama_ensure_test.go` | 176 |
| `internal/embedding/genai_coverage_test.go` | 124 |
| `internal/embedding/task_selector_test.go` | 60 |
| `internal/embedding/genai_bench_test.go` | 44 |

## 5. Behavior summary

Package **embedding** is a living codeNERD subsystem: Embedding engines (including Ollama) and vector generation.

APIs and control flow are defined by the cited source files — this corpus does not invent signatures.

## 6. Known limitations

- Inventory/type extraction is automated; deepen narrative when this package is the active design target.
- Completeness heuristic (90%) is not go cover %.
- Cross-package wiring claims require grep of registration hubs before treating as live.
