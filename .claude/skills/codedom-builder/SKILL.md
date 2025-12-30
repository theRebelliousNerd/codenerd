---
name: codedom-builder
description: Build and extend the polyglot Code DOM system for surgical code editing. This skill should be used when implementing language parsers (Python, TypeScript, Rust, Kotlin), designing Mangle schemas for code analysis, creating semantic bridge rules, implementing language-specific safety guardrails, or working with Tree-sitter AST parsing. Covers the stratified bridge pattern, cross-language inference via wire names, and advanced code analysis patterns.
---

# Code DOM Builder: Polyglot Surgical Code Editing

The **Code DOM (Document Object Model)** is codeNERD's semantic code editing infrastructure. It treats source code not as flat text but as a structured, interactive tree of semantic elements (functions, structs, methods, interfaces). This enables "surgical" editing - modifying specific code elements by stable reference rather than line numbers.

## Quick Reference

| Component | Location | Purpose |
|-----------|----------|---------|
| CodeElement | `internal/world/code_elements.go` | Core element struct with Ref, Body, Type |
| CodeParser interface | `internal/world/parser_interface.go` | Polyglot parser contract |
| FileScope | `internal/world/scope.go` | 1-hop dependency management |
| VirtualStore handlers | `internal/core/virtual_store_codedom.go` | Action routing |
| Tool definitions | `internal/tools/codedom/` | get_elements, edit_lines, etc. |
| Mangle schema | `internal/core/defaults/schemas_codedom.mg` | 30+ predicates |

## The Core Problem: Read-Write Asymmetry

**Current State:** The system can READ multiple languages via `internal/world/ast.go` (using Tree-sitter/regex), but can only EDIT Go surgically because `CodeElementParser` uses `go/ast` directly.

**Target State:** Any language with a Tree-sitter grammar should support the same surgical editing capabilities as Go.

```text
User Input: "Rename user_id to sub_id across backend and frontend"
                    |
    +--------------+---------------+
    |              |               |
 Go Parser    Python Parser   TS Parser
 (go/ast)    (Tree-sitter)  (Tree-sitter)
    |              |               |
    +--------------+---------------+
                   |
         Unified CodeElement[]
                   |
              ToFacts() ─────────────> Mangle Kernel
                   |                        |
         code_element(Ref, Type, ...)       |
                   |                        |
    <── Stratified Bridge ─────────────────+
                   |
         is_data_contract(Ref)
         api_dependency(Backend, Frontend)
```

## Architecture Overview

### The CodeElement: Atomic Unit of Code

```go
type CodeElement struct {
    Ref       string       // Stable ID: "fn:context.Compressor.Compress"
    Type      ElementType  // /function, /method, /struct, /interface
    File      string       // Source file path
    StartLine int          // 1-indexed inclusive
    EndLine   int          // 1-indexed inclusive
    Signature string       // Declaration line
    Body      string       // Full source text
    Parent    string       // Containing element's Ref
    Visibility Visibility  // /public or /private
    Actions   []ActionType // /view, /replace, /insert_before, etc.
}
```

### The CodeParser Interface (Polyglot Contract)

```go
// internal/world/parser_interface.go
type CodeParser interface {
    // Parse extracts CodeElements from source content
    Parse(path string, content []byte) ([]CodeElement, error)

    // SupportedExtensions returns file types (e.g., [".py", ".pyw"])
    SupportedExtensions() []string
}
```

### Repo-Anchored Reference URIs

Stable references must be physically anchored to the repository root, not language runtime:

```text
Format: lang:path/from/root:Context.Symbol

Examples:
  go:backend/auth/user.go:User.Login
  py:backend/auth/user.py:User.login
  ts:frontend/types.ts:IUser.userId
  rs:core/lib.rs:Config.load
  kt:mobile/models/User.kt:User.userId
```

## The Stratified Bridge Pattern

The key architectural insight: emit language-specific facts (Stratum 0), then normalize via Mangle rules (Stratum 1).

### Stratum 0: Language-Specific Facts (EDB)

