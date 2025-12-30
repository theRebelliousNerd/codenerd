# 100: Parser Implementation Guide

## The CodeParser Interface

All language parsers must implement this interface:

```go
// internal/world/parser_interface.go
package world

// CodeParser defines the contract for language-specific parsers.
type CodeParser interface {
    // Parse extracts CodeElements from the given file content.
    Parse(path string, content []byte) ([]CodeElement, error)

    // SupportedExtensions returns file extensions this parser handles.
    SupportedExtensions() []string

    // EmitLanguageFacts returns language-specific Mangle facts.
    // These become Stratum 0 facts (e.g., py_class, go_struct).
    EmitLanguageFacts(elements []CodeElement) []MangleFact
}
```

## Parser Factory Pattern

The CodeElementParser acts as a factory/router:

```go
// internal/world/code_elements.go
type CodeElementParser struct {
    parsers      map[string]CodeParser // Extension -> parser
    projectRoot  string                // For repo-anchored refs
}

func NewCodeElementParser(projectRoot string) *CodeElementParser {
    p := &CodeElementParser{
        parsers:     make(map[string]CodeParser),
        projectRoot: projectRoot,
    }

    // Register parsers
    p.Register(NewGoParser())
    p.Register(NewPythonParser())
    p.Register(NewTypeScriptParser())
    p.Register(NewRustParser())
    p.Register(NewKotlinParser())
    p.Register(NewMangleParser())

    return p
}

func (p *CodeElementParser) Register(parser CodeParser) {
    for _, ext := range parser.SupportedExtensions() {
        p.parsers[ext] = parser
    }
}

func (p *CodeElementParser) ParseFile(path string, content []byte) ([]CodeElement, error) {
    ext := filepath.Ext(path)
    parser, ok := p.parsers[ext]
    if !ok {
        return nil, fmt.Errorf("no parser for extension: %s", ext)
    }
    return parser.Parse(path, content)
}
```

## Tree-sitter Integration

### Dependencies

```go
import (
    sitter "github.com/smacker/go-tree-sitter"
    "github.com/smacker/go-tree-sitter/python"
    "github.com/smacker/go-tree-sitter/typescript/typescript"
    "github.com/smacker/go-tree-sitter/rust"
    "github.com/smacker/go-tree-sitter/kotlin"
)
```

### Parser Lifecycle

```go
type TreeSitterParser struct {
    language *sitter.Language
    parser   *sitter.Parser
}

func NewTreeSitterParser(lang *sitter.Language) *TreeSitterParser {
    parser := sitter.NewParser()
    parser.SetLanguage(lang)
    return &TreeSitterParser{
        language: lang,
        parser:   parser,
    }
}

func (p *TreeSitterParser) Parse(content []byte) (*sitter.Tree, error) {
    tree, err := p.parser.ParseCtx(context.Background(), nil, content)
    if err != nil {
        return nil, fmt.Errorf("tree-sitter parse failed: %w", err)
    }
    return tree, nil
}

// IMPORTANT: Always close the tree when done
func (p *TreeSitterParser) Close(tree *sitter.Tree) {
    tree.Close()
}
```

## Python Parser Implementation

