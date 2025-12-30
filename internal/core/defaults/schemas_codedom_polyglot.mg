# Cortex 1.5.0 Schemas (EDB Declarations)
# Version: 1.5.0
# Philosophy: Logic determines Reality; the Model merely describes it.

# Modular Schema: CODEDOM_POLYGLOT
# Sections: 6
# Purpose: Language-specific Stratum 0 facts for polyglot Code DOM support.
# These predicates are emitted by language-specific parsers and normalized
# into semantic archetypes by bridge rules (Stratum 1).

# =============================================================================
# SECTION 1: GO-SPECIFIC PREDICATES (Stratum 0)
# =============================================================================
# Emitted by internal/world/go_parser.go via EmitLanguageFacts()

# go_struct(Ref) - Go struct type declaration
Decl go_struct(Ref).

# go_interface(Ref) - Go interface type declaration
Decl go_interface(Ref).

# go_tag(Ref, TagContent) - Struct field tag (for wire name extraction)
# Example: go_tag("struct:user.User", `json:"user_id" db:"user_id"`)
Decl go_tag(Ref, TagContent).

# go_goroutine(Ref) - Function spawns goroutines
Decl go_goroutine(Ref).

# go_uses_context(Ref) - Function uses context.Context parameter
Decl go_uses_context(Ref).

# go_returns_error(Ref) - Function returns error type
Decl go_returns_error(Ref).


# =============================================================================
# SECTION 2: PYTHON-SPECIFIC PREDICATES (Stratum 0)
# =============================================================================
# Emitted by internal/world/python_parser.go via EmitLanguageFacts()

# py_class(Ref) - Python class definition
Decl py_class(Ref).

# py_decorator(Ref, DecoratorName) - Decorator applied to function/class
# Example: py_decorator("py:user.py:User.login", "login_required")
Decl py_decorator(Ref, DecoratorName).

# py_async_def(Ref) - Async function/method
Decl py_async_def(Ref).

# has_pydantic_base(Ref) - Class inherits from pydantic BaseModel
Decl has_pydantic_base(Ref).

# py_typed_function(Ref) - Function has return type annotation
Decl py_typed_function(Ref).


# =============================================================================
# SECTION 3: TYPESCRIPT/JAVASCRIPT-SPECIFIC PREDICATES (Stratum 0)
# =============================================================================
# Emitted by internal/world/typescript_parser.go via EmitLanguageFacts()

# ts_class(Ref) - TypeScript class declaration
Decl ts_class(Ref).

# ts_interface(Ref) - TypeScript interface declaration
Decl ts_interface(Ref).

# ts_interface_prop(Ref, PropName) - Interface property (for wire name extraction)
# Example: ts_interface_prop("ts:types.ts:IUser", "userId")
Decl ts_interface_prop(Ref, PropName).

# ts_type_alias(Ref) - TypeScript type alias
Decl ts_type_alias(Ref).

# ts_async_function(Ref) - Async function
Decl ts_async_function(Ref).

# ts_component(Ref, ComponentName) - React/Vue component
# Example: ts_component("ts:App.tsx:UserProfile", "UserProfile")
Decl ts_component(Ref, ComponentName).

# ts_hook(Ref, HookName) - React hook usage
# Example: ts_hook("ts:App.tsx:UserProfile", "useState")
Decl ts_hook(Ref, HookName).

# ts_extends(Ref) - Class extends another class
Decl ts_extends(Ref).

# ts_implements(Ref) - Class implements interface
Decl ts_implements(Ref).


# =============================================================================
# SECTION 4: RUST-SPECIFIC PREDICATES (Stratum 0)
# =============================================================================
# Emitted by internal/world/rust_parser.go via EmitLanguageFacts()

# rs_struct(Ref) - Rust struct declaration
Decl rs_struct(Ref).

# rs_trait(Ref) - Rust trait declaration
Decl rs_trait(Ref).

# rs_async_fn(Ref) - Async function
Decl rs_async_fn(Ref).

