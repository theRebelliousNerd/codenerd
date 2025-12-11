# Failure Mode 6: Infinite Recursion in Fixpoint

## Category
Semantic Safety & Logic (Termination Violation)

## Severity
CRITICAL - Infinite loop, program never terminates

## Error Pattern
Unbounded recursive rules that generate infinite facts. Mangle uses **eager, exhaustive evaluation** - it computes ALL derivable facts, so unbounded generation causes infinite loops.

## Wrong Code
```mangle
# WRONG - Counter fallacy (generates 1, 2, 3, ... forever)
next_id(ID) :- current_id(Old), ID = fn:plus(Old, 1).
current_id(ID) :- next_id(ID).
# RESULT: Infinite loop!

# WRONG - Unbounded sequence generation
natural(0).
natural(N) :- natural(M), N = fn:plus(M, 1).
# Generates all natural numbers - never stops!

# WRONG - Unbounded string concatenation
build_string(S) :- build_string(Prev), S = fn:concat(Prev, "x").
build_string("").
# Generates "", "x", "xx", "xxx", ... forever!

# WRONG - No termination condition on depth
expand(X, Y) :- node(X), expand(Y, Z), edge(X, Z).
# Missing depth limit!

# WRONG - Self-expanding list
grow(L) :- grow(Prev), L = fn:append(Prev, 1).
grow([]).
# Generates [], [1], [1,1], [1,1,1], ... infinitely!
```

## Correct Code
```mangle
# CORRECT - Graph traversal (bounded by finite edges)
reachable(X, Y) :- edge(X, Y).
reachable(X, Z) :- edge(X, Y), reachable(Y, Z).
# Terminates: limited by finite edge relation

# CORRECT - Explicit depth limit
path(X, Y, 1) :- edge(X, Y).
path(X, Z, D) :-
    edge(X, Y),
    path(Y, Z, D1),
    D = fn:plus(D1, 1),
    D < 10.  # Hard limit prevents infinite recursion

# CORRECT - Bounded by explicit set
reachable_within_10(X, Y) :-
    reachable_at_depth(X, Y, D),
    D <= 10.

reachable_at_depth(X, Y, 1) :- edge(X, Y).
reachable_at_depth(X, Z, D) :-
    edge(X, Y),
    reachable_at_depth(Y, Z, D1),
    D = fn:plus(D1, 1),
    D < 10.

# CORRECT - Counter constrained by existing facts
next_available_id(ID) :-
    current_max(Max),
    ID = fn:plus(Max, 1).

current_max(Max) :-
    existing_id(X) |>
    let Max = fn:Max(X).

# CORRECT - Recursive closure bounded by monotonicity
# (Can only add facts, existing facts provide bound)
ancestor(X, Y) :- parent(X, Y).
ancestor(X, Z) :- parent(X, Y), ancestor(Y, Z).
# Terminates: ancestors limited by finite parent relation

# CORRECT - String building with explicit length limit
build_string(S, Len) :- Len = 0, S = "".
build_string(S, Len) :-
    Len > 0,
    Len < 100,  # Explicit limit
    Len1 = fn:minus(Len, 1),
    build_string(Prev, Len1),
    S = fn:concat(Prev, "x").
```

## Detection
- **Symptom**: Program runs forever, consuming memory
- **Symptom**: CPU usage spikes to 100% and never returns
- **Pattern**: Recursive rule with incrementing values (counters, sequences)
- **Pattern**: Recursive rule without decreasing measure or bound
- **Pattern**: Self-referential rule that generates new values from old
- **Test**: Trace execution - do new facts keep being generated indefinitely?

## Prevention

### Termination Checklist
For every recursive rule, verify ONE of these conditions:

1. **Bounded by finite base facts**
   - Recursion only combines existing facts
   - Cannot create new constants/values
   - Example: Graph traversal over fixed edges

2. **Explicit depth/iteration limit**
   - Counter variable with hard upper bound
   - Example: `D < 10` in recursion

3. **Decreasing measure**
   - Each recursive call reduces some value
   - Base case when value reaches minimum
   - Example: List length decreases in recursion

4. **Monotonic convergence**
   - Facts can only be added, not generated infinitely
   - Finite domain ensures fixpoint
   - Example: Transitive closure over finite relations

### Mental Model: Eager vs Lazy Evaluation

**AI assumes (WRONG)**: Lazy evaluation
```python
# Python generators - compute on demand
def natural_numbers():
    n = 0
    while True:
        yield n
        n += 1

# Only computes when you ask for next value
```

**Mangle reality (CORRECT)**: Eager evaluation
```mangle
# Mangle computes ALL derivable facts immediately
natural(0).
natural(N) :- natural(M), N = fn:plus(M, 1).
# Tries to compute ALL natural numbers before returning!
```

## Safe Recursion Patterns

### Pattern 1: Transitive Closure (Finite Domain)
```mangle
# Bounded by finite edge relation
reachable(X, Y) :- edge(X, Y).                    # Base case
reachable(X, Z) :- reachable(X, Y), edge(Y, Z).   # Recursive case
# Terminates: Can only reach finitely many nodes via finite edges
```

### Pattern 2: Explicit Depth Counter
```mangle
# Bounded by explicit depth limit
ancestor_within_5(X, Y, 1) :- parent(X, Y).
ancestor_within_5(X, Z, D) :-
    parent(X, Y),
    ancestor_within_5(Y, Z, D1),
    D = fn:plus(D1, 1),
    D <= 5.  # Explicit bound
```

### Pattern 3: Decreasing List Recursion
```mangle
# Bounded by list length (decreases to empty)
list_sum([], 0).
list_sum(L, Sum) :-
    :match_cons(L, Head, Tail),  # Splits list
    list_sum(Tail, TailSum),     # Recursive on smaller list
    Sum = fn:plus(Head, TailSum).
# Terminates: List gets smaller each time
```

