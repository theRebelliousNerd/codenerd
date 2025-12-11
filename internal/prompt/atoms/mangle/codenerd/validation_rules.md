# Mangle Validation Rules for codeNERD

Critical validation rules to prevent the Top 30 common errors AI coding agents make when writing Mangle code. Based on C:\CodeProjects\codeNERD\internal\mangle\claude.md.

## Overview

These errors are categorized by the layer where the "Stochastic Gap" occurs:
1. **Syntactic Hallucinations** - SQL/Prolog/Soufflé syntax forced into Mangle
2. **Semantic Safety & Logic** - Datalog validity violations
3. **Data Types & Functions** - JSON/OOP accessor hallucinations
4. **Go Integration** - API boundary failures

---

## I. Syntactic Hallucinations (The "Soufflé/SQL" Bias)

AI models trained on SQL, Prolog, and Soufflé often force those syntaxes into Mangle.

### 1. Atom vs. String Confusion

**ERROR:**
```mangle
user_intent(/u1, "mutation", /fix, "auth.go", "").
```

**CORRECTION:**
```mangle
user_intent(/u1, /mutation, /fix, "auth.go", "").
```

**Rule**: Use `/atom` for enums/IDs. Mangle treats atoms and strings as disjoint types; they will never unify.

**Detection**: String where Type<n> expected, or atom where Type<string> expected.

---

### 2. Soufflé Declarations

**ERROR:**
```mangle
.decl edge(x:number, y:number).
```

**CORRECTION:**
```mangle
Decl edge(X.Type<int>, Y.Type<int>).
```

**Rule**:
- Use uppercase `Decl` (not `.decl`)
- Variables are UPPERCASE (not lowercase)
- Type syntax is `.Type<type>` (not `:type`)

---

### 3. Lowercase Variables

**ERROR:**
```mangle
ancestor(x, y) :- parent(x, y).
```

**CORRECTION:**
```mangle
ancestor(X, Y) :- parent(X, Y).
```

**Rule**: All variables MUST be UPPERCASE. Lowercase is reserved for predicate names.

---

### 4. Inline Aggregation (SQL Style)

**ERROR:**
```mangle
total(Sum) :- item(X), Sum = sum(X).
```

**CORRECTION:**
```mangle
total(Sum) :-
    item(X)
    |> do fn:group_by(),
       let Sum = fn:sum(X).
```

**Rule**: Use the pipe operator `|>` with `do fn:group_by()` for aggregation. No SQL-style inline aggregation.

---

### 5. Implicit Grouping

**ERROR:**
```mangle
# Assuming variables in head auto-trigger GROUP BY
count_by_lang(Lang, Count) :-
    file_topology(_, _, Lang, _, _),
    Count = fn:count().
```

**CORRECTION:**
```mangle
count_by_lang(Lang, Count) :-
    file_topology(_, _, Lang, _, _)
    |> do fn:group_by(Lang),
       let Count = fn:count().
```

**Rule**: Grouping is EXPLICIT via `do fn:group_by(...)` transform step. No implicit SQL-style GROUP BY.

---

### 6. Missing Periods

**ERROR:**
```mangle
file_topology("test.go", "hash", /go, 123, /true)
```

**CORRECTION:**
```mangle
file_topology("test.go", "hash", /go, 123, /true).
```

**Rule**: EVERY clause (fact or rule) MUST end with a period `.`

---

### 7. Comment Syntax

**ERROR:**
```mangle
// This is a comment
/* Multi-line comment */
```

**CORRECTION:**
```mangle
# This is a comment
# Multi-line comments use multiple # lines
```

**Rule**: Use `#` for comments (not `//` or `/* */`).

---

### 8. Assignment vs. Unification

**ERROR:**
```mangle
result(X) :- X := 5.
result(X) :- let X = 5.  # Outside transform block
```

**CORRECTION:**
```mangle
result(X) :- X = 5.  # Unification
# OR in transform block:
result(X) :-
    base(Y)
    |> do fn:group_by(),
       let X = fn:sum(Y).
```

