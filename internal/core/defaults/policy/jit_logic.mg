# JIT Prompt Compiler Logic
# Extracted from jit_compiler.mg
# Implements context matching rules.
# Selection logic moved to jit_selection.mg

# -----------------------------------------------------------------------------
# Internal Helper Declarations (IDB)
# -----------------------------------------------------------------------------

# Context match indicators
Decl atom_has_shard_match(AtomID).
Decl atom_has_mode_match(AtomID).
Decl atom_has_phase_match(AtomID).
Decl atom_has_verb_match(AtomID).
Decl atom_has_lang_match(AtomID).
Decl atom_has_framework_match(AtomID).
Decl atom_has_state_match(AtomID).
Decl atom_has_init_match(AtomID).
Decl atom_has_ouroboros_match(AtomID).
Decl atom_has_northstar_match(AtomID).
Decl atom_has_layer_match(AtomID).

# -----------------------------------------------------------------------------
# Contextual Matching Rules
# -----------------------------------------------------------------------------

atom_has_shard_match(AtomID) :-
    atom_selector(AtomID, /shard_type, ShardType),
    compile_shard(_, ShardType).

atom_has_mode_match(AtomID) :-
    atom_selector(AtomID, /operational_mode, Mode),
    compile_context(/operational_mode, Mode).

atom_has_phase_match(AtomID) :-
    atom_selector(AtomID, /campaign_phase, Phase),
    compile_context(/campaign_phase, Phase).

atom_has_verb_match(AtomID) :-
    atom_selector(AtomID, /intent_verb, Verb),
    compile_context(/intent_verb, Verb).

atom_has_lang_match(AtomID) :-
    atom_selector(AtomID, /language, Lang),
    compile_context(/language, Lang).

atom_has_framework_match(AtomID) :-
    atom_selector(AtomID, /framework, Framework),
    compile_context(/framework, Framework).

atom_has_state_match(AtomID) :-
    atom_selector(AtomID, /world_state, State),
    compile_context(/world_state, State).

atom_has_init_match(AtomID) :-
    atom_selector(AtomID, /init_phase, Phase),
    compile_context(/init_phase, Phase).

atom_has_ouroboros_match(AtomID) :-
    atom_selector(AtomID, /ouroboros_stage, Stage),
    compile_context(/ouroboros_stage, Stage).

atom_has_northstar_match(AtomID) :-
    atom_selector(AtomID, /northstar_phase, Phase),
    compile_context(/northstar_phase, Phase).

atom_has_layer_match(AtomID) :-
    atom_selector(AtomID, /build_layer, Layer),
    compile_context(/build_layer, Layer).

# Final atom score from Go-computed boost (virtual predicate)
atom_matches_context(AtomID, FinalScore) :-
    prompt_atom(AtomID, _, _, _, _),
    atom_context_boost(AtomID, FinalScore).

# Mandatory atoms always get max score (100)
atom_matches_context(AtomID, 100) :-
    prompt_atom(AtomID, _, _, _, /true).

# -----------------------------------------------------------------------------
# Dependency Resolution (Stratified)
# -----------------------------------------------------------------------------

# Helper: atom would meet score threshold (potential candidate)
atom_meets_threshold(AtomID) :-
    atom_matches_context(AtomID, Score),
    Score > 40.

# Helper: atom is mandatory (always meets threshold)
atom_meets_threshold(AtomID) :-
    prompt_atom(AtomID, _, _, _, /true).

# Helper: atom has at least one unsatisfiable hard dependency
has_unsatisfied_hard_dep(AtomID) :-
    atom_dependency(AtomID, DepID, /hard),
    prompt_atom(DepID, _, _, _, _),
    !atom_meets_threshold(DepID).

# Atom dependencies are satisfied if no unsatisfiable hard deps exist
atom_dependency_satisfied(AtomID) :-
    prompt_atom(AtomID, _, _, _, _),
    !has_unsatisfied_hard_dep(AtomID).
