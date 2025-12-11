# Mangle Builtin Functions: String Operations

## String Construction

### fn:string_concat

**Signature**: `fn:string_concat(Str1, Str2, ..., StrN) → String`

**Purpose**: Concatenates multiple strings into one.

**Examples**:
```mangle
# Join two strings
full_name(Full) :-
    first_name(First),
    last_name(Last),
    Full = fn:string_concat(First, " ", Last).

# Build message
message(M) :-
    name(N),
    M = fn:string_concat("Hello, ", N, "!").

# Multiple parts
path(P) :-
    dir("/home"),
    user("alice"),
    file("data.txt"),
    P = fn:string_concat(dir, "/", user, "/", file).
```

**Note**: Takes variable number of arguments (variadic).

---

### fn:string_replace

**Signature**: `fn:string_replace(String, Pattern, Replacement, Count) → String`

**Purpose**: Replaces occurrences of a pattern with a replacement string.

**Arguments**:
- `String` - The original string
- `Pattern` - What to find
- `Replacement` - What to replace it with
- `Count` - Maximum number of replacements (-1 for all)

**Examples**:
```mangle
# Replace all occurrences
sanitized(S) :-
    input(I),
    S = fn:string_replace(I, "bad_word", "***", -1).

# Replace first occurrence only
first_replace(S) :-
    text(T),
    S = fn:string_replace(T, "foo", "bar", 1).

# Remove substring (replace with empty string)
remove_spaces(S) :-
    input(I),
    S = fn:string_replace(I, " ", "", -1).
```

**Count Parameter**:
- `-1` - Replace all occurrences
- `0` - Replace none (returns original)
- `N > 0` - Replace first N occurrences

---

## Type Conversion to String

### fn:name_to_string

**Signature**: `fn:name_to_string(Name) → String`

**Purpose**: Converts a name (atom) to a string.

**Examples**:
```mangle
# Convert atom to string
name_string(S) :- S = fn:name_to_string(/john).  # "john"

# Process atom as string
atom_length(Len) :-
    atom(/person/alice),
    S = fn:name_to_string(atom),  # "person/alice"
    Len = fn:len(S).  # If len works on strings
```

**Note**: Names use slash notation like `/person/alice`, the string version removes the slashes.

---

### fn:number_to_string

**Signature**: `fn:number_to_string(Number) → String`

**Purpose**: Converts an integer to a string.

**Examples**:
```mangle
# Convert number to string
age_string(S) :- age(30), S = fn:number_to_string(30).  # "30"

# Build message with number
message(M) :-
    age(A),
    age_str(AS),
    AS = fn:number_to_string(A),
    M = fn:string_concat("Age: ", AS).
```

---

### fn:float64_to_string

**Signature**: `fn:float64_to_string(Float) → String`

**Purpose**: Converts a floating-point number to a string.

**Examples**:
```mangle
# Convert float to string
temp_string(S) :-
    temperature(98.6),
    S = fn:float64_to_string(98.6).  # "98.6"

# Format price
price_display(D) :-
    price(P),
    price_str(PS),
    PS = fn:float64_to_string(P),
    D = fn:string_concat("$", PS).
```

---

## Name (Atom) Path Operations

Names in Mangle can be hierarchical (e.g., `/person/alice/age`). These functions work with name paths:

### fn:name_root

**Signature**: `fn:name_root(Name) → Name`

**Purpose**: Extracts the hierarchical prefix (everything before the last component).

**Examples**:
```mangle
# Get parent path
parent_path(Parent) :-
    path(/person/alice/age),
    Parent = fn:name_root(path).  # /person/alice
```

---

### fn:name_tip

**Signature**: `fn:name_tip(Name) → Name`

**Purpose**: Gets the final segment of a hierarchical name.

**Examples**:
```mangle
# Get last component
last_segment(Tip) :-
    path(/person/alice/age),
    Tip = fn:name_tip(path).  # /age
```

---

### fn:name_list

**Signature**: `fn:name_list(Name) → List`

**Purpose**: Decomposes a name into a list of its path components.

