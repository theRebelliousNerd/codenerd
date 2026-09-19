# 200: Complete Syntax Reference

**Purpose**: Definitive language specification. Every construct, operator, function, and rule.

## Quick Lookup Table

| Construct | Syntax | Example | Section |
|-----------|--------|---------|---------|
| **Facts** | `pred(args).` | `parent(/a, /b).` | 2.1 |
| **Rules** | `head :- body.` | `sib(X,Y) :- parent(P,X), parent(P,Y).` | 2.2 |
| **Queries** | `?pred(args)` | `?sibling(X, Y)` | 2.3 |
| **Names** | `/identifier` | `/oedipus`, `/critical` | 3.1 |
| **Variables** | `UPPERCASE` | `X`, `Person`, `Count` | 3.2 |
| **Numbers** | `-?\d+(\.\d+)?` | `42`, `-17`, `3.14` | 3.3 |
| **Strings** | `"text"` | `"log4j"`, `"CVE-2021"` | 3.4 |
| **Lists** | `[T, ...]` | `[1, 2, 3]`, `[/a]` | 3.5 |
| **Maps** | `{/k: v}` | `{/name: "Alice"}` | 3.6 |
| **Negation** | `!atom` | `!excluded(X)` | 4.3 |
| **Comparison** | `X op Y` | `X != Y`, `X > 100` | 4.5 |
| **Transform** | `... \|> ...` | `data \|> fn:count()` | 5 |
| **Grouping** | `fn:group_by(V)` | `fn:group_by(Cat)` | 5.2 |
| **Aggregation** | `fn:sum(V)` | `let Total = fn:sum(Value)` | 5.3 |

---

## 1. Program Structure

### 1.1 Overall Organization

```mangle
# 1. Schema declarations (required: every predicate needs a Decl)
Decl base(X) bound [/name].
Decl condition(X) bound [/name].
Decl derived(X) bound [/name].

# 2. Base facts (EDB - Extensional Database)
base(/a).
condition(/a).

# 3. Rules (IDB - Intensional Database)
derived(X) :- base(X), condition(X).

# 4. Queries are not program text: `?derived(X)` is the interactive interpreter's
#    syntax. From codeNERD: nerd check-mangle --standalone --eval derived file.mg
```

### 1.2 File Format

**Extension**: `.mg` or `.mangle`
**Encoding**: UTF-8
**Line endings**: Any (Unix/Windows/Mac)
**Whitespace**: Ignored except in strings

---

## 2. Core Constructs

### 2.1 Facts (Ground Atoms)

**Syntax**: `predicate_symbol(term1, term2, ..., termN).`

**Examples**:
```mangle
parent(/oedipus, /antigone).
vulnerable("log4j", "2.14.0", "CVE-2021-44228").
contains_jar(/project1, /jar_name, "1.0.0").
age(/alice, 30).
```

**Rules**:
- Must end with period `.`
- All terms must be ground (no variables)
- Predicate symbol: lowercase start
- Arity (number of arguments) is part of identity

### 2.2 Rules (Horn Clauses)

**Syntax**: `head :- body1, body2, ..., bodyN.`

**Synonyms**: `:−` or `⟸` (Unicode alternative to `:-`)

**Components**:
```mangle
# head :- body_atom1, body_atom2, ...
Decl parent(P, X).
sibling(X, Y) :- parent(P, X), parent(P, Y), X != Y.
#  ↑              ↑              ↑              ↑
# head         body atom 1    body atom 2   body atom 3
```

**Semantics**: "Head is true if all body atoms are true"

**Multi-line rules allowed**:
```mangle
Decl project(Project).
Decl depends_on(Project, Lib, Version).
Decl cve_affects(Lib, Version, CVE, Severity).
vulnerable_project(Project, CVE, Severity) :- 
    project(Project),
    depends_on(Project, Lib, Version),
    cve_affects(Lib, Version, CVE, Severity).
```

**Multiple rules for same predicate** (union semantics):
```mangle
Decl source1(X).
Decl source2(X).
Decl source3(X).
result(X) :- source1(X).
result(X) :- source2(X).
result(X) :- source3(X).
# result = source1 ∪ source2 ∪ source3
```

