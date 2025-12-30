# 300: Language-Specific Safety Guardrails

## Overview

Safety guardrails are Mangle rules that detect language-specific anti-patterns and vulnerabilities that AI agents commonly introduce. They implement "Semantic Diffing" - comparing code state before and after edits to catch regressions.

## Architecture: Snapshot-Based Safety

```text
┌─────────────────────┐     ┌─────────────────────┐
│  SNAPSHOT FACTS     │     │  CANDIDATE FACTS    │
│  (Before Edit)      │     │  (After Edit)       │
│  snapshot:py_dec... │     │  candidate:py_dec...│
└─────────┬───────────┘     └──────────┬──────────┘
          │                            │
          └────────────┬───────────────┘
                       │
                       ▼
          ┌────────────────────────┐
          │   SAFETY RULES         │
          │   deny_edit(Ref, Rsn)  │
          └────────────────────────┘
                       │
                       ▼
          ┌────────────────────────┐
          │   CONSTITUTIONAL GATE  │
          │   permitted(Action)    │
          └────────────────────────┘
```

## Snapshot Management

Before any edit, snapshot the current state:

```go
func (v *VirtualStore) snapshotElement(ref string) error {
    elements := v.scope.GetCoreElementsByRef(ref)
    for _, elem := range elements {
        facts := elem.ToFacts()
        for _, fact := range facts {
            // Prefix with "snapshot:"
            snapshotFact := prefixPredicate(fact, "snapshot")
            v.kernel.Assert(snapshotFact)
        }
    }
    return nil
}

func (v *VirtualStore) assertCandidateFacts(elements []CodeElement) error {
    for _, elem := range elements {
        facts := elem.ToFacts()
        for _, fact := range facts {
            // Prefix with "candidate:"
            candidateFact := prefixPredicate(fact, "candidate")
            v.kernel.Assert(candidateFact)
        }
    }
    return nil
}
```

## Python Safety Rules

### Decorator Stripping Detection

```mangle
# =============================================================================
# PYTHON: Security Decorator Stripping
# Detect when security decorators are removed from functions
# =============================================================================

# Security-critical decorators
security_decorator(/login_required).
security_decorator(/authenticated).
security_decorator(/permission_required).
security_decorator(/csrf_protect).
security_decorator(/rate_limit).
security_decorator(/validate_input).
security_decorator(/require_admin).

# Detect stripping
deny_edit(Ref, /security_regression) :-
    # Was protected in snapshot
    snapshot:py_decorator(Ref, DecName),
    security_decorator(DecName),
    # Is NOT protected in candidate
    not candidate:py_decorator(Ref, DecName).

# Also check for @app.route without authentication
deny_edit(Ref, /unprotected_route) :-
    candidate:py_decorator(Ref, "route"),
    not candidate:py_decorator(Ref, /login_required),
    not candidate:py_decorator(Ref, /authenticated),
    route_requires_auth(Ref).

route_requires_auth(Ref) :-
    element_body(Ref, Body),
    fn:contains(Body, "current_user").
```

### Type Hint Regression

```mangle
# =============================================================================
# PYTHON: Type Hint Regression
# Detect when type hints are removed (reduces type safety)
# =============================================================================

deny_edit(Ref, /type_hint_regression) :-
    snapshot:py_function(Ref),
    snapshot:element_signature(Ref, OldSig),
    fn:contains(OldSig, "->"),
    candidate:py_function(Ref),
    candidate:element_signature(Ref, NewSig),
    not fn:contains(NewSig, "->").
```

## Go Safety Rules

### Goroutine Leak Detection (Forgotten Sender)

```mangle
# =============================================================================
# GO: Goroutine Leak Detection
# Detect goroutines without proper synchronization
# =============================================================================

# A goroutine is safe if it has synchronization
has_sync(Ref) :- go_channel_send(Ref, _).
has_sync(Ref) :- go_channel_recv(Ref, _).
has_sync(Ref) :- element_body(Ref, Body), fn:contains(Body, "wg.Done()").
has_sync(Ref) :- element_body(Ref, Body), fn:contains(Body, "wg.Add(").
has_sync(Ref) :- element_body(Ref, Body), fn:contains(Body, "close(").

# Detect leak in candidate code
deny_edit(Ref, /goroutine_leak) :-
    candidate:go_goroutine(Ref),
    not has_sync(Ref).

# Also check for context cancellation
deny_edit(Ref, /goroutine_no_context) :-
    candidate:go_goroutine(Ref),
    candidate:element_signature(Ref, Sig),
    not fn:contains(Sig, "ctx"),
    not fn:contains(Sig, "context.Context").
```

### Error Handling Regression

