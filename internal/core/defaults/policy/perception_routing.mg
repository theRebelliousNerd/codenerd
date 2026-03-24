# Perception Routing Rules (P3: Routing Assertion)
# Derives candidate_mode, best_candidate_priority, and derived_mode from
# current_understanding EDB facts and the existing mode_from_* routing tables.
#
# Stratification analysis:
#   Stratum N   (positive EDB): current_understanding, mode_from_semantic, mode_from_action
#   Stratum N   (positive IDB): candidate_mode  (depends only on positive EDB)
#   Stratum N+1 (aggregation):  best_candidate_priority (fn:max over candidate_mode)
#   Stratum N+2 (positive):     derived_mode via candidate_mode + best_candidate_priority
#   Stratum N+2 (negation):     derived_mode fallback via ~candidate_mode (safe: bound)
#
# No cycles. Negation in the fallback rule is safe because candidate_mode variables
# are fully bound by the positive body before ~candidate_mode is checked.

# NERD-EVOLVE-START: perception_routing_rules

# Phase 1: Collect candidate modes from EDB (Stratum N, all positive)

candidate_mode(Mode, /edb_semantic, Priority) :-
    current_understanding(SemanticType, _, _, _),
    mode_from_semantic(SemanticType, Mode, Priority).

candidate_mode(Mode, /edb_action, Priority) :-
    current_understanding(_, ActionType, _, _),
    mode_from_action(ActionType, Mode, Priority).

# Phase 2: Select best mode (Stratum N+1, aggregation)

best_candidate_priority(MaxPriority) :-
    candidate_mode(_, _, Priority) |> do fn:group_by(), let MaxPriority = fn:max(Priority).

# Phase 3: Derive winning mode (Stratum N+2)

derived_mode(Mode) :-
    candidate_mode(Mode, _, Priority),
    best_candidate_priority(Priority).

# Fallback: when no EDB routing matches, trust the LLM's own suggestion.
# Safe negation: ~candidate_mode(_, _, _) is checked after candidate_mode is
# fully defined (Stratum N); variables are irrelevant (wildcard), so binding
# is not required — Mangle treats this as a closed-world negation over the
# entire relation.
derived_mode(Mode) :-
    llm_suggested_mode(Mode),
    !candidate_mode(_, _, _).

# NERD-EVOLVE-END: perception_routing_rules
