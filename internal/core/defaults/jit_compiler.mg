# JIT Compiler Logic (The "Gatekeeper")
# Determines which atoms are selected for the final prompt.

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
    
# An atom is blocked if it has a constraint on Dim, but constraint is not satisfied.
blocked_by_context(Atom) :-
    has_constraint(Atom, Dim),
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
    
# --- 4. LINKING (Dependency Resolution) ---

# If A is selected, Dep is selected.
final_selection(Atom) :- mandatory_selection(Atom).
final_selection(Atom) :- candidate_selection(Atom, _).

# Recursive dependency selection
final_selection(Dep) :-
    final_selection(Atom),
    atom_requires(Atom, Dep).
    
# Safety check: Remove if prohibited (overrules selection)
# This handles the case where a dependency pulls in a prohibited atom -> parent must be dropped.
# This logic is circular if not careful. "Stratified Datalog" usually handles this.
# For now, we assume dependencies are verified safe at ingest time or handled by "prohibited" check above.

# Valid Selection Set
valid_atom(Atom) :-
    final_selection(Atom),
    !prohibited(Atom).

# --- 5. EXCLUSION GROUPS (Mutual Exclusion) ---

# Identify best score in group.
# group_best_score(GroupID, MaxScore) :- ... (Hard in pure Datalog without aggregation)
# Go runtime often handles the "pick best from group" final step.
# We will flag them for Go.

# --- 6. OUTPUTS ---

# Report selected atoms for Go Assembly
# selected_result(Atom, Priority, Source)
selected_result(Atom, Prio, /skeleton) :-
    valid_atom(Atom),
    atom_priority(Atom, Prio),
    mandatory_selection(Atom).

selected_result(Atom, Prio, /flesh) :-
    valid_atom(Atom),
    atom_priority(Atom, Prio),
    !mandatory_selection(Atom).