### 2.3 Queries

**Interactive REPL only** (not in `.mg` files)

**Syntax**: `?predicate(pattern)`, in the interactive interpreter only: a `.mg` file cannot hold
queries. From codeNERD's CLI use `nerd query <predicate>` (the live kernel) or
`nerd check-mangle --eval <predicate> file.mg` (a file).

**Examples**:
```text
# Variable pattern
?sibling(X, Y)              # Find all siblings

# Constant pattern
?parent(/oedipus, /antigone) # Check if true

# Mixed pattern
?parent(/oedipus, X)         # Find oedipus's children
```

**REPL returns**:
- All matching facts
- Variable bindings
- Or "No results" if none match

---

## 3. Data Types

### 3.1 Names (Atoms)

**Syntax**: `/identifier`

**Identifier rules**:
- Start with letter or underscore
- `/` then letters, digits, `.`, `-`, `_`, `~` or `%`; more `/`-separated segments are allowed
- Because `.` is a name character, a name right before a clause's period absorbs it
  (`X = /done.` never ends the clause): put a space or a line break before the period

**Examples**:
```mangle
Decl name_example(N) bound [/name].
name_example(/oedipus).
name_example(/critical_severity).
name_example(/production_2024).
name_example(/web_app).
name_example(/us-east-1).
name_example(/codenerd/policy/jit).   # segments separated by '/'
```

**Case-sensitive**: `/Alice` ≠ `/alice`

### 3.2 Variables

**Syntax**: an uppercase letter, then letters and digits only. No `_` inside a name
(`P_Loser` does not lex); `_` alone is the wildcard.

**Examples**:
```text
X
Person
ProjectName
Count
TotalValue
_           # the wildcard: a fresh variable for each occurrence
```

**Scoping**:
- Variables scoped to single rule
- Same variable in different rules is DIFFERENT variable

**Anonymous variable**: `_` (matches anything, not bound)
```mangle
# Ignore middle argument
Decl triple(X, Y, Z).
first_and_last(A, C) :- triple(A, _, C).
```

### 3.3 Numbers

**Integers**:
```mangle
Decl number_example(N) bound [/number].
number_example(42).
number_example(-17).
number_example(0).
number_example(1000000).
```
**Range**: 64-bit signed integer

**Floats**:
```mangle
Decl measurement(Name, Value) bound [/name, /float64].
measurement(/pi, 3.14).
measurement(/offset, -2.5).
measurement(/big, 1.0e6).
measurement(/tiny, -3.7e-10).
```
Floats cannot be compared: `<`, `<=`, `>`, `>=` are integer-only and abort evaluation on a
float (`value 0.9 (4) is not a number`). Keep anything you compare as an integer (codeNERD scores
are 0..100).
**Format**: 64-bit IEEE 754

### 3.4 Strings

**Syntax**: `"characters"`

**Escaping**:
```mangle
Decl string_example(S) bound [/string].
string_example("normal string").
string_example("with \"quotes\"").
string_example("with\nnewline").
string_example("with\ttab").
string_example("with\\backslash").
```

**Supported escapes**: `\"`, `\n`, `\t`, `\\`

### 3.5 Lists

**Syntax**: `[elem1, elem2, ..., elemN]`

**Examples**:
```mangle
Decl list_example(L) bound [/any].
list_example([1, 2, 3]).
list_example([/a, /b, /c]).
list_example(["one", "two"]).
list_example([]).                  # Empty list
list_example([[1, 2], [3, 4]]).    # Nested
```

**Homogeneous preferred** (all same type)

**List construction**:
```mangle
Decl edge(From, To) bound [/name, /name].
Decl path(From, To, Nodes) bound [/name, /name, fn:List(/name)].

# A list literal in the head
path(Start, End, [Start, End]) :- edge(Start, End).

# Prepend with fn:list:cons; there is no [Head|Tail] syntax (a parse error).
# Terminates only on an acyclic graph: every longer path is a new fact.
path(Start, End, Nodes) :-
    edge(Start, Mid),
    path(Mid, End, Rest),
    Nodes = fn:list:cons(Start, Rest).
```

