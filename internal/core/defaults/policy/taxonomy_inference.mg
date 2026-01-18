
# Inference Logic for Intent Refinement
# This module takes raw intent candidates (from regex/LLM) and refines them
# using contextual logic and safety constraints.

Decl candidate_intent(Verb, RawScore).
Decl context_token(Token).
Decl user_input_string(Input).

# Import learned patterns
# Decl learned_exemplar imported from schema/learning.mg

Decl boost(Verb, Amount).
Decl penalty(Verb, Amount).

# EDB Declarations for data loaded from Go
Decl verb_def(Verb, Category, Shard, Priority).
Decl verb_synonym(Verb, Synonym).
Decl verb_pattern(Verb, Regex).

# Intermediate score generation
Decl potential_score(Verb, Score).

# 1. Base Score
# Convert float score to int for calculation if needed, but here we just pass it.
# candidate_intent RawScore is already scaled to int64 by engine.go if it was > 1.0.
potential_score(Verb, Score) :- candidate_intent(Verb, Score).

# Learned Pattern Override (Highest Priority)
# If the input matches a learned pattern, give it a massive boost.
potential_score(Verb, 100) :-
    user_input_string(Input),
    learned_exemplar(Pattern, Verb, _, _, _),
    Input = Pattern.

# 2. Boosted Scores (Rule-based)
# Use integer arithmetic for scores (0-100 scale).
potential_score(Verb, NewScore) :-
    candidate_intent(Verb, Base),
    Verb = /security,
    context_token("security"),
    NewScore = fn:plus(Base, 30).

potential_score(Verb, NewScore) :-
    candidate_intent(Verb, Base),
    Verb = /security,
    context_token("vulnerability"),
    NewScore = fn:plus(Base, 30).

potential_score(Verb, NewScore) :-
    candidate_intent(Verb, Base),
    Verb = /test,
    context_token("coverage"),
    NewScore = fn:plus(Base, 20).

potential_score(Verb, NewScore) :-
    candidate_intent(Verb, Base),
    Verb = /debug,
    context_token("panic"),
    NewScore = fn:plus(Base, 15).

potential_score(Verb, NewScore) :-
    candidate_intent(Verb, Base),
    Verb = /debug,
    context_token("stacktrace"),
    NewScore = fn:plus(Base, 15).

potential_score(Verb, NewScore) :-
    candidate_intent(Verb, Base),
    Verb = /test,
    context_token("verify"),
    NewScore = fn:plus(Base, 20).

# Note: Aggregation (finding max score) is now handled in Go code to avoid
# "no modes declared" errors caused by complex negation in Mangle.

# =============================================================================
# SEMANTIC MATCHING INFERENCE
# =============================================================================
# These rules use semantic_match facts to influence verb selection.

# EDB declarations for semantic matching (facts asserted by SemanticClassifier)
Decl semantic_match(UserInput, CanonicalSentence, Verb, Target, Rank, Similarity).
Decl verb_composition(Verb1, Verb2, ComposedAction, Priority).

# Derived predicates for semantic matching
Decl semantic_suggested_verb(Verb, Similarity).
Decl compound_suggestion(Verb1, Verb2).

# Derive suggested verbs from semantic matches (top 3 only, similarity >= 60)
semantic_suggested_verb(Verb, Similarity) :-
    semantic_match(_, _, Verb, _, Rank, Similarity),
    Rank <= 3,
    Similarity >= 60.

# HIGH-CONFIDENCE SEMANTIC OVERRIDE
# If rank 1 match has similarity >= 85, override to max score
potential_score(Verb, 100) :-
    semantic_match(_, _, Verb, _, 1, Similarity),
    Similarity >= 85.

# MEDIUM-CONFIDENCE SEMANTIC BOOST
# Rank 1-3 with similarity 70-84 get +30 boost
potential_score(Verb, NewScore) :-
    candidate_intent(Verb, Base),
    semantic_match(_, _, Verb, _, Rank, Similarity),
    Rank <= 3,
    Similarity >= 70,
    Similarity < 85,
    NewScore = fn:plus(Base, 30).

