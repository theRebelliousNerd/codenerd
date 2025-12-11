# Mangle Builtin Functions: Structured Data

## Pairs

### fn:pair

**Signature**: `fn:pair(First, Second) → Pair`

**Purpose**: Constructs a pair (2-tuple) from two elements.

**Examples**:
```mangle
# Create pair
coordinate(P) :- P = fn:pair(10, 20).

# Pair of different types
person_age(P) :- P = fn:pair(/john, 30).

# Dynamic pair creation
make_pair(P) :- a(X), b(Y), P = fn:pair(X, Y).
```

**Access**:
- Use `:match_pair(Pair, First, Second)` to deconstruct
- Use `fn:pair:fst(Pair)` to get first element
- Use `fn:pair:snd(Pair)` to get second element

---

### fn:pair:fst

**Signature**: `fn:pair:fst(Pair) → First`

**Purpose**: Extracts the first element of a pair.

**Examples**:
```mangle
# Get first element
first_coordinate(X) :- coordinate(P), X = fn:pair:fst(P).

# Extract key from key-value pair
key(K) :- kv_pair(P), K = fn:pair:fst(P).
```

**Alternative**: Use `:match_pair` pattern matching.

---

### fn:pair:snd

**Signature**: `fn:pair:snd(Pair) → Second`

**Purpose**: Extracts the second element of a pair.

**Examples**:
```mangle
# Get second element
second_coordinate(Y) :- coordinate(P), Y = fn:pair:snd(P).

# Extract value from key-value pair
value(V) :- kv_pair(P), V = fn:pair:snd(P).
```

**Alternative**: Use `:match_pair` pattern matching.

---

## Tuples

### fn:tuple

**Signature**: `fn:tuple(Elem1, Elem2, ..., ElemN) → Tuple` (N ≥ 3)

**Purpose**: Constructs a tuple with 3 or more elements.

**Examples**:
```mangle
# 3-tuple (coordinate with z)
point_3d(P) :- P = fn:tuple(10, 20, 30).

# 4-tuple (RGBA color)
color(C) :- C = fn:tuple(255, 128, 0, 255).

# Dynamic tuple
make_triple(T) :- a(X), b(Y), c(Z), T = fn:tuple(X, Y, Z).
```

**Implementation Note**: Tuples are represented as nested pairs internally.

**Access**: Use pattern matching with `:match_pair` repeatedly:
```mangle
# Extract elements from 3-tuple
tuple_elements(A, B, C) :-
    data(T),
    :match_pair(T, A, Rest),  # Get first element
    :match_pair(Rest, B, C).  # Get second and third
```

**Note**: For exactly 2 elements, use `fn:pair` instead.

---

## Maps

### fn:map

**Signature**: `fn:map(Key1, Value1, ..., KeyN, ValueN) → Map`

**Purpose**: Constructs a map (key-value dictionary) from alternating keys and values.

**Examples**:
```mangle
# Create map with two entries
person(M) :- M = fn:map(/name, "Alice", /age, 30).

# Equivalent bracket notation
person(M) :- M = [/name: "Alice", /age: 30].

# Dynamic map
make_map(M) :- name(N), age(A), M = fn:map(/name, N, /age, A).

# Empty map
empty(M) :- M = fn:map().
empty(M) :- M = [:].  # Bracket notation
```

**Key Requirements**:
- Keys should be unique
- Keys can be any constant (typically atoms like `/name`)
- Values can be any type

**Access**: Use `:match_entry(Map, Key, Value)` to extract values.

---

### Map Operations

**Extract value**:
```mangle
person_name(N) :- person(M), :match_entry(M, /name, N).
```

**Check key exists**:
```mangle
has_age(M) :- person(M), :match_entry(M, /age, _).
```

**Multiple field extraction**:
```mangle
full_info(Name, Age) :-
    person(M),
    :match_entry(M, /name, Name),
    :match_entry(M, /age, Age).
```

---

### Map Equality

Two maps are equal if they have:
- The same keys
- The same values for each key
- Order doesn't matter

```mangle
# These are equal
map1([/a: 1, /b: 2]).
map2([/b: 2, /a: 1]).

same :- map1(M1), map2(M2), M1 = M2.  # Succeeds
```

---

## Structs

### fn:struct

**Signature**: `fn:struct(Label1, Value1, ..., LabelN, ValueN) → Struct`

**Purpose**: Constructs a struct (record with named fields).

**Examples**:
```mangle
# Create struct
person(S) :- S = fn:struct(/name, "Alice", /age, 30, /city, "NYC").

# Equivalent brace notation
person(S) :- S = {/name: "Alice", /age: 30, /city: "NYC"}.

# Dynamic struct
make_struct(S) :-
    name(N),
    age(A),
    S = fn:struct(/name, N, /age, A).
```

**Difference from Maps**:
- Structs typically have fixed field names
- Maps can have dynamic keys
- Internally very similar

**Access**: Use `:match_field(Struct, Label, Value)` to extract fields.

---

### Struct Operations

**Extract field**:
```mangle
person_name(N) :- person(S), :match_field(S, /name, N).
```

**Multiple field extraction**:
```mangle
full_info(Name, Age) :-
    person(S),
    :match_field(S, /name, Name),
    :match_field(S, /age, Age).
```