### 3.6 Maps & Structs

**Syntax**: structs `{/key1: value1, /key2: value2}`; maps `[key1: value1, key2: value2]`

**Structs and maps**:
```mangle
Decl person_record(R) bound [fn:Struct(/name, /string, /age, /number)].
Decl setting(M) bound [fn:Map(/string, /string)].
person_record({/name: "Alice", /age: 30}).   # struct: {/field: value}
setting(["host": "x", "port": "1"]).          # map: [key: value]
```

**Structs** (fixed schema, semantically equivalent to maps):
```mangle
person_record(1, {/name: "Alice", /age: 30, /city: "NYC"}).
```

**Access**:
```mangle
# Via :match_field
Decl person_record(ID, Info).
record_name(ID, Name) :- 
    person_record(ID, Info),
    :match_field(Info, /name, Name).

# Via :match_entry (same semantics)
record_name(ID, Name) :- 
    person_record(ID, Info),
    :match_entry(Info, /name, Name).
```

---

## 4. Operators

### 4.1 Rule Implication

**Operators**: `:−` or `⟸` (Unicode)

**Associativity**: Right

**Meaning**: "If body, then head"

### 4.2 Conjunction (AND)

**Operator**: `,`

**Associativity**: Left

**Meaning**: ALL atoms must be true

```mangle
# X and Y and Z all must hold
Decl cond1(A).
Decl cond2(B).
Decl cond3(C).
rule(A, B, C) :- cond1(A), cond2(B), cond3(C).
```

### 4.3 Negation

**Operator**: `not`

**Position**: Prefix (before atom)

**Example**:
```mangle
Decl candidate(X).
Decl excluded(X).
Decl safe(X).
safe(X) :- candidate(X), !excluded(X).
```

**Safety constraint**: every variable in a negated atom must be bound by a positive atom in the
same body (the engine moves the negation after the atoms that bind it).

```mangle
Decl item(X).
Decl excluded(X).
Decl foo(X).
Decl safe(X).
Decl bad(X).

# ✅ SAFE
safe(X) :- item(X), !excluded(X).  # X bound by item

# ❌ UNSAFE (analysis: "variable X is not bound")
bad(X) :- !foo(X).  # X not bound
```

**Stratification required** (see 100-FUNDAMENTALS.md)

### 4.4 Unification

**Operator**: `=`

**Meaning**: Make two terms equal (bind variables)

```mangle
Decl condition(Y).
Decl data(D).
Decl rule(X).
Decl pair_of(X, Y).

# Bind X to 42
rule(X) :- condition(Y), X = 42.

# Read struct fields with :match_field. (A struct literal with variables in a
# premise, `data({/a: X, /b: Y})`, loads but does not destructure: evaluation
# fails with "produced something that is not a value". And one predicate has one
# arity, so this is a second predicate, not rule/2.)
pair_of(X, Y) :- data(D), :match_field(D, /a, X), :match_field(D, /b, Y).
```

### 4.5 Comparison

**Operators**:
| Operator | Meaning | Types |
|----------|---------|-------|
| `=` | Unification | All |
| `!=` | Inequality | All |
| `<` | Less than | Numeric |
| `<=` | Less or equal | Numeric |
| `>` | Greater than | Numeric |
| `>=` | Greater or equal | Numeric |

**Examples**:
```mangle
Decl person(X).
Decl age(X, A).
Decl item(X).
Decl size(X, S).
adult(X) :- person(X), age(X, A), A >= 18.
different(X, Y) :- item(X), item(Y), X != Y.
large(X) :- size(X, S), S > 1000.
```

**Type restrictions**:
- Numeric operators: integers and floats only
- Equality/inequality: any types

### 4.6 Pipeline

**Operator**: `|>`

**Purpose**: Chain transforms (see Section 5)

**Position**: Infix

```mangle
Decl sale(Category, Value) bound [/name, /number].
Decl category_total(Category, Total) bound [/name, /number].
category_total(Cat, Total) :-
    sale(Cat, Value)
    |> do fn:group_by(Cat), let Total = fn:sum(Value).
```

