# Failure Mode 5: Stratification Violations (Negative Cycles)

## Category
Semantic Safety & Logic (Datalog Stratification)

## Severity
CRITICAL - Program rejected at analysis time

## Error Pattern
Creating circular dependencies through negation. Mangle rejects programs where predicate A depends on "not B" and B depends on "not A" (or longer cycles).

## Wrong Code
```mangle
# WRONG - Direct negative cycle (game theory fallacy)
winning(X) :- move(X, Y), losing(Y).
losing(X) :- not winning(X).  # STRATIFICATION ERROR!
# Cycle: winning → losing → NOT winning

# WRONG - Mutual negation
good(X) :- not bad(X).
bad(X) :- not good(X).
# Cycle: good → NOT bad → NOT good

# WRONG - Indirect cycle through negation
a(X) :- b(X), not c(X).
b(X) :- c(X).
c(X) :- not a(X).
# Cycle: a → NOT c → NOT a

# WRONG - Self-referential negation
stable(X) :- item(X), not stable(Y), related(X, Y).
# Direct negative self-reference

# WRONG - Negation in recursive closure
reachable(X, Y) :- edge(X, Y).
reachable(X, Z) :- reachable(X, Y), edge(Y, Z), not blocked(X, Z).
blocked(X, Z) :- reachable(X, Z), dangerous(Z).
# Cycle: reachable → NOT blocked → reachable
```

## Correct Code
```mangle
# CORRECT - Add termination condition (no cycle)
terminal_loss(X) :- position(X), not has_move(X).
has_move(X) :- move(X, _).
losing(X) :- terminal_loss(X).  # Base case
winning(X) :- move(X, Y), losing(Y).  # Derived from base

# CORRECT - Use explicit stratification with depth
losing_at_depth(X, 0) :- terminal_loss(X).
winning_at_depth(X, D) :-
    move(X, Y),
    losing_at_depth(Y, D1),
    D = fn:plus(D1, 1),
    D < 10.  # Depth limit prevents infinite recursion

# CORRECT - One-directional dependency
good(X) :- quality_check(X, /passed).
bad(X) :- quality_check(X, /failed).
# No cycle: both derive from same base predicate

# CORRECT - Negation only in final layer
# Layer 1: Base facts
base_valid(X) :- source(X), constraint(X, /ok).

# Layer 2: Recursive closure (no negation)
transitively_valid(X) :- base_valid(X).
transitively_valid(X) :- transitively_valid(Y), extends(X, Y).

# Layer 3: Filtering with negation (after closure complete)
final_valid(X) :- transitively_valid(X), not excluded(X).

# CORRECT - Stratified reachability with blocking
# Layer 1: Compute all reachable nodes
reachable(X, Y) :- edge(X, Y).
reachable(X, Z) :- reachable(X, Y), edge(Y, Z).

# Layer 2: Filter using negation (after closure)
safe_reachable(X, Y) :- reachable(X, Y), not dangerous(Y).
```

## Detection
- **Symptom**: Error like "stratification violation" or "negative cycle detected"
- **Symptom**: Analysis phase rejects program before evaluation
- **Pattern**: Predicate A's rules contain `not B`, and B's rules contain `not A`
- **Pattern**: Predicate depends on its own negation (directly or indirectly)
- **Test**: Draw dependency graph; look for cycles passing through negation

## Prevention

### Stratification Rules
1. **No cycles through negation**: If A depends on "not B", then B cannot depend on A (even indirectly)
2. **Recursion and negation cannot mix**: Recursive predicates cannot use negation on themselves
3. **Layer your rules**: Compute complete recursive closures BEFORE applying negation

### Dependency Graph Check
For each predicate, trace dependencies:
```
A depends on B (positive) → Draw edge A → B
A depends on NOT C → Draw dashed edge A ⇢ C

Check: No cycles containing dashed edges!
```

### Mental Model: Stratified Layers

Think of your program as layers, where each layer can only reference:
- **Same layer** (but not through negation)
- **Lower layers** (including through negation)

```
Layer 3: final_results(X) :- layer2(X), not excluded(X).
           ↑ Can negate Layer 2 ↑
Layer 2: derived(X) :- base(X), recursive_step(X).
           ↑ Recursion OK here ↑
Layer 1: base(X) :- external_data(X).
           ↑ Base facts ↑
```

## Correct Patterns Reference

### Pattern 1: Termination Condition
```mangle
# Recursive closure with base case
ancestor(X, Y) :- parent(X, Y).  # Base
ancestor(X, Z) :- parent(X, Y), ancestor(Y, Z).  # Recursive

# Filter AFTER closure is complete
not_ancestor(X, Y) :-
    person(X),
    person(Y),
    not ancestor(X, Y).  # Safe: ancestor fully computed
```

