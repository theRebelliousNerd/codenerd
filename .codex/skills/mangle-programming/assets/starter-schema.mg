# Mangle Schema Template
# Extensional Database (EDB) Declarations
# These define the base facts that will be loaded from external sources.
#
# Usage: Copy this file and customize predicates for your domain.
# Mangle v0.4.0 compatible

# =============================================================================
# SECTION 1: Core Entity Predicates
# =============================================================================

# Define your primary entities here
# Example: Decl entity(ID.Type<n>, Name.Type<string>).

Decl node(ID.Type<n>)
  descr [doc("A node in the graph")].

Decl edge(From.Type<n>, To.Type<n>)
  descr [doc("Directed edge between nodes")].

# =============================================================================
# SECTION 2: Attribute Predicates
# =============================================================================

# Define attributes for your entities
# Example: Decl entity_attr(ID.Type<n>, Value.Type<string>).

Decl node_label(ID.Type<n>, Label.Type<string>)
  descr [doc("Human-readable label for a node")].

Decl node_type(ID.Type<n>, Type.Type<n>)
  descr [doc("Classification type for a node")].

Decl edge_weight(From.Type<n>, To.Type<n>, Weight.Type<float>)
  descr [doc("Numeric weight for an edge")].

# =============================================================================
# SECTION 3: Relationship Predicates
# =============================================================================

# Define relationships between entities
# Example: Decl relates(Entity1.Type<n>, Entity2.Type<n>, Relation.Type<n>).

Decl parent(Parent.Type<n>, Child.Type<n>)
  descr [doc("Parent-child relationship")].

Decl owns(Owner.Type<n>, Resource.Type<n>)
  descr [doc("Ownership relationship")].

# =============================================================================
# SECTION 4: Temporal Predicates (if needed)
# =============================================================================

# Define time-based facts
# Example: Decl event(ID.Type<n>, Timestamp.Type<int>, Type.Type<n>).

Decl valid_from(Entity.Type<n>, Timestamp.Type<int>)
  descr [doc("When an entity became valid")].

Decl valid_until(Entity.Type<n>, Timestamp.Type<int>)
  descr [doc("When an entity expires")].

# =============================================================================
# SECTION 5: Structured Data Predicates
# =============================================================================

# Define predicates that hold structured data (maps, lists, structs)
# Example: Decl config(Data.Type<{/key: string}>).

Decl metadata(Entity.Type<n>, Data.Type<{/key: string, /value: string}>)
  descr [doc("Key-value metadata for entities")].

Decl tags(Entity.Type<n>, Tags.Type<[n]>)
  descr [doc("List of tags for an entity")].

# =============================================================================
# SECTION 6: Derived Predicate Declarations (IDB)
# =============================================================================

# Declare predicates that will be computed by rules in policy.gl
# These should match the heads of rules defined elsewhere.

Decl reachable(From.Type<n>, To.Type<n>)
  descr [doc("Transitive closure of edges")].

Decl ancestor(Ancestor.Type<n>, Descendant.Type<n>)
  descr [doc("Transitive closure of parent relationship")].

# =============================================================================
# SECTION 7: Aggregation Result Predicates
# =============================================================================

# Declare predicates that hold aggregation results

Decl node_count(Count.Type<int>)
  descr [doc("Total number of nodes")].

Decl edge_count_by_source(Source.Type<n>, Count.Type<int>)
  descr [doc("Number of outgoing edges per source node")].