```mangle
# Python
Decl py_class(Ref.Type<string>).
Decl py_decorator(Ref.Type<string>, Name.Type<string>).
Decl py_async_def(Ref.Type<string>).

# Go
Decl go_struct(Ref.Type<string>).
Decl go_tag(Ref.Type<string>, Content.Type<string>).
Decl go_goroutine(Ref.Type<string>).

# TypeScript
Decl ts_interface(Ref.Type<string>).
Decl ts_component(Ref.Type<string>, TagName.Type<string>).
Decl ts_hook(Ref.Type<string>, HookName.Type<string>).

# Rust
Decl rs_struct(Ref.Type<string>).
Decl rs_impl(Ref.Type<string>, Trait.Type<string>).
Decl rs_unsafe_block(Ref.Type<string>).

# Kotlin
Decl kt_data_class(Ref.Type<string>).
Decl kt_suspend_fun(Ref.Type<string>).
Decl kt_annotation(Ref.Type<string>, Name.Type<string>).
```

### Stratum 1: Semantic Bridge Rules (IDB)

```mangle
# The "Data Contract" Archetype
is_data_contract(Ref) :- go_struct(Ref).
is_data_contract(Ref) :- py_class(Ref), has_pydantic_base(Ref).
is_data_contract(Ref) :- ts_interface(Ref).
is_data_contract(Ref) :- rs_struct(Ref).
is_data_contract(Ref) :- kt_data_class(Ref).

# The "Async Context" Archetype
is_async_context(Ref) :- go_goroutine(Ref).
is_async_context(Ref) :- py_async_def(Ref).
is_async_context(Ref) :- rs_async_fn(Ref).
is_async_context(Ref) :- kt_suspend_fun(Ref).

# Cross-Language Wire Protocol
wire_name(Ref, Name) :-
    go_tag(Ref, TagContent),
    fn:match("json:\"([a-zA-Z_]+)\"", TagContent, Name).

wire_name(Ref, Name) :- py_field_alias(Ref, Name).
wire_name(Ref, Name) :- kt_annotation(Ref, "SerializedName", Name).
wire_name(Ref, Name) :- ts_interface_prop(Ref, Name).

# API Dependency Inference
api_dependency(BackendRef, FrontendRef) :-
    wire_name(BackendRef, Key),
    wire_name(FrontendRef, Key).
```

## When to Use This Skill

| Task | Use This Skill |
|------|----------------|
| Implementing a new language parser | Yes - follow Tree-sitter patterns |
| Adding language-specific facts | Yes - extend Stratum 0 schema |
| Creating semantic bridge rules | Yes - extend Stratum 1 IDB |
| Implementing safety guardrails | Yes - see safety patterns |
| Understanding CodeElement structure | Yes - architecture reference |
| Debugging parser issues | Yes - Tree-sitter troubleshooting |
| Cross-language refactoring | Yes - wire name inference |

## Reference Documentation

| Reference | Contents |
|-----------|----------|
| [000-ARCHITECTURE](references/000-ARCHITECTURE.md) | Full system architecture, data flow |
| [100-PARSER-IMPLEMENTATION](references/100-PARSER-IMPLEMENTATION.md) | Tree-sitter parser guide |
| [200-MANGLE-INTEGRATION](references/200-MANGLE-INTEGRATION.md) | Schema design, bridge patterns |
| [300-SAFETY-GUARDRAILS](references/300-SAFETY-GUARDRAILS.md) | Language-specific safety rules |
| [400-ADVANCED-PATTERNS](references/400-ADVANCED-PATTERNS.md) | AST analysis, taint tracking |
| [500-PRACTICAL-CONCERNS](references/500-PRACTICAL-CONCERNS.md) | Error recovery, incremental parsing, performance |
| [600-TRANSACTION-SAFETY](references/600-TRANSACTION-SAFETY.md) | 2PC protocol, shadow mode, test impact analysis |
| [700-JIT-PROMPT-INTEGRATION](references/700-JIT-PROMPT-INTEGRATION.md) | Prompt atoms for semantic mode paradigm shift |

## Critical Mangle Patterns for Code DOM

### CRITICAL: Atom vs String

```mangle
# WRONG - string literal
code_element(Ref, "function", File, Start, End).

# CORRECT - atom constant
code_element(Ref, /function, File, Start, End).
```

### Struct Field Access

```mangle
# WRONG - dot notation
bad(Name) :- element(E), E.name = Name.

# CORRECT - match_field
good(Name) :- element(E), :match_field(E, /name, Name).
```

### Safety in Negation

```mangle
# WRONG - unbound variable
unsafe(Ref) :- not has_tests(Ref).

# CORRECT - bind first
safe_to_edit(Ref) :- code_element(Ref, _, _, _, _), not has_tests(Ref).
```

## Universal Refactor Workflow

