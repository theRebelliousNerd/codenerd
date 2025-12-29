# internal/world - World Model & Code Graph Projection

This package implements the World Model layer of codeNERD - projecting the filesystem and AST into Mangle facts that form the Extensional Database (EDB) for logical reasoning.

**Related Packages:**
- [internal/core](../core/CLAUDE.md) - Kernel that consumes world facts
- [internal/store](../store/CLAUDE.md) - Persistence for world facts

## Architecture

The world package projects the codebase into logic facts:
- **file_topology** - File metadata (path, hash, language, lines)
- **symbol_graph** - Code symbols (functions, types, methods)
- **code_defines/code_calls** - Holographic structural relationships
- **assigns/guards/uses** - Data flow facts for taint analysis

## File Index

| File | Description |
|------|-------------|
| `apply_incremental.go` | Updates kernel with incremental scan results. Exports ApplyIncrementalResult() for delta or full world fact replacement. |
| `ast.go` | ASTParser for multi-language code parsing with tree-sitter fallback. Exports NewASTParser() and Parse() supporting Go, Python, Rust, TypeScript, JavaScript. |
| `ast_test.go` | Unit tests for ASTParser parsing logic. Tests symbol extraction and fact generation across languages. |
| `ast_treesitter.go` | Tree-sitter binding for multi-language AST parsing. Exports TreeSitterParser with ParseGo(), ParsePython(), ParseRust(), ParseTypeScript(). |
| `cache.go` | FileCache for metadata caching to avoid re-hashing unchanged files. Exports NewFileCache(), Save(), and CacheEntry for incremental scanning. |
| `cartographer.go` | Holographic Code Graph projection with structural facts. Exports Cartographer with MapFile() emitting code_defines, code_calls, dependency_link. |
| `code_elements.go` | CodeElement DOM representation with stable references. Exports ElementType, Visibility, ActionType constants and CodeElement struct. |
| `code_elements_mangle.go` | Mangle syntax statement parsing for symbol extraction. Exports splitMangleStatements() and extractMangleSymbolFacts(). |
| `code_elements_mangle_test.go` | Unit tests for Mangle file parsing. Tests statement splitting and predicate symbol extraction. |
| `dataflow.go` | DataFlowExtractor using program slicing for Go data flow analysis. Exports ExtractDataFlow() emitting assigns, guards_return, guards_block, uses facts. |
| `dataflow_cache.go` | DataFlowCache with hash-based invalidation for persistent fact caching. Exports Load(), Store(), and GetEntry() for per-file analysis results. |
| `dataflow_cache_test.go` | Unit tests for DataFlowCache persistence and invalidation. Tests hash-based cache invalidation and serialization. |
| `dataflow_multilang.go` | MultiLangDataFlowExtractor extending data flow to Python, TS, JS, Rust. Exports ExtractDataFlow() and DetectLanguage() using tree-sitter. |
| `dataflow_multilang_test.go` | Unit tests for multi-language data flow extraction. Tests taint analysis across Python, TypeScript, JavaScript, Rust. |
| `dataflow_test.go` | Unit tests for DataFlowExtractor program slicing. Tests assignment, guard, and use fact extraction. |
| `deep_scan.go` | DeepResult and EnsureDeepFacts for Cartographer deep parsing. Exports parallel Go file deep scanning with fingerprint-based caching. |
| `fs.go` | Scanner for filesystem indexing with concurrent workers. Exports ScanWorkspace() and ScanWorkspaceCtx() for file_topology fact generation. |
| `fs_cache_test.go` | Unit tests for FileCache caching logic. Tests cache persistence and dirty tracking. |
| `fs_test.go` | Unit tests for Scanner filesystem scanning. Tests file topology extraction and ignore pattern filtering. |
| `holographic.go` | HolographicContext provider for rich multi-dimensional code context. Exports BuildHolographicContext() with package scope, architectural layer, and call graph. |
| `holographic_test.go` | Unit tests for HolographicContext building. Tests package sibling, import, and symbol resolution. |
| `incremental_scan.go` | IncrementalResult and ScanWorkspaceIncremental for fast delta scanning. Exports cache-aware scanning with change detection and fact diffing. |
| `mangle_fastparse.go` | Fast Mangle symbol extraction without full parsing. Exports extractMangleSymbolFacts() for predicate/arity symbol_graph generation. |
| `persist.go` | PersistFastSnapshotToDB for world fact persistence to LocalStore. Exports full fast scan snapshot writing for DB/scan.mg synchronization. |
| `scanner_config.go` | ScannerConfig for workspace scanning performance tuning. Exports DefaultScannerConfig() with concurrency, ignore patterns, and max file size. |
| `scope.go` | FileScope managing 1-hop dependency scope for Code DOM. Exports NewFileScope() with active file, in-scope files, elements, and dependency maps. |
| `scope_mangle_test.go` | Unit tests for Mangle scope handling in FileScope. Tests predicate element extraction and scope calculation. |
| `scope_package_test.go` | Unit tests for Go package scope resolution. Tests import resolution and sibling file loading. |
| `world_predicates.go` | WorldPredicates list of EDB predicates produced by world model. Exports WorldPredicateSet() for safe kernel fact replacement. |

