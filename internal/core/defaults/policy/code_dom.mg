# Code DOM Rules
# Section 22 of Cortex Executive Policy

# EDB Declarations
Decl active_file(File).
Decl file_in_scope(File, Reason, Context, Timestamp).
Decl code_element(Ref, Type, File, Span, Extra).
Decl code_interactable(Ref, Action).
Decl element_signature(Ref, Sig).
Decl element_parent(Child, Parent).
Decl high_element_count_flag().
Decl user_intent(ID, Category, Action, Target, Params).
Decl file_topology(File, Lang, Type, IsTest, IsMock).
Decl modified(File).
Decl code_edit_outcome(Ref, EditType, Success, Timestamp).
Decl element_visibility(Ref, Visibility).
Decl api_handler_function(Ref, Method, Path).
Decl generated_code(File, Generator, Timestamp).
Decl cgo_code(File).
Decl file_hash_mismatch(File, Expected, Actual).
Decl element_stale(Ref, Reason).
Decl element_modified(Ref, Diff, Timestamp).
Decl parse_error(File, Line, Message).
Decl api_client_function(Ref, Service, Endpoint).
Decl shard_result(TaskID, Status, ShardType, Description, Meta).
Decl pending_test(TaskID, Description).
Decl pending_review(TaskID, Description).
Decl interrupt_requested().
Decl pending_clarification(Ref, Question, Context).
Decl continuation_step(Current, Max).
Decl max_continuation_steps(Limit).
Decl pending_subtask_count_computed(Count).
Decl copular_verb(Verb, Type, Priority).
Decl state_adjective(Adjective, ImpliedVerb, StateCategory, Priority).
Decl churn_rate(File, Rate).

# IDB Declarations
Decl in_scope(File).
Decl editable(Ref).
Decl function_in_scope(Ref, File, Sig).
Decl method_in_scope(Ref, File, Sig).
Decl method_of(MethodRef, StructRef).
Decl code_contains(Parent, Child).
Decl safe_to_modify(Ref).
Decl element_count_high().
Decl requires_campaign(Intent).
Decl next_action(Action).
Decl scope_refreshed(File).
Decl successful_edit(Ref, EditType).
Decl failed_edit(Ref, EditType).
Decl proven_safe_edit(Ref, EditType).
Decl promote_to_long_term(Category, Value).
Decl activation(Ref, Score).
Decl has_external_callers(Ref).
Decl breaking_change_risk(Ref, Level, Reason).
Decl edit_unsafe(Ref, Reason).
Decl interface_impl(StructRef, InterfaceRef).
Decl mock_file(TestFile, SourceFile).
Decl suggest_update_mocks(Ref).
Decl file_modified_externally(Path).
Decl needs_scope_refresh().
Decl element_edit_blocked(Ref, Reason).
Decl requires_integration_test(Ref).
Decl requires_contract_check(Ref).
Decl api_edit_warning(Ref, Reason).
Decl has_pending_subtask(TaskID, Description, ShardType).
Decl continuation_blocked(Reason).
Decl has_continuation_block().
Decl should_auto_continue().
Decl has_blocking_condition().
Decl pending_subtask_count(Count).


# File Scope Rules

# A file is in scope if it's the active file
in_scope(File) :- active_file(File).

# A file is in scope if Code DOM loaded it into scope.
in_scope(File) :-
    file_in_scope(File, _, _, _).

# Element Accessibility Rules

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

# Transitive Element Containment

# Direct containment
code_contains(Parent, Child) :- element_parent(Child, Parent).

# Transitive containment
code_contains(Ancestor, Descendant) :-
    element_parent(Mid, Ancestor),
    code_contains(Mid, Descendant).

# Safety and Complexity Rules

# Safe to modify: element is in scope (implicitly has context)
safe_to_modify(Ref) :-
    code_element(Ref, _, File, _, _),
    in_scope(File).

