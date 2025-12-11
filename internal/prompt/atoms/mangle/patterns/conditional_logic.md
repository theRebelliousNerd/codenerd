# Conditional Logic Patterns

## Problem Description

Expressing conditional logic (if-then-else) in Datalog requires multiple rules. Common needs:
- Simple if-then-else
- Multiple conditions (if-elif-else)
- Nested conditions
- Case/switch statements
- Guard clauses
- Boolean expressions

## Core Pattern: Basic If-Then-Else

### Template
```mangle
# If condition is true, use then_value
result(X, ThenValue) :- condition(X), ThenValue = "then_branch".

# Else (condition is false), use else_value
result(X, ElseValue) :- not condition(X), ElseValue = "else_branch".
```

### Complete Working Example
```mangle
# Schema
Decl user(UserId.Type<string>).
Decl is_premium(UserId.Type<string>).
Decl discount(UserId.Type<string>, Rate.Type<float>).

# Facts
user("u1").
user("u2").
user("u3").

is_premium("u1").
is_premium("u3").

# If premium, 20% discount
discount(UserId, 0.20) :- is_premium(UserId).

# Else, 5% discount
discount(UserId, 0.05) :- user(UserId), not is_premium(UserId).

# Query: discount(UserId, Rate)
# Results:
# discount("u1", 0.20)  # Premium
# discount("u2", 0.05)  # Not premium
# discount("u3", 0.20)  # Premium
```

## Variation 1: Multiple Conditions (If-Elif-Else)

### Problem
Chain multiple conditions with priority.

### Solution
```mangle
# Schema
Decl item(Id.Type<string>, Price.Type<int>).
Decl category(Id.Type<string>).

# Priority 1: If price > 1000
category(Id) = /expensive :- item(Id, Price), Price > 1000.

# Priority 2: Elif price > 100
category(Id) = /moderate :-
  item(Id, Price),
  Price > 100,
  Price <= 1000.

# Priority 3: Else
category(Id) = /cheap :-
  item(Id, Price),
  Price <= 100.

# Note: The above uses functional syntax which might not work in Mangle
# Better approach with separate predicates:

Decl expensive(Id.Type<string>).
Decl moderate(Id.Type<string>).
Decl cheap(Id.Type<string>).

expensive(Id) :- item(Id, Price), Price > 1000.

moderate(Id) :-
  item(Id, Price),
  Price > 100,
  Price <= 1000.

cheap(Id) :-
  item(Id, Price),
  Price <= 100.
```

### Example
```mangle
item("i1", 50).
item("i2", 500).
item("i3", 2000).

# Results:
# cheap("i1")
# moderate("i2")
# expensive("i3")
```

## Variation 2: Guard Clauses (Early Exit)

### Problem
Check prerequisites before main logic.

### Solution
```mangle
# Schema
Decl input(X.Type<string>, Value.Type<int>).
Decl validated(X.Type<string>).
Decl result(X.Type<string>, Output.Type<string>).

# Guard: Only process validated inputs
validated(X) :- input(X, Value), Value > 0, Value < 1000.

# Main logic: Only runs if guard passes
result(X, "processed") :- validated(X), input(X, Value).

# Error case: Guard failed
result(X, "invalid_input") :- input(X, Value), not validated(X).
```

### Example
```mangle
input("x1", 500).    # Valid
input("x2", -10).    # Invalid (negative)
input("x3", 2000).   # Invalid (too large)

# Results:
# validated("x1")
# result("x1", "processed")
# result("x2", "invalid_input")
# result("x3", "invalid_input")
```

## Variation 3: Nested Conditions

### Problem
Conditions within conditions (nested if-else).

### Solution
```mangle
# Schema
Decl person(Id.Type<string>, Age.Type<int>, Income.Type<int>).
Decl loan_approved(Id.Type<string>, Reason.Type<string>).
Decl loan_denied(Id.Type<string>, Reason.Type<string>).

# If age >= 18
#   If income >= 50000
#     Approved
#   Else
#     Denied (low income)
# Else
#   Denied (too young)

loan_approved(Id, "sufficient_income") :-
  person(Id, Age, Income),
  Age >= 18,
  Income >= 50000.

loan_denied(Id, "low_income") :-
  person(Id, Age, Income),
  Age >= 18,
  Income < 50000.

loan_denied(Id, "too_young") :-
  person(Id, Age, Income),
  Age < 18.
```

