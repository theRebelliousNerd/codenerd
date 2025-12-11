# Mangle Struct and Map Functions

Complete reference for struct and map construction and manipulation in Mangle.

## Overview

Mangle supports three key-value data structures:
- **Structs** - Immutable records with named fields (keys are atoms/names)
- **Maps** - Key-value mappings (keys and values can be any type)
- **Pairs** - Two-element tuples

---

## Struct Construction

### fn:struct

**Signature:** `fn:struct(Key1, Value1, Key2, Value2, ...) -> Struct`

**Description:** Constructs a struct from alternating key-value pairs.

**Parameters:**
- Even number of arguments
- Keys are typically `Type<name>` (atoms)
- Values can be any type
- Returns `Type<struct>`

**Examples:**

```mangle
# Basic struct
Person = fn:struct(/name, "Alice", /age, 30).
# Person = { /name: "Alice", /age: 30 }

# Empty struct
Empty = fn:struct().
# Empty = {}

# Nested struct
Record = fn:struct(
  /id, 123,
  /user, fn:struct(/name, "Bob", /email, "bob@example.com"),
  /active, /true
).

# Mixed value types
Data = fn:struct(
  /count, 42,
  /label, "test",
  /flag, /enabled
).
```

**Common Mistakes:**
- ❌ Odd number of arguments: `fn:struct(/a, 1, /b)` - error
- ❌ Using strings as keys: prefer atoms `/key` over `"key"`
- ❌ Duplicate keys: behavior undefined (last value may win)

---

## Struct Deconstruction

### :match_field

**Signature:** `:match_field(Struct, FieldName, Value)`

**Mode:** `(Input, Input, Output)`

**Description:** Extracts a field value from a struct by field name.

**Parameters:**
- First arg: `Type<struct>` (must be bound)
- Second arg: `Type<name>` field name (must be bound)
- Third arg: variable to bind to field value (output)

**Examples:**

```mangle
# Extract field from struct
Person = fn:struct(/name, "Alice", /age, 30),
:match_field(Person, /name, Name).
# Binds Name = "Alice"

# Check specific field value
has_name(S, N) :- :match_field(S, /name, N).

# In rule body
get_user_name(User, Name) :-
  user_data(User, Data),
  :match_field(Data, /name, Name).

# Field doesn't exist - fails
:match_field(fn:struct(/a, 1), /b, X).  # Fails

# Nested struct access
get_nested(Outer, Inner, Value) :-
  :match_field(Outer, /inner, Inner),
  :match_field(Inner, /value, Value).
```

**Common Mistakes:**
- ❌ Unbound struct: struct must be bound before matching
- ❌ Non-existent field: predicate fails (not an error)
- ❌ Using string keys: `:match_field(S, "key", V)` should be `:match_field(S, /key, V)`

---

### fn:struct:get

**Signature:** `fn:struct:get(Struct, FieldName) -> Value`

**Description:** Function form of field access. Returns field value or raises error if not found.

**Parameters:**
- First arg: `Type<struct>`
- Second arg: `Type<name>` field name
- Returns value of the field

**Examples:**

```mangle
# Get field value
Person = fn:struct(/name, "Alice", /age, 30),
Name = fn:struct:get(Person, /name).
# Name = "Alice"

# In computation
full_info(Info) :-
  data(D),
  Name = fn:struct:get(D, /name),
  Age = fn:struct:get(D, /age),
  Info = fn:string_concat(Name, " is ", Age, " years old").

# Error if field missing
X = fn:struct:get(fn:struct(/a, 1), /b).  # Runtime error
```

**Common Mistakes:**
- ❌ Missing field causes error (unlike `:match_field` which fails)
- ❌ Can't use for optional fields - use `:match_field` for that

---

## Map Construction

### fn:map

**Signature:** `fn:map(Key1, Value1, Key2, Value2, ...) -> Map`

**Description:** Constructs a map from alternating key-value pairs.

**Parameters:**
- Even number of arguments
- Keys can be any type (must be hashable)
- Values can be any type
- Returns `Type<map<K, V>>`

**Examples:**

```mangle
# Basic map
Scores = fn:map(/alice, 100, /bob, 95, /charlie, 88).
# Scores = map{ /alice: 100, /bob: 95, /charlie: 88 }

# Empty map
Empty = fn:map().

# Map with string keys
Config = fn:map("host", "localhost", "port", 8080).

# Map with int keys
Data = fn:map(1, "first", 2, "second", 3, "third").

# Nested map
Complex = fn:map(
  /users, fn:map(/alice, 30, /bob, 25),
  /active, /true
).
```