# Helper: check if element count is high for complexity analysis
element_count_high() :-
    high_element_count_flag().

# Trigger campaign for complex refactors affecting many elements
requires_campaign(/current_intent) :-
    user_intent(/current_intent, /mutation, _, Target, _),
    in_scope(Target),
    element_count_high().

# Next Action Derivation for Code DOM

# Open file when targeting a file that's not yet in scope
next_action(/open_file) :-
    user_intent(/current_intent, _, _, Target, _),
    file_topology(Target, _, /go, _, _),
    !active_file(Target),
    !in_scope(Target).

next_action(/open_file) :-
    user_intent(/current_intent, _, _, Target, _),
    file_topology(Target, _, /mangle, _, _),
    !active_file(Target),
    !in_scope(Target).

# Query elements when file is open and we need to find something
next_action(/query_elements) :-
    active_file(_),
    user_intent(/current_intent, /query, _, _, _).

# Edit element when mutation targets a known element
next_action(/edit_element) :-
    user_intent(/current_intent, /mutation, _, Ref, _),
    code_element(Ref, _, File, _, _),
    in_scope(File).

# Refresh scope after external changes
next_action(/refresh_scope) :-
    active_file(File),
    modified(File),
    !scope_refreshed(File).

# Helper for safe negation
scope_refreshed(File) :-
    file_in_scope(File, _, _, _),
    !modified(File).

# Learning From Code Edits

# Track edit outcomes for learning
successful_edit(Ref, EditType) :-
    code_edit_outcome(Ref, EditType, /true, _).

failed_edit(Ref, EditType) :-
    code_edit_outcome(Ref, EditType, /false, _).

# Proven safe: edit pattern has succeeded multiple times (3+)
proven_safe_edit(Ref, EditType) :-
    code_edit_outcome(Ref, EditType, /true, _),
    code_edit_outcome(Ref2, EditType, /true, _),
    code_edit_outcome(Ref3, EditType, /true, _),
    Ref != Ref2, Ref2 != Ref3, Ref != Ref3.

# Promote to long-term memory when edit pattern is proven
promote_to_long_term(/edit_pattern, EditType) :-
    proven_safe_edit(_, EditType).

# Code DOM Activation Rules

# Boost activation for elements in the active file
activation(Ref, 100) :-
    code_element(Ref, _, File, _, _),
    active_file(File).

# Boost activation for elements matching current intent target
activation(Ref, 120) :-
    code_element(Ref, _, _, _, _),
    user_intent(/current_intent, _, _, Ref, _).

# Suppress activation for elements in files outside scope
activation(Ref, -50) :-
    code_element(Ref, _, File, _, _),
    file_topology(File, _, _, _, _),
    !in_scope(File).

# Edit Safety & Risk Assessment Rules

# Public function has external callers (exported = can be called from outside)
has_external_callers(Ref) :-
    code_element(Ref, /function, _, _, _),
    element_visibility(Ref, /public).

has_external_callers(Ref) :-
    code_element(Ref, /method, _, _, _),
    element_visibility(Ref, /public).

# Breaking change risk: HIGH for public API functions
breaking_change_risk(Ref, /high, "public_api") :-
    has_external_callers(Ref),
    api_handler_function(Ref, _, _).

# Breaking change risk: HIGH for public interface methods
breaking_change_risk(Ref, /high, "interface_contract") :-
    code_element(Ref, /method, _, _, _),
    element_visibility(Ref, /public),
    element_parent(Ref, InterfaceRef),
    code_element(InterfaceRef, /interface, _, _, _).

# Breaking change risk: MEDIUM for public functions in libraries
breaking_change_risk(Ref, /medium, "public_function") :-
    has_external_callers(Ref),
    !api_handler_function(Ref, _, _).

# Breaking change risk: LOW for private elements
breaking_change_risk(Ref, /low, "private") :-
    code_element(Ref, _, _, _, _),
    element_visibility(Ref, /private).

