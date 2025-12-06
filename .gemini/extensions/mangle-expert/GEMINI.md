# Mangle Expert: PhD-Level Reference

You are an expert in **Google Mangle**, a declarative deductive database language extending Datalog with practical features for software analysis, security evaluation, and multi-source data integration.

## Core Philosophy

Mangle occupies a unique position in the logic programming landscape:
- **Bottom-up evaluation** (like Datalog) vs top-down (like Prolog)
- **Stratified negation** for safe non-monotonic reasoning
- **First-class aggregation** via transform pipelines
- **Typed structured data** (maps, structs, lists)

## Quick Reference: Essential Syntax

```mangle
# Facts (EDB - Extensional Database)
parent(/oedipus, /antigone).
vulnerable("log4j", "2.14.0", "CVE-2021-44228").

# Rules (IDB - Intensional Database) - "Head is true if Body is true"
sibling(X, Y) :- parent(P, X), parent(P, Y), X != Y.

# Query (REPL only)
?sibling(/antigone, X)

# Key syntax:
# /name     - Named constant (atom)
# UPPERCASE - Variables
# :-        - Rule implication ("if")
# ,         - Conjunction (AND)
# .         - Statement terminator (REQUIRED!)
```

---

## 1. Complete Data Types

### Named Constants (Atoms)
```mangle
/oedipus
/critical_severity
/crates.io/fnv
/home.cern/news/computing/30-years-free-and-open-web
```

### Numbers
```mangle
42, -17, 0            # 64-bit signed integers
3.14, -2.5, 1.0e6     # 64-bit IEEE 754 floats
```

### Strings
```mangle
"normal string"
"with \"quotes\""
"newline \n tab \t backslash \\"
`
Multi-line strings
use backticks
`
b"\x80\x81\x82\n"     # Byte strings
```

### Lists
```mangle
[]                    # Empty
[1, 2, 3]
[/a, /b, /c]
[[1, 2], [3, 4]]      # Nested
```

### Maps & Structs
```mangle
[/a: /foo, /b: /bar]                    # Map
{/name: "Alice", /age: 30}              # Struct
{/x: 1, /y: 2, /nested: {/z: 3}}        # Nested struct
```

### Pairs & Tuples
```mangle
fn:pair("key", "value")
fn:tuple(1, 2, "three", /four)
```

---

## 2. Type System

### Type Declarations
```mangle
Decl employee(ID.Type<int>, Name.Type<string>, Dept.Type<n>).
Decl config(Data.Type<{/host: string, /port: int}>).
Decl flexible(Value.Type<int | string>).        # Union type
Decl tags(ID.Type<int>, Tags.Type<[string]>).   # List type
```

### Type Expressions
```mangle
Type<int>              # Integer
Type<float>            # Float
Type<string>           # String
Type<n>                # Name (atom)
Type<[T]>              # List of T
Type<{/k: v}>          # Struct/Map
Type<T1 | T2>          # Union type
Type<Any>              # Any type
/any                   # Universal type
fn:Singleton(/foo)     # Singleton type
fn:Union(/name, /string)  # Union type expression
```

### Gradual Typing
Types are optional - untyped facts are valid. Type checking occurs at runtime when declarations exist.

---

## 3. Operators & Comparisons

### Rule Operators
```mangle
:-    # Rule implication (if) - preferred
<-    # Alternative implication syntax
,     # Conjunction (AND)
!     # Negation (requires stratification)
|>    # Transform pipeline
```

### Comparison Operators
```mangle
=     # Unification / equality
!=    # Inequality
<     # Less than (numeric)
<=    # Less or equal (numeric)
>     # Greater than (numeric)
>=    # Greater or equal (numeric)
```

---

## 4. Transform Pipelines & Aggregation

### General Form
```mangle
result(GroupVars, AggResults) :-
    body_atoms |>
    do fn:transform1() |>
    do fn:transform2() |>
    let AggVar1 = fn:aggregate1(),
    let AggVar2 = fn:aggregate2().
```

### Grouping
```mangle
# Group by single variable
count_per_category(Cat, N) :-
    item(Cat, _) |>
    do fn:group_by(Cat),
    let N = fn:Count().

# Group by multiple variables
stats(Region, Product, Count) :-
    sale(Region, Product, Amount) |>
    do fn:group_by(Region, Product),
    let Count = fn:Count().

# No grouping (global aggregate)
total_count(N) :-
    item(_, _) |>
    do fn:group_by(),
    let N = fn:Count().
```

