# Mangle Type Conversion Functions

Complete reference for type conversion and type-related functions in Mangle.

## Type Conversion Overview

Mangle has strict typing and requires explicit conversion between types. The main type families are:
- **Numbers** (`Type<int>`)
- **Floats** (`Type<float64>`)
- **Strings** (`Type<string>`)
- **Names/Atoms** (`Type<name>`)
- **Booleans** (represented as atoms: `/true`, `/false`)

---

## Number to String Conversion

### fn:number:to_string

**Signature:** `fn:number:to_string(Int) -> String`

**Description:** Converts an integer to its string representation.

**Parameters:**
- Single arg: `Type<int>`
- Returns `Type<string>`

**Examples:**

```mangle
# Basic conversion
X = fn:number:to_string(42).
# X = "42"

# Negative numbers
Y = fn:number:to_string(-100).
# Y = "-100"

# Zero
Z = fn:number:to_string(0).
# Z = "0"

# In string building
message(Msg) :-
  count(N),
  Msg = fn:string_concat("Count is ", fn:number:to_string(N)).

# Large numbers
Big = fn:number:to_string(1234567890).
# Big = "1234567890"
```

**Common Mistakes:**
- ❌ Using with float: `fn:number:to_string(3.14)` - type error, use `fn:float64:to_string`
- ❌ Expecting format control: no padding or formatting options

**Note:** `fn:string_concat` automatically calls this for numbers, so explicit conversion is often unnecessary:
```mangle
# These are equivalent:
X = fn:string_concat("Count: ", fn:number:to_string(42)).
Y = fn:string_concat("Count: ", 42).  # Auto-converts
```

---

## Float to String Conversion

### fn:float64:to_string

**Signature:** `fn:float64:to_string(Float64) -> String`

**Description:** Converts a float64 to its string representation.

**Parameters:**
- Single arg: `Type<float64>`
- Returns `Type<string>`

**Format:** Uses Go's default float formatting (precision varies)

**Examples:**

```mangle
# Basic conversion
X = fn:float64:to_string(3.14).
# X = "3.14"

# Negative float
Y = fn:float64:to_string(-2.718).
# Y = "-2.718"

# Integer-valued float
Z = fn:float64:to_string(5.0).
# Z = "5"

# Scientific notation (for very large/small)
Big = fn:float64:to_string(1234567890.123).
# Big might be "1.234567890123e+09" or "1234567890.123" depending on size

# In expressions
temp_message(Msg) :-
  temperature(T),
  Msg = fn:string_concat("Temperature: ", fn:float64:to_string(T), "°C").
```

**Common Mistakes:**
- ❌ Using with int: `fn:float64:to_string(42)` - type error, use `fn:number:to_string`
- ❌ Expecting precision control: format is automatic

**Note:** `fn:string_concat` auto-converts floats too:
```mangle
# Equivalent:
X = fn:string_concat("Pi is ", fn:float64:to_string(3.14159)).
Y = fn:string_concat("Pi is ", 3.14159).  # Auto-converts
```

---

## Name to String Conversion

### fn:name:to_string

**Signature:** `fn:name:to_string(Name) -> String`

**Description:** Converts a name/atom to its string representation (including the leading `/`).

**Parameters:**
- Single arg: `Type<name>`
- Returns `Type<string>` with leading `/`

**Examples:**

```mangle
# Basic conversion
X = fn:name:to_string(/hello).
# X = "/hello"

# Nested path
Y = fn:name:to_string(/path/to/resource).
# Y = "/path/to/resource"

# In string building
id_string(ID, Str) :-
  Str = fn:string_concat("ID: ", fn:name:to_string(ID)).

# Converting enum-like atoms
status_text(Status, Text) :-
  status(Status),
  Text = fn:name:to_string(Status).
# /active -> "/active"
```

**Important:** The result includes the leading `/` character that's part of the atom syntax.

**Common Mistakes:**
- ❌ Expecting string without `/`: result is `"/atom"` not `"atom"`
- ❌ Using with string: `fn:name:to_string("hello")` - type error

**Note:** `fn:string_concat` also auto-converts names:
```mangle
# Equivalent:
X = fn:string_concat("User: ", fn:name:to_string(/alice)).
Y = fn:string_concat("User: ", /alice).  # Auto-converts
```