```go
// internal/world/python_parser.go
package world

import (
    "context"
    "fmt"
    "path/filepath"
    "strings"

    sitter "github.com/smacker/go-tree-sitter"
    "github.com/smacker/go-tree-sitter/python"
)

type PythonParser struct {
    parser      *sitter.Parser
    projectRoot string
}

func NewPythonParser(projectRoot string) *PythonParser {
    parser := sitter.NewParser()
    parser.SetLanguage(python.GetLanguage())
    return &PythonParser{
        parser:      parser,
        projectRoot: projectRoot,
    }
}

func (p *PythonParser) SupportedExtensions() []string {
    return []string{".py", ".pyw"}
}

func (p *PythonParser) Parse(path string, content []byte) ([]CodeElement, error) {
    tree, err := p.parser.ParseCtx(context.Background(), nil, content)
    if err != nil {
        return nil, fmt.Errorf("tree-sitter parse failed: %w", err)
    }
    defer tree.Close()

    var elements []CodeElement
    root := tree.RootNode()

    p.walkNode(root, path, "", &elements, content)

    return elements, nil
}

func (p *PythonParser) walkNode(node *sitter.Node, path, parentRef string, elements *[]CodeElement, content []byte) {
    for i := 0; i < int(node.ChildCount()); i++ {
        child := node.Child(i)
        elem := p.nodeToElement(child, path, parentRef, content)

        if elem != nil {
            *elements = append(*elements, *elem)
            // Recurse into classes to find methods
            if elem.Type == ElementStruct {
                p.walkNode(child, path, elem.Ref, elements, content)
            }
        }
    }
}

func (p *PythonParser) nodeToElement(node *sitter.Node, path, parentRef string, content []byte) *CodeElement {
    nodeType := node.Type()

    var elemType ElementType
    var name string

    switch nodeType {
    case "function_definition":
        if parentRef != "" {
            elemType = ElementMethod
        } else {
            elemType = ElementFunction
        }
        name = p.childContent(node, "name", content)

    case "class_definition":
        elemType = ElementStruct // Map Python class to struct
        name = p.childContent(node, "name", content)

    case "decorated_definition":
        // Handle decorated functions/classes
        for i := 0; i < int(node.ChildCount()); i++ {
            child := node.Child(i)
            if child.Type() == "function_definition" || child.Type() == "class_definition" {
                elem := p.nodeToElement(child, path, parentRef, content)
                if elem != nil {
                    // Extend start line to include decorators
                    elem.StartLine = int(node.StartPoint().Row) + 1
                    return elem
                }
            }
        }
        return nil

    default:
        return nil
    }

    if name == "" {
        return nil
    }

    // Build repo-anchored Ref
    relPath := p.relativePath(path)
    ref := fmt.Sprintf("py:%s:%s", relPath, name)
    if parentRef != "" {
        // Extract parent name from ref
        parts := strings.Split(parentRef, ":")
        parentName := parts[len(parts)-1]
        ref = fmt.Sprintf("py:%s:%s.%s", relPath, parentName, name)
    }

    startLine := int(node.StartPoint().Row) + 1
    endLine := int(node.EndPoint().Row) + 1

    // Extract signature (first line)
    signature := strings.Split(string(content[node.StartByte():node.EndByte()]), "\n")[0]

    return &CodeElement{
        Ref:        ref,
        Type:       elemType,
        Name:       name,
        File:       path,
        StartLine:  startLine,
        EndLine:    endLine,
        Signature:  strings.TrimSpace(signature),
        Parent:     parentRef,
        Visibility: p.determineVisibility(name),
        Actions:    []ActionType{ActionView, ActionReplace, ActionDelete},
    }
}

func (p *PythonParser) determineVisibility(name string) Visibility {
    if strings.HasPrefix(name, "_") {
        return VisibilityPrivate
    }
    return VisibilityPublic
}

func (p *PythonParser) childContent(node *sitter.Node, fieldName string, content []byte) string {
    child := node.ChildByFieldName(fieldName)
    if child == nil {
        return ""
    }
    return string(content[child.StartByte():child.EndByte()])
}

func (p *PythonParser) relativePath(path string) string {
    rel, err := filepath.Rel(p.projectRoot, path)
    if err != nil {
        return path
    }
    return filepath.ToSlash(rel)
}

// EmitLanguageFacts generates Python-specific Mangle facts
func (p *PythonParser) EmitLanguageFacts(elements []CodeElement) []MangleFact {
    var facts []MangleFact

    for _, elem := range elements {
        switch elem.Type {
        case ElementStruct:
            facts = append(facts, MangleFact{
                Predicate: "py_class",
                Args:      []interface{}{elem.Ref},
            })
        case ElementFunction, ElementMethod:
            if strings.Contains(elem.Signature, "async def") {
                facts = append(facts, MangleFact{
                    Predicate: "py_async_def",
                    Args:      []interface{}{elem.Ref},
                })
            }
        }
    }

    return facts
}
```

## TypeScript Parser Implementation

