# Mangle Performance Anti-Patterns
# Examples of common performance issues for testing profile_rules.py

# ============================================================================
# ANTI-PATTERN 1: Cartesian Product (HIGH RISK)
# ============================================================================
# Problem: Predicates with independent variables create cross-product before
# the connecting predicate filters them down.

# BAD: Spatial relationship check
# interactable has ~500 items, so this creates 500 × 500 = 250K combinations
# before the geometry check can filter them
left_of(A, B) :-
    interactable(A, _),
    interactable(B, _),
    geometry(A, Ax, _, _, _),
    geometry(B, Bx, _, _, _),
    Ax < Bx.

# BETTER: Add early constraint
# left_of_better(A, B) :-
#     interactable(A, _),
#     interactable(B, _),
#     A != B,  # Filter early
#     geometry(A, Ax, _, _, _),
#     geometry(B, Bx, _, _, _),
#     Ax < Bx.

# ============================================================================
# ANTI-PATTERN 2: Late Filtering (MEDIUM RISK)
# ============================================================================
# Problem: Comparisons after expensive joins

# BAD: Filter after joins
expensive_join(X, Y, Z) :-
    table_a(X),
    table_b(Y),
    table_c(Z),
    X < Y,
    Y < Z.

# BETTER: Filter as early as possible
# efficient_join(X, Y, Z) :-
#     table_a(X),
#     table_b(Y),
#     X < Y,  # Filter early
#     table_c(Z),
#     Y < Z.

# ============================================================================
# ANTI-PATTERN 3: Unbounded Recursion (HIGH RISK)
# ============================================================================
# Problem: No base case or depth limit

# BAD: No explicit base case visible
all_paths(X, Y) :- edge(X, Y).
all_paths(X, Z) :- edge(X, Y), all_paths(Y, Z).
# Could explode on cyclic graphs or dense graphs

# BETTER: With explicit termination
# all_paths_bounded(X, Y, 0) :- edge(X, Y).
# all_paths_bounded(X, Z, Depth) :-
#     Depth < 10,  # Depth limit
#     edge(X, Y),
#     all_paths_bounded(Y, Z, D1),
#     Depth = D1 + 1.

# ============================================================================
# ANTI-PATTERN 4: Late Negation (MEDIUM RISK)
# ============================================================================
# Problem: Negation after expensive operations

# BAD: Negation after joins
filtered_result(X) :-
    big_table_1(X),
    big_table_2(X),
    big_table_3(X),
    !excluded(X).

# BETTER: Check negation early (if safe)
# filtered_result_better(X) :-
#     big_table_1(X),
#     !excluded(X),  # Filter early
#     big_table_2(X),
#     big_table_3(X).

# ============================================================================
# GOOD PATTERN: Filter First, Then Join
# ============================================================================
# This is the correct way to structure queries

efficient_query(X, Y) :-
    filter(X, Y),      # Filter binds both X and Y (100 rows)
    table_a(X),        # Verify X exists in table_a
    table_b(Y).        # Verify Y exists in table_b
# Cost: 100 filter results × 2 lookups = 200 operations

# ============================================================================
# GOOD PATTERN: Recursive with Base Case
# ============================================================================
# Proper recursion structure

reachable(X) :- start(X).                      # Base case
reachable(Y) :- reachable(X), edge(X, Y).      # Recursive case
# Has clear base case, safe

# ============================================================================
# ANTI-PATTERN 5: Multiple Independent Variables
# ============================================================================
# Problem: Three independent variables

# BAD: A, B, C all independent until relate
very_slow(A, B, C) :-
    pred_1(A),         # 5K rows
    pred_2(B),         # 5K rows
    pred_3(C),         # 5K rows
    relate(A, B, C).   # Only 100 matches
# Cost: 5K × 5K × 5K = 125 billion combinations!

# BETTER: Use join table first
# very_fast(A, B, C) :-
#     relate(A, B, C),   # 100 rows with all three variables
#     pred_1(A),         # Verify A
#     pred_2(B),         # Verify B
#     pred_3(C).         # Verify C
# Cost: 100 × 3 = 300 lookups
