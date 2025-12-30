# Cortex 1.5.0 Schemas (EDB Declarations)
# Version: 1.5.0
# Philosophy: Logic determines Reality; the Model merely describes it.

# Modular Schema: INTENT
# Sections: 1, 2

# =============================================================================
# SECTION 1: INTENT SCHEMA (§1.1)
# =============================================================================

# user_intent(ID, Category, Verb, Target, Constraint)
# Category: /query, /mutation, /instruction
# Verb: /explain, /refactor, /debug, /generate, /scaffold, /init, /test, /review, /fix, /run, /research, /explore, /implement
# Priority: 100
# SerializationOrder: 1
Decl user_intent(ID, Category, Verb, Target, Constraint).

# =============================================================================
# SECTION 2: FOCUS RESOLUTION (§1.2)
# =============================================================================

# focus_resolution(RawReference, ResolvedPath, SymbolName, Confidence)
# Priority: 100
# SerializationOrder: 2
Decl focus_resolution(RawReference, ResolvedPath, SymbolName, Confidence).

# ambiguity_flag(MissingParam, ContextClue, Hypothesis)
Decl ambiguity_flag(MissingParam, ContextClue, Hypothesis).

# intent_definition(Sentence, Verb, Target)
# Canonical intent examples for heuristic matching
Decl intent_definition(Sentence, Verb, Target).
Decl intent_category(Sentence, Category).

# =============================================================================
# SECTION 3: LLM ROUTING SCHEMA (Used by intent_routing.mg facts)
# =============================================================================

Decl valid_semantic_type(Type, Description).
Decl valid_action_type(Action, Description).
Decl valid_domain(Domain, Description).
Decl valid_scope_level(Level, Order).
Decl valid_mode(Mode, Description).
Decl valid_urgency(Urgency, Order).

Decl mode_from_semantic(SemanticType, Mode, Priority).
Decl mode_from_action(ActionType, Mode, Priority).
Decl mode_from_domain(Domain, Mode, Priority).
Decl mode_from_signal(Signal, Mode, Priority).

Decl context_affinity_semantic(SemanticType, ContextCategory, Weight).
Decl context_affinity_action(ActionType, ContextCategory, Weight).
Decl context_affinity_domain(Domain, ContextCategory, Weight).

Decl shard_affinity_action(ActionType, ShardType, Weight).
Decl shard_affinity_domain(Domain, ShardType, Weight).

Decl tool_affinity_action(ActionType, Tool, Weight).
Decl tool_affinity_domain(Domain, Tool, Weight).

Decl best_mode(Mode, Score).
Decl best_shard(Shard, Score).
Decl context_category_priority(ContextCategory, Score).
Decl tool_priority(Tool, Score).

Decl constraint_type(Constraint, Effect).
Decl constraint_forces_mode(Constraint, Mode).
Decl constraint_blocks_tool(Constraint, Tool).

