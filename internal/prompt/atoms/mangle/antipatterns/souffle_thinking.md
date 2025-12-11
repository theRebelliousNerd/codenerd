# Anti-Pattern: Soufflé Thinking

## Category
Syntax Translation Error (Soufflé Bias)

## Description
Using Soufflé-specific syntax, type systems, directives, and optimizations that don't exist in Mangle.

---

## Anti-Pattern 1: .decl Declarations (lowercase)

### Wrong Approach
```souffle
.decl edge(x:number, y:number)
.decl path(x:number, y:number)
```

### Why It Fails
Mangle uses **Decl** (capital D) with different type syntax.

### Correct Mangle Way
```mangle
Decl edge(X.Type<int>, Y.Type<int>).
Decl path(X.Type<int>, Y.Type<int>).
```

**Key Differences:**
- `Decl` not `.decl`
- Variables are UPPERCASE
- Type syntax: `Var.Type<typename>` not `var:typename`

---

## Anti-Pattern 2: Type Annotations (Soufflé Style)

### Wrong Approach
```souffle
.type NodeID = number
.type Status = symbol

.decl node(id:NodeID, status:Status)
```

### Why It Fails
Mangle doesn't have `.type` aliases. Types are declared inline.

### Correct Mangle Way
```mangle
# Use Type<> syntax directly in Decl:
Decl node(Id.Type<int>, Status.Type</atom>).

# Or use string/atom types:
Decl node(Id.Type<int>, Status.Type<string>).
```

**Note:** Mangle's type system is simpler - no custom type aliases.

---

## Anti-Pattern 3: .input and .output Directives

### Wrong Approach
```souffle
.decl edge(x:number, y:number)
.input edge

.decl path(x:number, y:number)
.output path
```

### Why It Fails
Mangle has no `.input`/`.output` directives. Fact loading and querying happen via Go API.

### Correct Mangle Way
```mangle
# Just declare the predicates:
Decl edge(X.Type<int>, Y.Type<int>).
Decl path(X.Type<int>, Y.Type<int>).

# Load facts in Go:
// store.Add(engine.NewAtom("edge", engine.Number(1), engine.Number(2)))

# Query results in Go:
// results := store.Query("path", X, Y)
```

---

## Anti-Pattern 4: .plan Optimization Hints

### Wrong Approach
```souffle
.plan 1:(2,1)
path(x, y) :- edge(x, z), path(z, y).
```

### Why It Fails
Mangle has no `.plan` directives for manual query optimization.

### Correct Mangle Way
```mangle
# Write rules naturally - engine handles optimization:
path(X, Y) :- edge(X, Z), path(Z, Y).

# For performance, order atoms by selectivity:
# Put most selective atoms first
result(X) :-
    specific_id(X),      # Most selective first
    large_table(X, _).   # Broad match second
```

---

## Anti-Pattern 5: .pragma Directives

### Wrong Approach
```souffle
.pragma "magic-transform" "true"
.pragma "provenance" "explain"
```

### Why It Fails
Mangle has no pragma system for engine configuration.

### Correct Mangle Way
```mangle
# Engine configuration happens in Go:
// cfg := engine.Config{
//     MaxSteps: 10000,
//     Debug: true,
// }
// engine.EvalProgram(program, cfg)
```

---

## Anti-Pattern 6: Record Types

### Wrong Approach
```souffle
.type Pair = [x:number, y:number]

.decl point(p:Pair)
```

### Why It Fails
Mangle doesn't have Soufflé-style record types with named fields.

### Correct Mangle Way
```mangle
# Use structs with /atom keys:
Decl point(P.Type<{/x: int, /y: int}>).

# Then use :match_field to access:
get_x(Point, X) :-
    point(Point),
    :match_field(Point, /x, X).

# Or just flatten the structure:
Decl point(X.Type<int>, Y.Type<int>).
```

---

## Anti-Pattern 7: Algebraic Data Types (ADTs)

### Wrong Approach
```souffle
.type Tree = Leaf{value:number} | Node{left:Tree, right:Tree}
```

### Why It Fails
Mangle has no ADT syntax. Use atoms to tag variants.

### Correct Mangle Way
```mangle
# Tag variants with atoms:
Decl tree_leaf(Id.Type</atom>, Value.Type<int>).
Decl tree_node(Id.Type</atom>, Left.Type</atom>, Right.Type</atom>).

# Example:
tree_leaf(/leaf1, 42).
tree_node(/node1, /leaf1, /leaf2).

# Check variant:
is_leaf(Id) :- tree_leaf(Id, _).
is_node(Id) :- tree_node(Id, _, _).
```

---

## Anti-Pattern 8: Subsumption

### Wrong Approach
```souffle
.decl path(x:number, y:number) btree

path(x, y) :- edge(x, y).
path(x, y) :- path(x, z), edge(z, y).
```

### Why It Fails
Mangle has no `btree` or subsumption hints for duplicate elimination.

### Correct Mangle Way
```mangle
# Mangle automatically handles set semantics:
path(X, Y) :- edge(X, Y).
path(X, Y) :- path(X, Z), edge(Z, Y).

# Duplicates are impossible - facts are sets
```

---

## Anti-Pattern 9: Stratified Negation with .strata

### Wrong Approach
```souffle
.decl a(x:number)
.decl b(x:number)

b(x) :- a(x), !c(x).

.strata 0 a
.strata 1 c
.strata 2 b
```

### Why It Fails
Mangle automatically computes stratification. No manual `.strata` directives.

### Correct Mangle Way
```mangle
# Just write the rules - stratification is automatic:
Decl a(X.Type<int>).
Decl b(X.Type<int>).
Decl c(X.Type<int>).

b(X) :- a(X), not c(X).

# Mangle will detect the stratification:
# Stratum 0: a (base facts)
# Stratum 1: c (depends on a)
# Stratum 2: b (depends on c via negation)
```

