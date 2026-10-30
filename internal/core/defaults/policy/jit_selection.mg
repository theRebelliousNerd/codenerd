# JIT Prompt Selection Logic
# Extracted from jit_logic.mg to reduce file size.
# Handles ranking and final selection of prompt atoms.
#
# ---------------------------------------------------------------------------
# STATUS (as of 2026-08-11): this ruleset is PARTIALLY LIVE.
# ---------------------------------------------------------------------------
#
# 1. jit_compiler.mg now consumes prohibited_atom and conflict_loser as vetoes
#    -- see the "Policy veto bridge" comment there. So the firewall and
#    conflict-resolution rules in this file bind on every compile.
#
# 2. selected_atom, candidate_atom and mandatory_atom are admissions, and
#    nothing queries them. Wiring selected_atom into tentative was measured on
#    2026-08-11 and rejected: a /fix compile went from 67 atoms and 26279 tokens
#    to 254 atoms and 65036 of 65536 tokens, 99.2 percent budget saturation,
#    because each admitted atom recursively pulls its atom_requires dependencies
#    into tentative. The Go selector still queries only selected_result/2 from
#    jit_compiler.mg. The rule this establishes: a second opinion in a selector
#    may veto, never admit.
#
# 3. /shard_type history, corrected. The old block claimed both rules that
#    matched /shard_type were fixed to /shard. Only mandatory_atom was; the
#    candidate_atom rule still read /shard_type until change 1 above. Commit
#    ce65c7b9 separately made selector.go emit /shard_type as well, so the rule
#    became live rather than dead -- and because /shard_type is not in
#    jit_compiler.mg's regime_dimension list, it was live and ungated. That
#    combination is what produced the saturation measured in point 2.
#
# 4. Two base_prohibited rules in this file remain inert because no producer
#    emits dimension /tag; the INERT comments above them carry the detail.

# -----------------------------------------------------------------------------
# Selection Algorithm (Stratified)
# -----------------------------------------------------------------------------

# The live admission in this scope is the mandatory atoms: the score threshold,
# dependency, conflict and exclusion-group rules that used to stand here read
# atom_context_boost, atom_dependency, atom_conflict and atom_exclusion_group,
# which nothing produces (selector_facts.go emits atom_requires and
# atom_conflicts, and jit_compiler.mg resolves them), so only this rule ever
# fired. atom_final_order was never produced either: the assembler orders atoms.
atom_selected(AtomID) :-
    prompt_atom(AtomID, _, _, /true).

# -----------------------------------------------------------------------------
# Compilation Validation
# -----------------------------------------------------------------------------

# Helper: at least one identity atom is selected
has_identity_atom() :-
    atom_selected(AtomID),
    prompt_atom(AtomID, /identity, _, _).

# Helper: at least one protocol atom is selected
has_protocol_atom() :-
    atom_selected(AtomID),
    prompt_atom(AtomID, /protocol, _, _).

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
    prompt_atom(AtomID, _, _, /true),
    !prohibited_atom(AtomID),
    !atom_selected(AtomID).


# -----------------------------------------------------------------------------
# Integration with Spreading Activation
# -----------------------------------------------------------------------------

# High activation for selected atoms
activation(AtomID, 95) :-
    atom_selected(AtomID).

# -----------------------------------------------------------------------------
# Learning Signals from Prompt Compilation
# -----------------------------------------------------------------------------

# Signal: atom was selected and shard execution succeeded
# compile_shard+shard_success is an existential check (ShardID unused in head)
# Extract existence helper to avoid cross-product with atom_selected
# This used to read shard_executed(ShardID, _, /success, _), but arg 3 of
# shard_executed/4 is the Task ("fix the auth bug"), not an outcome -
# ShardManager.ResultToFacts fills it from the task description and records the
# outcome separately as shard_success(ShardID) / shard_error(ShardID, Msg). No
# task is ever literally /success, so the rule could never fire. shard_success
# is the outcome predicate, and it implies shard_executed for the same ShardID
# because ResultToFacts emits both from the same result.
Decl has_successful_shard() bound [].
has_successful_shard() :-
    compile_shard(ShardID, _),
    shard_success(ShardID).

effective_prompt_atom(AtomID) :-
    atom_selected(AtomID),
    has_successful_shard().