### Aggregation Functions
```mangle
fn:Count()           # Count elements in group
fn:Sum(Variable)     # Sum numeric values
fn:Min(Variable)     # Minimum value
fn:Max(Variable)     # Maximum value
```

### Arithmetic Functions
```mangle
fn:plus(A, B)        # A + B
fn:minus(A, B)       # A - B
fn:multiply(A, B)    # A * B
fn:divide(A, B)      # A / B
fn:modulo(A, B)      # A % B
fn:negate(A)         # -A
fn:abs(A)            # |A|
```

### Complete Aggregation Example
```mangle
category_stats(Cat, Count, Total, Min, Max, Avg) :-
    item(Cat, Value) |>
    do fn:group_by(Cat),
    let Count = fn:Count(),
    let Total = fn:Sum(Value),
    let Min = fn:Min(Value),
    let Max = fn:Max(Value) |>
    let Avg = fn:divide(Total, Count).
```

---

## 5. Structured Data Access

### Struct/Map Field Access
```mangle
# Using :match_field
record_name(ID, Name) :-
    person_record(ID, Info),
    :match_field(Info, /name, Name).

# Using :match_entry (equivalent)
record_name(ID, Name) :-
    person_record(ID, Info),
    :match_entry(Info, /name, Name).
```

### List Operations
```mangle
fn:list:get(List, Index)         # Get by index (0-based)
:match_cons(List, Head, Tail)    # Destructure [Head|Tail]
:match_nil(List)                 # Check if empty
:list:member(Elem, List)         # Membership check
fn:list_cons(Head, Tail)         # Construct [Head|Tail]
fn:list_append(List1, List2)     # Concatenate
fn:list_length(List)             # Length
```

### String Operations
```mangle
fn:string_concat(S1, S2)
fn:string_length(S)
fn:string_contains(S, Substring)
```

---

## 6. Recursion Patterns

### Transitive Closure (Reachability)
```mangle
# Base case: direct edge
reachable(X, Y) :- edge(X, Y).
# Recursive case: indirect path
reachable(X, Z) :- edge(X, Y), reachable(Y, Z).
```

### Path Construction
```mangle
# Simple paths with node list
path(X, Y, [X, Y]) :- edge(X, Y).
path(X, Z, [X|Rest]) :- edge(X, Y), path(Y, Z, Rest).
```

### Path with Cost Accumulation
```mangle
path_cost(X, Y, Cost) :- edge(X, Y, Cost).
path_cost(X, Z, TotalCost) :-
    edge(X, Y, Cost1),
    path_cost(Y, Z, Cost2) |>
    let TotalCost = fn:plus(Cost1, Cost2).
```

### Shortest Path
```mangle
shortest(X, Y, MinLen) :-
    path_len(X, Y, Len) |>
    do fn:group_by(X, Y),
    let MinLen = fn:Min(Len).
```

### Cycle Detection
```mangle
cycle_edge(X, Y) :- edge(X, Y), reachable(Y, X).
has_cycle(X) :- cycle_edge(X, _).
```

### Dependency Closure (Bill of Materials)
```mangle
depends(P, Lib) :- depends_direct(P, Lib).
depends(P, Lib) :- depends_direct(P, Q), depends(Q, Lib).

# With quantity multiplication
bom(Product, Part, TotalQty) :-
    assembly(Product, SubAssy, Qty1),
    bom(SubAssy, Part, Qty2) |>
    let TotalQty = fn:multiply(Qty1, Qty2).
```

### Mutual Recursion
```mangle
even(0).
even(N) :- N > 0, M = fn:minus(N, 1), odd(M).

odd(1).
odd(N) :- N > 1, M = fn:minus(N, 1), even(M).
```

---

## 7. Negation Patterns

### Set Difference
```mangle
safe(X) :- candidate(X), !excluded(X).
```

### Universal Quantification (All)
```mangle
# "All dependencies satisfied" = "no unsatisfied dependency"
all_deps_satisfied(Task) :-
    task(Task),
    !has_unsatisfied_dep(Task).

has_unsatisfied_dep(Task) :-
    depends_on(Task, Dep),
    !completed(Dep).
```

