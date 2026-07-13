# tools — Current State

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/tools/` (complete internal coverage)
> **Implementation: `internal/tools/` — 25 non-test .go, 21 tests, 0 .mg**


## 1. Source location

- Primary package: `internal/tools/` (exists; 25 non-test Go files)
- 1:1 mapping: `Docs/architecture/tools/` ↔ `internal/tools/`

## 2. Largest source files

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
| `internal/tools/codedom/elements.go` | 239 | source |
| `internal/tools/research/thinking.go` | 211 | source |
| `internal/tools/types.go` | 134 | source |
| `internal/tools/core/workspace_guard.go` | 102 | source |
| `internal/tools/research/register.go` | 44 | source |
| `internal/tools/codedom/register.go` | 34 | source |
| `internal/tools/core/register.go` | 33 | source |
| `internal/tools/shell/register.go` | 29 | source |

## 3. Complete source inventory (capped)

| Path | Lines |
|------|------:|
| `internal/tools/codedom/doc.go` | 15 |
| `internal/tools/codedom/elements.go` | 239 |
| `internal/tools/codedom/lines.go` | 279 |
| `internal/tools/codedom/register.go` | 34 |
| `internal/tools/codedom/run_impacted_tests.go` | 433 |
| `internal/tools/core/doc.go` | 14 |
| `internal/tools/core/file_ops.go` | 465 |
| `internal/tools/core/register.go` | 33 |
| `internal/tools/core/search.go` | 362 |
| `internal/tools/core/workspace_guard.go` | 102 |
| `internal/tools/errors.go` | 27 |
| `internal/tools/registry.go` | 341 |
| `internal/tools/research/browser.go` | 355 |
| `internal/tools/research/cache.go` | 303 |
| `internal/tools/research/context7.go` | 297 |
| `internal/tools/research/doc.go` | 12 |
| `internal/tools/research/grounding.go` | 334 |
| `internal/tools/research/register.go` | 44 |
| `internal/tools/research/thinking.go` | 211 |
| `internal/tools/research/web_fetch.go` | 263 |
| `internal/tools/research/web_search.go` | 246 |
| `internal/tools/shell/doc.go` | 11 |
| `internal/tools/shell/execute.go` | 772 |
| `internal/tools/shell/register.go` | 29 |
| `internal/tools/types.go` | 134 |

## 4. Tests (sample)

| Path | Lines |
|------|------:|
| `internal/tools/research/research_coverage_test.go` | 1912 |
| `internal/tools/research/research_test.go` | 398 |
| `internal/tools/shell/execute_test.go` | 389 |
| `internal/tools/core/search_test.go` | 364 |
| `internal/tools/codedom/elements_test.go` | 359 |
| `internal/tools/core/file_ops_test.go` | 352 |
| `internal/tools/codedom/lines_test.go` | 293 |
| `internal/tools/shell/shell_integration_test.go` | 281 |
| `internal/tools/codedom/impact_test.go` | 231 |
| `internal/tools/registry_test.go` | 227 |

## 5. Behavior summary

Package **tools** is a living codeNERD subsystem: Tool registry and research/tool integrations.

APIs and control flow are defined by the cited source files — this corpus does not invent signatures.

## 6. Known limitations

- Inventory/type extraction is automated; deepen narrative when this package is the active design target.
- Completeness heuristic (85%) is not go cover %.
- Cross-package wiring claims require grep of registration hubs before treating as live.
