# 400: Advanced Code Analysis Patterns

## Overview

These patterns leverage Mangle's graph traversal and recursive capabilities to perform sophisticated code analysis. They are derived from the mangle-programming skill's "God-Tier" patterns adapted for Code DOM.

## Pattern 1: Meta-Circular Linter (AST Analysis)

### Concept

Standard Datalog operates on flat tuples. Mangle operates on **Trees (Maps/Lists)**. This allows traversing deeply nested, irregular structures like ASTs natively in logic.

### Recursive AST Traversal

```mangle
# =============================================================================
# GENERIC AST TRAVERSAL
# Find any node type in a nested AST structure
# =============================================================================

# Base case: direct match
contains_node_type(Root, TargetType, Node) :-
    :match_field(Root, /type, TargetType),
    Node = Root.

# Recursive: search children list
contains_node_type(Root, TargetType, Node) :-
    :match_field(Root, /children, Children),
    :list:member(Child, Children),
    contains_node_type(Child, TargetType, Node).

# Recursive: search body
contains_node_type(Root, TargetType, Node) :-
    :match_field(Root, /body, Body),
    contains_node_type(Body, TargetType, Node).

# Recursive: search statements
contains_node_type(Root, TargetType, Node) :-
    :match_field(Root, /statements, Stmts),
    :list:member(Stmt, Stmts),
    contains_node_type(Stmt, TargetType, Node).
```

### Goroutine Leak Detector (Full Implementation)

```mangle
# =============================================================================
# AST-BASED GOROUTINE LEAK DETECTOR
# Detect goroutines without proper exit paths
# =============================================================================

# 1. Find goroutine spawn sites
spawn_site(Node) :-
    :match_field(Node, /type, /GoStmt),
    :match_field(Node, /call, CallExpr).

# 2. Check for exit paths in a block
has_exit_path(Block) :-
    :match_field(Block, /statements, Stmts),
    :list:member(Stmt, Stmts),
    is_return_or_close(Stmt).

is_return_or_close(Stmt) :- :match_field(Stmt, /type, /ReturnStmt).
is_return_or_close(Stmt) :- :match_field(Stmt, /type, /CloseStmt).
is_return_or_close(Stmt) :-
    :match_field(Stmt, /type, /SendStmt),
    :match_field(Stmt, /chan, Chan),
    channel_has_receiver(Chan).

# 3. Recursive block analysis (nested if/for/switch)
has_exit_path(Block) :-
    :match_field(Block, /statements, Stmts),
    :list:member(Stmt, Stmts),
    :match_field(Stmt, /body, InnerBlock),
    has_exit_path(InnerBlock).

# 4. The leak detector
goroutine_leak(Node) :-
    spawn_site(Node),
    :match_field(Node, /body, Body),
    !has_exit_path(Body).
```

### Function Call Finder

```mangle
# =============================================================================
# FIND ALL FUNCTION CALLS IN A FILE
# =============================================================================

# Convert file AST to query
function_call_in_file(File, CallNode) :-
    file_ast(File, FileAST),
    contains_node_type(FileAST, /CallExpr, CallNode).

# Extract callee name
call_to_function(File, FuncName) :-
    function_call_in_file(File, CallNode),
    :match_field(CallNode, /fun, FunNode),
    :match_field(FunNode, /name, FuncName).

# Build call graph
calls(Caller, Callee) :-
    code_element(Caller, /function, File, _, _),
    call_to_function(File, CalleeName),
    code_element(Callee, /function, _, _, _),
    element_name(Callee, CalleeName).
```

## Pattern 2: Poisoned River (Taint Analysis)

### Concept

Track data flow from untrusted sources (user input) to sensitive sinks (SQL queries, shell commands) to detect injection vulnerabilities.

### Full Taint Analysis Engine