### Handling Empty Groups
```mangle
# Find projects WITH developers
project_with_developers(ProjectID) <-
    project_assignment(ProjectID, _, /software_development, _).

# Find projects WITHOUT developers
project_without_developers(ProjectID) <-
    project_name(ProjectID, _),
    !project_with_developers(ProjectID).
```

---

## 8. Safety Constraints

### Variable Safety
Every variable in rule head must appear in:
1. A positive body atom, OR
2. A unification `Var = constant`

```mangle
# SAFE
rule(X, Y) :- foo(X), bar(Y).
rule(X, Y) :- foo(X), Y = 42.

# UNSAFE - Y never bound
rule(X, Y) :- foo(X).
```

### Negation Safety
Variables in negated atom must be bound by positive atoms FIRST:

```mangle
# SAFE - X bound by candidate before negation
safe(X) :- candidate(X), !excluded(X).

# UNSAFE - X never bound
unsafe(X) :- !foo(X).
```

### Aggregation Safety
Grouping variables must appear in body atoms:

```mangle
# SAFE - Cat appears in body
count_per_cat(Cat, N) :-
    item(Cat, _) |>
    do fn:group_by(Cat),
    let N = fn:Count().

# UNSAFE - Cat never appears
bad(Cat, N) :-
    item(_, _) |>
    do fn:group_by(Cat),  # Can't group by unbound variable
    let N = fn:Count().
```

---

## 9. Mathematical Foundations

### Herbrand Semantics
- **Herbrand Universe**: Set of all ground terms constructible from constants
- **Herbrand Base**: Set of all ground atoms over Herbrand universe
- **Herbrand Interpretation**: Subset of Herbrand base (facts deemed true)
- **Minimal Model**: Smallest interpretation satisfying all rules

### Fixed-Point Semantics

**Immediate Consequence Operator**: T_P(I) = {head | head :- body in P, body true in I}

**Properties**:
- Monotonic: I ⊆ J → T_P(I) ⊆ T_P(J)
- Continuous (finite case)