**Common Mistakes:**
- ❌ Odd number of arguments
- ❌ Duplicate keys: last value wins (order undefined)

---

## Map Deconstruction

### :match_entry

**Signature:** `:match_entry(Map, Key, Value)`

**Mode:** `(Input, Input, Output)`

**Description:** Retrieves a value from a map by key.

**Parameters:**
- First arg: `Type<map<K,V>>` (must be bound)
- Second arg: key of type K (must be bound)
- Third arg: variable to bind to value (output)

**Examples:**

```mangle
# Get value by key
Scores = fn:map(/alice, 100, /bob, 95),
:match_entry(Scores, /alice, Score).
# Binds Score = 100

# Check if key exists and get value
has_score(User, Score) :-
  scores(Map),
  :match_entry(Map, User, Score).

# Key doesn't exist - fails
:match_entry(fn:map(/a, 1), /b, X).  # Fails

# Iterate all entries (nondeterministic)
map_entry(K, V) :-
  data(Map),
  :match_entry(Map, K, V).
# This will generate all key-value pairs!
```

**Important:** If the key is unbound, `:match_entry` can generate all entries in the map (non-deterministic iteration).

---

### fn:map:get

**Signature:** `fn:map:get(Map, Key) -> Value`

**Description:** Function form of map lookup. Returns value or raises error if key not found.

**Parameters:**
- First arg: `Type<map<K,V>>`
- Second arg: key of type K
- Returns value of type V

**Examples:**

```mangle
# Get value
Scores = fn:map(/alice, 100, /bob, 95),
AliceScore = fn:map:get(Scores, /alice).
# AliceScore = 100

# Error if key missing
X = fn:map:get(fn:map(/a, 1), /b).  # Runtime error
```

---

## Pair Construction and Deconstruction

### fn:pair

**Signature:** `fn:pair(First, Second) -> Pair`

**Description:** Constructs a two-element pair (tuple).

**Parameters:**
- Two arguments of any type
- Returns `Type<pair<T1, T2>>`

**Examples:**

```mangle
# Basic pair
P = fn:pair(1, 2).
# P = (1, 2)

# Pair of different types
Coord = fn:pair("x", 100).

# Pair of pairs (nested)
Nested = fn:pair(fn:pair(1, 2), fn:pair(3, 4)).
```

---

### :match_pair

**Signature:** `:match_pair(Pair, First, Second)`

**Mode:** `(Input, Output, Output)`

**Description:** Deconstructs a pair into its two components.

**Parameters:**
- First arg: `Type<pair<T1, T2>>` (must be bound)
- Second arg: variable to bind first element (output)
- Third arg: variable to bind second element (output)

**Examples:**

```mangle
# Deconstruct pair
P = fn:pair(/alice, 100),
:match_pair(P, Name, Score).
# Binds Name = /alice, Score = 100

# Get just first element
first(Pair, Fst) :- :match_pair(Pair, Fst, _).

# Get just second element
second(Pair, Snd) :- :match_pair(Pair, _, Snd).

# Nested pairs
:match_pair(fn:pair(fn:pair(1, 2), 3), Inner, X).
# Binds Inner = (1, 2), X = 3

# Swap pair
swap(fn:pair(A, B), fn:pair(B, A)).
```

**Common Mistakes:**
- ❌ Using on non-pair: fails (not an error)
- ❌ Expecting more than 2 elements: use `fn:tuple` for N-tuples

---

## Tuples (N-ary)

### fn:tuple

**Signature:** `fn:tuple(Elem1, Elem2, ...) -> Tuple`

**Description:** Constructs an N-element tuple. Implemented as nested pairs internally.

**Parameters:**
- One or more arguments
- Returns nested pair structure

**Examples:**

```mangle
# 1-element tuple (identity)
fn:tuple(42) = 42.

# 2-element tuple (same as pair)
fn:tuple(1, 2) = fn:pair(1, 2).

# 3-element tuple
Triple = fn:tuple(/a, /b, /c).
# Internal representation: pair(/a, pair(/b, /c))

# 4-element tuple
Quad = fn:tuple(1, 2, 3, 4).
# pair(1, pair(2, pair(3, 4)))
```

**Deconstruction:** Use nested `:match_pair`:
```mangle
# Deconstruct 3-tuple
T = fn:tuple(1, 2, 3),
:match_pair(T, First, Rest),     # First = 1, Rest = (2, 3)
:match_pair(Rest, Second, Third). # Second = 2, Third = 3
```

---

