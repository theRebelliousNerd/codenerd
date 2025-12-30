# JIT Compiler Logic (The "Gatekeeper")
# Determines which atoms are selected for the final prompt.

# =============================================================================
# IDB (Derived) Predicate Declarations
# =============================================================================

# Context matching helpers
Decl has_constraint(Atom, Dim).
Decl satisfied_constraint(Atom, Dim).
Decl blocked_by_context(Atom).

# Selection predicates
Decl mandatory_selection(Atom).
Decl prohibited(Atom).
Decl candidate_selection(Atom, Score).

# Conflict resolution
Decl beats(A, B).
Decl suppressed(Atom).

# Dependency resolution
Decl tentative(Atom).
Decl missing_dep(Atom).
Decl invalid(Atom).

# Final output
Decl final_valid(Atom).
Decl selected_result(Atom, Priority, Source).

# =============================================================================

# --- 1. SKELETON (Deterministic Selection) ---

# Context Matching Helper
# An atom matches context if ALL its tag dimensions align with current_context.
# (Logic: For every tag dimension D required by Atom, current_context must have a matching tag).
# This is tricky in Datalog without "forall".
# Simplified Approach: An atom is "mismatched" if it has a tag that CONTRADICTS current context.
# Assuming atom_tag implies "Required".

# Helper: Atom has a tag in Dimension D, but context has a DIFFERENT tag in Dimension D.
# (Implicitly assuming single-value per dimension in context, identifying mismatch).
# tag_mismatch(Atom) :-
#     atom_tag(Atom, Dim, Tag),
#     current_context(Dim, CtxTag),
#     Tag != CtxTag.
    
# Better Approach: Positive Matching
# An atom matches if it is NOT mismatched.
# matches_context(Atom) :-
#     atom(Atom),
#     !tag_mismatch(Atom).

# Wait, tags can be multi-valued (e.g., supports /go AND /python).
# So: Mismatch is if Atom defines a set of tags for Dim, and Context has a tag for Dim, 
# but Context's tag is NOT in Atom's set.
# This requires knowing if Atom HAS a constraint on Dim.

has_constraint(Atom, Dim) :- atom_tag(Atom, Dim, _).

satisfied_constraint(Atom, Dim) :-
    atom_tag(Atom, Dim, Tag),
    current_context(Dim, Tag).
    
# An atom is blocked only if context EXPLICITLY has a different value for Dim.
# If context doesn't specify a dimension at all, atoms with that dimension pass through.
# This prevents atoms from being blocked when their dimension isn't relevant to current context.
blocked_by_context(Atom) :-
    has_constraint(Atom, Dim),
    current_context(Dim, _),
    !satisfied_constraint(Atom, Dim).

# Safe Skeleton: Mandatory atoms that are NOT blocked.
mandatory_selection(Atom) :-
    is_mandatory(Atom),
    !blocked_by_context(Atom).

# --- 2. EXCLUSION (The Firewall) ---

# Explicit prohibitions (e.g., safety rules)
prohibited(Atom) :-
    atom_tag(Atom, /mode, /active),
    atom_tag(Atom, /tag, /dream_only).
    
# Dependency-based prohibition
prohibited(Atom) :-
    atom_requires(Atom, Dep),
    prohibited(Dep).

# Conflict-based suppression
# If A and B conflict, and A is mandatory, prohibited B.
prohibited(B) :-
    atom_conflicts(A, B),
    mandatory_selection(A).
    
# --- 3. FLESH (Probabilistic Selection) ---

# Candidates from Vector Search
# Must match context, not be prohibited, and score high enough.
candidate_selection(Atom, Score) :-
    vector_hit(Atom, Score),
    !blocked_by_context(Atom),
    !prohibited(Atom).

# --- 4. CONFLICT RESOLUTION (Score-Based) ---

# Conflict: A beats B if they conflict and A has higher score.
# If scores equal, break tie using atom ID (lexicographical).
beats(A, B) :-
    atom_conflicts(A, B),
    candidate_selection(A, ScoreA),
    candidate_selection(B, ScoreB),
    ScoreA > ScoreB.

beats(A, B) :-
    atom_conflicts(A, B),
    candidate_selection(A, Score),
    candidate_selection(B, Score),
    A < B. # Lexicographical tie-breaker

# Atom is suppressed if something beats it.
suppressed(Atom) :- beats(_, Atom).

# --- 5. DEPENDENCY RESOLUTION (Recursive) ---

# Tentative Selection: Mandatory OR Candidate (if not suppressed)
tentative(Atom) :- mandatory_selection(Atom).
tentative(Atom) :- candidate_selection(Atom, _), !suppressed(Atom).

# Recursive dependency inclusion: If A is selected, Dep must be selected.
# This expands the set to include dependencies.
# Note: This might pull in atoms that were NOT in candidates.
# We must ensure pulled-in deps are not prohibited.
tentative(Dep) :-
    tentative(Atom),
    atom_requires(Atom, Dep),
    !prohibited(Dep).

# Missing Dependency Check:
# An atom has a missing dependency if it requires Dep, 
# but Dep is NOT in the tentative set (perhaps prohibited or filtered).
missing_dep(Atom) :-
    tentative(Atom),
    atom_requires(Atom, Dep),
    !tentative(Dep).

# Iterate validity: An atom is invalid if it has a missing dep.
# This handles chains: A->B->C. If C missing, B invalid, then A invalid.
invalid(Atom) :- missing_dep(Atom).

# A parent is invalid if it requires an invalid child.
invalid(Atom) :-
    tentative(Atom),
    atom_requires(Atom, Dep),
    invalid(Dep).

# --- 6. FINAL OUTPUT ---

# Valid Selection: Tentative AND NOT Invalid
final_valid(Atom) :-
    tentative(Atom),
    !invalid(Atom).

# Report selected atoms for Go Assembly
# selected_result(Atom, Priority, Source)
selected_result(Atom, Prio, /skeleton) :-
    final_valid(Atom),
    atom_priority(Atom, Prio),
    mandatory_selection(Atom).

selected_result(Atom, Prio, /flesh) :-
    final_valid(Atom),
    atom_priority(Atom, Prio),
    !mandatory_selection(Atom).
