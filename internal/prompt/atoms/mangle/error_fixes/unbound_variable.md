# Fix: Unbound Variable Error

## Error Pattern
```
unbound variable X in negation
unsafe variable in head
```

## Cause
A variable appears in a negated atom or rule head without being bound by a positive atom first.

## Fix Strategy

### For Negation Errors

**Before (Invalid):**
```mangle
safe(X) :- not dangerous(X).
```

**Fix:** Add a binding predicate that defines the domain of X:
```mangle
safe(X) :- candidate(X), not dangerous(X).
```

### For Head Variable Errors

**Before (Invalid):**
```mangle
result(X, Y) :- source(Z).
# X and Y are not bound in the body
```

**Fix:** Ensure all head variables appear in the body:
```mangle
result(X, Y) :- source(X), derived(X, Y).
```

## Common Binding Predicates

For different domains, use these binding predicates:

| Domain | Binding Predicate |
|--------|-------------------|
| Users | `user_intent(ID, _, _, _, _)` |
| Files | `file_topology(Path, _, _, _, _)` |
| Shards | `shard_profile(Name, _, _)` |
| Actions | `candidate_action(Action)` |
| Tasks | `campaign_task(TaskID, _, _, _, _)` |
| Campaigns | `campaign(ID, _, _, _, _)` |

## Example Fix

### Problem: Unbound in negation
```mangle
# WRONG
blocked_user(U) :- not permitted_user(U).
```

### Solution: Add domain binding
```mangle
# CORRECT - user() provides domain
blocked_user(U) :- user(U), not permitted_user(U).
```

### Problem: Unbound in head
```mangle
# WRONG - Result not bound
compute(Input, Result) :- valid(Input).
```

### Solution: Derive or compute the result
```mangle
# CORRECT - Result derived from process
compute(Input, Result) :- valid(Input), process(Input, Result).
```
