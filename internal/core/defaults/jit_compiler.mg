# JIT Compiler Logic (The "Gatekeeper")
# Determines which atoms are selected for the final prompt.

# =============================================================================
# IDB (Derived) Predicate Declarations
# =============================================================================

# Context matching helpers
Decl has_constraint(Atom, Dim).
Decl satisfied_constraint(Atom, Dim).
Decl blocked_by_context(Atom).
Decl regime_dimension(Dim) bound [/name].

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
# TODO: Verify if all new atom types implicitly require their tags as constraints.
    current_context(Dim, Tag).
    
# An atom is blocked only if context EXPLICITLY has a different value for Dim.
# If context doesn't specify a dimension at all, atoms with that dimension pass through.
# This prevents atoms from being blocked when their dimension isn't relevant to current context.
#
# This permissive default is correct for SITUATIONAL dimensions (language,
# framework, campaign phase): "no language in context" should not suppress an
# atom that happens to mention Go.
blocked_by_context(Atom) :-
    has_constraint(Atom, Dim),
    current_context(Dim, _),
    !satisfied_constraint(Atom, Dim).

# ...but a REGIME dimension is not situational. It answers "which workflow is
# this compile part of", and the honest answer for a compile that never set the
# dimension is "not that one" -- so these are fail-closed.
#
# /shard   -- which persona is speaking
# /mode    -- which operating mode (active, dream, thunderdome, ...)
# /phase   -- which campaign phase
# /layer   -- which build layer
# /init_phase, /northstar_phase, /ouroboros_stage -- which step of a wizard
#
# Under the permissive rule alone, a compile that set none of these admitted
# EVERY atom gated on them. Observed live: one `explain this file` turn compiled
# 114 mandatory atoms / ~60k tokens carrying 25+ contradictory identities
# (Nemesis, Coder, Tester, Legislator, Perception Firewall, the Ouroboros Tool
# Generator, the Northstar wizard's "thought partner", ...). The model obeyed
# the Perception Layer persona it found there -- "you describe what the user
# wants, the harness fulfills it" -- and answered with an intent announcement
# instead of doing the work. Identity leakage does not degrade a prompt, it
# replaces the agent.
#
# Every one of these dimensions has a real producer, so the workflows that need
# their own atoms still get them: /shard from the assembler, /phase from the
# session context, /init_phase from internal/init/jit_integration.go,
# /northstar_phase from cmd/nerd/chat/northstar_llm.go, /ouroboros_stage and
# /layer from SessionContext.ExtraContext.
#
# /intent and /lang are deliberately NOT here. /lang is a relevance hint, and
# /intent gates the bulk of useful flesh -- an atom that mentions `/fix` is
# still worth reading on a turn whose verb has not been resolved.
#
# Go's matchSelector already implements exactly this (a constraint with no
# context value returns false), but it only runs in fallbackFleshSelection --
# the path used when Mangle is unavailable. This makes the live kernel path
# agree with it.
regime_dimension(/shard).
regime_dimension(/mode).
regime_dimension(/phase).
regime_dimension(/layer).
regime_dimension(/init_phase).
regime_dimension(/northstar_phase).
regime_dimension(/ouroboros_stage).

blocked_by_context(Atom) :-
    regime_dimension(Dim),
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
# Policy veto bridge (restrictive direction only): wire policy/jit_selection.mg as
# vetoes, not admissions. prohibited_atom and conflict_loser are the two things
# policy/jit_selection.mg owns that this file does not have -- a firewall that
# propagates through atom_requires, and conflict resolution that lets a mandatory
# atom beat a candidate. Both are vetoes. Wiring them makes the ruleset live on
# every compile while moving the atom count down or equal, never up. A second
# opinion in a selector should be able to veto, not to admit.
# Stratification: no cycle -- prohibited_atom derives from base_prohibited over
# compile_context/atom_tag/atom_requires, and conflict_loser derives from
# candidate_atom/mandatory_atom/atom_conflicts/prompt_atom. Nothing in either
# chain reads this file's prohibited or suppressed.
prohibited(Atom) :- prohibited_atom(Atom).
suppressed(Atom) :- conflict_loser(Atom).

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
