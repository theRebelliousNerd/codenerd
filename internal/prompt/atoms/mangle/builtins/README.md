# Mangle Built-in Functions and Predicates - Complete Reference

This directory contains comprehensive documentation for ALL Mangle built-in functions and predicates. Each file includes exact signatures, parameter types, return types, multiple examples, and common mistakes.

## Quick Reference Index

### Arithmetic Operations
**File:** [arithmetic.md](arithmetic.md)

**Integer Functions:**
- `fn:plus(Int, ...) -> Int` - Addition
- `fn:minus(Int, ...) -> Int` - Subtraction (unary negation with 1 arg)
- `fn:mult(Int, ...) -> Int` - Multiplication
- `fn:div(Int, ...) -> Int` - Integer division

**Float Functions:**
- `fn:float:plus(Float, ...) -> Float` - Float addition
- `fn:float:mult(Float, ...) -> Float` - Float multiplication
- `fn:float:div(Float, ...) -> Float` - Float division
- `fn:sqrt(Float|Int) -> Float` - Square root

---

### Comparison Predicates
**File:** [comparison.md](comparison.md)

- `:lt(Number, Number)` - Less than (<)
- `:le(Number, Number)` - Less than or equal (≤)
- `:gt(Number, Number)` - Greater than (>)
- `:ge(Number, Number)` - Greater than or equal (≥)

**All require both arguments to be bound (Input mode)**

---

### String Functions
**File:** [string_functions.md](string_functions.md)

**Predicates:**
- `:string:starts_with(String, Prefix)` - Check if string starts with prefix
- `:string:ends_with(String, Suffix)` - Check if string ends with suffix
- `:string:contains(String, Substring)` - Check if string contains substring

**Functions:**
- `fn:string_concat(Value, ...) -> String` - Concatenate values (auto-converts numbers/names)
- `fn:string:replace(String, Old, New, Count) -> String` - Replace substrings

**Name Predicates:**
- `:match_prefix(Name, Prefix)` - Check if name has prefix

---

### List Functions
**File:** [list_functions.md](list_functions.md)

**Construction:**
- `fn:list(Elem, ...) -> List` - Build list from elements
- `fn:cons(Elem, List) -> List` - Prepend element (O(1))
- `fn:append(List, Elem) -> List` - Append element (O(n))

**Deconstruction Predicates:**
- `:match_nil(List)` - Check if list is empty
- `:match_cons(List, Head, Tail)` - Deconstruct into head and tail

**Query Functions:**
- `fn:list:len(List) -> Int` - Get list length
- `fn:list:get(List, Index) -> Elem` - Get element at index (0-based)
- `fn:list:contains(List, Elem) -> Bool` - Check membership (returns boolean)

**Iteration Predicate:**
- `:list:member(Elem, List)` - Generate/check membership (non-deterministic)

---

### Aggregation Functions
**File:** [aggregation_functions.md](aggregation_functions.md)

**⚠️ CRITICAL: These use PascalCase (capital first letter)**

**Numeric Aggregations:**
- `fn:Count(Var) -> Int` - Count rows
- `fn:Sum(Var) -> Int` - Sum integers
- `fn:Min(Var) -> Int` - Minimum integer
- `fn:Max(Var) -> Int` - Maximum integer
- `fn:Avg(Var) -> Float64` - Average (returns float)

**Float Aggregations:**
- `fn:FloatSum(Var) -> Float64` - Sum floats
- `fn:FloatMin(Var) -> Float64` - Minimum float
- `fn:FloatMax(Var) -> Float64` - Maximum float

**Collection Aggregations:**
- `fn:Collect(Var, ...) -> List` - Collect values into list
- `fn:CollectDistinct(Var, ...) -> List` - Collect unique values
- `fn:CollectToMap(Key, Value) -> Map` - Build map from key-value pairs
- `fn:PickAny(Var) -> T` - Pick arbitrary value

**All aggregations must be used inside `|> do fn:group_by(...)` transform pipelines**

---

### Transform Pipeline
**File:** [transform_functions.md](transform_functions.md)

**Pipeline Syntax:**
```mangle
result(GroupVars, AggValues) :-
  predicates(...)
  |> do fn:group_by(GroupVars),
     let Agg1 = fn:AggFunction1(Var1),
     let Agg2 = fn:AggFunction2(Var2).
```

**Keywords:**
- `|>` - Pipe operator
- `do` - Start transform block
- `fn:group_by(Vars, ...)` - Group by variables (empty = aggregate all)
- `let Variable = Expression` - Bind aggregation result

---

### Struct and Map Functions
**File:** [struct_functions.md](struct_functions.md)

**Struct:**
- `fn:struct(Key1, Val1, ...) -> Struct` - Build struct
- `:match_field(Struct, Field, Value)` - Extract field
- `fn:struct:get(Struct, Field) -> Value` - Get field (function form)

**Map:**
- `fn:map(Key1, Val1, ...) -> Map` - Build map
- `:match_entry(Map, Key, Value)` - Extract entry
- `fn:map:get(Map, Key) -> Value` - Get value (function form)

