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

