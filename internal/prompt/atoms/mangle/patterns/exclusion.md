# Exclusion Patterns (Safe Negation)

## Problem Description

Negation is powerful but dangerous in Datalog. Unsafe negation leads to infinite derivations or incorrect results. Safe negation requires all variables to be "grounded" (bound by positive predicates) before appearing in negated atoms.

Common exclusion needs:
- Set difference (A - B)
- Finding missing items
- Filtering out unwanted records
- Complement queries

## Core Pattern: Safe Negation

### Template
```mangle
# SAFE: Variable X is bound by positive predicate first
result(X) :- candidate(X), not excluded(X).

# UNSAFE: Variable X appears only in negation
# BAD: result(X) :- not excluded(X).
```

### Complete Working Example
```mangle
# Schema
Decl all_users(UserId.Type<string>).
Decl banned_users(UserId.Type<string>).
Decl active_users(UserId.Type<string>).

# Facts
all_users("u1").
all_users("u2").
all_users("u3").
all_users("u4").

banned_users("u2").
banned_users("u4").

# Safe negation: X bound by all_users first
active_users(X) :- all_users(X), not banned_users(X).

# Query: active_users(X)
# Results: "u1", "u3"
```

## Variation 1: Set Difference (A - B)

### Problem
Get all elements in set A that are not in set B.

### Solution
```mangle
# Schema
Decl set_a(Elem.Type<string>).
Decl set_b(Elem.Type<string>).
Decl difference(Elem.Type<string>).

# A - B
difference(X) :- set_a(X), not set_b(X).
```

### Example
```mangle
set_a("a").
set_a("b").
set_a("c").

set_b("b").
set_b("d").

# Results: difference("a"), difference("c")
# NOT "b" (in both), NOT "d" (only in B)
```

## Variation 2: Missing Items / Gap Detection

### Problem
Find items that should exist but don't.

### Solution
```mangle
# Schema
Decl expected(Id.Type<string>).
Decl actual(Id.Type<string>).
Decl missing(Id.Type<string>).

# Expected but not actual
missing(X) :- expected(X), not actual(X).
```

### Example
```mangle
# Expected test files
expected("test_auth.go").
expected("test_db.go").
expected("test_api.go").

# Actual test files
actual("test_auth.go").
actual("test_api.go").

# Result: missing("test_db.go")
```

## Variation 3: Orphaned Records

### Problem
Find child records without parent.

### Solution
```mangle
# Schema
Decl child(ChildId.Type<string>, ParentId.Type<string>).
Decl parent(ParentId.Type<string>).
Decl orphaned(ChildId.Type<string>).

# Children whose parent doesn't exist
orphaned(ChildId) :-
  child(ChildId, ParentId),
  not parent(ParentId).
```

### Example
```mangle
child("c1", "p1").
child("c2", "p2").
child("c3", "p99").  # Orphan!

parent("p1").
parent("p2").

# Result: orphaned("c3")
```

## Variation 4: Never-Occurred Events

### Problem
Find entities that never experienced a certain event.

### Solution
```mangle
# Schema
Decl user(UserId.Type<string>).
Decl login_event(UserId.Type<string>, Timestamp.Type<int>).
Decl never_logged_in(UserId.Type<string>).

# Users with no login events
never_logged_in(UserId) :- user(UserId), not login_event(UserId, _).
```

### Example
```mangle
user("u1").
user("u2").
user("u3").

login_event("u1", 1000).
login_event("u1", 1100).
login_event("u3", 1050).

# Results: never_logged_in("u2")
```

## Variation 5: Exclusive Or (XOR)

### Problem
Items in A or B, but not both.

### Solution
```mangle
# Schema
Decl set_a(Elem.Type<string>).
Decl set_b(Elem.Type<string>).
Decl xor_result(Elem.Type<string>).

# In A but not B
xor_result(X) :- set_a(X), not set_b(X).

# In B but not A
xor_result(X) :- set_b(X), not set_a(X).
```

