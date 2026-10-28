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
Decl mandatory_superseded(Atom).
Decl prohibited(Atom).
Decl candidate_selection(Atom, Score).
Decl flesh_recall(Atom, Score).
Decl atom_relevance_band(Atom, Band) bound [/string, /name].
Decl relevance_judged(Atom) bound [/string].
Decl flesh_admitted(Atom, Row) bound [/string, /name].
Decl flesh_refused(Atom, Row) bound [/string, /name].
Decl refused_by_relevance(Atom) bound [/string].

# Conflict resolution
Decl beats(A, B).
Decl suppressed(Atom).

# Dependency resolution
Decl tentative(Atom).
Decl missing_dep(Atom).
Decl invalid(Atom).

# Final output
Decl final_valid(Atom).
Decl selected_result(Atom, Source).

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
# /provider and /model are the pin dimensions, and they are the strictest case
# of "not situational" in the list. An atom acquires a pin one of two ways:
# an author writes `providers:`/`models:` in its YAML, or the prompt evolution
# loop generates it from failures observed on one specific vendor and stamps
# the provenance onto it (internal/autopoiesis/prompt_evolution). In both cases
# the atom encodes a claim about how ONE model behaves -- a workaround for a
# tokenizer quirk, a refusal pattern, a tool-call format that only one vendor
# gets wrong. Serving that to a different model is not a weaker prompt, it is a
# wrong one: it spends budget teaching Claude to route around a Gemini defect.
#
# Fail-closed is therefore the only defensible default. A compile that never
# named its provider cannot show that the pinned atom's claim holds, so the
# atom does not run. The cost of the strict reading is bounded and visible (a
# pinned atom sits out a compile that forgot to set Provider); the cost of the
# permissive reading is silent and unbounded (every vendor-specific workaround
# in the corpus lands in every prompt). Unpinned atoms -- the overwhelming
# majority -- declare no tag on these dimensions and are untouched.
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
regime_dimension(/provider).
regime_dimension(/model).

# /state -- a world state (atom_tag(A, /state, S) from `world_states:`). A gate
# on a state serves the atom exactly when that state holds, measured by the
# executor or derived as a need (policy/jit_needs.mg); a compile that carries
# no state at all holds none of them. Go's pre-filter
# (atomMatchesActiveWorldState) already drops such atoms before this program
# sees them; until 2026-09-23 this program alone read /state as situational and
# would have admitted them.
regime_dimension(/state).

blocked_by_context(Atom) :-
    regime_dimension(Dim),
    has_constraint(Atom, Dim),
    !satisfied_constraint(Atom, Dim).

# Pure policy exclusions must bind mandatory selection and supersession too.
# base_prohibited depends only on input facts and dependency closure; using
# prohibited here would introduce a negative cycle through mandatory_selection.
blocked_by_context(Atom) :- base_prohibited(Atom).

# Supersession: a mandatory atom that another LIVE mandatory atom declares a
# conflict against steps aside for it.
#
# This exists because `prohibited` cannot do it. Read the consumers of that
# predicate below: it is joined only in candidate_selection (the vector/flesh
# path) and in the atom_requires pull-in. Nothing consults it for a mandatory
# atom. So the rule further down --
#
#     prohibited(B) :- atom_conflicts(A, B), mandatory_selection(A).
#
# -- derives B as prohibited and then has no effect on B whatsoever, because B
# reaches the output through mandatory_selection -> tentative -> final_valid,
# and no step on that path reads prohibited. Measured against a real kernel: a
# mandatory atom and a mandatory atom conflicting with it were BOTH returned in
# selected_result. The conflict was computed and discarded.
#
# The direct fix (adding !prohibited(Atom) to mandatory_selection) is not
# available: prohibited already depends on mandatory_selection, so that closes a
# cycle through negation and the program stops being stratifiable. Hence a
# separate predicate that derives only from EDB plus blocked_by_context, none of
# which reads mandatory_selection.
#
# The relation is DIRECTIONAL on purpose. atom_conflicts(A, B) means "A
# supersedes B", not "A and B are incompatible". A mutual pair would otherwise
# cancel both atoms and silently delete the guidance entirely -- the worst
# outcome available. The producer is responsible for emitting one direction.
#
# `is_mandatory(A)` rather than `mandatory_selection(A)` keeps the stratum
# clean, and `!blocked_by_context(A)` is what makes supersession conditional: a
# superseding atom that is itself blocked this compile (for example a
# model-pinned variant on the wrong model) does not take its base down with it.
# That is the property the marathon overlay depends on -- the optimized variant
# replaces the shipped atom on the model it was written for, and on every other
# model the shipped atom is still there, untouched.
#
# Live on the shipped corpus: the delegation judge's atoms
# (internal/prompt/atoms/eval/delegation_judge.yaml) supersede the Piggyback
# protocol and OODA methodology atoms, so a judge compile has no protocol or
# methodology atom by design; the selector reads this predicate to tell that
# shape from a skeleton selection defect (unexplainedSkeletonGaps).
mandatory_superseded(B) :-
    atom_conflicts(A, B),
    is_mandatory(A),
    !blocked_by_context(A).

