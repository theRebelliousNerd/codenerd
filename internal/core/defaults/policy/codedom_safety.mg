# CodeDOM Polyglot Safety Guardrails
# Version: 1.0.0
# Purpose: Block dangerous edits based on semantic analysis
#
# Philosophy: Safety is not a prompt instruction but a logic rule.
# The Constitutional Gate blocks edits that violate safety constraints.
#
# This file implements Stratum 2 safety rules that derive deny_edit facts
# when edits violate language-specific or cross-language safety constraints.

# =============================================================================
# SECTION 1: SCHEMA DECLARATIONS
# =============================================================================
# OUTPUT predicates derived in this file (not declared elsewhere).
# Input predicates are declared in schemas_codedom.mg and schemas_codedom_polyglot.mg.

# Output predicates (derived by rules in this file)
Decl deny_edit(Ref, Reason).
Decl edit_warning(Ref, Reason).
Decl safe_to_edit(Ref).
Decl has_warnings(Ref).
Decl has_deny_edit(Ref).
Decl blocked_action(Action, Reason).

# Input predicates unique to this file (not in schemas_*.mg)
Decl is_serialization_boundary(Ref).
Decl returns_error_type(Ref).
Decl element_action(Action, Ref).


# =============================================================================
# SECTION 2: GO SAFETY RULES
# =============================================================================

# Go: Goroutine leak detection
# Block edits that remove context cancellation from async code
deny_edit(Ref, /goroutine_leak_risk) :-
    go_goroutine(Ref),
    is_async_context(Ref),
    !go_uses_context(Ref).


# =============================================================================
# SECTION 3: PYTHON SAFETY RULES
# =============================================================================

# Python: Auth decorator removal detection
# Block removal of authentication decorators
deny_edit(Ref, /auth_removed) :-
    code_element(Ref, _, _, _, _),
    has_auth_guard(Ref),
    element_modified(Ref, _, _),
    !has_auth_guard(Ref).

# Python: Type annotation removal
# Warn when removing type hints from previously typed functions
edit_warning(Ref, /type_annotation_removed) :-
    py_typed_function(Ref),
    element_modified(Ref, _, _).


# =============================================================================
# SECTION 4: RUST SAFETY RULES
# =============================================================================

# Rust: Unsafe block introduction
# Warn when adding unsafe blocks
edit_warning(Ref, /unsafe_introduced) :-
    rs_unsafe_block(Ref),
    element_modified(Ref, _, _).

# Rust: .unwrap() usage
# Warn when using .unwrap() which can panic
edit_warning(Ref, /unwrap_usage) :-
    rs_uses_unwrap(Ref).


# =============================================================================
# SECTION 5: TYPESCRIPT SAFETY RULES
# =============================================================================

# TypeScript: Hook rules violation
# React hooks must follow rules (called at top level, not in conditions)
# This is detected at runtime, but we can flag patterns

# TypeScript: Interface breaking change
# Warn when modifying interfaces that have API dependencies
edit_warning(Ref, /interface_breaking_change) :-
    ts_interface(Ref),
    api_dependency(_, Ref),
    element_modified(Ref, _, _).


# =============================================================================
# SECTION 6: CROSS-LANGUAGE SAFETY RULES
# =============================================================================

# Cross-language: Wire name consistency
# Block edits that break API contracts between backend and frontend
deny_edit(Ref, /api_contract_violation) :-
    is_serialization_boundary(Ref),
    cross_lang_refactor_target(Ref),
    element_modified(Ref, _, _).

# Cross-language: Data contract modification
# Warn when modifying data contracts that are serialization boundaries
edit_warning(Ref, /data_contract_change) :-
    is_data_contract(Ref),
    is_serialization_boundary(Ref),
    element_modified(Ref, _, _).


# =============================================================================
# SECTION 7: GENERATED CODE SAFETY
# =============================================================================

# Block editing generated code
deny_edit(Ref, /generated_code_readonly) :-
    code_element(Ref, _, File, _, _),
    generated_code(File, _, _).


# =============================================================================
# SECTION 8: TEST COVERAGE SAFETY
# =============================================================================

# Warn when editing public functions without test coverage
edit_warning(Ref, /no_test_coverage) :-
    code_element(Ref, /function, _, _, _),
    element_visibility(Ref, /public),
    element_modified(Ref, _, _),
    !has_test_coverage(Ref).

# Warn when editing public methods without test coverage
edit_warning(Ref, /no_test_coverage) :-
    code_element(Ref, /method, _, _, _),
    element_visibility(Ref, /public),
    element_modified(Ref, _, _),
    !has_test_coverage(Ref).


# =============================================================================
# SECTION 9: MANGLE SAFETY RULES
# =============================================================================

# Mangle: Stratification violation risk
# Warn when editing rules with negation (risk of stratification issues)
edit_warning(Ref, /stratification_risk) :-
    mg_negation_rule(Ref),
    element_modified(Ref, _, _).

# Mangle: Recursion termination risk
# Warn when editing recursive rules
edit_warning(Ref, /recursion_risk) :-
    mg_recursive_rule(Ref),
    element_modified(Ref, _, _).


# =============================================================================
# SECTION 10: ASYNC SAFETY
# =============================================================================

# Async: Missing error handling in async code
# Warn when async code doesn't return errors properly
edit_warning(Ref, /async_error_handling) :-
    is_async_context(Ref),
    !returns_error_type(Ref).


# =============================================================================
# SECTION 11: HELPER PREDICATES
# =============================================================================

# Helper: check if any deny_edit exists for a ref
# (needed because !deny_edit(Ref, _) has unbound variable in negation)
has_deny_edit(Ref) :- deny_edit(Ref, _).

# safe_to_edit is true when there are no deny_edit rules for the ref
safe_to_edit(Ref) :-
    code_element(Ref, _, _, _, _),
    !has_deny_edit(Ref).

# has_warnings is true when there are edit_warning rules for the ref
has_warnings(Ref) :-
    edit_warning(Ref, _).


# =============================================================================
# SECTION 12: CONSTITUTIONAL GATE INTEGRATION
# =============================================================================

# Block action if deny_edit is active
# (Integration with constitutional gate in policy/constitution.mg)
blocked_action(Action, /safety_violation) :-
    element_action(Action, Ref),
    deny_edit(Ref, _).

