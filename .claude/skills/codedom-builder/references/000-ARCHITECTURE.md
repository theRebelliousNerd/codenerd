# 000: Code DOM Architecture

## System Overview

The Code DOM provides a semantic abstraction over source code, enabling agents to reason about and manipulate code elements (functions, structs, methods) rather than raw text lines.

```text
┌─────────────────────────────────────────────────────────────────┐
│                        USER / AGENT                              │
│                "Rename user_id to sub_id"                        │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                     PERCEPTION TRANSDUCER                        │
│              NL → Intent Atoms (rename, /user_id, /sub_id)       │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                      PARSER LAYER                                │
│  ┌──────────┐  ┌───────────────┐  ┌────────────┐  ┌──────────┐  │
│  │ GoParser │  │ PythonParser  │  │  TSParser  │  │ RustParser│  │
│  │ (go/ast) │  │ (Tree-sitter) │  │(Tree-sitter)│  │(Tree-sit) │  │
│  └────┬─────┘  └──────┬────────┘  └─────┬──────┘  └────┬─────┘  │
│       │               │                 │              │         │
│       └───────────────┴────────┬────────┴──────────────┘         │
│                                │                                 │
│                    Unified CodeElement[]                         │
└────────────────────────────────┼────────────────────────────────┘
                                 │
                                 ▼
┌─────────────────────────────────────────────────────────────────┐
│                      MANGLE KERNEL                               │
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │ STRATUM 0 (EDB): Language-Specific Facts                   │ │
│  │   py_class("py:user.py:User")                              │ │
│  │   go_struct("go:user.go:User")                             │ │
│  │   ts_interface("ts:types.ts:IUser")                        │ │
│  └────────────────────────────────────────────────────────────┘ │
│                          │                                       │
│                          ▼                                       │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │ STRATUM 1 (IDB): Semantic Bridge                           │ │
│  │   is_data_contract(Ref) :- go_struct(Ref).                 │ │
│  │   is_data_contract(Ref) :- py_class(Ref).                  │ │
│  │   api_dependency(Backend, Frontend) :- wire_name(B, K),    │ │
│  │                                        wire_name(F, K).    │ │
│  └────────────────────────────────────────────────────────────┘ │
│                          │                                       │
│                          ▼                                       │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │ STRATUM 2 (IDB): Safety Guardrails                         │ │
│  │   deny_edit(Ref, /security_regression) :- ...              │ │
│  │   deny_edit(Ref, /goroutine_leak) :- ...                   │ │
│  └────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
                                 │
                                 ▼
┌─────────────────────────────────────────────────────────────────┐
│                     VIRTUALSTORE                                 │
│       Routes CodeDOM actions to handlers                         │
│   open_file → get_elements → edit_element → refresh_scope       │
└─────────────────────────────────────────────────────────────────┘
                                 │
                                 ▼
┌─────────────────────────────────────────────────────────────────┐
│                    FILE SYSTEM                                   │
│        Actual file modifications via FileEditor                  │
└─────────────────────────────────────────────────────────────────┘
```

## Core Data Structures

### CodeElement

The atomic unit of the Code DOM. Represents a single semantic code entity.

```go
type CodeElement struct {
    // Identity
    Ref       string       // Stable URI: "lang:path:Context.Symbol"
    Type      ElementType  // /function, /method, /struct, /interface, /type, /const, /var
    Name      string       // Simple name without path
    Package   string       // Package/module name

    // Location
    File      string       // Absolute file path
    StartLine int          // 1-indexed, inclusive
    EndLine   int          // 1-indexed, inclusive

    // Content
    Signature string       // Declaration line (first line)
    Body      string       // Full source text (optional)

    // Relationships
    Parent    string       // Ref of containing element (e.g., struct for methods)

    // Metadata
    Visibility Visibility  // /public or /private
    Actions   []ActionType // Available operations

    // Language-specific metadata
    Metadata  map[string]interface{}
}
```

