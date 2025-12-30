# LSP Integration for codeNERD

## Overview

The LSP (Language Server Protocol) integration provides **code intelligence for Mangle files** and serves as a foundation for multi-language code understanding within the World Model subsystem.

### Architecture Philosophy

**LSP is not just a tool - it's a World Model Enhancement.**

Unlike traditional LSP servers that only serve editors, codeNERD's LSP integration is architected as a **semantic projection layer** that enriches the World Model with code intelligence facts, enabling:

1. **Spreading Activation** - Logic-driven context selection based on symbol relationships
2. **Shard Intelligence** - CoderShard, ReviewerShard, LegislatorShard can validate and analyze code
3. **External Editors** - VSCode, Neovim, etc. get native Mangle language support

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│              World Model (Ground Truth)                 │
│  ┌───────────────────────────────────────────────────┐  │
│  │  LSP Manager (Multi-Language Code Intelligence)  │  │
│  │  • Mangle LSP (built-in)                         │  │
│  │  • gopls (future: Go code intelligence)          │  │
│  │  • [extensible to any language]                  │  │
│  └───────────────────────────────────────────────────┘  │
│                         │                               │
│                         ▼                               │
│  ┌───────────────────────────────────────────────────┐  │
│  │   Fact Projection (into Mangle EDB)               │  │
│  │   • symbol_defined(Lang, Name, File, Line, Col)   │  │
│  │   • symbol_referenced(Lang, Name, File, ...)      │  │
│  │   • code_diagnostic(File, Line, Severity, Msg)    │  │
│  └───────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────┘
                         │
       ┌─────────────────┼─────────────────┐
       ▼                 ▼                 ▼
┌──────────────┐  ┌─────────────┐  ┌──────────────┐
│   Spreading  │  │   Shards    │  │ Stdio/MCP    │
│  Activation  │  │  (Tools)    │  │  (Editors)   │
│   (Logic)    │  │             │  │              │
└──────────────┘  └─────────────┘  └──────────────┘
```

## Components

### 1. LSP Manager (`manager.go`)

The central coordinator that:
- Manages language server instances (currently Mangle LSP, extensible to gopls, etc.)
- Projects LSP data into World Model facts
- Provides batch query APIs for shards
- Exposes stdio server for external editors

**Key Methods:**
```go
// Initialize and index workspace
manager.Initialize(ctx)

// Project all LSP data to Mangle facts
facts, _ := manager.ProjectToFacts()

// Batch queries for shards
defFacts, _ := manager.GetDefinitions("user_intent")
refFacts, _ := manager.GetReferences("next_action")
diagFacts, _ := manager.ValidateCode(filePath, code)

// Stdio server for editors
manager.ServeStdio(ctx)
```

### 2. Mangle LSP Server (`internal/mangle/lsp.go`)

The existing LSP server implementation (924 lines) that provides:
- **Document Management**: Open/close/change tracking
- **Symbol Indexing**: Definitions, references, hover docs
- **Diagnostics**: Syntax errors, missing periods, unbalanced parens
- **LSP Methods**: Definition, references, hover, completion
- **JSON-RPC**: Full LSP protocol over stdin/stdout

**New Batch Query APIs** (added for World Model integration):
```go
// Get all data for projection
allDefs := server.GetAllDefinitions()
allRefs := server.GetAllReferences()
allDiags := server.GetAllDiagnostics()

// Validate code without opening as document
diags := server.ValidateCode(uri, code)
```

### 3. World Model Integration

**New EDB Predicates** (`internal/core/defaults/schemas_world.mg`):

```mangle
# Symbol definitions
Decl symbol_defined(Lang, SymbolName, FilePath, Line, Column).

# Symbol references
Decl symbol_referenced(Lang, SymbolName, FilePath, Line, Column, Kind).

# LSP diagnostics
Decl code_diagnostic(FilePath, Line, Severity, Message).

