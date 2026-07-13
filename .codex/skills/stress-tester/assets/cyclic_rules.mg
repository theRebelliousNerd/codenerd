# Cyclic Mangle Rules for Stress Testing
# WARNING: These rules are designed to cause derivation explosion!
# Use with caution - may exhaust memory and trigger gas limits.

# =============================================================================
# SCHEMA DECLARATIONS
# =============================================================================

Decl edge(from: name, to: name).
Decl node(n: name).
Decl reachable(from: name, to: name).
Decl path(from: name, to: name, length: num).
Decl connected(a: name, b: name).
Decl cycle_member(n: name).
Decl stress_fact(id: num, payload: name).

# =============================================================================
# BASE FACTS (EDB) - Creates a dense graph
# =============================================================================

# Create nodes
node(/n1). node(/n2). node(/n3). node(/n4). node(/n5).
node(/n6). node(/n7). node(/n8). node(/n9). node(/n10).
node(/n11). node(/n12). node(/n13). node(/n14). node(/n15).
node(/n16). node(/n17). node(/n18). node(/n19). node(/n20).

# Create dense edges (many paths between nodes)
edge(/n1, /n2). edge(/n2, /n3). edge(/n3, /n4). edge(/n4, /n5).
edge(/n5, /n6). edge(/n6, /n7). edge(/n7, /n8). edge(/n8, /n9).
edge(/n9, /n10). edge(/n10, /n1).  # Cycle back to start

edge(/n1, /n3). edge(/n2, /n4). edge(/n3, /n5). edge(/n4, /n6).
edge(/n5, /n7). edge(/n6, /n8). edge(/n7, /n9). edge(/n8, /n10).
edge(/n9, /n1). edge(/n10, /n2).  # More cycles

edge(/n1, /n5). edge(/n2, /n6). edge(/n3, /n7). edge(/n4, /n8).
edge(/n5, /n9). edge(/n6, /n10). edge(/n7, /n1). edge(/n8, /n2).
edge(/n9, /n3). edge(/n10, /n4).  # Cross edges

# Additional layer of nodes and edges
edge(/n11, /n12). edge(/n12, /n13). edge(/n13, /n14). edge(/n14, /n15).
edge(/n15, /n16). edge(/n16, /n17). edge(/n17, /n18). edge(/n18, /n19).
edge(/n19, /n20). edge(/n20, /n11).  # Second ring

# Connect the two rings
edge(/n1, /n11). edge(/n2, /n12). edge(/n3, /n13). edge(/n4, /n14).
edge(/n5, /n15). edge(/n6, /n16). edge(/n7, /n17). edge(/n8, /n18).
edge(/n9, /n19). edge(/n10, /n20).

edge(/n11, /n1). edge(/n12, /n2). edge(/n13, /n3). edge(/n14, /n4).
edge(/n15, /n5). edge(/n16, /n6). edge(/n17, /n7). edge(/n18, /n8).
edge(/n19, /n9). edge(/n20, /n10).

# =============================================================================
# EXPLOSIVE RULES (IDB) - These cause derivation explosion
# =============================================================================

# Rule 1: Transitive closure (quadratic explosion)
reachable(X, Y) :- edge(X, Y).
reachable(X, Z) :- reachable(X, Y), edge(Y, Z).

# Rule 2: Path with length (exponential in dense graphs)
path(X, Y, 1) :- edge(X, Y).
path(X, Z, N) :- path(X, Y, M), edge(Y, Z), N = M + 1, N < 20.

# Rule 3: Bidirectional connectivity (doubles the explosion)
connected(A, B) :- reachable(A, B).
connected(A, B) :- reachable(B, A).

# Rule 4: Cycle membership (identifies all nodes in cycles)
cycle_member(N) :- reachable(N, N).

# =============================================================================
# STRESS QUERY TARGETS
# =============================================================================

# Query these predicates to trigger derivation:
# - reachable: Should derive O(n^2) facts for n nodes
# - path: Should derive O(n^k) facts for paths up to length k
# - connected: Should be ~2x reachable
# - cycle_member: Should identify all 20 nodes

# =============================================================================
# INTENTIONALLY PROBLEMATIC RULES (Use at own risk!)
# =============================================================================

# Uncomment these for more extreme stress testing:

# Self-referential rule (will hit gas limit quickly)
# stress_fact(N, /payload) :- stress_fact(M, _), N = M + 1, N < 100000.
# stress_fact(0, /seed).

# Cartesian product (exponential blowup)
# Decl pair(a: name, b: name).
# pair(X, Y) :- node(X), node(Y).  # 400 facts from 20 nodes

# Triple product (cubic blowup)
# Decl triple(a: name, b: name, c: name).
# triple(X, Y, Z) :- node(X), node(Y), node(Z).  # 8000 facts from 20 nodes
