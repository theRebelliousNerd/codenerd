# world — Public API and Types

> Last verified: **2026-07-13**  
> Exported surface that callers actually use. File refs are under `internal/world/` unless noted.

## Type aliases

| Alias | Canonical | File |
|-------|-----------|------|
| `Fact` | `types.Fact` | `types.go` |
| `MangleAtom` | `types.MangleAtom` | `types.go` |
| `GraphQuery` | `types.GraphQuery` | `graph_interface.go` |

## Scanning

### Types

| Type | File | Notes |
|------|------|-------|
| `Scanner` | `fs.go` | Concurrent topology + fast AST |
| `ScannerConfig` | `scanner_config.go` | Workers, ignore, AST size |
| `ScanResult` | `fs.go` | Counts + facts + language map |
| `IncrementalOptions` | `incremental_scan.go` | `SkipWhenUnchanged` |
| `IncrementalResult` | `incremental_scan.go` | Full/delta/unchanged |
| `DeepResult` | `deep_scan.go` | Deep facts + retracts |
| `FileCache` / `CacheEntry` | `cache.go` | Hash manifest |

### Functions

| Symbol | File | Role |
|--------|------|------|
| `NewScanner` | `fs.go` | Default config |
| `NewScannerWithConfig` | `fs.go` | Custom config |
| `DefaultScannerConfig` | `scanner_config.go` | Features-aware defaults |
| `(*Scanner).ScanWorkspace` | `fs.go` | Full scan |
| `(*Scanner).ScanWorkspaceCtx` | `fs.go` | Cancellable full scan |
| `(*Scanner).ScanDirectory` | `fs.go` | Core walk |
| `(*Scanner).ScanWorkspaceIncremental` | `incremental_scan.go` | Delta/full |
| `ApplyIncrementalResult` | `apply_incremental.go` | Kernel apply |
| `PersistFastSnapshotToDB` | `persist.go` | DB snapshot |
| `EnsureDeepFacts` | `deep_scan.go` | Cartographer batch |
| `ScanGitHistory` | `git_scanner.go` | Git facts |
| `NewFileCache` | `cache.go` | Load manifest |
| `(*FileCache).Get/Update/Save` | `cache.go` | Cache ops |

## Predicates

| Symbol | File |
|--------|------|
| `WorldPredicates` | `world_predicates.go` |
| `WorldPredicateSet` | `world_predicates.go` |

## Cartographer & dataflow

| Symbol | File |
|--------|------|
| `Cartographer` / `NewCartographer` / `MapFile` / `Close` | `cartographer.go` |
| `SupportedLanguages` / `IsLanguageSupported` | `cartographer.go` |
| `DataFlowExtractor` / `NewDataFlowExtractor` / `ExtractDataFlow` | `dataflow.go` |
| `DataFlowSummary` / `SummarizeDataFlow` | `dataflow.go` |
| `MultiLangDataFlowExtractor` / `NewMultiLang…` / `ExtractDataFlow` | `dataflow_multilang.go` |
| `DetectLanguage` | `dataflow_multilang.go` |
| `MultiLangDataFlowSummary` / `SummarizeMultiLangDataFlow` | `dataflow_multilang.go` |
| `DataFlowCache` / `NewDataFlowCache` | `dataflow_cache.go` |
| `CacheStats` | `dataflow_cache.go` |

## AST facades

| Symbol | File |
|--------|------|
| `ASTParser` / `NewASTParser` / `Parse` / `Close` | `ast.go` |
| `TreeSitterParser` / `NewTreeSitterParser` / `ParseGo|Python|…` / `Close` | `ast_treesitter.go` |

## CodeDOM

| Symbol | File |
|--------|------|
| `CodeParser` interface | `parser_interface.go` |
| `LanguageMetadata`, `ParseResult`, `ParseError` | `parser_interface.go` |
| `ParserFactory` / `NewParserFactory` / `DefaultParserFactory` | `parser_factory.go` |
| `Register`, `GetParser`, `HasParser`, `Parse`, `ParseWithFacts`, `EmitAllFacts` | `parser_factory.go` |
| `GoCodeParser` / `NewGoCodeParser` | `go_parser.go` |
| `PythonCodeParser` / `NewPythonCodeParser` | `python_parser.go` |
| `TypeScriptCodeParser` / `NewTypeScriptCodeParser` | `typescript_parser.go` |
| `RustCodeParser` / `NewRustCodeParser` | `rust_parser.go` |
| `MangleCodeParser` / `NewMangleCodeParser` | `mangle_parser.go` |
| `ElementType`, `Visibility`, `ActionType` | `code_elements.go` |
| `CodeElement` / `ToFacts` | `code_elements.go` |
| `CodeElementParser` / `NewCodeElementParserWithFactory` / `WithRoot` | `code_elements.go` |
| `GetElement`, `GetElementsByType`, `GetElementsInRange`, `GetMethodsOfStruct` | `code_elements.go` |
| `DetectCodePatterns`, `CodePatterns`, `APIPattern` | `code_elements.go` |
| `ElementsToFacts` | `code_elements.go` |

## Scope

| Symbol | File |
|--------|------|
| `FileScope` / `NewFileScope` | `scope.go` |
| `SetFactCallback`, `Open`, `Refresh`, `Close` | `scope.go` |
| Element getters / `ScopeFacts` / hash verify | `scope.go` |
| `EncodingInfo` | `scope.go` |

## Holographic

| Symbol | File |
|--------|------|
| `HolographicContext` and nested types | `holographic.go` |
| `HolographicProvider` / `NewHolographicProvider` | `holographic.go` |
| `GetContext`, `GetContextWithContext` | `holographic.go` |
| `BuildWithImpactPriorities`, `ResolvePrioritizedCallers` | `holographic_impact.go` |
| `CountTODOs` | `holographic_formatting.go` |

## Test dependency

| Symbol | File |
|--------|------|
| `TestDependencyBuilder` / `NewTestDependencyBuilder` / `Build` | `test_dependency.go` |

## LSP (`internal/world/lsp`)

| Symbol | File |
|--------|------|
| `Manager` / `NewManager` | `lsp/manager.go` |
| `Initialize`, `ProjectToFacts` | `lsp/manager.go` |
| Definition/reference/diagnostic batch helpers | `lsp/manager.go` |
| `ServeStdio` (if present in full file) | `lsp/manager.go` |

## Common call patterns

```go
// Boot / init topology
scanner := world.NewScanner()
facts, err := scanner.ScanWorkspace(root)

// Chat steady state
res, err := scanner.ScanWorkspaceIncremental(ctx, root, db, world.IncrementalOptions{SkipWhenUnchanged: true})
_ = world.ApplyIncrementalResult(kernel, res)

// Deep Go graph for open files
deep, err := world.EnsureDeepFacts(ctx, paths, db, workers)

// Agent review context
hp := world.NewHolographicProvider(realKernel, workDir)
hc, err := hp.BuildWithImpactPriorities(ctx, file)

// CodeDOM scope
scope := world.NewFileScope(root)
_ = scope.Open(path)
```
