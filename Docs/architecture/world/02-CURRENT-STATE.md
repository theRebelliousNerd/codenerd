# world — Current State

> Last verified: **2026-07-13**  
> Inventory of `internal/world/` as implemented.

## Package stats

| Kind | Count (approx.) |
|------|----------------:|
| Non-test `.go` (root) | ~36 |
| Test `.go` (root) | ~30 |
| Subpackage `lsp/` non-test | 1 (`manager.go`) |
| Subpackage `lsp/` tests | 3 |
| Package-local `.mg` | 0 (schemas in core) |
| Testdata | `testdata/large_file.go` |
| Artifact | `debug_program_ERROR.mg` (dump, not source of truth) |

## File inventory by role

### Topology & persistence

| File | Role |
|------|------|
| `fs.go` | `Scanner`, concurrent walk, hash, topology + fast AST |
| `scanner_config.go` | Defaults, ignore, features hooks |
| `cache.go` | `.nerd/cache/manifest.json` hash cache |
| `incremental_scan.go` | Delta scan, entry points, project language |
| `apply_incremental.go` | Kernel retract/load |
| `persist.go` | Full snapshot → LocalStore |
| `deep_scan.go` | Cartographer deep facts + DB cache |
| `git_scanner.go` | `git log` → history/churn facts |
| `world_predicates.go` | Replaceable predicate list |

### AST / holographic / dataflow

| File | Role |
|------|------|
| `ast.go` | `ASTParser` facade |
| `ast_treesitter.go` | Multi-lang tree-sitter symbols |
| `cartographer.go` | Deep Go defines/calls + dataflow hook |
| `dataflow.go` | Go dataflow heuristics |
| `dataflow_cache.go` | Cached dataflow results |
| `dataflow_multilang.go` | Router + summary |
| `dataflow_python.go` / `_javascript.go` / `_rust.go` | Per-lang extractors |
| `holographic.go` | Context builder |
| `holographic_formatting.go` | AST format helpers, TODOs |
| `holographic_impact.go` | Priority callers for review |

### CodeDOM & scope

| File | Role |
|------|------|
| `parser_interface.go` | `CodeParser`, `ParseResult`, metadata |
| `parser_factory.go` | Registration + default set |
| `go_parser.go` | go/ast CodeDOM |
| `python_parser.go` | Tree-sitter CodeDOM |
| `typescript_parser.go` | TS/JS CodeDOM |
| `rust_parser.go` | Rust CodeDOM |
| `mangle_parser.go` | Mangle CodeDOM |
| `mangle_fastparse.go` | Scan-path mangle symbols |
| `code_elements.go` | Element model, patterns, facts |
| `code_elements_mangle.go` | Mangle-specific element parse |
| `scope.go` | 1-hop FileScope |

### Misc

| File | Role |
|------|------|
| `types.go` | Fact/MangleAtom aliases |
| `graph_interface.go` | GraphQuery alias |
| `test_dependency.go` | Test→source graph |
| `lsp/manager.go` | Mangle LSP projection |
| `lsp/README.md` | Subpackage architecture notes |

## Hotspots (edit carefully)

1. **`fs.go` + `incremental_scan.go`** — path identity, concurrency, ignore rules; regressions break all sessions.
2. **`cartographer.go` + `dataflow.go`** — fact shape consumed by policy impact rules.
3. **`scope.go`** — CodeDOM + diagnostics + VirtualStore bridge types.
4. **`apply_incremental.go` + `world_predicates.go`** — wrong set → ghost facts or over-delete.
5. **`holographic_impact.go`** — prompt size / correctness for campaigns.

## Runtime modes in practice

| Mode | Trigger | Output |
|------|---------|--------|
| Full fast scan | Init, first incremental, explicit scan | Full topology + symbol_graph |
| Incremental fast | Chat sync / dream / instruction | Delta facts + directory refresh |
| Deep | EnsureDeepFacts / HolographicCodeScope | code_defines/calls/dataflow for Go |
| Scope open | VirtualStore code tools | CodeDOM + active_file |
| Holographic get | Campaign / review | Prompt structs |
| LSP index | mangle-lsp / ProjectToFacts | symbol_defined/referenced/diags |

## Dual / overlapping systems

| System A | System B | Overlap |
|----------|----------|---------|
| Chat `Scanner` incremental | `WorldModelIngestorShard` + `ASTParser` | Both can feed topology/symbols |
| Fast tree-sitter symbols | Cartographer deep graph | Different predicates, same files |
| FileScope elements | Holographic package parse | Both parse Go package files |

These are intentional layers more than bugs, but operators must know which path ran.

## Completeness snapshot

| Area | State |
|------|-------|
| Topology | Production mature |
| Fast multi-lang AST | Production usable |
| Deep Go graph | Production usable |
| Deep non-Go graph | Code partial / entry incomplete |
| CodeDOM | Mature factory; scope Go-strong |
| Holographic | Go-strong; impact depends on kernel facts |
| LSP | Mangle only |
| Tests | Broad unit coverage; integration uneven |