---

## 5. Transforms & Aggregation

### 5.1 Transform Syntax

**General form**:
```text
head(GroupVars, R1, R3) :-
    premise1(...), premise2(...), Comparison
    |> do fn:group_by(GroupVars), let R1 = fn:count(), let R2 = fn:sum(V), let R3 = fn:div(R2, R1).
```
One stage is one `do` followed by `let`s, and a `let` may use the lets before it in the same stage.
`do A, do B` in one stage is a parse error, and a second `|>` stage of only `let`s does not bind
its variables (`variable Avg is not bound`): compute the value in the same stage, or in a second
rule over the aggregate's result.

**Keywords**:
- `|>` - Pipeline operator
- `do` - Apply transform function
- `let` - Bind aggregation result

### 5.2 Grouping

**Function**: `fn:group_by(Var1, Var2, ...)`

**Purpose**: Partition facts by grouping variables

```mangle
# Group by single variable
Decl item(X, Y).
Decl sale(Region, Product, Amount).
count_per_category(Cat, N) :-
    item(Cat, _) |>
    do fn:group_by(Cat),
    let N = fn:count().

# Group by multiple variables
stats(Region, Product, Count) :-
    sale(Region, Product, Amount) |>
    do fn:group_by(Region, Product),
    let Count = fn:count().
```

### 5.3 Aggregation Functions

**Count**:
```text
fn:count()  # Count elements in group
```

**Sum**:
```text
fn:sum(Variable)  # Sum numeric values
```

**Min/Max**:
```text
fn:min(Variable)  # Minimum value
fn:max(Variable)  # Maximum value
```

**Example - all aggregations**:
```mangle
Decl item(Cat, Value).
category_stats(Cat, Count, Total, Avg, Min, Max) :-
    item(Cat, Value) |>
    do fn:group_by(Cat),
    let Count = fn:count(),
    let Total = fn:sum(Value),
    let Min = fn:min(Value),
    let Max = fn:max(Value),
    let Avg = fn:div(Total, Count).
```

### 5.4 Arithmetic Functions

**Basic operations**:
```text
fn:plus(A, B)      # A + B (variadic: fn:plus(A, B, C, ...))
fn:minus(A, B)     # A - B (variadic: (A - B) - C ...; fn:minus(A) is -A)
fn:mult(A, B)      # A × B (variadic)
fn:div(A, B)       # A / B integer division (variadic)
fn:sqrt(A)         # square root: /float64 in, /float64 out
```
They are used in a rule body: `Total = fn:plus(A, B)`.

**Float variants** (for float64 precision):
```text
fn:float:plus(A, B)   # Float addition
fn:float:mult(A, B)   # Float multiplication
fn:float:div(A, B)    # Float division
```

**Usage in transforms**:
```mangle
# Calculate average
Decl item(Cat, Value).
average(Cat, Avg) :-
    item(Cat, Value) |>
    do fn:group_by(Cat),
    let Total = fn:sum(Value),
    let Count = fn:count(),
    let Avg = fn:div(Total, Count).
```

### 5.5 Comparison Predicates

**Predicates** (use in rule body, NOT as functions):
```text
:lt(A, B)   # A < B
:le(A, B)   # A ≤ B
:gt(A, B)   # A > B
:ge(A, B)   # A ≥ B
```

**Or use infix operators directly**:
```text
A < B       # Less than
A <= B      # Less or equal
A > B       # Greater than
A >= B      # Greater or equal
A = B       # Unification (equality)
A != B      # Inequality
# < <= > >= are integer-only: a float aborts evaluation ("is not a number")
```

**Usage**:
```mangle
# Filter with comparison
Decl item(Cat, Value).
high_values(Cat, N) :-
    item(Cat, Value),
    Value > 1000 |>
    do fn:group_by(Cat),
    let N = fn:count().
```

### 5.6 Data Structure Functions and Predicates

**Struct/Map access** (predicates):
```text
:match_field(Struct, /field_name, Value)   # Extract struct field
:match_entry(Map, /key, Value)             # Extract map entry
```

