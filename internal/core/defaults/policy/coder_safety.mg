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
# hollow_success/2 for the failure reason and turn_done/1 for the single
# completion signal instead of reimplementing these checks imperatively.
#
# Every relation in this block is keyed by Turn: one execution of the turn
# verdict, an atom the executor mints per turn (/turn_<pid>_<n>) and carries
# into every assertion, join, query and retraction. The kernel is shared --
# CloneForTask hands the same one to every delegated task, and a campaign runs
# tasks side by side -- so a key by verb let two concurrent /fix turns read
# each other's gates, coverage debt and hollow reasons (external audit F1,
# 2026-09-19). The verb is an attribute of turn_evidence, joined where the
# intent tables need it; it is never the identity of a turn.
#
# Verb uses /name (not /string) so it unifies with the /name-typed
# write_oriented_intent/1 and intent_requires_tool_call/1 facts in
# delegation.mg; Go asserts it via MangleAtom ("/create" -> /create). A
# /string verb would never unify with those atoms and the hollow rules
# below would silently never fire.
Decl turn_evidence(Turn, Verb, ToolCount, WriteCount, TestCount, ClaimedOutput, DreamMode) bound [/name, /name, /number, /number, /number, /name, /name].
Decl hollow_success(Turn, Reason) bound [/name, /string].
Decl turn_done(Turn) bound [/name].
Decl turn_executed(Turn) bound [/name].
Decl turn_verified(Turn) bound [/name].
Decl turn_unverified(Turn) bound [/name].
Decl turn_wrote(Turn) bound [/name].
Decl turn_build_failed(Turn) bound [/name].
Decl turn_missing_evidence(Turn, Missing) bound [/name, /name].
Decl turn_acceptance(Turn, Contract, Snapshot) bound [/name, /string, /string].
Decl has_turn_acceptance(Turn) bound [/name].
# turn_gate is THIS turn's post-edit gate as the session executor measured it
# (recordBuildState): Gate is /build, /test or /vet, Verdict is /passing or
# /failing, and only an affirmative verdict is ever asserted. The verdict rules
# below read the evidence of the turn they judge and nothing else.
# build_state/1 and test_state/1 are the session-global workspace state --
# written by the same gates, but also by the run_tests tool the model invokes,
# the TDD loop's state machine and the log reader, none of them per-turn -- and
# until 2026-09-18 turn_verified read test_state(/passing) as if it were this
# turn's gate: a green run left behind by a tool call in turn N verified a
# write in turn N+k whose own test gate was skipped (REVIEW-wave1 F2).
# Retracted with turn_evidence at the end of the turn.
Decl turn_gate(Turn, Gate, Verdict) bound [/name, /name, /name].
Decl turn_build_green(Turn) bound [/name].
Decl turn_build_red(Turn) bound [/name].
Decl turn_tests_green(Turn) bound [/name].
Decl turn_tests_red(Turn) bound [/name].
turn_build_green(Turn) :- turn_gate(Turn, /build, /passing).
turn_build_red(Turn) :- turn_gate(Turn, /build, /failing).
turn_tests_green(Turn) :- turn_gate(Turn, /test, /passing).
turn_tests_red(Turn) :- turn_gate(Turn, /test, /failing).

# turn_untested is THIS turn's coverage debt as the session executor measured it
# on disk: a production Go file the turn wrote with no test file beside it
# (build_verify.go, untestedWithoutCoverageOnDisk). Until 2026-09-18 that list
# went to the log and the subagent return and nowhere else, so a turn that wrote
# production code and no test derived turn_verified on a green build and a green
# run of the tests that already existed, and was recorded /done. Asserted by the
# executor with the turn's gates, retracted with them, unreachable by the model.
Decl turn_untested(Turn, Path) bound [/name, /string].
Decl turn_has_untested(Turn) bound [/name].
turn_has_untested(Turn) :- turn_untested(Turn, _).

# turn_uncovered is the rest of this turn's coverage debt: a file the turn
# changed holding blocks, on the lines it changed, that no test executes
# (the executor's coverage profile narrowed to the turn's own lines,
# build_verify.go). turn_untested asks whether a test file exists beside the
# code; this asks whether any test runs the code the turn wrote. Until
# 2026-09-19 the list reached the log and an advisory critic only, and a turn
# whose 27 new blocks no test executed was recorded /done.
Decl turn_uncovered(Turn, Path) bound [/name, /string].
Decl turn_has_uncovered(Turn) bound [/name].
turn_has_uncovered(Turn) :- turn_uncovered(Turn, _).

