# Knowledge Atom Integration & Retrieval
# Section 17, 17B, 25 of Cortex Executive Policy


# Section 17: Knowledge Atom Integration

# When high-confidence knowledge about the domain exists
# Knowledge atoms inform strategy selection (confidence on 0-100 scale)
active_strategy(/domain_expert) :-
    knowledge_atom(_, _, _, Confidence),
    Confidence > 80,
    user_intent(/current_intent, _, _, _, _).

# Section 17B: Learned Knowledge Application

# 1. User preferences influence tool selection
# If user prefers a language, boost activation for related tools
activation(Tool, 85) :-
    learned_preference(/prefer_language, _),
    tool_capabilities(Tool, /code_generation),
    tool_language(Tool, _).

# 2. Learned constraints become safety checks
# Constraints from knowledge.db feed into constitutional logic
constraint_violation(Action, Reason) :-
    learned_constraint(Predicate, Args),
    action_violates(Action, Predicate, Args),
    Reason = Args.

# 3. User facts inform context
# Facts about the user/project activate relevant context
context_atom(fn:pair(Pred, Args)) :-
    learned_fact(Pred, Args),
    relevant_to_intent(Pred, Intent),
    user_intent(/current_intent, _, _, _, Intent).

# 4. Knowledge graph links spread activation
# Entity relationships from knowledge_graph propagate energy
activation(EntityB, 60) :-
    knowledge_link(EntityA, /related_to, EntityB),
    activation(EntityA, Score),
    Score > 50.

activation(EntityB, 70) :-
    knowledge_link(EntityA, /depends_on, EntityB),
    activation(EntityA, Score),
    Score > 40.

# 5. High-activation facts boost related content
# Recent activations from activation_log inform focus
context_priority(FactID, 80) :-
    activation(FactID, Score),
    Score > 70.

# 6. Session continuity - recent turns inform context
# Session history provides conversational context
context_atom(UserInput) :-
    session_turn(_, TurnNum, UserInput, _),
    TurnNum > 0.

# 7. Similar content retrieval for semantic search
# Vector recall results inform related context
related_context(Content) :-
    similar_content(Rank, Content),
    Rank < 5.

# Section 25: Holographic Retrieval (Cartographer)

# Bridge world model facts to holographic schema (Fix 15.6)
code_defines(File, SymbolID, Type, 0, "") :-
    symbol_graph(SymbolID, Type, _, File, _).

code_calls(CallerID, CalleeID) :-
    dependency_link(CallerID, CalleeID, _).

# 1. Callers of the target symbol
relevant_context(File) :-
    user_intent(/current_intent, _, _, TargetSymbol, _),
    code_calls(Caller, TargetSymbol),
    code_defines(File, Caller, _, _, _).

# 2. Definitions in the target file
relevant_context(Symbol) :-
    user_intent(/current_intent, _, _, TargetFile, _),
    code_defines(TargetFile, Symbol, _, _, _).

# 3. Implementations of target interface
relevant_context(Struct) :-
    user_intent(/current_intent, _, _, Interface, _),
    code_implements(Struct, Interface).

# 4. Structs implementing the target interface (if target is interface)
relevant_context(StructFile) :-
    user_intent(/current_intent, _, _, Interface, _),
    code_implements(Struct, Interface),
    code_defines(StructFile, Struct, _, _, _).

# Boost activation for holographic matches
activation(Ctx, 85) :-
    relevant_context(Ctx).
