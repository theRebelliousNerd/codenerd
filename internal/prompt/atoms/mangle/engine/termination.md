# Mangle Engine: Termination Conditions

## Guaranteed Termination: Pure Datalog

Pure Datalog (without extensions) provides **guaranteed termination** due to these properties:

1. **Monotonicity** - Can only add facts, never remove
2. **Finite domain** - All values come from a finite set of constants
3. **No function symbols** - Cannot generate new values
4. **Safety constraints** - All variables must be bound

### Why Pure Datalog Terminates

Given a finite set of constants C, the maximum number of facts for a predicate p with arity n is:
```
|C|^n
```

Since rules can only derive facts from existing constants, and the fact space is finite, evaluation **must** reach a fixpoint.

## Lost Guarantees: Mangle Extensions

The Mangle documentation states:

> "Mangle contains Datalog as a fragment and adds extensions that make its use more practical. Some of the good properties like guaranteed termination are lost when such extensions are used."

### Extensions That Break Termination

1. **Function Calls**
   - Generate new values not in the original constant set
   - Example: `fn:plus(X, 1)` creates unbounded integers

2. **Aggregation**
   - May require computing over infinite sets
   - Grouping can create new structured values

3. **Structured Data**
   - Lists, maps, structs can be arbitrarily nested
   - Each nesting level multiplies the value space

4. **Unbounded Recursion**
   - Without proper base cases, rules recurse forever

## Common Non-Termination Patterns

### Counter Pattern (DANGEROUS)

```mangle
# Infinite counter - WILL NOT TERMINATE
count(N) :- count(M), N = fn:plus(M, 1).
count(0).
```

**Why it fails**: Each iteration generates a new value, never reaching fixpoint.

### List Growth Pattern (DANGEROUS)

```mangle
# Infinite list builder - WILL NOT TERMINATE
grow(L) :- grow(M), L = fn:cons(1, M).
grow([]).
```

**Why it fails**: Generates infinitely nested lists.

### Unconstrained String Generation (DANGEROUS)

```mangle
# Infinite string concatenation - WILL NOT TERMINATE
message(S) :- message(M), S = fn:string_concat(M, "!").
message("hello").
```

**Why it fails**: Creates unbounded strings.

## Safe Termination Patterns

### Bounded Counter

```mangle
# Safe counter with limit
count(N) :- count(M), N = fn:plus(M, 1), N < 100.
count(0).
```

**Why it terminates**: The `N < 100` constraint bounds the recursion.

### Finite Domain Recursion

```mangle
# Recurse over finite set
ancestor(X, Y) :- parent(X, Y).
ancestor(X, Z) :- parent(X, Y), ancestor(Y, Z).
```

**Why it terminates**: `parent` is a finite extensional relation. Ancestors can only be derived from existing people.

### Bounded List Processing

```mangle
# Process finite list
sum_list([], 0).
sum_list([H|T], S) :-
    sum_list(T, SubSum),
    S = fn:plus(H, SubSum).
```

**Why it terminates**: Input lists are finite, recursion decreases list size each step.

## Ensuring Termination

### Rule Design Guidelines

1. **Base cases first** - Always define terminating conditions
2. **Monotonic decrease** - Recursive calls should work on smaller inputs
3. **Finite domains** - Ensure all recursion is over finite sets
4. **Explicit bounds** - Add numeric constraints when using arithmetic
5. **Structural recursion** - Recurse on substructures (list tails, smaller numbers)

### Verification Checklist

Before deploying Mangle rules, verify:

- [ ] Every recursive rule has a base case
- [ ] Recursive calls work on "smaller" inputs
- [ ] No unbounded value generation (counters, string building, etc.)
- [ ] All function applications are bounded by constraints
- [ ] Fact limits are set appropriately

### Testing for Termination

```go
// Always test with fact limits during development
opts := []engine.EvalOption{
    engine.WithCreatedFactLimit(10_000),  // Conservative limit
}

err := engine.EvalProgram(program, facts, opts...)
if err != nil {
    // If this triggers, review rules for non-termination
    log.Printf("Evaluation may not terminate: %v", err)
}
```

## Stratification and Termination

Stratified negation does NOT affect termination guarantees:

- **Each stratum** must terminate independently
- **Negation** cannot introduce non-termination (it's only a lookup)
- **Stratification check** fails if cycles exist through negation

If stratification succeeds, termination depends only on the positive rules in each stratum.

## Fixpoint Detection

The engine reaches fixpoint when:

1. **No new facts** are derived in an iteration
2. **All rules** have been applied to all fact combinations
3. **Fact store is stable** - subsequent iterations would be no-ops

### Fixpoint Guarantees

- **Pure Datalog**: Fixpoint is unique and will be reached
- **Mangle with extensions**: Fixpoint may not exist (non-termination)
- **Stratified programs**: Each stratum reaches fixpoint, then next proceeds

## Debugging Non-Termination

If evaluation doesn't terminate:

1. **Check statistics** - Use `EvalProgramWithStats` to see fact growth
2. **Inspect new facts** - Look for patterns (incrementing numbers, growing lists)
3. **Identify culprit rule** - Which rule is generating unbounded facts?
4. **Add constraints** - Bound the recursive case
5. **Restructure logic** - Sometimes the approach needs rethinking

### Example Debug Session

```go
stats, err := engine.EvalProgramWithStats(program, facts, opts...)
fmt.Printf("Facts created: %d\n", stats.FactsCreated)
fmt.Printf("Iterations: %d\n", stats.Iterations)

// If FactsCreated approaches the limit and Iterations is high,
// suspect non-termination
```
