# Mangle Schema Template
# Extensional Database (EDB) and derived-predicate declarations.
#
# Usage: copy this file and starter-policy.mg, then customize for your domain. The two load as
# one unit (every predicate is declared exactly once, here). Check them together:
#   cat starter-schema.mg starter-policy.mg > program.mg
#   nerd check-mangle --standalone --eval reachable,sink_node program.mg
#
# Engine: codeberg.org/TauCeti/mangle-go v0.5.1-0.20260413190942-4dcaa582c6d3 (pinned by codeNERD).
# One bound per argument; the type functions are capitalised (fn:List, fn:Map, fn:Struct, ...).

# =============================================================================
# SECTION 1: Core Entity Predicates
# =============================================================================

Decl node(ID)
  descr [doc("A node in the graph")] bound [/name].

Decl edge(From, To)
  descr [doc("Directed edge between nodes")] bound [/name, /name].

# =============================================================================
# SECTION 2: Attribute Predicates
# =============================================================================

Decl node_label(ID, Label)
  descr [doc("Human-readable label for a node")] bound [/name, /string].

Decl node_type(ID, Type)
  descr [doc("Classification type for a node")] bound [/name, /name].

# Integer weights: comparisons (<, >) are integer-only on this engine.
Decl edge_weight(From, To, Weight)
  descr [doc("Numeric weight for an edge")] bound [/name, /name, /number].

# =============================================================================
# SECTION 3: Relationship Predicates
# =============================================================================

Decl parent(Parent, Child)
  descr [doc("Parent-child relationship")] bound [/name, /name].

Decl owns(Owner, Resource)
  descr [doc("Ownership relationship")] bound [/name, /name].

# =============================================================================
# SECTION 4: Time (integer timestamps; codeNERD's kernel has no temporal store)
# =============================================================================

Decl valid_from(Entity, Timestamp)
  descr [doc("When an entity became valid")] bound [/name, /number].

Decl valid_until(Entity, Timestamp)
  descr [doc("When an entity expires")] bound [/name, /number].

Decl current_time(Now)
  descr [doc("The evaluation's notion of now, asserted by the host")] bound [/number].

# =============================================================================
# SECTION 5: Structured Data Predicates
# =============================================================================

Decl metadata(Entity, Data)
  descr [doc("Key-value metadata for entities")] bound [/name, fn:Map(/string, /string)].

Decl tags(Entity, Tags)
  descr [doc("List of tags for an entity")] bound [/name, fn:List(/string)].

# =============================================================================
# SECTION 6: Derived Predicates (IDB, computed in starter-policy.mg)
# =============================================================================

Decl reachable(From, To) bound [/name, /name].
Decl ancestor(Ancestor, Descendant) bound [/name, /name].
Decl path(From, To, Nodes) bound [/name, /name, fn:List(/name)].
Decl has_out_edge(N) bound [/name].
Decl has_in_edge(N) bound [/name].
Decl has_parent(N) bound [/name].
Decl has_child(N) bound [/name].
Decl sink_node(N) bound [/name].
Decl source_node(N) bound [/name].
Decl isolated_node(N) bound [/name].
Decl leaf_node(N) bound [/name].
Decl root_node(N) bound [/name].
Decl internal_node(N) bound [/name].
Decl sibling(X, Y) bound [/name, /name].
Decl cousin(X, Y) bound [/name, /name].
Decl has_metadata_key(Entity, Key) bound [/name, /string].
Decl has_tag(Entity, Tag) bound [/name, /string].
Decl currently_valid(Entity) bound [/name].
Decl expired(Entity) bound [/name].
Decl has_cycle(N) bound [/name].
Decl orphan_edge(From, To) bound [/name, /name].

# =============================================================================
# SECTION 7: Aggregation Result Predicates
# =============================================================================

Decl node_count(Count) bound [/number].
Decl edge_count_by_source(Source, Count) bound [/name, /number].
Decl highly_connected(Node, Degree) bound [/name, /number].
Decl total_weight(Node, Total) bound [/name, /number].
Decl avg_degree(Avg) bound [/float64].
