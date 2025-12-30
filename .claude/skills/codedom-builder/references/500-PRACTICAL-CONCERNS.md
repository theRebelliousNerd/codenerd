# 500: Practical Implementation Concerns

## Overview

This document covers real-world implementation challenges not addressed in the core architecture docs: error recovery, incremental parsing, symbol resolution, memory management, and performance optimization.

## Error Recovery (Tree-sitter's Killer Feature)

### Why It Matters

Real code has syntax errors during development. Unlike traditional parsers that fail completely, Tree-sitter produces **valid ASTs with ERROR nodes** for broken code. This is essential for IDE-like experiences.

### Handling ERROR Nodes

```go
// Check if parsing produced errors
func (p *PythonParser) Parse(path string, content []byte) ([]CodeElement, []ParseError, error) {
    tree, err := p.parser.ParseCtx(context.Background(), nil, content)
    if err != nil {
        return nil, nil, err
    }
    defer tree.Close()

    var elements []CodeElement
    var errors []ParseError

    root := tree.RootNode()

    // Collect ERROR nodes for diagnostics
    p.collectErrors(root, path, content, &errors)

    // Still extract valid elements from non-error parts
    p.walkNode(root, path, "", &elements, content)

    return elements, errors, nil
}

func (p *PythonParser) collectErrors(node *sitter.Node, path string, content []byte, errors *[]ParseError) {
    if node.Type() == "ERROR" || node.IsMissing() {
        *errors = append(*errors, ParseError{
            File:    path,
            Line:    int(node.StartPoint().Row) + 1,
            Column:  int(node.StartPoint().Column) + 1,
            Message: fmt.Sprintf("Syntax error near: %s", truncate(string(content[node.StartByte():node.EndByte()]), 50)),
            Context: string(content[node.StartByte():min(node.EndByte()+20, uint32(len(content)))]),
        })
    }

    // Recurse into children
    for i := 0; i < int(node.ChildCount()); i++ {
        p.collectErrors(node.Child(i), path, content, errors)
    }
}

// Skip ERROR nodes during element extraction
func (p *PythonParser) walkNode(node *sitter.Node, path, parentRef string, elements *[]CodeElement, content []byte) {
    // Skip ERROR nodes but continue with siblings
    if node.Type() == "ERROR" || node.IsMissing() {
        return
    }

    // ... normal processing
}
```

### Mangle Facts for Errors

```mangle
# =============================================================================
# PARSE ERROR TRACKING
# =============================================================================

Decl parse_error(File.Type<string>, Line.Type<int>, Message.Type<string>).
Decl file_has_errors(File.Type<string>).

file_has_errors(File) :- parse_error(File, _, _).

# Block edits on files with syntax errors
deny_edit(Ref, /syntax_errors) :-
    code_element(Ref, _, File, _, _),
    file_has_errors(File).
```

## Incremental Parsing

### Why It Matters

Re-parsing entire files on every keystroke is expensive. Tree-sitter's incremental parsing only re-processes changed regions, achieving **sub-millisecond updates**.

### Implementation

```go
type IncrementalParser struct {
    parser    *sitter.Parser
    trees     map[string]*sitter.Tree  // file -> latest tree
    contents  map[string][]byte        // file -> latest content
    mu        sync.RWMutex
}

func NewIncrementalParser(lang *sitter.Language) *IncrementalParser {
    parser := sitter.NewParser()
    parser.SetLanguage(lang)
    return &IncrementalParser{
        parser:   parser,
        trees:    make(map[string]*sitter.Tree),
        contents: make(map[string][]byte),
    }
}

// Parse or re-parse a file with incremental support
func (p *IncrementalParser) ParseIncremental(path string, newContent []byte, edit *EditInfo) (*sitter.Tree, error) {
    p.mu.Lock()
    defer p.mu.Unlock()

    oldTree := p.trees[path]
    oldContent := p.contents[path]

    // If we have an old tree and edit info, do incremental parse
    if oldTree != nil && edit != nil {
        // Convert edit to Tree-sitter InputEdit
        inputEdit := sitter.InputEdit{
            StartIndex:  uint32(edit.StartByte),
            OldEndIndex: uint32(edit.OldEndByte),
            NewEndIndex: uint32(edit.NewEndByte),
            StartPoint:  pointFromOffset(oldContent, edit.StartByte),
            OldEndPoint: pointFromOffset(oldContent, edit.OldEndByte),
            NewEndPoint: pointFromOffset(newContent, edit.NewEndByte),
        }
        oldTree.Edit(inputEdit)
    }

    // Parse with old tree as reference (incremental)
    newTree, err := p.parser.ParseCtx(context.Background(), oldTree, newContent)
    if err != nil {
        return nil, err
    }

    // Clean up old tree
    if oldTree != nil {
        oldTree.Close()
    }

    // Store new state
    p.trees[path] = newTree
    p.contents[path] = newContent

    return newTree, nil
}

// EditInfo describes a text edit
type EditInfo struct {
    StartByte   int
    OldEndByte  int
    NewEndByte  int
}

func pointFromOffset(content []byte, offset int) sitter.Point {
    row := 0
    col := 0
    for i := 0; i < offset && i < len(content); i++ {
        if content[i] == '\n' {
            row++
            col = 0
        } else {
            col++
        }
    }
    return sitter.Point{Row: uint32(row), Column: uint32(col)}
}

// Close releases all trees
func (p *IncrementalParser) Close() {
    p.mu.Lock()
    defer p.mu.Unlock()
    for _, tree := range p.trees {
        tree.Close()
    }
    p.trees = nil
    p.contents = nil
}
```

