# world — Current State

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/world/` (complete internal coverage)
> **Implementation: `internal/world/` — 37 non-test .go, 31 tests, 1 .mg**


## 1. Source location

- Primary package: `internal/world/` (exists; 37 non-test Go files)
- 1:1 mapping: `Docs/architecture/world/` ↔ `internal/world/`

## 2. Largest source files

| Path | Lines | Kind |
|------|------:|------|
| `internal/world/scope.go` | 1023 | source |
| `internal/world/rust_parser.go` | 784 | source |
| `internal/world/holographic_impact.go` | 750 | source |
| `internal/world/ast_treesitter.go` | 736 | source |
| `internal/world/dataflow.go` | 706 | source |
| `internal/world/typescript_parser.go` | 643 | source |
| `internal/world/holographic.go` | 614 | source |
| `internal/world/fs.go` | 571 | source |
| `internal/world/dataflow_javascript.go` | 551 | source |
| `internal/world/incremental_scan.go` | 545 | source |
| `internal/world/dataflow_rust.go` | 526 | source |
| `internal/world/test_dependency.go` | 473 | source |
| `internal/world/dataflow_cache.go` | 467 | source |
| `internal/world/dataflow_python.go` | 465 | source |
| `internal/world/code_elements.go` | 464 | source |
| `internal/world/go_parser.go` | 459 | source |
| `internal/world/code_elements_mangle.go` | 457 | source |
| `internal/world/lsp/manager.go` | 347 | source |
| `internal/world/python_parser.go` | 343 | source |
| `internal/world/holographic_formatting.go` | 236 | source |

## 3. Complete source inventory (capped)

| Path | Lines |
|------|------:|
| `internal/world/apply_incremental.go` | 48 |
| `internal/world/ast.go` | 144 |
| `internal/world/ast_treesitter.go` | 736 |
| `internal/world/cache.go` | 132 |
| `internal/world/cartographer.go` | 208 |
| `internal/world/code_elements.go` | 464 |
| `internal/world/code_elements_mangle.go` | 457 |
| `internal/world/dataflow.go` | 706 |
| `internal/world/dataflow_cache.go` | 467 |
| `internal/world/dataflow_javascript.go` | 551 |
| `internal/world/dataflow_multilang.go` | 166 |
| `internal/world/dataflow_python.go` | 465 |
| `internal/world/dataflow_rust.go` | 526 |
| `internal/world/deep_scan.go` | 114 |
| `internal/world/fs.go` | 571 |
| `internal/world/git_scanner.go` | 107 |
| `internal/world/go_parser.go` | 459 |
| `internal/world/graph_interface.go` | 7 |
| `internal/world/holographic.go` | 614 |
| `internal/world/holographic_formatting.go` | 236 |
| `internal/world/holographic_impact.go` | 750 |
| `internal/world/incremental_scan.go` | 545 |
| `internal/world/lsp/manager.go` | 347 |
| `internal/world/mangle_fastparse.go` | 31 |
| `internal/world/mangle_parser.go` | 220 |
| `internal/world/parser_factory.go` | 202 |
| `internal/world/parser_interface.go` | 110 |
| `internal/world/persist.go` | 59 |
| `internal/world/python_parser.go` | 343 |
| `internal/world/rust_parser.go` | 784 |
| `internal/world/scanner_config.go` | 100 |
| `internal/world/scope.go` | 1023 |
| `internal/world/test_dependency.go` | 473 |
| `internal/world/testdata/large_file.go` | 1 |
| `internal/world/types.go` | 12 |
| `internal/world/typescript_parser.go` | 643 |
| `internal/world/world_predicates.go` | 37 |

## 4. Tests (sample)

| Path | Lines |
|------|------:|
| `internal/world/parser_test.go` | 1248 |
| `internal/world/holographic_test.go` | 951 |
| `internal/world/dataflow_test.go` | 802 |
| `internal/world/scan_edge_test.go` | 520 |
| `internal/world/fs_test.go` | 458 |
| `internal/world/dataflow_cache_test.go` | 452 |
| `internal/world/dataflow_multilang_test.go` | 452 |
| `internal/world/test_dependency_test.go` | 356 |
| `internal/world/parser_factory_test.go` | 344 |
| `internal/world/ast_test.go` | 297 |

## 5. Behavior summary

Package **world** is a living codeNERD subsystem: World model: filesystem topology, AST/symbol projection.

APIs and control flow are defined by the cited source files — this corpus does not invent signatures.

## 6. Known limitations

- Inventory/type extraction is automated; deepen narrative when this package is the active design target.
- Completeness heuristic (85%) is not go cover %.
- Cross-package wiring claims require grep of registration hubs before treating as live.
