# Mangle Analysis: Stratification

## What is Stratification?

Stratification is the process of separating predicates into **layers (strata)** such that:

1. Each layer only depends on **lower layers**
2. Negation only references **strictly lower layers**
3. Recursion within a layer is **positive only** (no negation)

This enables safe evaluation of programs with negation.

## Why Stratification Matters

Pure Datalog has no negation - everything is monotonic (adding facts).

With negation, we face a problem:
```mangle
# What does this mean?
p(X) :- not q(X).
q(X) :- not p(X).
```

If `p` is true, then `q` must be false (from second rule), which makes `p` false (from first rule), contradiction!

Stratification prevents such circular dependencies through negation.

## Stratify Function

The `analysis.Stratify` function checks whether a program can be stratified.

### Returns

On **success**:
- List of **strongly-connected components** (SCCs)
- Map from **predicate to stratum number**
- Strata are **topologically sorted**

On **failure**:
- Returns an **error** indicating why stratification failed

### What It Checks

1. **Dependency graph** - Which predicates depend on which others
2. **Negative edges** - Which dependencies go through negation
3. **Cycles** - Are there cycles involving negation?

If a cycle passes through negation → **STRATIFICATION FAILS**

## Valid Stratification Examples

### Example 1: Two Strata

```mangle
# Stratum 0 (base facts)
parent(/alice, /bob).
parent(/bob, /charlie).

# Stratum 1 (depends on stratum 0)
ancestor(X, Y) :- parent(X, Y).
ancestor(X, Z) :- parent(X, Y), ancestor(Y, Z).

# Stratum 2 (uses negation of stratum 1)
unrelated(X, Y) :-
    person(X),
    person(Y),
    not ancestor(X, Y),
    not ancestor(Y, X).
```

**Why this works**:
- `ancestor` is computed to fixpoint first (stratum 1)
- `unrelated` uses negation of `ancestor` (stratum 2)
- No cycle through negation

### Example 2: Sequential Computation

```mangle
# Stratum 0
reachable(X, Y) :- edge(X, Y).
reachable(X, Z) :- edge(X, Y), reachable(Y, Z).

# Stratum 1
unreachable(X, Y) :- node(X), node(Y), not reachable(X, Y).
```

Evaluation order:
1. Compute all `reachable` facts (fixpoint)
2. Then compute `unreachable` using negation

## Invalid Stratification Examples

### Example 1: Cycle Through Negation

```mangle
# INVALID - cannot stratify
p(X) :- not q(X).
q(X) :- not p(X).
```

**Why it fails**: `p` depends on negation of `q`, and `q` depends on negation of `p`. This is a cycle through negation.

### Example 2: Self-Negation in Recursion

```mangle
# INVALID - cannot stratify
reachable(X, Y) :- edge(X, Y).
reachable(X, Z) :-
    reachable(X, Y),
    edge(Y, Z),
    not reachable(X, Z).  # Negates itself!
```

**Why it fails**: `reachable` depends on its own negation. Cannot compute to fixpoint.

### Example 3: Indirect Cycle

```mangle
# INVALID - cannot stratify
a(X) :- b(X), not c(X).
b(X) :- c(X).
c(X) :- a(X).
```

**Why it fails**: `a` → (negates) `c` → `a`, forming a cycle.

## Stratification Rules

### Rule 1: No Recursion Through Negation

**CRITICAL**: Negation cannot be part of predicate definitions that recursively depend on each other.

```mangle
# INVALID
ancestor(X, Y) :- parent(X, Y).
ancestor(X, Z) :- ancestor(X, Y), not ancestor(Y, Z).  # BAD
```

### Rule 2: Positive Recursion is Fine

Within a single stratum, positive recursion is allowed:

```mangle
# VALID - positive recursion in same stratum
ancestor(X, Y) :- parent(X, Y).
ancestor(X, Z) :- parent(X, Y), ancestor(Y, Z).
```

### Rule 3: Negation Only References Lower Strata

```mangle
# Stratum 0
base(X) :- data(X).

# Stratum 1 - can negate stratum 0
derived(X) :- candidate(X), not base(X).
```

## Stratified Evaluation Process

Once stratification succeeds, evaluation proceeds:

1. **Sort strata topologically** - Lower strata first
2. **For each stratum**:
   - Treat all lower strata as **extensional** (fixed facts)
   - Compute stratum to **fixpoint** using semi-naive evaluation
   - Derived facts become **fixed** for next stratum
3. **Continue** until all strata are evaluated

### Example Evaluation

```mangle
# Stratum 0
edge(/a, /b).
edge(/b, /c).

# Stratum 1
path(X, Y) :- edge(X, Y).
path(X, Z) :- path(X, Y), edge(Y, Z).

# Stratum 2
no_path(X, Y) :- node(X), node(Y), not path(X, Y).
```

**Evaluation steps**:
1. Load `edge` facts (extensional)
2. Compute `path` to fixpoint → `path(/a,/b)`, `path(/b,/c)`, `path(/a,/c)`
3. Treat `path` as fixed, compute `no_path`

## Semipositive Datalog

Programs that stratify are called **semipositive Datalog**:

- **Semipositive** = negation applies only to extensional relations (in lower strata)
- **Positive** within each stratum (recursion without negation)

These programs can be safely composed sequentially.

## Strongly-Connected Components (SCCs)

The `Stratify` function returns SCCs, which are:

- Sets of predicates that **mutually depend** on each other
- Within an SCC, all predicates are in the **same stratum**
- SCCs are **topologically sorted** by dependency

### Example SCC Analysis

```mangle
ancestor(X, Y) :- parent(X, Y).
ancestor(X, Z) :- ancestor(X, Y), parent(Y, Z).

descendant(X, Y) :- ancestor(Y, X).
```

**SCCs**:
- SCC 1: `{ancestor}` - recursive
- SCC 2: `{descendant}` - depends on SCC 1

## Checking Stratification in Go

```go
import "github.com/google/mangle/analysis"

// Check if program can be stratified
strata, predToStratum, err := analysis.Stratify(program)
if err != nil {
    log.Printf("Cannot stratify: %v", err)
    // Program uses negation incorrectly
    return
}

// Use stratified evaluation
for i, stratum := range strata {
    log.Printf("Stratum %d: %v", i, stratum)
}
```

## Debugging Stratification Failures

If stratification fails:

1. **Identify the cycle** - Which predicates form the cycle?
2. **Find negations** - Which rules use negation?
3. **Break the cycle**:
   - Remove negation, OR
   - Restructure rules to eliminate circular dependency
4. **Verify** - Run `Stratify` again

### Common Fixes

**Problem**: Self-negation in recursive rule

```mangle
# INVALID
reachable(X, Y) :- edge(X, Y), not reachable(X, Y).
```

**Fix**: Remove the negation (it makes no logical sense anyway)

```mangle
# VALID
reachable(X, Y) :- edge(X, Y).
```

**Problem**: Mutual negation

```mangle
# INVALID
a(X) :- not b(X).
b(X) :- not a(X).
```

**Fix**: Redesign logic - this is likely a modeling error.

## Stratification and Termination

Stratification does NOT affect termination:

- Each stratum must **terminate independently**
- Negation is only a **lookup** (doesn't add facts)
- If all strata terminate, the whole program terminates

If stratification succeeds, termination depends only on the **positive rules** in each stratum.

## Best Practices

1. **Design in layers** - Think about dependency order upfront
2. **Negation last** - Compute positive facts first, then negate
3. **Test stratification** - Run `Stratify` during development
4. **Document strata** - Comment which rules are in which layer
5. **Avoid mutual negation** - Usually indicates a logic error