**Pair:**
- `fn:pair(First, Second) -> Pair` - Build pair
- `:match_pair(Pair, First, Second)` - Deconstruct pair

**Tuple:**
- `fn:tuple(Elem, ...) -> Tuple` - Build N-element tuple (nested pairs)

---

### Type Conversion Functions
**File:** [type_functions.md](type_functions.md)

**To String:**
- `fn:number:to_string(Int) -> String` - Convert integer to string
- `fn:float64:to_string(Float) -> String` - Convert float to string
- `fn:name:to_string(Name) -> String` - Convert name to string (includes `/`)

**Name Manipulation:**
- `fn:name:root(Name) -> Name` - Extract root component
- `fn:name:tip(Name) -> Name` - Extract last component
- `fn:name:list(Name) -> List<Name>` - Split into component list

**Option:**
- `fn:some(Value) -> Option<T>` - Wrap in option type

**Note:** `fn:string_concat` auto-converts numbers and names, so explicit conversion often unnecessary.

---

### Special Predicates
**File:** [special_predicates.md](special_predicates.md)

- `:filter(Bool)` - Filter by boolean constant (true/false)
- `:list:member(Elem, List)` - Membership check/generation (Output or Input mode)
- `:within_distance(Num1, Num2, Distance)` - Check if `|Num1 - Num2| < Distance`

---

## Complete Built-in Function List (Alphabetical)

### Functions (return values)

| Function | Return Type | Category | File |
|----------|-------------|----------|------|
| `fn:append` | List | List | list_functions.md |
| `fn:Avg` | Float64 | Aggregation | aggregation_functions.md |
| `fn:Collect` | List | Aggregation | aggregation_functions.md |
| `fn:CollectDistinct` | List | Aggregation | aggregation_functions.md |
| `fn:CollectToMap` | Map | Aggregation | aggregation_functions.md |
| `fn:cons` | List | List | list_functions.md |
| `fn:Count` | Int | Aggregation | aggregation_functions.md |
| `fn:div` | Int | Arithmetic | arithmetic.md |
| `fn:float:div` | Float64 | Arithmetic | arithmetic.md |
| `fn:float:mult` | Float64 | Arithmetic | arithmetic.md |
| `fn:float:plus` | Float64 | Arithmetic | arithmetic.md |
| `fn:float64:to_string` | String | Type Conversion | type_functions.md |
| `fn:FloatMax` | Float64 | Aggregation | aggregation_functions.md |
| `fn:FloatMin` | Float64 | Aggregation | aggregation_functions.md |
| `fn:FloatSum` | Float64 | Aggregation | aggregation_functions.md |
| `fn:group_by` | (Special) | Transform | transform_functions.md |
| `fn:list` | List | List | list_functions.md |
| `fn:list:contains` | Bool | List | list_functions.md |
| `fn:list:get` | T | List | list_functions.md |
| `fn:list:len` | Int | List | list_functions.md |
| `fn:map` | Map | Struct/Map | struct_functions.md |
| `fn:map:get` | V | Struct/Map | struct_functions.md |
| `fn:Max` | Int | Aggregation | aggregation_functions.md |
| `fn:Min` | Int | Aggregation | aggregation_functions.md |
| `fn:minus` | Int | Arithmetic | arithmetic.md |
| `fn:mult` | Int | Arithmetic | arithmetic.md |
| `fn:name:list` | List<Name> | Type Conversion | type_functions.md |
| `fn:name:root` | Name | Type Conversion | type_functions.md |
| `fn:name:tip` | Name | Type Conversion | type_functions.md |
| `fn:name:to_string` | String | Type Conversion | type_functions.md |
| `fn:number:to_string` | String | Type Conversion | type_functions.md |
| `fn:pair` | Pair | Struct/Map | struct_functions.md |
| `fn:PickAny` | T | Aggregation | aggregation_functions.md |
| `fn:plus` | Int | Arithmetic | arithmetic.md |
| `fn:some` | Option<T> | Type Conversion | type_functions.md |
| `fn:sqrt` | Float64 | Arithmetic | arithmetic.md |
| `fn:string_concat` | String | String | string_functions.md |
| `fn:string:replace` | String | String | string_functions.md |
| `fn:struct` | Struct | Struct/Map | struct_functions.md |
| `fn:struct:get` | T | Struct/Map | struct_functions.md |
| `fn:Sum` | Int | Aggregation | aggregation_functions.md |
| `fn:tuple` | Tuple | Struct/Map | struct_functions.md |

### Predicates (succeed/fail)