### Edit Detection Integration

```go
// Integrate with file watcher
func (fs *FileScope) OnFileChanged(path string, edit EditInfo) error {
    content, err := os.ReadFile(path)
    if err != nil {
        return err
    }

    // Incremental re-parse
    tree, err := fs.incrementalParser.ParseIncremental(path, content, &edit)
    if err != nil {
        return err
    }

    // Extract elements from new tree
    elements := fs.extractElements(tree, path, content)

    // Update scope
    fs.updateFileElements(path, elements)

    // Refresh Mangle facts
    fs.refreshFacts(path, elements)

    return nil
}
```

## Symbol Resolution

### Why It Matters

Refactoring requires resolving references (function calls, imports, type usages) to their definitions. This enables "find all references" and "rename across files".

### Building the Symbol Table

```go
type Symbol struct {
    Name       string
    Kind       SymbolKind   // Function, Class, Variable, Type, etc.
    Ref        string       // CodeElement ref
    File       string
    Line       int
    Exported   bool
    Type       string       // For typed languages
}

type SymbolTable struct {
    symbols   map[string][]Symbol  // name -> symbols (multiple due to overloading/shadowing)
    byRef     map[string]Symbol    // ref -> symbol
    imports   map[string][]Import  // file -> imports
    exports   map[string][]string  // file -> exported symbol names
}

// Build symbol table from parsed elements
func BuildSymbolTable(elements []CodeElement) *SymbolTable {
    st := &SymbolTable{
        symbols: make(map[string][]Symbol),
        byRef:   make(map[string]Symbol),
        imports: make(map[string][]Import),
        exports: make(map[string][]string),
    }

    for _, elem := range elements {
        sym := Symbol{
            Name:     elem.Name,
            Kind:     elementTypeToSymbolKind(elem.Type),
            Ref:      elem.Ref,
            File:     elem.File,
            Line:     elem.StartLine,
            Exported: elem.Visibility == VisibilityPublic,
        }
        st.symbols[elem.Name] = append(st.symbols[elem.Name], sym)
        st.byRef[elem.Ref] = sym

        if sym.Exported {
            st.exports[elem.File] = append(st.exports[elem.File], elem.Name)
        }
    }

    return st
}
```

### Reference Resolution

```go
// Resolve a name to its definition(s)
func (st *SymbolTable) Resolve(name string, fromFile string, context ResolveContext) []Symbol {
    var results []Symbol

    // 1. Check local scope first
    for _, sym := range st.symbols[name] {
        if sym.File == fromFile {
            results = append(results, sym)
        }
    }

    // 2. Check imports
    for _, imp := range st.imports[fromFile] {
        if imp.Alias == name || imp.Name == name {
            // Resolve through import
            for _, sym := range st.symbols[imp.Name] {
                if sym.File == imp.SourceFile && sym.Exported {
                    results = append(results, sym)
                }
            }
        }
    }

    // 3. Check global/builtin scope
    if len(results) == 0 {
        for _, sym := range st.symbols[name] {
            if sym.Exported {
                results = append(results, sym)
            }
        }
    }

    return results
}
```

### Mangle Integration for References

