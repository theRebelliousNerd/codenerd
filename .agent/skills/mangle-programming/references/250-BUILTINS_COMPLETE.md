# 250: Complete Built-in Functions and Predicates Reference

**Purpose**: Authoritative reference for all built-in functions and predicates, derived from the TypeScript LSP implementation (which mirrors upstream Go).

**Source**: `packages/mangle-vscode/server/builtins/` (ported from `upstream/mangle/builtin/builtin.go`)

---

## Quick Reference

| Category | Prefix | Example | Purpose |
|----------|--------|---------|---------|
| Functions | `fn:` | `fn:plus(X, Y)` | Value computation |
| Reducers | `fn:` | `fn:sum(X)` | Aggregation in transforms |
| Predicates | `:` | `:match_field(S, /k, V)` | Pattern matching, filtering |

---

## 1. Built-in Functions (`fn:`)

### 1.1 Arithmetic Functions

| Function | Arity | Description |
|----------|-------|-------------|
| `fn:plus(X, Y, ...)` | Variable | Addition: `(X + Y) + ...`. Single arg returns X. |
| `fn:minus(X, Y, ...)` | Variable | Subtraction: `(X - Y) - ...`. Single arg returns -X. |
| `fn:mult(X, Y, ...)` | Variable | Multiplication: `(X * Y) * ...` |
| `fn:div(X, Y, ...)` | Variable | Integer division: `(X / Y) / ...` |
| `fn:sqrt(X)` | 1 | Square root of X |

**Float variants** (for float64 precision):
| Function | Arity | Description |
|----------|-------|-------------|
| `fn:float:plus(X, Y, ...)` | Variable | Float addition |
| `fn:float:mult(X, Y, ...)` | Variable | Float multiplication |
| `fn:float:div(X, Y, ...)` | Variable | Float division |

### 1.2 Grouping Function

| Function | Arity | Description |
|----------|-------|-------------|
| `fn:group_by(V1, V2, ...)` | Variable | Groups tuples by key variables. Empty `fn:group_by()` treats whole relation as one group. |

**Usage in transforms**:
```mangle
count_by_cat(Cat, N) :-
    item(Cat, _) |>
    do fn:group_by(Cat),
    let N = fn:count().
```

### 1.3 List Functions

| Function | Arity | Description |
|----------|-------|-------------|
| `fn:list(A, B, ...)` | Variable | Constructs a list from arguments |
| `fn:list:append(List, Elem)` | 2 | Appends element to list |
| `fn:list:get(List, Index)` | 2 | Returns element at index (0-based) |
| `fn:list:contains(List, Elem)` | 2 | Returns `/true` if element in list |
| `fn:list:len(List)` | 1 | Returns list length |
| `fn:list:cons(Head, Tail)` | 2 | Constructs list from head and tail |

### 1.4 Pair and Tuple Functions

| Function | Arity | Description |
|----------|-------|-------------|
| `fn:pair(A, B)` | 2 | Constructs a pair |
| `fn:tuple(...)` | Variable | Identity (1 arg), pair (2 args), or nested pairs (3+) |
| `fn:some(X)` | 1 | Constructs an option type element |

### 1.5 Map and Struct Functions

| Function | Arity | Description |
|----------|-------|-------------|
| `fn:map(...)` | Variable | Constructs a map from key-value pairs |
| `fn:map:get(Map, Key)` | 2 | Returns value at key |
| `fn:struct(...)` | Variable | Constructs a struct from field-value pairs |
| `fn:struct:get(Struct, Field)` | 2 | Returns value of field |

### 1.6 String Functions

| Function | Arity | Description |
|----------|-------|-------------|
| `fn:string:concat(S1, S2, ...)` | Variable | Concatenates strings |
| `fn:string:replace(Str, Old, New, N)` | 4 | Replaces first N occurrences of Old with New |

**Note**: String searching uses **predicates** (`:string:contains`), not functions.

### 1.7 Conversion Functions

| Function | Arity | Description |
|----------|-------|-------------|
| `fn:number:to_string(N)` | 1 | Converts integer to string |
| `fn:float64:to_string(F)` | 1 | Converts float to string |
| `fn:name:to_string(Name)` | 1 | Converts name atom to string |
| `fn:name:root(Name)` | 1 | Returns first part of hierarchical name |
| `fn:name:tip(Name)` | 1 | Returns last part of hierarchical name |
| `fn:name:list(Name)` | 1 | Converts name to list of parts |

---

## 2. Reducer Functions (Aggregation)

Reducer functions are used in `let Var = fn:reducer()` within transform pipelines.

### 2.1 Collection Reducers

| Function | Arity | Description |
|----------|-------|-------------|
| `fn:collect(...)` | Variable | Collects tuples into a list |
| `fn:collect_distinct(...)` | Variable | Collects unique tuples into a list |
| `fn:collect_to_map(Key, Value)` | 2 | Collects key-value pairs into a map |
| `fn:pick_any(X)` | 1 | Picks any single element from set |

### 2.2 Numeric Reducers

| Function | Arity | Description |
|----------|-------|-------------|
| `fn:sum(X)` | 1 | Sum of numeric values |
| `fn:max(X)` | 1 | Maximum value |
| `fn:min(X)` | 1 | Minimum value |
| `fn:avg(X)` | 1 | Average of numeric values |
| `fn:count()` | 0 | Count of elements |
| `fn:count_distinct()` | 0 | Count of unique elements |

