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

# =============================================================================
# SECTION 2.1: INTENT CLARIFICATION + LEARNING (§1.3)
# =============================================================================

# intent_unknown(Input, Reason)
# Reason: /llm_failed, /heuristic_low, /no_verb_match
Decl intent_unknown(INPUT, REASON).

# intent_unmapped(Verb, Reason)
# Reason: /unknown_verb, /no_action_mapping, /deprecated_verb
Decl intent_unmapped(VERB, REASON).

# no_action_reason(IntentID, Reason)
# Reason: /unmapped_verb, /no_route, /blocked_by_constitution, /ooda_timeout, /no_action_derived
Decl no_action_reason(INTENTID, REASON).

# learning_candidate(Phrase, Verb, Target, Reason)
# Staged for confirmation before promotion to learned_exemplar
Decl learning_candidate(PHRASE, VERB, TARGET, REASON).
# learning_candidate_fact(Phrase, Verb, Target, Reason, Fact)
# Stores raw learned_exemplar text for confirmation flows
Decl learning_candidate_fact(PHRASE, VERB, TARGET, REASON, FACT).

# learning_confirmation_needed(Phrase, Verb, Target, Reason)
# Derived from learning_candidate when explicit confirmation is required
Decl learning_confirmation_needed(PHRASE, VERB, TARGET, REASON).

# clarification_question(IntentID, Question)
Decl clarification_question(INTENTID, QUESTION).

# clarification_option(IntentID, OptionVerb, OptionLabel)
Decl clarification_option(INTENTID, OPTIONVERB, OPTIONLABEL).

# learning_candidate_count(Phrase, Count)
Decl learning_candidate_count(PHRASE, COUNT).

# learning_candidate_ready(Phrase, Verb)
Decl learning_candidate_ready(PHRASE, VERB).

# intent_definition(Sentence, Verb, Target)
# Canonical intent examples for heuristic matching
Decl intent_definition(Sentence, Verb, Target).
Decl intent_category(Sentence, Category).

# =============================================================================
# SECTION 2.2: INTENT QUALIFIERS (Lexicon)
# =============================================================================
# Declarations for qualifier lexicons used by taxonomy_qualifiers.mg.
# Facts live in schema/intent_qualifiers.mg (taxonomy engine).
Decl interrogative_type(Word, SemanticType, DefaultVerb, Priority).
Decl modal_type(Word, ModalMeaning, Transformation, Priority).
Decl state_adjective(Adjective, ImpliedVerb, StateCategory, Priority).
Decl negation_marker(Word, NegationType, Priority).
Decl copular_verb(Word, Tense, Number).
Decl existence_pattern(Pattern, QueryType, DefaultVerb, Priority).
Decl interrogative_state_signal(InterrogType, StateCategory, CombinedVerb, Priority).

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