**Rule**:
- Use `=` for unification inside rule body
- Use `let` ONLY within `|> do` transform blocks
- Never use `:=` (not valid Mangle syntax)

---

## II. Semantic Safety & Logic (The "Datalog" Gap)

Mangle requires strict logical validity that probabilistic models often miss.

### 9. Unsafe Head Variables

**ERROR:**
```mangle
result(X) :- other(Y).  # X is unbounded
```

**CORRECTION:**
```mangle
result(X) :- other(Y), related(X, Y).
```

**Rule**: Every variable in the rule HEAD must appear in a positive atom in the BODY.

---

### 10. Unsafe Negation

**ERROR:**
```mangle
safe(X) :- not distinct(X).  # X is unbound in negation
```

**CORRECTION:**
```mangle
safe(X) :- candidate(X), not distinct(X).
```

**Rule**: All variables in a negated atom MUST be bound by positive atoms first.

---

### 11. Stratification Cycles

**ERROR:**
```mangle
p(X) :- not q(X).
q(X) :- not p(X).
```

**CORRECTION:**
```mangle
# Restructure into strict layers (strata)
# No recursion through negation allowed
base_fact(X) :- input(X).
p(X) :- base_fact(X), not q(X).
q(X) :- other_base(X).
```

**Rule**: NEVER allow recursion that passes through negation. Ensure strict stratification.

---

### 12. Infinite Recursion (Counter Fallacy)

**ERROR:**
```mangle
count(N) :- count(M), N = fn:plus(M, 1).  # Unbounded generation
```

**CORRECTION:**
```mangle
count(N) :-
    base(M),
    N = fn:plus(M, 1),
    N < 100.  # Bounded domain
```

**Rule**: Always bound recursion with a limit or finite domain (e.g., `N < 100`).

---

### 13. Cartesian Product Explosion

**ERROR:**
```mangle
res(X) :- huge_table(X), X = /specific_id.  # Filter late
```

**CORRECTION:**
```mangle
res(X) :- X = /specific_id, huge_table(X).  # Filter early
```

**Rule**: Selectivity FIRST. Place most selective filters early in rule body.

---

### 14. Null Checking (Open World Bias)

**ERROR:**
```mangle
check(X) :- data(X), X != null.
```

**CORRECTION:**
```mangle
check(X) :- data(X).  # If fact exists, it's not null
```

**Rule**: Mangle follows Closed World Assumption. If a fact exists, it is NOT null. Missing facts are simply absent.

---

### 15. Duplicate Rule Definitions

**ERROR:**
```mangle
# Thinking second rule overwrites first
p(X) :- a(X).
p(X) :- b(X).  # ERROR: Assuming this replaces the first
```

**CORRECTION:**
```mangle
# Multiple rules create UNION (this is correct)
p(X) :- a(X).
p(X) :- b(X).
# p is true if a OR b is true
```

**Rule**: Multiple rules for same predicate create a UNION (logical OR). They do NOT overwrite.

---

### 16. Anonymous Variable Misuse

**ERROR:**
```mangle
result(_, Y) :- other(X, Y).  # _ used but X is available
```

**CORRECTION:**
```mangle
result(X, Y) :- other(X, Y).  # Use X explicitly
# OR if truly don't care:
side_effect() :- other(_, _).  # Correct use of _
```

**Rule**: Use `_` ONLY for values you truly don't care about. It never binds and can't be referenced.

---

## III. Data Types & Functions (The "JSON" Bias)

AI agents hallucinate object-oriented accessors for Mangle's structured data.

### 17. Map Dot Notation

**ERROR:**
```mangle
result(Val) :- map_data(Map), Val = Map.key.
result(Val) :- map_data(Map), Val = Map['key'].
```

**CORRECTION:**
```mangle
result(Val) :- map_data(Map), :match_entry(Map, /key, Val).
# OR for structs:
result(Val) :- struct_data(S), :match_field(S, /key, Val).
```