### Example
```mangle
person("p1", 25, 60000).  # Adult, sufficient income
person("p2", 30, 40000).  # Adult, low income
person("p3", 16, 70000).  # Too young

# Results:
# loan_approved("p1", "sufficient_income")
# loan_denied("p2", "low_income")
# loan_denied("p3", "too_young")
```

## Variation 4: Case/Switch Statement

### Problem
Select based on discrete values (like switch/case).

### Solution
```mangle
# Schema
Decl event(EventId.Type<string>, Type.Type<atom>).
Decl priority(EventId.Type<string>, Level.Type<int>).

# Case /critical -> Priority 1
priority(EventId, 1) :- event(EventId, /critical).

# Case /error -> Priority 2
priority(EventId, 2) :- event(EventId, /error).

# Case /warning -> Priority 3
priority(EventId, 3) :- event(EventId, /warning).

# Case /info -> Priority 4
priority(EventId, 4) :- event(EventId, /info).

# Default case
priority(EventId, 5) :-
  event(EventId, Type),
  Type != /critical,
  Type != /error,
  Type != /warning,
  Type != /info.
```

### Example
```mangle
event("e1", /critical).
event("e2", /warning).
event("e3", /debug).

# Results:
# priority("e1", 1)
# priority("e2", 3)
# priority("e3", 5)  # Default
```

## Variation 5: Boolean Expressions (AND, OR, NOT)

### Problem
Combine multiple boolean conditions.

### Solution
```mangle
# Schema
Decl item(Id.Type<string>, InStock.Type<atom>, OnSale.Type<atom>, Featured.Type<atom>).
Decl should_display(Id.Type<string>).

# Complex condition: (InStock AND OnSale) OR Featured
should_display(Id) :-
  item(Id, /yes, /yes, _).  # InStock AND OnSale

should_display(Id) :-
  item(Id, _, _, /yes).     # Featured (regardless of stock/sale)

# Alternative: All conditions in one rule
Decl should_display_v2(Id.Type<string>).
should_display_v2(Id) :-
  item(Id, InStock, OnSale, Featured),
  (InStock = /yes, OnSale = /yes) ; Featured = /yes.

# Note: OR (;) syntax may not work in all Mangle versions
# Safer to use multiple rules as shown first
```

### Example
```mangle
item("i1", /yes, /yes, /no).   # In stock, on sale
item("i2", /yes, /no, /yes).   # In stock, featured
item("i3", /no, /no, /yes).    # Out of stock, but featured
item("i4", /yes, /no, /no).    # Just in stock

# Results:
# should_display("i1")  # InStock AND OnSale
# should_display("i2")  # Featured
# should_display("i3")  # Featured
# NOT i4 (only in stock, not on sale or featured)
```

## Variation 6: Range Conditions

### Problem
Categorize values based on ranges.

### Solution
```mangle
# Schema
Decl measurement(Id.Type<string>, Value.Type<int>).
Decl grade(Id.Type<string>, Letter.Type<atom>).

# A: 90-100
grade(Id, /A) :-
  measurement(Id, Value),
  Value >= 90,
  Value <= 100.

# B: 80-89
grade(Id, /B) :-
  measurement(Id, Value),
  Value >= 80,
  Value < 90.

# C: 70-79
grade(Id, /C) :-
  measurement(Id, Value),
  Value >= 70,
  Value < 80.

# D: 60-69
grade(Id, /D) :-
  measurement(Id, Value),
  Value >= 60,
  Value < 70.

# F: Below 60
grade(Id, /F) :-
  measurement(Id, Value),
  Value < 60.
```

### Example
```mangle
measurement("m1", 95).
measurement("m2", 85).
measurement("m3", 72).
measurement("m4", 55).

# Results:
# grade("m1", /A)
# grade("m2", /B)
# grade("m3", /C)
# grade("m4", /F)
```

## Variation 7: Ternary Operator (Inline If)

### Problem
Simple condition for selecting between two values.

### Solution
```mangle
# Schema
Decl value(X.Type<string>, V.Type<int>).
Decl result(X.Type<string>, Output.Type<int>).

# If V > 100, use V; else use 100 (max function)
result(X, V) :- value(X, V), V > 100.
result(X, 100) :- value(X, V), V <= 100.

# Or: If V < 0, use 0; else use V (clamp to non-negative)
Decl clamped(X.Type<string>, Output.Type<int>).
clamped(X, 0) :- value(X, V), V < 0.
clamped(X, V) :- value(X, V), V >= 0.
```

