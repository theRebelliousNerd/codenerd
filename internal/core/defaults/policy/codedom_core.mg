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

# interface_impl (Decl in schemas_codedom.mg) intentionally has NO derivation
# rule. The previous rule paired every struct having >=1 method with every
# interface in the same file — no method-set matching at all — materializing
# a per-file struct x interface product of FALSE facts. Nothing consumed it,
# so it was pure cost plus misinformation for ad-hoc `nerd logic` queries.
# Real interface satisfaction needs method-set comparison, which belongs in
# the Go analyzer (assert code_implements facts from go/types), not Mangle.

# A source file is covered by a mock/test when a Go _test.go lives in its
# package directory. Two linear derivations replace the former
# mock_file(TestFile, SourceFile) pair table. History: the first version
# paired every test with every source across the repo (~500K facts, kernel
# fact-limit overflow); the second bounded it per directory (~31K facts on
# this codebase) but still materialised every pair, and once the world shard
# began evaluating on every turn (item 55, 2026-09-04) that join alone cost
# 17 s of a 24.5 s evaluation. The only consumer, suggest_update_mocks, needs
# "does File's directory hold a test", never the pairs, so the pairs are gone.
dir_has_go_test(Dir) :-
    file_topology(TestFile, _, /go, _, /true),
    file_dir(TestFile, Dir).

source_has_test_in_dir(SourceFile) :-
    file_topology(SourceFile, _, /go, _, /false),
    file_dir(SourceFile, Dir),
    dir_has_go_test(Dir).

# Suggest updating mocks when source function signature changes
suggest_update_mocks(Ref) :-
    code_element(Ref, /function, File, _, _),
    element_visibility(Ref, /public),
    element_modified(Ref, _, _),
    source_has_test_in_dir(File).

suggest_update_mocks(Ref) :-
    code_element(Ref, /method, File, _, _),
    element_visibility(Ref, /public),
    element_modified(Ref, _, _),
    source_has_test_in_dir(File).

# --- Scope Staleness Detection ---

# File modified externally: hash doesn't match what we loaded
file_modified_externally(Path) :-
    file_hash_mismatch(Path, _, _).

# Scope needs refresh when any in-scope file was modified
# active_file(_) is an existence guard (is any file open?), not a join — the
# zero-arg head dedupes the product to a single fact.
needs_scope_refresh() :-
    active_file(_),
    in_scope(File),
    modified(File).

needs_scope_refresh() :-
    file_modified_externally(_).

# Helper for safe negation
scope_refreshed(File) :-
    file_in_scope(File, _, _, _),
    !modified(File).