```go
// internal/world/typescript_parser.go
package world

import (
    sitter "github.com/smacker/go-tree-sitter"
    "github.com/smacker/go-tree-sitter/typescript/typescript"
)

type TypeScriptParser struct {
    parser      *sitter.Parser
    projectRoot string
}

func NewTypeScriptParser(projectRoot string) *TypeScriptParser {
    parser := sitter.NewParser()
    parser.SetLanguage(typescript.GetLanguage())
    return &TypeScriptParser{
        parser:      parser,
        projectRoot: projectRoot,
    }
}

func (p *TypeScriptParser) SupportedExtensions() []string {
    return []string{".ts", ".tsx"}
}

func (p *TypeScriptParser) Parse(path string, content []byte) ([]CodeElement, error) {
    tree, err := p.parser.ParseCtx(context.Background(), nil, content)
    if err != nil {
        return nil, err
    }
    defer tree.Close()

    var elements []CodeElement
    root := tree.RootNode()

    p.walkNode(root, path, "", &elements, content)
    return elements, nil
}

func (p *TypeScriptParser) walkNode(node *sitter.Node, path, parentRef string, elements *[]CodeElement, content []byte) {
    for i := 0; i < int(node.ChildCount()); i++ {
        child := node.Child(i)

        switch child.Type() {
        case "function_declaration":
            elem := p.parseFunctionDecl(child, path, parentRef, content)
            if elem != nil {
                *elements = append(*elements, *elem)
            }

        case "class_declaration":
            elem := p.parseClassDecl(child, path, content)
            if elem != nil {
                *elements = append(*elements, *elem)
                // Recurse for methods
                p.walkNode(child, path, elem.Ref, elements, content)
            }

        case "interface_declaration":
            elem := p.parseInterfaceDecl(child, path, content)
            if elem != nil {
                *elements = append(*elements, *elem)
            }

        case "method_definition":
            elem := p.parseMethodDef(child, path, parentRef, content)
            if elem != nil {
                *elements = append(*elements, *elem)
            }

        case "arrow_function":
            // Handle exported arrow functions
            if p.isExportedArrow(child) {
                elem := p.parseArrowFunction(child, path, content)
                if elem != nil {
                    *elements = append(*elements, *elem)
                }
            }

        default:
            // Recurse into other nodes
            p.walkNode(child, path, parentRef, elements, content)
        }
    }
}

// Implementation details for each node type...
```

## Rust Parser Implementation

```go
// internal/world/rust_parser.go
package world

import (
    sitter "github.com/smacker/go-tree-sitter"
    "github.com/smacker/go-tree-sitter/rust"
)

type RustParser struct {
    parser      *sitter.Parser
    projectRoot string
}

func NewRustParser(projectRoot string) *RustParser {
    parser := sitter.NewParser()
    parser.SetLanguage(rust.GetLanguage())
    return &RustParser{
        parser:      parser,
        projectRoot: projectRoot,
    }
}

func (p *RustParser) SupportedExtensions() []string {
    return []string{".rs"}
}

// Key node types for Rust:
// - function_item
// - struct_item
// - impl_item
// - trait_item
// - enum_item
// - const_item
// - static_item
// - macro_definition

func (p *RustParser) nodeToElement(node *sitter.Node, path, parentRef string, content []byte) *CodeElement {
    switch node.Type() {
    case "function_item":
        return p.parseFunctionItem(node, path, parentRef, content)

    case "struct_item":
        return p.parseStructItem(node, path, content)

    case "impl_item":
        // impl blocks create a context for methods
        return p.parseImplItem(node, path, content)

    case "trait_item":
        return p.parseTraitItem(node, path, content)

    default:
        return nil
    }
}

// Visibility in Rust:
// - pub         -> /public
// - pub(crate)  -> /public (crate-visible)
// - pub(super)  -> /private (parent-visible)
// - (none)      -> /private

func (p *RustParser) determineVisibility(node *sitter.Node, content []byte) Visibility {
    // Check for visibility modifier
    for i := 0; i < int(node.ChildCount()); i++ {
        child := node.Child(i)
        if child.Type() == "visibility_modifier" {
            vis := string(content[child.StartByte():child.EndByte()])
            if strings.HasPrefix(vis, "pub") {
                return VisibilityPublic
            }
        }
    }
    return VisibilityPrivate
}

// EmitLanguageFacts for Rust
func (p *RustParser) EmitLanguageFacts(elements []CodeElement) []MangleFact {
    var facts []MangleFact

    for _, elem := range elements {
        switch elem.Type {
        case ElementStruct:
            facts = append(facts, MangleFact{
                Predicate: "rs_struct",
                Args:      []interface{}{elem.Ref},
            })

        case ElementFunction:
            // Check for async
            if strings.Contains(elem.Signature, "async fn") {
                facts = append(facts, MangleFact{
                    Predicate: "rs_async_fn",
                    Args:      []interface{}{elem.Ref},
                })
            }
            // Check for unsafe
            if strings.Contains(elem.Signature, "unsafe") {
                facts = append(facts, MangleFact{
                    Predicate: "rs_unsafe_block",
                    Args:      []interface{}{elem.Ref},
                })
            }
        }
    }

    return facts
}
```