**Examples**:
```mangle
# Split name into parts
components(Parts) :-
    path(/person/alice/age),
    Parts = fn:name_list(path).  # [/person, /alice, /age]
```

---

## String Predicates (Pattern Matching)

See `predicates.md` for these string-matching predicates:

- `:match_prefix(String, Prefix)` - Check if string starts with prefix
- `:starts_with(String, Prefix)` - Alternative name
- `:ends_with(String, Suffix)` - Check if string ends with suffix
- `:contains(String, Substring)` - Check if string contains substring

---

## String Limitations

Mangle does NOT have (as of documented version):

- **String interpolation** - No `"Hello, ${name}!"` syntax
  - Use `fn:string_concat` instead

- **Substring extraction** - No `fn:substring(Str, Start, End)`
  - Would need external predicate or different approach

- **String length** - No documented `fn:string_length`
  - May exist but not in main docs

- **Case conversion** - No `fn:to_upper` / `fn:to_lower`
  - Would need external implementation

- **String splitting** - No `fn:string_split(Str, Delimiter)`
  - Use pattern matching or external predicate

- **Regular expressions** - No regex support in core
  - May be available via external predicates

---

## Working with Strings

### Building Messages

```mangle
# Greeting message
greeting(Msg) :-
    user_name(Name),
    time_of_day(Time),
    Msg = fn:string_concat("Good ", Time, ", ", Name, "!").
```

### Sanitizing Input

```mangle
# Remove special characters
sanitize(Clean) :-
    input(Raw),
    Step1 = fn:string_replace(Raw, "<", "", -1),
    Step2 = fn:string_replace(Step1, ">", "", -1),
    Clean = fn:string_replace(Step2, "&", "", -1).
```

### Formatting Output

```mangle
# Format person info
person_display(Display) :-
    person(Name, Age, City),
    AgeStr = fn:number_to_string(Age),
    Display = fn:string_concat(Name, " (", AgeStr, ") from ", City).
```

---

## String Constants

**Syntax**: Use double quotes for string literals.

```mangle
message("Hello, world!").
name("Alice").
error("File not found").
```

**Escaping**: Standard escape sequences (likely):
- `\"` - Quote
- `\\` - Backslash
- `\n` - Newline
- `\t` - Tab

(Exact escape syntax not fully documented)

---

## Bytes vs Strings

Mangle distinguishes between:
- **Strings** (`/string` type) - Text data
- **Bytes** (`/bytes` type) - Binary data

The documented functions work with strings. Byte operations may require different functions or external predicates.

---

## String Functions Summary

| Function | Arguments | Return | Purpose |
|----------|-----------|--------|---------|
| `fn:string_concat` | (str, ...) | string | Concatenate strings |
| `fn:string_replace` | (str, pat, repl, count) | string | Replace pattern |
| `fn:name_to_string` | (name) | string | Convert name to string |
| `fn:number_to_string` | (number) | string | Convert int to string |
| `fn:float64_to_string` | (float) | string | Convert float to string |
| `fn:name_root` | (name) | name | Get parent path |
| `fn:name_tip` | (name) | name | Get last segment |
| `fn:name_list` | (name) | list | Split into components |

---

## Common Patterns

### Conditional Formatting

```mangle
# Different message based on age
age_message(Msg) :-
    person(Name, Age),
    Age >= 18,
    Msg = fn:string_concat(Name, " is an adult").

age_message(Msg) :-
    person(Name, Age),
    Age < 18,
    Msg = fn:string_concat(Name, " is a minor").
```

### Building Structured Text

```mangle
# CSV-like format
csv_line(Line) :-
    data(Name, Age, City),
    AgeStr = fn:number_to_string(Age),
    Line = fn:string_concat(Name, ",", AgeStr, ",", City).
```

### Path Manipulation

```mangle
# Build file path
file_path(Path) :-
    directory("/home/user"),
    filename("data.txt"),
    Path = fn:string_concat(directory, "/", filename).

# Change file extension
new_extension(NewPath) :-
    original("file.txt"),
    base(fn:string_replace(original, ".txt", "", 1)),
    NewPath = fn:string_concat(base, ".csv").
```
