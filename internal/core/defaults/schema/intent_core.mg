# Intent Core - Decl statements

Decl intent_definition(Sentence, Verb, Target).
Decl intent_category(Sentence, Category).

Decl multistep_pattern(Pattern, Category, Relation, Priority).
Decl multistep_keyword(Pattern, Keyword).
Decl multistep_verb_pair(Pattern, Verb1, Verb2).
Decl multistep_example(Pattern, Example).
Decl is_multistep_input(Input).
Decl best_multistep_pattern(Input, Pattern, Priority).
Decl pattern_verb_pair(Pattern, Verb1, Verb2).
Decl pattern_relation(Pattern, Relation).

# =============================================================================
# TAXONOMY INFERENCE DECLARATIONS
# =============================================================================
Decl candidate_intent(Verb, RawScore).
Decl context_token(Token).
Decl user_input_string(Input).
Decl boost(Verb, Amount).
Decl penalty(Verb, Amount).
Decl potential_score(Verb, Score).
Decl verb_def(Verb, Category, Shard, Priority).
Decl verb_synonym(Verb, Synonym).
Decl verb_pattern(Verb, Regex).
Decl verb_composition(Verb1, Verb2, Relation, Priority).
Decl semantic_match(UserInput, CanonicalSentence, Verb, Target, Rank, Similarity).
Decl semantic_suggested_verb(Verb, Similarity).
Decl compound_suggestion(Verb1, Verb2).