### Example
```mangle
set_a("a").
set_a("b").

set_b("b").
set_b("c").

# Results: xor_result("a"), xor_result("c")
# NOT "b" (in both)
```

## Variation 6: Negation with Multiple Conditions

### Problem
Exclude based on multiple negative conditions (AND of negations).

### Solution
```mangle
# Schema
Decl item(Id.Type<string>).
Decl has_issue_a(Id.Type<string>).
Decl has_issue_b(Id.Type<string>).
Decl clean_items(Id.Type<string>).

# Items with neither issue
clean_items(Id) :- item(Id), not has_issue_a(Id), not has_issue_b(Id).
```

### Example
```mangle
item("i1").
item("i2").
item("i3").

has_issue_a("i1").
has_issue_b("i2").

# Result: clean_items("i3")
# NOT i1 (has issue A)
# NOT i2 (has issue B)
```

## Variation 7: Complement with Constraints

### Problem
Exclude items, but only if they match certain criteria.

### Solution
```mangle
# Schema
Decl candidate(Id.Type<string>, Value.Type<int>).
Decl excluded(Id.Type<string>).
Decl result(Id.Type<string>, Value.Type<int>).

# Candidates that are not excluded AND meet criteria
result(Id, Value) :-
  candidate(Id, Value),
  Value > 100,
  not excluded(Id).
```

### Example
```mangle
candidate("c1", 150).
candidate("c2", 200).
candidate("c3", 50).

excluded("c2").

# Results:
# result("c1", 150)  # Value > 100 and not excluded
# NOT c2 (excluded)
# NOT c3 (value too low)
```

## Variation 8: Stratified Negation (Negation on Derived Predicate)

### Problem
Negate a derived predicate (not just base facts).

### Solution
```mangle
# Schema
Decl edge(From.Type<string>, To.Type<string>).
Decl reachable(From.Type<string>, To.Type<string>).
Decl isolated(Node.Type<string>).
Decl node(N.Type<string>).

# Derive reachable (Stratum 1)
reachable(X, Y) :- edge(X, Y).
reachable(X, Z) :- reachable(X, Y), edge(Y, Z).

# All nodes
node(X) :- edge(X, _).
node(X) :- edge(_, X).

# Nodes not reachable from "start" (Stratum 2 - after reachable is computed)
isolated(X) :- node(X), not reachable("start", X).
```

### Example
```mangle
edge("start", "a").
edge("a", "b").
edge("x", "y").  # Separate component

# Results:
# reachable("start", "a")
# reachable("start", "b")
# isolated("x")
# isolated("y")
# isolated("start") - can't reach itself (unless there's a cycle)
```

## Variation 9: Negation with Aggregation

### Problem
Exclude items based on aggregated values.

### Solution
```mangle
# Schema
Decl purchase(UserId.Type<string>, Amount.Type<int>).
Decl total_spent(UserId.Type<string>, Total.Type<int>).
Decl low_spenders(UserId.Type<string>).
Decl user(UserId.Type<string>).

# Calculate totals (Stratum 1)
total_spent(UserId, Total) :-
  purchase(UserId, Amount)
  |> do fn:group_by(UserId),
     let Total = fn:Sum(Amount).

# Users who spent less than threshold (Stratum 2)
low_spenders(UserId) :- total_spent(UserId, Total), Total < 100.

# Users who never made purchases (Stratum 2)
low_spenders(UserId) :- user(UserId), not purchase(UserId, _).
```

### Example
```mangle
user("u1").
user("u2").
user("u3").

purchase("u1", 50).
purchase("u1", 30).  # Total: 80
purchase("u2", 200). # Total: 200

# Results:
# low_spenders("u1")  # Total < 100
# low_spenders("u3")  # No purchases
# NOT u2 (high spender)
```

## Variation 10: Double Negation (Positive Logic)

### Problem
Express "all X have property Y" using negation.

