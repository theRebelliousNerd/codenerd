# Fix: Infinite Recursion

## Error Pattern
```
timeout
gas limit exceeded
maximum iterations reached
out of memory
```

## Cause
A recursive rule generates an unbounded number of facts.

## Common Patterns

### 1. Unbounded Counter
**Problem:**
```mangle
# INFINITE - no upper bound
count(0).
count(N) :- count(M), N = fn:plus(M, 1).
```

**Fix:** Add a limit
```mangle
count(0).
count(N) :- count(M), N = fn:plus(M, 1), M < 100.
```

### 2. Self-Referencing Without Base Case
**Problem:**
```mangle
# INFINITE - always fires
reachable(X) :- reachable(X).
```

**Fix:** Require actual progress
```mangle
reachable(X) :- start(X).
reachable(Y) :- reachable(X), edge(X, Y).
```

### 3. Mutual Recursion Without Termination
**Problem:**
```mangle
# INFINITE - a <-> b forever
a(X) :- b(X).
b(X) :- a(X).
```

**Fix:** Ground one predicate
```mangle
a(X) :- initial(X).
b(X) :- a(X), condition(X).
a(Y) :- b(X), derive(X, Y), not a(Y).  # Progress + check
```

### 4. Cartesian Product Explosion
**Problem:**
```mangle
# EXPLOSIVE - creates huge intermediate result
combo(X, Y, Z) :- big_table1(X), big_table2(Y), big_table3(Z).
```

**Fix:** Add early filters
```mangle
combo(X, Y, Z) :-
    filter_condition(X),  # Reduce X first
    big_table1(X),
    related(X, Y),        # Only related Y
    big_table2(Y),
    derives(Y, Z),        # Only derives Z
    big_table3(Z).
```

## Prevention Strategies

### 1. Always Limit Recursive Depth
```mangle
# Safe recursive pattern
step(0, Start) :- start(Start).
step(N, Next) :-
    step(M, Curr),
    transition(Curr, Next),
    N = fn:plus(M, 1),
    M < 50.  # MUST have limit
```

### 2. Use Monotonic Progress
```mangle
# Each step must make measurable progress
path(X, Y, 1) :- edge(X, Y).
path(X, Z, N) :-
    path(X, Y, M),
    edge(Y, Z),
    N = fn:plus(M, 1),
    M < 20,       # Depth limit
    not path(X, Z, _).  # Only new paths
```

### 3. Filter Before Join
```mangle
# Good: filter first, then join
result(X, Y) :-
    X = /specific_value,  # Filter first
    table1(X, _),
    table2(X, Y).
```

## Detection Checklist

1. Does the rule call itself (directly or indirectly)?
2. Is there a base case that eventually stops?
3. Does each recursive step make progress toward termination?
4. Is there a maximum depth/count limit?