```text
1. User Intent: "Rename user_id to sub_id"
       |
2. Mangle Impact Query:
   - Find go:User.UserID
   - Propagate via api_dependency to ts:IUser.userId, kt:User.userId
       |
3. Plan Aggregation:
   {
     "task": "rename",
     "new_name": "sub_id",
     "targets": [
       {"ref": "go:backend/user.go:User.UserID", "driver": "go_ast"},
       {"ref": "ts:frontend/types.ts:IUser.userId", "driver": "ts_parser"},
       {"ref": "kt:mobile/models/User.kt:User.userId", "driver": "kt_parser"}
     ]
   }
       |
4. Generation: Create AST patches per driver
       |
5. Validation: Run deny_edit rules, check for regressions
       |
6. Atomic Commit: Apply all changes in single transaction
```

## Implementation Checklist

When implementing a new language parser:

- [ ] Create `internal/world/{lang}_parser.go`
- [ ] Implement `CodeParser` interface
- [ ] Use Tree-sitter for AST parsing (not regex)
- [ ] Generate repo-anchored Ref URIs
- [ ] Map language constructs to standard ElementTypes
- [ ] Emit language-specific Stratum 0 facts
- [ ] Add bridge rules to Stratum 1
- [ ] Implement visibility detection
- [ ] Add safety guardrails for language-specific pitfalls
- [ ] Register parser in factory
- [ ] Add tests with real-world code samples

## Key Files Quick Reference

```text
Parser Layer:
  internal/world/code_elements.go    # CodeElement struct
  internal/world/parser_interface.go # CodeParser interface (NEW)
  internal/world/go_parser.go        # Go implementation
  internal/world/python_parser.go    # Python implementation (NEW)
  internal/world/scope.go            # FileScope management

VirtualStore Layer:
  internal/core/virtual_store_codedom.go  # Action handlers
  internal/core/virtual_store_types.go    # ActionType constants

Tools Layer:
  internal/tools/codedom/register.go   # Tool registration
  internal/tools/codedom/elements.go   # get_elements, get_element
  internal/tools/codedom/lines.go      # edit_lines, insert_lines

Mangle Layer:
  internal/core/defaults/schemas_codedom.mg  # Predicates
  internal/core/defaults/bridge.mg           # Bridge rules (NEW)
  internal/core/defaults/safety.mg           # Safety guardrails (NEW)
```

## Common Pitfalls

| Pitfall | Solution |
|---------|----------|
| Regex parsing for Python | Use Tree-sitter - regex fails on decorators, multiline |
| Hardcoded `go/ast` imports | Abstract behind CodeParser interface |
| Unstable Refs in dynamic languages | Anchor to repo root, not runtime |
| Missing wire name extraction | Implement for json tags, annotations |
| Ignoring visibility | Map `_` prefix to /private in Python |
| Safety rules not triggering | Check atom vs string in facts |

## Critical Implementation Concerns

### The Split Brain Problem

Multi-file refactoring creates a new failure mode: if some edits succeed and others fail, the codebase is left inconsistent.

#### Solution: Two-Phase Commit (2PC)

1. **Prepare Phase:** Apply all edits to shadow filesystem, validate with Mangle
2. **Commit Phase:** Atomically flush to real filesystem, or abort entirely

See [600-TRANSACTION-SAFETY](references/600-TRANSACTION-SAFETY.md) for implementation.

### Test Impact Analysis

Running `go test ./...` after every edit is expensive. Use the dependency graph to select only impacted tests:

```mangle
impacted_test(TestRef) :-
    plan_edit(TargetRef),
    test_depends_on_transitive(TestRef, TargetRef).
```

### Incremental Graph Maintenance

After edits, refresh only affected files instead of re-parsing the entire codebase:

```go
// Atomic single-file refresh
func (p *Parser) Refresh(ctx context.Context, file string) error {
    p.kernel.RetractByFile(file)
    elements := p.ParseFile(file)
    p.kernel.AssertFacts(p.EmitFacts(elements))
}
```

### Prompt Blindness Prevention

The LLM needs a paradigm shift from "Text Editor" to "Graph Architect". Create JIT prompt atoms that teach this mental model:

1. **Core Concept Atom:** Teach that Refs are the primary interface, not filenames
2. **Impact Simulation Atom:** Query Mangle before editing, not after
3. **Tool Steering Atom:** Forbid grep/sed in favor of semantic tools
4. **Exemplar Atom:** Concrete example of polyglot refactor workflow

See [700-JIT-PROMPT-INTEGRATION](references/700-JIT-PROMPT-INTEGRATION.md) for complete atom templates.
