# Mangle Aggregation Functions

Complete reference for aggregation (reducer) functions in Mangle. These are used within transform pipelines (`|> do ...`).

## CRITICAL CASING RULES

Aggregation functions have **specific casing requirements**:

| Function | Correct Casing | Common Mistake |
|----------|---------------|----------------|
| Count | `fn:Count` | ❌ `fn:count` |
| Sum | `fn:Sum` | ❌ `fn:sum` |
| Min | `fn:Min` | ❌ `fn:min` |
| Max | `fn:Max` | ❌ `fn:max` |
| Avg | `fn:Avg` | ❌ `fn:avg` |
| FloatSum | `fn:FloatSum` | ❌ `fn:float_sum` |
| FloatMin | `fn:FloatMin` | ❌ `fn:float_min` |
| FloatMax | `fn:FloatMax` | ❌ `fn:float_max` |

**Note:** These are **PascalCase** (capital first letter), unlike most Mangle functions which are lowercase.

---

## Basic Aggregation Functions

---

### fn:Count

**Signature:** `fn:Count(Variable) -> Int`

**Description:** Counts the number of rows in a group.

**Usage:** Inside `do` transform after `fn:group_by`

**Parameters:**
- Takes variable from the group (the variable is not actually used for counting)
- Returns `Type<int>` count of rows

**Examples:**

```mangle
# Count all rows
total_count(Count) :-
  data(X, Y)
  |> do fn:group_by(), let Count = fn:Count(X).

# Count per group
count_by_category(Category, Count) :-
  item(Item, Category)
  |> do fn:group_by(Category), let Count = fn:Count(Item).

# From example file
aggregated(Subject, Verb, Topic, Count)
  :- observed(Subject, Verb, Topic, Weight, _)
  |> do fn:group_by(Subject, Verb, Topic), let Count = fn:Count(Weight).
```

**Common Mistakes:**
- ❌ Using lowercase: `fn:count` - won't work, must be `fn:Count`
- ❌ Using without group_by: must be inside `|> do fn:group_by(...)`
- ❌ Passing multiple variables: `fn:Count(X, Y)` - takes one variable

---

### fn:Sum

**Signature:** `fn:Sum(Variable) -> Int`

**Description:** Sums integer values in a group.

**Usage:** Inside `do` transform after `fn:group_by`

**Parameters:**
- Variable bound to `Type<int>` values
- Returns `Type<int>` sum

**Examples:**

```mangle
# Sum all values
total(Total) :-
  value(X)
  |> do fn:group_by(), let Total = fn:Sum(X).

# Sum by category
category_total(Cat, Total) :-
  item(Item, Cat, Amount)
  |> do fn:group_by(Cat), let Total = fn:Sum(Amount).

# From aggregation example
aggregated(Subject, Verb, Topic, Sum)
  :- observed(Subject, Verb, Topic, Weight, _)
  |> do fn:group_by(Subject, Verb, Topic), let Sum = fn:Sum(Weight).

# Multiple aggregations
stats(Category, Total, Count) :-
  value(Category, Val)
  |> do fn:group_by(Category),
     let Total = fn:Sum(Val),
     let Count = fn:Count(Val).
```

**Common Mistakes:**
- ❌ Using lowercase: `fn:sum` must be `fn:Sum`
- ❌ Summing non-integers: use `fn:FloatSum` for floats
- ❌ Using on empty group: returns 0 for int sums

---

### fn:Min

**Signature:** `fn:Min(Variable) -> Int`

**Description:** Finds minimum integer value in a group.

**Usage:** Inside `do` transform after `fn:group_by`

**Parameters:**
- Variable bound to `Type<int>` values
- Returns `Type<int>` minimum value

**Examples:**

```mangle
# Global minimum
min_value(Min) :-
  value(X)
  |> do fn:group_by(), let Min = fn:Min(X).

# Minimum per group
min_by_category(Cat, Min) :-
  measurement(Cat, Value)
  |> do fn:group_by(Cat), let Min = fn:Min(Value).

# Find lowest score
lowest_score(Player, Score) :-
  game_score(Player, Score)
  |> do fn:group_by(Player), let MinScore = fn:Min(Score),
  Score = MinScore.
```

