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