---

## Name Manipulation Functions

### fn:name:root

**Signature:** `fn:name:root(Name) -> Name`

**Description:** Extracts the root (first component) of a hierarchical name.

**Parameters:**
- Single arg: `Type<name>`
- Returns `Type<name>` root component

**Examples:**

```mangle
# Extract root
X = fn:name:root(/foo/bar/baz).
# X = /foo

# Single component name (no slash) returns itself
Y = fn:name:root(/simple).
# Y = /simple

# Two-level name
Z = fn:name:root(/parent/child).
# Z = /parent

# Used in grouping
namespace(Name, Root) :-
  entity(Name),
  Root = fn:name:root(Name).
```

**Behavior:**
- Finds first `/` after initial `/`
- Returns everything up to (and including) that slash
- If no internal slash, returns the name itself

---

### fn:name:tip

**Signature:** `fn:name:tip(Name) -> Name`

**Description:** Extracts the tip (last component) of a hierarchical name.

**Parameters:**
- Single arg: `Type<name>`
- Returns `Type<name>` last component

**Examples:**

```mangle
# Extract tip
X = fn:name:tip(/foo/bar/baz).
# X = /baz

# Single component name returns itself
Y = fn:name:tip(/simple).
# Y = /simple

# Two-level name
Z = fn:name:tip(/parent/child).
# Z = /child

# Get filename from path-like name
filename(Path, Name) :-
  file(Path),
  Name = fn:name:tip(Path).
```

**Behavior:**
- Finds last `/`
- Returns everything after that slash
- If no slash (other than leading), returns the name itself

---

### fn:name:list

**Signature:** `fn:name:list(Name) -> List<Name>`

**Description:** Splits a hierarchical name into a list of components.

**Parameters:**
- Single arg: `Type<name>`
- Returns `Type<list<name>>` of components

**Examples:**

```mangle
# Split into components
X = fn:name:list(/foo/bar/baz).
# X = [/foo, /bar, /baz]

# Single component
Y = fn:name:list(/simple).
# Y = [/simple]

# Two components
Z = fn:name:list(/parent/child).
# Z = [/parent, /child]

# Process each component
component(C) :-
  path(/a/b/c/d),
  Components = fn:name:list(path),
  :list:member(C, Components).
# Generates: /a, /b, /c, /d

# Count depth
path_depth(Path, Depth) :-
  Components = fn:name:list(Path),
  Depth = fn:list:len(Components).
```

**Behavior:**
- Splits on `/` boundaries
- Each component includes its leading `/`
- Result is a list of names

---

## Boolean Type

Mangle doesn't have a primitive boolean type. Booleans are represented as atoms:

### Boolean Atoms
- **True:** `/true` (or `true` constant)
- **False:** `/false` (or `false` constant)

**Examples:**

```mangle
# Boolean values as atoms
flag(/true).
flag(/false).

# Comparison returns boolean constants
X = fn:list:contains(fn:list(1, 2, 3), 2).
# X = true (the constant)

Y = fn:list:contains(fn:list(1, 2, 3), 5).
# Y = false (the constant)

# Using with :filter predicate
valid(Item) :-
  check(Item, Result),
  :filter(Result).  # Succeeds if Result = true
```

---

## Option Type

Some functions return `Type<option<T>>` which can be:
- **Some(value):** contains a value
- **None:** no value

### fn:some

**Signature:** `fn:some(Value) -> Option<T>`

**Description:** Wraps a value in an option type.

**Examples:**

```mangle
# Create optional value
X = fn:some(42).
# X = some(42)

# Used with functions that return options
Result = fn:list:get(fn:list(1, 2, 3), 1).
# Result = 2 (automatically unwrapped if valid)
```

**Note:** Most Mangle code doesn't need explicit option handling. Functions like `fn:list:get` that return options will raise errors on "none" rather than returning an option value to handle.

---

## Type Checking Patterns

Mangle doesn't have built-in `typeof` or `instanceof` functions. Type checking is done through pattern matching and predicates.

### Check if List is Empty
```mangle
is_empty(L) :- :match_nil(L).
```

