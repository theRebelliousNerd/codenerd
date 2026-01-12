# Intent Qualifier Inference Logic
# This module detects linguistic qualifiers (interrogatives, modals, negation)
# to enhance verb selection.
# Extracted from taxonomy_inference.mg to satisfy Density Rule.

# Import shared declarations from taxonomy_inference.mg (or schema)
Decl context_token(Token).

# EDB Declarations for qualifiers (loaded from schema/intent_qualifiers.mg)
# Decl interrogative_type(Phrase, SemanticType, DefaultVerb, Priority).
# Decl modal_type(Phrase, ModalMeaning, Transformation, Priority).
# Decl state_adjective(Adjective, ImpliedVerb, StateCategory, Priority).
# Decl negation_marker(Word, NegationType, Priority).
# Decl existence_pattern(Pattern, Regex, DefaultVerb, Priority).

# Output Predicates (Used by taxonomy_inference.mg)
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
