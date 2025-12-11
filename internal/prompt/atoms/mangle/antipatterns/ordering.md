# Anti-Pattern: Ordering Assumptions

## Category
Semantic Mismatch (Declarative Semantics vs Sequential Execution)

## Description
Assuming that rules execute in a specific order, or that the order of atoms in a rule body matters for control flow, when Mangle evaluation is unordered and declarative.

---

## Anti-Pattern 1: Rule Execution Order

### Wrong Approach
```python
# Imperative: these execute in order
step1()
step2()
step3()
```

Attempting:
```mangle
# WRONG - assuming rules execute in order
rule1(X) :- input(X).
rule2(Y) :- rule1(Y).  # Assumes rule1 executed first
rule3(Z) :- rule2(Z).  # Assumes rule2 executed first
```

### Why It Fails
Rules don't execute in order. All rules are evaluated **simultaneously** until a fixed point.

### Correct Mangle Way
```mangle
# Order doesn't matter - dependencies are explicit:
rule1(X) :- input(X).
rule2(Y) :- rule1(Y).  # Works because rule1 is a dependency, not a sequential step
rule3(Z) :- rule2(Z).

# These can be written in ANY order:
rule3(Z) :- rule2(Z).
rule1(X) :- input(X).
rule2(Y) :- rule1(Y).

# Results are identical!
```

**Key Insight:** Dependencies create partial ordering, not sequential execution order.

---

## Anti-Pattern 2: Atom Order in Rule Body

### Wrong Approach
```python
# Imperative: order matters
x = expensive_query()
if x > 10:
    result = process(x)
```

Attempting:
```mangle
# WRONG - assuming atoms execute in order
result(R) :-
    expensive_query(X),  # Execute first
    X > 10,              # Then check
    process(X, R).       # Then process
```

### Why It Fails (Partially)
While Mangle **may** optimize based on atom order, you can't **rely** on it for correctness. The order expresses logical conjunction (AND), not sequential execution.

### Correct Mangle Way
```mangle
# Write for correctness first, optimization second:
result(R) :-
    expensive_query(X),
    X > 10,
    process(X, R).

# If expensive_query returns many results, use selectivity hints:
# Put most selective atoms first (good practice, not required)
result(R) :-
    specific_id(Id),         # Very selective
    expensive_query(Id, X),  # Now only queries for this Id
    X > 10,
    process(X, R).

# But logical meaning is identical regardless of order!
```

**Good Practice:** Put most selective atoms first for performance, but don't rely on order for correctness.

---

## Anti-Pattern 3: Short-Circuit Evaluation

### Wrong Approach
```javascript
// Short-circuit: if first is false, second never evaluates
if (cheap_check(x) && expensive_check(x)) {
    // ...
}
```

Attempting:
```mangle
# WRONG - expecting short-circuit
result(X) :-
    cheap_check(X),      # If this fails, expensive_check shouldn't run
    expensive_check(X).  # But it might!
```

### Why It Fails
Mangle doesn't short-circuit. Both predicates will be evaluated (though the engine may optimize).

### Correct Mangle Way
```mangle
# Don't rely on short-circuiting for correctness:
result(X) :-
    cheap_check(X),
    expensive_check(X).

# If expensive_check should only run when cheap_check passes,
# structure your data so expensive_check only has facts for valid X:
Decl cheap_check(X.Type</atom>).
Decl expensive_check(X.Type</atom>).

cheap_check(/a).
cheap_check(/b).

# Only add expensive_check facts for items that passed cheap_check:
expensive_check(/a).  # /a passed cheap_check, so we computed this
# /b failed expensive check, so no fact

result(X) :-
    cheap_check(X),
    expensive_check(X).  # Only unifies with /a
```

---

## Anti-Pattern 4: Negation Before Binding

### Wrong Approach
```python
# Find items NOT in the exclusion list
for item in all_items:
    if item not in exclusion_list:
        result.append(item)
```

Attempting:
```mangle
# WRONG - negation before binding
result(X) :-
    not excluded(X),  # X is unbound!
    item(X).
```

### Why It Fails
**Safety rule:** Variables in `not` must be bound elsewhere. Order matters for safety!

### Correct Mangle Way
```mangle
# Bind X first, then negate:
result(X) :-
    item(X),          # X is now bound
    not excluded(X).  # Safe

# Order matters here! This is wrong:
# result(X) :- not excluded(X), item(X).  # UNSAFE
```

**Exception:** Negation DOES require specific ordering for safety. But this is a safety constraint, not control flow.

---

## Anti-Pattern 5: Aggregation Before Grouping

### Wrong Approach
```python
# Imperative: group, then sum
groups = group_by(data, key)
for group in groups:
    total = sum(group)
```

Attempting:
```mangle
# WRONG - aggregating before grouping variables are bound
total(T) :-
    data(Key, Value),
    T = fn:Sum(Value),     # What are we grouping by?
    group_key(Key).
```

### Why It Fails
Grouping variables must be clear and bound before aggregation.

