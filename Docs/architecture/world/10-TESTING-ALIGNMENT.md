# world — Testing Alignment

> Last verified: **2026-07-13**

## Commands

```powershell
go test ./internal/world/...
go test ./internal/world/lsp/...
```

Targeted:

```powershell
go test ./internal/world/ -run Scan -count=1
go test ./internal/world/ -run DataFlow -count=1
go test ./internal/world/ -run Holographic -count=1
go test ./internal/world/ -run FileScope -count=1
go test ./internal/world/lsp/ -count=1
```

## Test file map

| Test file | Focus |
|-----------|-------|
| `fs_test.go` | Scanner, language, test-file, topology shape |
| `fs_cache_test.go` | Blind-spot hidden dirs, cache hit behavior |
| `cache_test.go` | Load/save, corrupt, errors |
| `incremental_scan_test.go` | Delta scan |
| `cartographer_test.go` | Go map file defines/calls |
| `ast_test.go` | Py/RS/TS/JS ASTParser |
| `dataflow_test.go` | Guards, uses, call args, concurrency |
| `dataflow_multilang_test.go` | Multi-lang extract + cartographer language helpers |
| `dataflow_cache_test.go` | GetOrCompute, invalidate, version |
| `code_elements*_test.go` | Elements, patterns, mangle, factory |
| `go_parser_test.go` | Interface compliance |
| `parser_factory_test.go` / `parser_test.go` | Factory routing, polyglot |
| `scope_test.go` / `scope_package_test.go` / `scope_mangle_test.go` | FileScope |
| `holographic*_test.go` | Context, priorities, perf edges |
| `persist_test.go` | Snapshot |
| `world_predicates_test.go` | Predicate set |
| `test_dependency_test.go` | Test graph |
| `graph_interface_test.go` | Alias |
| `scan_edge_test.go` | Edge cases |
| `reviewer_capabilities_test.go` | Reviewer-oriented capabilities |
| `lsp/manager*_test.go` | Initialize, project, extras |

## Coverage strengths

- Topology walk + ignore + cache nano invalidation
- Go dataflow patterns (nil, error, dominates)
- Multi-lang tree-sitter smoke for symbols/dataflow
- Holographic nil-safety and priority formatting
- FileScope Go package behavior

## Testing gaps

| Gap | Risk | Suggested test |
|-----|------|----------------|
| Full vs incremental **path identity** | Duplicate/stale EDB | Property: same relative Path for same file |
| `ApplyIncrementalResult` Full replace completeness | Ghost facts | Assert predicates outside set remain vs listed |
| Deep scan non-Go intentionally empty | Regression if MapFile extended | Document assert `.py` → 0 deep defines |
| End-to-end chat sync wiring | Integration silent fail | `helpers_scan` level test with fake kernel |
| Git scanner | Env dependent | Skip or fixture git repo |
| Large repo performance | CI time | Keep optional / benchmark tags |

## Alignment with principles

Tests currently enforce many **performance/correctness fixes** called out in comments (blind spot hidden dirs, hash thrashing, semaphore ordering via behavior). Prefer regression tests when editing those comments.

## Fixtures

- `testdata/large_file.go` — size-gate / perf paths
- Inline temp dirs in most unit tests
