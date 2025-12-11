# Mangle String Functions and Predicates

Complete reference for string manipulation in Mangle.

## String Predicates (Pattern Matching)

These are **predicates** that succeed/fail based on string patterns.

---

### :string:starts_with

**Signature:** `:string:starts_with(String, Pattern)`

**Mode:** `(Input, Input)` - both arguments must be bound strings

**Description:** Succeeds if the first string starts with the second string.

**Parameters:**
- First arg: `Type<string>` to check
- Second arg: `Type<string>` prefix pattern

**Examples:**

```mangle
# Basic usage
:string:starts_with("hello world", "hello").     # Succeeds
:string:starts_with("hello world", "world").     # Fails

# Empty prefix always matches
:string:starts_with("anything", "").             # Succeeds

# In rule body
log_error(Line) :- log_line(Line), :string:starts_with(Line, "ERROR:").

# Case sensitive
:string:starts_with("Hello", "hello").           # Fails (case matters)

# With variables
has_prefix(Text, Prefix) :- :string:starts_with(Text, Prefix).
```

**Common Mistakes:**
- ❌ Confusing argument order - pattern is second, not first
- ❌ Using atoms instead of strings: `:string:starts_with(/atom, "test")` - type error
- ❌ Expecting case-insensitive matching - it's case-sensitive

---

### :string:ends_with

**Signature:** `:string:ends_with(String, Pattern)`

**Mode:** `(Input, Input)`

**Description:** Succeeds if the first string ends with the second string.

**Parameters:**
- First arg: `Type<string>` to check
- Second arg: `Type<string>` suffix pattern

**Examples:**

```mangle
# Basic usage
:string:ends_with("hello.txt", ".txt").          # Succeeds
:string:ends_with("hello.txt", ".pdf").          # Fails

# Empty suffix always matches
:string:ends_with("anything", "").               # Succeeds

# File extension filtering
text_file(Path) :- file(Path), :string:ends_with(Path, ".txt").

# Multi-extension check
source_file(Path) :-
  file(Path),
  (:string:ends_with(Path, ".go") ; :string:ends_with(Path, ".py")).
```

---

### :string:contains

**Signature:** `:string:contains(String, Substring)`

**Mode:** `(Input, Input)`

**Description:** Succeeds if the first string contains the second string anywhere within it.

**Parameters:**
- First arg: `Type<string>` to search in
- Second arg: `Type<string>` to search for

**Examples:**

```mangle
# Basic usage
:string:contains("hello world", "lo wo").        # Succeeds
:string:contains("hello world", "xyz").          # Fails

# Empty substring always matches
:string:contains("anything", "").                # Succeeds

# Search pattern
suspicious_log(Line) :-
  log_line(Line),
  :string:contains(Line, "suspicious").

# Multiple conditions
error_with_code(Line, Code) :-
  log_line(Line),
  :string:contains(Line, "ERROR"),
  :string:contains(Line, Code).

# Case sensitive
:string:contains("Hello World", "hello").        # Fails (case matters)
```

---

## String Functions (Constructive)

These **functions** build and transform strings.

---

### fn:string_concat (or fn:string:concatenate)

**Signature:** `fn:string_concat(Value, Value, ...) -> String`

**Description:** Concatenates multiple values into a single string. Automatically converts numbers and names to strings.

**Parameters:**
- Variable number of arguments
- Each can be `Type<string>`, `Type<int>`, `Type<float64>`, or `Type<name>`
- Returns `Type<string>`

**Type Conversion:**
- Strings: used as-is
- Numbers (`Type<int>`): converted via `fn:number:to_string`
- Floats (`Type<float64>`): converted via `fn:float64:to_string`
- Names (`Type<name>`): converted via `fn:name:to_string`

**Examples:**

```mangle
# Basic concatenation
X = fn:string_concat("hello", " ", "world").     # X = "hello world"

# Auto-conversion of numbers
Y = fn:string_concat("Count: ", 42).             # Y = "Count: 42"

# Auto-conversion of names
Z = fn:string_concat("ID is ", /user123).        # Z = "ID is /user123"

# Single argument
W = fn:string_concat("solo").                    # W = "solo"

# Building messages
message(Msg) :-
  user(Name),
  score(Points),
  Msg = fn:string_concat("User ", Name, " scored ", Points, " points").

# Empty strings
E = fn:string_concat("", "test", "").            # E = "test"

# Mixing types
M = fn:string_concat("Pi is about ", 3.14159).   # M = "Pi is about 3.14159"
```

**Common Mistakes:**
- ❌ Expecting string interpolation: `fn:string_concat("x=$x")` won't substitute
- ❌ Trying to concat lists directly - convert elements first
- ❌ No separator argument - must include spaces manually

---

### fn:string:replace

**Signature:** `fn:string:replace(String, Old, New, Count) -> String`

**Description:** Replaces occurrences of a substring with another substring.

**Parameters:**
- First arg: `Type<string>` - original string
- Second arg: `Type<string>` - substring to find
- Third arg: `Type<string>` - replacement string
- Fourth arg: `Type<int>` - maximum number of replacements
  - `-1` means replace all occurrences
  - `0` means replace none (returns original)
  - `n > 0` means replace first n occurrences