**Common Mistakes:**
- ❌ Using lowercase: `fn:min` must be `fn:Min`
- ❌ Empty group behavior: returns `math.MinInt64` which may cause issues

---

### fn:Max

**Signature:** `fn:Max(Variable) -> Int`

**Description:** Finds maximum integer value in a group.

**Usage:** Inside `do` transform after `fn:group_by`

**Parameters:**
- Variable bound to `Type<int>` values
- Returns `Type<int>` maximum value

**Examples:**

```mangle
# Global maximum
max_value(Max) :-
  value(X)
  |> do fn:group_by(), let Max = fn:Max(X).

# Maximum per group
max_by_category(Cat, Max) :-
  measurement(Cat, Value)
  |> do fn:group_by(Cat), let Max = fn:Max(Value).

# From map_aggregation example
user_max_score(User, MaxScore)
  :- user_language(User, Language),
     language_popularity(Language, Score)
  |> do fn:group_by(User), let MaxScore = fn:Max(Score).

# High score
top_score(Player, Score) :-
  game_score(Player, Score)
  |> do fn:group_by(Player), let MaxScore = fn:Max(Score),
  Score = MaxScore.
```

**Common Mistakes:**
- ❌ Using lowercase: `fn:max` must be `fn:Max`
- ❌ Empty group behavior: returns `math.MinInt64` (not MaxInt64!)

---

### fn:Avg

**Signature:** `fn:Avg(Variable) -> Float64`

**Description:** Computes average of values in a group. Returns float even for integer inputs.

**Usage:** Inside `do` transform after `fn:group_by`

**Parameters:**
- Variable bound to `Type<int>` or `Type<float64>` values
- Returns `Type<float64>` average

**Examples:**

```mangle
# Global average
avg_value(Avg) :-
  value(X)
  |> do fn:group_by(), let Avg = fn:Avg(X).

# Average per group
avg_by_category(Cat, Avg) :-
  measurement(Cat, Value)
  |> do fn:group_by(Cat), let Avg = fn:Avg(Value).

# Average score
player_avg(Player, AvgScore) :-
  game_score(Player, Score)
  |> do fn:group_by(Player), let AvgScore = fn:Avg(Score).
```

**Common Mistakes:**
- ❌ Using lowercase: `fn:avg` must be `fn:Avg`
- ❌ Empty group: returns NaN (special handling needed)
- ❌ Expecting integer result: always returns float64

---

## Floating-Point Aggregation Functions

---

### fn:FloatSum

**Signature:** `fn:FloatSum(Variable) -> Float64`

**Description:** Sums floating-point values. Can accept int or float64 (ints auto-converted).

**Usage:** Inside `do` transform after `fn:group_by`

**Parameters:**
- Variable bound to `Type<float64>` or `Type<int>`
- Returns `Type<float64>` sum

**Examples:**

```mangle
# Sum floats
total(Total) :-
  measurement(X)
  |> do fn:group_by(), let Total = fn:FloatSum(X).

# Sum mixed int and float
mixed_sum(Sum) :-
  value(V)  # V can be int or float
  |> do fn:group_by(), let Sum = fn:FloatSum(V).
```

**Common Mistakes:**
- ❌ Using snake_case: `fn:float_sum` must be `fn:FloatSum`

---

### fn:FloatMin

**Signature:** `fn:FloatMin(Variable) -> Float64`

**Description:** Finds minimum float value. Accepts float64 only.

**Usage:** Inside `do` transform after `fn:group_by`

**Parameters:**
- Variable bound to `Type<float64>`
- Returns `Type<float64>` minimum

**Examples:**

```mangle
# Minimum float
min_temp(Min) :-
  temperature(T)
  |> do fn:group_by(), let Min = fn:FloatMin(T).
```

**Common Mistakes:**
- ❌ Using snake_case: `fn:float_min` must be `fn:FloatMin`
- ❌ Empty group: returns `-MaxFloat64`

---

### fn:FloatMax

**Signature:** `fn:FloatMax(Variable) -> Float64`

**Description:** Finds maximum float value. Accepts float64 only.

**Usage:** Inside `do` transform after `fn:group_by`

**Parameters:**
- Variable bound to `Type<float64>`
- Returns `Type<float64>` maximum

**Examples:**

```mangle
# Maximum float
max_temp(Max) :-
  temperature(T)
  |> do fn:group_by(), let Max = fn:FloatMax(T).
```