**Rule**: Use `:match_entry(Map, /key, Val)` or `:match_field(Struct, /key, Val)`. No dot notation.

---

### 18. List Indexing

**ERROR:**
```mangle
result(Head) :- list_data(List), Head = List[0].
```

**CORRECTION:**
```mangle
result(Head) :- list_data(List), :match_cons(List, Head, Tail).
# OR:
result(Head) :- list_data(List), fn:list:get(List, 0, Head).
```

**Rule**: Use `:match_cons(List, Head, Tail)` or `fn:list:get(List, Index, Value)`. No bracket indexing.

---

### 19. Type Mismatch (Int vs Float)

**ERROR:**
```mangle
Decl score(Value.Type<float>).
score(5).  # ERROR: 5 is int, not float
```

**CORRECTION:**
```mangle
Decl score(Value.Type<float>).
score(5.0).  # Use 5.0 for floats
```

**Rule**: Mangle is strictly typed. Use `5.0` for floats, `5` for ints.

---

### 20. String Interpolation

**ERROR:**
```mangle
message("Error: $Code").
```

**CORRECTION:**
```mangle
message(Msg) :-
    error_code(Code),
    fn:string_concat("Error: ", Code, Msg).
```

**Rule**: Use `fn:string_concat` or build list structures. NO string interpolation.

---

### 21. Hallucinated Functions

**ERROR:**
```mangle
result(Parts) :- input(Str), Parts = fn:split(Str, ",").
result(Date) :- fn:date().
result(Sub) :- input(Str), Sub = fn:substring(Str, 0, 5).
```

**CORRECTION:**
```mangle
# Verify function exists in builtin package
# Mangle's stdlib is MINIMAL - don't assume Python/JavaScript functions exist
```

**Rule**: Verify function existence in builtin package. Don't hallucinate functions from other languages.

**Available Functions**: `fn:plus`, `fn:minus`, `fn:times`, `fn:div`, `fn:mod`, `fn:string_concat`, `fn:count`, `fn:sum`, `fn:min`, `fn:max`, `fn:group_by`

---

### 22. Aggregation Safety

**ERROR:**
```mangle
total(Sum) :-
    item(X)
    |> do fn:group_by(UnboundVar),  # ERROR: UnboundVar not in rule body
       let Sum = fn:sum(X).
```

**CORRECTION:**
```mangle
total(Sum) :-
    item(X)
    |> do fn:group_by(),
       let Sum = fn:sum(X).
```

**Rule**: Grouping variables MUST be bound in the rule body before the pipe `|>`.

---

### 23. Struct Syntax

**ERROR:**
```mangle
config({"key": "value"}).  # JSON style
```

**CORRECTION:**
```mangle
config({ /key: "value" }).  # Note atom key and spacing
```

**Rule**: Use `{ /key: "value" }` syntax. Note the atom `/key` and spacing.

---

## IV. Go Integration & Architecture (The "API" Gap)

Boundary failures between Go and Mangle logic.

### 24. Fact Store Type Errors

**ERROR:**
```go
store.Add("pred", "arg")  # Wrong types
```

**CORRECTION:**
```go
engine.AddFact("pred", "arg")  # Use engine.AddFact with proper types
```

**Rule**: Must use proper engine API with `interface{}` args that match declared types.

---

### 25. Incorrect Engine Entry Point

**ERROR:**
```go
engine.Run()  # Hallucinated method
```

**CORRECTION:**
```go
engine.Query(ctx, query)
# OR
engine.AddFact(predicate, args...)
```

**Rule**: Use `Query()`, `AddFact()`, `AddFacts()`, `GetFacts()`. No `Run()` method exists.

---

### 26. Ignoring Imports

**ERROR:**
```go
// Generating Mangle code without checking imports
```

**CORRECTION:**
```go
import (
    "codenerd/internal/mangle"
    "github.com/google/mangle/ast"
)
```

**Rule**: Explicitly manage imports. codeNERD uses wrapped Mangle from `codenerd/internal/mangle`.

---

### 27. External Predicate Signature

