# Mangle Data Model: Complete Reference

## Core Principle

Mangle represents data as **facts** - logical statements combining values through predicates.

## Constants

Constants are unique identifiers representing objects.

### Name Constants (Atoms)

**Syntax**: Slash-prefixed hierarchical paths

```mangle
/friday
/person/hilbert
/mathematician/euler
/company/google/employee/12345
```

**Properties**:
- Globally unique
- Immutable
- Case-sensitive
- `/person/hilbert` ≠ `/mathematician/hilbert` (always distinct)

**Use Cases**:
- Entity identifiers
- Enumeration values
- Hierarchical namespaces
- Symbolic constants

---

### Numbers

**Syntax**: Integer literals

```mangle
42
-17
0
999999
```

**Type**: `ast.NumberType`, stored as `int64`

**Range**: -9,223,372,036,854,775,808 to 9,223,372,036,854,775,807

**Use Cases**:
- Counts, ages, quantities
- Indices
- Arithmetic computations

---

### Floating-Point Numbers

**Syntax**: Decimal literals

```mangle
3.14
-0.5
2.71828
1.0
```

**Type**: `ast.Float64Type`, stored as `float64`

**Precision**: IEEE 754 double precision

**Use Cases**:
- Measurements
- Scientific calculations
- Percentages, ratios

**Important**: Must use decimal point. `3.0` is float, `3` is int. Types don't mix.

---

### Strings

**Syntax**: Double-quoted text

```mangle
"Hello, world!"
"Alice Smith"
"what is the meaning of life?"
""  # Empty string
```

**Type**: `ast.StringType`

**Escaping**: Standard escape sequences (likely):
- `\"` - Quote
- `\\` - Backslash
- `\n` - Newline
- `\t` - Tab

**Use Cases**:
- Text data
- Descriptions, messages
- File paths (though atoms often preferred)

---

### Bytes

**Syntax**: Not documented in main specs (likely hexadecimal or base64)

**Type**: `ast.BytesType`

**Use Cases**:
- Binary data
- Cryptographic hashes
- Raw file content

---

## Structured Data Types

### Pairs

**Syntax**: `fn:pair(First, Second)` or accessor functions

**Examples**:
```mangle
coordinate(fn:pair(10, 20)).
key_value(fn:pair(/user, "Alice")).
nested(fn:pair(fn:pair(1, 2), fn:pair(3, 4))).
```

**Access**:
- `:match_pair(P, First, Second)` - Deconstruct
- `fn:pair:fst(P)` - Get first
- `fn:pair:snd(P)` - Get second

**Use Cases**:
- Coordinates
- Key-value pairs
- Simple tuples

---

### Tuples

**Syntax**: `fn:tuple(E1, E2, E3, ...)` (3+ elements)

**Examples**:
```mangle
rgb(fn:tuple(255, 128, 0)).
person(fn:tuple(/john, 30, /nyc)).
```

**Implementation**: Nested pairs internally

**Access**: Repeated `:match_pair` deconstruction

**Use Cases**:
- Multi-field records
- Complex coordinates
- Compound values

---

### Lists

**Syntax**:
- `[Elem1, Elem2, ..., ElemN]` (bracket notation)
- `fn:list(Elem1, Elem2, ..., ElemN)` (function notation)
- `[]` (empty list)

**Examples**:
```mangle
numbers([1, 2, 3, 4, 5]).
names(["Alice", "Bob", "Charlie"]).
nested([[1, 2], [3, 4], [5, 6]]).
empty([]).
```

**Construction**:
- `fn:cons(Head, Tail)` - Prepend element
- `fn:append(List1, List2)` - Concatenate

**Deconstruction**:
- `:match_cons(List, Head, Tail)` - Get head and tail
- `:match_nil(List)` - Check empty
- `fn:list:get(List, Index)` - Access by index

**Properties**:
- **Ordered** - [1,2,3] ≠ [3,2,1]
- **Homogeneous** (by convention) - same type elements
- **Equality** - Same elements in same order

