# Mangle Patterns - Comprehensive Catalog

## Basic Patterns

### Selection (WHERE clause)
```mangle
# Filter records by condition
adult(Person) :-
    person(Person),
    age(Person, Age),
    Age >= 18.
```

### Projection (SELECT columns)
```mangle
# Extract specific fields
person_name(ID, Name) :-
    person_record(R),
    :match_field(R, /id, ID),
    :match_field(R, /name, Name).
```

### Join (SQL JOIN)
```mangle
# Natural join on shared variable
employee_dept(EmpName, DeptName) :-
    employee(EmpID, EmpName, DeptID),
    department(DeptID, DeptName).
```

### Union (OR)
```mangle
# Multiple rules = union
contact(Person) :- employee(Person).
contact(Person) :- contractor(Person).
contact(Person) :- vendor(Person).
# contact = employee ∪ contractor ∪ vendor
```

## Negation Patterns

### Set Difference (NOT IN)
```mangle
# Items in A but not in B
difference(X) :-
    set_a(X),
    not set_b(X).
```

### Left Anti-Join (SQL LEFT JOIN ... WHERE B IS NULL)
```mangle
# Employees without departments
unassigned_employee(Emp) :-
    employee(Emp),
    not has_department(Emp).

has_department(Emp) :-
    employee_dept(Emp, _).
```

### Universal Quantification (FOR ALL)
```mangle
# All dependencies satisfied
all_deps_met(Package) :-
    package(Package),
    not has_unmet_dep(Package).

has_unmet_dep(Pkg) :-
    requires(Pkg, Dep),
    not installed(Dep).
```

### Complement Set
```mangle
# Items NOT matching condition
inactive(User) :-
    user(User),
    not active(User).

active(User) :-
    user_status(User, /active).
```

## Recursive Patterns

### Transitive Closure (Reachability)
```mangle
# Base case: direct edge
reachable(X, Y) :- edge(X, Y).

# Recursive case: path via intermediate
reachable(X, Z) :- edge(X, Y), reachable(Y, Z).
```

### Ancestor/Descendant
```mangle
# Ancestors
ancestor(X, Y) :- parent(X, Y).
ancestor(X, Z) :- parent(X, Y), ancestor(Y, Z).

# Descendants (reverse)
descendant(X, Y) :- ancestor(Y, X).
```

### Same Generation (via Common Ancestor)
```mangle
# Same level in hierarchy
same_generation(X, Y) :-
    depth(X, D),
    depth(Y, D),
    X != Y.

depth(Root, 0) :- root(Root).
depth(Node, D) :-
    parent(P, Node),
    depth(P, PD) |>
    let D = fn:plus(PD, 1).
```

### Path Construction
```mangle
# Track full path
path(X, Y, [X, Y]) :- edge(X, Y).
path(X, Z, [X|Rest]) :-
    edge(X, Y),
    path(Y, Z, Rest).
```

### Path with Cost
```mangle
# Accumulate cost along path
path_cost(X, Y, Cost) :- edge(X, Y, Cost).
path_cost(X, Z, TotalCost) :-
    edge(X, Y, Cost1),
    path_cost(Y, Z, Cost2) |>
    let TotalCost = fn:plus(Cost1, Cost2).
```

### Shortest Path
```mangle
# First compute all path lengths
path_len(X, Y, Len) :- ...  # As above

# Then find minimum
shortest(X, Y, MinLen) :-
    path_len(X, Y, Len) |>
    do fn:group_by(X, Y),
    let MinLen = fn:Min(Len).
```

### Cycle Detection
```mangle
# Detect if cycle exists from node
has_cycle(X) :- cycle_edge(X, _).

# Back edge creates cycle
cycle_edge(X, Y) :-
    edge(X, Y),
    reachable(Y, X).
```