**Returns:** `Type<string>`

**Examples:**

```mangle
# Replace all occurrences
X = fn:string:replace("hello hello", "hello", "hi", -1).
# X = "hi hi"

# Replace first occurrence only
Y = fn:string:replace("foo foo foo", "foo", "bar", 1).
# Y = "bar foo foo"

# Replace first two occurrences
Z = fn:string:replace("aaa aaa aaa", "aaa", "bbb", 2).
# Z = "bbb bbb aaa"

# No replacement
W = fn:string:replace("test", "x", "y", 0).
# W = "test"

# Replace with empty (delete)
D = fn:string:replace("a-b-c", "-", "", -1).
# D = "abc"

# In a rule
sanitized(Clean) :-
  raw_input(Dirty),
  Clean = fn:string:replace(Dirty, "<", "&lt;", -1).
```

**Common Mistakes:**
- ❌ Forgetting the count argument - it's required
- ❌ Using 0 when you meant -1 (replace all)
- ❌ Case sensitivity - "Hello" won't match "hello"

---

## Name Prefix Matching (Special Case)

### :match_prefix

**Signature:** `:match_prefix(Name, Prefix)`

**Mode:** `(Input, Input)` - for names, not strings

**Description:** Succeeds if a name starts with the given prefix. Works on `Type<name>`, NOT strings.

**Parameters:**
- First arg: `Type<name>` to check
- Second arg: `Type<name>` prefix (must be shorter)

**Examples:**

```mangle
# Basic usage
:match_prefix(/foo/bar/baz, /foo/bar).           # Succeeds
:match_prefix(/foo/bar, /foo).                   # Succeeds
:match_prefix(/foo, /foo).                       # Fails (must be strict prefix)

# In rules - find all children of a namespace
child_of_namespace(Name) :- entity(Name), :match_prefix(Name, /namespace).

# Directory-like hierarchy
subpath(/foo/bar/file, /foo).                    # Succeeds
```

**Common Mistakes:**
- ❌ Using strings instead of names - use `:string:starts_with` for strings
- ❌ Expecting equal names to match - prefix must be strictly shorter

---

## String Type Conversion

See `type_functions.md` for full details on conversion functions:
- `fn:number:to_string(Int) -> String`
- `fn:float64:to_string(Float) -> String`
- `fn:name:to_string(Name) -> String`

---

## Common Patterns

### File Extension Checking
```mangle
is_source_file(Path) :-
  file(Path),
  (:string:ends_with(Path, ".go") ;
   :string:ends_with(Path, ".rs") ;
   :string:ends_with(Path, ".py")).
```

### Log Parsing
```mangle
error_log(Level, Message) :-
  log_line(Line),
  :string:starts_with(Line, "ERROR"),
  # Further parsing logic...
```

### String Building
```mangle
report_line(Line) :-
  metric(Name, Value),
  Line = fn:string_concat(Name, ": ", Value).
```

### Sanitization
```mangle
safe_text(Output) :-
  user_input(Input),
  T1 = fn:string:replace(Input, "<script>", "", -1),
  T2 = fn:string:replace(T1, "</script>", "", -1),
  Output = T2.
```

---

## Comparison Table

| Function/Predicate | Type | Purpose | Mode | Example |
|-------------------|------|---------|------|---------|
| `:string:starts_with` | Predicate | Check prefix | (I,I) | `:string:starts_with("hello", "he")` |
| `:string:ends_with` | Predicate | Check suffix | (I,I) | `:string:ends_with("file.txt", ".txt")` |
| `:string:contains` | Predicate | Check substring | (I,I) | `:string:contains("hello", "ell")` |
| `fn:string_concat` | Function | Build string | - | `fn:string_concat("a", "b")` |
| `fn:string:replace` | Function | Replace substring | - | `fn:string:replace("hi", "h", "b", -1)` |
| `:match_prefix` | Predicate | Name prefix | (I,I) | `:match_prefix(/foo/bar, /foo)` |

---

## Type Safety

### Strings vs Names vs Atoms

```mangle
# ✓ String operations on strings
:string:contains("hello", "ell").

# ✗ String operations on atoms
:string:contains(/atom, "test").                 # Type error

# ✗ String operations on names
:string:starts_with(/path/to/file, "/path").     # Type error

# ✓ Name operations on names
:match_prefix(/path/to/file, /path).
```

### Automatic Conversion in fn:string_concat

```mangle
# These all work due to auto-conversion
X = fn:string_concat("number: ", 42).            # int -> string
Y = fn:string_concat("float: ", 3.14).           # float -> string
Z = fn:string_concat("name: ", /foo).            # name -> string

# But predicates don't auto-convert
:string:contains("test", 42).                    # Type error
```

---

## Performance Notes

- String operations create new strings (immutable)
- `fn:string:replace` with count = -1 scans entire string
- Pattern predicates are fast substring searches
- Avoid building very large strings in tight loops