### Check Type by Attempting Operations
```mangle
# Check if value is a list by trying to get length
is_list(X) :- _ = fn:list:len(X).  # Succeeds if X is a list

# Check if value is a number by trying arithmetic
is_number(X) :- _ = fn:plus(X, 0).  # Succeeds if X is int
```

---

## Auto-Conversion Summary

`fn:string_concat` automatically converts these types:

| Input Type | Conversion Function | Example | Result |
|-----------|---------------------|---------|---------|
| `Type<int>` | `fn:number:to_string` | `fn:string_concat("N=", 42)` | `"N=42"` |
| `Type<float64>` | `fn:float64:to_string` | `fn:string_concat("F=", 3.14)` | `"F=3.14"` |
| `Type<name>` | `fn:name:to_string` | `fn:string_concat("A=", /foo)` | `"A=/foo"` |
| `Type<string>` | None (passthrough) | `fn:string_concat("S=", "hi")` | `"S=hi"` |

---

## Type Conversion Table

| From | To | Function | Example |
|------|-----|----------|---------|
| Int | String | `fn:number:to_string` | `fn:number:to_string(42)` → `"42"` |
| Float64 | String | `fn:float64:to_string` | `fn:float64:to_string(3.14)` → `"3.14"` |
| Name | String | `fn:name:to_string` | `fn:name:to_string(/foo)` → `"/foo"` |
| Name | List<Name> | `fn:name:list` | `fn:name:list(/a/b)` → `[/a, /b]` |
| Name | Name (root) | `fn:name:root` | `fn:name:root(/a/b)` → `/a` |
| Name | Name (tip) | `fn:name:tip` | `fn:name:tip(/a/b)` → `/b` |
| Value | Option<T> | `fn:some` | `fn:some(42)` → `some(42)` |

**No conversions exist for:**
- String → Int (use external predicates)
- String → Float (use external predicates)
- String → Name (names are compile-time literals)
- Int → Float (use float functions which auto-convert)
- Float → Int (no built-in truncation)

---

## Common Patterns

### Format Number with Label
```mangle
format_count(Label, Count, Output) :-
  Output = fn:string_concat(Label, ": ", fn:number:to_string(Count)).
```

### Build Path from Components
```mangle
# Reverse of fn:name:list - no built-in function
# Must build string and convert (but can't convert string to name at runtime)
# Names must be compile-time literals
```

### Extract Namespace
```mangle
get_namespace(FullPath, Namespace) :-
  Namespace = fn:name:root(FullPath).
```

### Get File Extension (if using name-style paths)
```mangle
# No built-in extension function for names
# Use :string:ends_with on string representation instead
has_extension(Path, Ext) :-
  PathStr = fn:name:to_string(Path),
  :string:ends_with(PathStr, Ext).
```

---

## Limitations

### No String to Number
```mangle
# ✗ No built-in string to int
# X = fn:string_to_number("42").  # Doesn't exist

# Must use external predicates or parse at program load time
```

### No Runtime Name Construction
```mangle
# ✗ Can't build names from strings at runtime
# Name = fn:string_to_name("/dynamic/path").  # Doesn't exist

# Names must be literals in source code
```

### No Float Formatting Control
```mangle
# ✗ Can't specify precision
# X = fn:float64:to_string(3.14159, 2).  # Not supported

# Use default formatting only
```

---

## Performance Notes

- String conversions allocate new strings
- Name manipulation is relatively fast (string operations)
- Auto-conversion in `fn:string_concat` has minimal overhead
- Prefer auto-conversion over explicit conversion when building strings

---

## Complete Example: Building a Report

```mangle
# Generate user report
user_report(UserID, Report) :-
  user_name(UserID, Name),
  user_score(UserID, Score),
  user_level(UserID, Level),

  # Build report string
  Report = fn:string_concat(
    "User ", Name,
    " (ID: ", UserID, ")",  # Auto-converts name to string
    " - Score: ", Score,     # Auto-converts int to string
    " - Level: ", Level      # Auto-converts atom to string
  ).

# Example facts:
user_name(/user123, "Alice").
user_score(/user123, 9500).
user_level(/user123, /expert).

# Result:
# user_report(/user123, "User Alice (ID: /user123) - Score: 9500 - Level: /expert").
```
