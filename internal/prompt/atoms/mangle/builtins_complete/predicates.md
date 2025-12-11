# Mangle Builtin Predicates: Complete Reference

## Comparison Predicates

### Equality: =

**Signature**: `Left = Right`

**Purpose**: Unifies two terms or checks equality of constants.

**Behavior**:
- If both are constants: checks if they're equal
- If one is variable: binds the variable
- If both are variables: unifies them (same value)

**Examples**:
```mangle
# Constant equality check
result(X) :- age(X, A), A = 30.

# Variable binding
result(X) :- age(X, A), A = Y, Y = 30.

# Filter by constant
result(X) :- parent(X, /john).

# Equivalent to
result(X) :- parent(X, Y), Y = /john.
```

**Safety**: Variables become bound through unification with constants or other bound variables.

---

### Inequality: !=

**Signature**: `Left != Right`

**Purpose**: Checks that two terms are NOT equal.

**Behavior**:
- Succeeds if terms don't unify
- Fails if terms are equal
- Does NOT bind variables

**Examples**:
```mangle
# Different people
different_people(X, Y) :- person(X), person(Y), X != Y.

# Not a specific value
not_john(X) :- person(X), X != /john.

# Different ages
age_difference(X, Y) :- age(X, A), age(Y, B), A != B.
```

**Safety**: Variables in `!=` must be bound before the inequality.

**UNSAFE**:
```mangle
# ERROR: X and Y not bound
different(X, Y) :- X != Y.
```

**SAFE**:
```mangle
different(X, Y) :- thing(X), thing(Y), X != Y.
```

---

### Less Than: <

**Signature**: `Left < Right`

**Purpose**: Numeric comparison - left is less than right.

**Type**: Both arguments must be **numbers** (integers) or both **floats**.

**Examples**:
```mangle
# Ages under 18
minor(X) :- person(X), age(X, A), A < 18.

# Price comparison
cheaper(X, Y) :- price(X, P1), price(Y, P2), P1 < P2.

# Range check
in_range(X) :- value(X, V), V < 100, V > 0.
```

**Safety**: Variables in `<` must be bound before comparison.

**Type Error**:
```mangle
# ERROR: Comparing number to string
bad(X) :- age(X, A), A < "30".
```

---

### Less Than or Equal: <=

**Signature**: `Left <= Right`

**Purpose**: Numeric comparison - left is less than or equal to right.

**Type**: Both arguments must be **numbers** or both **floats**.

**Examples**:
```mangle
# At most 18
at_most_18(X) :- person(X), age(X, A), A <= 18.

# Non-positive
non_positive(X) :- value(X, V), V <= 0.

# Budget check
within_budget(Item, Budget) :- price(Item, P), P <= Budget.
```

**Note**: Mangle does NOT have `>` or `>=` operators. Use negation of opposite:
- `X > Y` becomes `not (X <= Y)` BUT requires both bound
- `X >= Y` becomes `not (X < Y)` BUT requires both bound

Better: Flip the operands:
- `X > Y` → `Y < X`
- `X >= Y` → `Y <= X` or `not (Y > X)` → `not (X < Y)`

---

## Pattern Matching Predicates

### Match Pair: :match_pair

**Signature**: `:match_pair(Pair, First, Second)`

**Purpose**: Deconstructs a pair into its two components.

**Behavior**:
- If `Pair` is a pair: binds `First` and `Second` to the elements
- If `Pair` is not a pair: fails
- Can be used for both extraction and construction verification

**Examples**:
```mangle
# Extract pair elements
first_element(F) :- data(P), :match_pair(P, F, _).
second_element(S) :- data(P), :match_pair(P, _, S).

# Both elements
both(F, S) :- data(P), :match_pair(P, F, S).

# Check pair structure and values
specific_pair(P) :-
    data(P),
    :match_pair(P, /key, Value),
    Value = "expected".
```

**Alternative**: Use accessor functions `fn:pair:fst` and `fn:pair:snd`.

---

### Match Cons: :match_cons

**Signature**: `:match_cons(List, Head, Tail)`

**Purpose**: Deconstructs a non-empty list into head (first element) and tail (remaining elements).

**Behavior**:
- If `List` is non-empty: binds `Head` to first element, `Tail` to rest
- If `List` is empty: fails
- "cons" refers to the LISP constructor: head + tail

**Examples**:
```mangle
# Get first element
first(H) :- list_data(L), :match_cons(L, H, _).

# Process list recursively
sum_list([], 0).
sum_list(L, Sum) :-
    :match_cons(L, H, T),
    sum_list(T, SubSum),
    Sum = fn:plus(H, SubSum).

# Check list starts with specific value
starts_with_one(L) :- :match_cons(L, H, _), H = 1.

# Pattern match on multi-element list
at_least_two(L) :-
    :match_cons(L, First, Rest),
    :match_cons(Rest, Second, _).
```

**Safety**: `List` must be bound before matching.

---

### Match Nil: :match_nil

**Signature**: `:match_nil(List)`

**Purpose**: Checks if a list is empty.

**Behavior**:
- Succeeds if `List` is `[]`
- Fails if `List` is non-empty

