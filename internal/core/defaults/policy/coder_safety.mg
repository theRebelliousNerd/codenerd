# Coder Shard Policy - Edit Safety & Blocking
# Description: Safety rules to prevent dangerous or low-quality edits.

# =============================================================================
# SECTION 5: EDIT SAFETY & BLOCKING
# =============================================================================

# -----------------------------------------------------------------------------
# 5.1 Block Write Conditions
# -----------------------------------------------------------------------------

# test_coverage is derived from test_file_for pairings computed by the world
# scanner because it needs string manipulation Mangle does not do. Coverage is
# deliberately conservative: the x_test.go convention misses a source file
# covered only by a package-level test with another name. That under-reports
# coverage, which leaves the four rules cautious rather than falsely permissive,
# and cautious is the correct direction to be wrong in for a rule that gates
# refactors and writes.
test_coverage(SourceFile) :- test_file_for(_, SourceFile).
Decl created_source(File) bound [/string].
Decl missing_test_for(File) bound [/string].

# A turn that created new source owes a test for it. test_coverage is
# derived from test_file_for, which the world scanner emits for the
# x_test.go convention and which this turn also asserts for any test file
# it creates, so a source and its test written in the same turn satisfy
# this without waiting for a rescan.
missing_test_for(File) :- created_source(File), !test_coverage(File).

# F-RUN-3: verify claimed test output — Go measures, Mangle decides.
# claimed_test_output/1 is asserted when the response presents runner output,
# executed_test_tool/1 when a test tool actually ran. unverified_test_claim
# fires when the former is true and the latter is not; Verb is bound by the
# positive atom so negation is safe. Uses /string for Verb so a leading-slash
# verb ("/fix") is stored as a string, not a Mangle name constant.
Decl claimed_test_output(Verb) bound [/string].
Decl executed_test_tool(Verb) bound [/string].
Decl unverified_test_claim(Verb) bound [/string].
unverified_test_claim(Verb) :- claimed_test_output(Verb), !executed_test_tool(Verb).

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

# Turn verdict — the executor asserts one turn_evidence fact per turn (Go
# measures: tool/write/test counts, claimed-output flag, dream flag) and
# Mangle derives the verdict (Mangle decides). Go consumers query
# hollow_success/1 for the failure reason and turn_done/1 for the single
# completion signal instead of reimplementing these checks imperatively.
# The Go fallback in checkHollowSuccess stays for nil/degraded kernels.
# Verb uses /name (not /string) so it unifies with the /name-typed
# write_oriented_intent/1 and intent_requires_tool_call/1 facts in
# delegation.mg; Go asserts it via MangleAtom ("/create" -> /create). A
# /string verb would never unify with those atoms and the hollow rules
# below would silently never fire.
Decl turn_evidence(Verb, ToolCount, WriteCount, TestCount, ClaimedOutput, DreamMode) bound [/name, /number, /number, /number, /name, /name].
Decl hollow_success(Reason) bound [/string].
Decl turn_done(Verb) bound [/name].
Decl has_turn_tools(Verb) bound [/name].
Decl has_turn_write(Verb) bound [/name].
Decl has_turn_test(Verb) bound [/name].
Decl has_hollow_success() bound [].
has_turn_tools(Verb) :- turn_evidence(Verb, ToolCount, _, _, _, _), ToolCount > 0.
has_turn_write(Verb) :- turn_evidence(Verb, _, WriteCount, _, _, _), WriteCount > 0.
has_turn_test(Verb) :- turn_evidence(Verb, _, _, TestCount, _, _), TestCount > 0.
has_hollow_success() :- hollow_success(_).
hollow_success("requires side effects but no tool call succeeded") :-
    turn_evidence(Verb, _, _, _, _, /false),
    intent_requires_tool_call(Verb),
    !has_turn_tools(Verb).
hollow_success("write-oriented intent completed without a recognized write-mutation tool") :-
    turn_evidence(Verb, _, _, _, _, /false),
    write_oriented_intent(Verb),
    has_turn_tools(Verb),
    !has_turn_write(Verb).
hollow_success("response presents test-runner output but no test-execution tool ran") :-
    turn_evidence(Verb, _, _, _, /true, /false),
    !has_turn_test(Verb).
hollow_success("new source was created without a test file") :-
    turn_evidence(Verb, _, _, _, _, /false),
    missing_test_for(File).
# turn_done is the single completion signal. It must never derive alongside
# hollow_success (a no-write / no-tool / unverified turn is not done) nor
# while the build is red (a failed build is not done). Deriving done in
# either case is hollow success with a policy stamp on it.
turn_done(Verb) :- turn_evidence(Verb, _, _, _, _, _), !has_hollow_success(), !build_state(/failing).
# Helper: any pending edit is implementation
Decl has_implementation_edit() bound [].
has_implementation_edit() :-
    edit_is_implementation(_).

# Block edits during active TDD red phase (tests should fail first)
coder_block_action(/edit, "tdd_red_phase") :-
    !has_implementation_edit(),
    pending_edit(_, _),
    tdd_state(/red).

# Helpers
# Decl is_generated_file(Path) - Declared in schemas_coder.mg
is_generated_file(Path) :-
    path_contains(Path, "generated").

is_generated_file(Path) :-
    path_contains(Path, "_gen.").

# Decl is_vendor_file(Path) - Declared in schemas_coder.mg
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
