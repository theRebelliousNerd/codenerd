# JIT Prompt Compiler Logic
# Extracted from jit_compiler.mg
# Implements context matching rules.
# Selection logic moved to jit_selection.mg

# -----------------------------------------------------------------------------
# Internal Helper Declarations (IDB)
# -----------------------------------------------------------------------------

# REMOVED 2026-09-09: the atom_has_*_match dimension rules.
#
# Thirteen Decls and thirteen rules used to live here, one per selector
# dimension (shard, mode, phase, verb, language, framework, world state, init
# phase, ouroboros stage, northstar phase, build layer, provider, model). All
# thirteen joined on atom_selector/3, and all thirteen were inert at both ends:
#
#   - No production producer. The only Go emitter of atom_selector is
#     PromptAtom.ToSelectorFacts (internal/prompt/atoms.go:540), called from
#     atoms_test.go and atom_pinning_test.go and nowhere else. The live
#     selector emits a different vocabulary entirely — atom, prompt_atom,
#     atom_category, atom_priority, atom_tag, is_mandatory, atom_requires,
#     atom_conflicts, atom_requires_tool, available_tool, compile_shard — which
#     is what promptEphemeralPredicates (internal/prompt/compiler.go:60-75)
#     retracts per compile. atom_selector is not in that list.
#   - No consumer. Nothing in any .mg file or any Go file referenced
#     atom_has_*_match. atom_matches_context below joins prompt_atom and
#     atom_context_boost; it never mentions them.
#
# So they derived nothing from nothing, at N x M join cost per compile.
#
# The dimensional matching itself is NOT missing. It happens on the live path:
# jit_compiler.mg joins atom_tag against current_context to derive
# blocked_by_context, and selected_result/3 is what selector.go actually
# queries (selector.go:877 and :1044). The fail-closed regime_dimension block
# in jit_compiler.mg is what makes a dimension binding. This file's version was
# a second, parallel design that was superseded and never removed.
#
# Recorded rather than silently dropped so the idea is not rediscovered and
# re-added. If dimension scoring is ever wanted as a score rather than a veto,
# the producer to write is in selector.go, and it should extend the live
# vocabulary rather than revive this one.

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
