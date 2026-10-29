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
Decl file_topology(Path, Hash, Language, LastModified, IsTestFile) bound [/string, /string, /name, /number, /name].

# directory(Path, Name)
Decl directory(Path, Name) bound [/string, /string].

# file_dir(Path, Dir) - a file's package directory (companion to file_topology).
# Emitted by the world scanner so relational rules can join files within the
# same package/directory instead of forming repo-wide Cartesian products.
Decl file_dir(Path, Dir) bound [/string, /string].

# file_metadata_only(Path, Language, LastModified, Size, Treatment) - a member
# file the scanner deliberately did not hash, read or parse because an
# operator-confirmed treatment (workspace.TreatmentDecision.Enforced) says
# /index_names_only for its scope. It has no hash by design: file_topology's
# hash slot is never forged, so an unknown body stays unknown. Re-derived by
# every scan, never persisted.
Decl file_metadata_only(Path, Language, LastModified, Size, Treatment) bound [/string, /name, /number, /number, /name].

# treatment_scope(Dir, Treatment, Files, Bytes) - the coverage record of one
# enforced scope: how many member files under Dir (the governing census
# directory) were left unread, and their bytes. It is the evidence that an
# empty search or impact answer for Dir covers names only (/index_names_only)
# or nothing at all (/exclude); absence there is not proof of absence.
Decl treatment_scope(Dir, Treatment, Files, Bytes) bound [/string, /name, /number, /number].

# file_exists(Path) - Derived from file_topology and file_metadata_only
# Checks if a file exists in the workspace
Decl file_exists(Path) bound [/string].
file_exists(Path) :- file_topology(Path, _, _, _, _).
file_exists(Path) :- file_metadata_only(Path, _, _, _, _).

# modified(FilePath) - marks a file as modified
# Priority: 85
# SerializationOrder: 8
Decl modified(FilePath) bound [/string].

# test_coverage(FilePath) - marks a file as having test coverage
Decl test_coverage(FilePath) bound [/string].

# =============================================================================
# SECTION 4: SYMBOL GRAPH / AST PROJECTION (§2.3)
# =============================================================================

# symbol_graph(SymbolID, Type, Visibility, DefinedAt, Signature)
# Type: /function, /class, /interface, /struct, /variable, /constant
# Visibility: /public, /private, /protected
# Priority: 75
# SerializationOrder: 12
Decl symbol_graph(SymbolID, Type, Visibility, DefinedAt, Signature) bound [/string, /name, /name, /string, /string].

# dependency_link(CallerID, CalleeID, ImportPath)
# Priority: 70
# SerializationOrder: 13
Decl dependency_link(CallerID, CalleeID, ImportPath) bound [/string, /string, /string].

# Import-graph measurements (internal/world/import_cycles.go).
# imports_project_file(File): File has a resolved in-workspace import edge.
Decl imports_project_file(File) bound [/string].
# import_cycle_member(File, Root): File lies on an import cycle (a strongly
# connected component of two or more files), named by its smallest path.
Decl import_cycle_member(File, Root) bound [/string, /string].

# =============================================================================
# SECTION 5: DIAGNOSTICS / LINTER-LOGIC BRIDGE (§2.2)
# =============================================================================

# diagnostic(Severity, FilePath, Line, ErrorCode, Message)
# Severity: /panic, /error, /warning, /info
# Priority: 95
# SerializationOrder: 3
Decl diagnostic(Severity, FilePath, Line, ErrorCode, Message) bound [/name, /string, /number, /string, /string].

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
Decl symbol_defined(Lang, SymbolName, FilePath, Line, Column) bound [/name, /string, /string, /number, /number].

# symbol_referenced(Lang, SymbolName, FilePath, Line, Column, Kind)
# Marks where a symbol is referenced/used in code
# Kind: /head, /body, /fact, /query (for Mangle), /call, /import (for other languages)
# Priority: 70
# SerializationOrder: 13
Decl symbol_referenced(Lang, SymbolName, FilePath, Line, Column, Kind) bound [/name, /string, /string, /number, /number, /name].

# code_diagnostic(FilePath, Line, Severity, Message)
# LSP-derived diagnostics (syntax errors, warnings, hints)
# Severity: /error, /warning, /info, /hint
# Priority: 95
# SerializationOrder: 3
Decl code_diagnostic(FilePath, Line, Severity, Message) bound [/string, /number, /name, /string].

# symbol_completion(FilePath, Line, Column, Suggestions)
# Available completions at a cursor position (list of suggestion strings)
# Used by LegislatorShard and CoderShard for intelligent code generation
# Priority: 60
# SerializationOrder: 15
Decl symbol_completion(FilePath, Line, Column, Suggestions) bound [/string, /number, /number, /string].