# LOW-CONFIDENCE SEMANTIC BOOST
# Rank 1-5 with similarity 60-69 get +15 boost
potential_score(Verb, NewScore) :-
    candidate_intent(Verb, Base),
    semantic_match(_, _, Verb, _, Rank, Similarity),
    Rank <= 5,
    Similarity >= 60,
    Similarity < 70,
    NewScore = fn:plus(Base, 15).

# VERB COMPOSITION FROM MULTIPLE MATCHES
# If two different verbs both have high similarity, suggest composition
compound_suggestion(V1, V2) :-
    semantic_suggested_verb(V1, S1),
    semantic_suggested_verb(V2, S2),
    V1 != V2,
    S1 >= 65,
    S2 >= 65,
    verb_composition(V1, V2, _, Priority),
    Priority >= 80.

# LEARNED PATTERN PRIORITY
# Semantic matches from learned patterns (detected by constraint presence)
# get additional boost - these represent user-specific preferences
potential_score(Verb, NewScore) :-
    semantic_match(_, Sentence, Verb, _, 1, Similarity),
    Similarity >= 70,
    learned_exemplar(Sentence, Verb, _, _, _),
    candidate_intent(Verb, Base),
    NewScore = fn:plus(Base, 40).

# =============================================================================
# INTENT QUALIFIER INFERENCE
# =============================================================================
# These rules use the intent qualifiers (interrogatives, modals, copular states,
# negation) to enhance verb selection beyond simple pattern matching.

# --- Derived predicates for qualifier detection ---
Decl detected_interrogative(Word, SemanticType, DefaultVerb, Priority).
Decl detected_modal(Word, ModalMeaning, Transformation, Priority).
Decl detected_state_adj(Adjective, ImpliedVerb, StateCategory, Priority).
Decl detected_negation(Word, NegationType, Priority).
Decl detected_existence(Pattern, DefaultVerb, Priority).
Decl has_negation(Flag).
Decl has_polite_modal(Flag).
Decl has_hypothetical_modal(Flag).

# --- Detect interrogatives from context tokens ---
# Single-word tokens are often atomized if they are identifiers (like 'where', 'is')
detected_interrogative(Word, SemanticType, DefaultVerb, Priority) :-
    context_token(Word),
    interrogative_type(Word, SemanticType, DefaultVerb, Priority).

# Two-word interrogatives (check for both tokens present)
detected_interrogative(Phrase, SemanticType, DefaultVerb, Priority) :-
    context_token(/what),
    context_token(/is),
    interrogative_type("what is", SemanticType, DefaultVerb, Priority),
    Phrase = "what is".

detected_interrogative(Phrase, SemanticType, DefaultVerb, Priority) :-
    context_token(/what),
    context_token(/if),
    interrogative_type("what if", SemanticType, DefaultVerb, Priority),
    Phrase = "what if".

detected_interrogative(Phrase, SemanticType, DefaultVerb, Priority) :-
    context_token(/why),
    context_token(/is),
    interrogative_type("why is", SemanticType, DefaultVerb, Priority),
    Phrase = "why is".

detected_interrogative(Phrase, SemanticType, DefaultVerb, Priority) :-
    context_token(/why),
    context_token(/does),
    interrogative_type("why does", SemanticType, DefaultVerb, Priority),
    Phrase = "why does".

detected_interrogative(Phrase, SemanticType, DefaultVerb, Priority) :-
    context_token(/how),
    context_token(/do),
    context_token(/i),
    interrogative_type("how do i", SemanticType, DefaultVerb, Priority),
    Phrase = "how do i".

detected_interrogative(Phrase, SemanticType, DefaultVerb, Priority) :-
    context_token(/how),
    context_token(/can),
    context_token(/i),
    interrogative_type("how can i", SemanticType, DefaultVerb, Priority),
    Phrase = "how can i".

detected_interrogative(Phrase, SemanticType, DefaultVerb, Priority) :-
    context_token(/where),
    context_token(/is),
    interrogative_type("where is", SemanticType, DefaultVerb, Priority),
    Phrase = "where is".