```mangle
# =============================================================================
# TAINT ANALYSIS ENGINE
# Track flow from sources to sinks
# =============================================================================

# 1. Define Sources (User Input)
source(Var) :- var_annotation(Var, /user_input).
source(Var) :- function_return(Var, /http_request_body).
source(Var) :- function_return(Var, /query_param).
source(Var) :- function_return(Var, /form_data).
source(Var) :- function_return(Var, /cookie_value).
source(Var) :- function_return(Var, /header_value).

# 2. Define Sinks (Dangerous Operations)
sink(Var, /sql_injection) :- var_context(Var, /sql_query).
sink(Var, /xss) :- var_context(Var, /html_output).
sink(Var, /command_injection) :- var_context(Var, /shell_exec).
sink(Var, /path_traversal) :- var_context(Var, /file_path).
sink(Var, /ssrf) :- var_context(Var, /http_request_url).

# 3. Define Sanitization (The Filter)
sanitized(Out) :- function_call(/escape_sql, _, Out).
sanitized(Out) :- function_call(/html_encode, _, Out).
sanitized(Out) :- function_call(/validate_input, _, Out).
sanitized(Out) :- function_call(/sanitize_path, _, Out).
sanitized(Out) :- function_call(/url_encode, _, Out).

# 4. Recursive Flow (Data Movement)
flows_to(A, B) :- assignment(A, B).
flows_to(A, B) :- function_call(_, A, B).
flows_to(A, C) :- flows_to(A, B), flows_to(B, C).

# 5. Taint Propagation
tainted(Var) :- source(Var).
tainted(B) :-
    flows_to(A, B),
    tainted(A),
    !sanitized(B).

# 6. Vulnerability Detection
vulnerability(Var, Type, /critical) :-
    sink(Var, Type),
    tainted(Var).

# 7. Path Reconstruction (for debugging)
taint_path(Src, Dst, [Src, Dst]) :-
    source(Src),
    flows_to(Src, Dst),
    sink(Dst, _).

taint_path(Src, Dst, [Src | Rest]) :-
    source(Src),
    flows_to(Src, Mid),
    taint_path(Mid, Dst, Rest).
```

### Language-Specific Source Detection

```mangle
# =============================================================================
# PYTHON: Source Detection
# =============================================================================

source(Var) :-
    py_function(Ref),
    element_body(Ref, Body),
    fn:match("request\\.(args|form|json|data)\\[", Body, _),
    extract_var_from_match(Body, Var).

# Flask/Django request parameters
source(Var) :-
    call_to_function(File, "request.args.get"),
    result_assigned_to(File, Var).

# =============================================================================
# GO: Source Detection
# =============================================================================

source(Var) :-
    go_function(Ref),
    element_body(Ref, Body),
    fn:match("r\\.URL\\.Query\\(\\)", Body, _),
    extract_var_from_match(Body, Var).

source(Var) :-
    go_function(Ref),
    element_body(Ref, Body),
    fn:match("r\\.FormValue\\(", Body, _),
    extract_var_from_match(Body, Var).
```

## Pattern 3: Bisimulation (Regression Verification)

### Concept

Verify that a code change doesn't accidentally remove security checks, permissions, or alter behavior. Compare the "logical truth" of V1 vs V2.

### Security Regression Detector

```mangle
# =============================================================================
# SECURITY REGRESSION DETECTOR
# Compare permissions/access between versions
# =============================================================================

# 1. Establish effective permissions for V1
v1_allowed(User, Resource, Action) :-
    v1_policy(User, Role),
    v1_role_perm(Role, Resource, Action).

v1_allowed(User, Resource, Action) :-
    v1_policy(User, Role),
    v1_role_inherits(Role, ParentRole),
    v1_role_perm(ParentRole, Resource, Action).

# 2. Establish effective permissions for V2
v2_allowed(User, Resource, Action) :-
    v2_policy(User, Role),
    v2_role_perm(Role, Resource, Action).

v2_allowed(User, Resource, Action) :-
    v2_policy(User, Role),
    v2_role_inherits(Role, ParentRole),
    v2_role_perm(ParentRole, Resource, Action).

# 3. Privilege Escalation (CRITICAL)
# Access exists in V2 but NOT in V1
security_regression(User, Resource, Action) :-
    v2_allowed(User, Resource, Action),
    !v1_allowed(User, Resource, Action).

# 4. Privilege Reduction (may be intentional)
permission_removed(User, Resource, Action) :-
    v1_allowed(User, Resource, Action),
    !v2_allowed(User, Resource, Action).

# 5. Critical Regressions
critical_regression(User, Resource, Action) :-
    security_regression(User, Resource, Action),
    sensitive_resource(Resource).

sensitive_resource(/admin_panel).
sensitive_resource(/user_data).
sensitive_resource(/payment_info).
sensitive_resource(/api_keys).
```

### Behavioral Equivalence Check