### Correct Mangle Way
```mangle
# Use pipe syntax - explicit grouping first:
total_by_key(Key, T) :-
    data(Key, Value)
    |> do fn:group_by(Key)
    |> let T = fn:Sum(Value).

# Order in pipe matters:
# 1. Generate facts: data(Key, Value)
# 2. Group by Key
# 3. Aggregate within groups
```

**Note:** Pipe operations ARE ordered (transforms are sequential).

---

## Anti-Pattern 6: Multiple Rules Overwriting

### Wrong Approach
```python
# Later assignments overwrite earlier ones
x = 5
x = 10  # x is now 10
```

Attempting:
```mangle
# WRONG - expecting second rule to "overwrite"
value(X) :- base_value(X).
value(10).  # Expecting this to overwrite
```

### Why It Fails
Multiple rules create a **union**, not an overwrite. Both facts derive.

### Correct Mangle Way
```mangle
# Both rules fire - results are combined:
value(X) :- base_value(X).
value(10).

# If base_value(5) exists:
# Results: value(5), value(10)  (both derive!)

# For precedence, use explicit priority:
value(X) :-
    override_value(X).

value(X) :-
    not override_value(_),  # Only if no override
    base_value(X).

# Or use single rule with conditional logic:
value(X) :-
    override_value(X).

value(X) :-
    base_value(X),
    not override_value(_).
```

---

## Anti-Pattern 7: "First Match" Semantics

### Wrong Approach
```python
# Return first match
for item in items:
    if condition(item):
        return item  # First match wins
```

Attempting:
```mangle
# WRONG - expecting first rule to win
first_match(X) :- condition1(X).  # Try this first
first_match(X) :- condition2(X).  # Only if first doesn't match
```

### Why It Fails
**All** matching rules fire. Both will derive if both conditions are true.

### Correct Mangle Way
```mangle
# All matches derive:
match(X) :- condition1(X).
match(X) :- condition2(X).

# If X satisfies both, both derive!

# For priority/precedence:
best_match(X) :-
    condition1(X).  # Highest priority

best_match(X) :-
    not condition1(_),  # Only if condition1 has no matches
    condition2(X).

# Or explicitly rank:
match(X, 1) :- condition1(X).
match(X, 2) :- condition2(X).

best_match(X) :-
    match(X, Priority)
    |> do fn:group_by(X)
    |> let MinPriority = fn:Min(Priority)
    |> where Priority = MinPriority.
```

---

## Anti-Pattern 8: Assuming Stratification Order

### Wrong Approach
```python
# These run in order due to dependencies
step1 = compute_base()
step2 = compute_negation(step1)
step3 = compute_result(step2)
```

Attempting:
```mangle
# WRONG - relying on stratification execution order
stratum1(X) :- base(X).
stratum2(X) :- stratum1(X), not excluded(X).
stratum3(X) :- stratum2(X).

# Assuming stratum1 completes before stratum2 starts
```

### Why It Fails (Partially)
Stratification DOES create execution layers, but you shouldn't rely on the specific order for anything other than safety.

### Correct Mangle Way
```mangle
# Stratification ensures safety, not control flow:
stratum1(X) :- base(X).
stratum2(X) :- stratum1(X), not excluded(X).
stratum3(X) :- stratum2(X).

# This is correct! Stratification will compute:
# Layer 0: base facts
# Layer 1: stratum1 (depends on base)
# Layer 2: excluded (if it exists), stratum2 (negation of excluded)
# Layer 3: stratum3

# But don't write code that depends on observing intermediate strata
# Only the final fixed point is guaranteed
```

---

## Anti-Pattern 9: Ordered Choice

### Wrong Approach
```python
# Try option1, if it fails, try option2
try:
    result = option1()
except:
    result = option2()
```

Attempting:
```mangle
# WRONG - expecting ordered choice
result(R) :- option1(R).  # Try first
result(R) :- option2(R).  # Fallback
```

### Why It Fails
Both options derive if both succeed. No "fallback" semantics.

### Correct Mangle Way
```mangle
# Both succeed = both derive:
result(R) :- option1(R).
result(R) :- option2(R).

# For true fallback (only use option2 if option1 fails):
result(R) :- option1(R).

result(R) :-
    not option1(_),  # No results from option1
    option2(R).

# Or use priority:
result(R, /primary) :- option1(R).
result(R, /fallback) :- option2(R), not option1(_).

preferred_result(R) :-
    result(R, /primary).

preferred_result(R) :-
    not result(_, /primary),
    result(R, /fallback).
```

---

## Anti-Pattern 10: File Order in Multi-File Programs

### Wrong Approach
```c
// main.c includes header.h
// Assumes header.h is processed first
```

Attempting:
```mangle
# file1.gl
step1(X) :- input(X).

# file2.gl
step2(X) :- step1(X).  # Assumes file1 loaded first
```

### Why It Fails
File loading order shouldn't matter (though in practice, it might for imports).