**Examples**:
```mangle
# Base case for recursion
sum_list(L, 0) :- :match_nil(L).

# Check emptiness
is_empty(L) :- list_data(L), :match_nil(L).

# Alternative to checking against []
process_list(L) :-
    :match_nil(L),
    # handle empty case
    .

process_list(L) :-
    :match_cons(L, H, T),
    # handle non-empty case
    .
```

**Alternative**: Can also use `L = []` directly.

---

### Match Entry: :match_entry

**Signature**: `:match_entry(Map, Key, Value)`

**Purpose**: Extracts a key-value pair from a map.

**Behavior**:
- If `Map` contains `Key`: binds `Value` to the associated value
- If `Map` doesn't contain `Key`: fails
- Can be used to check key existence or extract value

**Examples**:
```mangle
# Extract value for specific key
age_value(V) :-
    person_map(M),
    :match_entry(M, /age, V).

# Check key exists with specific value
is_adult(M) :-
    person_map(M),
    :match_entry(M, /age, Age),
    Age >= 18.

# Extract multiple fields
full_person(Name, Age) :-
    person_map(M),
    :match_entry(M, /name, Name),
    :match_entry(M, /age, Age).

# Check if key exists (don't care about value)
has_email(M) :- person_map(M), :match_entry(M, /email, _).
```

**Safety**: `Map` must be bound. `Key` should be bound (though it could be a variable for iteration, but this is inefficient).

---

### Match Field: :match_field

**Signature**: `:match_field(Struct, FieldName, Value)`

**Purpose**: Extracts a field value from a struct.

**Behavior**:
- If `Struct` contains `FieldName`: binds `Value` to the field value
- If field doesn't exist: fails
- Structs are like maps but with fixed field names

**Examples**:
```mangle
# Extract struct field
person_name(N) :-
    person_record(R),
    :match_field(R, /name, N).

# Multiple field extraction
person_info(Name, Age) :-
    person_record(R),
    :match_field(R, /name, Name),
    :match_field(R, /age, Age).

# Check field value
adult_record(R) :-
    person_record(R),
    :match_field(R, /age, Age),
    Age >= 18.

# Nested struct access
address_city(City) :-
    person_record(R),
    :match_field(R, /address, Addr),
    :match_field(Addr, /city, City).
```

**Difference from maps**: Structs use atom keys (e.g., `/name`), maps can use any constant key.

---

## String Pattern Matching

### Match Prefix: :match_prefix

**Signature**: `:match_prefix(String, Prefix)`

**Purpose**: Checks if a string starts with a given prefix.

**Examples**:
```mangle
# Find strings starting with "test"
test_files(F) :- file(F), :match_prefix(F, "test").

# Name starts with "Dr."
doctor(P) :- person_name(P, Name), :match_prefix(Name, "Dr.").
```

**Alternative**: Could be called `:starts_with` but uses `:match_prefix` naming.

---

### Starts With: :starts_with

**Signature**: `:starts_with(String, Prefix)`

**Purpose**: Checks if a string starts with a given prefix (may be alias of :match_prefix).

---

### Ends With: :ends_with

**Signature**: `:ends_with(String, Suffix)`

**Purpose**: Checks if a string ends with a given suffix.

**Examples**:
```mangle
# Find .go files
go_file(F) :- file(F), :ends_with(F, ".go").

# Names ending in "son"
surname_son(P) :- person(P), name(P, N), :ends_with(N, "son").
```

---

### Contains: :contains

**Signature**: `:contains(String, Substring)`

**Purpose**: Checks if a string contains a substring.

**Examples**:
```mangle
# Find files with "test" anywhere in name
test_related(F) :- file(F), :contains(F, "test").

# Description mentions "urgent"
urgent_task(T) :- task(T), description(T, D), :contains(D, "urgent").
```

---

## Built-in Type Checks

While not explicitly documented, Mangle likely supports type checking predicates:

### Type Checking Patterns

Based on the type system, these patterns are common:

```mangle
# Check if value is a number
is_number(X) :- number_value(X), X = X.  # Unification forces type check

# Check if value is a list
is_list(L) :- :match_cons(L, _, _).
is_list(L) :- :match_nil(L).

# Check if value is a map
has_map_entry(M) :- :match_entry(M, _, _).
```

**Note**: Explicit type predicates may not exist - type checking is typically done via pattern matching and type bounds in declarations.

## Summary Table

| Predicate | Arity | Purpose | Binds Variables? |
|-----------|-------|---------|------------------|
| `=` | 2 | Equality/unification | Yes |
| `!=` | 2 | Inequality | No |
| `<` | 2 | Less than (numeric) | No |
| `<=` | 2 | Less than or equal | No |
| `:match_pair` | 3 | Deconstruct pair | Yes |
| `:match_cons` | 3 | Deconstruct list (head/tail) | Yes |
| `:match_nil` | 1 | Check empty list | No |
| `:match_entry` | 3 | Extract map entry | Yes |
| `:match_field` | 3 | Extract struct field | Yes |
| `:match_prefix` | 2 | String starts with | No |
| `:starts_with` | 2 | String starts with | No |
| `:ends_with` | 2 | String ends with | No |
| `:contains` | 2 | String contains | No |