# Safe Skeleton: Mandatory atoms that are NOT blocked and NOT superseded.
mandatory_selection(Atom) :-
    is_mandatory(Atom),
    !blocked_by_context(Atom),
    !mandatory_superseded(Atom).

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
    
# --- 3. FLESH (Recall, then relevance) ---

# Recall: the vector channel's hits. Go asserts vector_hit for the recall
# pool, and vector_slot for the hits the vector channel alone keeps (its
# relative floor and top-K). Without Jev the pool IS those slots.
flesh_recall(Atom, Score) :-
    vector_hit(Atom, Score),
    !blocked_by_context(Atom),
    !prohibited(Atom).

# Jev's per-turn reading of "Does this guidance apply to this request?"
# (internal/atomjudge), asserted as atom_relevance(Atom, Percent) only for an
# atom Jev answered about this turn, read against the turn's thresholds
# (typesafe judgment_yes_percent / judgment_no_percent).
atom_relevance_band(Atom, /applies) :-
    atom_relevance(Atom, P),
    atom_relevance_threshold(/yes, Yes),
    P >= Yes.
atom_relevance_band(Atom, /does_not_apply) :-
    atom_relevance(Atom, P),
    atom_relevance_threshold(/no, No),
    P <= No.
atom_relevance_band(Atom, /unsettled) :-
    atom_relevance(Atom, P),
    atom_relevance_threshold(/yes, Yes),
    atom_relevance_threshold(/no, No),
    P < Yes,
    P > No.
relevance_judged(Atom) :- atom_relevance(Atom, _).

# The admission table, one row per band. A missing judgment (Jev off, failed,
# late, or the atom not asked) is never a "no": it is the vector channel's
# decision, exactly as without Jev.
#   applies          -> admitted from anywhere in the recall pool
#   unsettled        -> admitted only from a vector slot
#   not judged       -> admitted only from a vector slot
#   does not apply   -> refused, even from a vector slot
flesh_admitted(Atom, /judged_applies) :-
    flesh_recall(Atom, _),
    atom_relevance_band(Atom, /applies).
flesh_admitted(Atom, /unsettled_vector_slot) :-
    flesh_recall(Atom, _),
    atom_relevance_band(Atom, /unsettled),
    vector_slot(Atom).
flesh_admitted(Atom, /unjudged_vector_slot) :-
    flesh_recall(Atom, _),
    vector_slot(Atom),
    !relevance_judged(Atom).
# Carried: an atom this task's previous judged compile admitted
# (flesh_carried, internal/prompt/relevance.go) stays unless Jev now reads it
# as not applying. Unsettled is not enough to drop it: the selection holds
# until the task changes or the relevance falls past the no threshold, so the
# prompt's bytes, and the provider's cached prefix, hold with it.
flesh_admitted(Atom, /carried) :-
    flesh_recall(Atom, _),
    flesh_carried(Atom),
    !refused_by_relevance(Atom).
refused_by_relevance(Atom) :-
    atom_relevance_band(Atom, /does_not_apply).
flesh_refused(Atom, /judged_does_not_apply) :-
    flesh_recall(Atom, _),
    atom_relevance_band(Atom, /does_not_apply).

candidate_selection(Atom, Score) :-
    flesh_admitted(Atom, _),
    flesh_recall(Atom, Score).

# Runtime knowledge and learnings do not live in the prompt-vector index.
# Their retrieval witness admits a candidate, while the same executive
# context, conflict, dependency, and final budget checks still apply. The
# retrieval already matched them to this task, so they enter at full score;
# their IDs are content hashes, so no authored conflict names them.
candidate_selection(Atom, 100) :-
    retrieved_context(Atom),
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
prohibited(Atom) :- blocked_by_context(Atom).
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
# selected_result(Atom, Source)
selected_result(Atom, /skeleton) :-
    final_valid(Atom),
    mandatory_selection(Atom).

selected_result(Atom, /flesh) :-
    final_valid(Atom),
    !mandatory_selection(Atom).
