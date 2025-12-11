# Failure Mode 8: Structured Data Access Errors

## Category
Data Types & Functions (JSON Bias)

## Severity
HIGH - Parse errors or runtime failures

## Error Pattern
Using dot notation, bracket syntax, or direct field access instead of Mangle's explicit `:match_field` and `:match_entry` functions.

## Wrong Code
```mangle
# WRONG - Python/JavaScript dot notation
bad(Name) :- record({/name: Name}).
bad(Name) :- record(R), R.name = Name.

# WRONG - SQL dot notation
bad(Name) :- person.name = Name.

# WRONG - Bracket syntax (Python/JavaScript)
bad(Val) :- config(C), Val = C[/key].
bad(Val) :- config(C), Val = C["key"].

# WRONG - Direct unification with partial struct
bad(Name) :- record(R), R = {/name: Name}.

# WRONG - Assuming property extraction
bad(X, Y) :- point({/x: X, /y: Y}).  # Looks correct but won't work

# WRONG - Using fn: prefix instead of : prefix
bad(Name) :- record(R), fn:match_field(R, /name, Name).

# WRONG - Treating maps like functions
bad(Val) :- get(Config, /key, Val).  # Not how it works
```

## Correct Code
```mangle
# CORRECT - Use :match_field for structs
good(Name) :-
    record(R),
    :match_field(R, /name, Name).

# CORRECT - Multiple field extraction
person_info(ID, Name, Age) :-
    person_record(R),
    :match_field(R, /id, ID),
    :match_field(R, /name, Name),
    :match_field(R, /age, Age).

# CORRECT - Use :match_entry for maps (same as match_field)
config_value(Val) :-
    server_config(Cfg),
    :match_entry(Cfg, /timeout, Val).

# CORRECT - Nested structure access
nested_value(Val) :-
    data(D),
    :match_field(D, /outer, Outer),
    :match_field(Outer, /inner, Val).

# CORRECT - Use fn:map_get for default values
timeout_value(Val) :-
    config(Cfg),
    Val = fn:map_get(Cfg, /timeout, 30).  # Default = 30

# CORRECT - Pattern matching in rule head (if schema allows)
Decl point(P.Type<{/x: int, /y: int}>).
distance_from_origin(Dist) :-
    point(P),
    :match_field(P, /x, X),
    :match_field(P, /y, Y),
    Dist = fn:plus(fn:mult(X, X), fn:mult(Y, Y)).

# CORRECT - List access with :match_cons
first_element(Head) :-
    my_list(L),
    :match_cons(L, Head, _Tail).
```

## Detection
- **Symptom**: Parse error mentioning "unexpected token '.'"
- **Symptom**: "Unknown function" error for bracket access
- **Pattern**: Dot notation appearing in rule body
- **Pattern**: Brackets used with variables/atoms
- **Test**: `grep -E '\.[a-z_]+' *.mg | grep -v '^#'` to find dot access
- **Test**: `grep -E '\[' *.mg` to find bracket syntax

## Prevention

### Map/Struct Access Functions
| Function | Purpose | Syntax | Returns |
|----------|---------|--------|---------|
| `:match_field` | Extract struct field | `:match_field(Struct, /key, Value)` | Unifies Value |
| `:match_entry` | Extract map entry (same as match_field) | `:match_entry(Map, /key, Value)` | Unifies Value |
| `fn:map_get` | Get with default | `Val = fn:map_get(Map, /key, Default)` | Value or default |
| `:match_cons` | List head/tail | `:match_cons(List, Head, Tail)` | Splits list |

**CRITICAL**: Use `:` prefix (not `fn:`) for `match_field`, `match_entry`, and `match_cons`!

### Struct Field Extraction Pattern
```mangle
# Template: Extract single field
result(Value) :-
    source_predicate(StructVar),
    :match_field(StructVar, /field_name, Value).

# Template: Extract multiple fields
result(V1, V2, V3) :-
    source(S),
    :match_field(S, /field1, V1),
    :match_field(S, /field2, V2),
    :match_field(S, /field3, V3).

# Template: Nested access
result(InnerValue) :-
    source(S),
    :match_field(S, /outer_field, Outer),
    :match_field(Outer, /inner_field, InnerValue).
```

## Correct Patterns Reference

### Pattern 1: Simple Field Access
```mangle
# Schema
Decl user(Data.Type<{/id: int, /name: string, /age: int}>).

# Extract single field
user_name(Name) :-
    user(U),
    :match_field(U, /name, Name).

# Extract multiple fields
user_info(ID, Name) :-
    user(U),
    :match_field(U, /id, ID),
    :match_field(U, /name, Name).
```

### Pattern 2: Conditional Field Access
```mangle
# Get field value, check condition
adult_user(Name) :-
    user(U),
    :match_field(U, /name, Name),
    :match_field(U, /age, Age),
    Age >= 18.

# Field-based filtering
premium_users(Name) :-
    user(U),
    :match_field(U, /name, Name),
    :match_field(U, /tier, Tier),
    Tier = /premium.
```