# Breaking change risk: CRITICAL for generated code
breaking_change_risk(Ref, /critical, "generated_will_be_overwritten") :-
    code_element(Ref, _, File, _, _),
    generated_code(File, _, _).

# Edit unsafe: generated code will be overwritten
edit_unsafe(Ref, "generated_code") :-
    code_element(Ref, _, File, _, _),
    generated_code(File, _, _).

# Edit unsafe: CGo requires special handling
edit_unsafe(Ref, "cgo_code") :-
    code_element(Ref, _, File, _, _),
    cgo_code(File).

# Edit unsafe: file has hash mismatch (concurrent modification)
edit_unsafe(Ref, "concurrent_modification") :-
    code_element(Ref, _, File, _, _),
    file_hash_mismatch(File, _, _).

# Edit unsafe: element is stale
edit_unsafe(Ref, "stale_reference") :-
    element_stale(Ref, _).

# Mock & Interface Rules

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

# Scope Staleness Detection

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

# Element edit blocked due to concurrent modification
element_edit_blocked(Ref, "concurrent_modification") :-
    code_element(Ref, _, File, _, _),
    file_modified_externally(File).

# Element edit blocked due to generated code
element_edit_blocked(Ref, "generated_code") :-
    code_element(Ref, _, File, _, _),
    generated_code(File, _, _).

# Element edit blocked due to parse error
element_edit_blocked(Ref, "parse_error") :-
    code_element(Ref, _, File, _, _),
    parse_error(File, _, _).

# API Client/Handler Awareness

# API client functions should be tested carefully
requires_integration_test(Ref) :-
    api_client_function(Ref, _, _).

# API handlers need contract validation
requires_contract_check(Ref) :-
    api_handler_function(Ref, _, _),
    element_modified(Ref, _, _).

# Warn when editing API client without corresponding test
api_edit_warning(Ref, "no_integration_test") :-
    api_client_function(Ref, _, _),
    element_modified(Ref, _, _).

# Continuation Protocol (Multi-Step Task Execution)

# Pending Subtask Detection

# Code was generated but no tests exist → need tests
has_pending_subtask(TaskID, Description, /tester) :-
    shard_result(_, /code_generated, /coder, Task, _),
    pending_test(TaskID, Description).

# Changes were made but not reviewed → need review
has_pending_subtask(TaskID, Description, /reviewer) :-
    shard_result(_, /code_generated, /coder, Task, _),
    pending_review(TaskID, Description).

# Shard execution was incomplete → continue with same shard
has_pending_subtask(TaskID, Description, ShardType) :-
    shard_result(TaskID, /incomplete, ShardType, Description, _).

# Tests needed status → trigger tester
has_pending_subtask(TaskID, Description, /tester) :-
    shard_result(TaskID, /tests_needed, _, Description, _).

# Review needed status → trigger reviewer
has_pending_subtask(TaskID, Description, /reviewer) :-
    shard_result(TaskID, /review_needed, _, Description, _).

# Continuation Blocking Conditions

# User pressed Ctrl+X
continuation_blocked(/user_interrupted) :-
    interrupt_requested().

# Clarification is pending
continuation_blocked(/needs_clarification) :-
    pending_clarification(_, _, _).

# Max steps reached (safety limit)
continuation_blocked(/max_steps_reached) :-
    continuation_step(Current, _),
    max_continuation_steps(Limit),
    Current >= Limit.

# Auto-Continue Signal

# Helper: check if any blocking condition exists
has_continuation_block() :-
    continuation_blocked(_).

# Should continue if there's pending work and not blocked
should_auto_continue() :-
    has_pending_subtask(_, _, _),
    !has_continuation_block().

# Step Counting Helpers

# Helper: check if we have any blocking condition
has_blocking_condition() :-
    continuation_blocked(_).

# Helper: count pending subtasks (for progress display)
pending_subtask_count(Count) :-
    pending_subtask_count_computed(Count).