```mangle
# =============================================================================
# SYMBOL RESOLUTION FACTS
# =============================================================================

Decl symbol_def(Name.Type<string>, Ref.Type<string>, File.Type<string>).
Decl symbol_ref(Name.Type<string>, Ref.Type<string>, File.Type<string>, Line.Type<int>).
Decl imports(File.Type<string>, Module.Type<string>, Alias.Type<string>).

# Resolve reference to definition
resolves_to(RefRef, DefRef) :-
    symbol_ref(Name, RefRef, RefFile, _),
    symbol_def(Name, DefRef, DefFile),
    import_path(RefFile, DefFile).

# Find all references
all_references(DefRef, RefRefs) :-
    symbol_def(_, DefRef, _),
    resolves_to(RefRef, DefRef) |>
    do fn:group_by(DefRef),
    let RefRefs = fn:collect(RefRef).

# Detect unused definitions
unused_definition(DefRef) :-
    symbol_def(_, DefRef, _),
    not resolves_to(_, DefRef),
    element_visibility(DefRef, /private).
```

## Memory Management

### The CGO Problem

Tree-sitter uses C code via CGO. Memory allocated by C must be explicitly freed.

### Safe Patterns

```go
// ALWAYS use defer for tree cleanup
func (p *Parser) Parse(content []byte) ([]CodeElement, error) {
    tree, err := p.parser.ParseCtx(context.Background(), nil, content)
    if err != nil {
        return nil, err
    }
    defer tree.Close()  // CRITICAL: Always close trees

    // Process tree...
    return elements, nil
}

// For long-lived trees, use explicit lifecycle management
type TreeCache struct {
    trees  map[string]*sitter.Tree
    mu     sync.RWMutex
    closed bool
}

func (c *TreeCache) Store(path string, tree *sitter.Tree) {
    c.mu.Lock()
    defer c.mu.Unlock()

    if c.closed {
        tree.Close()  // Don't leak if cache is closed
        return
    }

    // Close old tree if replacing
    if old, exists := c.trees[path]; exists {
        old.Close()
    }
    c.trees[path] = tree
}

func (c *TreeCache) Close() {
    c.mu.Lock()
    defer c.mu.Unlock()

    c.closed = true
    for _, tree := range c.trees {
        tree.Close()
    }
    c.trees = nil
}

// Use finalizer as safety net (not primary cleanup)
func NewParser() *Parser {
    p := &Parser{/*...*/}
    runtime.SetFinalizer(p, func(p *Parser) {
        p.Close()
    })
    return p
}
```

### Memory Leak Detection

```go
// Track allocations in debug mode
var (
    treeAllocs   int64
    treeFrees    int64
    allocMu      sync.Mutex
)

func trackAlloc() {
    allocMu.Lock()
    treeAllocs++
    allocMu.Unlock()
}

func trackFree() {
    allocMu.Lock()
    treeFrees++
    allocMu.Unlock()
}

func CheckLeaks() error {
    allocMu.Lock()
    defer allocMu.Unlock()
    if treeAllocs != treeFrees {
        return fmt.Errorf("tree leak: %d allocated, %d freed", treeAllocs, treeFrees)
    }
    return nil
}
```

## Parallel Parsing

### Why It Matters

Large codebases have thousands of files. Serial parsing is too slow.

### Safe Concurrent Parsing

```go
// IMPORTANT: Tree-sitter parsers are NOT thread-safe
// Each goroutine needs its own parser instance

func ParseFilesParallel(files []string, projectRoot string, lang *sitter.Language) ([]CodeElement, error) {
    type result struct {
        elements []CodeElement
        err      error
    }

    results := make(chan result, len(files))
    sem := make(chan struct{}, runtime.NumCPU())  // Limit concurrency

    var wg sync.WaitGroup
    for _, file := range files {
        wg.Add(1)
        go func(f string) {
            defer wg.Done()
            sem <- struct{}{}        // Acquire
            defer func() { <-sem }() // Release

            // Each goroutine creates its own parser
            parser := sitter.NewParser()
            parser.SetLanguage(lang)
            defer parser.Close()

            content, err := os.ReadFile(f)
            if err != nil {
                results <- result{err: err}
                return
            }

            tree, err := parser.ParseCtx(context.Background(), nil, content)
            if err != nil {
                results <- result{err: err}
                return
            }
            defer tree.Close()

            elements := extractElements(tree, f, projectRoot, content)
            results <- result{elements: elements}
        }(file)
    }

    // Close results channel when done
    go func() {
        wg.Wait()
        close(results)
    }()

    // Collect results
    var allElements []CodeElement
    var firstErr error
    for r := range results {
        if r.err != nil && firstErr == nil {
            firstErr = r.err
        }
        allElements = append(allElements, r.elements...)
    }

    return allElements, firstErr
}
```