detected_interrogative(Phrase, SemanticType, DefaultVerb, Priority) :-
    context_token(/who),
    context_token(/wrote),
    interrogative_type("who wrote", SemanticType, DefaultVerb, Priority),
    Phrase = "who wrote".

detected_interrogative(Phrase, SemanticType, DefaultVerb, Priority) :-
    context_token(/which),
    context_token(/file),
    interrogative_type("which file", SemanticType, DefaultVerb, Priority),
    Phrase = "which file".

detected_interrogative(Phrase, SemanticType, DefaultVerb, Priority) :-
    context_token(/which),
    context_token(/files),
    interrogative_type("which files", SemanticType, DefaultVerb, Priority),
    Phrase = "which files".

# --- Detect modals from context tokens ---
detected_modal(Word, ModalMeaning, Transformation, Priority) :-
    context_token(Word),
    modal_type(Word, ModalMeaning, Transformation, Priority).

# Two-word modals
detected_modal(Phrase, ModalMeaning, Transformation, Priority) :-
    context_token(/can),
    context_token(/you),
    modal_type("can you", ModalMeaning, Transformation, Priority),
    Phrase = "can you".

detected_modal(Phrase, ModalMeaning, Transformation, Priority) :-
    context_token(/could),
    context_token(/you),
    modal_type("could you", ModalMeaning, Transformation, Priority),
    Phrase = "could you".

detected_modal(Phrase, ModalMeaning, Transformation, Priority) :-
    context_token(/would),
    context_token(/you),
    modal_type("would you", ModalMeaning, Transformation, Priority),
    Phrase = "would you".

detected_modal(Phrase, ModalMeaning, Transformation, Priority) :-
    context_token(/help),
    context_token(/me),
    modal_type("help me", ModalMeaning, Transformation, Priority),
    Phrase = "help me".

detected_modal(Phrase, ModalMeaning, Transformation, Priority) :-
    context_token(/what),
    context_token(/if),
    modal_type("what if", ModalMeaning, Transformation, Priority),
    Phrase = "what if".

# --- Detect state adjectives from context tokens ---
detected_state_adj(Adjective, ImpliedVerb, StateCategory, Priority) :-
    context_token(Adjective),
    state_adjective(Adjective, ImpliedVerb, StateCategory, Priority).

# --- Detect negation from context tokens ---
detected_negation(Word, NegationType, Priority) :-
    context_token(Word),
    negation_marker(Word, NegationType, Priority).

# Flag if any negation is present (use /true sentinel for boolean)
has_negation(/true) :-
    detected_negation(_, _, _).

# Flag if polite modal is present (use /true sentinel for boolean)
has_polite_modal(/true) :-
    detected_modal(_, /polite_request, _, _).

# Flag if hypothetical modal is present (use /true sentinel for boolean)
has_hypothetical_modal(/true) :-
    detected_modal(_, /hypothetical, _, _).

# --- Detect existence patterns ---
detected_existence(Pattern, DefaultVerb, Priority) :-
    context_token(/is),
    context_token(/there),
    existence_pattern("is there", _, DefaultVerb, Priority),
    Pattern = "is there".

detected_existence(Pattern, DefaultVerb, Priority) :-
    context_token(/are),
    context_token(/there),
    existence_pattern("are there", _, DefaultVerb, Priority),
    Pattern = "are there".

detected_existence(Pattern, DefaultVerb, Priority) :-
    context_token(/do),
    context_token(/we),
    context_token(/have),
    existence_pattern("do we have", _, DefaultVerb, Priority),
    Pattern = "do we have".

# =============================================================================
# QUALIFIER-ENHANCED VERB SCORING
# =============================================================================

# --- NEGATION BLOCKING (Highest Priority) ---
# If negation + verb detected, DO NOT select that verb
# Instead, convert to an instruction intent
Decl negated_verb(Verb).
negated_verb(Verb) :-
    has_negation(/true),
    context_token(VerbWord),
    verb_synonym(Verb, VerbWord).

# Negated verbs get negative score (effectively blocked)
potential_score(Verb, -100) :-
    negated_verb(Verb).

