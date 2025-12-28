# Mangle Policy Template
# Intensional Database (IDB) Rules
# These define derived facts computed from base facts in schemas.mg
#
# Usage: Copy this file and customize rules for your domain.
# Mangle v0.4.0 compatible

# =============================================================================
# SECTION 1: Transitive Closure Rules
# =============================================================================

# Reachability - the most common recursive pattern
# Base case: direct connection
reachable(X, Y) :- edge(X, Y).
# Recursive case: indirect connection
reachable(X, Z) :- edge(X, Y), reachable(Y, Z).

# Ancestor relationship
ancestor(A, D) :- parent(A, D).
ancestor(A, D) :- parent(A, C), ancestor(C, D).

# =============================================================================
# SECTION 2: Path Existence Rules
# =============================================================================

# Check if a path exists (without tracking the actual path)
# Note: For path tracking with lists, see advanced patterns
path_exists(Start, End) :- edge(Start, End).
path_exists(Start, End) :-
    edge(Start, Mid),
    path_exists(Mid, End).

# =============================================================================
# SECTION 3: Negation Patterns (Set Difference)
# =============================================================================

# Find nodes with no outgoing edges (sinks)
sink_node(N) :- node(N), !edge(N, _).

# Find nodes with no incoming edges (sources)
source_node(N) :- node(N), !edge(_, N).

# Find isolated nodes (no edges at all)
isolated_node(N) :- node(N), !edge(N, _), !edge(_, N).

# =============================================================================
# SECTION 4: Aggregation Rules
# =============================================================================

# Count all nodes
node_count(N) :-
    node(_) |>
    do fn:group_by(),
    let N = fn:count().

# Count outgoing edges per node
edge_count_by_source(Src, Count) :-
    edge(Src, _) |>
    do fn:group_by(Src),
    let Count = fn:count().

# Find nodes with most connections
highly_connected(Node, Degree) :-
    edge_count_by_source(Node, Degree),
    Degree > 5.

# Sum weights for a node's outgoing edges
total_weight(Node, Total) :-
    edge_weight(Node, _, W) |>
    do fn:group_by(Node),
    let Total = fn:sum(W).

# =============================================================================
# SECTION 5: Classification Rules
# =============================================================================

# Classify nodes based on properties
leaf_node(N) :- node(N), parent(_, N), !parent(N, _).
root_node(N) :- node(N), !parent(_, N), parent(N, _).
internal_node(N) :- node(N), parent(_, N), parent(N, _).

# =============================================================================
# SECTION 6: Sibling and Peer Rules
# =============================================================================

# Find siblings (same parent, different identity)
sibling(X, Y) :- parent(P, X), parent(P, Y), X != Y.

# Find cousins (parents are siblings)
cousin(X, Y) :- parent(PX, X), parent(PY, Y), sibling(PX, PY).

# =============================================================================
# SECTION 7: Structured Data Access Rules
# =============================================================================

# Check if entity has specific metadata key
has_metadata_key(Entity, Key) :-
    metadata(Entity, Key, _).

# =============================================================================
# SECTION 8: Temporal Rules
# =============================================================================

# Declare current_time as an EDB fact (must be asserted externally)
# Check if entity is currently valid
currently_valid(Entity) :-
    valid_from(Entity, Start),
    valid_until(Entity, End),
    current_time(Now),
    Start <= Now,
    Now <= End.

# Find expired entities
expired(Entity) :-
    valid_until(Entity, End),
    current_time(Now),
    End < Now.

# =============================================================================
# SECTION 9: Safety and Validation Rules
# =============================================================================

# Detect cycles (node reachable from itself)
has_cycle(N) :- reachable(N, N).

# Validate that all edges reference existing nodes
orphan_edge(From, To) :-
    edge(From, To),
    !node(From).

orphan_edge(From, To) :-
    edge(From, To),
    !node(To).

# =============================================================================
# SECTION 10: Multi-Stage Aggregation Example
# =============================================================================

# Average degree of nodes
avg_degree(Avg) :-
    edge_count_by_source(_, Count) |>
    do fn:group_by(),
    let Total = fn:sum(Count),
    let Num = fn:count() |>
    let Avg = fn:float:div(Total, Num).
