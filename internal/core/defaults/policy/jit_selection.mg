# JIT Prompt Selection Logic
# Extracted from jit_logic.mg to reduce file size.
# Handles ranking and final selection of prompt atoms.

# -----------------------------------------------------------------------------
# Selection Algorithm (Stratified)
# -----------------------------------------------------------------------------

# Phase 1: Candidate atoms pass score threshold and have satisfied dependencies
atom_candidate(AtomID) :-
    atom_matches_context(AtomID, Score),
    Score > 40,
    atom_dependency_satisfied(AtomID).

# Mandatory atoms are always candidates
atom_candidate(AtomID) :-
    prompt_atom(AtomID, _, _, _, /true).

# Phase 2: Detect conflicts among candidates
# An atom loses to a conflicting atom with higher score
atom_loses_conflict(AtomID) :-
    atom_candidate(AtomID),
    atom_conflict(AtomID, OtherID),
    atom_candidate(OtherID),
    atom_matches_context(AtomID, MyScore),
    atom_matches_context(OtherID, OtherScore),
    OtherScore > MyScore.

atom_loses_conflict(AtomID) :-
    atom_candidate(AtomID),
    atom_conflict(OtherID, AtomID),
    atom_candidate(OtherID),
    atom_matches_context(AtomID, MyScore),
    atom_matches_context(OtherID, OtherScore),
    OtherScore > MyScore.

# An atom loses in exclusion group to higher-scoring atom
atom_loses_exclusion(AtomID) :-
    atom_candidate(AtomID),
    atom_exclusion_group(AtomID, GroupID),
    atom_exclusion_group(OtherID, GroupID),
    AtomID != OtherID,
    atom_candidate(OtherID),
    atom_matches_context(AtomID, MyScore),
    atom_matches_context(OtherID, OtherScore),
    OtherScore > MyScore.

# Helper: atom is excluded for any reason
is_excluded(AtomID) :-
    atom_loses_conflict(AtomID).

is_excluded(AtomID) :-
    atom_loses_exclusion(AtomID).

# Exclude if dependency not satisfied (computed early, no cycle)
is_excluded(AtomID) :-
    prompt_atom(AtomID, _, _, _, _),
    !atom_dependency_satisfied(AtomID).

# Phase 3: Final selection - candidates that are not excluded
atom_selected(AtomID) :-
    atom_candidate(AtomID),
    !is_excluded(AtomID).

# -----------------------------------------------------------------------------
# Final Ordering
# -----------------------------------------------------------------------------

# Order selected atoms by category first, then by match score within category.
final_atom(AtomID, Order) :-
    atom_selected(AtomID),
    atom_final_order(AtomID, Order).

# -----------------------------------------------------------------------------
# Compilation Validation
# -----------------------------------------------------------------------------

# Helper: at least one identity atom is selected
has_identity_atom() :-
    atom_selected(AtomID),
    prompt_atom(AtomID, /identity, _, _, _).

# Helper: at least one protocol atom is selected
has_protocol_atom() :-
    atom_selected(AtomID),
    prompt_atom(AtomID, /protocol, _, _, _).

# Helper: at least one compilation error exists
has_compilation_error() :-
    compilation_error(_, _).

# Compilation is valid if: has identity, has protocol, no errors
compilation_valid() :-
    has_identity_atom(),
    has_protocol_atom(),
    !has_compilation_error().

# Error: missing mandatory atom (mandatory atom not selected)
compilation_error(/missing_mandatory, AtomID) :-
    prompt_atom(AtomID, _, _, _, /true),
    !atom_selected(AtomID).

# Error: circular dependency
compilation_error(/circular_dependency, AtomID) :-
    atom_dependency(AtomID, DepID, /hard),
    atom_dependency(DepID, AtomID, /hard).

# -----------------------------------------------------------------------------
# Integration with Spreading Activation
# -----------------------------------------------------------------------------

# High activation for selected atoms
activation(AtomID, 95) :-
    atom_selected(AtomID).

# Medium activation for atoms matching context but not selected
activation(AtomID, 60) :-
    atom_matches_context(AtomID, Score),
    Score > 30,
    !atom_selected(AtomID).

# -----------------------------------------------------------------------------
# Learning Signals from Prompt Compilation
# -----------------------------------------------------------------------------

# Signal: atom was selected and shard execution succeeded
effective_prompt_atom(AtomID) :-
    atom_selected(AtomID),
    compile_shard(ShardID, _),
    shard_executed(ShardID, _, /success, _).

# Learning signal: promote effective atoms to higher priority
learning_signal(/effective_prompt_atom, AtomID) :-
    effective_prompt_atom(AtomID).

