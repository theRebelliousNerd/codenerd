# Cortex 1.5.0 Schemas (EDB Declarations)
# Version: 1.5.0
# Philosophy: Logic determines Reality; the Model merely describes it.

# Modular Schema: MEMORY
# Sections: 7

# =============================================================================
# SECTION 7: MEMORY SHARDS (§7.1-7.4)
# =============================================================================

# vector_recall(Query, Content, Score)
Decl vector_recall(Query, Content, Score) bound [/string, /string, /number].

# knowledge_link(EntityA, Relation, EntityB)
Decl knowledge_link(EntityA, Relation, EntityB) bound [/string, /string, /string].

# new_fact(FactID) - marks a fact as newly added (for activation)
Decl new_fact(FactID) bound [/string].

# trace_recall_result(TraceID, Score, Outcome, Summary)
# Score is integer 0-100 (scaled from 0.0-1.0 in Go)
Decl trace_recall_result(TraceID, Score, Outcome, Summary) bound [/string, /number, /name, /string].

# learning_recall_result(LearningID, Score, Predicate, Description)
# Score is integer 0-100 (scaled from 0.0-1.0 in Go)
Decl learning_recall_result(LearningID, Score, Predicate, Description) bound [/string, /number, /string, /string].

# knowledge_edge(Subject, Relation, Object) - Knowledge graph relationship edge
# Records entity relationships in the knowledge graph tier
Decl knowledge_edge(Subject, Relation, Object) bound [/string, /name, /string].

# cold_storage_entry(Key, Value) - Long-term cold storage entries
# Records entries persisted in the cold storage memory tier
Decl cold_storage_entry(Key, Value) bound [/string, /string].

# compressed_context(SessionID, Summary) - Semantically compressed context entries
# Produced by the context compressor for infinite context support
Decl compressed_context(SessionID, Summary) bound [/string, /string].

# =============================================================================
# SECTION 7B: VIRTUAL PREDICATES FOR KNOWLEDGE QUERIES (Bound)
# =============================================================================
# These predicates are resolved by VirtualStore FFI to query knowledge.db
# Virtual predicates computed on-demand by the Go runtime (VirtualStore)

# query_learned(Predicate, Args) - Queries cold_storage for learned facts
# External predicate: both args are output (enumeration mode)
Decl query_learned(Predicate, Args) descr [external(), mode('-', '-')] bound [/string, /string].

# query_session(SessionID, TurnNumber, UserInput) - Queries session_history
# External predicate: SessionID required as input
Decl query_session(SessionID, TurnNumber, UserInput) descr [external(), mode('+', '-', '-')] bound [/string, /number, /string].

# recall_similar(Query, TopK, Results) - Semantic search on vectors table
# External predicate: Query required as input
Decl recall_similar(Query, TopK, Results) descr [external(), mode('+', '-', '-')] bound [/string, /number, /string].

# query_knowledge_graph(EntityA, Relation, EntityB) - Entity relationships
# External predicate: EntityA required as input
Decl query_knowledge_graph(EntityA, Relation, EntityB) descr [external(), mode('+', '-', '-')] bound [/string, /string, /string].

# query_strategic(Category, Content, Confidence) - Strategic knowledge atoms
# External predicate: all outputs (enumeration mode)
Decl query_strategic(Category, Content, Confidence) descr [external(), mode('-', '-', '-')] bound [/name, /string, /number].

# query_activations(FactID, Score) - Activation log scores
# External predicate: all outputs (enumeration mode)
Decl query_activations(FactID, Score) descr [external(), mode('-', '-')] bound [/string, /number].

# has_learned(Predicate) - Check if facts exist in cold_storage
# External predicate: output (enumeration mode)
Decl has_learned(Predicate) descr [external(), mode('-')] bound [/string].

# =============================================================================
# SECTION 7C: HYDRATED KNOWLEDGE FACTS (asserted by HydrateLearnings)
# =============================================================================
# These EDB predicates are populated by VirtualStore.HydrateLearnings() during
# the OODA Observe phase. They make learned knowledge available to Mangle rules.

