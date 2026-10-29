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
# The key is a /string, not a /name: VirtualStore.HydrateLearnings asserts
# toAtomOrString(storedFact.Predicate) over cold_storage rows whose predicate is
# whatever key the memory operation used ("user_preference",
# "prefer_explicit_returns", ...). Those keys never carry a leading "/", so
# toAtomOrString leaves them as string constants.
activation(Tool, 85) :-
    learned_preference("prefer_language", _),
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
# Entity relationships propagate energy along knowledge_link facts that were
# asserted LIVE this session (the ingest and document paths: /has_file,
# /has_chunk, /has_source_doc, and memory operations that record /related_to
# or /depends_on). The stored knowledge graph itself is NOT resident: it is the
# whole world's symbol map (307k rows on this repository, 2026-09-18) and
# loading it at boot filled the kernel's EDB ceiling before the turn began, so
# HydrateLearnings no longer loads it. Its dependency edges are the same
# information as the world model's dependency_link facts, which activation.mg
# rule 4 already spreads along.
#
# Reading the store from here through the external query_knowledge_graph is
# not possible on the pinned engine: an external premise whose input argument
# is a variable bound by an earlier atom panics in EvalExternalQuery
# (topdown.go:99 asserts the input is a Constant; seminaivebottomup.go:831
# passes the premise without applying the substitution). Externals join only
# with constant inputs written in the rule text. Recorded as an engine
# limitation for the M2 "mount SQLite as virtual predicates" pattern.
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

# code_defines and code_calls are cartographer EDB, owned by the world shard
# (internal/world/cartographer.go goSymbolFacts; internal/shards/registration.go).
# Symbol ids are <pkg>.<Name> and <pkg>.<Recv>.<Name>, with the real line span.
# code_calls uses that same id, and so does modified_function
# (internal/core/codedom_modified_symbols.go symbolIDFromRef). impact.mg joins
# the three.
#
# Do not derive either predicate from the fast scan. symbol_graph ids are
# func:<Name> / method:<recv>.<Name> (internal/world/ast_treesitter.go) and
# carry no line span; copying them into code_defines put 0,0 rows in this
# shard, and ShardsFor followed that rule so CortexKernel.Query never read
# the cartographer rows (they were unowned, so they sat in the catch-all).
# dependency_link is a file-to-import edge (schemas_world.mg); copying it
# into code_calls typed a file path as a symbol.

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
