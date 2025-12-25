# Cortex 1.5.0 Schemas (EDB Declarations)
# Version: 1.5.0
# Philosophy: Logic determines Reality; the Model merely describes it.

# Modular Schema: WORLD
# Sections: 3, 4, 5

# =============================================================================
# SECTION 3: FILE TOPOLOGY (§2.1)
# =============================================================================

# file_topology(Path, Hash, Language, LastModified, IsTestFile)
# Language: /go, /python, /ts, /rust, /java, /js
# IsTestFile: /true, /false
# Priority: 80
# SerializationOrder: 10
Decl file_topology(Path, Hash, Language, LastModified, IsTestFile).

# directory(Path, Name)
Decl directory(Path, Name).

# modified(FilePath) - marks a file as modified
# Priority: 85
# SerializationOrder: 8
Decl modified(FilePath).

# test_coverage(FilePath) - marks a file as having test coverage
Decl test_coverage(FilePath).

# =============================================================================
# SECTION 4: SYMBOL GRAPH / AST PROJECTION (§2.3)
# =============================================================================

# symbol_graph(SymbolID, Type, Visibility, DefinedAt, Signature)
# Type: /function, /class, /interface, /struct, /variable, /constant
# Visibility: /public, /private, /protected
# Priority: 75
# SerializationOrder: 12
Decl symbol_graph(SymbolID, Type, Visibility, DefinedAt, Signature).

# dependency_link(CallerID, CalleeID, ImportPath)
# Priority: 70
# SerializationOrder: 13
Decl dependency_link(CallerID, CalleeID, ImportPath).

# =============================================================================
# SECTION 5: DIAGNOSTICS / LINTER-LOGIC BRIDGE (§2.2)
# =============================================================================

# diagnostic(Severity, FilePath, Line, ErrorCode, Message)
# Severity: /panic, /error, /warning, /info
# Priority: 95
# SerializationOrder: 3
Decl diagnostic(Severity, FilePath, Line, ErrorCode, Message).

# =============================================================================
# SECTION 5.1: LSP CODE INTELLIGENCE (Language Server Protocol Integration)
# =============================================================================
# These predicates are derived from LSP servers (Mangle LSP, gopls, etc.)
# and provide semantic code intelligence to the World Model.

# symbol_defined(Lang, SymbolName, FilePath, Line, Column)
# Marks where a symbol is defined in code
# Lang: /mangle, /go, /python, /rust, etc.
# Priority: 75
# SerializationOrder: 12
Decl symbol_defined(Lang, SymbolName, FilePath, Line, Column).

# symbol_referenced(Lang, SymbolName, FilePath, Line, Column, Kind)
# Marks where a symbol is referenced/used in code
# Kind: /head, /body, /fact, /query (for Mangle), /call, /import (for other languages)
# Priority: 70
# SerializationOrder: 13
Decl symbol_referenced(Lang, SymbolName, FilePath, Line, Column, Kind).

# code_diagnostic(FilePath, Line, Severity, Message)
# LSP-derived diagnostics (syntax errors, warnings, hints)
# Severity: /error, /warning, /info, /hint
# Priority: 95
# SerializationOrder: 3
Decl code_diagnostic(FilePath, Line, Severity, Message).

# symbol_completion(FilePath, Line, Column, Suggestions)
# Available completions at a cursor position (list of suggestion strings)
# Used by LegislatorShard and CoderShard for intelligent code generation
# Priority: 60
# SerializationOrder: 15
Decl symbol_completion(FilePath, Line, Column, Suggestions).

