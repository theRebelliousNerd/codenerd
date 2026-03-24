# Perception Latency Schemas (P3: Routing Assertion)
# Declares predicates written by the transducer into the EDB each turn
# so that C1/C4 composition rules can derive from them without a query round-trip.
#
# Stratum: EDB-only (all facts asserted from Go; no rules derive these directly).
# Per-turn lifecycle: retracted at turn start in process.go, reasserted by deriveRouting.

# NERD-EVOLVE-START: P3_schema_decls

# current_understanding(SemanticType, ActionType, Domain, ScopeLevel)
# Asserted once per turn with the LLM's normalised understanding.
Decl current_understanding(SemanticType, ActionType, Domain, ScopeLevel).

# llm_suggested_mode(Mode)
# The raw mode suggested by the LLM before Mangle override.
Decl llm_suggested_mode(Mode).

# candidate_mode(Mode, Source, Priority)
# Collected from EDB routing facts; used by perception_routing.mg to select best mode.
Decl candidate_mode(Mode, Source, Priority).

# best_candidate_priority(MaxPriority)
# Aggregated maximum priority across all candidate_mode facts.
Decl best_candidate_priority(MaxPriority).

# derived_mode(Mode)
# The final harness mode after Mangle selection (or LLM fallback).
Decl derived_mode(Mode).

# derived_primary_shard(ShardID)
# The primary shard selected by the harness for this turn.
Decl derived_primary_shard(ShardID).

# derived_context_priority(Category, Priority)
# Per-category context priorities derived from routing affinity rules.
Decl derived_context_priority(Category, Priority).

# derived_tool_priority(Tool, Priority)
# Per-tool priorities derived from routing affinity rules.
Decl derived_tool_priority(Tool, Priority).

# NERD-EVOLVE-END: P3_schema_decls