**Common Mistakes:**
- ❌ Using snake_case: `fn:float_max` must be `fn:FloatMax`
- ❌ Empty group: returns `-MaxFloat64` (not positive MaxFloat64!)

---

## Collection Aggregation Functions

---

### fn:Collect

**Signature:** `fn:Collect(Variable, ...) -> List`

**Description:** Collects values into a list. Can collect single values or tuples.

**Usage:** Inside `do` transform after `fn:group_by`

**Parameters:**
- One or more variables to collect
- Returns `Type<list<T>>` or `Type<list<tuple>>`

**Examples:**

```mangle
# Collect single values
all_values(Values) :-
  data(X)
  |> do fn:group_by(), let Values = fn:Collect(X).

# Collect per group
values_by_category(Cat, Values) :-
  item(Item, Cat)
  |> do fn:group_by(Cat), let Values = fn:Collect(Item).

# Collect tuples (multiple variables)
pairs(Pairs) :-
  edge(X, Y)
  |> do fn:group_by(), let Pairs = fn:Collect(X, Y).

# From map_aggregation example
user_languages_list(User, Languages)
  :- user_language(User, Language)
  |> do fn:group_by(User), let Languages = fn:Collect(Language).
```

**Common Mistakes:**
- ❌ Using lowercase: `fn:collect` must be `fn:Collect`
- ❌ Order of elements: not guaranteed, may vary

---

### fn:CollectDistinct

**Signature:** `fn:CollectDistinct(Variable, ...) -> List`

**Description:** Collects unique values into a list (duplicates removed).

**Usage:** Inside `do` transform after `fn:group_by`

**Parameters:**
- One or more variables to collect
- Returns `Type<list<T>>` with no duplicates

**Examples:**

```mangle
# Collect unique values
unique_values(Values) :-
  data(X)
  |> do fn:group_by(), let Values = fn:CollectDistinct(X).

# Unique tags per item
item_tags(Item, Tags) :-
  tag(Item, Tag)
  |> do fn:group_by(Item), let Tags = fn:CollectDistinct(Tag).
```

**Common Mistakes:**
- ❌ Expecting sorted output: order not guaranteed
- ❌ Case issues: `fn:collect_distinct` must be `fn:CollectDistinct`

---

### fn:CollectToMap

**Signature:** `fn:CollectToMap(Key, Value) -> Map`

**Description:** Collects key-value pairs into a map. Duplicate keys are deduplicated.

**Usage:** Inside `do` transform after `fn:group_by`

**Parameters:**
- First variable: map key
- Second variable: map value
- Returns `Type<map<K, V>>`

**Examples:**

```mangle
# Build a map
config_map(Map) :-
  config(Key, Value)
  |> do fn:group_by(), let Map = fn:CollectToMap(Key, Value).

# From map_aggregation example
user_language_scores(User, LanguageScoreMap)
  :- user_language(User, Language),
     language_popularity(Language, Score)
  |> do fn:group_by(User), let LanguageScoreMap = fn:CollectToMap(Language, Score).

# Create lookup per group
category_lookup(Cat, Lookup) :-
  item(Cat, Key, Val)
  |> do fn:group_by(Cat), let Lookup = fn:CollectToMap(Key, Val).
```

**Common Mistakes:**
- ❌ Wrong casing: must be `fn:CollectToMap` (not `fn:collect_to_map`)
- ❌ Duplicate keys: last value wins (but which is "last" is undefined)
- ❌ Wrong argument order: Key first, then Value

---

### fn:PickAny

**Signature:** `fn:PickAny(Variable) -> T`

**Description:** Picks an arbitrary value from the group. Non-deterministic.

**Usage:** Inside `do` transform after `fn:group_by`

**Parameters:**
- Variable to pick from
- Returns single value of same type

**Examples:**

```mangle
# Pick any representative
representative(Cat, Rep) :-
  item(Item, Cat)
  |> do fn:group_by(Cat), let Rep = fn:PickAny(Item).

# Get one example per group
example(Group, Example) :-
  data(Group, Value)
  |> do fn:group_by(Group), let Example = fn:PickAny(Value).
```

**Common Mistakes:**
- ❌ Expecting deterministic selection: result is arbitrary
- ❌ Casing: must be `fn:PickAny`

