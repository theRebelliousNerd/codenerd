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
Decl node(ID)
    descr [doc("A node in the graph")]
    bound [/name].

Decl edge(From, To)
    descr [doc("Directed edge between nodes")]
    bound [/name, /name].

# =============================================================================
# SECTION 2: Attribute Predicates
# =============================================================================

# Define attributes for your entities
Decl node_label(ID, Label)
    descr [doc("Human-readable label for a node")]
    bound [/name, /string].

Decl node_type(ID, Type)
    descr [doc("Classification type for a node")]
    bound [/name, /name].

Decl edge_weight(From, To, Weight)
    descr [doc("Numeric weight for an edge")]
    bound [/name, /name, /number].

# =============================================================================
# SECTION 3: Relationship Predicates
# =============================================================================

# Define relationships between entities
Decl parent(Parent, Child)
    descr [doc("Parent-child relationship")]
    bound [/name, /name].

Decl owns(Owner, Resource)
    descr [doc("Ownership relationship")]
    bound [/name, /name].

# =============================================================================
# SECTION 4: Temporal Predicates (if needed)
# =============================================================================

# Define time-based facts
Decl valid_from(Entity, Timestamp)
    descr [doc("When an entity became valid")]
    bound [/name, /number].

Decl valid_until(Entity, Timestamp)
    descr [doc("When an entity expires")]
    bound [/name, /number].

# =============================================================================
# SECTION 5: Structured Data Predicates
# =============================================================================

# Define predicates that hold structured data
# Note: Complex struct types require proper Mangle struct syntax
Decl metadata(Entity, Key, Value)
    descr [doc("Key-value metadata for entities")]
    bound [/name, /string, /string].

Decl has_tag(Entity, Tag)
    descr [doc("Tag for an entity")]
    bound [/name, /name].

# =============================================================================
# SECTION 6: Derived Predicate Declarations (IDB)
# =============================================================================

# Declare predicates that will be computed by rules in policy.mg
Decl reachable(From, To)
    descr [doc("Transitive closure of edges")]
    bound [/name, /name].

Decl ancestor(Ancestor, Descendant)
    descr [doc("Transitive closure of parent relationship")]
    bound [/name, /name].

# =============================================================================
# SECTION 7: Aggregation Result Predicates
# =============================================================================

# Declare predicates that hold aggregation results
Decl node_count(Count)
    descr [doc("Total number of nodes")]
    bound [/number].

Decl edge_count_by_source(Source, Count)
    descr [doc("Number of outgoing edges per source node")]
    bound [/name, /number].
