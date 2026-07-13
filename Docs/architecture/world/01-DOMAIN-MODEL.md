# world — Domain Model

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/world/` (complete internal coverage)
> **Implementation: `internal/world/` — 37 non-test .go, 31 tests, 1 .mg**


## Package

`internal/world/`

## Exported types (sampled, up to 40)

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

## Exported functions/methods (sampled, up to 30)

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

## Mangle surface

| Artifact | Count |
|----------|------:|
| Package-local `.mg` | 1 |

| Path | Lines |
|------|------:|
| `internal/world/debug_program_ERROR.mg` | 17975 |

Global kernel schemas/policy (when this package participates):

- `internal/core/defaults/schemas.mg`
- `internal/core/defaults/policy/`

## Fact-flow placement

```
user input → perception → user_intent → kernel(core/mangle) → next_action
  → VirtualStore / shards / tools → articulation
```

This package: **World model: filesystem topology, AST/symbol projection**
