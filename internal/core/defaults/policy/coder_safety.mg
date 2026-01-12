# Coder Shard Policy - Edit Safety & Blocking
# Description: Safety rules to prevent dangerous or low-quality edits.

# =============================================================================
# SECTION 5: EDIT SAFETY & BLOCKING
# =============================================================================

# -----------------------------------------------------------------------------
# 5.1 Block Write Conditions
# -----------------------------------------------------------------------------

# Block if impacted files lack test coverage
coder_block_write(File, "uncovered_impact") :-
    pending_edit(File, _),
    dependency_link(Dependent, File, _),
    coder_impacted(Dependent),
    !test_coverage(Dependent).

# Block writes outside workspace
coder_block_action(/edit, "forbidden_path") :-
    pending_edit(Path, _),
    !path_in_workspace(Path).

# Block binary file modifications
coder_block_action(/edit, "binary_file") :-
    pending_edit(Path, _),
    is_binary_file(Path).

# Block edits to generated files
coder_block_action(/edit, "generated_file") :-
    pending_edit(Path, _),
    is_generated_file(Path).

# Block edits to vendor/third-party code
coder_block_action(/edit, "vendor_file") :-
    pending_edit(Path, _),
    is_vendor_file(Path).

# Helper: any pending edit is implementation
Decl has_implementation_edit().
has_implementation_edit() :-
    edit_is_implementation(_).

# Block edits during active TDD red phase (tests should fail first)
coder_block_action(/edit, "tdd_red_phase") :-
    !has_implementation_edit(),
    pending_edit(_, _),
    tdd_state(/red).

# Helpers
Decl is_generated_file(Path).
is_generated_file(Path) :-
    path_contains(Path, "generated").

is_generated_file(Path) :-
    path_contains(Path, "_gen.").

Decl is_vendor_file(Path).
is_vendor_file(Path) :-
    path_contains(Path, "vendor/").

is_vendor_file(Path) :-
    path_contains(Path, "node_modules/").

# -----------------------------------------------------------------------------
# 5.2 Safety Check Aggregation
# -----------------------------------------------------------------------------

# Helper for safe negation: true if any block exists for file
has_coder_block(File) :-
    coder_block_write(File, _).

has_coder_block(File) :-
    coder_block_action(/edit, _),
    pending_edit(File, _).

# Safe to write check
coder_safe_to_write(File) :-
    pending_edit(File, _),
    !has_coder_block(File).

# -----------------------------------------------------------------------------
# 5.3 Edit Quality Gates
# -----------------------------------------------------------------------------

# Edit should include tests if creating new code
edit_needs_tests(File) :-
    coder_task(_, /create, File, _),
    detected_language(File, Lang),
    testable_language(Lang),
    !is_test_file(File).

# Edit should update docs if modifying public API
edit_needs_docs(File) :-
    coder_task(_, /modify, File, _),
    is_public_api(File),
    !doc_exists_for(File).

# Testable languages
testable_language(/go).
testable_language(/python).
testable_language(/typescript).
testable_language(/javascript).
testable_language(/rust).
testable_language(/java).
