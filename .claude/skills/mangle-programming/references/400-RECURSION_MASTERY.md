# 400: Recursion Mastery - Deep Dive

**Purpose**: Master every recursive technique for graph analysis, dependency tracking, and hierarchical data.

## Recursion Fundamentals

### Linear Recursion
```mangle
# Single recursive call per rule
Decl parent(X, Y).
ancestor(X, Y) :- parent(X, Y).
ancestor(X, Z) :- parent(X, Y), ancestor(Y, Z).
```

### Non-Linear Recursion
```mangle
# Multiple recursive paths
Decl edge(X, Y).
connected(X, Y) :- edge(X, Y).
connected(X, Y) :- edge(Y, X).  # Symmetric
connected(X, Z) :- connected(X, Y), connected(Y, Z).  # Transitive
```

## Path Construction

### Simple Paths
```mangle
Decl edge(From, To) bound [/name, /name].
Decl path(From, To, Nodes) bound [/name, /name, fn:List(/name)].
path(X, Y, [X, Y]) :- edge(X, Y).
path(X, Z, Nodes) :- edge(X, Y), path(Y, Z, Rest), Nodes = fn:list:cons(X, Rest).
```

### Path with Metadata
```mangle
# One predicate has one arity: a weighted edge is a different predicate.
Decl edge(From, To) bound [/name, /name].
Decl weighted_edge(From, To, Cost) bound [/name, /name, /number].
Decl path_len(From, To, Len) bound [/name, /name, /number].
Decl path_cost(From, To, Cost) bound [/name, /name, /number].

# Track length (arithmetic is an assignment in the body, not a transform)
path_len(X, Y, 1) :- edge(X, Y).
path_len(X, Z, Len) :-
    edge(X, Y),
    path_len(Y, Z, SubLen),
    Len = fn:plus(SubLen, 1).

# Track cost
path_cost(X, Y, Cost) :- weighted_edge(X, Y, Cost).
path_cost(X, Z, TotalCost) :-
    weighted_edge(X, Y, Cost1),
    path_cost(Y, Z, Cost2),
    TotalCost = fn:plus(Cost1, Cost2).
```

## Cycle Detection

### Back Edges
```mangle
Decl edge(X, Y).
Decl reachable(Y, X).
cycle_edge(X, Y) :- edge(X, Y), reachable(Y, X).
```

### Has Cycle
```mangle
Decl cycle_edge(X, Y).
has_cycle(X) :- cycle_edge(X, _).
```

### Cycle-Free Paths
```mangle
# Prevent revisiting nodes. There is no [H|T] pattern and no user-defined member/2:
# fn:list:contains returns /true or /false.
Decl edge(From, To) bound [/name, /name].
Decl acyclic_path(From, To, Nodes) bound [/name, /name, fn:List(/name)].
acyclic_path(X, Y, [X, Y]) :- edge(X, Y), X != Y.
acyclic_path(X, Z, Nodes) :-
    edge(X, Y),
    acyclic_path(Y, Z, Rest),
    fn:list:contains(Rest, X) = /false,
    Nodes = fn:list:cons(X, Rest).
```

## Distance & Optimization

### Shortest Path
```mangle
Decl edge(From, To) bound [/name, /name].
Decl path_len(From, To, Len) bound [/name, /name, /number].
Decl shortest(From, To, MinLen) bound [/name, /name, /number].

# All path lengths first (terminates on an acyclic graph)
path_len(X, Y, 1) :- edge(X, Y).
path_len(X, Z, Len) :- edge(X, Y), path_len(Y, Z, Rest), Len = fn:plus(Rest, 1).

# Then minimize, in a later stratum
shortest(X, Y, MinLen) :-
    path_len(X, Y, Len) |>
    do fn:group_by(X, Y),
    let MinLen = fn:min(Len).
```