# rs_unsafe_block(Ref) - Contains unsafe block
Decl rs_unsafe_block(Ref).

# rs_returns_result(Ref) - Function returns Result<T, E>
Decl rs_returns_result(Ref).

# rs_uses_unwrap(Ref) - Uses .unwrap() or .expect() (potential panic)
Decl rs_uses_unwrap(Ref).

# rs_derive(Ref, DeriveName) - Derive macro applied
# Example: rs_derive("rs:lib.rs:Config", "Serialize")
Decl rs_derive(Ref, DeriveName).

# rs_serde_rename(Ref, FieldName, WireName) - Serde field rename (for wire name extraction)
# Example: rs_serde_rename("rs:lib.rs:Config", "user_id", "userId")
Decl rs_serde_rename(Ref, FieldName, WireName).


# =============================================================================
# SECTION 5: MANGLE-SPECIFIC PREDICATES (Stratum 0)
# =============================================================================
# Emitted by internal/world/mangle_parser.go via EmitLanguageFacts()

# mg_decl(Ref, PredicateName) - Mangle declaration
Decl mg_decl(Ref, PredicateName).

# mg_rule(Ref, HeadPredicate) - Mangle rule
Decl mg_rule(Ref, HeadPredicate).

# mg_fact(Ref, PredicateName) - Mangle ground fact
Decl mg_fact(Ref, PredicateName).

# mg_query(Ref, PredicateName) - Mangle query
Decl mg_query(Ref, PredicateName).

# mg_recursive_rule(Ref) - Rule is recursive
Decl mg_recursive_rule(Ref).

# mg_negation_rule(Ref) - Rule contains negation
Decl mg_negation_rule(Ref).

# mg_aggregation_rule(Ref) - Rule contains aggregation
Decl mg_aggregation_rule(Ref).


# =============================================================================
# SECTION 6: CROSS-LANGUAGE DERIVED PREDICATES (IDB/Stratum 1)
# =============================================================================
# These are semantic archetypes derived from language-specific facts.
# Implementation in internal/core/defaults/policy/bridge.mg

# is_data_contract(Ref) - Unified data contract archetype
# True for: go_struct, py_class+pydantic, ts_interface, rs_struct+serde
Decl is_data_contract(Ref).

# is_async_context(Ref) - Unified async context archetype
# True for: go_goroutine, py_async_def, ts_async_function, rs_async_fn
Decl is_async_context(Ref).

# wire_name(Ref, Name) - API wire protocol field name
# Extracted from: go_tag json, py field alias, ts interface prop, rs serde rename
Decl wire_name(Ref, Name).

# api_dependency(BackendRef, FrontendRef) - Cross-language API coupling
# Derived when BackendRef and FrontendRef share wire_name
Decl api_dependency(BackendRef, FrontendRef).

# is_ui_component(Ref) - UI component archetype
# True for: ts_component, vue_component, etc.
Decl is_ui_component(Ref).

# has_auth_guard(Ref) - Element has authentication protection
# True for: py_decorator with login_required, etc.
Decl has_auth_guard(Ref).

# potential_panic(Ref) - Element may panic at runtime
# True for: rs_uses_unwrap, go without error handling, etc.
Decl potential_panic(Ref).

# has_test_coverage(Ref) - Element has associated tests
Decl has_test_coverage(Ref).

# cross_lang_refactor_target(Ref) - Element is target of cross-language refactoring
Decl cross_lang_refactor_target(Ref).


# =============================================================================
# SECTION 6: TEST IMPACT PREDICATES
# =============================================================================
# Predicates for test impact analysis (asserted by Go code or derived in policy).

# file_imports(Importer, Imported) - File imports another file
Decl file_imports(Importer, Imported).

# type_embeds(Type, EmbeddedType) - Type embeds another type (Go struct embedding)
Decl type_embeds(Type, EmbeddedType).

# plan_edit(Ref) - Element is planned for editing
Decl plan_edit(Ref).

# modified_file(File) - File has been modified
Decl modified_file(File).

