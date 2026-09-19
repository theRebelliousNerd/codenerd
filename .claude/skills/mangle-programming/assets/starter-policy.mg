# Mangle Policy Template
# Intensional Database (IDB) rules over the declarations in starter-schema.mg.
#
# Usage: copy this file and starter-schema.mg; the two load as one unit. Check them together:
#   cat starter-schema.mg starter-policy.mg > program.mg
#   nerd check-mangle --standalone --eval reachable,sink_node,leaf_node program.mg
#
# Engine: codeberg.org/TauCeti/mangle-go v0.5.1-0.20260413190942-4dcaa582c6d3 (pinned by codeNERD).

# =============================================================================
# SECTION 1: Transitive Closure Rules
# =============================================================================

# Reachability - the most common recursive pattern
reachable(X, Y) :- edge(X, Y).
reachable(X, Z) :- edge(X, Y), reachable(Y, Z).

# Ancestor relationship
ancestor(A, D) :- parent(A, D).
ancestor(A, D) :- parent(A, C), ancestor(C, D).

# =============================================================================
# SECTION 2: Path Construction Rules
# =============================================================================

# fn:list:cons prepends; there is no [Head | Tail] syntax. Every longer path is a new
# fact, so this terminates only on an acyclic graph: check has_cycle (section 9) first.
path(Start, End, [Start, End]) :- edge(Start, End).
path(Start, End, Full) :-
    edge(Start, Mid),
    path(Mid, End, Rest),
    Full = fn:list:cons(Start, Rest).

# =============================================================================
# SECTION 3: Negation Patterns (Set Difference)
# =============================================================================

# Project first, then negate the projection. A wildcard inside a negated atom
# (`!edge(N, _)`) is silently deleted by the engine, and every node would match.
has_out_edge(N) :- edge(N, _).
has_in_edge(N) :- edge(_, N).

# Nodes with no outgoing edges (sinks)
sink_node(N) :- node(N), !has_out_edge(N).

# Nodes with no incoming edges (sources)
source_node(N) :- node(N), !has_in_edge(N).

# Isolated nodes (no edges at all)
isolated_node(N) :- node(N), !has_out_edge(N), !has_in_edge(N).

# =============================================================================
# SECTION 4: Aggregation Rules (name every column in a transform rule's body)
# =============================================================================

node_count(N) :-
    node(Node) |>
    do fn:group_by(),
    let N = fn:count().

edge_count_by_source(Src, Count) :-
    edge(Src, Dst) |>
    do fn:group_by(Src),
    let Count = fn:count().

highly_connected(Node, Degree) :-
    edge_count_by_source(Node, Degree),
    Degree > 5.

total_weight(Node, Total) :-
    edge_weight(Node, Dst, W) |>
    do fn:group_by(Node),
    let Total = fn:sum(W).

# =============================================================================
# SECTION 5: Classification Rules
# =============================================================================

has_parent(N) :- parent(_, N).
has_child(N) :- parent(N, _).

leaf_node(N) :- node(N), has_parent(N), !has_child(N).
root_node(N) :- node(N), has_child(N), !has_parent(N).
internal_node(N) :- node(N), has_parent(N), has_child(N).

# =============================================================================
# SECTION 6: Sibling and Peer Rules
# =============================================================================

sibling(X, Y) :- parent(P, X), parent(P, Y), X != Y.
cousin(X, Y) :- parent(PX, X), parent(PY, Y), sibling(PX, PY).

# =============================================================================
# SECTION 7: Structured Data Access Rules
# =============================================================================

# Map entries: :match_entry(Map, Key, Value) needs the map bound and the value free.
has_metadata_key(Entity, "owner") :-
    metadata(Entity, Data),
    :match_entry(Data, "owner", Value).

has_tag(Entity, Tag) :-
    tags(Entity, TagList),
    :list:member(Tag, TagList).

# =============================================================================
# SECTION 8: Time Rules (integer timestamps)
# =============================================================================

currently_valid(Entity) :-
    current_time(Now),
    valid_from(Entity, Start),
    valid_until(Entity, End),
    Start <= Now,
    Now <= End.

expired(Entity) :-
    current_time(Now),
    valid_until(Entity, End),
    End < Now.

# =============================================================================
# SECTION 9: Safety and Validation Rules
# =============================================================================

has_cycle(N) :- reachable(N, N).

orphan_edge(From, To) :- edge(From, To), !node(From).
orphan_edge(From, To) :- edge(From, To), !node(To).

# =============================================================================
# SECTION 10: Average (fn:avg yields a float; declare the column /float64)
# =============================================================================

avg_degree(Avg) :-
    edge_count_by_source(Src, Count) |>
    do fn:group_by(),
    let Avg = fn:avg(Count).