### Pattern 2: Explicit Depth/Counter
```mangle
# Bounded recursion with explicit counter
path(X, Y, 1) :- edge(X, Y).
path(X, Z, D) :-
    edge(X, Y),
    path(Y, Z, D1),
    D = fn:plus(D1, 1),
    D < 10,              # Explicit bound
    not blocked(X, Z).   # Safe: no cycle through blocked

# blocked doesn't depend on path
blocked(X, Y) :- blacklist(X, Y).
```

### Pattern 3: Separate Concerns
```mangle
# Layer 1: Compute candidates (recursive)
candidate(X) :- seed(X).
candidate(X) :- candidate(Y), generates(Y, X).

# Layer 2: Compute exclusions (no recursion)
excluded(X) :- blacklist(X).
excluded(X) :- candidate(X), fails_check(X).

# Layer 3: Final result (negation on lower layers)
final(X) :- candidate(X), not excluded(X).
```

### Pattern 4: Mutual Exclusion Without Negation
```mangle
# Instead of:
# good(X) :- not bad(X).  # Creates cycle if bad uses not good
# bad(X) :- not good(X).

# Use explicit categorization:
category(X, /good) :- quality(X, Score), Score >= 80.
category(X, /bad) :- quality(X, Score), Score < 40.
category(X, /medium) :- quality(X, Score), Score >= 40, Score < 80.

# Then derive:
good(X) :- category(X, /good).
bad(X) :- category(X, /bad).
```

## Complex Example: Game State Analysis

```mangle
# WRONG - Cycle through negation
# winning(X) :- move(X, Y), not winning(Y).  # ILLEGAL!

# CORRECT - Stratified by depth
Decl position(Pos.Type<n>).
Decl move(From.Type<n>, To.Type<n>).
Decl winning_at_depth(Pos.Type<n>, Depth.Type<int>).
Decl losing_at_depth(Pos.Type<n>, Depth.Type<int>).

# Layer 1: Terminal positions (base facts)
terminal(X) :- position(X), not has_move_from(X).
has_move_from(X) :- move(X, _).

# Layer 2: Losing at depth 0 (no moves = loss)
losing_at_depth(X, 0) :- terminal(X).

# Layer 3: Winning if can move to losing position
winning_at_depth(X, D) :-
    move(X, Y),
    losing_at_depth(Y, D1),
    D = fn:plus(D1, 1),
    D < 20.  # Search depth limit

# Layer 4: Losing if all moves lead to winning (only compute after winning)
all_moves_winning(X, D) :-
    position(X),
    has_move_from(X),
    not exists_losing_move(X, D).

exists_losing_move(X, D) :-
    move(X, Y),
    not winning_at_depth(Y, D).

losing_at_depth(X, D) :-
    all_moves_winning(X, D1),
    D = fn:plus(D1, 1),
    D < 20.
```

## Why Stratification Matters

### Semantic Consistency
Without stratification, negation can have **no stable meaning**:

```mangle
# What is the truth value of p?
p :- not q.
q :- not p.

# If p is true → q is false → p should be false (contradiction!)
# If p is false → q is true → p should be true (contradiction!)
# No consistent solution exists!
```

### Mangle's Solution
Reject such programs at analysis time, forcing you to restructure logic into layers where meaning is unambiguous.

## Training Bias Origins
| Language | Pattern | Leads to Wrong Mangle |
|----------|---------|----------------------|
| Prolog | Negation as failure with backtracking | Cycles "work" through search |
| SQL | Recursive CTEs with NOT EXISTS | Limited recursion hides issue |
| Game Theory | "Winning = not losing" | Direct negative dependency |

## Quick Check
For every rule using negation:
1. Draw dependency graph of predicates
2. Mark negated dependencies with dashed lines
3. Find cycles: Any path from A back to A?
4. If cycle contains dashed line → **STRATIFICATION VIOLATION**
5. Restructure: Add base cases, explicit depth, or separate layers

## Debugging Aid
```mangle
# To find dependencies, add trace rules:
depends_on(A, B) :- rule_uses(A, B).  # Track which rules use which predicates
depends_on_negated(A, B) :- rule_uses_negation(A, B).

# Find cycles:
cycle(X) :- depends_on(X, X).
cycle(X) :- depends_on(X, Y), cycle(Y).
negative_cycle(X) :- depends_on_negated(X, Y), depends_on(Y, X).
```