# `go vet` over the packages the turn wrote, charged with the findings the turn
# introduced. Only the red side withholds the verdict: a turn with no Go to vet
# asserts no gate, and owing a green one would leave every non-Go write
# unverifiable.
Decl turn_vet_green(Turn) bound [/name].
Decl turn_vet_red(Turn) bound [/name].
turn_vet_green(Turn) :- turn_gate(Turn, /vet, /passing).
turn_vet_red(Turn) :- turn_gate(Turn, /vet, /failing).

# A turn that created new Go source owes a test for it. turn_created_source
# and turn_created_test are the files this turn created, as the executor
# recorded them; they are the turn's, not the world's. The executor used to
# write created_source and test_file_for into the shared kernel for them, and
# its end-of-turn sweep then retracted every test_file_for fact, the world
# scanner's included. A created source is covered by a test the world already
# pairs with it (test_coverage, from the scanner's test_file_for) or by a test
# this same turn created, so a source and its test written together satisfy it
# without waiting for a rescan.
Decl turn_created_test(Turn, TestFile, SourceFile) bound [/name, /string, /string].
Decl turn_test_coverage(Turn, File) bound [/name, /string].
Decl turn_missing_test(Turn, File) bound [/name, /string].
turn_test_coverage(Turn, File) :- turn_created_source(Turn, File), test_coverage(File).
turn_test_coverage(Turn, File) :- turn_created_test(Turn, _, File).
turn_missing_test(Turn, File) :- turn_created_source(Turn, File), !turn_test_coverage(Turn, File).

Decl has_turn_tools(Turn) bound [/name].
Decl has_turn_write(Turn) bound [/name].
Decl has_turn_test(Turn) bound [/name].
Decl has_hollow_success(Turn) bound [/name].
has_turn_tools(Turn) :- turn_evidence(Turn, _, ToolCount, _, _, _, _), ToolCount > 0.
has_turn_write(Turn) :- turn_evidence(Turn, _, _, WriteCount, _, _, _), WriteCount > 0.
has_turn_test(Turn) :- turn_evidence(Turn, _, _, _, TestCount, _, _), TestCount > 0.
has_hollow_success(Turn) :- hollow_success(Turn, _).
hollow_success(Turn, "requires side effects but no tool call succeeded") :-
    turn_evidence(Turn, Verb, _, _, _, _, /false),
    intent_requires_tool_call(Verb),
    !has_turn_tools(Turn).
hollow_success(Turn, "write-oriented intent completed without a recognized write-mutation tool") :-
    turn_evidence(Turn, Verb, _, _, _, _, /false),
    write_oriented_intent(Verb),
    has_turn_tools(Turn),
    !has_turn_write(Turn).
hollow_success(Turn, "response presents test-runner output but no test-execution tool ran") :-
    turn_evidence(Turn, _, _, _, _, /true, /false),
    !has_turn_test(Turn).
hollow_success(Turn, "new source was created without a test file") :-
    turn_evidence(Turn, _, _, _, _, _, /false),
    turn_missing_test(Turn, _).
# turn_done is the single completion signal. It must never derive alongside
# hollow_success (a no-write / no-tool / unverified turn is not done) nor
# while the build is red (a failed build is not done). Deriving done in
# either case is hollow success with a policy stamp on it.
#
# Execution is weaker than completion. Execution asks whether the turn really
# acted; verification asks whether the mechanical evidence THIS intent requires
# came back affirmative. They are separate questions about the same turn, which
# is why turn_verified does not carry turn_executed in its body — conjoining
# them is turn_done's job and nothing else's.
turn_executed(Turn) :- turn_evidence(Turn, _, _, _, _, _, _), !has_hollow_success(Turn), !turn_build_red(Turn).
turn_done(Turn) :- turn_executed(Turn), turn_verified(Turn).