# learned_preference(Predicate, Args) - User preferences from cold_storage
Decl learned_preference(Predicate, Args) bound [/string, /string].

# learned_fact(Predicate, Args) - User facts from cold_storage
Decl learned_fact(Predicate, Args) bound [/string, /string].

# learned_constraint(Predicate, Args) - User constraints from cold_storage
Decl learned_constraint(Predicate, Args) bound [/string, /string].

# activation(FactID, Score) - Recent activation scores
Decl activation(FactID, Score) bound [/string, /number].

# session_turn(SessionID, TurnNumber, UserInput, Response) - Conversation history
Decl session_turn(SessionID, TurnNumber, UserInput, Response) bound [/string, /number, /string, /string].

# conversation_turn(TurnID, Speaker, Message, Intent) - Compressed conversation turn
Decl conversation_turn(TurnID, Speaker, Message, Intent) bound [/string, /string, /string, /string].

# turn_references_file(TurnID, FilePath) - Files referenced in a turn
Decl turn_references_file(TurnID, FilePath) bound [/string, /string].

# turn_error_message(TurnID, ErrorMessage) - Error messages captured in a turn
Decl turn_error_message(TurnID, ErrorMessage) bound [/string, /string].

# turn_topic(TurnID, Topic) - Topics extracted from a turn
Decl turn_topic(TurnID, Topic) bound [/string, /string].

# turn_references_back(TurnID, ReferencedTurnID) - Reference-back linkage
Decl turn_references_back(TurnID, ReferencedTurnID) bound [/string, /string].

# similar_content(Rank, Content) - Semantic search results
Decl similar_content(Rank, Content) bound [/number, /string].

# =============================================================================
# SECTION 7D: HELPER PREDICATES FOR LEARNED KNOWLEDGE RULES
# =============================================================================
# These support the IDB rules in policy.gl SECTION 17B

# tool_language(Tool, Language) - Maps tools to programming languages
Decl tool_language(Tool, Language) bound [/name, /name].

# action_violates(Action, Predicate, Args) - Check if action violates a constraint
Decl action_violates(Action, Predicate, Args) bound [/name, /string, /string].

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
Decl atom_final_order(AtomID, Order) bound [/string, /number].

# unhandled_case_count_computed(ShardName, Count) - List count for unhandled cases
# Mangle doesn't have list length function; Go computes this
Decl unhandled_case_count_computed(ShardName, Count) bound [/string, /number].

# high_element_count_flag() - True when code_element count >= 5
# Aggregation-based rules can't be validated statically; Go computes this
Decl high_element_count_flag().

# pending_subtask_count_computed(Count) - Count of pending subtasks
# Aggregation computed by Go runtime
Decl pending_subtask_count_computed(Count) bound [/number].

# context_pressure_level(CampaignID, Level) - Context utilization level
# Level: /normal, /high (>80%), /critical (>95%)
# Computed by Go based on context_window_state Used/Total ratio
Decl context_pressure_level(CampaignID, Level) bound [/string, /name].

# campaign_progress_over_50(CampaignID) - True when campaign > 50% complete
# Computed by Go: (Completed / Total) >= 0.5
Decl campaign_progress_over_50(CampaignID) bound [/string].

# relevant_to_intent(Predicate, Intent) - Maps predicates to user intents
Decl relevant_to_intent(Predicate, Intent) bound [/string, /name].

# context_priority(FactID, Priority) - Priority level for context inclusion
Decl context_priority(FactID, Priority) bound [/string, /number].

# related_context(Content) - Content related to current context
Decl related_context(Content) bound [/string].

# constraint_violation(Action, Reason) - Detected constraint violations
Decl constraint_violation(Action, Reason) bound [/name, /string].

# =============================================================================
# SECTION 7F: TELEMETRY / OBSERVABILITY (Go-Asserted)
# =============================================================================

# jit_fallback(ShardType, Reason)
# Asserted by Go when JIT prompt compilation fails and Articulation falls back.
Decl jit_fallback(ShardType, Reason) bound [/name, /string].