**Nested struct access**:
```mangle
# Struct with nested struct
person({
    /name: "Alice",
    /address: {
        /city: "NYC",
        /zip: "10001"
    }
}).

city(C) :-
    person(P),
    :match_field(P, /address, Addr),
    :match_field(Addr, /city, C).
```

---

### Struct with Optional Fields

**Type Declaration**:
```mangle
Decl person_record(Struct)
  bounds [
    fn:Struct(
        /name: /string,
        /age: /number,
        /email: fn:opt(/string)  # Optional field
    )
  ].
```

**Usage**:
```mangle
# With email
person_record({/name: "Alice", /age: 30, /email: "alice@example.com"}).

# Without email (valid because it's optional)
person_record({/name: "Bob", /age: 25}).
```

---

## Complex Structured Data

### List of Structs

```mangle
# Triangle as list of coordinate structs
triangle([
    {/x: 1, /y: 2},
    {/x: 5, /y: 10},
    {/x: 12, /y: 5}
]).

# Extract coordinates
vertex_x(X) :-
    triangle(T),
    :match_cons(T, Vertex, _),
    :match_field(Vertex, /x, X).
```

### Map with List Values

```mangle
# Person with multiple skills
person([
    /name: "Alice",
    /skills: [/python, /go, /rust]
]).

# Check if person has skill
has_skill(Skill) :-
    person(M),
    :match_entry(M, /skills, Skills),
    fn:list_contains(Skills, Skill).
```

### Struct with Map Field

```mangle
# Person with metadata map
person({
    /name: "Alice",
    /metadata: [/created: "2024-01-01", /updated: "2024-12-11"]
}).

created_date(Date) :-
    person(P),
    :match_field(P, /metadata, Meta),
    :match_entry(Meta, /created, Date).
```

---

## Construction vs. Deconstruction

| Type | Constructor | Deconstructor |
|------|-------------|---------------|
| Pair | `fn:pair(A, B)` | `:match_pair(P, A, B)` |
| List | `fn:list(...)` or `[...]` | `:match_cons(L, H, T)` |
| Map | `fn:map(...)` or `[K:V, ...]` | `:match_entry(M, K, V)` |
| Struct | `fn:struct(...)` or `{L:V, ...}` | `:match_field(S, L, V)` |

**Pattern**: Constructors build, pattern matchers deconstruct.

---

## Accessor Functions vs. Pattern Matching

**Accessors** (functions that extract):
- `fn:pair:fst(P)` - Get first element of pair
- `fn:pair:snd(P)` - Get second element of pair
- `fn:list:get(L, I)` - Get element at index

**Pattern Matchers** (predicates that deconstruct):
- `:match_pair(P, First, Second)` - Deconstruct pair
- `:match_cons(L, Head, Tail)` - Deconstruct list
- `:match_entry(M, Key, Value)` - Extract map entry
- `:match_field(S, Label, Value)` - Extract struct field

**When to use which**:
- Use **accessors** when you just need one element
- Use **pattern matching** when you need multiple elements or want to check structure

```mangle
# Just need first element - use accessor
first(F) :- data(P), F = fn:pair:fst(P).

# Need both elements - use pattern matching
both(F, S) :- data(P), :match_pair(P, F, S).
```

---

## Type Declarations for Structured Data

### Pair Type

```mangle
Decl coordinate(Pair)
  bounds [ fn:Pair(/number, /number) ].
```

### List Type

```mangle
Decl numbers(List)
  bounds [ fn:List(/number) ].
```

### Map Type

```mangle
Decl person_map(Map)
  bounds [ fn:Map(/atom, /any) ].  # Atom keys, any values
```

### Struct Type

```mangle
Decl person_struct(Struct)
  bounds [
    fn:Struct(
        /name: /string,
        /age: /number
    )
  ].
```

### Nested Types

```mangle
# List of pairs
Decl coordinates(List)
  bounds [ fn:List(fn:Pair(/number, /number)) ].

# Map with list values
Decl person_skills(Map)
  bounds [ fn:Map(/atom, fn:List(/atom)) ].
```

---

## Common Patterns

### Converting Between Formats

**Struct to Map**:
```mangle
# Not directly possible - both are primitives
# Would need to manually reconstruct
```

**Map to List of Pairs**:
```mangle
# Extract all entries (if you know the keys)
map_to_pairs(Pairs) :-
    data(M),
    :match_entry(M, K1, V1),
    :match_entry(M, K2, V2),
    Pairs = [fn:pair(K1, V1), fn:pair(K2, V2)].
```

---

## Structured Data Functions Summary

| Function | Arguments | Return | Purpose |
|----------|-----------|--------|---------|
| `fn:pair` | (any, any) | pair | Construct pair |
| `fn:pair:fst` | (pair) | any | Get first element |
| `fn:pair:snd` | (pair) | any | Get second element |
| `fn:tuple` | (any, any, any, ...) | tuple | Construct tuple (≥3 elements) |
| `fn:map` | (key, val, ...) | map | Construct map |
| `fn:struct` | (label, val, ...) | struct | Construct struct |

**Related Predicates**:
- `:match_pair(pair, first, second)` - Deconstruct pair
- `:match_entry(map, key, value)` - Extract map entry
- `:match_field(struct, label, value)` - Extract struct field