**ERROR:**
```go
func myPredicate(args ...interface{}) (interface{}, error) {
    // Wrong signature
}
```

**CORRECTION:**
```go
func myPredicate(query engine.Query, cb func(engine.Fact)) error {
    // Correct signature for virtual predicates
    return nil
}
```

**Rule**: External predicates require `func(query engine.Query, cb func(engine.Fact)) error`.

---

### 28. Parsing vs. Execution

**ERROR:**
```go
engine.EvalProgram(rawString)  # Passing unparsed string
```

**CORRECTION:**
```go
unit, _ := parse.Unit(bytes.NewReader([]byte(schema)))
analyzed, _ := analysis.AnalyzeOneUnit(unit, nil)
// Then use analyzed program
```

**Rule**: Code must be parsed (`parse.Unit`) and analyzed (`analysis.AnalyzeOneUnit`) before evaluation.

---

### 29. Assuming IO Access

**ERROR:**
```mangle
file_content(Path, Content) :- read_file(Path, Content).
```

**CORRECTION:**
```go
// In Go:
content, _ := os.ReadFile(path)
engine.AddFact("file_content", path, string(content))
```

**Rule**: Mangle is PURE. IO must happen in Go before execution (loading facts) or via external predicates.

---

### 30. Package Hallucination (Slopsquatting)

**ERROR:**
```mangle
use /std/date.
use /std/json.
```

**CORRECTION:**
```mangle
# Verify imports exist in Mangle ecosystem
# Most "std" modules don't exist - Mangle has minimal stdlib
```

**Rule**: Verify imports. Mangle has a VERY small, specific ecosystem. Don't assume standard libraries.

---

## How to Avoid These Mistakes

### 1. Feed the Grammar

Provide the complete syntax reference in prompt context. Use skill atoms from `.claude/skills/mangle-programming/`.

### 2. Solver-in-the-Loop

Don't trust "Zero-Shot" code generation:
1. Generate code
2. Parse with `mangle/parse`
3. Feed errors back to LLM
4. Regenerate

### 3. Explicit Typing

Force AI to declare types (`Decl`) first. This forces decision between `/atoms` and `"strings"` early.

### 4. Review for Liveness

Manually audit recursive rules for termination conditions. Check for:
- Base cases
- Bounded domains
- No recursion through negation
- Selectivity ordering

---

## Validation Checklist

Before accepting Mangle code, verify:

- [ ] All variables are UPPERCASE
- [ ] All clauses end with `.`
- [ ] All predicates have `Decl` statements
- [ ] Variables in negation are bound elsewhere
- [ ] No recursion through negation
- [ ] Recursive rules have termination conditions
- [ ] Aggregations use `|> do fn:group_by()`
- [ ] Atoms use `/atom` syntax for enums
- [ ] Strings use `"string"` syntax for text
- [ ] No hallucinated functions
- [ ] No dot notation or bracket indexing
- [ ] Selectivity filters early in rules
- [ ] Types match declarations (int vs float, atom vs string)

---

## Quick Reference Card

```text
✓ CORRECT                           ✗ INCORRECT
----------------------------------  ----------------------------------
Decl pred(X.Type<int>).            .decl pred(x:int).
ancestor(X, Y) :- parent(X, Y).    ancestor(x, y) :- parent(x, y).
file_topology(..., /go, ...).      file_topology(..., "go", ...).
# Comment                          // Comment
X = 5                              X := 5
|> do fn:group_by()                implicit grouping
:match_entry(Map, /key, Val)       Val = Map.key
:match_cons(List, Head, Tail)      Head = List[0]
safe(X) :- data(X), not bad(X).    safe(X) :- not bad(X).
```

---

## See Also

- [schemas.md](schemas.md) - Correct predicate declarations
- [policy.md](policy.md) - Correct rule patterns
- [fact_patterns.md](fact_patterns.md) - How to create facts
- [query_patterns.md](query_patterns.md) - Query examples
- [integration_points.md](integration_points.md) - Go integration
