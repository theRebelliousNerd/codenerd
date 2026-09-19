# Advanced Mangle Patterns

## Stratified Negation

### Understanding Stratification

Programs with negation must be **stratified**: predicates partitioned into layers where:
- Positive dependencies allowed within/between strata
- Negative dependencies only from higher to lower strata

```mangle
Decl cve_database(Lib, Version, CVE).
Decl depends_on(Project, Lib, Version).
Decl project(Project).
Decl vulnerable_lib(Lib, Version).
Decl has_vuln(Project).
Decl clean_project(Project).

# Stratum 0: Base facts
vulnerable_lib(Lib, Version) :- cve_database(Lib, Version, _).

# Stratum 1: Uses stratum 0
has_vuln(Project) :- 
    depends_on(Project, Lib, Version),
    vulnerable_lib(Lib, Version).

# Stratum 2: Uses negation of stratum 1
clean_project(Project) :-
    project(Project),
    !has_vuln(Project).
```

### Stratification Rules
- Compute dependency graph
- Identify strongly connected components
- Topologically sort
- Negation edges must go backward only

## Performance Optimization

### Rule Ordering Strategy

Place **most selective predicates first** to minimize intermediate results:

```mangle
# ❌ INEFFICIENT: Creates huge cross product
Decl large_table1(X).
Decl large_table2(Y).
Decl small_filter(X, Y, Z).
bad(X, Y, Z) :- 
    large_table1(X),      # 10,000 rows
    large_table2(Y),      # 10,000 rows
    small_filter(X, Y, Z). # 100 rows matching

# ✅ EFFICIENT: Filter early
good(X, Y, Z) :- 
    small_filter(X, Y, Z), # 100 rows
    large_table1(X),       # 100 joins
    large_table2(Y).       # 100 joins
```

### Semi-Naive Evaluation

Mangle uses **semi-naive evaluation** by default:

**Naive** (slow):
- Recomputes ALL facts every iteration
- O(n × r × |facts|²) per iteration

**Semi-Naive** (fast):
- Only processes NEW facts (deltas)
- O(r × |Δfacts| × |facts|) per iteration

**Benefit**: Dramatically faster for recursive queries.

### Memory Management

```go
// Set fact limit to prevent runaway computation
store := factstore.NewSimpleInMemoryStore()
engine.EvalProgram(programInfo, store, 
    engine.WithCreatedFactLimit(1000000))
```

## Advanced Recursive Patterns

### Path Tracking

```mangle
# Track paths through dependency graph (a list literal can sit in the head;
# fn:list:cons prepends; there is no [H|T] syntax)
Decl contains_jar_directly(Project, Lib, Version).
Decl project_depends(Project, Dependency).
Decl dependency_path(Project, Lib, Path) bound [/name, /name, fn:List(/name)].

dependency_path(Project, Lib, [Project, Lib]) :-
    contains_jar_directly(Project, Lib, _).

dependency_path(Project, Lib, Path) :-
    project_depends(Project, Intermediate),
    dependency_path(Intermediate, Lib, SubPath),
    Path = fn:list:cons(Project, SubPath).
```

### Cycle Detection

```mangle
# Detect cycles in dependencies
Decl depends_on(X, Y).
Decl reachable(Y, X).
cycle_edge(X, Y) :- 
    depends_on(X, Y),
    reachable(Y, X).

has_cycle(X) :- cycle_edge(X, _).
```

### Distance/Cost Accumulation

```mangle
# Track minimum distance
Decl edge(Start, End, Dist).
min_distance(Start, End, Dist) :-
    edge(Start, End, Dist).

min_distance(Start, End, TotalDist) :-
    edge(Start, Mid, Dist1),
    min_distance(Mid, End, Dist2) |>
    let TotalDist = fn:plus(Dist1, Dist2).
```

## Complex Aggregations

### Multi-level Grouping

```mangle
# Group by category, then summarize
Decl item(Category, Item, Value).
category_stats(Category, Count, TotalValue) :- 
    item(Category, Item, Value) |> 
    do fn:group_by(Category), 
    let Count = fn:count(),
    let TotalValue = fn:sum(Value).
```

### Conditional Aggregation

```mangle
# Count only items meeting criteria: the filter is a comparison in the body
Decl item(Category, Value) bound [/name, /number].
Decl high_value_count(Category, Count) bound [/name, /number].
high_value_count(Category, Count) :-
    item(Category, Value), Value > 1000
    |> do fn:group_by(Category), let Count = fn:count().
```

### Nested Aggregation

```mangle
# Average of category totals
Decl category_stats(Cat, Total).
overall_average(Avg) :- 
    category_stats(Cat, Total) |> 
    do fn:group_by(), 
    let Avg = fn:avg(Total).
```

## Type System Advanced Features

### Gradual Typing

```mangle
# Optional type declarations
Decl employee(ID, Name) bound [/number, /string].

# Type inference from usage
employee(1, "Alice").
employee(2, "Bob").

# Type checking at runtime
high_id(Name) :- 
    employee(ID, Name),
    ID > 1000.  # Type checker ensures ID is numeric
```

### Structured Type Declarations

