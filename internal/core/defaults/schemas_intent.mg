# Cortex 1.5.0 Schemas (EDB Declarations)
# Version: 1.5.0
# Philosophy: Logic determines Reality; the Model merely describes it.

# Modular Schema: INTENT
# Sections: 1, 2

# =============================================================================
# SECTION 1: INTENT SCHEMA (§1.1)
# =============================================================================

# user_intent(ID, Category, Verb, Target, Constraint)
# ID: /current_intent for the interactive turn, /task_intent_N for a SubAgent
#     run. A NAME constant, not a string: every Go producer emits one, either
#     explicitly (transducer.go ToFact, session/executor.go ProcessWithIntent
#     use types.MangleAtom) or implicitly (chat/process.go, chat/process_seed.go,
#     shards/system/perception.go and context/serializer.go pass the bare Go
#     string "/current_intent", which Fact.ToAtom promotes to a name), and every
#     rule body in the corpus matches the /current_intent literal. This slot is
#     pure EDB — no .mg head asserts user_intent — so the Decl was the only place
#     the disagreement could show, and nothing read it.
# Category: /query, /mutation, /instruction
# Verb: /explain, /refactor, /debug, /generate, /scaffold, /init, /test, /review, /fix, /run, /research, /explore, /implement
# Target: /string. Usually a file path or a free-form noun phrase from the
#     user's words; the sub-command rules in policy/capabilities.mg match it as
#     a quoted literal ("setup", "test", "patch"), never as a name.
# Priority: 100
# SerializationOrder: 1
Decl user_intent(ID, Category, Verb, Target, Constraint) bound [/name, /name, /name, /string, /string].

# multi_step_signal(Signal) - EDB asserted by Go.
# Step 5: the multi-step CLASSIFICATION decision moves to policy, while the
# regex/keyword/verb-count EXTRACTION stays in Go (cmd/nerd/chat/delegation.go:
# detectMultiStepTask, and the decompose corpus in schema/intent_multi_step.mg).
# Go computes each signal from the (quote-stripped) input and asserts one fact
# per detected signal; Mangle ORs them into is_multi_step.
# Signal: /campaign_verb, /keyword_match, /verb_count_high, /compound_pattern
Decl multi_step_signal(Signal) bound [/name].

# is_multi_step() - derived: the current request should be decomposed into
# multiple steps. Queried by Go, which falls back to the legacy Go boolean if the
# kernel is unavailable or returns nothing.
Decl is_multi_step() bound [].

# intent_signal(Signal) - EDB asserted by Go per turn (retract-before-assert).
# Carries perception's boolean understanding signals into policy so routing
# arbitration can reason over them.
# Signal: /is_question (user wants an answer, not work performed)
Decl intent_signal(Signal) bound [/name].

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
# IntentID: the same /name intent id user_intent carries. Both producers pass it
#   as a bare Go string — shards/system/router.go takes it out of the pending
#   action payload, shards/system/executive_intent.go copies intent.ID off a
#   user_intent readback — and its value is "/current_intent", which Fact.ToAtom
#   promotes to a name constant. Declaring it /string made this the one relation
#   whose reader disagreed with it: clarification.mg copies IntentID straight
#   into clarification_question/1, declared /name.
# Reason: /unmapped_verb, /no_route, /blocked_by_constitution, /ooda_timeout, /no_action_derived
Decl no_action_reason(INTENTID, REASON) bound [/name, /name].

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
# IntentID is a name constant: the interactive turn owns /current_intent and
# delegated runs get /task_intent_N (session/executor.go asserts both as
# types.MangleAtom). Question is free-form user-facing text.
Decl clarification_question(INTENTID, QUESTION) bound [/name, /string].

# clarification_option(IntentID, OptionVerb, OptionLabel)
# OptionVerb is drawn from the verb vocabulary (/explain, /fix, /learn_yes...);
# OptionLabel is the free-form text shown to the user.
Decl clarification_option(INTENTID, OPTIONVERB, OPTIONLABEL) bound [/name, /name, /string].

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
# Number here is GRAMMATICAL number (/singular, /plural, /neutral), not a count.
Decl copular_verb(Word, Tense, Number) bound [/string, /name, /name].
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
# Level is the scope name (/line ... /codebase); Order is its ordinal rank.
# Same shape as valid_urgency(Urgency, Order) below.
Decl valid_scope_level(Level, Order) bound [/name, /number].
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
