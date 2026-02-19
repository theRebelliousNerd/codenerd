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
Decl user_intent(ID, Category, Verb, Target, Constraint) bound [/string, /name, /name, /string, /string].

# =============================================================================
# SECTION 2: FOCUS RESOLUTION (§1.2)
# =============================================================================

# focus_resolution(RawReference, ResolvedPath, SymbolName, Confidence)
# Priority: 100
# SerializationOrder: 2
Decl focus_resolution(RawReference, ResolvedPath, SymbolName, Confidence) bound [/string, /string, /string, /number].

# ambiguity_flag(MissingParam, ContextClue, Hypothesis)
Decl ambiguity_flag(MissingParam, ContextClue, Hypothesis) bound [/string, /string, /string].

# =============================================================================
# SECTION 2.1: INTENT CLARIFICATION + LEARNING (§1.3)
# =============================================================================

# intent_unknown(Input, Reason)
# Reason: /llm_failed, /heuristic_low, /no_verb_match
Decl intent_unknown(INPUT, REASON) bound [/string, /name].

# intent_unmapped(Verb, Reason)
# Reason: /unknown_verb, /no_action_mapping, /deprecated_verb
Decl intent_unmapped(VERB, REASON) bound [/name, /name].

# no_action_reason(IntentID, Reason)
# Reason: /unmapped_verb, /no_route, /blocked_by_constitution, /ooda_timeout, /no_action_derived
Decl no_action_reason(INTENTID, REASON) bound [/string, /name].

# learning_candidate(Phrase, Verb, Target, Reason)
# Staged for confirmation before promotion to learned_exemplar
Decl learning_candidate(PHRASE, VERB, TARGET, REASON) bound [/string, /string, /string, /name].
# learning_candidate_fact(Phrase, Verb, Target, Reason, Fact)
# Stores raw learned_exemplar text for confirmation flows
Decl learning_candidate_fact(PHRASE, VERB, TARGET, REASON, FACT) bound [/string, /string, /string, /name, /string].

# learning_confirmation_needed(Phrase, Verb, Target, Reason)
# Derived from learning_candidate when explicit confirmation is required
Decl learning_confirmation_needed(PHRASE, VERB, TARGET, REASON) bound [/string, /string, /string, /name].

# learning_confirmation_active(Status)
Decl learning_confirmation_active(Status) bound [/name].

# clarification_question(IntentID, Question)
Decl clarification_question(INTENTID, QUESTION) bound [/string, /string].

# clarification_option(IntentID, OptionVerb, OptionLabel)
Decl clarification_option(INTENTID, OPTIONVERB, OPTIONLABEL) bound [/string, /string, /string].

# learning_candidate_count(Phrase, Count)
Decl learning_candidate_count(PHRASE, COUNT) bound [/string, /number].

# learning_candidate_ready(Phrase, Verb)
Decl learning_candidate_ready(PHRASE, VERB) bound [/string, /string].

# multistep_verb_pair(Pattern, Verb1, Verb2)
Decl multistep_verb_pair(Pattern, Verb1, Verb2) bound [/string, /name, /name].

# multistep_pattern(Pattern, Category, Relation, Priority)
Decl multistep_pattern(Pattern, Category, Relation, Priority) bound [/string, /name, /name, /number].

# multistep_keyword(Pattern, Keyword)
Decl multistep_keyword(Pattern, Keyword) bound [/string, /string].

# multistep_example(Pattern, Example)
Decl multistep_example(Pattern, Example) bound [/string, /string].

# intent_definition(Sentence, Verb, Target)
# Canonical intent examples for heuristic matching
Decl intent_definition(Sentence, Verb, Target) bound [/string, /name, /string].
Decl intent_category(Sentence, Category) bound [/string, /name].

# =============================================================================
# SECTION 2.2: INTENT QUALIFIERS (Grammar & Modality)
# =============================================================================

# =============================================================================
# TAXONOMY INFERENCE DECLARATIONS (Moved from intent_core.mg)
# =============================================================================
# These predicates support the taxonomy inference engine (inference.mg / taxonomy_inference.mg)

Decl candidate_intent(Verb, RawScore) bound [/name, /number].
Decl context_token(Token) bound [/string].
Decl user_input_string(Input) bound [/string].
Decl boost(Verb, Amount) bound [/name, /number].
Decl penalty(Verb, Amount) bound [/name, /number].
Decl potential_score(Verb, Score) bound [/name, /number].
Decl verb_def(Verb, Category, Shard, Priority) bound [/name, /name, /name, /number].
Decl verb_synonym(Verb, Synonym) bound [/name, /string].
Decl verb_pattern(Verb, Regex) bound [/name, /string].
Decl verb_composition(Verb1, Verb2, Relation, Priority) bound [/name, /name, /name, /number].