| Predicate | Mode | Category | File |
|-----------|------|----------|------|
| `:filter` | (Input) | Special | special_predicates.md |
| `:ge` | (Input, Input) | Comparison | comparison.md |
| `:gt` | (Input, Input) | Comparison | comparison.md |
| `:le` | (Input, Input) | Comparison | comparison.md |
| `:list:member` | (Out/In, Input) | Special | special_predicates.md |
| `:lt` | (Input, Input) | Comparison | comparison.md |
| `:match_cons` | (Input, Output, Output) | List | list_functions.md |
| `:match_entry` | (Input, Input, Output) | Struct/Map | struct_functions.md |
| `:match_field` | (Input, Input, Output) | Struct/Map | struct_functions.md |
| `:match_nil` | (Input) | List | list_functions.md |
| `:match_pair` | (Input, Output, Output) | Struct/Map | struct_functions.md |
| `:match_prefix` | (Input, Input) | String | string_functions.md |
| `:string:contains` | (Input, Input) | String | string_functions.md |
| `:string:ends_with` | (Input, Input) | String | string_functions.md |
| `:string:starts_with` | (Input, Input) | String | string_functions.md |
| `:within_distance` | (Input, Input, Input) | Special | special_predicates.md |

---

## Common Pitfalls (Cross-Reference)

### Casing Errors
**WRONG:** `fn:sum`, `fn:count`, `fn:max`
**CORRECT:** `fn:Sum`, `fn:Count`, `fn:Max` (PascalCase for aggregations)

See: [aggregation_functions.md](aggregation_functions.md)

### Atom vs String Confusion
**WRONG:** Using `"active"` when schema requires `/active`
**CORRECT:** Use `/atom` for enums/IDs

See: All files, especially [string_functions.md](string_functions.md)

### Division by Zero
All division operations (`fn:div`, `fn:float:div`) raise runtime errors on division by zero.

See: [arithmetic.md](arithmetic.md)

### Unsafe Negation
Variables in negated atoms must be bound first.

**WRONG:** `safe(X) :- not distinct(X).`
**CORRECT:** `safe(X) :- candidate(X), not distinct(X).`

### Aggregation Outside Transform
**WRONG:** `result(Sum) :- value(X), Sum = fn:Sum(X).`
**CORRECT:** `result(Sum) :- value(X) |> do fn:group_by(), let Sum = fn:Sum(X).`

See: [aggregation_functions.md](aggregation_functions.md), [transform_functions.md](transform_functions.md)

---

## Usage Examples by Task

### Sum values by category
```mangle
category_total(Cat, Total) :-
  item(Cat, Amount)
  |> do fn:group_by(Cat), let Total = fn:Sum(Amount).
```
See: [aggregation_functions.md](aggregation_functions.md)

### Filter list membership
```mangle
whitelisted(Item) :-
  item(Item),
  whitelist(List),
  :list:member(Item, List).
```
See: [list_functions.md](list_functions.md), [special_predicates.md](special_predicates.md)

### String building
```mangle
report(Text) :-
  user(Name),
  score(Points),
  Text = fn:string_concat("User ", Name, " scored ", Points, " points").
```
See: [string_functions.md](string_functions.md)

### Extract struct field
```mangle
get_name(Data, Name) :-
  :match_field(Data, /name, Name).
```
See: [struct_functions.md](struct_functions.md)

---

## Performance Quick Reference

**Fast (O(1)):**
- Arithmetic: `fn:plus`, `fn:minus`, `fn:mult`, `fn:div`, `fn:sqrt`
- Comparisons: `:lt`, `:le`, `:gt`, `:ge`
- List cons: `fn:cons`
- Pattern matching: `:match_cons`, `:match_nil`, `:match_pair`
- Distance: `:within_distance`

**Linear (O(n)):**
- List operations: `fn:append`, `fn:list:len`, `fn:list:get`, `fn:list:contains`
- String operations: `:string:starts_with`, `:string:ends_with`, `fn:string_concat`
- Struct/Map access: `:match_field`, `:match_entry`
- Aggregations: all `fn:Count`, `fn:Sum`, etc.

**Quadratic or worse:**
- `:string:contains` - O(n*m) substring search
- Cartesian products with `:list:member`

---

## Type System Quick Reference

| Type | Example Literal | Constructor | File |
|------|----------------|-------------|------|
| Int | `42`, `-10` | N/A | arithmetic.md |
| Float64 | `3.14`, `-2.5` | N/A | arithmetic.md |
| String | `"hello"` | `fn:string_concat` | string_functions.md |
| Name | `/atom`, `/path/to/thing` | N/A (compile-time) | type_functions.md |
| List | N/A | `fn:list(...)` | list_functions.md |
| Struct | N/A | `fn:struct(...)` | struct_functions.md |
| Map | N/A | `fn:map(...)` | struct_functions.md |
| Pair | N/A | `fn:pair(...)` | struct_functions.md |
| Bool | `true`, `false` | N/A (constants) | special_predicates.md |

---

## Contributing Updates

When adding or updating built-in documentation:
1. Update the specific category file
2. Update this README index
3. Add to the alphabetical function/predicate tables
4. Include 2-3 examples minimum
5. Document common mistakes
6. Specify exact casing and signatures

---

## Source Reference

All built-in functions are implemented in the Google Mangle library:
- **Predicates:** `github.com/google/mangle/builtin/builtin.go`
- **Functions:** `github.com/google/mangle/functional/functional.go`
- **Version:** Based on Mangle v0.4.0