### ElementType Constants

```go
const (
    ElementFunction  ElementType = "/function"
    ElementMethod    ElementType = "/method"
    ElementStruct    ElementType = "/struct"
    ElementInterface ElementType = "/interface"
    ElementType      ElementType = "/type"
    ElementConst     ElementType = "/const"
    ElementVar       ElementType = "/var"
    // Mangle-specific
    ElementDecl      ElementType = "/decl"
    ElementRule      ElementType = "/rule"
    ElementFact      ElementType = "/fact"
    ElementQuery     ElementType = "/query"
)
```

### Visibility

```go
const (
    VisibilityPublic  Visibility = "/public"
    VisibilityPrivate Visibility = "/private"
)
```

### ActionType

```go
const (
    ActionView         ActionType = "/view"
    ActionReplace      ActionType = "/replace"
    ActionInsertBefore ActionType = "/insert_before"
    ActionInsertAfter  ActionType = "/insert_after"
    ActionDelete       ActionType = "/delete"
)
```

## FileScope: The 1-Hop Context

FileScope manages the "working set" of files during editing operations. It includes:
- The **active file** being edited
- All files **imported by** the active file (outbound deps)
- All files that **import** the active file (inbound deps)

```go
type FileScope struct {
    ActiveFile   string                // Primary file being worked on
    InScope      map[string]bool       // All files in scope
    Elements     []CodeElement         // All elements in scope
    OutboundDeps map[string][]string   // file -> files it imports
    InboundDeps  map[string][]string   // file -> files that import it
    FileHashes   map[string]string     // Content hashes for change detection
    Parser       *CodeElementParser    // The parser instance
}
```

### Scope Operations

| Method | Purpose |
|--------|---------|
| `Open(path)` | Load file + 1-hop dependencies |
| `ScopeFacts()` | Generate Mangle facts for all in-scope elements |
| `GetCoreElementsByFile(file)` | Get elements in a specific file |
| `GetCoreElement(ref)` | Get element by stable reference |
| `RefreshWithRetry(n)` | Re-parse with retry on change detection |
| `VerifyFileHash(file)` | Check for concurrent modifications |
| `Close()` | Release the scope |

## Ref URI Format

Stable references anchor elements to the repository, not the runtime:

```text
Format: lang:path/from/root:Context.Symbol

Components:
  lang     - Language prefix (go, py, ts, rs, kt, java, mg)
  path     - File path relative to repo root
  Context  - Nesting context (Class, Struct, Impl)
  Symbol   - Element name

Examples:
  go:internal/auth/user.go:User.Login
  py:backend/models/user.py:User.validate
  py:backend/models/user.py:User.__init__
  ts:frontend/components/UserCard.tsx:UserCard.render
  rs:core/src/lib.rs:Config::load
  kt:mobile/app/User.kt:User.toJson
  mg:internal/mangle/policy.mg:rule:permitted/1#3
```

### Why Repo-Anchored?

| Approach | Problem |
|----------|---------|
| Runtime-based (`module.Class.method`) | Python has no canonical package root |
| Line-based (`file:42`) | Lines shift with edits |
| Hash-based | Changes break identity |
| Repo-anchored | Stable across edits, unique across languages |

## VirtualStore Integration

The VirtualStore routes Code DOM actions through handlers:

```go
// Action constants
const (
    ActionOpenFile      ActionType = "open_file"
    ActionGetElements   ActionType = "get_elements"
    ActionGetElement    ActionType = "get_element"
    ActionEditElement   ActionType = "edit_element"
    ActionRefreshScope  ActionType = "refresh_scope"
    ActionCloseScope    ActionType = "close_scope"
    ActionEditLines     ActionType = "edit_lines"
    ActionInsertLines   ActionType = "insert_lines"
    ActionDeleteLines   ActionType = "delete_lines"
)
```

### Handler Flow