**Float variants**:
| Function | Arity | Description |
|----------|-------|-------------|
| `fn:float:sum(X)` | 1 | Sum of float64 values |
| `fn:float:max(X)` | 1 | Maximum float64 value |
| `fn:float:min(X)` | 1 | Minimum float64 value |

### 2.3 Aggregation Example

```mangle
sales_stats(Region, Total, Avg, Max, Count) :-
    sale(Region, Amount) |>
    do fn:group_by(Region),
    let Total = fn:sum(Amount),
    let Avg = fn:avg(Amount),
    let Max = fn:max(Amount),
    let Count = fn:count().
```

---

## 3. Built-in Predicates (`:`)

Predicates start with `:` and are used in rule bodies for pattern matching and filtering.

### 3.1 String/Name Matching Predicates

| Predicate | Arity | Description |
|-----------|-------|-------------|
| `:match_prefix(Name, Prefix)` | 2 | Name starts with Prefix |
| `:string:starts_with(Str, Prefix)` | 2 | String starts with Prefix |
| `:string:ends_with(Str, Suffix)` | 2 | String ends with Suffix |
| `:string:contains(Str, Sub)` | 2 | String contains Substring |

**Note**: These are **predicates**, not functions. Use in rule body:
```mangle
# Find strings containing "error"
has_error(S) :- message(S), :string:contains(S, "error").
```

### 3.2 Comparison Predicates

| Predicate | Arity | Description |
|-----------|-------|-------------|
| `:lt(X, Y)` | 2 | X < Y (less than) |
| `:le(X, Y)` | 2 | X <= Y (less or equal) |
| `:gt(X, Y)` | 2 | X > Y (greater than) |
| `:ge(X, Y)` | 2 | X >= Y (greater or equal) |

**Usage**: Can use these OR infix operators (`<`, `<=`, `>`, `>=`).

### 3.3 List Predicates

| Predicate | Arity | Mode | Description |
|-----------|-------|------|-------------|
| `:list:member(Elem, List)` | 2 | out, in | Binds Elem to each element of List |

**Example**:
```mangle
# Iterate over list elements
process_item(Item) :- items(List), :list:member(Item, List).
```

### 3.4 Pattern Matching Predicates

| Predicate | Arity | Mode | Description |
|-----------|-------|------|-------------|
| `:match_pair(Pair, A, B)` | 3 | in, out, out | Destructures pair |
| `:match_cons(List, Head, Tail)` | 3 | in, out, out | Destructures list |
| `:match_nil(List)` | 1 | in | Matches empty list |
| `:match_entry(Map, Key, Value)` | 3 | in, in, out | Extracts map entry |
| `:match_field(Struct, Field, Value)` | 3 | in, in, out | Extracts struct field |

**Mode legend**: `in` = bound input, `out` = output (binds variable)

**Examples**:
```mangle
# Extract struct fields
person_name(ID, Name) :-
    person(ID, Data),
    :match_field(Data, /name, Name).

# Process list recursively
sum_list(List, 0) :- :match_nil(List).
sum_list(List, Total) :-
    :match_cons(List, Head, Tail),
    sum_list(Tail, Rest),
    Total = fn:plus(Head, Rest).
```

### 3.5 Filter Predicate

| Predicate | Arity | Description |
|-----------|-------|-------------|
| `:filter(BoolExpr)` | 1 | Turns boolean expression into predicate |

### 3.6 Distance Predicate

| Predicate | Arity | Description |
|-----------|-------|-------------|
| `:within_distance(X, Y, Z)` | 3 | True if \|X - Y\| < Z |

---

## 4. Casing Conventions

**IMPORTANT**: Function names are **lowercase** after the prefix:

```mangle
# CORRECT
fn:sum(X)
fn:count()
fn:group_by(Cat)
fn:list:contains(L, E)

# WRONG (will cause errors)
fn:Sum(X)       # Wrong - capital S
fn:Count()      # Wrong - capital C
fn:Group_By()   # Wrong - capitals
```

**Exception**: The `fn:` prefix itself is always lowercase.

---

## 5. Functions That DO NOT Exist

These are commonly hallucinated by AI but **do not exist in Mangle**:

```mangle
# NONE OF THESE EXIST:
fn:contains(...)         # Use :string:contains or fn:list:contains
fn:string_contains(...)  # Use :string:contains (predicate!)
fn:substring(...)        # Not available
fn:regex(...)            # Not available
fn:match(...)            # Not available
fn:lower(...)            # Not available
fn:upper(...)            # Not available
fn:trim(...)             # Not available
fn:split(...)            # Not available
fn:startswith(...)       # Use :string:starts_with (predicate!)
fn:endswith(...)         # Use :string:ends_with (predicate!)
fn:len(...)              # Use fn:list:len for lists
fn:append(...)           # Use fn:list:append
fn:get(...)              # Use fn:list:get, fn:map:get, or fn:struct:get
```

---

## 6. Function vs Predicate Decision Guide

| Need to... | Use | Example |
|------------|-----|---------|
| **Compute a value** | Function (`fn:`) | `X = fn:plus(A, B)` |
| **Check a condition** | Predicate (`:`) | `:string:contains(S, "err")` |
| **Extract from structure** | Predicate (`:`) | `:match_field(R, /name, N)` |
| **Aggregate values** | Reducer in transform | `let N = fn:count()` |
| **Test membership** | Predicate (`:`) | `:list:member(E, List)` |

---

**Next**: See [300-PATTERN_LIBRARY](300-PATTERN_LIBRARY.md) for common usage patterns.