### Acyclic Path (Prevent Cycles)
```mangle
# Path without revisiting nodes
acyclic_path(X, Y, [X, Y]) :- edge(X, Y).
acyclic_path(X, Z, [X|Rest]) :-
    edge(X, Y),
    acyclic_path(Y, Z, Rest),
    not member(X, Rest).  # X not already in path

# List membership
member(X, [X|_]).
member(X, [_|Tail]) :- member(X, Tail).
```

## Aggregation Patterns

### Count by Group
```mangle
count_per_category(Cat, N) :-
    item(Cat, _) |>
    do fn:group_by(Cat),
    let N = fn:Count().
```

### Sum by Group
```mangle
total_by_region(Region, Total) :-
    sale(Region, Amount) |>
    do fn:group_by(Region),
    let Total = fn:Sum(Amount).
```

### Average (Derived from Sum/Count)
```mangle
average_per_category(Cat, Avg) :-
    item(Cat, Value) |>
    do fn:group_by(Cat),
    let Total = fn:Sum(Value),
    let Count = fn:Count(),
    let Avg = fn:divide(Total, Count).
```

### Multi-Dimensional Grouping
```mangle
sales_summary(Region, Product, Count, Revenue) :-
    sale(Region, Product, Amount) |>
    do fn:group_by(Region, Product),
    let Count = fn:Count(),
    let Revenue = fn:Sum(Amount).
```

### Conditional Aggregation (HAVING)
```mangle
# Categories with more than 10 items
large_categories(Cat, N) :-
    count_per_category(Cat, N),
    N > 10.
```

### Nested Aggregation
```mangle
# Step 1: Subtotals
category_total(Cat, Total) :-
    item(Cat, Value) |>
    do fn:group_by(Cat),
    let Total = fn:Sum(Value).

# Step 2: Grand total from subtotals
grand_total(GrandTotal) :-
    category_total(_, Total) |>
    do fn:group_by(),
    let GrandTotal = fn:Sum(Total).
```

## Domain-Specific Patterns

### Dependency Analysis
```mangle
# Direct dependencies
depends(Project, Lib) :- depends_direct(Project, Lib).

# Transitive dependencies
depends(Project, Lib) :-
    depends_direct(Project, Intermediate),
    depends(Intermediate, Lib).
```

### Bill of Materials (Recursive Quantities)
```mangle
# Direct components
bom(Product, Part, Qty) :- assembly(Product, Part, Qty).

# Recursive components (multiply quantities)
bom(Product, Part, TotalQty) :-
    assembly(Product, SubAssy, Qty1),
    bom(SubAssy, Part, Qty2) |>
    let TotalQty = fn:multiply(Qty1, Qty2).
```

### Access Control (RBAC)
```mangle
# Permission via direct grant
has_permission(User, Resource, Action) :-
    grant(User, Resource, Action).

# Permission via role
has_permission(User, Resource, Action) :-
    user_role(User, Role),
    role_permission(Role, Resource, Action).

# Effective permissions (after denials)
effective_permission(U, R, A) :-
    has_permission(U, R, A),
    not deny(U, R, A).
```

### Data Lineage
```mangle
# Direct transformation
depends_on(Target, Source) :- transforms(Source, Target).

# Join dependency
depends_on(Target, Source) :- joins(Source, _, Target).
depends_on(Target, Source) :- joins(_, Source, Target).

# Transitive dependency
depends_on(Target, Source) :-
    depends_on(Target, Intermediate),
    depends_on(Intermediate, Source).

# Source lineage (trace back to original sources)
source_lineage(Table, Source) :-
    final_table(Table),
    depends_on(Table, Source),
    source_table(Source).
```

### Vulnerability Propagation
```mangle
# Direct vulnerability
vulnerable_project(Project, CVE, Severity) :-
    project(Project),
    depends_on(Project, Lib, Version),
    cve_affects(Lib, Version, CVE, Severity).

# Transitive vulnerability
has_dependency(Project, Lib, Version) :-
    depends_on(Project, Lib, Version).

has_dependency(Project, Lib, Version) :-
    depends_on(Project, Intermediate, _),
    has_dependency(Intermediate, Lib, Version).
```