```text
Agent Request: edit_element("go:user.go:User.Login", newBody)
     │
     ▼
VirtualStore.RouteAction(ActionEditElement, params)
     │
     ▼
handleEditElement()
  │
  ├── scope.GetCoreElement(ref)        // Find the element
  ├── scope.VerifyFileHash(file)       // Check for concurrent mods
  ├── editor.ReplaceElement(...)       // Perform the edit
  ├── scope.RefreshWithRetry(3)        // Re-parse
  ├── clearCodeDOMFacts()              // Remove stale facts
  └── kernel.Assert(scope.ScopeFacts()) // Update Mangle
     │
     ▼
Return: { lines_affected, new_line_count, success }
```

## Mangle Fact Generation

CodeElement.ToFacts() converts elements to Mangle atoms:

```mangle
# Base element facts
code_element("go:user.go:User", /struct, "internal/auth/user.go", 10, 45).
code_element("go:user.go:User.Login", /method, "internal/auth/user.go", 15, 30).

# Signatures
element_signature("go:user.go:User", "type User struct {").
element_signature("go:user.go:User.Login", "func (u *User) Login(ctx context.Context) error").

# Relationships
element_parent("go:user.go:User.Login", "go:user.go:User").
element_visibility("go:user.go:User.Login", /public).

# Actions
code_interactable("go:user.go:User.Login", /view).
code_interactable("go:user.go:User.Login", /replace).

# Language-specific (Stratum 0)
go_struct("go:user.go:User").
go_tag("go:user.go:User.ID", "json:\"user_id\"").
```

## Safety Mechanisms

### File Hash Verification

Before editing, verify the file hasn't changed since loading:

```go
func (s *FileScope) VerifyFileHash(file string) error {
    current := hashFile(file)
    expected := s.FileHashes[file]
    if current != expected {
        return ErrConcurrentModification
    }
    return nil
}
```

### Refresh with Retry

After edits, re-parse with exponential backoff:

```go
func (s *FileScope) RefreshWithRetry(maxRetries int) error {
    for i := 0; i < maxRetries; i++ {
        err := s.Refresh()
        if err == nil {
            return nil
        }
        if !isConcurrentModError(err) {
            return err
        }
        time.Sleep(time.Duration(1<<i) * 100 * time.Millisecond)
    }
    return ErrRefreshFailed
}
```

### Constitutional Gate

All edits pass through the Constitutional Gate:

```mangle
# Default deny
permitted(Action) :- safe_action(Action), !deny_edit(Action, _).

# Block edits that would cause regressions
deny_edit(Ref, /security_regression) :-
    snapshot:has_decorator(Ref, "/login_required"),
    not candidate:has_decorator(Ref, "/login_required").
```

## Error Handling

### Common Error Facts

```mangle
scope_open_failed(Path, Error).
parse_error(File, Error, Timestamp).
file_hash_mismatch(Path, ExpectedHash, ActualHash).
element_stale(Ref, Reason).
scope_refresh_failed(Path, Error).
encoding_issue(Path, IssueType).  # /bom_detected, /crlf_inconsistent, /non_utf8
large_file_warning(Path, LineCount, ByteSize).
```

### Recovery Patterns

| Error | Recovery |
|-------|----------|
| `file_hash_mismatch` | Reload scope, retry operation |
| `element_stale` | Refresh scope, re-resolve ref |
| `parse_error` | Log warning, skip file |
| `encoding_issue` | Normalize encoding, retry |

## Testing Infrastructure

### Integration Tests

Location: `internal/system/dom_demo_test.go`, `internal/system/dom_mangle_test.go`

Test workflow:
1. Create temp workspace with test files
2. Initialize FileScope
3. Call open_file, get_elements, edit_element
4. Verify Mangle facts generated correctly
5. Verify file modifications applied
6. Clean up

### Test Fixtures

Create realistic code samples covering:
- Functions with various signatures
- Nested classes/structs
- Decorators/annotations
- Multi-line definitions
- Edge cases (empty files, single elements)