```mangle
# =============================================================================
# BEHAVIORAL EQUIVALENCE
# Check if two versions produce the same outputs for the same inputs
# =============================================================================

# Define test cases
Decl test_case(Name.Type<string>, Input.Type<string>, ExpectedOutput.Type<string>).

# V1 behavior
v1_output(TestName, Output) :-
    test_case(TestName, Input, _),
    v1_function_result(Input, Output).

# V2 behavior
v2_output(TestName, Output) :-
    test_case(TestName, Input, _),
    v2_function_result(Input, Output).

# Behavior divergence
behavior_regression(TestName, V1Out, V2Out) :-
    v1_output(TestName, V1Out),
    v2_output(TestName, V2Out),
    V1Out != V2Out.

# All tests pass
behaviorally_equivalent() :-
    not behavior_regression(_, _, _).
```

## Pattern 4: Dependency Impact Analysis

### Transitive Dependency Tracking

```mangle
# =============================================================================
# DEPENDENCY IMPACT ANALYSIS
# Track what changes when an element is modified
# =============================================================================

# Direct dependencies
depends_on(A, B) :- calls(A, B).
depends_on(A, B) :- imports(A, B).
depends_on(A, B) :- extends(A, B).
depends_on(A, B) :- implements(A, B).

# Transitive closure
depends_on_transitive(A, B) :- depends_on(A, B).
depends_on_transitive(A, C) :- depends_on(A, B), depends_on_transitive(B, C).

# Reverse dependencies (who depends on me?)
depended_by(A, B) :- depends_on(B, A).
depended_by_transitive(A, B) :- depends_on_transitive(B, A).

# Impact analysis for a change
impacted_by_change(Changed, Impacted) :-
    depended_by_transitive(Changed, Impacted).

# Aggregate impact
change_impact_count(Changed, Count) :-
    code_element(Changed, _, _, _, _),
    impacted_by_change(Changed, _) |>
    do fn:group_by(Changed),
    let Count = fn:count().

# High-impact changes (affects many dependents)
high_impact_change(Changed) :-
    change_impact_count(Changed, Count),
    Count > 10.
```

### Breaking Change Detection

```mangle
# =============================================================================
# BREAKING CHANGE DETECTION
# Detect changes that would break dependents
# =============================================================================

# Signature change
breaking_change(Ref, /signature_changed) :-
    snapshot:element_signature(Ref, OldSig),
    candidate:element_signature(Ref, NewSig),
    OldSig != NewSig,
    has_external_dependents(Ref).

# Visibility reduction
breaking_change(Ref, /visibility_reduced) :-
    snapshot:element_visibility(Ref, /public),
    candidate:element_visibility(Ref, /private),
    has_external_dependents(Ref).

# Element removed
breaking_change(Ref, /element_removed) :-
    snapshot:code_element(Ref, _, _, _, _),
    not candidate:code_element(Ref, _, _, _, _),
    has_external_dependents(Ref).

# Has dependents outside its own file
has_external_dependents(Ref) :-
    code_element(Ref, _, File, _, _),
    depends_on(Dependent, Ref),
    code_element(Dependent, _, DepFile, _, _),
    File != DepFile.
```

## Pattern 5: Code Duplication Detection

```mangle
# =============================================================================
# CODE DUPLICATION DETECTION
# Find similar code blocks that could be refactored
# =============================================================================

# Normalize function body (strip whitespace, variable names)
normalized_body(Ref, Normalized) :-
    element_body(Ref, Body),
    Normalized = fn:normalize(Body).

# Exact duplicates
exact_duplicate(Ref1, Ref2) :-
    normalized_body(Ref1, Body),
    normalized_body(Ref2, Body),
    Ref1 < Ref2.  # Avoid self-comparison and duplicates

# Similar functions (same structure, different names)
similar_function(Ref1, Ref2, Similarity) :-
    code_element(Ref1, /function, _, _, _),
    code_element(Ref2, /function, _, _, _),
    Ref1 < Ref2,
    element_body(Ref1, Body1),
    element_body(Ref2, Body2),
    Similarity = fn:jaccard_similarity(Body1, Body2),
    Similarity > 0.8.

# Suggest extraction
extraction_candidate(Refs, CommonBody) :-
    exact_duplicate(R1, R2) |>
    do fn:group_by(CommonBody),
    let Refs = fn:collect([R1, R2]).
```

## Pattern 6: Complexity Analysis