### Solution
```mangle
# Schema
Decl module(ModuleId.Type<string>).
Decl has_test(ModuleId.Type<string>).
Decl all_modules_tested(Result.Type<atom>).

# "All modules have tests" = "There does NOT exist a module that does NOT have a test"
all_modules_tested(/yes) :- not (module(M), not has_test(M)).

# Simpler alternative (find counterexample)
Decl untested_module(ModuleId.Type<string>).
untested_module(M) :- module(M), not has_test(M).

all_modules_tested(/no) :- untested_module(_).
all_modules_tested(/yes) :- not untested_module(_).
```

### Example
```mangle
# Scenario 1: All tested
module("m1").
module("m2").
has_test("m1").
has_test("m2").
# Result: all_modules_tested(/yes)

# Scenario 2: One untested
module("m3").
# Result: all_modules_tested(/no)
```

## Anti-Patterns

### WRONG: Unsafe Negation (Ungrounded Variable)
```mangle
# WRONG - X is not bound!
bad_rule(X) :- not excluded(X).

# This asks: "Give me all X that are not in excluded"
# But X is infinite! No finite answer.

# Fix: Ground X first
good_rule(X) :- candidate(X), not excluded(X).
```

### WRONG: Negation in Recursive Rule (Stratification Violation)
```mangle
# WRONG - recursion through negation
p(X) :- q(X), not p(Y), related(X, Y).

# This creates a circular dependency:
# - To compute p, we need to know what's NOT in p
# - But p is still being computed!

# Fix: Restructure into strata (break the cycle)
```

### WRONG: Forgetting the Domain
```mangle
# WRONG - assumes you know all possible values
missing(X) :- not present(X).

# This only works if Mangle knows the domain of X
# Fix: Provide explicit domain
missing(X) :- all_possible_values(X), not present(X).
```

### WRONG: Negating Without Checking Existence
```mangle
# Misleading
no_orders(UserId) :- not order(UserId, _).

# This doesn't check if UserId actually exists!
# Better: explicitly state user exists
no_orders(UserId) :- user(UserId), not order(UserId, _).
```

## Safety Rules (Checklist)

1. **Ground Before Negate**: Every variable in `not pred(X, Y)` must appear in a positive predicate earlier in the rule
2. **No Recursion Through Negation**: If `p` depends on `not q`, then `q` cannot depend on `p` (stratification)
3. **Closed World Assumption**: Negation means "not provable from known facts", not "definitely false in reality"

## Performance Tips

1. **Materialize the Positive First**: Compute what exists before checking what doesn't
2. **Use Existence Checks**: `not exists(X)` is faster than enumerating all non-existing X
3. **Index Exclusion Lists**: If excluding based on a predicate, ensure it's indexed
4. **Minimize Negation**: Positive logic is usually faster

## Common Use Cases in codeNERD

### Files Without Tests
```mangle
Decl source_file(File.Type<string>).
Decl test_file(TestFile.Type<string>, SourceFile.Type<string>).
Decl untested_files(File.Type<string>).

untested_files(File) :- source_file(File), not test_file(_, File).
```

### Unreachable Code
```mangle
Decl function(FuncId.Type<string>).
Decl called_by(Callee.Type<string>, Caller.Type<string>).
Decl reachable_from_main(FuncId.Type<string>).
Decl dead_code(FuncId.Type<string>).

# Reachable from main
reachable_from_main("main").
reachable_from_main(Callee) :- reachable_from_main(Caller), called_by(Callee, Caller).

# Dead code
dead_code(FuncId) :- function(FuncId), not reachable_from_main(FuncId).
```

### Unhandled Error Paths
```mangle
Decl error_type(ErrorId.Type<string>).
Decl handled_error(ErrorId.Type<string>).
Decl unhandled_errors(ErrorId.Type<string>).

unhandled_errors(ErrorId) :- error_type(ErrorId), not handled_error(ErrorId).
```