**Use Cases**:
- Collections
- Sequences
- Paths, routes

---

### Maps

**Syntax**:
- `[Key1: Value1, Key2: Value2, ...]` (bracket notation)
- `fn:map(Key1, Value1, Key2, Value2, ...)` (function notation)
- `[:]` (empty map)

**Examples**:
```mangle
person([/name: "Alice", /age: 30, /city: "NYC"]).
config([/timeout: 5000, /retries: 3]).
empty([:]).
```

**Access**:
- `:match_entry(Map, Key, Value)` - Extract entry
- Check key exists: `:match_entry(Map, /key, _)`

**Properties**:
- **Unordered** - `[/a: 1, /b: 2]` = `[/b: 2, /a: 1]`
- **Unique keys** - Each key appears once
- **Any key type** - But atoms (/name) are conventional

**Equality**: Same keys with same values (order irrelevant)

**Use Cases**:
- Dictionaries
- Configuration
- Sparse data

---

### Structs

**Syntax**:
- `{Label1: Value1, Label2: Value2, ...}` (brace notation)
- `fn:struct(Label1, Value1, Label2, Value2, ...)` (function notation)

**Examples**:
```mangle
person({/name: "Alice", /age: 30, /city: "NYC"}).
point({/x: 10, /y: 20, /z: 30}).
```

**Access**:
- `:match_field(Struct, Label, Value)` - Extract field

**Difference from Maps**:
- Structs: Fixed schema (conventionally)
- Maps: Dynamic keys

**Equality**: Same fields with same values

**Use Cases**:
- Structured records
- Database rows
- Typed entities

---

## Type System

### Basic Types

| Type | Mangle Syntax | Go Type | Examples |
|------|---------------|---------|----------|
| Name | `/atom` | `ast.NameType` | `/john`, `/color/red` |
| Number | `42` | `ast.NumberType` (int64) | `0`, `-17`, `999` |
| Float | `3.14` | `ast.Float64Type` (float64) | `0.5`, `-2.71828` |
| String | `"text"` | `ast.StringType` | `"Alice"`, `""` |
| Bytes | (varies) | `ast.BytesType` | Binary data |

### Constructed Types

| Type | Constructor | Example |
|------|-------------|---------|
| Pair | `fn:pair(A, B)` | `fn:pair(1, 2)` |
| Tuple | `fn:tuple(A, B, C, ...)` | `fn:tuple(1, 2, 3)` |
| List | `[A, B, ...]` | `[1, 2, 3]` |
| Map | `[K: V, ...]` | `[/a: 1, /b: 2]` |
| Struct | `{L: V, ...}` | `{/x: 1, /y: 2}` |

### Type Declarations

**Basic Type Bounds**:
```mangle
Decl person(Name, Age, City)
  bounds [ /string, /number, /string ].
```

**Constructed Type Bounds**:
```mangle
# List of numbers
Decl numbers(List)
  bounds [ fn:List(/number) ].

# Pair of string and number
Decl labeled_value(Pair)
  bounds [ fn:Pair(/string, /number) ].

# Map with name keys and number values
Decl scores(Map)
  bounds [ fn:Map(/name, /number) ].

# Struct with specific fields
Decl person_record(Struct)
  bounds [
    fn:Struct(
        /name: /string,
        /age: /number,
        /email: fn:opt(/string)  # Optional field
    )
  ].
```

### Union Types

```mangle
Decl id(Value)
  bounds [ fn:Union(/number, /string) ].

# Accepts either number or string
id(42).
id("user-123").
```

### Singleton Types

```mangle
Decl status(Value)
  bounds [ fn:Singleton(/active), fn:Singleton(/inactive) ].

# Only these exact values allowed
status(/active).
status(/inactive).
```

### Any Type

```mangle
Decl flexible(Value)
  bounds [ /any ].

# Accepts any value
flexible(42).
flexible("text").
flexible([1, 2, 3]).
```

---

## Variables

**Syntax**: UPPERCASE identifiers

```mangle
X
Y
Person
Value
ResultList
```