### Topological Sort (Level Assignment)
```mangle
# Level 0: no dependencies
level(Node, 0) :-
    node(Node),
    not has_dependency(Node).

has_dependency(Node) :- depends_on(Node, _).

# Level N+1: all dependencies at level ≤ N
level(Node, Lev) :-
    depends_on(Node, Dep),
    level(Dep, DepLev),
    not has_higher_dep(Node, DepLev) |>
    let Lev = fn:plus(DepLev, 1).

has_higher_dep(Node, L) :-
    depends_on(Node, D),
    level(D, DL),
    DL > L.
```

### Temporal Validity
```mangle
# Valid during time period
valid_at(Fact, Time) :-
    fact_validity(Fact, Start, End),
    Time >= Start,
    Time <= End.

# Currently valid facts
current_facts(Fact) :-
    fact_validity(Fact, Start, End),
    current_time(Now),
    Now >= Start,
    Now <= End.
```

### Provenance Tracking
```mangle
# Track source of derived facts
derived(X, Y, Source) :-
    base_fact(X, Y) |>
    let Source = [/base, X, Y].

derived(X, Z, Source) :-
    derived(X, Y, SourceSoFar),
    transformation(Y, Z) |>
    let Source = [/transform | SourceSoFar].
```

## Optimization Patterns

### Early Filtering (Selectivity)
```mangle
# ❌ BAD: Cartesian product first
slow(X, Y) :-
    large_table1(X),    # 10,000 rows
    large_table2(Y),    # 10,000 rows
    filter(X, Y).       # 1,000 matches
# 100,000,000 intermediate results!

# ✅ GOOD: Filter first
fast(X, Y) :-
    filter(X, Y),       # 1,000 matches
    large_table1(X),    # Verify existence
    large_table2(Y).    # Verify existence
# Only 1,000 checks!
```

### Join Ordering (Small to Large)
```mangle
# Place most selective predicates first
efficient(X, Y, Z) :-
    rare_condition(Y, Z),      # 10 results
    join_condition(X, Y),      # 100 results after join
    common_condition(X).       # Check only 100 items
```

### Materialization (Reuse Expensive Computation)
```mangle
# Materialize expensive intermediate result
expensive_step(X, Y) :-
    complex_computation(X, Y).

# Reuse materialized result
final_result(X, Y, Z) :-
    expensive_step(X, Y),
    cheap_step(Y, Z).
```

## SQL Equivalence Table

| SQL Pattern | Mangle Equivalent |
|-------------|-------------------|
| `SELECT DISTINCT col` | Rule head variables |
| `WHERE condition` | Additional body atoms |
| `JOIN` | Shared variables in body |
| `LEFT JOIN ... WHERE B IS NULL` | Negation pattern |
| `UNION` | Multiple rules for same predicate |
| `GROUP BY` | `\|> do fn:group_by()` |
| `HAVING` | Filter on aggregated results |
| `WITH RECURSIVE` | Recursive rules |
| `NOT EXISTS` | Negation with generator |
| `COUNT(*)` | `fn:Count()` in pipeline |
| `SUM(col)` | `fn:Sum(Var)` in pipeline |

## Common Anti-Patterns to Avoid

### Unbounded Negation
```mangle
# ❌ WRONG: X unbound
bad(X) :- not foo(X).

# ✅ CORRECT: X bound first
good(X) :- candidate(X), not foo(X).
```

### Cartesian Product
```mangle
# ❌ WRONG: Huge intermediate result
bad(X, Y) :- table1(X), table2(Y), X = Y.

# ✅ CORRECT: Filter first
good(X) :- table1(X), table2(X).
```

### Infinite Recursion
```mangle
# ❌ WRONG: Unbounded generation
bad_counter(N) :-
    bad_counter(M),
    N = fn:plus(M, 1).

# ✅ CORRECT: Bounded by finite domain
reachable(X, Y) :- edge(X, Y).
reachable(X, Z) :- edge(X, Y), reachable(Y, Z).
```