# Learning signal: an atom that was selected when the shard succeeded
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
# Reordered: bind ShardType from atom_tag first, then join compile_shard on shared ShardType
mandatory_atom(AtomID) :-
    prompt_atom(AtomID, Category, _, _),
    skeleton_category(Category),
    atom_tag(AtomID, /shard, ShardType),
    compile_shard(_, ShardType),
    !prohibited_atom(AtomID).

# Atoms explicitly marked as mandatory are always mandatory
mandatory_atom(AtomID) :-
    prompt_atom(AtomID, _, _, /true),
    !prohibited_atom(AtomID).

# Atoms with is_mandatory flag
mandatory_atom(AtomID) :-
    is_mandatory(AtomID),
    !prohibited_atom(AtomID).

# FIREWALL (Prohibited in certain contexts)

# Base prohibitions: context-based blocking
# INERT: atom_tag with /tag never matches — PromptAtom has no Tags field and
# selector.go (1340-1352) never emits dimension /tag, so this rule derives nothing.
# Pending a Tags field on PromptAtom; same is true of the /tag rule in jit_compiler.mg near line 137.
base_prohibited(AtomID) :-
    compile_context(/operational_mode, /production),
    atom_tag(AtomID, /tag, /debug_only).

base_prohibited(AtomID) :-
    compile_context(/operational_mode, /dream),
    prompt_atom(AtomID, /ouroboros, _, _).

base_prohibited(AtomID) :-
    compile_context(/operational_mode, /init),
    prompt_atom(AtomID, /campaign, _, _).

# INERT: atom_tag with /tag never matches — PromptAtom has no Tags field and
# selector.go (1340-1352) never emits dimension /tag, so this rule derives nothing.
# Pending a Tags field on PromptAtom; same is true of the /tag rule in jit_compiler.mg near line 137.
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
# 1. Have a vector hit with sufficient similarity (> 30 on 0-100 scale)
# 2. Not be prohibited by firewall rules
candidate_atom(AtomID) :-
    vector_hit(AtomID, Score),
    Score > 30,
    !prohibited_atom(AtomID).

# Also consider, without a vector hit, this shard's atoms whose neglect Jev
# judged to produce a wrong result or worse (atom_importance level 4
# correctness or 5 critical; internal/prompt/importance.go). candidate_atom
# only feeds the conflict_loser veto below; it admits nothing.
# Reordered: negation checks early (after binding AtomID), bind ShardType from atom_tag before compile_shard
candidate_atom(AtomID) :-
    atom_importance(AtomID, Level),
    Level >= 4,
    !prohibited_atom(AtomID),
    !mandatory_atom(AtomID),
    atom_tag(AtomID, /shard, ShardType),
    compile_shard(_, ShardType).

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

# Helper: Two candidates conflict, the less important one loses. Both need a
# build-time importance; an unscored atom loses no conflict on importance.
# Use atom_conflicts to bind OtherID from AtomID (shared variable), avoiding cross-product
conflict_loser(AtomID) :-
    candidate_atom(AtomID),
    atom_conflicts(AtomID, OtherID),
    candidate_atom(OtherID),
    atom_importance(AtomID, LevelA),
    atom_importance(OtherID, LevelB),
    LevelA < LevelB.

conflict_loser(AtomID) :-
    candidate_atom(AtomID),
    atom_conflicts(OtherID, AtomID),
    candidate_atom(OtherID),
    atom_importance(AtomID, LevelA),
    atom_importance(OtherID, LevelB),
    LevelA < LevelB.

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
    prompt_atom(AtomID, Category, _, _),
    skeleton_category(Category).

missing_skeleton_category(Category) :-
    skeleton_category(Category),
    !has_skeleton_category(Category).

# Report missing skeleton as compilation error
compilation_error(/missing_skeleton, Category) :-
    missing_skeleton_category(Category).


# Capability gating: tool-specific instructions are admitted only when the
# effective catalog contains every required tool. Empty catalog blocks all
# tool-gated atoms (fail-closed). Skeleton safety/evidence atoms carry no
# requires_tools and are never affected. Dependents of a blocked atom are
# blocked transitively through the existing base_prohibited(atom_requires)
# propagation above rather than rendered dangling.
blocked_by_missing_tool(AtomID) :-
    atom_requires_tool(AtomID, Tool),
    !available_tool(Tool).

base_prohibited(AtomID) :-
    blocked_by_missing_tool(AtomID).