### Pattern 3: Nested Structures
```mangle
# Schema
Decl server(
    Config.Type<{
        /network: {/host: string, /port: int},
        /auth: {/enabled: n, /method: n}
    }>
).

# Access nested fields
server_host(Host) :-
    server(S),
    :match_field(S, /network, Network),
    :match_field(Network, /host, Host).

# Multiple nested accesses
server_auth_config(Method) :-
    server(S),
    :match_field(S, /auth, Auth),
    :match_field(Auth, /enabled, /true),
    :match_field(Auth, /method, Method).
```

### Pattern 4: Map with Default Values
```mangle
# Get config value with fallback
effective_timeout(Val) :-
    config(Cfg),
    Val = fn:map_get(Cfg, /timeout, 30).  # Default 30 if missing

# Chain defaults
effective_retries(Val) :-
    config(Cfg),
    # Try user config, fallback to system default
    Val = fn:map_get(Cfg, /retries, DefaultRetries),
    default_config(DefCfg),
    DefaultRetries = fn:map_get(DefCfg, /retries, 3).  # Ultimate default: 3
```

### Pattern 5: List Operations
```mangle
# Get first element
first(Head) :-
    my_list(L),
    :match_cons(L, Head, _).

# Get rest of list
rest(Tail) :-
    my_list(L),
    :match_cons(L, _, Tail).

# Recursive list processing
list_sum([], 0).
list_sum(L, Sum) :-
    :match_cons(L, Head, Tail),
    list_sum(Tail, TailSum),
    Sum = fn:plus(Head, TailSum).

# Check list length
list_length(L, Len) :-
    Len = fn:len(L).
```

## Complex Example: Configuration Processing

```mangle
# Schema
Decl app_config(
    Name.Type<n>,
    Config.Type<{
        /server: {/host: string, /port: int, /timeout: int},
        /database: {/url: string, /pool_size: int},
        /features: [n]
    }>
).

# WRONG - Dot notation
# bad_server_info(Name, Host, Port) :-
#     app_config(Name, Cfg),
#     Host = Cfg.server.host,        # PARSE ERROR
#     Port = Cfg.server.port.        # PARSE ERROR

# CORRECT - Explicit field access
server_info(Name, Host, Port) :-
    app_config(Name, Cfg),
    :match_field(Cfg, /server, Server),
    :match_field(Server, /host, Host),
    :match_field(Server, /port, Port).

# Get timeout with default
effective_timeout(Name, Timeout) :-
    app_config(Name, Cfg),
    :match_field(Cfg, /server, Server),
    Timeout = fn:map_get(Server, /timeout, 60).  # Default 60s

# Check if feature is enabled
has_feature(Name, Feature) :-
    app_config(Name, Cfg),
    :match_field(Cfg, /features, Features),
    :member(Feature, Features).  # Built-in list membership

# Extract database pool size
db_pool_size(Name, Size) :-
    app_config(Name, Cfg),
    :match_field(Cfg, /database, DB),
    :match_field(DB, /pool_size, Size).
```

## Why These Functions Exist

### Type Safety
Unlike dynamic languages, Mangle enforces explicit field access to:
1. Catch typos at compile time
2. Make data flow explicit
3. Support static analysis
4. Enable optimization

### Unification Semantics
```mangle
# This doesn't work:
# point(P), P = {/x: X, /y: Y}.
# Why? P is already bound to a struct, can't reunify

# This works:
point(P),
:match_field(P, /x, X),
:match_field(P, /y, Y).
# Why? match_field extracts, doesn't reunify
```

## Training Bias Origins
| Language | Syntax | Leads to Wrong Mangle |
|----------|--------|----------------------|
| JavaScript | `obj.field` or `obj['field']` | Dot/bracket notation |
| Python | `dict['key']` or `obj.attr` | Bracket/dot access |
| JSON | `{"key": "value"}` | Assuming direct access |
| SQL | `table.column` | Dot notation |

## Quick Check
Before accessing structured data:
1. Is it a struct/map? → Use `:match_field` or `:match_entry`
2. Need default value? → Use `fn:map_get(Map, /key, Default)`
3. Is it a list? → Use `:match_cons` for head/tail
4. Multiple fields? → Chain multiple `:match_field` calls
5. Nested structure? → Chain field accesses

## Common Mistakes Summary

| Wrong | Correct | Reason |
|-------|---------|--------|
| `R.name` | `:match_field(R, /name, N)` | No dot notation |
| `R[/key]` | `:match_field(R, /key, V)` | No bracket syntax |
| `fn:match_field(...)` | `:match_field(...)` | Use `:` not `fn:` |
| `R = {/x: X}` | `:match_field(R, /x, X)` | Can't reunify bound struct |
| `get(M, /k)` | `:match_field(M, /k, V)` | match_field, not get |

## Debugging Aid
```mangle
# To debug struct contents, create extraction rules:
Decl debug_fields(Source.Type<n>, Fields.Type<[n]>).

# List all fields in a struct (if known at compile time)
debug_user_fields([/id, /name, /age, /email]).

# Extract all fields for inspection
debug_user(ID, Name, Age, Email) :-
    user(U),
    :match_field(U, /id, ID),
    :match_field(U, /name, Name),
    :match_field(U, /age, Age),
    :match_field(U, /email, Email).

# Query: ?debug_user(ID, Name, Age, Email)
# Shows all field values for debugging
```
