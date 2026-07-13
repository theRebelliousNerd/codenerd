# world — Implemented Spec

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/world/` (complete internal coverage)
> **Implementation: `internal/world/` — 37 non-test .go, 31 tests, 1 .mg**


## 1. Purpose

World model: filesystem topology, AST/symbol projection

## 2. Source paths (1:1)

| Path | Role |
|------|------|
| `internal/world/` | Primary implementation |
| `Docs/architecture/world/` | This full corpus |

## 3. Implementation Status

> Living code status — **not** pre-implementation zeroing.

| Component | Status | Completion |
|-----------|--------|------------|
| Source package tree | Implemented | **85%** |
| Exported types (sampled) | Implemented | **80%** |
| Tests | Implemented | **85%** |
| Mangle local sources | Implemented | **85%** |
| Full architecture corpus | Implemented | **100%** |

**Overall (heuristic): 85%** as living package (37 src / 31 tests)

## 4. Public surface inventory

### Largest files

| Path | Lines |
|------|------:|
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

### Types (sampled)

| Type | Location |
|------|----------|
| `ASTParser` | `internal/world/ast.go:14` |
| `TreeSitterParser` | `internal/world/ast_treesitter.go:21` |
| `CacheEntry` | `internal/world/cache.go:12` |
| `FileCache` | `internal/world/cache.go:19` |
| `Cartographer` | `internal/world/cartographer.go:31` |
| `ElementType` | `internal/world/code_elements.go:14` |
| `Visibility` | `internal/world/code_elements.go:34` |
| `ActionType` | `internal/world/code_elements.go:42` |
| `CodeElement` | `internal/world/code_elements.go:54` |
| `CodeElementParser` | `internal/world/code_elements.go:133` |
| `CodePatterns` | `internal/world/code_elements.go:270` |
| `APIPattern` | `internal/world/code_elements.go:282` |
| `DataFlowExtractor` | `internal/world/dataflow.go:27` |
| `DataFlowSummary` | `internal/world/dataflow.go:654` |
| `DataFlowCache` | `internal/world/dataflow_cache.go:26` |
| `CacheDataFlowEntry` | `internal/world/dataflow_cache.go:38` |
| `SerializedFact` | `internal/world/dataflow_cache.go:50` |
| `SerializedArg` | `internal/world/dataflow_cache.go:56` |
| `CacheStats` | `internal/world/dataflow_cache.go:399` |
| `MultiLangDataFlowExtractor` | `internal/world/dataflow_multilang.go:15` |
| `MultiLangDataFlowSummary` | `internal/world/dataflow_multilang.go:102` |
| `DeepResult` | `internal/world/deep_scan.go:16` |
| `Scanner` | `internal/world/fs.go:18` |
| `ScanResult` | `internal/world/fs.go:104` |
| `GoCodeParser` | `internal/world/go_parser.go:17` |
| `GraphQuery` | `internal/world/graph_interface.go:7` |
| `HolographicContext` | `internal/world/holographic.go:27` |
| `PrioritizedCaller` | `internal/world/holographic.go:67` |
| `SymbolSignature` | `internal/world/holographic.go:76` |
| `TypeDefinition` | `internal/world/holographic.go:88` |
| `ConstDefinition` | `internal/world/holographic.go:99` |
| `ImportInfo` | `internal/world/holographic.go:109` |
| `RelatedEntity` | `internal/world/holographic.go:115` |
| `CallEdge` | `internal/world/holographic.go:122` |
| `HolographicProvider` | `internal/world/holographic.go:128` |
| `IncrementalOptions` | `internal/world/incremental_scan.go:19` |
| `IncrementalResult` | `internal/world/incremental_scan.go:26` |
| `Manager` | `internal/world/lsp/manager.go:20` |
| `MangleCodeParser` | `internal/world/mangle_parser.go:14` |
| `ParserFactory` | `internal/world/parser_factory.go:20` |

### Functions (sampled)

| Symbol | Location |
|--------|----------|
| `ApplyIncrementalResult` | `internal/world/apply_incremental.go:8` |
| `NewASTParser` | `internal/world/ast.go:19` |
| `Parse` | `internal/world/ast.go:27` |
| `Close` | `internal/world/ast.go:139` |
| `NewTreeSitterParser` | `internal/world/ast_treesitter.go:30` |
| `Close` | `internal/world/ast_treesitter.go:42` |
| `ParseGo` | `internal/world/ast_treesitter.go:52` |
| `ParsePython` | `internal/world/ast_treesitter.go:296` |
| `ParseRust` | `internal/world/ast_treesitter.go:393` |
| `ParseJavaScript` | `internal/world/ast_treesitter.go:521` |
| `ParseTypeScript` | `internal/world/ast_treesitter.go:634` |
| `NewFileCache` | `internal/world/cache.go:27` |
| `Save` | `internal/world/cache.go:61` |
| `Get` | `internal/world/cache.go:104` |
| `Update` | `internal/world/cache.go:122` |
| `NewCartographer` | `internal/world/cartographer.go:36` |
| `MapFile` | `internal/world/cartographer.go:45` |
| `Close` | `internal/world/cartographer.go:193` |
| `SupportedLanguages` | `internal/world/cartographer.go:200` |
| `IsLanguageSupported` | `internal/world/cartographer.go:205` |
| `ToFacts` | `internal/world/code_elements.go:91` |
| `NewCodeElementParserWithFactory` | `internal/world/code_elements.go:146` |
| `NewCodeElementParserWithRoot` | `internal/world/code_elements.go:157` |
| `Factory` | `internal/world/code_elements.go:163` |
| `ParseFile` | `internal/world/code_elements.go:169` |
| `GetElement` | `internal/world/code_elements.go:227` |
| `GetElementsByType` | `internal/world/code_elements.go:237` |
| `GetElementsInRange` | `internal/world/code_elements.go:248` |
| `GetMethodsOfStruct` | `internal/world/code_elements.go:259` |
| `DetectCodePatterns` | `internal/world/code_elements.go:289` |

## 5. Integration relevance

| Surface | Relevance |
|---------|-----------|
| Kernel | Related |
| VirtualStore | Consumer if effectful |
| Shards | Related |
| Prompt JIT | Optional |
| CLI | Related via `cmd/nerd` |
| Config | Reader |

## 6. Non-goals of this corpus revision

- Full prose rewrite of every function body
- Docs/Spec 18-file product templates (`spec-doc-sprint`)
- Implementing missing features (corpus-build implementation mode)
