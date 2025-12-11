# Mangle Stratification

## What is Stratification?

Stratification ensures that negation is evaluated in a well-defined order. A program is stratifiable if predicates can be organized into layers (strata) where:
- Predicates in stratum N only depend on predicates in strata < N through negation
- No predicate depends on itself through negation (directly or indirectly)

## Stratification Errors

### Direct Cycle (Immediate)
```mangle
# INVALID - p depends on not p
p(X) :- not p(X).
```

### Mutual Negation Cycle
```mangle
# INVALID - p <-> q through negation
p(X) :- not q(X).
q(X) :- not p(X).
```

### Indirect Cycle (Transitive)
```mangle
# INVALID - a -> not b -> c -> not a
a(X) :- not b(X).
b(X) :- c(X).
c(X) :- not a(X).
```

## How to Fix

### 1. Use Positive Intermediates
**Before:**
```mangle
blocked(X) :- not allowed(X).
allowed(X) :- not blocked(X).
```

**After:**
```mangle
# Base facts determine state
allowed(X) :- user(X), has_permission(X).
blocked(X) :- user(X), not allowed(X).
```

### 2. Break with Ground Facts
**Before:**
```mangle
winner(X) :- player(X), not loser(X).
loser(X) :- player(X), not winner(X).
```

**After:**
```mangle
# Determine winner from game result (ground truth)
winner(X) :- game_result(X, /won).
loser(X) :- player(X), not winner(X).
```

### 3. Separate Domains
**Before:**
```mangle
active(X) :- not inactive(X).
inactive(X) :- not active(X).
```

**After:**
```mangle
# Derive from explicit status
active(X) :- status(X, /active).
inactive(X) :- status(X, /inactive).
```

## Detection Checklist

1. Draw the predicate dependency graph
2. Mark edges that go through negation
3. Check for cycles that include negation edges
4. If a cycle exists through negation, restructure

## Safe Patterns

### Default with Override
```mangle
# Safe: defaults to open, override to close
open(X) :- domain(X), not closed(X).
closed(X) :- explicitly_closed(X).
```

### Hierarchical Negation
```mangle
# Safe: each level only negates lower levels
level1(X) :- base_fact(X).
level2(X) :- domain(X), not level1(X).
level3(X) :- domain(X), not level2(X), not level1(X).
```