### Pattern 4: Stratified Dependencies
```mangle
# Each layer depends only on previous (lower) layers
# Layer 1: Base facts
direct_dep(/app1, /lib1).
direct_dep(/lib1, /core).

# Layer 2: Transitive closure
all_deps(X, Y) :- direct_dep(X, Y).
all_deps(X, Z) :- direct_dep(X, Y), all_deps(Y, Z).
# Terminates: Limited by finite direct_dep facts

# Layer 3: Analysis (uses Layer 2, doesn't feed back)
deep_dep(X, Y, D) :-
    all_deps(X, Y),
    path_length(X, Y, D),
    D > 3.
```

## Dangerous Patterns (DO NOT USE)

### Anti-Pattern 1: Counter Generation
```mangle
# WRONG - Unbounded counter
counter(N) :- counter(M), N = fn:plus(M, 1).
counter(0).
# Generates: 0, 1, 2, 3, ... infinitely!

# CORRECT - Bounded counter
counter(N) :- counter(M), N = fn:plus(M, 1), N < 100.
counter(0).
# Generates: 0, 1, 2, ..., 99 (then stops)
```

### Anti-Pattern 2: Sequence Expansion
```mangle
# WRONG - Unbounded sequence
fib(0, 0).
fib(1, 1).
fib(N, F) :-
    fib(N1, F1),
    fib(N2, F2),
    N1 = fn:plus(N2, 1),
    N = fn:plus(N1, 1),
    F = fn:plus(F1, F2).
# Tries to compute ALL Fibonacci numbers!

# CORRECT - Bounded Fibonacci
fib(0, 0).
fib(1, 1).
fib(N, F) :-
    N > 1,
    N < 20,  # Explicit limit
    N1 = fn:minus(N, 1),
    N2 = fn:minus(N, 2),
    fib(N1, F1),
    fib(N2, F2),
    F = fn:plus(F1, F2).
```

### Anti-Pattern 3: Self-Referential Value Generation
```mangle
# WRONG - Each rule application generates new value
grow(L) :- grow(Prev), L = fn:append(Prev, /x).
grow([]).
# Generates: [], [/x], [/x, /x], ... infinitely!

# CORRECT - Bounded growth
grow(L, Len) :- Len = 0, L = [].
grow(L, Len) :-
    Len > 0,
    Len <= 10,  # Explicit bound
    Len1 = fn:minus(Len, 1),
    grow(Prev, Len1),
    L = fn:append(Prev, /x).
```

## Complex Example: Safe Path Finding

```mangle
# Schema
Decl node(ID.Type<n>).
Decl edge(From.Type<n>, To.Type<n>).
Decl path(From.Type<n>, To.Type<n>, Depth.Type<int>).
Decl path_exists(From.Type<n>, To.Type<n>).

# Base facts
node(/a). node(/b). node(/c). node(/d).
edge(/a, /b). edge(/b, /c). edge(/c, /d).

# WRONG - Unbounded recursion
# path(X, Y) :- edge(X, Y).
# path(X, Z) :- path(X, Y), edge(Y, Z).  # No depth limit!

# CORRECT - Bounded by depth
path(X, Y, 1) :- edge(X, Y).
path(X, Z, D) :-
    edge(X, Y),
    path(Y, Z, D1),
    D = fn:plus(D1, 1),
    D < 10.  # Prevents infinite recursion

# CORRECT - Simple existence check (boolean, not depth-tracked)
path_exists(X, Y) :- edge(X, Y).
path_exists(X, Z) :- edge(X, Y), path_exists(Y, Z).
# Safe: Can only visit finitely many nodes via finitely many edges
```

## Why This Matters

### Mangle's Evaluation Model
Mangle uses **bottom-up, eager evaluation**:
1. Start with base facts (EDB)
2. Apply ALL rules to generate new facts
3. Repeat step 2 until **no new facts** are generated (fixpoint)
4. Return final fact set

**Key insight**: Mangle doesn't stop until the fixpoint. If facts keep being generated, it runs forever!

### Comparison to Other Languages
| Language | Evaluation | Infinite Generation |
|----------|------------|---------------------|
| Python | Lazy (generators) | Allowed (yields on demand) |
| SQL | Set-based | Limited by data size |
| Prolog | Backtracking | Can loop, but may terminate on first solution |
| **Mangle** | **Eager fixpoint** | **NEVER allowed - will hang!** |

## Training Bias Origins
| Language | Pattern | Leads to Wrong Mangle |
|----------|---------|----------------------|
| Python | `while True: yield n; n += 1` | Unbounded generation |
| SQL | `WITH RECURSIVE` (DB limits) | Assuming automatic cutoff |
| Prolog | Backtracking terminates | Thinking first solution enough |

## Quick Check
For every recursive rule:
1. **Find the recursive call**: Where does predicate call itself?
2. **Trace what changes**: Do values grow? Lists expand? Counters increase?
3. **Find the bound**: What prevents infinite growth?
   - Explicit limit? (`D < 10`)
   - Finite domain? (Graph with N nodes)
   - Decreasing measure? (List gets shorter)
4. **If no bound found** → Add one or restructure logic!

## Debugging Aid
```mangle
# To debug infinite recursion, add counters to facts:
Decl debug_depth(Predicate.Type<n>, Depth.Type<int>).

# Track maximum depth reached
debug_depth(/my_recursive_pred, MaxDepth) :-
    my_recursive_pred(_, _, D) |>
    let MaxDepth = fn:Max(D).

# If MaxDepth keeps growing → unbounded recursion!
```
