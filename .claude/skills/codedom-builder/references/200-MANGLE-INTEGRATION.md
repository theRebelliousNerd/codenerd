# 200: Mangle Integration for Code DOM

## The Stratified Schema Architecture

Code DOM uses a **Stratified Bridge** pattern for Mangle integration:

```text
┌─────────────────────────────────────────────────────────────┐
│ STRATUM 0: Language-Specific Facts (EDB)                    │
│   High fidelity, raw facts from parsers                     │
│   py_class, go_struct, ts_interface, rs_impl, kt_data_class │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│ STRATUM 1: Semantic Bridge (IDB)                            │
│   Universal archetypes for cross-language reasoning         │
│   is_data_contract, is_async_context, wire_name             │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│ STRATUM 2: Safety Guardrails (IDB)                          │
│   Language-specific safety rules                            │
│   deny_edit, security_regression, goroutine_leak            │
└─────────────────────────────────────────────────────────────┘
```

## Stratum 0: Language-Specific Schema

### Base Code Element Facts

```mangle
# =============================================================================
# BASE CODE DOM SCHEMA (All Languages)
# =============================================================================

# Core element fact
Decl code_element(
    Ref.Type<string>,
    ElemType.Type<n>,      # /function, /method, /struct, etc.
    File.Type<string>,
    StartLine.Type<int>,
    EndLine.Type<int>
).

# Element metadata
Decl element_signature(Ref.Type<string>, Signature.Type<string>).
Decl element_body(Ref.Type<string>, Body.Type<string>).
Decl element_parent(Ref.Type<string>, ParentRef.Type<string>).
Decl element_visibility(Ref.Type<string>, Visibility.Type<n>).
Decl element_package(Ref.Type<string>, Package.Type<string>).

# Interactable actions
Decl code_interactable(Ref.Type<string>, ActionType.Type<n>).
```

### Python-Specific Facts

```mangle
# =============================================================================
# PYTHON STRATUM 0
# =============================================================================

Decl py_class(Ref.Type<string>).
Decl py_function(Ref.Type<string>).
Decl py_method(Ref.Type<string>).
Decl py_async_def(Ref.Type<string>).
Decl py_decorator(Ref.Type<string>, DecoratorName.Type<string>).
Decl py_base_class(Ref.Type<string>, BaseClassName.Type<string>).
Decl py_field_alias(Ref.Type<string>, AliasName.Type<string>).
Decl py_import(File.Type<string>, ModuleName.Type<string>).
```

### Go-Specific Facts

```mangle
# =============================================================================
# GO STRATUM 0
# =============================================================================

Decl go_struct(Ref.Type<string>).
Decl go_interface(Ref.Type<string>).
Decl go_function(Ref.Type<string>).
Decl go_method(Ref.Type<string>).
Decl go_receiver(Ref.Type<string>, ReceiverType.Type<string>).
Decl go_tag(Ref.Type<string>, TagContent.Type<string>).
Decl go_goroutine(Ref.Type<string>).
Decl go_channel_send(Ref.Type<string>, ChanVar.Type<string>).
Decl go_channel_recv(Ref.Type<string>, ChanVar.Type<string>).
Decl go_defer(Ref.Type<string>).
Decl go_embed_directive(File.Type<string>, EmbedPath.Type<string>).
Decl go_build_tag(File.Type<string>, Tag.Type<string>).
```

### TypeScript-Specific Facts

```mangle
# =============================================================================
# TYPESCRIPT STRATUM 0
# =============================================================================

Decl ts_interface(Ref.Type<string>).
Decl ts_type_alias(Ref.Type<string>).
Decl ts_class(Ref.Type<string>).
Decl ts_function(Ref.Type<string>).
Decl ts_arrow_function(Ref.Type<string>).
Decl ts_component(Ref.Type<string>, ComponentName.Type<string>).
Decl ts_hook(Ref.Type<string>, HookName.Type<string>).
Decl ts_hook_dep(Ref.Type<string>, DepVar.Type<string>).
Decl ts_hook_reads(Ref.Type<string>, Var.Type<string>).
Decl ts_interface_prop(Ref.Type<string>, PropName.Type<string>).
Decl ts_export(Ref.Type<string>).
```

### Rust-Specific Facts

```mangle
# =============================================================================
# RUST STRATUM 0
# =============================================================================

Decl rs_struct(Ref.Type<string>).
Decl rs_enum(Ref.Type<string>).
Decl rs_trait(Ref.Type<string>).
Decl rs_impl(Ref.Type<string>, ForType.Type<string>).
Decl rs_trait_impl(Ref.Type<string>, Trait.Type<string>, ForType.Type<string>).
Decl rs_function(Ref.Type<string>).
Decl rs_async_fn(Ref.Type<string>).
Decl rs_unsafe_block(Ref.Type<string>).
Decl rs_unsafe_fn(Ref.Type<string>).
Decl rs_derive(Ref.Type<string>, DeriveName.Type<string>).
Decl rs_lifetime(Ref.Type<string>, LifetimeName.Type<string>).
Decl rs_mutex_guard(Ref.Type<string>, Var.Type<string>).
Decl rs_await_point(Ref.Type<string>).
```