**List functions**:
```text
fn:list(A, B, C)              # Construct list [A, B, C]
fn:list:cons(Head, Tail)      # Prepend Head (there is no [Head|Tail] syntax)
fn:list:append(List, Elem)    # Append ONE element: appending [3] to [1, 2] gives [1, 2, [3]]
fn:list:get(List, Index)      # Get element at index (0-based)
fn:list:len(List)             # Get list length
fn:list:contains(List, Elem)  # /true if Elem is in List, else /false
```

**List predicates**:
```text
:match_cons(List, Head, Tail)  # Destructure a bound list into head and tail
:match_nil(List)               # Match the empty list (List must already be bound)
:list:member(Elem, List)       # Bind Elem to each element
```

**String functions**:
```text
fn:string:concat(S1, S2, ...)  # Concatenate strings
fn:string:replace(Str, Old, New, N)  # Replace first N occurrences (exactly 4 arguments)
```

**String predicates** (NOT functions!):
```text
:string:contains(Str, Sub)     # True if Str contains Sub
:string:starts_with(Str, Pre)  # True if Str starts with Pre
:string:ends_with(Str, Suf)    # True if Str ends with Suf
```

---

## 6. Type System

### 6.1 Type Declarations

**Syntax**: `Decl predicate(Arg1, Arg2) bound [/type1, /type2].`

**Example**:
```mangle
Decl employee(ID, Name, Dept) bound [/number, /string, /name].
Decl salary(ID, Amount) bound [/number, /float64].
```

**Type syntax**:
```text
/number              # 64-bit integer
/float64             # 64-bit float
/string              # UTF-8 string
/name                # name constant, e.g. /active
/bytes               # byte string, b"..."
/any                 # anything
fn:List(T)  fn:Map(K, V)  fn:Pair(A, B)  fn:Struct(/field, T, ...)  fn:Union(T1, T2)  fn:Singleton(/n)
# One bound per argument, exactly; omit the whole bound block to accept anything.
# The table with literals for each: 600-TYPE_SYSTEM.md
```

### 6.2 Gradual Typing

**Optional**: omit the whole `bound` block, or give a column you do not constrain `/any`
(the number of bounds must equal the number of arguments)
```mangle
# Decls are required; a column you do not constrain is /any
Decl employee(ID, Name, Dept) bound [/number, /any, /any].
employee(1, "Alice", /engineering).
```

**Type checking**: facts are NOT typechecked against bounds at load; a
fact whose arg shape disagrees with a rule simply never unifies.
**Type inference**: none. The Go engine applies heuristics (see
internal/mangle/engine.go convertValueToTypedTerm), not inference.

### 6.3 Structured Types

**List types**:
```mangle
Decl tags(ID, Tags) bound [/number, fn:List(/string)].

tags(1, ["critical", "urgent"]).
```

**Map types**:
```mangle
Decl config(Data).

config({/host: "localhost", /port: 8080}).
```

**Union types**:
```mangle
Decl flexible(Value).

flexible(42).
flexible("text").
```

---

## 7. Safety Constraints

### 7.1 Variable Safety

**Rule**: Every variable in rule head must appear in:
1. A positive body atom, OR
2. A unification `Var = constant`

**Examples**:
```mangle
Decl foo(X).
Decl bar(Y).
Decl rule(X, Y).

# ✅ SAFE
rule(X, Y) :- foo(X), bar(Y).           # Both X, Y bound
rule(X, Y) :- foo(X), Y = 42.            # X bound by atom, Y by unification

# ❌ UNSAFE (analysis: "variable Y is not bound")
rule(X, Y) :- foo(X).                    # Y unbound
```

### 7.2 Negation Safety

**Rule**: every variable in a negated atom must be bound by a positive atom in the same body.
Position does not matter: the engine moves a negation after the atoms that bind it.

**Examples**:
```mangle
Decl candidate(X).
Decl excluded(X).
Decl foo(X).
Decl safe(X).
Decl unsafe(X).

# ✅ SAFE
safe(X) :- candidate(X), !excluded(X).  # X bound by candidate

# ❌ UNSAFE (analysis: "variable X is not bound")
unsafe(X) :- !foo(X).                   # X never bound
```

