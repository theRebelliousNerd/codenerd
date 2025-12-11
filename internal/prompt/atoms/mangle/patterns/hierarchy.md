# Hierarchy Patterns

## Problem Description

Hierarchical data (org charts, file systems, class inheritance, AST trees) requires:
- Ancestor/descendant queries
- Subtree extraction
- Level/depth calculations
- Sibling relationships

## Core Pattern: Ancestor-Descendant

### Template
```mangle
# Direct parent-child
ancestor(X, Y) :- parent(X, Y).

# Transitive: ancestor of ancestor
ancestor(X, Z) :- ancestor(X, Y), parent(Y, Z).
```

### Complete Working Example
```mangle
# Schema
Decl parent(Parent.Type<string>, Child.Type<string>).
Decl ancestor(Ancestor.Type<string>, Descendant.Type<string>).

# Facts - Org chart
parent("CEO", "VP_Eng").
parent("CEO", "VP_Sales").
parent("VP_Eng", "EM_Backend").
parent("VP_Eng", "EM_Frontend").
parent("EM_Backend", "Dev1").
parent("EM_Backend", "Dev2").

# Rules
ancestor(X, Y) :- parent(X, Y).
ancestor(X, Z) :- ancestor(X, Y), parent(Y, Z).

# Query: Who are all descendants of CEO?
# Answer: ancestor("CEO", X) gives VP_Eng, VP_Sales, EM_Backend, EM_Frontend, Dev1, Dev2
```

## Variation 1: Depth/Level in Hierarchy

### Problem
Calculate how many levels deep each node is.

### Solution
```mangle
# Schema
Decl parent(Parent.Type<string>, Child.Type<string>).
Decl depth(Node.Type<string>, Level.Type<int>).
Decl root(Node.Type<string>).

# Root nodes have depth 0
root(X) :- parent(X, _), not parent(_, X).
depth(X, 0) :- root(X).

# Children are one level deeper
depth(Child, D) :-
  parent(Parent, Child),
  depth(Parent, ParentDepth),
  D = fn:plus(ParentDepth, 1).
```

### Example
```mangle
parent("A", "B").
parent("A", "C").
parent("B", "D").

# Results:
# depth("A", 0)  # root
# depth("B", 1)
# depth("C", 1)
# depth("D", 2)
```

## Variation 2: Subtree Extraction

### Problem
Get all nodes under a specific subtree root.

### Solution
```mangle
# Schema
Decl parent(Parent.Type<string>, Child.Type<string>).
Decl in_subtree(Root.Type<string>, Node.Type<string>).

# Root is in its own subtree
in_subtree(Root, Root).

# All descendants are in subtree
in_subtree(Root, Child) :-
  in_subtree(Root, Node),
  parent(Node, Child).
```

### Example
```mangle
parent("A", "B").
parent("A", "C").
parent("B", "D").
parent("B", "E").
parent("C", "F").

# Query: in_subtree("B", X)
# Results: "B", "D", "E"
```

## Variation 3: Leaf Nodes

### Problem
Find all leaf nodes (nodes with no children).

### Solution
```mangle
# Schema
Decl parent(Parent.Type<string>, Child.Type<string>).
Decl leaf(Node.Type<string>).
Decl node(N.Type<string>).

# All nodes (parents and children)
node(X) :- parent(X, _).
node(X) :- parent(_, X).

# Leaf: node with no children
leaf(X) :- node(X), not parent(X, _).
```

### Example
```mangle
parent("A", "B").
parent("A", "C").
parent("B", "D").

# Results:
# leaf("D")
# leaf("C")
# NOT leaf("A") - has children
# NOT leaf("B") - has children
```

## Variation 4: Siblings

### Problem
Find all siblings (nodes with same parent).

### Solution
```mangle
# Schema
Decl parent(Parent.Type<string>, Child.Type<string>).
Decl sibling(A.Type<string>, B.Type<string>).

# Two nodes are siblings if they share a parent and are different
sibling(X, Y) :-
  parent(P, X),
  parent(P, Y),
  X != Y.
```

### Example
```mangle
parent("A", "B").
parent("A", "C").
parent("A", "D").
parent("B", "E").

# Results:
# sibling("B", "C"), sibling("C", "B")
# sibling("B", "D"), sibling("D", "B")
# sibling("C", "D"), sibling("D", "C")
# NOT sibling("B", "E") - different generations
```

## Variation 5: Lowest Common Ancestor (LCA)

### Problem
Find the lowest common ancestor of two nodes.

### Solution
```mangle
# Schema
Decl parent(Parent.Type<string>, Child.Type<string>).
Decl ancestor(A.Type<string>, D.Type<string>).
Decl common_ancestor(A.Type<string>, B.Type<string>, Anc.Type<string>).
Decl depth(Node.Type<string>, D.Type<int>).
Decl lca(A.Type<string>, B.Type<string>, Anc.Type<string>).

# Build ancestor relation
ancestor(X, Y) :- parent(X, Y).
ancestor(X, Z) :- ancestor(X, Y), parent(Y, Z).

# Common ancestors
common_ancestor(A, B, Anc) :- ancestor(Anc, A), ancestor(Anc, B).

# Calculate depths (from earlier pattern)
root(X) :- parent(X, _), not parent(_, X).
depth(X, 0) :- root(X).
depth(Child, D) :-
  parent(Parent, Child),
  depth(Parent, PD),
  D = fn:plus(PD, 1).

# LCA is the deepest common ancestor
lca(A, B, Anc) :-
  common_ancestor(A, B, Anc),
  depth(Anc, D)
  |> do fn:group_by(A, B),
     let MaxDepth = fn:Max(D),
  depth(Anc, MaxDepth).
```