# -----------------------------------------------------------------------------
# SELECTION RULES (From former Section 46)
# -----------------------------------------------------------------------------

# SKELETON (Mandatory - Fail if missing)

# Define skeleton categories - these are non-negotiable prompt sections
skeleton_category(/identity).
skeleton_category(/protocol).
skeleton_category(/safety).
skeleton_category(/methodology).

# An atom is mandatory if:
# 1. It belongs to a skeleton category
# 2. It matches the current shard type (if tagged)
# 3. It is not explicitly prohibited
mandatory_atom(AtomID) :-
    prompt_atom(AtomID, Category, _, _, _),
    skeleton_category(Category),
    compile_shard(_, ShardType),
    atom_tag(AtomID, /shard_type, ShardType),
    !prohibited_atom(AtomID).

# Atoms explicitly marked as mandatory are always mandatory
mandatory_atom(AtomID) :-
    prompt_atom(AtomID, _, _, _, /true),
    !prohibited_atom(AtomID).

# Atoms with is_mandatory flag
mandatory_atom(AtomID) :-
    is_mandatory(AtomID),
    !prohibited_atom(AtomID).

# FIREWALL (Prohibited in certain contexts)

# Base prohibitions: context-based blocking
base_prohibited(AtomID) :-
    compile_context(/operational_mode, /production),
    atom_tag(AtomID, /tag, /debug_only).

base_prohibited(AtomID) :-
    compile_context(/operational_mode, /dream),
    atom_tag(AtomID, /category, /ouroboros).

base_prohibited(AtomID) :-
    compile_context(/operational_mode, /init),
    atom_tag(AtomID, /category, /campaign).

base_prohibited(AtomID) :-
    compile_context(/operational_mode, /active),
    atom_tag(AtomID, /tag, /dream_only).

# Dependency-based prohibition
base_prohibited(AtomID) :-
    atom_requires(AtomID, DepID),
    base_prohibited(DepID).

# prohibited_atom = base_prohibited
prohibited_atom(AtomID) :- base_prohibited(AtomID).

# FLESH (Vector candidates filtered by Mangle)

# Candidate atoms must:
# 1. Have a vector hit with sufficient similarity (> 0.3)
# 2. Not be prohibited by firewall rules
candidate_atom(AtomID) :-
    vector_hit(AtomID, Score),
    Score > 0.3,
    !prohibited_atom(AtomID).

# Also consider atoms matching context dimensions even without vector hit
candidate_atom(AtomID) :-
    prompt_atom(AtomID, _, Priority, _, _),
    Priority > 50,
    atom_tag(AtomID, /shard_type, ShardType),
    compile_shard(_, ShardType),
    !prohibited_atom(AtomID),
    !mandatory_atom(AtomID).

# Final Selection (with Conflict Resolution)

# Helper: An atom loses a conflict to a mandatory atom
conflict_loser(AtomID) :-
    candidate_atom(AtomID),
    atom_conflicts(AtomID, MandatoryID),
    mandatory_atom(MandatoryID).

conflict_loser(AtomID) :-
    candidate_atom(AtomID),
    atom_conflicts(MandatoryID, AtomID),
    mandatory_atom(MandatoryID).

# Helper: Two candidates conflict, lower priority loses
conflict_loser(AtomID) :-
    candidate_atom(AtomID),
    candidate_atom(OtherID),
    atom_conflicts(AtomID, OtherID),
    prompt_atom(AtomID, _, PriorityA, _, _),
    prompt_atom(OtherID, _, PriorityB, _, _),
    PriorityA < PriorityB.

conflict_loser(AtomID) :-
    candidate_atom(AtomID),
    candidate_atom(OtherID),
    atom_conflicts(OtherID, AtomID),
    prompt_atom(AtomID, _, PriorityA, _, _),
    prompt_atom(OtherID, _, PriorityB, _, _),
    PriorityA < PriorityB.

# Final selection: mandatory atoms always selected
selected_atom(AtomID) :- mandatory_atom(AtomID).

# Candidates selected if not a conflict loser
selected_atom(AtomID) :-
    candidate_atom(AtomID),
    !mandatory_atom(AtomID),
    !conflict_loser(AtomID).

# Section 46 Validation

has_skeleton_category(Category) :-
    selected_atom(AtomID),
    prompt_atom(AtomID, Category, _, _, _),
    skeleton_category(Category).

missing_skeleton_category(Category) :-
    skeleton_category(Category),
    !has_skeleton_category(Category).

# Report missing skeleton as compilation error
compilation_error(/missing_skeleton, Category) :-
    missing_skeleton_category(Category).