### 7.3 Aggregation Safety

**Rule**: Grouping variables must appear in body atoms.

```mangle
Decl item(Category, Item).
Decl count_per_cat(Category, N).
Decl bad(Category, N).

# ✅ SAFE
count_per_cat(Cat, N) :-
    item(Cat, Item) |>        # Cat appears in the body (name every column)
    do fn:group_by(Cat),      # Group by Cat
    let N = fn:count().

# ❌ UNSAFE (analysis: "variable Cat is not bound")
bad(Cat, N) :-
    item(_, _) |>             # Cat never appears
    do fn:group_by(Cat),      # Can't group by unbound
    let N = fn:count().
```

---

## 8. Evaluation Model

### 8.1 Bottom-Up Semi-Naive

**Algorithm**:
```
Δ₀ = EDB (base facts)
For each stratum S (in order):
    i = 0
    repeat:
        Δᵢ₊₁ = apply rules to Δᵢ (using all facts)
        Δᵢ₊₁ = Δᵢ₊₁ \ (all previously derived facts)
        i++
    until Δᵢ = ∅ (fixpoint)
```

**Key**: Only NEW facts trigger re-evaluation.

### 8.2 Stratification

**For programs with negation**:

1. Compute dependency graph
2. Find strongly connected components
3. Topologically sort
4. Ensure negation edges go backward only
5. Evaluate strata in order

**Example**:
```mangle
Decl source(X).
Decl cond(X).
Decl base(X).
Decl derived(X).
Decl final(X).

# Stratum 0
base(X) :- source(X).

# Stratum 1 (positive dependency on 0)
derived(X) :- base(X), cond(X).

# Stratum 2 (negative dependency on 1)
final(X) :- base(X), !derived(X).
```

**Evaluation**: Stratum 0 → fixpoint, then Stratum 1 → fixpoint, then Stratum 2 → fixpoint.

---

## 9. Comments

**Syntax**: `#` to end of line

**Examples**:
```mangle
# Single line comment

parent(/a, /b).  # Inline comment

# Multi-line "comment":
# Just use multiple
# single-line comments
```

**No block comments** (`/* */` not supported).

---

## 10. REPL Commands

**Interactive interpreter commands**:

| Command | Effect |
|---------|--------|
| `<decl>.` | Add type declaration |
| `<clause>.` | Add clause (fact/rule), evaluate |
| `?<atom>` | Query predicate |
| `::load <path>` | Load source file |
| `::help` | Show help |
| `::pop` | Reset to previous state |
| `::show <pred>` | Show predicate info |
| `::show all` | Show all predicates |
| `Ctrl-D` | Exit REPL |

**Examples**:
```
# Load program
::load vulnerability_scanner.mg

# Query
?vulnerable_project(P, CVE, Sev)

# Show predicate info
::show vulnerable_project

# Exit
Ctrl-D
```

---

## 11. Grammar Summary

**Complete EBNF**:
```ebnf
Program     ::= (Decl | Clause)*
Decl        ::= 'Decl' Atom '.'
Clause      ::= Atom (':-' Atom (',' Atom)*)? '.'
Atom        ::= PredicateSym '(' Term (',' Term)* ')'
             |  'not' Atom
             |  Term Op Term
Term        ::= Const | Var | List | Map | Transform
Const       ::= Name | Int | Float | String
Name        ::= '/' Identifier
Var         ::= UppercaseIdentifier
List        ::= '[' (Term (',' Term)*)? ']'
Map         ::= '{' (Name ':' Term (',' Name ':' Term)*)? '}'
Transform   ::= Term '|>' TransformOp
TransformOp ::= 'do' Function | 'let' Var '=' Function
Op          ::= '=' | '!=' | '<' | '<=' | '>' | '>='
```

---

**Next**: With complete syntax knowledge, see [300-PATTERN_LIBRARY](300-PATTERN_LIBRARY.md) for comprehensive pattern catalog.