---

## Anti-Pattern 10: Choice Domain

### Wrong Approach
```souffle
.decl assign(task:number, worker:number) choice-domain worker

assign(t, w) :- task(t), worker(w), can_do(w, t).
```

### Why It Fails
Mangle has no `choice-domain` for non-deterministic choice.

### Correct Mangle Way
```mangle
# All valid assignments will derive:
assign(Task, Worker) :-
    task(Task),
    worker(Worker),
    can_do(Worker, Task).

# If you need exactly one choice, implement in Go:
# results := store.Query("assign", task, _)
# chosen := results[0]  // Pick first/random
```

---

## Anti-Pattern 11: Aggregation Syntax Differences

### Wrong Approach
```souffle
.decl total(sum:number)

total(s) :- s = sum x : { item(x) }.
```

### Why It Fails
Mangle uses pipe syntax for aggregation, not Soufflé's `sum x : { ... }`.

### Correct Mangle Way
```mangle
Decl total(Sum.Type<int>).

total(Sum) :-
    item(X)
    |> do fn:group_by()
    |> let Sum = fn:Sum(X).
```

---

## Anti-Pattern 12: Inline Constraints

### Wrong Approach
```souffle
path(x, y) :- edge(x, y), x < y.
```

### Why It Fails
This actually works in Mangle! But the syntax for complex constraints differs.

### Correct Mangle Way
```mangle
# Simple constraints work the same:
path(X, Y) :- edge(X, Y), X < Y.

# Complex constraints may need fn: functions:
valid(X, Y) :-
    data(X, Y),
    fn:abs(X) > 10.
```

---

## Anti-Pattern 13: Component Syntax

### Wrong Approach
```souffle
.comp Graph<T> {
    .decl edge(x:T, y:T)
    .decl path(x:T, y:T)

    path(x, y) :- edge(x, y).
}

.init myGraph = Graph<number>
```

### Why It Fails
Mangle has no component/template system.

### Correct Mangle Way
```mangle
# Use packages for modularity:
package graph

Decl edge(X.Type<int>, Y.Type<int>).
Decl path(X.Type<int>, Y.Type<int>).

path(X, Y) :- edge(X, Y).

# Import in another file:
# use myproject/graph
```

---

## Anti-Pattern 14: .limitsize Directive

### Wrong Approach
```souffle
.limitsize path(n=1000000)

path(x, y) :- edge(x, y).
path(x, y) :- path(x, z), edge(z, y).
```

### Why It Fails
Mangle has no `.limitsize` for relation size limits.

### Correct Mangle Way
```mangle
# Add explicit depth/size constraints:
path(X, Y, 1) :- edge(X, Y).

path(X, Y, Depth) :-
    path(X, Z, D1),
    edge(Z, Y),
    D1 < 100,  # Depth limit
    Depth = fn:plus(D1, 1).

# Or configure max steps in Go:
// cfg := engine.Config{MaxSteps: 1000000}
```

---

## Anti-Pattern 15: Lattice Operations

### Wrong Approach
```souffle
.type Cost = number

.decl cost(x:symbol) choice-domain x

cost(x) :- base_cost(x, c), cost(x) = min c.
```

### Why It Fails
Mangle has no lattice/semilattice operations or choice-domain.

### Correct Mangle Way
```mangle
# Use explicit aggregation:
min_cost(X, MinCost) :-
    base_cost(X, Cost)
    |> do fn:group_by(X)
    |> let MinCost = fn:Min(Cost).
```

---

## Key Differences: Soufflé vs Mangle

| Soufflé | Mangle |
|---------|--------|
| `.decl` | `Decl` |
| `var:type` | `Var.Type<type>` |
| `.type Name = ...` | Inline types only |
| `.input`/`.output` | Go API |
| `.plan` | Automatic optimization |
| `.pragma` | Go config |
| Record types | Structs with `/atom` keys |
| ADTs | Tagged atoms |
| `btree` subsumption | Automatic set semantics |
| `.strata` | Automatic stratification |
| `choice-domain` | Derive all, choose in Go |
| `s = sum x : { ... }` | `\|> do fn:group_by() \|> let S = fn:Sum(X)` |
| `.comp`/`.init` | Packages |
| `.limitsize` | Explicit constraints |
| Lattice ops | Explicit aggregation |

---

## Migration Checklist

When translating Soufflé to Mangle:

- [ ] Convert `.decl` to `Decl`
- [ ] Convert `var:type` to `Var.Type<type>`
- [ ] Remove `.type` aliases - use inline types
- [ ] Remove `.input`/`.output` - use Go API
- [ ] Remove `.plan` directives - let engine optimize
- [ ] Remove `.pragma` - configure in Go
- [ ] Convert record types to structs or flatten
- [ ] Convert ADTs to tagged atoms
- [ ] Remove subsumption hints - automatic
- [ ] Remove `.strata` - automatic
- [ ] Remove `choice-domain` - choose in Go
- [ ] Convert aggregation to pipe syntax
- [ ] Remove components - use packages
- [ ] Remove `.limitsize` - add explicit constraints
- [ ] Convert lattice ops to aggregation

---

## Special Note: What IS Similar

Despite differences, these work the same:

- Basic rule syntax: `head :- body.`
- Recursion: `path(X,Y) :- path(X,Z), edge(Z,Y).`
- Negation: `not pred(X)`
- Comparison: `X < Y`, `X = Y`
- String literals: `"hello"`
- Numbers: `42`, `3.14`
- Comments: `# comment`

Focus on the type system and directives - that's where most translation errors occur.