```mangle
# =============================================================================
# GO: Error Handling Regression
# Detect when error checks are removed
# =============================================================================

# Count error checks in body
error_check_count(Ref, Count) :-
    element_body(Ref, Body),
    Count = fn:count_occurrences(Body, "if err != nil").

deny_edit(Ref, /error_handling_regression) :-
    snapshot:go_function(Ref),
    snapshot:error_check_count(Ref, OldCount),
    candidate:go_function(Ref),
    candidate:error_check_count(Ref, NewCount),
    NewCount < OldCount.

# Detect ignored errors (using _ for error)
deny_edit(Ref, /ignored_error) :-
    candidate:element_body(Ref, Body),
    fn:contains(Body, ", _ :="),
    fn:contains(Body, "err").
```

### Defer Without Unlock

```mangle
# =============================================================================
# GO: Mutex Safety
# Detect Lock() without defer Unlock()
# =============================================================================

has_lock(Ref) :- element_body(Ref, Body), fn:contains(Body, ".Lock()").
has_defer_unlock(Ref) :- element_body(Ref, Body), fn:contains(Body, "defer").

deny_edit(Ref, /lock_without_defer) :-
    candidate:go_function(Ref),
    candidate:has_lock(Ref),
    not candidate:has_defer_unlock(Ref).
```

## TypeScript/React Safety Rules

### Stale Closure Detection

```mangle
# =============================================================================
# REACT: Stale Closure Detection
# Detect useEffect hooks that read state but don't declare dependencies
# =============================================================================

# Variables that are state
is_state_variable(Var) :-
    ts_hook(_, "useState"),
    element_body(_, Body),
    fn:match("const \\[([a-zA-Z]+),", Body, Var).

# Detect stale closure
deny_edit(Ref, /react_stale_closure) :-
    candidate:ts_hook(Ref, "useEffect"),
    candidate:ts_hook_reads(Ref, Var),
    is_state_variable(Var),
    not candidate:ts_hook_dep(Ref, Var).
```

### Missing Error Boundary

```mangle
# =============================================================================
# REACT: Error Boundary Check
# Components that fetch data should have error handling
# =============================================================================

fetches_data(Ref) :-
    ts_component(Ref, _),
    element_body(Ref, Body),
    fn:contains(Body, "fetch(").

fetches_data(Ref) :-
    ts_component(Ref, _),
    element_body(Ref, Body),
    fn:contains(Body, "useQuery(").

has_error_handling(Ref) :-
    element_body(Ref, Body),
    fn:contains(Body, "catch").

has_error_handling(Ref) :-
    element_body(Ref, Body),
    fn:contains(Body, "onError").

deny_edit(Ref, /missing_error_handling) :-
    candidate:fetches_data(Ref),
    not candidate:has_error_handling(Ref).
```

### Prop Type Removal

```mangle
# =============================================================================
# TYPESCRIPT: Prop Type Safety
# Detect when TypeScript types are weakened to 'any'
# =============================================================================

deny_edit(Ref, /type_weakened_to_any) :-
    snapshot:ts_component(Ref, _),
    snapshot:element_signature(Ref, OldSig),
    not fn:contains(OldSig, ": any"),
    candidate:ts_component(Ref, _),
    candidate:element_signature(Ref, NewSig),
    fn:contains(NewSig, ": any").
```

## Kotlin Safety Rules

### Force Unwrap Detection

```mangle
# =============================================================================
# KOTLIN: Null Safety
# Detect use of !! (force unwrap) which can cause runtime crashes
# =============================================================================

deny_edit(Ref, /kotlin_force_unwrap) :-
    candidate:kt_force_unwrap(Ref),
    is_new_code(Ref).

# Only flag in newly generated code
is_new_code(Ref) :-
    candidate:code_element(Ref, _, _, _, _),
    not snapshot:code_element(Ref, _, _, _, _).
```

### Suspend Function Without Dispatcher

```mangle
# =============================================================================
# KOTLIN: Coroutine Safety
# Suspend functions doing IO should specify dispatcher
# =============================================================================

does_io(Ref) :-
    kt_suspend_fun(Ref),
    element_body(Ref, Body),
    fn:contains(Body, "readFile").

does_io(Ref) :-
    kt_suspend_fun(Ref),
    element_body(Ref, Body),
    fn:contains(Body, "httpClient").

has_dispatcher(Ref) :-
    element_body(Ref, Body),
    fn:contains(Body, "withContext(").

deny_edit(Ref, /suspend_without_dispatcher) :-
    candidate:does_io(Ref),
    not candidate:has_dispatcher(Ref).
```

## Rust Safety Rules

### Async Lock Hazard

```mangle
# =============================================================================
# RUST: Async Lock Safety
# Detect MutexGuard held across await points (causes !Send Future)
# =============================================================================

deny_edit(Ref, /rust_async_lock_hazard) :-
    candidate:rs_async_fn(Ref),
    candidate:rs_mutex_guard(Ref, Var),
    candidate:rs_await_point(Ref),
    guard_alive_at_await(Ref, Var).

# Simplified heuristic: guard created before await, no drop
guard_alive_at_await(Ref, Var) :-
    element_body(Ref, Body),
    fn:index_of(Body, Var, GuardPos),
    fn:index_of(Body, ".await", AwaitPos),
    GuardPos < AwaitPos,
    not fn:contains(Body, "drop(").
```

