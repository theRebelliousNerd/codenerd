# tools — Current State

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded corpus**
> Mode: dark-factory autonomous generation via arch-propose/corpus-build port
> **Implementation: present under `internal/tools/` (25 non-test .go, 21 tests, 0 .mg)**


## 1. Source location

- Primary package: `internal/tools/` (**exists** with 25 non-test Go files)
- Supporting global surfaces: `internal/core/defaults/` when schemas/policy apply

## 2. File inventory (largest sources)

| Path | Lines | Kind |
|------|------:|------|
| `internal/tools/shell/execute.go` | 772 | source |
| `internal/tools/core/file_ops.go` | 465 | source |
| `internal/tools/codedom/run_impacted_tests.go` | 433 | source |
| `internal/tools/core/search.go` | 362 | source |
| `internal/tools/research/browser.go` | 355 | source |
| `internal/tools/registry.go` | 341 | source |
| `internal/tools/research/grounding.go` | 334 | source |
| `internal/tools/research/cache.go` | 303 | source |
| `internal/tools/research/context7.go` | 297 | source |
| `internal/tools/codedom/lines.go` | 279 | source |
| `internal/tools/research/web_fetch.go` | 263 | source |
| `internal/tools/research/web_search.go` | 246 | source |

## 3. Test inventory (sample)

| Path | Lines |
|------|------:|
| `internal/tools/research/research_coverage_test.go` | 1912 |
| `internal/tools/research/research_test.go` | 398 |
| `internal/tools/shell/execute_test.go` | 389 |
| `internal/tools/core/search_test.go` | 364 |
| `internal/tools/codedom/elements_test.go` | 359 |
| `internal/tools/core/file_ops_test.go` | 352 |

## 4. Current behavior (summary)

Package **tools** is a living codeNERD subsystem: Tool registry and research/tool integrations.

Behavior is defined by the source files above. This corpus does **not** invent APIs —
consult the cited paths for signatures and control flow.

## 5. Known limitations (honest)

- Corpus generated in dark-factory mode from inventory + lightweight type extraction; deep behavioral narrative may lag micro-refactors.
- Completeness heuristic (85%) is not coverage % — run `go test` for truth.
- Cross-package wiring must be validated against `internal/shards/registration.go` and VirtualStore routes when relevant.