## Node Type Reference

### Python

| Node Type | Maps To | Notes |
|-----------|---------|-------|
| `function_definition` | ElementFunction/Method | Check parent for method |
| `class_definition` | ElementStruct | Contains methods |
| `decorated_definition` | (wrapper) | Extends element start line |
| `async_function_definition` | ElementFunction + async fact | |

### TypeScript

| Node Type | Maps To | Notes |
|-----------|---------|-------|
| `function_declaration` | ElementFunction | |
| `class_declaration` | ElementStruct | |
| `interface_declaration` | ElementInterface | |
| `method_definition` | ElementMethod | |
| `arrow_function` | ElementFunction | If exported |
| `type_alias_declaration` | ElementType | |

### Rust

| Node Type | Maps To | Notes |
|-----------|---------|-------|
| `function_item` | ElementFunction/Method | Check if in impl |
| `struct_item` | ElementStruct | |
| `impl_item` | (context) | Contains methods |
| `trait_item` | ElementInterface | |
| `enum_item` | ElementType | |

### Kotlin

| Node Type | Maps To | Notes |
|-----------|---------|-------|
| `function_declaration` | ElementFunction/Method | |
| `class_declaration` | ElementStruct | Check for `data class` |
| `interface_declaration` | ElementInterface | |
| `object_declaration` | ElementStruct | Singleton |

## Testing Parsers

### Test Structure

```go
func TestPythonParser_Parse(t *testing.T) {
    parser := NewPythonParser("/project")

    tests := []struct {
        name     string
        code     string
        expected []CodeElement
    }{
        {
            name: "simple function",
            code: `def hello():
    return "world"`,
            expected: []CodeElement{
                {
                    Ref:       "py:test.py:hello",
                    Type:      ElementFunction,
                    Name:      "hello",
                    StartLine: 1,
                    EndLine:   2,
                },
            },
        },
        {
            name: "class with methods",
            code: `class User:
    def __init__(self, name):
        self.name = name

    def greet(self):
        return f"Hello, {self.name}"`,
            expected: []CodeElement{
                {Ref: "py:test.py:User", Type: ElementStruct},
                {Ref: "py:test.py:User.__init__", Type: ElementMethod, Parent: "py:test.py:User"},
                {Ref: "py:test.py:User.greet", Type: ElementMethod, Parent: "py:test.py:User"},
            },
        },
        {
            name: "decorated function",
            code: `@login_required
@cached(ttl=300)
def get_user(user_id):
    return User.find(user_id)`,
            expected: []CodeElement{
                {
                    Ref:       "py:test.py:get_user",
                    Type:      ElementFunction,
                    StartLine: 1, // Includes decorators
                    EndLine:   5,
                },
            },
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            elements, err := parser.Parse("/project/test.py", []byte(tt.code))
            assert.NoError(t, err)
            assert.Len(t, elements, len(tt.expected))
            // Verify each element...
        })
    }
}
```

### Edge Cases to Test

1. **Empty files** - Should return empty slice, no error
2. **Syntax errors** - Tree-sitter is error-tolerant, should parse what it can
3. **Unicode** - File paths and identifiers with Unicode
4. **Large files** - Performance with 10k+ line files
5. **Nested structures** - Deeply nested classes/functions
6. **Anonymous functions** - Lambdas, closures
7. **Generated code** - Protobuf, OpenAPI stubs
