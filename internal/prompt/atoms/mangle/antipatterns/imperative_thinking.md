# Anti-Pattern: Imperative Thinking

## Category
Paradigm Mismatch (Declarative vs Imperative)

## Description
Trying to write imperative, step-by-step procedural code when Mangle is purely declarative. Mangle describes WHAT is true, not HOW to compute it.

---

## Anti-Pattern 1: Sequential Steps / Ordering

### Wrong Approach
```python
# Imperative thinking:
x = 5
y = x + 1
z = y * 2
print(z)
```

Attempting in Mangle:
```mangle
# WRONG - trying to enforce order
step1(X) :- X = 5.
step2(Y) :- step1(X), Y = fn:plus(X, 1).
step3(Z) :- step2(Y), Z = fn:times(Y, 2).
```

### Why It Fails
Mangle rules have **no execution order**. All rules are evaluated simultaneously until a fixed point.

### Correct Mangle Way
```mangle
# Declare the relationships, not the steps:
result(Z) :-
    X = 5,
    Y = fn:plus(X, 1),
    Z = fn:times(Y, 2).

# Or derive each level separately (order doesn't matter):
value(5).
incremented(Y) :- value(X), Y = fn:plus(X, 1).
final(Z) :- incremented(Y), Z = fn:times(Y, 2).
```

**Key Insight:** Rules can be written in ANY order. Mangle finds all facts that satisfy the logical constraints.

---

## Anti-Pattern 2: Loops / Iteration

### Wrong Approach
```javascript
// Imperative loop:
for (i = 0; i < 10; i++) {
    result[i] = i * 2;
}
```

Attempting:
```mangle
# WRONG - no loop construct
loop(I, Result) :-
    I < 10,
    Result = fn:times(I, 2),
    I = fn:plus(I, 1).  # Can't mutate I!
```

### Why It Fails
No loop constructs exist. Iteration is achieved through **recursion** or **derivation over a domain**.

### Correct Mangle Way
```mangle
# Option 1: Pre-load domain and derive
# In Go: load facts number(0). number(1). ... number(9).

double(N, Result) :-
    number(N),
    N < 10,
    Result = fn:times(N, 2).

# Option 2: Recursive generation (careful - needs bound!)
Decl generate(N.Type<int>).

generate(0).
generate(N) :-
    generate(M),
    M < 10,
    N = fn:plus(M, 1).

double(N, Result) :-
    generate(N),
    Result = fn:times(N, 2).
```

**Warning:** Unbounded recursion will diverge! Always include a termination condition.

---

## Anti-Pattern 3: Variable Assignment / Mutation

### Wrong Approach
```java
int x = 5;
x = x + 1;  // Mutation
x = x * 2;
```

Attempting:
```mangle
# WRONG - trying to mutate X
compute(X) :-
    X = 5,
    X = fn:plus(X, 1),  # X is already bound to 5!
    X = fn:times(X, 2).
```

### Why It Fails
Variables are **immutable bindings**, not storage locations. Once `X = 5`, it stays 5.

### Correct Mangle Way
```mangle
# Use different variables for each step:
compute(Z) :-
    X = 5,
    Y = fn:plus(X, 1),
    Z = fn:times(Y, 2).

# Or chain derivations:
step1(5).
step2(Y) :- step1(X), Y = fn:plus(X, 1).
step3(Z) :- step2(Y), Z = fn:times(Y, 2).
```

---

## Anti-Pattern 4: Early Return / Break

### Wrong Approach
```python
def find(items):
    for item in items:
        if item.matches:
            return item  # Early return
    return None
```

Attempting:
```mangle
# WRONG - no return statement
find_item(Item) :-
    items(Item),
    matches(Item),
    return(Item).  # Not a thing!
```

### Why It Fails
No `return` or `break`. All matching facts are derived.

### Correct Mangle Way
```mangle
# Derive all matches:
matching_item(Item) :-
    items(Item),
    matches(Item).

# To get "first" match, pick in Go:
// results := store.Query("matching_item", X)
// if len(results) > 0 {
//     first := results[0]
// }
```

---

## Anti-Pattern 5: If-Else Branching

### Wrong Approach
```javascript
if (x > 10) {
    result = "high";
} else if (x > 5) {
    result = "medium";
} else {
    result = "low";
}
```

Attempting:
```mangle
# WRONG - no if/else statements
classify(X, Result) :-
    if X > 10 then
        Result = /high
    else if X > 5 then
        Result = /medium
    else
        Result = /low.
```