### Example
```mangle
parent("A", "B").
parent("A", "C").
parent("B", "D").
parent("B", "E").
parent("C", "F").

# Query: lca("D", "E", X)
# Answer: X = "B"

# Query: lca("D", "F", X)
# Answer: X = "A"
```

## Variation 6: Path to Root

### Problem
Find the path from any node to the root.

### Solution
```mangle
# Schema
Decl parent(Parent.Type<string>, Child.Type<string>).
Decl path_to_root(Node.Type<string>, Ancestor.Type<string>, Distance.Type<int>).
Decl root(Node.Type<string>).

# Root definition
root(X) :- parent(X, _), not parent(_, X).

# Node is distance 0 from itself
path_to_root(X, X, 0).

# Walk up to parent
path_to_root(Node, Ancestor, Dist) :-
  parent(Parent, Node),
  path_to_root(Parent, Ancestor, ParentDist),
  Dist = fn:plus(ParentDist, 1).
```

### Example
```mangle
parent("A", "B").
parent("B", "C").
parent("C", "D").

# Query: path_to_root("D", Anc, Dist)
# Results:
# path_to_root("D", "D", 0)
# path_to_root("D", "C", 1)
# path_to_root("D", "B", 2)
# path_to_root("D", "A", 3)
```

## Variation 7: N-ary Tree Children Count

### Problem
Count how many children each node has.

### Solution
```mangle
# Schema
Decl parent(Parent.Type<string>, Child.Type<string>).
Decl child_count(Parent.Type<string>, Count.Type<int>).

child_count(Parent, Count) :-
  parent(Parent, Child)
  |> do fn:group_by(Parent),
     let Count = fn:Count().

# Include nodes with zero children
Decl node(N.Type<string>).
node(X) :- parent(X, _).
node(X) :- parent(_, X).

Decl child_count_with_zero(Node.Type<string>, Count.Type<int>).

child_count_with_zero(Node, Count) :- child_count(Node, Count).
child_count_with_zero(Node, 0) :- node(Node), not parent(Node, _).
```

### Example
```mangle
parent("A", "B").
parent("A", "C").
parent("A", "D").
parent("B", "E").

# Results:
# child_count_with_zero("A", 3)
# child_count_with_zero("B", 1)
# child_count_with_zero("C", 0)
# child_count_with_zero("D", 0)
# child_count_with_zero("E", 0)
```

## Anti-Patterns

### WRONG: Confusing Parent and Child Order
```mangle
# Backwards!
ancestor(Child, Parent) :- parent(Parent, Child).
# Always check: ancestor(older, younger) or ancestor(younger, older)?
# Convention: ancestor(ANCESTOR, descendant)
```

### WRONG: Infinite Recursion on Self-Loops
```mangle
parent("A", "A").  # Self-loop
ancestor(X, Y) :- parent(X, Y).
ancestor(X, Z) :- ancestor(X, Y), parent(Y, Z).
# Result: Infinite derivation of ancestor("A", "A")
# Fix: Add X != Y checks or ensure no self-loops in data
```

### WRONG: Missing Base Case
```mangle
# Only recursive case!
descendant(X, Z) :- descendant(X, Y), parent(Y, Z).
# Result: Empty predicate (nothing to build from)
```

## Performance Tips

1. **Materialize Depth First**: If doing multiple level-based queries, compute depth once
2. **Use Subtree Predicates**: Don't recompute transitive closure per query
3. **Index on Parent**: Most queries walk up from child to parent
4. **Bound Recursion**: For deep trees, add depth limits

## Common Use Cases in codeNERD

### File System Hierarchy
```mangle
Decl file_parent(Parent.Type<string>, Child.Type<string>).
Decl in_directory(Dir.Type<string>, File.Type<string>).

in_directory(Dir, Dir).
in_directory(Dir, File) :-
  in_directory(Dir, Subdir),
  file_parent(Subdir, File).
```

### Class Inheritance
```mangle
Decl extends(Child.Type<string>, Parent.Type<string>).
Decl inherits(Class.Type<string>, Ancestor.Type<string>).

inherits(C, P) :- extends(C, P).
inherits(C, A) :- inherits(C, P), extends(P, A).
```

### AST Node Traversal
```mangle
Decl ast_child(Parent.Type<int>, Child.Type<int>, Index.Type<int>).
Decl ast_descendant(Ancestor.Type<int>, Descendant.Type<int>).

ast_descendant(P, C) :- ast_child(P, C, _).
ast_descendant(A, D) :- ast_descendant(A, N), ast_child(N, D, _).
```