### Example
```mangle
value("x1", 150).
value("x2", 50).
value("x3", -10).

# result (max with 100):
# result("x1", 150)
# result("x2", 100)

# clamped (non-negative):
# clamped("x1", 150)
# clamped("x2", 50)
# clamped("x3", 0)
```

## Variation 8: State Machine

### Problem
Transition between states based on conditions.

### Solution
```mangle
# Schema
Decl current_state(EntityId.Type<string>, State.Type<atom>).
Decl event(EntityId.Type<string>, Event.Type<atom>).
Decl next_state(EntityId.Type<string>, State.Type<atom>).

# State transitions
# From /idle: event /start -> /running
next_state(EntityId, /running) :-
  current_state(EntityId, /idle),
  event(EntityId, /start).

# From /running: event /pause -> /paused
next_state(EntityId, /paused) :-
  current_state(EntityId, /running),
  event(EntityId, /pause).

# From /paused: event /resume -> /running
next_state(EntityId, /running) :-
  current_state(EntityId, /paused),
  event(EntityId, /resume).

# From /running: event /stop -> /stopped
next_state(EntityId, /stopped) :-
  current_state(EntityId, /running),
  event(EntityId, /stop).

# No transition (stay in same state)
next_state(EntityId, State) :-
  current_state(EntityId, State),
  not (event(EntityId, _), transition_exists(EntityId)).

Decl transition_exists(EntityId.Type<string>).
transition_exists(EntityId) :-
  current_state(EntityId, State),
  event(EntityId, Event),
  valid_transition(State, Event).

Decl valid_transition(State.Type<atom>, Event.Type<atom>).
valid_transition(/idle, /start).
valid_transition(/running, /pause).
valid_transition(/paused, /resume).
valid_transition(/running, /stop).
```

### Example
```mangle
current_state("e1", /idle).
current_state("e2", /running).

event("e1", /start).
event("e2", /pause).

# Results:
# next_state("e1", /running)  # idle + start -> running
# next_state("e2", /paused)   # running + pause -> paused
```

## Variation 9: Null Handling (Three-Valued Logic)

### Problem
Handle cases where values may be unknown/missing.

### Solution
```mangle
# Schema
Decl item(Id.Type<string>).
Decl has_value(Id.Type<string>, Value.Type<int>).
Decl result(Id.Type<string>, Output.Type<atom>).

# If has value and value > 100 -> /high
result(Id, /high) :- has_value(Id, Value), Value > 100.

# If has value and value <= 100 -> /low
result(Id, /low) :- has_value(Id, Value), Value <= 100.

# If no value -> /unknown
result(Id, /unknown) :- item(Id), not has_value(Id, _).
```

### Example
```mangle
item("i1").
item("i2").
item("i3").

has_value("i1", 150).
has_value("i2", 50).
# i3 has no value

# Results:
# result("i1", /high)
# result("i2", /low)
# result("i3", /unknown)
```

## Variation 10: Complex Predicate Conditions

### Problem
Condition based on derived predicate results.

### Solution
```mangle
# Schema
Decl user(UserId.Type<string>).
Decl purchase(PurchaseId.Type<string>, UserId.Type<string>, Amount.Type<int>).
Decl total_spent(UserId.Type<string>, Total.Type<int>).
Decl user_tier(UserId.Type<string>, Tier.Type<atom>).

# Derive total (intermediate predicate)
total_spent(UserId, Total) :-
  purchase(PurchaseId, UserId, Amount)
  |> do fn:group_by(UserId),
     let Total = fn:Sum(Amount).

# Condition based on derived total
user_tier(UserId, /gold) :- total_spent(UserId, Total), Total > 10000.
user_tier(UserId, /silver) :- total_spent(UserId, Total), Total > 5000, Total <= 10000.
user_tier(UserId, /bronze) :- total_spent(UserId, Total), Total <= 5000.
user_tier(UserId, /none) :- user(UserId), not purchase(_, UserId, _).
```