### Unsafe Block Addition

```mangle
# =============================================================================
# RUST: Unsafe Code Review
# Flag new unsafe blocks for review
# =============================================================================

deny_edit(Ref, /new_unsafe_code) :-
    candidate:rs_unsafe_block(Ref),
    not snapshot:rs_unsafe_block(Ref).

# Also flag unsafe fn
deny_edit(Ref, /new_unsafe_fn) :-
    candidate:rs_unsafe_fn(Ref),
    not snapshot:rs_unsafe_fn(Ref).
```

## Cross-Language Safety Rules

### Breaking Change Detection

```mangle
# =============================================================================
# CROSS-LANGUAGE: Breaking API Change
# Detect changes to wire names that would break API consumers
# =============================================================================

deny_edit(Ref, /breaking_api_change) :-
    # Wire name changed
    snapshot:wire_name(Ref, OldName),
    candidate:wire_name(Ref, NewName),
    OldName != NewName,
    # Has API consumers
    api_dependency(Ref, _).
```

### Test Coverage Regression

```mangle
# =============================================================================
# UNIVERSAL: Test Coverage
# Elements with tests should not have tests removed
# =============================================================================

has_test(Ref) :-
    code_element(TestRef, /function, TestFile, _, _),
    fn:contains(TestFile, "_test"),
    element_body(TestRef, Body),
    fn:contains(Body, Ref).

deny_edit(Ref, /test_coverage_regression) :-
    snapshot:has_test(Ref),
    not candidate:has_test(Ref).
```

## Constitutional Gate Integration

```mangle
# =============================================================================
# CONSTITUTIONAL GATE
# Final permission check before any edit
# =============================================================================

# Default: edits are permitted if no deny rule fires
permitted(edit, Ref) :-
    code_element(Ref, _, _, _, _),
    not deny_edit(Ref, _).

# Block edits with reasons
blocked(Ref, Reason) :- deny_edit(Ref, Reason).

# Require explicit override for blocked edits
permitted(edit, Ref) :-
    deny_edit(Ref, _),
    explicit_override(Ref).

# Override must be logged
Decl explicit_override(Ref.Type<string>).
Decl override_reason(Ref.Type<string>, Reason.Type<string>).
```

## Implementation in VirtualStore

```go
func (v *VirtualStore) handleEditElement(ctx context.Context, req ActionRequest) (ActionResult, error) {
    ref := req.Params["ref"].(string)
    newBody := req.Params["body"].(string)

    // 1. Snapshot current state
    if err := v.snapshotElement(ref); err != nil {
        return ActionResult{}, err
    }

    // 2. Parse candidate (new code)
    candidateElements, err := v.parseCandidate(newBody)
    if err != nil {
        return ActionResult{}, err
    }

    // 3. Assert candidate facts
    if err := v.assertCandidateFacts(candidateElements); err != nil {
        return ActionResult{}, err
    }

    // 4. Check safety rules
    blocked, err := v.kernel.Query(fmt.Sprintf("blocked(%q, Reason)", ref))
    if err != nil {
        return ActionResult{}, err
    }

    if len(blocked) > 0 {
        reasons := make([]string, len(blocked))
        for i, b := range blocked {
            reasons[i] = b["Reason"].(string)
        }
        return ActionResult{
            Success: false,
            Error:   fmt.Sprintf("Edit blocked: %v", reasons),
            Data:    map[string]interface{}{"blocked_reasons": reasons},
        }, nil
    }

    // 5. Perform the edit
    // ...

    // 6. Clean up snapshot facts
    v.clearSnapshotFacts()
    v.clearCandidateFacts()

    return ActionResult{Success: true}, nil
}
```

## Remediation Suggestions

When a deny rule fires, provide actionable remediation:

```go
var remediations = map[string]string{
    "/security_regression":    "Restore the security decorator or add @login_required",
    "/goroutine_leak":         "Add WaitGroup, channel, or context cancellation",
    "/react_stale_closure":    "Add missing variable to useEffect dependency array",
    "/kotlin_force_unwrap":    "Use safe call (?.) or null check instead of !!",
    "/rust_async_lock_hazard": "Drop the MutexGuard before .await or use tokio::sync::Mutex",
    "/breaking_api_change":    "Update all API consumers or add backward compatibility",
}

func (v *VirtualStore) getRemediation(reason string) string {
    if r, ok := remediations[reason]; ok {
        return r
    }
    return "Review the edit and ensure it doesn't introduce regressions"
}
```