### Parser Pool Pattern

```go
type ParserPool struct {
    pool     chan *sitter.Parser
    language *sitter.Language
}

func NewParserPool(lang *sitter.Language, size int) *ParserPool {
    pool := &ParserPool{
        pool:     make(chan *sitter.Parser, size),
        language: lang,
    }

    // Pre-allocate parsers
    for i := 0; i < size; i++ {
        p := sitter.NewParser()
        p.SetLanguage(lang)
        pool.pool <- p
    }

    return pool
}

func (p *ParserPool) Acquire() *sitter.Parser {
    select {
    case parser := <-p.pool:
        return parser
    default:
        // Pool exhausted, create new parser
        parser := sitter.NewParser()
        parser.SetLanguage(p.language)
        return parser
    }
}

func (p *ParserPool) Release(parser *sitter.Parser) {
    select {
    case p.pool <- parser:
        // Returned to pool
    default:
        // Pool full, close parser
        parser.Close()
    }
}

func (p *ParserPool) Close() {
    close(p.pool)
    for parser := range p.pool {
        parser.Close()
    }
}
```

## Import/Dependency Graph

### Building the Graph

```go
type DependencyGraph struct {
    nodes    map[string]bool                    // file -> exists
    edges    map[string]map[string]bool         // file -> imports
    reverse  map[string]map[string]bool         // file -> imported by
}

func BuildDependencyGraph(files []string, parser *CodeElementParser) (*DependencyGraph, error) {
    g := &DependencyGraph{
        nodes:   make(map[string]bool),
        edges:   make(map[string]map[string]bool),
        reverse: make(map[string]map[string]bool),
    }

    for _, file := range files {
        g.nodes[file] = true
        g.edges[file] = make(map[string]bool)
    }

    for _, file := range files {
        imports, err := parser.ExtractImports(file)
        if err != nil {
            continue  // Skip files with errors
        }

        for _, imp := range imports {
            resolved := resolveImport(imp, file)
            if resolved != "" && g.nodes[resolved] {
                g.edges[file][resolved] = true

                if g.reverse[resolved] == nil {
                    g.reverse[resolved] = make(map[string]bool)
                }
                g.reverse[resolved][file] = true
            }
        }
    }

    return g, nil
}

// Get files that depend on a given file (1-hop)
func (g *DependencyGraph) Dependents(file string) []string {
    var deps []string
    for dep := range g.reverse[file] {
        deps = append(deps, dep)
    }
    return deps
}

// Get transitive closure of dependents
func (g *DependencyGraph) TransitiveDependents(file string) []string {
    visited := make(map[string]bool)
    var result []string

    var visit func(f string)
    visit = func(f string) {
        for dep := range g.reverse[f] {
            if !visited[dep] {
                visited[dep] = true
                result = append(result, dep)
                visit(dep)
            }
        }
    }

    visit(file)
    return result
}
```

### Mangle Integration

```mangle
# =============================================================================
# DEPENDENCY GRAPH FACTS
# =============================================================================

Decl file_imports(From.Type<string>, To.Type<string>).
Decl import_transitive(From.Type<string>, To.Type<string>).

# Transitive closure
import_transitive(A, B) :- file_imports(A, B).
import_transitive(A, C) :- file_imports(A, B), import_transitive(B, C).

# Circular dependency detection
circular_dependency(A, B) :-
    import_transitive(A, B),
    import_transitive(B, A).

# Impact analysis: what files need re-testing if file changes
needs_retest(Changed, File) :-
    file_imports(File, Changed).

needs_retest(Changed, File) :-
    needs_retest(Changed, Mid),
    file_imports(File, Mid).

# Aggregate impact count
change_impact_files(Changed, Files) :-
    code_file(Changed),
    needs_retest(Changed, File) |>
    do fn:group_by(Changed),
    let Files = fn:collect(File).
```

## Performance Optimization

### File Change Detection

```go
// Only re-parse files that actually changed
type ChangeDetector struct {
    hashes map[string]string  // file -> content hash
    mu     sync.RWMutex
}

func (d *ChangeDetector) HasChanged(path string, content []byte) bool {
    newHash := sha256Hash(content)

    d.mu.RLock()
    oldHash, exists := d.hashes[path]
    d.mu.RUnlock()

    if !exists || oldHash != newHash {
        d.mu.Lock()
        d.hashes[path] = newHash
        d.mu.Unlock()
        return true
    }
    return false
}

func sha256Hash(data []byte) string {
    h := sha256.Sum256(data)
    return hex.EncodeToString(h[:])
}
```