---

## Aggregation Function Table

| Function | Input Type | Output Type | Purpose | Casing |
|----------|-----------|-------------|---------|--------|
| `fn:Count` | Any | Int | Count rows | PascalCase |
| `fn:Sum` | Int | Int | Sum integers | PascalCase |
| `fn:Min` | Int | Int | Minimum integer | PascalCase |
| `fn:Max` | Int | Int | Maximum integer | PascalCase |
| `fn:Avg` | Int/Float | Float64 | Average | PascalCase |
| `fn:FloatSum` | Float/Int | Float64 | Sum floats | PascalCase |
| `fn:FloatMin` | Float | Float64 | Min float | PascalCase |
| `fn:FloatMax` | Float | Float64 | Max float | PascalCase |
| `fn:Collect` | Any | List | Collect values | PascalCase |
| `fn:CollectDistinct` | Any | List | Collect unique | PascalCase |
| `fn:CollectToMap` | Any, Any | Map | Build map | PascalCase |
| `fn:PickAny` | Any | T | Pick arbitrary | PascalCase |

---

## Usage Pattern: Transform Pipeline

All aggregation functions MUST be used inside a transform pipeline:

```mangle
# CORRECT: Inside do transform
result(Cat, Sum) :-
  data(Cat, Val)
  |> do fn:group_by(Cat), let Sum = fn:Sum(Val).

# WRONG: Outside transform pipeline
result(Cat, Sum) :-
  data(Cat, Val),
  Sum = fn:Sum(Val).  # ERROR: Sum is a reducer, not a regular function
```

**Pipeline Structure:**
```
predicate(Args)
  |> do fn:group_by(GroupVars),
     let Agg1 = fn:AggFunction1(Var1),
     let Agg2 = fn:AggFunction2(Var2).
```

---

## Multiple Aggregations

You can compute multiple aggregations in one pipeline:

```mangle
stats(Category, Count, Sum, Min, Max, Avg) :-
  measurement(Category, Value)
  |> do fn:group_by(Category),
     let Count = fn:Count(Value),
     let Sum = fn:Sum(Value),
     let Min = fn:Min(Value),
     let Max = fn:Max(Value),
     let Avg = fn:Avg(Value).
```

---

## Empty Group Behavior

| Function | Empty Group Result | Notes |
|----------|-------------------|-------|
| `fn:Count` | 0 | Safe |
| `fn:Sum` | 0 | Safe |
| `fn:Min` | MinInt64 | Dangerous! |
| `fn:Max` | MinInt64 | Dangerous! (not MaxInt!) |
| `fn:Avg` | NaN | Check for NaN |
| `fn:FloatSum` | 0.0 | Safe |
| `fn:FloatMin` | -MaxFloat64 | Dangerous! |
| `fn:FloatMax` | -MaxFloat64 | Dangerous! |
| `fn:Collect` | Empty list | Safe |
| `fn:PickAny` | Error | Will fail |

---

## Common Patterns

### Count Distinct (Manual)
```mangle
# Mangle doesn't have CountDistinct as a single function
# Use CollectDistinct + list:len
distinct_count(Cat, Count) :-
  item(Cat, Val)
  |> do fn:group_by(Cat),
     let Distinct = fn:CollectDistinct(Val),
  Count = fn:list:len(Distinct).
```

### Conditional Aggregation
```mangle
# Sum only positive values
positive_sum(Cat, Sum) :-
  value(Cat, Val),
  :gt(Val, 0)
  |> do fn:group_by(Cat), let Sum = fn:Sum(Val).
```

### Percentage Calculation
```mangle
category_percentage(Cat, Pct) :-
  item(Cat, Amount)
  |> do fn:group_by(Cat), let CatTotal = fn:Sum(Amount),
  all_items(_, TotalAmount)
  |> do fn:group_by(), let GrandTotal = fn:Sum(TotalAmount),
  Pct = fn:float:div(fn:float:mult(CatTotal, 100.0), GrandTotal).
```

---

## Performance Notes

- Aggregations require scanning all rows in a group
- `fn:Count` is fastest (doesn't process values)
- `fn:Collect` and `fn:CollectToMap` use more memory
- Multiple aggregations in one pipeline are more efficient than separate queries
