# Code DOM Edit Logic
# Editing safety, next actions, and learning

# --- Next Action Derivation for Code DOM ---

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

# --- Edit Safety & Risk Assessment Rules ---

# Edit unsafe: generated code will be overwritten
edit_unsafe(Ref, /generated_code) :-
    code_element(Ref, _, File, _, _),
    generated_code(File, _, _).

# Edit unsafe: CGo requires special handling
edit_unsafe(Ref, /cgo_code) :-
    code_element(Ref, _, File, _, _),
    cgo_code(File).

# Edit unsafe: file has hash mismatch (concurrent modification)
edit_unsafe(Ref, /concurrent_modification) :-
    code_element(Ref, _, File, _, _),
    file_hash_mismatch(File, _, _).

# Edit unsafe: element is stale
edit_unsafe(Ref, /stale_reference) :-
    element_stale(Ref, _).

# Element edit blocked due to concurrent modification
element_edit_blocked(Ref, /concurrent_modification) :-
    code_element(Ref, _, File, _, _),
    file_modified_externally(File).

# Element edit blocked due to generated code
element_edit_blocked(Ref, /generated_code) :-
    code_element(Ref, _, File, _, _),
    generated_code(File, _, _).

# Element edit blocked due to parse error
element_edit_blocked(Ref, /parse_error) :-
    code_element(Ref, _, File, _, _),
    parse_error(File, _, _).

# --- Breaking Change Risk Analysis ---

# Public function has external callers (exported = can be called from outside)
has_external_callers(Ref) :-
    code_element(Ref, /function, _, _, _),
    element_visibility(Ref, /public).

has_external_callers(Ref) :-
    code_element(Ref, /method, _, _, _),
    element_visibility(Ref, /public).

# Breaking change risk: HIGH for public API functions
breaking_change_risk(Ref, /high, /public_api) :-
    has_external_callers(Ref),
    api_handler_function(Ref, _, _).

# Breaking change risk: HIGH for public interface methods
breaking_change_risk(Ref, /high, /interface_contract) :-
    code_element(Ref, /method, _, _, _),
    element_visibility(Ref, /public),
    element_parent(Ref, InterfaceRef),
    code_element(InterfaceRef, /interface, _, _, _).

# Breaking change risk: MEDIUM for public functions in libraries
breaking_change_risk(Ref, /medium, /public_function) :-
    has_external_callers(Ref),
    !api_handler_function(Ref, _, _).

# Breaking change risk: LOW for private elements
breaking_change_risk(Ref, /low, /private) :-
    code_element(Ref, _, _, _, _),
    element_visibility(Ref, /private).

# Breaking change risk: CRITICAL for generated code
breaking_change_risk(Ref, /critical, /generated_will_be_overwritten) :-
    code_element(Ref, _, File, _, _),
    generated_code(File, _, _).

# --- API Client/Handler Awareness ---

# API client functions should be tested carefully
requires_integration_test(Ref) :-
    api_client_function(Ref, _, _).

# API handlers need contract validation
requires_contract_check(Ref) :-
    api_handler_function(Ref, _, _),
    element_modified(Ref, _, _).

# Warn when editing API client without corresponding test
api_edit_warning(Ref, /no_integration_test) :-
    api_client_function(Ref, _, _),
    element_modified(Ref, _, _).

# --- Safety and Complexity Rules ---

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

# --- Learning From Code Edits ---

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

# --- Code DOM Activation Rules ---

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