### Lazy Element Body Loading

```go
// Don't load full body until needed
type LazyCodeElement struct {
    CodeElement
    bodyLoader func() string
    bodyLoaded bool
}

func (e *LazyCodeElement) GetBody() string {
    if !e.bodyLoaded {
        e.Body = e.bodyLoader()
        e.bodyLoaded = true
    }
    return e.Body
}

// Factory that creates lazy elements
func (p *Parser) parseElementLazy(node *sitter.Node, path string, content []byte) *LazyCodeElement {
    startByte := node.StartByte()
    endByte := node.EndByte()

    return &LazyCodeElement{
        CodeElement: CodeElement{
            // ... other fields
            Body: "",  // Don't load yet
        },
        bodyLoader: func() string {
            return string(content[startByte:endByte])
        },
    }
}
```

### Caching Parsed Results

```go
type ParseCache struct {
    cache    *lru.Cache  // github.com/hashicorp/golang-lru
    maxAge   time.Duration
}

type CacheEntry struct {
    Elements  []CodeElement
    ParsedAt  time.Time
    Hash      string
}

func (c *ParseCache) Get(path string, content []byte) ([]CodeElement, bool) {
    hash := sha256Hash(content)

    if entry, ok := c.cache.Get(path); ok {
        ce := entry.(*CacheEntry)
        if ce.Hash == hash && time.Since(ce.ParsedAt) < c.maxAge {
            return ce.Elements, true
        }
    }
    return nil, false
}

func (c *ParseCache) Set(path string, content []byte, elements []CodeElement) {
    c.cache.Add(path, &CacheEntry{
        Elements: elements,
        ParsedAt: time.Now(),
        Hash:     sha256Hash(content),
    })
}
```

## Migration Path: Go-Only to Polyglot

### Phase 1: Interface Extraction

```go
// 1. Extract interface from existing GoParser
type CodeParser interface {
    Parse(path string, content []byte) ([]CodeElement, error)
    SupportedExtensions() []string
    EmitLanguageFacts([]CodeElement) []MangleFact
}

// 2. Make GoParser implement interface (no behavior change)
var _ CodeParser = (*GoParser)(nil)
```

### Phase 2: Add Parser Registry

```go
// 3. Create registry
type ParserRegistry struct {
    parsers map[string]CodeParser
}

func (r *ParserRegistry) Register(p CodeParser) {
    for _, ext := range p.SupportedExtensions() {
        r.parsers[ext] = p
    }
}

// 4. Migrate CodeElementParser to use registry
func (p *CodeElementParser) ParseFile(path string) ([]CodeElement, error) {
    ext := filepath.Ext(path)
    parser := p.registry.Get(ext)
    if parser == nil {
        return nil, ErrNoParser
    }
    content, _ := os.ReadFile(path)
    return parser.Parse(path, content)
}
```

### Phase 3: Add New Parsers

```go
// 5. Add Python parser
p.registry.Register(NewPythonParser(projectRoot))

// 6. Feature flag for gradual rollout
if config.EnablePolyglotParsing {
    p.registry.Register(NewTypeScriptParser(projectRoot))
    p.registry.Register(NewRustParser(projectRoot))
}
```

### Phase 4: Mangle Schema Evolution

```mangle
# 7. Add new predicates without breaking existing queries
Decl py_class(Ref.Type<string>).
Decl ts_interface(Ref.Type<string>).

# 8. Bridge rules work automatically once facts are asserted
is_data_contract(Ref) :- go_struct(Ref).    # Existing
is_data_contract(Ref) :- py_class(Ref).     # New
is_data_contract(Ref) :- ts_interface(Ref). # New
```

## Troubleshooting

### Common Issues

| Issue | Cause | Solution |
|-------|-------|----------|
| Memory growth | Trees not closed | Use `defer tree.Close()` |
| Slow parsing | Serial processing | Use parallel parsing with pool |
| Missing elements | ERROR nodes skipped | Check for syntax errors first |
| Wrong line numbers | 0-indexed vs 1-indexed | Tree-sitter is 0-indexed, add 1 |
| Concurrent panic | Parser reuse across goroutines | One parser per goroutine |
| Stale elements | No incremental update | Implement change detection |

### Debug Logging

```go
func (p *Parser) Parse(path string, content []byte) ([]CodeElement, error) {
    start := time.Now()
    defer func() {
        log.Debug("parsed file",
            "path", path,
            "bytes", len(content),
            "duration", time.Since(start),
        )
    }()

    // ... parsing logic
}
```