### Why It Fails
No `if/else` syntax. Use **separate rules** with guards.

### Correct Mangle Way
```mangle
classify(X, /high) :- X > 10.
classify(X, /medium) :- X > 5, X <= 10.
classify(X, /low) :- X <= 5.

# All applicable rules fire. If X=12:
# - classify(12, /high) derives
# - Other rules don't match
```

**Note:** Multiple rules may derive for the same input if their conditions overlap. Make them mutually exclusive if needed.

---

## Anti-Pattern 6: Try-Catch Error Handling

### Wrong Approach
```python
try:
    result = risky_operation()
except Exception:
    result = default_value
```

Attempting:
```mangle
# WRONG - no exceptions
safe_result(R) :-
    try {
        risky(R)
    } catch {
        R = /default
    }.
```

### Why It Fails
Mangle is pure logic. No exceptions, no errors in rules.

### Correct Mangle Way
```mangle
# Model success/failure as predicates:
safe_result(R) :- success(R).
safe_result(/default) :- not success(_).

# Or use optional pattern:
result_or_default(R) :- risky(R).
result_or_default(/default) :-
    not risky(_),
    default(/default).
```

**Design:** Errors are handled in Go before/after Mangle evaluation.

---

## Anti-Pattern 7: State Machine with Transitions

### Wrong Approach
```javascript
state = "start";
while (true) {
    switch (state) {
        case "start": state = "processing"; break;
        case "processing": state = "done"; break;
        case "done": return;
    }
}
```

Attempting:
```mangle
# WRONG - no mutable state
transition(State) :-
    State = /start,
    State = /processing,  # State can't be two things!
    State = /done.
```

### Why It Fails
No mutable state. States are facts, transitions are rules.

### Correct Mangle Way
```mangle
# Model transitions as facts:
Decl state(Time.Type<int>, State.Type</atom>).
Decl transition(From.Type</atom>, To.Type</atom>).

# Base fact:
state(0, /start).

# Transition rules:
transition(/start, /processing).
transition(/processing, /done).

# Derive next states:
state(T, To) :-
    state(T0, From),
    transition(From, To),
    T = fn:plus(T0, 1).

# Query: state(T, /done)?  # When do we reach done?
```

---

## Anti-Pattern 8: Counter / Accumulator

### Wrong Approach
```python
count = 0
for item in items:
    count += 1
return count
```

Attempting:
```mangle
# WRONG - can't mutate counter
count_items(Count) :-
    Count = 0,
    items(Item),
    Count = fn:plus(Count, 1).  # Count already 0!
```

### Why It Fails
No accumulator variable. Use **aggregation**.

### Correct Mangle Way
```mangle
count_items(Count) :-
    items(Item)
    |> do fn:group_by()
    |> let Count = fn:Count(Item).

# Or count via recursion (for learning purposes):
Decl item_list(Items.Type<list>).
Decl count(N.Type<int>).

count(N) :-
    item_list(Items),
    count_helper(Items, N).

Decl count_helper(List.Type<list>, N.Type<int>).

count_helper([], 0).
count_helper(List, N) :-
    :match_cons(List, _, Tail),
    count_helper(Tail, N1),
    N = fn:plus(N1, 1).
```

---

## Anti-Pattern 9: While Loop / Do-While

### Wrong Approach
```c
while (x < 100) {
    x = x * 2;
}
```

Attempting:
```mangle
# WRONG - no while loop
loop(X) :-
    X < 100,
    X = fn:times(X, 2),
    loop(X).  # Infinite unification!
```

### Why It Fails
No loop constructs. Use **bounded recursion**.

### Correct Mangle Way
```mangle
# Recursive doubling with explicit steps:
Decl double_until(Step.Type<int>, Value.Type<int>).

double_until(0, 1).  # Start value

double_until(Step, Value) :-
    double_until(PrevStep, PrevValue),
    PrevValue < 100,
    Value = fn:times(PrevValue, 2),
    Step = fn:plus(PrevStep, 1),
    Step < 20.  # Safety limit!

# Final value:
final_value(V) :-
    double_until(_, V),
    V >= 100.
```

---

## Anti-Pattern 10: Function Calls with Side Effects

### Wrong Approach
```python
def process(x):
    log("Processing: " + str(x))  # Side effect!
    db.save(x)                    # Side effect!
    return x * 2

result = process(5)
```

