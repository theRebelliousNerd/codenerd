# Cortex 1.5.0 Schemas (EDB Declarations)
# Version: 1.5.0
# Philosophy: Logic determines Reality; the Model merely describes it.

# Modular Schema: MEMORY
# Sections: 7

# =============================================================================
# SECTION 7: MEMORY SHARDS (§7.1-7.4)
# =============================================================================

# vector_recall(Query, Content, Score)
Decl vector_recall(Query, Content, Score).

# knowledge_link(EntityA, Relation, EntityB)
Decl knowledge_link(EntityA, Relation, EntityB).

# new_fact(FactID) - marks a fact as newly added (for activation)
Decl new_fact(FactID).

# trace_recall_result(TraceID, Score, Outcome, Summary)
# Score is integer 0-100 (scaled from 0.0-1.0 in Go)
Decl trace_recall_result(TraceID, Score, Outcome, Summary).

# learning_recall_result(LearningID, Score, Predicate, Description)
# Score is integer 0-100 (scaled from 0.0-1.0 in Go)
Decl learning_recall_result(LearningID, Score, Predicate, Description).

# =============================================================================
# SECTION 7B: VIRTUAL PREDICATES FOR KNOWLEDGE QUERIES (Bound)
# =============================================================================
# These predicates are resolved by VirtualStore FFI to query knowledge.db
# Virtual predicates computed on-demand by the Go runtime (VirtualStore)

# query_learned(Predicate, Args) - Queries cold_storage for learned facts
Decl query_learned(Predicate, Args).

# query_session(SessionID, TurnNumber, UserInput) - Queries session_history
Decl query_session(SessionID, TurnNumber, UserInput).

# recall_similar(Query, TopK, Results) - Semantic search on vectors table
Decl recall_similar(Query, TopK, Results).

# query_knowledge_graph(EntityA, Relation, EntityB) - Entity relationships
Decl query_knowledge_graph(EntityA, Relation, EntityB).

# query_strategic(Category, Content, Confidence) - Strategic knowledge atoms
Decl query_strategic(Category, Content, Confidence).

# query_activations(FactID, Score) - Activation log scores
Decl query_activations(FactID, Score).

# has_learned(Predicate) - Check if facts exist in cold_storage
Decl has_learned(Predicate).

# =============================================================================
# SECTION 7C: HYDRATED KNOWLEDGE FACTS (asserted by HydrateLearnings)
# =============================================================================
# These EDB predicates are populated by VirtualStore.HydrateLearnings() during
# the OODA Observe phase. They make learned knowledge available to Mangle rules.

# learned_preference(Predicate, Args) - User preferences from cold_storage
Decl learned_preference(Predicate, Args).

# learned_fact(Predicate, Args) - User facts from cold_storage
Decl learned_fact(Predicate, Args).

# learned_constraint(Predicate, Args) - User constraints from cold_storage
Decl learned_constraint(Predicate, Args).

# activation(FactID, Score) - Recent activation scores
Decl activation(FactID, Score).

# session_turn(SessionID, TurnNumber, UserInput, Response) - Conversation history
Decl session_turn(SessionID, TurnNumber, UserInput, Response).

# conversation_turn(TurnID, Speaker, Message, Intent) - Compressed conversation turn
Decl conversation_turn(TurnID, Speaker, Message, Intent).

# turn_references_file(TurnID, FilePath) - Files referenced in a turn
Decl turn_references_file(TurnID, FilePath).

# turn_error_message(TurnID, ErrorMessage) - Error messages captured in a turn
Decl turn_error_message(TurnID, ErrorMessage).

# turn_topic(TurnID, Topic) - Topics extracted from a turn
Decl turn_topic(TurnID, Topic).

# turn_references_back(TurnID, ReferencedTurnID) - Reference-back linkage
Decl turn_references_back(TurnID, ReferencedTurnID).

# similar_content(Rank, Content) - Semantic search results
Decl similar_content(Rank, Content).

# =============================================================================
# SECTION 7D: HELPER PREDICATES FOR LEARNED KNOWLEDGE RULES
# =============================================================================
# These support the IDB rules in policy.gl SECTION 17B

# tool_language(Tool, Language) - Maps tools to programming languages
Decl tool_language(Tool, Language).

# action_violates(Action, Predicate, Args) - Check if action violates a constraint
Decl action_violates(Action, Predicate, Args).

# =============================================================================
# SECTION 7E: TIME-BASED VIRTUAL PREDICATES
# =============================================================================
# These predicates handle time-based computations that cannot be done in Mangle.
# Mangle does not support scalar arithmetic in rules (only aggregation transforms).
# The Go VirtualStore computes these based on current time and thresholds.

# checkpoint_needed() - True when checkpoint interval (600s) has elapsed
# Computed by Go based on last_checkpoint_time vs current_time
Decl checkpoint_needed().

# ooda_timeout() - True when OODA loop has stalled (30s+ without action)
# Computed by Go based on last_action_time vs current_time
Decl ooda_timeout().

# atom_final_order(AtomID, Order) - Computed ordering for final_atom
# Order = (CategoryOrder * 1000) + Score, computed by Go
Decl atom_final_order(AtomID, Order).

# unhandled_case_count_computed(ShardName, Count) - List count for unhandled cases
# Mangle doesn't have list length function; Go computes this
Decl unhandled_case_count_computed(ShardName, Count).

# high_element_count_flag() - True when code_element count >= 5
# Aggregation-based rules can't be validated statically; Go computes this
Decl high_element_count_flag().

# pending_subtask_count_computed(Count) - Count of pending subtasks
# Aggregation computed by Go runtime
Decl pending_subtask_count_computed(Count).

# context_pressure_level(CampaignID, Level) - Context utilization level
# Level: /normal, /high (>80%), /critical (>95%)
# Computed by Go based on context_window_state Used/Total ratio
Decl context_pressure_level(CampaignID, Level).

# campaign_progress_over_50(CampaignID) - True when campaign > 50% complete
# Computed by Go: (Completed / Total) >= 0.5
Decl campaign_progress_over_50(CampaignID).

# relevant_to_intent(Predicate, Intent) - Maps predicates to user intents
Decl relevant_to_intent(Predicate, Intent).

# context_priority(FactID, Priority) - Priority level for context inclusion
Decl context_priority(FactID, Priority).

# related_context(Content) - Content related to current context
Decl related_context(Content).

# constraint_violation(Action, Reason) - Detected constraint violations
Decl constraint_violation(Action, Reason).

# =============================================================================
# SECTION 7F: TELEMETRY / OBSERVABILITY (Go-Asserted)
# =============================================================================

# jit_fallback(ShardType, Reason)
# Asserted by Go when JIT prompt compilation fails and Articulation falls back.
Decl jit_fallback(ShardType, Reason).