## Comparison: Struct vs Map vs Pair

| Feature | Struct | Map | Pair |
|---------|--------|-----|------|
| Keys | Atoms/names | Any type | N/A (2 elements) |
| Values | Any type | Any type | Any type |
| Size | Fixed at creation | Fixed at creation | Always 2 |
| Match Predicate | `:match_field` | `:match_entry` | `:match_pair` |
| Get Function | `fn:struct:get` | `fn:map:get` | N/A |
| Use Case | Records with named fields | Key-value lookups | Simple pairs |

---

## Common Patterns

### Extract Multiple Fields
```mangle
get_user_info(Data, Name, Email, Age) :-
  :match_field(Data, /name, Name),
  :match_field(Data, /email, Email),
  :match_field(Data, /age, Age).
```

### Optional Field
```mangle
# Use match_field with fallback
get_email(Data, Email) :-
  :match_field(Data, /email, Email).
get_email(Data, "no-email") :-
  not :match_field(Data, /email, _).
```

### Build Struct from Predicates
```mangle
user_struct(User, Struct) :-
  user_name(User, Name),
  user_age(User, Age),
  user_email(User, Email),
  Struct = fn:struct(/name, Name, /age, Age, /email, Email).
```

### Transform Map Values
```mangle
# Double all values in a map
doubled_entry(K, DoubledV) :-
  original(Map),
  :match_entry(Map, K, V),
  DoubledV = fn:mult(V, 2).

# Collect back to map
doubled_map(NewMap) :-
  doubled_entry(K, V)
  |> do fn:group_by(), let NewMap = fn:CollectToMap(K, V).
```

### Nested Struct Access
```mangle
get_nested_field(Outer, Value) :-
  :match_field(Outer, /config, Config),
  :match_field(Config, /database, DB),
  :match_field(DB, /host, Value).
```

---

## Iteration Patterns

### Iterate Struct Fields
```mangle
# Non-deterministic - generates all fields
field_value(F, V) :-
  data(Struct),
  :match_field(Struct, F, V).
```

**Note:** This requires knowing field names. Mangle doesn't have built-in "get all fields" introspection.

### Iterate Map Entries
```mangle
# If second arg to :match_entry is unbound, iterates all entries
entry(K, V) :-
  data(Map),
  :match_entry(Map, K, V).
```

---

## Type Safety

```mangle
# ✓ Homogeneous map values
fn:map(/a, 1, /b, 2, /c, 3).            # Map<name, int>

# ✗ Heterogeneous values may cause type errors
fn:map(/a, 1, /b, "two").               # Might fail type checking

# ✓ Struct with mixed types (OK)
fn:struct(/name, "Alice", /age, 30).    # Allowed

# ✓ Nested structures
fn:struct(
  /user, fn:struct(/name, "Bob"),
  /scores, fn:map(/math, 95, /english, 88)
).
```

---

## Performance Notes

- Struct and map construction is O(n) in number of entries
- Field/entry lookup is O(n) (linear scan, not hash table)
- Maps and structs are immutable (copy-on-write)
- Large maps/structs can be memory-intensive

---

## Limitations

### No Update Operation
Structs and maps are immutable. To "update," create a new one:

```mangle
# Can't update in place
# Instead, build new struct
updated_struct(Old, New) :-
  :match_field(Old, /name, Name),
  :match_field(Old, /age, OldAge),
  NewAge = fn:plus(OldAge, 1),
  New = fn:struct(/name, Name, /age, NewAge).
```

### No Delete Operation
Can't remove entries. Must rebuild without the entry.

### No Introspection
Can't get "list of all keys" or "number of fields" directly. Must know structure ahead of time or iterate non-deterministically.

---

## Complete Examples

### User Record Example
```mangle
# Define user records
user(1, fn:struct(/name, "Alice", /email, "alice@example.com", /age, 30)).
user(2, fn:struct(/name, "Bob", /email, "bob@example.com", /age, 25)).

# Extract names
user_name(ID, Name) :-
  user(ID, Data),
  :match_field(Data, /name, Name).

# Find users over 25
adult_user(ID, Name) :-
  user(ID, Data),
  :match_field(Data, /name, Name),
  :match_field(Data, /age, Age),
  :gt(Age, 25).
```

### Configuration Map Example
```mangle
# Configuration as map
config(fn:map(
  "host", "localhost",
  "port", 8080,
  "debug", /true
)).

# Get config value
get_config(Key, Value) :-
  config(Map),
  :match_entry(Map, Key, Value).

# Usage
db_host(Host) :- get_config("host", Host).
```