# ONE host emits verification, through two mechanisms. This block used to say
# "only the host verifier emits acceptance", and that is still true — it is just
# not the only thing the host verifies:
#
#   1. turn_acceptance — the acceptance transaction: an immutable caller
#      contract with current, executed behavioral witnesses. Strongest, and
#      still the only thing that can speak for REQUESTED BEHAVIOUR.
#   2. turn_gate — the session executor's own post-edit gates for THIS turn
#      (recordBuildState, from BuildCheck/TestCheck/VetCheck). These are not
#      the model's word for anything: the executor ran the compiler and the
#      test runner itself and recorded what they returned, and only an
#      affirmative verdict is ever asserted. A skipped or indeterminate gate
#      asserts nothing.
#
# So the evidence arms below do not weaken the contract path. They are the same
# host reporting what it mechanically measured, rather than what it was asked to
# prove — a weaker claim about the workspace, not a weaker claim about who is
# entitled to make it. The model can reach neither: turn_gate, build_state,
# test_state and every predicate in this block are hard-blocked in
# core.FilterMangleUpdates (predicateAllowed), ahead of any caller allowlist.
#
# The write arm reads this turn's gates and nothing older: a red test_state
# left in the session by another producer does not block a turn whose own gate
# ran green after the edit, because the gate is the fresher measurement; and a
# green one left behind does not verify a turn whose own gate was skipped. The
# arm is guarded on both sides (green present, red absent) so a turn that
# somehow recorded both is not verified on the strength of the green.
#
# A turn that changed nothing has no workspace claim to verify, so execution is
# the whole of what it can owe. A turn that wrote owes both gates green: the
# fact space records turn_created_source (files CREATED this turn) but has no
# evidence predicate for a file MODIFIED this turn, so the corpus cannot tell a
# markdown write from a Go one. Owing both gates is the cautious direction to be
# wrong in, which is the same argument test_coverage makes at the top of this
# file.
turn_verified(Turn) :- turn_evidence(Turn, _, _, _, _, _, _), !turn_wrote(Turn).
turn_verified(Turn) :- turn_evidence(Turn, _, _, _, _, _, _), turn_wrote(Turn), turn_build_green(Turn), turn_tests_green(Turn), !turn_build_red(Turn), !turn_tests_red(Turn), !turn_has_untested(Turn), !turn_has_uncovered(Turn), !turn_vet_red(Turn).
turn_verified(Turn) :- turn_evidence(Turn, _, _, _, _, _, _), has_turn_acceptance(Turn).

# has_turn_acceptance projects turn_acceptance/3 to a single argument, matching
# has_turn_tools / has_turn_write / has_turn_test above. The arm that uses it
# reads positively, where turn_acceptance(Turn, _, _) would also have worked;
# the projection is here so that a future rule can NEGATE the question safely.
# Negating the 3-ary literal directly (!turn_acceptance(Turn, _, _)) does NOT
# exclude on this engine — the wildcard negation trap documented in
# internal/mangle/agents.md — and would fail silently, deriving nothing and
# erroring nowhere. The single-argument form is the one that can be negated.
has_turn_acceptance(Turn) :- turn_acceptance(Turn, _, _).

# turn_wrote is "this turn made a claim about the workspace". The second arm
# exists for dream mode: every hollow_success rule is guarded by DreamMode
# /false, so a dream-mode /create that wrote nothing derives no hollow_success
# and reaches turn_executed. With only the write-count arm it would then look
# like a read-only turn and verify for free.
turn_wrote(Turn) :- has_turn_write(Turn).
turn_wrote(Turn) :- turn_evidence(Turn, Verb, _, _, _, _, _), write_oriented_intent(Verb).

# turn_unverified and turn_missing_evidence exist so the turn's outcome can NAME
# what is missing rather than Go guessing at it. turn_unverified never derives
# for a turn that wrote nothing (those verify), so no spurious reason is
# produced for a read-only turn.
turn_unverified(Turn) :- turn_executed(Turn), !turn_verified(Turn).
turn_missing_evidence(Turn, /build_not_green) :- turn_unverified(Turn), !turn_build_green(Turn).
turn_missing_evidence(Turn, /tests_not_green) :- turn_unverified(Turn), !turn_tests_green(Turn).
turn_missing_evidence(Turn, /tests_not_written) :- turn_unverified(Turn), turn_has_untested(Turn).
turn_missing_evidence(Turn, /changed_code_unexecuted) :- turn_unverified(Turn), turn_has_uncovered(Turn).
turn_missing_evidence(Turn, /vet_not_clean) :- turn_unverified(Turn), turn_vet_red(Turn).

# A red build is a failed turn, not merely an unverified one. turn_executed
# already excludes it; this names it so the outcome can say /failed instead of
# leaving Go to re-check BuildCheck itself.
turn_build_failed(Turn) :- turn_evidence(Turn, _, _, _, _, _, _), turn_build_red(Turn).
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