Attempting:
```mangle
# WRONG - predicates can't have side effects
process(X, Result) :-
    log("Processing", X),
    db_save(X),
    Result = fn:times(X, 2).
```

### Why It Fails
Mangle predicates are **pure**. No I/O, no database writes, no logging from within rules.

### Correct Mangle Way
```mangle
# Just derive facts:
process_result(X, Result) :-
    input_data(X),
    Result = fn:times(X, 2).

# Handle side effects in Go:
// results := store.Query("process_result", X, Result)
// for _, r := range results {
//     log.Printf("Processing: %v", r.X)
//     db.Save(r.X)
// }
```

---

## Anti-Pattern 11: Recursion Without Base Case

### Wrong Approach
```python
def infinite():
    return infinite()  # Stack overflow!
```

Attempting:
```mangle
# WRONG - unbounded recursion
infinite(X) :- infinite(X).

# Or:
generate(N) :-
    generate(M),
    N = fn:plus(M, 1).  # No limit!
```

### Why It Fails
Mangle will attempt to derive infinitely until hitting step limits or crashing.

### Correct Mangle Way
```mangle
# Always have a base case:
fibonacci(0, 0).
fibonacci(1, 1).

fibonacci(N, Result) :-
    N > 1,
    N < 50,  # Safety limit!
    N1 = fn:minus(N, 1),
    N2 = fn:minus(N, 2),
    fibonacci(N1, F1),
    fibonacci(N2, F2),
    Result = fn:plus(F1, F2).
```

---

## Anti-Pattern 12: Null/Undefined Assignment

### Wrong Approach
```javascript
let x;  // undefined
if (condition) {
    x = 5;
}
// x might be undefined
```

Attempting:
```mangle
# WRONG - no null/undefined
value(X) :-
    condition(true),
    X = 5.
value(null) :-  # Not a thing!
    not condition(true).
```

### Why It Fails
No `null` or `undefined`. Facts either exist or don't (Closed World Assumption).

### Correct Mangle Way
```mangle
# Use optional pattern:
value(/some, 5) :- condition(true).
value(/none) :- not condition(true).

# Or just check existence:
has_value(5) :- condition(true).

# In Go, check if query returns results:
// results := store.Query("has_value", X)
// if len(results) == 0 {
//     // No value
// }
```

---

## Key Principles: Declarative vs Imperative

| Imperative | Declarative (Mangle) |
|------------|----------------------|
| Steps in order | All rules apply simultaneously |
| Loops (`for`, `while`) | Recursion with base case |
| Variable mutation | New variable for each binding |
| Early return | All facts derive |
| If-else branches | Separate rules with guards |
| Try-catch errors | Success/failure predicates |
| State transitions | Facts with time/step indices |
| Accumulators | Aggregation functions |
| Side effects | Pure derivation only |
| Null/undefined | Closed World (absent = false) |

---

## Mental Model Shift

**Imperative:** "Do this, then that, then the other thing."

**Declarative:** "These are the facts. These are the rules. What can be derived?"

### Example: Sum of Numbers 1-10

**Imperative:**
```python
total = 0
for i in range(1, 11):
    total += i
```

**Declarative (Mangle):**
```mangle
# Define the domain:
number(1).
number(2).
# ... or load in Go: for i := 1; i <= 10; i++ { store.Add(...) }

# Derive the sum:
total(Sum) :-
    number(N)
    |> do fn:group_by()
    |> let Sum = fn:Sum(N).
```

---

## Migration Checklist

When translating imperative code to Mangle:

- [ ] Replace sequential steps with simultaneous rules
- [ ] Replace loops with recursion (bounded!)
- [ ] Replace mutation with new variables
- [ ] Replace early returns with derivation of all matches
- [ ] Replace if-else with multiple guarded rules
- [ ] Replace try-catch with success/failure predicates
- [ ] Replace state transitions with time-indexed facts
- [ ] Replace accumulators with aggregation
- [ ] Remove side effects - handle in Go
- [ ] Replace null/undefined with optional patterns
- [ ] Add base cases to all recursion
- [ ] Think "what is true" not "what to do"

---

## Pro Tip: Think in Constraints

Instead of:
> "Set x to 5, add 1, multiply by 2"

Think:
> "Z is some value such that Z = 2 * (X + 1) where X = 5"

Mangle finds values that satisfy constraints, not step-by-step procedures.