### Kotlin-Specific Facts

```mangle
# =============================================================================
# KOTLIN STRATUM 0
# =============================================================================

Decl kt_class(Ref.Type<string>).
Decl kt_data_class(Ref.Type<string>).
Decl kt_object(Ref.Type<string>).
Decl kt_interface(Ref.Type<string>).
Decl kt_function(Ref.Type<string>).
Decl kt_suspend_fun(Ref.Type<string>).
Decl kt_annotation(Ref.Type<string>, AnnotationName.Type<string>, Value.Type<string>).
Decl kt_force_unwrap(Ref.Type<string>).
Decl kt_nullable(Ref.Type<string>, Var.Type<string>).
```

## Stratum 1: Semantic Bridge Rules

### Data Contract Archetype

```mangle
# =============================================================================
# SEMANTIC BRIDGE: Data Contracts
# Any type that defines a data shape (struct, class, interface)
# =============================================================================

is_data_contract(Ref) :- go_struct(Ref).
is_data_contract(Ref) :- rs_struct(Ref).
is_data_contract(Ref) :- kt_data_class(Ref).
is_data_contract(Ref) :- ts_interface(Ref).
is_data_contract(Ref) :- py_class(Ref), has_pydantic_base(Ref).
is_data_contract(Ref) :- py_class(Ref), py_decorator(Ref, "dataclass").

# Pydantic detection
has_pydantic_base(Ref) :- py_base_class(Ref, "BaseModel").
has_pydantic_base(Ref) :- py_base_class(Ref, "pydantic.BaseModel").
```

### Async Context Archetype

```mangle
# =============================================================================
# SEMANTIC BRIDGE: Async Contexts
# Where concurrency and race conditions can occur
# =============================================================================

is_async_context(Ref) :- go_goroutine(Ref).
is_async_context(Ref) :- py_async_def(Ref).
is_async_context(Ref) :- rs_async_fn(Ref).
is_async_context(Ref) :- kt_suspend_fun(Ref).
is_async_context(Ref) :- ts_function(Ref), contains_await(Ref).

contains_await(Ref) :- element_body(Ref, Body), fn:contains(Body, "await").
```

### Wire Name Protocol (Cross-Language API)

```mangle
# =============================================================================
# SEMANTIC BRIDGE: Wire Names
# Extract the "wire name" (JSON field name) from various annotation styles
# =============================================================================

# Go: `json:"user_id"`
wire_name(Ref, Name) :-
    go_tag(Ref, TagContent),
    extract_json_tag(TagContent, Name).

# Helper: extract from json:"name" or json:"name,omitempty"
extract_json_tag(Tag, Name) :-
    :match_field(Tag, /json, JsonPart),
    fn:split(JsonPart, ",", Parts),
    fn:list:get(Parts, 0, Name),
    Name != "",
    Name != "-".

# Python: Field(alias="user_id")
wire_name(Ref, Name) :- py_field_alias(Ref, Name).

# Kotlin: @SerializedName("user_id")
wire_name(Ref, Name) :- kt_annotation(Ref, "SerializedName", Name).

# TypeScript: interface property name (direct match)
wire_name(Ref, Name) :- ts_interface_prop(Ref, Name).

# =============================================================================
# API DEPENDENCY INFERENCE
# If two elements share a wire name, they are API-coupled
# =============================================================================

api_dependency(BackendRef, FrontendRef) :-
    wire_name(BackendRef, Key),
    wire_name(FrontendRef, Key),
    is_backend_ref(BackendRef),
    is_frontend_ref(FrontendRef).

# Heuristic: backend vs frontend based on path
is_backend_ref(Ref) :- code_element(Ref, _, File, _, _), fn:contains(File, "backend").
is_backend_ref(Ref) :- code_element(Ref, _, File, _, _), fn:contains(File, "server").
is_backend_ref(Ref) :- code_element(Ref, _, File, _, _), fn:contains(File, "api").

is_frontend_ref(Ref) :- code_element(Ref, _, File, _, _), fn:contains(File, "frontend").
is_frontend_ref(Ref) :- code_element(Ref, _, File, _, _), fn:contains(File, "client").
is_frontend_ref(Ref) :- code_element(Ref, _, File, _, _), fn:contains(File, "web").
```

### Container Archetype

```mangle
# =============================================================================
# SEMANTIC BRIDGE: Containers
# Types that contain other elements (classes, structs, modules)
# =============================================================================

is_container(Ref) :- go_struct(Ref).
is_container(Ref) :- go_interface(Ref).
is_container(Ref) :- py_class(Ref).
is_container(Ref) :- ts_class(Ref).
is_container(Ref) :- ts_interface(Ref).
is_container(Ref) :- rs_struct(Ref).
is_container(Ref) :- rs_trait(Ref).
is_container(Ref) :- kt_class(Ref).
is_container(Ref) :- kt_interface(Ref).
```