**Properties**:
- **Logical variables** - Not assignment like in imperative languages
- **Unification** - Variables bind to values through pattern matching
- **Scope** - Local to a single clause

**Anonymous Variable**: `_` (underscore)
- Don't care value
- Never unifies
- Each `_` is distinct

---

## Predicates and Facts

### Predicates

**Syntax**: `predicate_name(Arg1, ..., ArgN)`

**Naming**: Lowercase, underscores allowed

**Arity**: Number of arguments (fixed per predicate)

**Examples**:
```mangle
person(Name, Age, City)
parent(Person, Child)
edge(From, To)
temperature(Location, Value, Timestamp)
```

### Facts

**Definition**: Predicate applied to **constants only**

**Examples**:
```mangle
person("Alice", 30, "NYC").
parent(/alice, /bob).
edge(/a, /b).
temperature(/sensor_1, 72.5, /timestamp/2024_12_11).
```

**Properties**:
- **Ground atoms** - No variables
- **Extensional database** (EDB) - Base facts
- **Input data** - What you start with

---

## Atoms

**Definition**: Predicate applied to arguments (constants OR variables)

**Examples**:
```mangle
# Fact (ground atom)
person("Alice", 30, "NYC").

# Atom with variables (used in rules)
person(Name, Age, City)
parent(X, Y)
```

---

## Complex Data Examples

### Volunteer Database

```mangle
volunteer_record({
    /id: /v/1,
    /name: "Aisha Salehi",
    /time_available: [
        fn:pair(/monday, /morning),
        fn:pair(/monday, /afternoon)
    ],
    /interest: [/skill/frontline],
    /skill: [/skill/admin, /skill/facilitate, /skill/teaching]
}).
```

### Triangle Geometry

```mangle
triangle_2d([
    {/x: 1, /y: 2},
    {/x: 5, /y: 10},
    {/x: 12, /y: 5}
]).
```

### Trip Itinerary

```mangle
trip({
    /legs: [
        {/from: /nyc, /to: /london, /price: 500},
        {/from: /london, /to: /paris, /price: 100}
    ],
    /total_price: 600
}).
```

---

## Data Equality

### Value Equality

Two constants are equal if:
- Same type
- Same value

```mangle
42 = 42          # True
42 = 43          # False
42 = "42"        # False (different types)
/alice = /alice  # True
/alice = /bob    # False
```

### Structural Equality

**Lists**: Same elements in same order
```mangle
[1, 2, 3] = [1, 2, 3]  # True
[1, 2, 3] = [3, 2, 1]  # False
```

**Maps**: Same keys with same values (order irrelevant)
```mangle
[/a: 1, /b: 2] = [/b: 2, /a: 1]  # True
```

**Structs**: Same fields with same values
```mangle
{/x: 1, /y: 2} = {/y: 2, /x: 1}  # True
```

**Pairs**: Both elements equal
```mangle
fn:pair(1, 2) = fn:pair(1, 2)  # True
```

---

## Data Model Best Practices

1. **Use names for identities**: `/user/alice` not `"user_alice"`
2. **Use numbers for quantities**: `age(30)` not `age("30")`
3. **Use structs for records**: `{/name: ..., /age: ...}` not separate facts
4. **Use lists for sequences**: `path([/a, /b, /c])` not `path_1(/a), path_2(/b), ...`
5. **Type declarations**: Always declare predicates with type bounds
6. **Consistent types**: Don't mix numbers and strings in same field

---

## Memory Representation (Internal)

### AST Nodes

From `ast/ast.go`:

```go
type Constant struct {
    Type      ConstantType  // NameType, NumberType, etc.
    Symbol    string        // For names, strings
    NumValue  int64         // For numbers
    FloatValue float64      // For floats
    fst, snd  *Value        // For pairs, lists, maps, structs
}
```

### Shapes

Complex types use "shapes":
- `ListShape` - Linked list via fst/snd
- `MapShape` - Tree structure
- `StructShape` - Field-value pairs
- `PairShape` - Simple pair

All built from primitives through recursive structure.
