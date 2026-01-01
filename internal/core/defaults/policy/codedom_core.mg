# Code DOM Core Logic
# File scope, element accessibility, and containment

# --- File Scope Rules ---

# A file is in scope if it's the active file
in_scope(File) :- active_file(File).

# A file is in scope if Code DOM loaded it into scope.
in_scope(File) :-
    file_in_scope(File, _, _, _).

# --- Element Accessibility Rules ---

# An element is editable if its file is in scope and it has replace action
editable(Ref) :-
    code_element(Ref, _, File, _, _),
    in_scope(File),
    code_interactable(Ref, /replace).

# All functions in scope (for querying)
function_in_scope(Ref, File, Sig) :-
    code_element(Ref, /function, File, _, _),
    in_scope(File),
    element_signature(Ref, Sig).

# All methods in scope
method_in_scope(Ref, File, Sig) :-
    code_element(Ref, /method, File, _, _),
    in_scope(File),
    element_signature(Ref, Sig).

# Method belongs to struct
method_of(MethodRef, StructRef) :- element_parent(MethodRef, StructRef).

# --- Transitive Element Containment ---

# Direct containment
code_contains(Parent, Child) :- element_parent(Child, Parent).

# Transitive containment
code_contains(Ancestor, Descendant) :-
    element_parent(Mid, Ancestor),
    code_contains(Mid, Descendant).

# --- Mock & Interface Rules ---

# Struct implements interface if it has methods matching interface methods
interface_impl(StructRef, InterfaceRef) :-
    code_element(StructRef, /struct, File, _, _),
    code_element(InterfaceRef, /interface, File, _, _),
    element_parent(MethodRef, StructRef),
    code_element(MethodRef, /method, _, _, _).

# Test file mocks source file if it's a _test.go in same package
# BUG-002 FIX: Constrained to prevent Cartesian explosion (was 592^2 = 349K facts)
# Now: only pairs (test file, source file) - much smaller set
mock_file(TestFile, SourceFile) :-
    file_topology(TestFile, _, /go, _, /true),   # TestFile must be a test file
    file_topology(SourceFile, _, /go, _, /false), # SourceFile must NOT be a test file
    TestFile != SourceFile.

# Suggest updating mocks when source function signature changes
suggest_update_mocks(Ref) :-
    code_element(Ref, /function, File, _, _),
    element_visibility(Ref, /public),
    element_modified(Ref, _, _),
    mock_file(TestFile, File).

suggest_update_mocks(Ref) :-
    code_element(Ref, /method, File, _, _),
    element_visibility(Ref, /public),
    element_modified(Ref, _, _),
    mock_file(TestFile, File).

# --- Scope Staleness Detection ---

# File modified externally: hash doesn't match what we loaded
file_modified_externally(Path) :-
    file_hash_mismatch(Path, _, _).

# Scope needs refresh when any in-scope file was modified
needs_scope_refresh() :-
    active_file(ActiveFile),
    in_scope(File),
    modified(File).

needs_scope_refresh() :-
    file_modified_externally(_).

# Helper for safe negation
scope_refreshed(File) :-
    file_in_scope(File, _, _, _),
    !modified(File).