### Maximum Depth
```mangle
Decl root(Root).
Decl child(Parent, Node).
depth(Root, 0) :- root(Root).
depth(Node, D) :- 
    child(Parent, Node),
    depth(Parent, PD) |>
    let D = fn:plus(PD, 1).

max_depth(MaxD) :- 
    depth(_, D) |>
    let MaxD = fn:max(D).
```

## Mutual Recursion

```mangle
# A and B defined in terms of each other. Evaluation is bottom-up, so a rule cannot
# test a number it has not generated (`N > 0` with N unbound fails analysis): count up
# from the base case and bound the domain.
Decl even(N) bound [/number].
Decl odd(N) bound [/number].
even(0).
odd(N) :- even(M), N = fn:plus(M, 1), N <= 10.
even(N) :- odd(M), N = fn:plus(M, 1), N <= 10.
```

## Transitive Patterns

### Dependency Closure
```mangle
Decl depends_direct(P, Lib).
depends(P, Lib) :- depends_direct(P, Lib).
depends(P, Lib) :- depends_direct(P, Q), depends(Q, Lib).
```

### Bill of Materials
```mangle
Decl assembly(Product, Part, Qty) bound [/name, /name, /number].
Decl bom(Product, Part, Qty) bound [/name, /name, /number].

# Direct components
bom(Product, Part, Qty) :- assembly(Product, Part, Qty).

# Recursive components (multiply quantities; the function is fn:mult)
bom(Product, Part, TotalQty) :-
    assembly(Product, SubAssy, Qty1),
    bom(SubAssy, Part, Qty2),
    TotalQty = fn:mult(Qty1, Qty2).
```

## Tree Operations

### Subtree Size
```mangle
# Aggregating inside the recursion cannot be stratified. Derive the descendants
# positively, count them in a later stratum, then add the node itself.
Decl node(N) bound [/name].
Decl child(Parent, Child) bound [/name, /name].
Decl descendant(Node, D) bound [/name, /name].
Decl has_descendant(Node) bound [/name].
Decl descendant_count(Node, N) bound [/name, /number].
Decl subtree_size(Node, Size) bound [/name, /number].

descendant(N, C) :- child(N, C).
descendant(N, D) :- child(N, C), descendant(C, D).
has_descendant(Node) :- descendant(Node, _).
descendant_count(Node, N) :- descendant(Node, D) |> do fn:group_by(Node), let N = fn:count().
subtree_size(Node, Size) :- descendant_count(Node, N), Size = fn:plus(N, 1).
subtree_size(Node, 1) :- node(Node), !has_descendant(Node).
```

### Topological Sort
```mangle
# A node's level is 1 + the deepest dependency's level. Negating or aggregating
# inside the recursion cannot be stratified; derive every depth positively, then take
# the maximum in a later stratum.
Decl node(N) bound [/name].
Decl depends_on(Node, Dep) bound [/name, /name].
Decl has_dependency(Node) bound [/name].
Decl depth(Node, D) bound [/name, /number].
Decl level(Node, L) bound [/name, /number].

has_dependency(Node) :- depends_on(Node, _).
depth(Node, 0) :- node(Node), !has_dependency(Node).
depth(Node, D) :- depends_on(Node, Dep), depth(Dep, DepDepth), D = fn:plus(DepDepth, 1).
level(Node, L) :- depth(Node, D) |> do fn:group_by(Node), let L = fn:max(D).
```

## Termination Analysis

### Guaranteed Termination
✅ **Finite base facts** + **Monotonic rules** → Always terminates

### Potential Non-Termination
❌ **Unbounded recursion**:
```mangle
# Infinite growth (don't do this!)
count_up(N) :- count_up(M), N = fn:plus(M, 1).
```

### Safe Patterns
✅ **Bounded by existing data**:
```mangle
# Can't create new nodes, only traverse existing
Decl edge(X, Y).
reachable(X, Y) :- edge(X, Y).
reachable(X, Z) :- reachable(X, Y), edge(Y, Z).
# Terminates when all paths explored
```

---

**See also**: 700-OPTIMIZATION.md for recursion performance tuning.