```mangle
# =============================================================================
# CYCLOMATIC COMPLEXITY
# Count decision points in code
# =============================================================================

# Count branches
decision_point_count(Ref, Count) :-
    element_body(Ref, Body),
    IfCount = fn:count_occurrences(Body, "if "),
    ForCount = fn:count_occurrences(Body, "for "),
    WhileCount = fn:count_occurrences(Body, "while "),
    SwitchCount = fn:count_occurrences(Body, "switch "),
    CaseCount = fn:count_occurrences(Body, "case "),
    Count = fn:plus(fn:plus(fn:plus(fn:plus(IfCount, ForCount), WhileCount), SwitchCount), CaseCount).

# Cyclomatic complexity = decision points + 1
cyclomatic_complexity(Ref, Complexity) :-
    decision_point_count(Ref, DecisionPoints),
    Complexity = fn:plus(DecisionPoints, 1).

# High complexity warning
high_complexity(Ref, Complexity) :-
    cyclomatic_complexity(Ref, Complexity),
    Complexity > 10.

# Very high complexity (needs refactoring)
critical_complexity(Ref, Complexity) :-
    cyclomatic_complexity(Ref, Complexity),
    Complexity > 20.
```

## Pattern 7: Dead Code Detection

```mangle
# =============================================================================
# DEAD CODE DETECTION
# Find unreachable or unused code
# =============================================================================

# Entry points (public API, main, handlers)
is_entry_point(Ref) :- element_visibility(Ref, /public).
is_entry_point(Ref) :- element_name(Ref, "main").
is_entry_point(Ref) :- py_decorator(Ref, "route").
is_entry_point(Ref) :- py_decorator(Ref, "handler").
is_entry_point(Ref) :- go_method(Ref), fn:contains(Ref, "Handler").

# Reachable from entry points
reachable(Ref) :- is_entry_point(Ref).
reachable(Ref) :- reachable(Caller), calls(Caller, Ref).

# Dead code
dead_code(Ref) :-
    code_element(Ref, /function, _, _, _),
    element_visibility(Ref, /private),
    not reachable(Ref).

dead_code(Ref) :-
    code_element(Ref, /method, _, _, _),
    element_visibility(Ref, /private),
    not reachable(Ref).

# Unused parameters
unused_parameter(Ref, ParamName) :-
    code_element(Ref, /function, _, _, _),
    element_signature(Ref, Sig),
    extract_param(Sig, ParamName),
    element_body(Ref, Body),
    not fn:contains(Body, ParamName).
```

## Mangle Limitations to Avoid

### Infinite Generation

```mangle
# WRONG - Will crash (infinite generation)
next_id(ID) :- current_id(Old), ID = fn:plus(Old, 1).
current_id(ID) :- next_id(ID).

# CORRECT - Bound by existing domain
valid_id(ID) :- known_ids(ID).
next_valid_id(ID, Next) :-
    valid_id(ID),
    valid_id(Next),
    Next = fn:plus(ID, 1).
```

### No Shortest Path

```mangle
# PROBLEMATIC - Generates ALL paths, not shortest
path_cost(X, Y, Cost) :- edge(X, Y, Cost).
path_cost(X, Z, TotalCost) :-
    edge(X, Y, Cost1),
    path_cost(Y, Z, Cost2),
    TotalCost = fn:plus(Cost1, Cost2).

# BETTER - Find connectivity, optimize externally
reachable(X, Y) :- edge(X, Y, _).
reachable(X, Z) :- edge(X, Y, _), reachable(Y, Z).
```

### Side Effects

```mangle
# IMPOSSIBLE - Mangle is pure
send_alert(User) :- violation(User).  # Can't actually send!

# CORRECT - Declare intent, execute in Go
should_alert(User, Reason) :- violation(User, Reason).
```

## Integration with Go

```go
// Run advanced analysis
func (k *Kernel) RunSecurityAnalysis() ([]Vulnerability, error) {
    results, err := k.Query("vulnerability(Var, Type, Severity)")
    if err != nil {
        return nil, err
    }

    var vulns []Vulnerability
    for _, r := range results {
        vulns = append(vulns, Vulnerability{
            Variable: r["Var"].(string),
            Type:     r["Type"].(string),
            Severity: r["Severity"].(string),
        })
    }
    return vulns, nil
}

// Get taint path for debugging
func (k *Kernel) GetTaintPath(sink string) ([]string, error) {
    results, err := k.Query(fmt.Sprintf("taint_path(Src, %q, Path)", sink))
    if err != nil {
        return nil, err
    }

    if len(results) == 0 {
        return nil, nil
    }

    path := results[0]["Path"].([]interface{})
    strPath := make([]string, len(path))
    for i, p := range path {
        strPath[i] = p.(string)
    }
    return strPath, nil
}
```
