# Bridge Rules: Stratum 1 Normalization Layer
# Version: 1.0.0
# Philosophy: Language-specific facts → Universal semantic archetypes
#
# This file implements the Stratified Bridge Pattern for polyglot Code DOM.
# Stratum 0 (EDB): Language-specific facts from parsers (go_struct, py_class, etc.)
# Stratum 1 (IDB): Semantic archetypes derived here (is_data_contract, wire_name, etc.)
#
# The bridge enables cross-language refactoring by mapping language idioms
# to universal concepts.

# =============================================================================
# SECTION 1: DATA CONTRACT ARCHETYPE
# =============================================================================
# A "data contract" is any type that represents a data structure used for
# serialization, API communication, or persistence.

# Go structs are data contracts
is_data_contract(Ref) :- go_struct(Ref).

# Python classes with pydantic base are data contracts
is_data_contract(Ref) :- py_class(Ref), has_pydantic_base(Ref).

# Python dataclasses are data contracts
is_data_contract(Ref) :- py_class(Ref), py_decorator(Ref, "dataclass").

# TypeScript interfaces are data contracts
is_data_contract(Ref) :- ts_interface(Ref).

# Rust structs with Serialize/Deserialize derive are data contracts
is_data_contract(Ref) :- rs_struct(Ref), rs_derive(Ref, "Serialize").
is_data_contract(Ref) :- rs_struct(Ref), rs_derive(Ref, "Deserialize").


# =============================================================================
# SECTION 2: ASYNC CONTEXT ARCHETYPE
# =============================================================================
# An "async context" is any function/method that involves concurrent execution.

# Go functions spawning goroutines
is_async_context(Ref) :- go_goroutine(Ref).

# Python async functions
is_async_context(Ref) :- py_async_def(Ref).

# TypeScript async functions
is_async_context(Ref) :- ts_async_function(Ref).

# Rust async functions
is_async_context(Ref) :- rs_async_fn(Ref).


# =============================================================================
# SECTION 3: UI COMPONENT ARCHETYPE
# =============================================================================
# A "UI component" is a function/class that renders UI elements.

# React/Vue components
is_ui_component(Ref) :- ts_component(Ref, _).


# =============================================================================
# SECTION 4: AUTHENTICATION GUARD ARCHETYPE
# =============================================================================
# An "auth guard" is a function protected by authentication.

# Python login_required decorator
has_auth_guard(Ref) :- py_decorator(Ref, "login_required").

# Python requires_auth decorator
has_auth_guard(Ref) :- py_decorator(Ref, "requires_auth").

# Python auth_required decorator
has_auth_guard(Ref) :- py_decorator(Ref, "auth_required").

# Python permission_required decorator
has_auth_guard(Ref) :- py_decorator(Ref, "permission_required").


# =============================================================================
# SECTION 5: POTENTIAL PANIC ARCHETYPE
# =============================================================================
# A function that may panic at runtime.

# Rust .unwrap() usage
potential_panic(Ref) :- rs_uses_unwrap(Ref).


# =============================================================================
# SECTION 6: ERROR HANDLING ARCHETYPE
# =============================================================================
# Functions that properly handle errors.

# Go functions returning error
returns_error_type(Ref) :- go_returns_error(Ref).

# Rust functions returning Result
returns_error_type(Ref) :- rs_returns_result(Ref).


# =============================================================================
# SECTION 7: CONTEXT AWARE ARCHETYPE
# =============================================================================
# Functions that properly use context for cancellation/timeout.

# Go functions using context.Context
is_context_aware(Ref) :- go_uses_context(Ref).


# =============================================================================
# SECTION 8: WIRE NAME EXTRACTION (Cross-Language API Coupling)
# =============================================================================
# Wire names are the field names used in serialized form (JSON, protobuf, etc.).
# This enables detecting when a backend change affects frontend code.

# Go: Extract json tag from struct tags
# Example: `json:"user_id"` -> wire_name(Ref, "user_id")
# Note: Full regex extraction would require fn:match support.
# For now, we emit wire_name facts directly from the parser.

# TypeScript: Interface property names are wire names
wire_name(Ref, PropName) :- ts_interface_prop(Ref, PropName).

# Rust: Serde rename attributes provide wire names
wire_name(Ref, WireName) :- rs_serde_rename(Ref, _, WireName).


# =============================================================================
# SECTION 9: API DEPENDENCY INFERENCE
# =============================================================================
# Detect when backend and frontend types share the same wire names,
# indicating an API coupling that must be maintained during refactoring.

# Two refs have API dependency if they share a wire name
# and one is backend (go, python, rust) and one is frontend (ts)
api_dependency(BackendRef, FrontendRef) :-
    wire_name(BackendRef, Key),
    wire_name(FrontendRef, Key),
    is_backend_ref(BackendRef),
    is_frontend_ref(FrontendRef).

# Helper: Identify backend refs by language prefix
is_backend_ref(Ref) :- go_struct(Ref).
is_backend_ref(Ref) :- py_class(Ref).
is_backend_ref(Ref) :- rs_struct(Ref).

# Helper: Identify frontend refs by language prefix
is_frontend_ref(Ref) :- ts_interface(Ref).
is_frontend_ref(Ref) :- ts_class(Ref).


# =============================================================================
# SECTION 10: CROSS-LANGUAGE REFACTORING TARGETS
# =============================================================================
# Mark elements that require multi-language coordination when refactored.

# If element has API dependency, it's a cross-language refactor target
cross_lang_refactor_target(Ref) :- api_dependency(Ref, _).
cross_lang_refactor_target(Ref) :- api_dependency(_, Ref).


# =============================================================================
# SECTION 11: HOOK PATTERN DETECTION
# =============================================================================
# React hook patterns for component analysis.

# Component uses state management
uses_state_management(Ref) :- ts_hook(Ref, "useState").
uses_state_management(Ref) :- ts_hook(Ref, "useReducer").

# Component uses side effects
uses_side_effects(Ref) :- ts_hook(Ref, "useEffect").
uses_side_effects(Ref) :- ts_hook(Ref, "useLayoutEffect").

# Component uses memoization
uses_memoization(Ref) :- ts_hook(Ref, "useMemo").
uses_memoization(Ref) :- ts_hook(Ref, "useCallback").


# =============================================================================
# SECTION 12: MANGLE CODE QUALITY
# =============================================================================
# Rules for analyzing Mangle code quality.

# Mangle code with recursion needs careful review
complex_mangle_rule(Ref) :- mg_recursive_rule(Ref).

# Mangle code with negation needs stratification check
complex_mangle_rule(Ref) :- mg_negation_rule(Ref).

# Mangle code with aggregation needs performance review
complex_mangle_rule(Ref) :- mg_aggregation_rule(Ref).


# =============================================================================
# SECTION 13: DERIVED PREDICATES FOR IMPACT ANALYSIS
# =============================================================================

# Element is a serialization boundary (affects external APIs)
is_serialization_boundary(Ref) :- is_data_contract(Ref), wire_name(Ref, _).

# Element change may affect multiple languages
multi_lang_impact(Ref) :- cross_lang_refactor_target(Ref).

# Element requires careful async review
async_review_needed(Ref) :- is_async_context(Ref).

