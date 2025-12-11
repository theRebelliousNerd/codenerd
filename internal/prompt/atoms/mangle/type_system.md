# Mangle Type System

## Core Types

| Type | Literal | Example |
|------|---------|---------|
| `name` | `/atom` | `/active`, `/user_123` |
| `string` | `"text"` | `"Hello world"` |
| `int` | `123` | `42`, `-5` |
| `float` | `1.5` | `3.14`, `-0.5` |
| `bool` | n/a | Represented as atoms `/true`, `/false` |
| `list` | `[...]` | `[1, 2, 3]`, `[/a, /b]` |

## Type Annotations

### Declaration Syntax
```mangle
Decl predicate_name(
    Arg1.Type<type>,
    Arg2.Type<type>
).
```

### Examples
```mangle
Decl user(Id.Type<name>, Name.Type<string>).
Decl score(User.Type<name>, Points.Type<int>).
Decl rating(Item.Type<name>, Value.Type<float>).
```

## Atoms vs Strings (Critical)

**Atoms (`name` type):**
- Start with `/`
- Used for identifiers, enums, IDs
- Efficiently stored and compared
- Examples: `/active`, `/pending`, `/user_42`

**Strings (`string` type):**
- Quoted with `""`
- Used for human-readable text
- Variable length content
- Examples: `"Error message"`, `"John Doe"`

### They NEVER Unify
```mangle
# This will NEVER match if status stores atoms
check(X) :- status(X, "active").  # WRONG

# Correct version
check(X) :- status(X, /active).   # RIGHT
```

## Type Errors

### Integer vs Float
```mangle
# Schema expects float
Decl metric(Name.Type<name>, Value.Type<float>).

# WRONG - integer literal
metric(/cpu, 50).

# RIGHT - float literal
metric(/cpu, 50.0).
```

### List Type Mismatch
```mangle
# WRONG - list in scalar field
Decl single_value(Name.Type<name>, Val.Type<int>).
single_value(/x, [1, 2, 3]).  # Type error

# RIGHT - scalar value
single_value(/x, 1).
```

### Atom in String Field
```mangle
# Schema expects string
Decl message(Id.Type<name>, Text.Type<string>).

# WRONG - atom instead of string
message(/msg1, /hello).

# RIGHT - string literal
message(/msg1, "hello").
```

## Type Inference Rules

1. Variables inherit type from first binding predicate
2. Same variable must have consistent type across all uses
3. Built-in functions have fixed signatures
4. Aggregations have specific input/output types

## Common Type Fixes

| Error Pattern | Fix |
|--------------|-----|
| `"active"` in atom field | Change to `/active` |
| `/text` in string field | Change to `"text"` |
| `5` in float field | Change to `5.0` |
| `5.0` in int field | Change to `5` |