```mangle
Decl person(ID, Info) bound [/number, fn:Struct(/name, /string, /age, /number)].

# Composite columns have type functions: fn:Struct, fn:List, fn:Map, fn:Pair,
# fn:Union (600-TYPE_SYSTEM); the number of bounds equals the number of arguments.
person(1, {/name: "Alice", /age: 30}).
```

### Union Types

```mangle
# Mangle has no union types: leave the arg unbound and it accepts anything.
Decl flexible(Value).

flexible(42).
flexible("text").
```

## Lattice Support (specified, not usable on the pinned engine)

**Concept**: maintain only the best element of a partial order. The engine's spec
(`docs/spec_decls.md`) describes custom lattices: a `Decl` with one `fundep([Key], [Value])` and a
`merge([...], <pred>)` descriptor naming a `deferred` three-place predicate, and the evaluator has
the code for it (`engine/seminaivebottomup.go`, `mergeDelta`). Run on the pinned engine
(2026-09-19), such a program loads and its evaluation never finishes, with the merge predicate
spelled as a string or a name. Use an aggregation for min and max instead:

**Use cases**:
- Interval analysis (min/max bounds)
- Provenance tracking
- Abstract interpretation

```mangle
Decl connection(Route, Price) bound [/name, /number].
Decl best_price(Route, Price) bound [/name, /number].

# Keep the minimum with an aggregation.
best_price(Route, Price) :-
    connection(Route, P) |>
    do fn:group_by(Route),
    let Price = fn:min(P).
```

## Production Patterns

### Multi-Source Integration

```mangle
# Combine data from database, API, files: put the constant in the head. (A clause
# ending in `= /database.` never ends: the name constant absorbs the period.)
Decl database_vuln(Lib, CVE) bound [/name, /string].
Decl api_vuln(Lib, CVE) bound [/name, /string].
Decl file_vuln(Lib, CVE) bound [/name, /string].
Decl all_vulnerabilities(Lib, CVE, Source) bound [/name, /string, /name].

all_vulnerabilities(Lib, CVE, /database) :- database_vuln(Lib, CVE).
all_vulnerabilities(Lib, CVE, /api) :- api_vuln(Lib, CVE).
all_vulnerabilities(Lib, CVE, /file) :- file_vuln(Lib, CVE).
```

### Incremental Updates

```go
// Initial evaluation
store := factstore.NewSimpleInMemoryStore()
engine.EvalProgram(programInfo, store)

// Add new facts
newFacts := []ast.Atom{...}
for _, fact := range newFacts {
    store.Add(fact)
}

// Re-evaluate (only new derivations computed)
engine.EvalProgram(programInfo, store)
```

### Monitoring Evaluation

```go
// Track performance
stats, _ := engine.EvalProgramWithStats(programInfo, store)

for i, duration := range stats.Duration {
    log.Printf("Stratum %d: %v", i, duration)
}
```

## Debugging Complex Programs

### Break into Intermediate Predicates

```mangle
# Instead of complex single rule:
# result(X, Y, Z) :- condition1(X), condition2(Y), condition3(Z), filter(X, Y, Z).

# Break into stages:
Decl condition1(X).
Decl condition2(Y).
Decl condition3(Z).
Decl filter(X, Y, Z).
stage1(X) :- condition1(X).
stage2(X, Y) :- stage1(X), condition2(Y).
result(X, Y, Z) :- stage2(X, Y), condition3(Z), filter(X, Y, Z).

# Query each stage to isolate issues
```

### Add Explicit Constraints

```mangle
# Make implicit constraints explicit
Decl person(Person).
Decl task(Task).
Decl has_skill(Person, Skill).
Decl requires_skill(Task, Skill).
Decl available(Person, Time).
Decl scheduled(Task, Time).
valid_assignment(Person, Task) :-
    person(Person),
    task(Task),
    has_skill(Person, Skill),
    requires_skill(Task, Skill),
    available(Person, Time),
    scheduled(Task, Time).
```

## Common Advanced Pitfalls

### Unbounded Recursion

```mangle
# ❌ DANGEROUS: No termination
infinite(X) :- infinite(X).

# ❌ DANGEROUS: Unconstrained growth
count_up(N) :- count_up(M), N = fn:plus(M, 1).

# ✅ SAFE: Bounded by condition
countdown(N) :- 
    countdown(M), 
    N = fn:plus(M, 1), 
    N < 1000.
```

### Negation with Unbound Variables

```mangle
Decl domain(X).
Decl exists(X).
Decl unmatched(X).

# ❌ WRONG (analysis: "variable X is not bound")
# unmatched(X) :- !exists(X).

# ✅ CORRECT (and `bound` is a keyword: it cannot name a predicate)
unmatched(X) :-
    domain(X),  # X bound here
    !exists(X).
```

### Excessive Intermediate Results

```mangle
# ❌ INEFFICIENT: Cartesian product
Decl all_x(X).
Decl all_y(Y).
Decl related(X, Y).
many_results(X, Y) :- 
    all_x(X),
    all_y(Y).

# ✅ EFFICIENT: Only join when needed
few_results(X, Y) :- 
    all_x(X),
    all_y(Y),
    related(X, Y).  # Selective join condition
```