# When negation present, boost /instruction or /explain instead
potential_score(/explain, 85) :-
    has_negation(/true),
    negated_verb(_).

# --- MODAL STRIPPING (High Priority) ---
# "Can you review this?" -> strip modal, boost /review
# This fires when polite modal + verb synonym detected
potential_score(Verb, 95) :-
    has_polite_modal(/true),
    context_token(VerbWord),
    verb_synonym(Verb, VerbWord),
    !negated_verb(Verb).

# --- HYPOTHETICAL MODE (High Priority) ---
# "What if I deleted this?" -> boost /dream
potential_score(/dream, 92) :-
    has_hypothetical_modal(/true).

# --- COPULAR + STATE ADJECTIVE (High Priority) ---
# "Is this code secure?" -> /security
# Requires copular verb + state adjective in context
Decl copular_state_intent(ImpliedVerb, Priority).
copular_state_intent(ImpliedVerb, Priority) :-
    context_token(Copular),
    copular_verb(Copular, _, _),
    detected_state_adj(_, ImpliedVerb, _, Priority).

# Helper predicates for safe negation (wildcards in negated atoms cause safety violations)
Decl has_copular_state_intent(Flag).
has_copular_state_intent(/true) :- copular_state_intent(_, _).

Decl has_candidate_intent(Flag).
has_candidate_intent(/true) :- candidate_intent(_, _).

potential_score(Verb, Score) :-
    copular_state_intent(Verb, BasePriority),
    !has_negation(/true),
    Score = fn:plus(BasePriority, 5).

# --- INTERROGATIVE + STATE COMBINATION (Very High Priority) ---
# "Why is this failing?" -> causation + error_state -> /debug
Decl interrogative_state_combo(CombinedVerb, Priority).
interrogative_state_combo(CombinedVerb, Priority) :-
    detected_interrogative(_, InterrogType, _, _),
    detected_state_adj(_, _, StateCategory, _),
    interrogative_state_signal(InterrogType, StateCategory, CombinedVerb, Priority).

Decl has_interrogative_state_combo(Flag).
has_interrogative_state_combo(/true) :- interrogative_state_combo(_, _).

potential_score(Verb, Score) :-
    interrogative_state_combo(Verb, Priority),
    !has_negation(/true),
    Score = fn:plus(Priority, 2).

# --- PURE INTERROGATIVE FALLBACK (Medium Priority) ---
# If interrogative detected but no verb match, use interrogative's default verb
Decl pure_interrogative_intent(DefaultVerb, Priority).
pure_interrogative_intent(DefaultVerb, Priority) :-
    detected_interrogative(_, _, DefaultVerb, Priority),
    !has_polite_modal(/true),
    !has_copular_state_intent(/true),
    !has_interrogative_state_combo(/true).

potential_score(Verb, Score) :-
    pure_interrogative_intent(Verb, Priority),
    !has_candidate_intent(/true),
    !has_negation(/true),
    Score = Priority.

# --- EXISTENCE QUERIES (Medium Priority) ---
# "Is there a config file?" -> /search
potential_score(DefaultVerb, Score) :-
    detected_existence(_, DefaultVerb, Priority),
    !has_negation(/true),
    Score = Priority.

# =============================================================================
# INTENT METADATA DERIVATION
# =============================================================================
# Derive additional metadata about the intent for routing decisions.
# Note: Mangle requires at least one argument per predicate; use /true sentinel for booleans.

Decl intent_is_question(Flag).
Decl intent_is_hypothetical(Flag).
Decl intent_is_negated(Flag).
Decl intent_semantic_type(Type).
Decl intent_state_category(Category).

intent_is_question(/true) :-
    detected_interrogative(_, _, _, _).

intent_is_hypothetical(/true) :-
    has_hypothetical_modal(/true).

intent_is_negated(/true) :-
    has_negation(/true).

intent_semantic_type(Type) :-
    detected_interrogative(_, Type, _, _).

intent_state_category(Category) :-
    detected_state_adj(_, _, Category, _).