### Example
```mangle
user("u1").
user("u2").
user("u3").
user("u4").

purchase("p1", "u1", 6000).
purchase("p2", "u1", 5000).  # Total: 11000
purchase("p3", "u2", 7000).   # Total: 7000
purchase("p4", "u3", 2000).   # Total: 2000

# Results:
# total_spent("u1", 11000)
# total_spent("u2", 7000)
# total_spent("u3", 2000)
# user_tier("u1", /gold)
# user_tier("u2", /silver)
# user_tier("u3", /bronze)
# user_tier("u4", /none)
```

## Variation 11: Short-Circuit Evaluation

### Problem
Optimize by checking cheaper conditions first.

### Solution
```mangle
# Schema
Decl item(Id.Type<string>, InStock.Type<atom>, Price.Type<int>).
Decl available_for_purchase(Id.Type<string>).

# Check cheap condition (in stock) before expensive computation
available_for_purchase(Id) :-
  item(Id, /yes, Price),  # InStock check is cheap (indexed)
  Price > 0,              # Then check price
  complex_validation(Id). # Finally expensive check

Decl complex_validation(Id.Type<string>).
complex_validation(Id) :- item(Id, _, Price), Price < 10000.  # Placeholder
```

## Anti-Patterns

### WRONG: Overlapping Conditions (Non-Exclusive)
```mangle
# Bad - both can match!
category(X, /big) :- value(X, V), V > 50.
category(X, /huge) :- value(X, V), V > 100.

# If V = 150, both /big and /huge are derived!
# Fix: Make mutually exclusive
category(X, /huge) :- value(X, V), V > 100.
category(X, /big) :- value(X, V), V > 50, V <= 100.
```

### WRONG: Missing Else Case
```mangle
# Bad - no fallback
result(X, "yes") :- condition(X).
# What if condition(X) is false? result is undefined!

# Fix: Add else case
result(X, "yes") :- condition(X).
result(X, "no") :- not condition(X).
```

### WRONG: Circular Conditions
```mangle
# Bad - circular dependency
result(X, /a) :- result(X, /b).
result(X, /b) :- result(X, /a).

# This creates a logical loop with no base case
# Fix: Establish base cases
```

## Performance Tips

1. **Order Conditions by Selectivity**: Most restrictive first
2. **Use Stratification**: Derive intermediate predicates first
3. **Avoid Redundant Checks**: Mutually exclusive rules
4. **Index Common Conditions**: Speed up frequent checks

## Common Use Cases in codeNERD

### Shard Selection Logic
```mangle
Decl task(TaskId.Type<string>, Type.Type<atom>).
Decl assign_shard(TaskId.Type<string>, ShardType.Type<atom>).

assign_shard(TaskId, /coder) :- task(TaskId, /code_generation).
assign_shard(TaskId, /tester) :- task(TaskId, /test_execution).
assign_shard(TaskId, /reviewer) :- task(TaskId, /code_review).
assign_shard(TaskId, /researcher) :- task(TaskId, /knowledge_gathering).
```

### File Type Classification
```mangle
Decl file(Path.Type<string>, Extension.Type<string>).
Decl file_category(Path.Type<string>, Category.Type<atom>).

file_category(Path, /source) :- file(Path, Ext), Ext = "go".
file_category(Path, /source) :- file(Path, Ext), Ext = "py".
file_category(Path, /test) :- file(Path, Ext), Ext = "go", :match_field(Path, "test", _).
file_category(Path, /config) :- file(Path, Ext), Ext = "json".
file_category(Path, /config) :- file(Path, Ext), Ext = "yaml".
file_category(Path, /other) :- file(Path, Ext), not is_known_type(Ext).

Decl is_known_type(Ext.Type<string>).
is_known_type("go").
is_known_type("py").
is_known_type("json").
is_known_type("yaml").
```

### Test Priority Assignment
```mangle
Decl test(TestId.Type<string>, Duration.Type<int>, Flakiness.Type<float>).
Decl test_priority(TestId.Type<string>, Priority.Type<atom>).

test_priority(TestId, /critical) :-
  test(TestId, Duration, Flakiness),
  Duration < 1000,
  Flakiness < 0.01.

test_priority(TestId, /normal) :-
  test(TestId, Duration, Flakiness),
  not (Duration < 1000, Flakiness < 0.01),
  not (Duration > 10000).

test_priority(TestId, /low) :-
  test(TestId, Duration, Flakiness),
  Duration > 10000.
```
