# Tree-sitter AST Parsing

This package now includes Tree-sitter-based AST parsing for Python, Rust, JavaScript, and TypeScript, replacing the previous regex-based parsing for improved accuracy and completeness.

## Architecture

### Files

- **ast.go** - Main AST parser with language detection and fallback logic
- **ast_treesitter.go** - Tree-sitter implementation for Python, Rust, JS, and TS
- **ast_test.go** - Comprehensive tests for all language parsers

### Dependencies

The following Tree-sitter packages are used:

```
github.com/smacker/go-tree-sitter           # Core Tree-sitter bindings
github.com/smacker/go-tree-sitter/python    # Python grammar
github.com/smacker/go-tree-sitter/rust      # Rust grammar
github.com/smacker/go-tree-sitter/javascript # JavaScript grammar
github.com/smacker/go-tree-sitter/typescript # TypeScript grammar
```

## Features

### Python Parser

Extracts:
- **Classes** with visibility detection (public, protected `_`, private `__`)
- **Functions** with parameter signatures
- **Imports** (both `import` and `from ... import` statements)

Example:
```python
class MyClass:           # → symbol_graph(class:MyClass, class, public, ...)
    def method(self):    # → symbol_graph(func:method, function, public, ...)
    def _protected(self): # → symbol_graph(func:_protected, function, protected, ...)
```

### Rust Parser

Extracts:
- **Functions** with `pub` visibility detection
- **Structs** with visibility
- **Enums** with visibility
- **Modules** with visibility
- **Use statements** (dependencies)

Example:
```rust
pub struct MyStruct {}   # → symbol_graph(struct:MyStruct, struct, public, ...)
fn private_fn() {}       # → symbol_graph(fn:private_fn, function, private, ...)
use std::io;            # → dependency_link(..., crate:std, ...)
```

### TypeScript Parser

Extracts:
- **Classes** with export detection
- **Interfaces** with export detection
- **Type aliases** with export detection
- **Functions** with parameter and return type signatures
- **Arrow functions** (const declarations)
- **Imports**

Example:
```typescript
export interface I {}    # → symbol_graph(interface:I, interface, public, ...)
export class C {}        # → symbol_graph(class:C, class, public, ...)
class Private {}         # → symbol_graph(class:Private, class, private, ...)
```

### JavaScript Parser

Extracts:
- **Classes** with export detection
- **Functions** with parameters
- **Arrow functions** (const declarations)
- **Imports**

Example:
```javascript
export class MyClass {}  # → symbol_graph(class:MyClass, class, public, ...)
const func = () => {}    # → symbol_graph(func:func, function, private, ...)
```

## Fallback Behavior

Each language parser follows this pattern:

1. **Try Tree-sitter first** - Use precise AST parsing
2. **Fallback to regex** - If Tree-sitter fails or returns no results, fall back to the original regex-based parsing
3. **Return results** - Ensures parsing always succeeds (graceful degradation)

This dual-layer approach provides:
- **High accuracy** when Tree-sitter works
- **Robustness** when files are malformed or Tree-sitter encounters issues
- **Backward compatibility** with existing regex-based parsing

## Integration

The `ASTParser` struct now includes a `TreeSitterParser`:

```go
type ASTParser struct {
    tsParser *TreeSitterParser
}

func NewASTParser() *ASTParser {
    return &ASTParser{
        tsParser: NewTreeSitterParser(),
    }
}

// Always call Close() to release resources
defer parser.Close()
```

## Usage

```go
parser := NewASTParser()
defer parser.Close()

facts, err := parser.Parse("/path/to/file.py")
if err != nil {
    // Handle error
}

// facts contains symbol_graph and dependency_link predicates
for _, fact := range facts {
    fmt.Printf("%s: %v\n", fact.Predicate, fact.Args)
}
```

## Generated Facts

### symbol_graph Predicate

Format: `symbol_graph(id, type, visibility, path, signature)`

- **id**: Unique identifier (e.g., `func:myFunction`, `class:MyClass`)
- **type**: Symbol type (`function`, `class`, `struct`, `enum`, `interface`, `type`, `module`)
- **visibility**: Access level (`public`, `protected`, `private`)
- **path**: File path where symbol is defined
- **signature**: Code signature (e.g., `def myFunction(x, y)`, `pub fn calculate()`)

### dependency_link Predicate

Format: `dependency_link(source, target, import_path)`

- **source**: File doing the importing
- **target**: Identifier for the imported module (e.g., `mod:os`, `crate:std`)
- **import_path**: Full import path

## Known Limitations

### Tree-sitter CGO Warning (Windows)

You may see this warning on Windows:

```
warning: redeclaration of 'callReadFunc' should not add 'dllexport' attribute
```

This is a harmless warning from the Tree-sitter CGO bindings and does not affect functionality.

### Visibility Detection

- **Python**: Based on naming conventions (`_` prefix = protected, `__` prefix = private)
- **Rust**: Based on `pub` keyword (default is private)
- **TypeScript/JavaScript**: Based on `export` keyword (default is private)

### Method Detection

Currently, methods are captured as functions. Future enhancement could link them to their parent class/struct via additional predicates.

## Testing

Run tests with:

```bash
cd internal/world
go test -v -run TestASTParser
```

Tests create temporary files for each language and verify:
- Symbol extraction (classes, functions, structs, etc.)
- Import/dependency extraction
- Visibility detection
- Fact generation

## Performance

Tree-sitter provides significant improvements over regex:

- **Accurate**: Handles nested structures, multiline definitions, complex syntax
- **Fast**: Incremental parsing (though we don't use this feature yet)
- **Robust**: Handles syntax errors gracefully
- **Complete**: Extracts full signatures, not just names

Regex fallback ensures performance remains acceptable even for malformed files.

## Future Enhancements

Potential improvements:

1. **Method-to-class linking**: Create predicates linking methods to their parent classes
2. **Field extraction**: Extract struct/class fields
3. **Call graph**: Extract function call relationships
4. **Incremental parsing**: Cache parse trees for large files
5. **More languages**: Add Go (replace go/parser), C++, Java, etc.
6. **Better error recovery**: Provide diagnostics when parsing fails

## Migration Notes

The new implementation is **backward compatible**:

- Same `Parse(path)` interface
- Same fact predicates (`symbol_graph`, `dependency_link`)
- Same fact structure
- Regex fallback ensures no breaking changes

Existing code using `ASTParser` will automatically benefit from Tree-sitter parsing without any changes.