# Completions (future)
Decl symbol_completion(FilePath, Line, Column, Suggestions).
```

**World Predicates List** (`internal/world/world_predicates.go`):
```go
var WorldPredicates = []string{
    // ... existing predicates ...
    "symbol_defined",
    "symbol_referenced",
    "code_diagnostic",
    "symbol_completion",
}
```

## Usage

### For External Editors (VSCode, Neovim, etc.)

#### 1. Start LSP Server

```bash
nerd mangle-lsp --workspace /path/to/project
```

#### 2. Configure Editor

**VSCode** (`settings.json`):
```json
{
  "mangle": {
    "server": {
      "command": "nerd",
      "args": ["mangle-lsp"]
    },
    "filetypes": ["mangle", "mg"]
  }
}
```

**Neovim** (`init.lua`):
```lua
require('lspconfig').mangle.setup{
  cmd = {'nerd', 'mangle-lsp'},
  filetypes = {'mangle', 'mg'},
  root_dir = function(fname)
    return vim.fn.getcwd()
  end
}
```

#### 3. Features Available

- **Autocomplete**: Trigger with `/`, `:`, `(` for predicates and name constants
- **Go to Definition**: Jump to where a predicate is declared or defined
- **Find References**: See all usages of a symbol across files
- **Hover Documentation**: View predicate arity and declaration on hover
- **Diagnostics**: Real-time syntax error highlighting

### For Shards (CoderShard, LegislatorShard, ReviewerShard)

Shards access LSP through the **SessionContext** or **direct LSP Manager API**:

#### Example: CoderShard Validating Generated Mangle Code

```go
// Get LSP manager from session context
lspMgr := sessionCtx.GetLSPManager()

// Validate generated code before committing
diags, err := lspMgr.ValidateCode("policy.mg", generatedCode)
if err != nil {
    return fmt.Errorf("validation failed: %w", err)
}

// Check for errors
hasErrors := false
for _, diag := range diags {
    if diag.Severity == "/error" {
        hasErrors = true
        log.Error("LSP error at line %d: %s", diag.Line, diag.Message)
    }
}

if hasErrors {
    // Regenerate code
    return regenerateCode()
}

// Code is valid, proceed with commit
```

#### Example: ReviewerShard Analyzing Impact

```go
// Find all references to a modified predicate
refFacts, err := lspMgr.GetReferences("user_intent")

// Convert to file list for impact analysis
affectedFiles := make(map[string]bool)
for _, fact := range refFacts {
    filePath := fact.Args[2].(string)
    affectedFiles[filePath] = true
}

log.Info("Modification to user_intent affects %d files", len(affectedFiles))
```

### For Kernel (Spreading Activation, Policy Rules)

LSP facts are queryable from Mangle logic:

```mangle
# Select files that reference a symbol the user mentioned
context_atom(File) :-
    user_intent(_, _, TargetSymbol, _),
    symbol_referenced(/mangle, TargetSymbol, File, _, _, _).

# Include definition files
context_atom(File) :-
    user_intent(_, _, TargetSymbol, _),
    symbol_defined(/mangle, TargetSymbol, File, _, _).

# Validation: code must pass LSP diagnostics
code_validated(File) :-
    file_topology(File, _, /mangle, _, _),
    not (code_diagnostic(File, _, /error, _)).

# Safe to commit only if validated
permitted(commit_file(File)) :-
    code_validated(File).
```

## Benefits

### 1. Unified Code Intelligence

One LSP Manager serves:
- **External editors** (VSCode, Neovim) via stdio
- **Internal shards** (CoderShard, ReviewerShard) via batch queries
- **Mangle kernel** (Spreading Activation) via fact projection

### 2. Logic-First Architecture

LSP data becomes **queryable facts**, enabling:
- Declarative impact analysis
- Automated hypothesis verification (ReviewerShard)
- Context selection based on semantic relationships

### 3. Multi-Language Extensibility

The architecture supports any language:
- **Phase 1**: Mangle LSP (implemented)
- **Phase 3**: gopls integration (for Go code)
- **Future**: rust-analyzer, typescript, python-lsp, etc.

### 4. Autopoiesis Enhancement

- **Ouroboros**: Validates generated tools via LSP before Thunderdome
- **Dream Mode**: Uses LSP for impact prediction before applying changes
- **Nemesis**: Analyzes policy for attack surfaces via symbol relationships

## Testing

### Manual Testing

1. **Create test file** (`test.mg`):
```mangle
Decl test_predicate(X, Y).

test_predicate(/foo, /bar).

test_rule(X) :- test_predicate(X, _).
```

2. **Start LSP server**:
```bash
nerd mangle-lsp --workspace .
```

3. **Send LSP request** (via editor or manual JSON-RPC):
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "textDocument/definition",
  "params": {
    "textDocument": {"uri": "file:///path/to/test.mg"},
    "position": {"line": 4, "character": 12}
  }
}
```

### Automated Testing

```bash
# Validate LSP fact projection
go test ./internal/world/lsp/...

# Test Mangle LSP server
go test ./internal/mangle/... -run LSP
```

## Future Enhancements

### Phase 2: Virtual Predicates

Add LSP queries as virtual predicates in VirtualStore:

```mangle
# Query directly from policy
?lsp_definition(/mangle, "user_intent", DefFile, DefLine).
?lsp_references(/mangle, "next_action", Refs).
```

### Phase 3: Multi-Language Support

Add gopls integration for Go code intelligence:

```go
// internal/world/lsp/gopls_client.go
type GoplsClient struct {
    // Subprocess LSP client for gopls
}

manager.AddLanguageServer("/go", NewGoplsClient())
```

### Phase 4: MCP Server

Expose LSP as an MCP server for external tools:

```bash
# Run as MCP server instead of stdio
nerd mangle-lsp --mode mcp --port 9000
```

### Phase 5: Cross-Language Analysis

Analyze relationships between Mangle policy and Go code:

```mangle
# Find Go functions that implement virtual predicates
go_implements_predicate(GoFunc, ManglePred) :-
    symbol_defined(/mangle, ManglePred, PolicyFile, _, _),
    symbol_defined(/go, GoFunc, GoFile, _, _),
    # Cross-language linking logic
    ...
```

## Files Created/Modified

### New Files

- `internal/world/lsp/manager.go` - LSP Manager (multi-language coordinator)
- `internal/world/lsp/README.md` - This documentation
- `cmd/nerd/cmd_mangle_lsp.go` - CLI command for starting LSP server

### Modified Files

- `internal/mangle/lsp.go` - Added batch query APIs (GetAllDefinitions, GetAllReferences, GetAllDiagnostics, ValidateCode)
- `internal/world/world_predicates.go` - Added LSP predicates to WorldPredicates list
- `internal/core/defaults/schemas_world.mg` - Added LSP predicate declarations
- `cmd/nerd/main.go` - Registered mangleLSPCmd

## References

- **LSP Specification**: https://microsoft.github.io/language-server-protocol/
- **Mangle Grammar**: `internal/mangle/grammar.go`
- **World Model Docs**: `internal/world/CLAUDE.md`
- **Shard Architecture**: `CLAUDE.md` (project root)

---

**Architecture Philosophy**: LSP data flows into the World Model, enabling logic-based reasoning about code structure. This inverts the traditional approach where LSP is merely an editor feature - here, LSP becomes a first-class citizen of the neuro-symbolic architecture.