## Key Types

### Scanner
```go
type Scanner struct {
    config ScannerConfig
}

func NewScanner() *Scanner
func (s *Scanner) ScanWorkspace(root string) ([]core.Fact, error)
func (s *Scanner) ScanWorkspaceIncremental(ctx, root, db, opts) (*IncrementalResult, error)
```

### Cartographer
```go
type Cartographer struct {
    dataFlowExtractor *MultiLangDataFlowExtractor
}

func NewCartographer() *Cartographer
func (c *Cartographer) MapFile(path string) ([]core.Fact, error)
```

### HolographicContext
```go
type HolographicContext struct {
    TargetFile        string
    PackageSiblings   []string
    PackageSignatures []SymbolSignature
    Layer             string  // "core", "api", "data", "cmd"
    CallGraph         []CallEdge
    ImpactPriority    int
}

func BuildHolographicContext(ctx, path, root, kernel) (*HolographicContext, error)
```

### FileScope
```go
type FileScope struct {
    ActiveFile   string
    InScope      []string
    Elements     []CodeElement
    OutboundDeps map[string][]string
    InboundDeps  map[string][]string
}

func NewFileScope(projectRoot string) *FileScope
func (fs *FileScope) Open(path string) error
func (fs *FileScope) ScopeFacts() []core.Fact
```

## World Predicates

| Predicate | Arguments | Description |
|-----------|-----------|-------------|
| `file_topology` | Path, Hash, Lang, Lines | File metadata |
| `directory` | Path | Directory existence |
| `symbol_graph` | SymbolID, Type, Visibility, File, Sig | Code symbols |
| `dependency_link` | CallerID, CalleeID, ImportPath | Import relationships |
| `code_defines` | File, Symbol, Type, Start, End | Definition locations |
| `code_calls` | Caller, Callee | Call graph edges |
| `assigns` | Var, TypeClass, File, Line | Variable assignments |
| `guards_return` | Var, GuardType, File, Line | Early return guards |
| `guards_block` | Var, GuardType, File, Start, End | Block guards |
| `uses` | File, Func, Var, Line | Variable uses |

## Scan Modes

### Fast Scan (`/scan`)
- File topology and basic symbols
- Concurrent workers for performance
- Uses FileCache for change detection

### Deep Scan (`/scan --deep`)
- Full Cartographer analysis
- Data flow extraction
- Holographic context building

### Incremental Scan
- Delta-based updates
- Fingerprint comparison
- Fact retraction for deleted files

## Testing

```bash
go test ./internal/world/...
```

---

**Remember: Push to GitHub regularly!**
