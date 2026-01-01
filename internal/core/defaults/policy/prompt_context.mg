# Dynamic Prompt Context Logic
# Section 41 of Cortex Executive Policy

# --- Shard-Specific Context Relevance ---

# Context relevance based on intent match - HIGH relevance (90)
# When shard type matches intent category, target is highly relevant
shard_context_atom(ShardID, Target, 90) :-
    active_shard(ShardID, ShardType),
    user_intent(/current_intent, ShardType, _, Target, _).

# Propagate specialist knowledge to context - HIGH relevance (80)
shard_context_atom(ShardID, Knowledge, 80) :-
    active_shard(ShardID, _),
    specialist_knowledge(ShardID, _, Knowledge).

# Reflection: recent trace recall hits (System 2 memory) - HIGH relevance (85)
shard_context_atom(ShardID, Summary, 85) :-
    active_shard(ShardID, _),
    trace_recall_result(_, Score, _, Summary),
    Score >= 85.

# Reflection: past failures are prioritized slightly higher (90)
shard_context_atom(ShardID, Summary, 90) :-
    active_shard(ShardID, _),
    trace_recall_result(_, Score, /failure, Summary),
    Score >= 85.

# Reflection: learned preferences and patterns - MEDIUM relevance (75)
shard_context_atom(ShardID, Description, 75) :-
    active_shard(ShardID, _),
    learning_recall_result(_, Score, _, Description),
    Score >= 80.

# Include campaign constraints in context - MEDIUM relevance (70)
shard_context_atom(ShardID, Constraint, 70) :-
    active_shard(ShardID, ShardType),
    campaign_active(CampaignID),
    campaign_prompt_policy(CampaignID, ShardType, Constraint).

# Include learned exemplars - MEDIUM relevance (60)
shard_context_atom(ShardID, Exemplar, 60) :-
    active_shard(ShardID, ShardType),
    user_intent(/current_intent, Category, _, _, _),
    prompt_exemplar(ShardType, Category, Exemplar).

# Include relevant tool descriptions - MEDIUM relevance (65)
shard_context_atom(ShardID, ToolDesc, 65) :-
    active_shard(ShardID, ShardType),
    relevant_tool(ShardType, ToolName),
    tool_description(ToolName, ToolDesc).

# Include recent successful trace patterns - LOW relevance (50)
shard_context_atom(ShardID, TracePattern, 50) :-
    active_shard(ShardID, ShardType),
    high_quality_trace(TraceID),
    reasoning_trace(TraceID, ShardType, _, _, /true, _),
    trace_pattern(TraceID, TracePattern).

# --- Injectable Context Selection (Threshold Filtering) ---

# Select injectable context based on relevance threshold (> 50)
injectable_context(ShardID, Atom) :-
    shard_context_atom(ShardID, Atom, Relevance),
    Relevance > 50.

# High-priority injectable context (relevance >= 80)
injectable_context_priority(ShardID, Atom, /high) :-
    shard_context_atom(ShardID, Atom, Relevance),
    Relevance >= 80.

# Medium-priority injectable context (60 <= relevance < 80)
injectable_context_priority(ShardID, Atom, /medium) :-
    shard_context_atom(ShardID, Atom, Relevance),
    Relevance >= 60,
    Relevance < 80.

# Low-priority injectable context (50 < relevance < 60)
injectable_context_priority(ShardID, Atom, /low) :-
    shard_context_atom(ShardID, Atom, Relevance),
    Relevance > 50,
    Relevance < 60.

# --- Context Budget Awareness ---

# Helper: shard has injectable context
has_injectable_context(ShardID) :-
    injectable_context(ShardID, _).

# Helper: shard has high-priority context
has_high_priority_context(ShardID) :-
    injectable_context_priority(ShardID, _, /high).

# When context budget is limited, only inject high-priority items
context_budget_constrained(ShardID) :-
    active_shard(ShardID, _),
    context_budget(ShardID, Budget),
    Budget < 5000.

# Full context injection allowed when budget is sufficient
context_budget_sufficient(ShardID) :-
    active_shard(ShardID, _),
    context_budget(ShardID, Budget),
    Budget >= 5000.

# Final injectable set: all items when budget sufficient
final_injectable(ShardID, Atom) :-
    context_budget_sufficient(ShardID),
    injectable_context(ShardID, Atom).

# Final injectable set: only high priority when budget constrained
final_injectable(ShardID, Atom) :-
    context_budget_constrained(ShardID),
    injectable_context_priority(ShardID, Atom, /high).

# --- Spreading Activation Integration ---

# Boost activation for atoms selected as injectable context
activation(Atom, 95) :-
    final_injectable(_, Atom).

# Boost activation for specialist knowledge atoms
activation(Knowledge, 85) :-
    specialist_knowledge(_, _, Knowledge).

# Boost activation for campaign prompt policy atoms
activation(Constraint, 75) :-
    campaign_active(_),
    campaign_prompt_policy(_, _, Constraint).

# Boost activation for learned exemplars
activation(Exemplar, 70) :-
    prompt_exemplar(_, _, Exemplar).

# --- Context Staleness Detection ---

# Context atom is stale if it references a modified file
context_stale(ShardID, Atom) :-
    shard_context_atom(ShardID, Atom, _),
    modified(Atom),
    !context_refreshed(ShardID, Atom).

# Helper for safe negation
context_refreshed(ShardID, Atom) :-
    shard_context_refreshed(ShardID, Atom, _).

# Context atom is stale if specialist knowledge was updated
context_stale(ShardID, Knowledge) :-
    shard_context_atom(ShardID, Knowledge, _),
    specialist_knowledge(ShardID, _, Knowledge),
    specialist_knowledge_updated(ShardID),
    !context_refreshed(ShardID, Knowledge).

# Helper: shard has stale context
has_stale_context(ShardID) :-
    context_stale(ShardID, _).

# Trigger context refresh when stale atoms detected
next_action(/refresh_shard_context) :-
    active_shard(ShardID, _),
    has_stale_context(ShardID).

# --- Learning Signals from Context Usage ---

# Track when injected context leads to successful task completion
context_injection_effective(ShardID, Atom) :-
    final_injectable(ShardID, Atom),
    shard_executed(ShardID, _, /success, _).

# Learn from effective context injections
learning_signal(/effective_context, Atom) :-
    context_injection_effective(_, Atom).

# Promote frequently effective context to long-term memory
promote_to_long_term(/context_pattern, Atom) :-
    context_injection_effective(S1, Atom),
    context_injection_effective(S2, Atom),
    context_injection_effective(S3, Atom),
    S1 != S2,
    S2 != S3,
    S1 != S3.