### Correct Mangle Way
```mangle
# Write files so order doesn't matter:

# file1.gl
Decl step1(X.Type<int>).
step1(X) :- input(X).

# file2.gl
Decl step2(X.Type<int>).
step2(X) :- step1(X).

# Both files declare their predicates
# Evaluation happens after all files are loaded
# Order of loading doesn't affect semantics
```

---

## Anti-Pattern 11: Lazy vs Eager Evaluation

### Wrong Approach
```haskell
-- Haskell: lazy evaluation
result = expensive_function x  -- Not computed until needed
```

Attempting:
```mangle
# WRONG - expecting lazy evaluation
result(R) :-
    expensive_predicate(X, R),  # Computed only if result is queried?
    condition(X).
```

### Why It Fails
Mangle uses **eager evaluation** (or rather, fixed-point iteration). All derivations are computed.

### Correct Mangle Way
```mangle
# All facts are derived eagerly:
result(R) :-
    expensive_predicate(X, R),
    condition(X).

# expensive_predicate will be evaluated for all X
# If you want to avoid expensive computation, filter first:
result(R) :-
    condition(X),               # Cheap check first
    specific_id(X),             # Further narrowing
    expensive_predicate(X, R).  # Only computed for matching X

# But this is an optimization, not lazy evaluation
# All reachable facts will eventually derive
```

---

## Anti-Pattern 12: Statement-by-Statement Debugging

### Wrong Approach
```python
x = input()       # Line 1
y = process(x)    # Line 2 - set breakpoint here
z = finalize(y)   # Line 3
```

Attempting:
```mangle
# WRONG - expecting to "step through" rules
rule1(X) :- input(X).      # Can't put breakpoint here
rule2(Y) :- rule1(Y).      # Then step to here
rule3(Z) :- rule2(Z).      # Then here
```

### Why It Fails
Rules don't execute sequentially. You can't "step through" them.

### Correct Mangle Way
```mangle
# All rules evaluate until fixed point
# Debugging is about querying intermediate facts:

rule1(X) :- input(X).
rule2(Y) :- rule1(Y).
rule3(Z) :- rule2(Z).

# In Go, query intermediate results:
// After evaluation:
// results1 := store.Query("rule1", X)
// log.Printf("rule1 results: %v", results1)
//
// results2 := store.Query("rule2", Y)
// log.Printf("rule2 results: %v", results2)

# Or add debug predicates:
Decl debug_rule1(X.Type<int>).
debug_rule1(X) :- input(X).  # Exactly same as rule1

# Query debug predicates to trace derivations
```

---

## What Order DOES Matter

While Mangle is mostly order-independent, these DO depend on order:

### 1. Safety in Negation

```mangle
# WRONG - unsafe
result(X) :- not excluded(X), item(X).

# RIGHT - safe
result(X) :- item(X), not excluded(X).
```

### 2. Aggregation Pipeline

```mangle
# Pipe operations are sequential:
result(Key, Avg) :-
    data(Key, Value)
    |> do fn:group_by(Key)        # Step 1
    |> let Sum = fn:Sum(Value)     # Step 2
    |> let Count = fn:Count(Value) # Step 3
    |> let Avg = fn:div(Sum, Count). # Step 4

# Order matters here!
```

### 3. Stratification (for safety, not control flow)

```mangle
# Strata are computed in order:
# Stratum 0: base facts
# Stratum 1: rules without negation
# Stratum 2: rules with negation of Stratum 1 predicates
# Etc.

# But this is automatic - you don't control it
```

---

## Key Principle: Declarative Semantics

| Imperative | Declarative (Mangle) |
|------------|----------------------|
| Rules execute in order | All rules apply simultaneously |
| Atoms execute in order | Atoms are conjuncts (AND) |
| Short-circuit evaluation | All predicates evaluated |
| Variables assigned sequentially | Variables bound by unification |
| Later rules overwrite | Rules create union |
| First match wins | All matches derive |
| Try-catch fallback | Explicit fallback with negation |
| Lazy evaluation | Eager (fixed-point) evaluation |
| Step-by-step debugging | Query intermediate facts |

**Mental Model:** "What are all the facts that can be derived?" not "What steps execute in order?"

---

## Migration Checklist

When translating sequential code to Mangle:

- [ ] Don't rely on rule definition order
- [ ] Don't rely on atom order in body (except for safety)
- [ ] Don't expect short-circuit evaluation
- [ ] Bind all variables before negation
- [ ] Use pipe syntax for ordered transformations
- [ ] Remember multiple rules = union, not overwrite
- [ ] Use explicit precedence for "first match" semantics
- [ ] Don't rely on observing intermediate strata
- [ ] Use negation for fallback, not rule order
- [ ] File order shouldn't affect semantics
- [ ] All derivations are eager (fixed-point)
- [ ] Debug by querying facts, not stepping through rules
- [ ] Think "what's true" not "what executes when"

---

## Pro Tip: Embrace Declarative Thinking

Instead of asking:
> "In what order do these rules run?"

Ask:
> "What facts will exist after fixed-point evaluation?"

The beauty of Mangle is that you specify WHAT you want, not HOW to compute it. Order is an implementation detail (mostly).