# Semantic Matching declarations
Decl semantic_match(UserInput, CanonicalSentence, Verb, Target, Rank, Similarity) bound [/string, /string, /name, /string, /number, /number].
Decl semantic_suggested_verb(Verb, MaxSimilarity) bound [/name, /number].
Decl compound_suggestion(Verb1, Verb2) bound [/name, /name].

# Qualifier taxonomy facts are loaded from schema/intent_qualifiers.mg.
Decl interrogative_type(Word, SemanticType, DefaultVerb, Priority) bound [/string, /name, /name, /number].
Decl modal_type(Word, ModalMeaning, Transformation, Priority) bound [/string, /name, /name, /number].
Decl state_adjective(Adjective, ImpliedVerb, StateCategory, Priority) bound [/string, /name, /name, /number].
Decl negation_marker(Word, NegationType, Priority) bound [/string, /name, /number].
Decl copular_verb(Word, Tense, Number) bound [/string, /name, /number].
Decl existence_pattern(Pattern, QueryType, DefaultVerb, Priority) bound [/string, /name, /name, /number].
Decl comparative_marker(Word, ComparisonType, Priority) bound [/string, /name, /number].
Decl interrogative_state_signal(InterrogType, StateCategory, CombinedVerb, Priority) bound [/name, /name, /name, /number].
Decl modal_verb_signal(ModalMeaning, VerbCategory, ResultingCategory) bound [/name, /name, /name].

# Derived Qualifier Predicates (Moved from policy/taxonomy_qualifiers.mg)
Decl detected_interrogative(Word, SemanticType, DefaultVerb, Priority) bound [/string, /name, /name, /number].
Decl detected_modal(Word, ModalMeaning, Transformation, Priority) bound [/string, /name, /name, /number].
Decl detected_state_adj(Adjective, ImpliedVerb, StateCategory, Priority) bound [/string, /name, /name, /number].
Decl detected_negation(Word, NegationType, Priority) bound [/string, /name, /number].
Decl detected_existence(Pattern, DefaultVerb, Priority) bound [/string, /name, /number].
Decl has_negation(Flag) bound [/name].
Decl has_polite_modal(Flag) bound [/name].
Decl has_hypothetical_modal(Flag) bound [/name].
Decl intent_is_question(Flag) bound [/name].
Decl intent_is_hypothetical(Flag) bound [/name].
Decl intent_is_negated(Flag) bound [/name].
Decl intent_semantic_type(Type) bound [/name].
Decl intent_state_category(Category) bound [/name].

# =============================================================================
# SECTION 3: LLM ROUTING SCHEMA (Used by intent_routing.mg facts)
# =============================================================================

# intent_action_type(ActionType) - Derived action type from intent (e.g. /create, /modify)
Decl intent_action_type(ActionType) bound [/name].

Decl valid_semantic_type(Type, Description) bound [/name, /string].
Decl valid_action_type(Action, Description) bound [/name, /string].
Decl valid_domain(Domain, Description) bound [/name, /string].
Decl valid_scope_level(Level, Order) bound [/number, /number].
Decl valid_mode(Mode, Description) bound [/name, /string].
Decl valid_urgency(Urgency, Order) bound [/name, /number].

Decl mode_from_semantic(SemanticType, Mode, Priority) bound [/name, /name, /number].
Decl mode_from_action(ActionType, Mode, Priority) bound [/name, /name, /number].
Decl mode_from_domain(Domain, Mode, Priority) bound [/name, /name, /number].
Decl mode_from_signal(Signal, Mode, Priority) bound [/name, /name, /number].

Decl context_affinity_semantic(SemanticType, ContextCategory, Weight) bound [/name, /name, /number].
Decl context_affinity_action(ActionType, ContextCategory, Weight) bound [/name, /name, /number].
Decl context_affinity_domain(Domain, ContextCategory, Weight) bound [/name, /name, /number].

Decl shard_affinity_action(ActionType, ShardType, Weight) bound [/name, /name, /number].
Decl shard_affinity_domain(Domain, ShardType, Weight) bound [/name, /name, /number].

Decl tool_affinity_action(ActionType, Tool, Weight) bound [/name, /name, /number].
Decl tool_affinity_domain(Domain, Tool, Weight) bound [/name, /name, /number].

Decl best_mode(Mode, Score) bound [/name, /number].
Decl best_shard(Shard, Score) bound [/name, /number].
Decl context_category_priority(ContextCategory, Score) bound [/name, /number].
Decl tool_priority(Tool, Score) bound [/name, /number].

Decl constraint_type(Constraint, Effect) bound [/name, /name].
Decl constraint_forces_mode(Constraint, Mode) bound [/name, /name].
Decl constraint_blocks_tool(Constraint, Tool) bound [/name, /name].