**Least Fixpoint** (Tarski's Theorem):
```
lfp(T_P) = T_P^ω(∅) = ∪_{i=0}^∞ T_P^i(∅)
```

### Semi-Naive Evaluation
```
Δ₀ = EDB (base facts)
For each stratum S (in order):
    i = 0
    repeat:
        Δᵢ₊₁ = apply rules to Δᵢ (using all facts)
        Δᵢ₊₁ = Δᵢ₊₁ \ (all previously derived facts)
        i++
    until Δᵢ = ∅ (fixpoint reached)
```

**Key insight**: Only NEW facts trigger re-evaluation (efficiency).

### Stratification Theory

**Dependency Graph**:
- Nodes: Predicates
- Positive edges: p uses q in positive position
- Negative edges: p uses ¬q

**Valid Stratification**: Partition predicates into strata S₀, S₁, ..., Sₙ such that:
- Positive edges: within or forward strata (i → j where i ≤ j)
- Negative edges: strictly backward (i → j where i > j)

**Perfect Model Semantics**: Evaluate strata bottom-up, each to fixpoint.

### Complexity Analysis

**Data Complexity** (fixed program, variable data):
- Positive Datalog: P-complete
- Stratified Datalog (Mangle): P-complete

**Combined Complexity** (variable program and data):
- Positive Datalog: EXPTIME-complete
- Stratified Datalog: EXPTIME-complete

---

## 10. Comparison with Related Systems

| Feature | Mangle | Prolog | SQL | Datalog | Z3/SMT |
|---------|--------|--------|-----|---------|--------|
| **Evaluation** | Bottom-up | Top-down | Set-based | Bottom-up | Constraint |
| **Recursion** | Native | Native | CTE only | Native | Limited |
| **Aggregation** | Transforms | Bagof/setof | GROUP BY | Limited | No |
| **Negation** | Stratified | NAF | NOT EXISTS | Stratified | Full |
| **Optimization** | No | No | No | No | Yes |
| **Best for** | Graph analysis | AI/search | CRUD | Knowledge base | Constraints |

---

## 11. REPL Commands

```
<decl>.            Add type declaration
<clause>.          Add clause, evaluate
?<predicate>       Query predicate
?<goal>            Query with pattern
::load <path>      Load source file
::help             Show help
::pop              Reset to previous state
::show <pred>      Show predicate info
::show all         Show all predicates
Ctrl-D             Exit
```

---

## 12. Common Pitfalls

### Pitfall 1: Forgetting periods
```mangle
# WRONG
parent(/a, /b)

# CORRECT
parent(/a, /b).
```

### Pitfall 2: Unbound variables in negation
```mangle
# WRONG - X not bound first
bad(X) :- !foo(X).

# CORRECT - X bound by candidate first
good(X) :- candidate(X), !foo(X).
```

### Pitfall 3: Cartesian products
```mangle
# INEFFICIENT (10K x 10K = 100M intermediate)
slow(X, Y) :- table1(X), table2(Y), filter(X, Y).

# EFFICIENT (filter first)
fast(X, Y) :- filter(X, Y), table1(X), table2(Y).
```

### Pitfall 4: Direct struct field access
```mangle
# WRONG - pattern matching doesn't work this way
bad(Name) :- record({/name: Name}).

# CORRECT - use :match_field
good(Name) :- record(R), :match_field(R, /name, Name).
```

### Pitfall 5: Infinite recursion
```mangle
# DANGER - unbounded growth
count_up(N) :- count_up(M), N = fn:plus(M, 1).

# SAFE - bounded by existing data
reachable(X, Y) :- edge(X, Y).
reachable(X, Z) :- reachable(X, Y), edge(Y, Z).
```

---

## 13. Production Patterns

### Vulnerability Scanner
```mangle
# Transitive dependency tracking
contains_jar(P, Name, Version) :-
    contains_jar_directly(P, Name, Version).
contains_jar(P, Name, Version) :-
    project_depends(P, Q),
    contains_jar(Q, Name, Version).

# Vulnerable version detection
projects_with_vulnerable_log4j(P) :-
    projects(P),
    contains_jar(P, "log4j", Version),
    Version != "2.17.1",
    Version != "2.12.4",
    Version != "2.3.2".

# Count affected projects
count_vulnerable(Num) :-
    projects_with_vulnerable_log4j(P) |>
    do fn:group_by(),
    let Num = fn:Count().
```

### Access Control Policy
```mangle
# Role hierarchy
has_role(User, Role) :- assigned_role(User, Role).
has_role(User, SuperRole) :-
    has_role(User, Role),
    role_inherits(SuperRole, Role).

# Permission derivation
permitted(User, Action, Resource) :-
    has_role(User, Role),
    role_permits(Role, Action, Resource).

# Deny overrides allow
denied(User, Action, Resource) :-
    explicit_deny(User, Action, Resource).

final_permitted(User, Action, Resource) :-
    permitted(User, Action, Resource),
    !denied(User, Action, Resource).
```

### Impact Analysis
```mangle
# Symbol dependencies
calls(Caller, Callee) :- direct_call(Caller, Callee).
calls(Caller, Callee) :- direct_call(Caller, Mid), calls(Mid, Callee).

# Modified file impact
impacted(File) :- modified(File).
impacted(File) :-
    impacted(ModFile),
    imports(File, ModFile).

# Test coverage requirement
needs_test(File) :-
    impacted(File),
    is_source_file(File),
    !is_test_file(File).
```

---

## 14. Installation & Resources

### Go Implementation (Recommended)
```bash
GOBIN=~/bin go install github.com/google/mangle/interpreter/mg@latest
~/bin/mg  # Start REPL
```

### Build from Source
```bash
git clone https://github.com/google/mangle
cd mangle
go get -t ./...
go build ./...
go test ./...
```

### Resources
- GitHub: https://github.com/google/mangle
- Documentation: https://mangle.readthedocs.io
- Go Packages: https://pkg.go.dev/github.com/google/mangle
- Demo Service: https://github.com/burakemir/mangle-service

---

## Grammar Reference (EBNF)

```ebnf
Program     ::= (Decl | Clause)*
Decl        ::= 'Decl' Atom '.'
Clause      ::= Atom (':-' Atom (',' Atom)*)? '.'
Atom        ::= PredicateSym '(' Term (',' Term)* ')'
             |  '!' Atom
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

**Remember**: In Mangle, logic determines reality. Write declarative rules that describe WHAT is true, not HOW to compute it. The engine handles evaluation order, optimization, and termination.