## Impact Analysis Rules

```mangle
# =============================================================================
# IMPACT ANALYSIS
# Track what changes when an element is modified
# =============================================================================

# Direct callers
impacts(Changed, Caller) :-
    calls(Caller, Changed).

# Transitive impact
impacts(Changed, Impacted) :-
    impacts(Changed, Mid),
    calls(Impacted, Mid).

# API dependency impact
impacts(Changed, Coupled) :-
    api_dependency(Changed, Coupled).

impacts(Changed, Coupled) :-
    api_dependency(Coupled, Changed).

# Collect all impacted refs
all_impacted(Changed, ImpactedList) :-
    code_element(Changed, _, _, _, _),
    impacts(Changed, Impacted) |>
    do fn:group_by(Changed),
    let ImpactedList = fn:collect(Impacted).
```

## Universal Refactor Plan

```mangle
# =============================================================================
# REFACTOR PLAN AGGREGATION
# Collect all targets for a rename operation
# =============================================================================

Decl plan_rename(OldName.Type<string>, NewName.Type<string>).

# Find all refs that need updating
rename_target(Ref) :-
    plan_rename(OldName, _),
    wire_name(Ref, OldName).

rename_target(Ref) :-
    plan_rename(OldName, _),
    code_element(Ref, _, _, _, _),
    fn:contains(Ref, OldName).

# Aggregate into work unit
refactor_work_unit(OldName, NewName, Targets) :-
    plan_rename(OldName, NewName),
    rename_target(Ref) |>
    do fn:group_by(OldName, NewName),
    let Targets = fn:collect(Ref).
```

## Critical Anti-Patterns

### Atom vs String (CRITICAL)

```mangle
# WRONG - Will silently produce empty results
code_element(Ref, "function", File, Start, End).
element_visibility(Ref, "public").

# CORRECT - Use atoms for enum-like values
code_element(Ref, /function, File, Start, End).
element_visibility(Ref, /public).
```

### Unbound Variables in Negation

```mangle
# WRONG - X is not bound before negation
orphan_element(X) :- not element_parent(X, _).

# CORRECT - Bind X first
orphan_element(X) :- code_element(X, _, _, _, _), not element_parent(X, _).
```

### Struct Field Access

```mangle
# WRONG - No dot notation in Mangle
bad(Name) :- element(E), E.name = Name.

# CORRECT - Use match_field
good(Name) :- element(E), :match_field(E, /name, Name).
```

### Aggregation Syntax

```mangle
# WRONG - SQL-style aggregation
count_by_file(File, Count) :-
    code_element(_, _, File, _, _),
    Count = count(*).

# CORRECT - Pipe transform syntax
count_by_file(File, Count) :-
    code_element(_, _, File, _, _) |>
    do fn:group_by(File),
    let Count = fn:count().
```

## Go Integration

### Asserting Facts

```go
// Convert CodeElement to Mangle facts
func (e *CodeElement) ToFacts() []engine.Atom {
    facts := []engine.Atom{}

    // Base element fact
    facts = append(facts, ast.NewAtom("code_element",
        ast.String(e.Ref),
        ast.Name(string(e.Type)),
        ast.String(e.File),
        ast.Number(e.StartLine),
        ast.Number(e.EndLine),
    ))

    // Signature
    facts = append(facts, ast.NewAtom("element_signature",
        ast.String(e.Ref),
        ast.String(e.Signature),
    ))

    // Parent relationship
    if e.Parent != "" {
        facts = append(facts, ast.NewAtom("element_parent",
            ast.String(e.Ref),
            ast.String(e.Parent),
        ))
    }

    // Visibility
    facts = append(facts, ast.NewAtom("element_visibility",
        ast.String(e.Ref),
        ast.Name(string(e.Visibility)),
    ))

    return facts
}
```

### Querying Facts

```go
// Query for all data contracts
func (k *Kernel) GetDataContracts() ([]string, error) {
    results, err := k.Query("is_data_contract(Ref)")
    if err != nil {
        return nil, err
    }

    var refs []string
    for _, result := range results {
        if ref, ok := result["Ref"].(string); ok {
            refs = append(refs, ref)
        }
    }
    return refs, nil
}

// Query for API dependencies
func (k *Kernel) GetAPIDependencies(ref string) ([]string, error) {
    results, err := k.Query(fmt.Sprintf("api_dependency(%q, Frontend)", ref))
    if err != nil {
        return nil, err
    }

    var deps []string
    for _, result := range results {
        if dep, ok := result["Frontend"].(string); ok {
            deps = append(deps, dep)
        }
    }
    return deps, nil
}
```
