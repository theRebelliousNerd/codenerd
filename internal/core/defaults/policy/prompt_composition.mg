# Dynamic Prompt Composition & Context
# Section 41, 42 of Cortex Executive Policy


# Shard-Specific Context Relevance (3-arity extension)
# shard_context_atom(ShardID, Atom, Relevance) - context relevance per shard
# Relevance is integer 0-100 scale (Mangle doesn't support floats)

# Context relevance based on intent match - HIGH relevance (90)
# When shard type matches intent category, target is highly relevant
shard_context_atom(ShardID, Target, 90) :-
    active_shard(ShardID, ShardType),
    user_intent(/current_intent, ShardType, _, Target, _).

# Propagate specialist knowledge to context - HIGH relevance (80)
shard_context_atom(ShardID, Knowledge, 80) :-
    active_shard(ShardID, _),
    specialist_knowledge(ShardID, _, Knowledge).

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

# Injectable Context Selection (Threshold Filtering)

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

# Context Budget Awareness (for context window management)

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

# Spreading Activation Integration

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

# Context Staleness Detection

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

# Learning Signals from Context Usage

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

# SECTION 42: NORTHSTAR VISION REASONING

# Critical Path Derivation

# Derive critical capabilities (priority = /critical)
critical_capability(CapID) :-
    northstar_capability(CapID, _, _, /critical).

# Derive high-risk items (both likelihood AND impact are high)
high_risk(RiskID) :-
    northstar_risk(RiskID, _, /high, /high).

# Helper: risk has at least one mitigation
has_mitigation(RiskID) :-
    northstar_mitigation(RiskID, _).

# Derive unmitigated risks (high risk without any mitigation)
unmitigated_risk(RiskID) :-
    high_risk(RiskID),
    !has_mitigation(RiskID).

# Alignment Analysis

# Capability addresses persona need when serves relationship exists
capability_addresses_need(CapID, PersonaID, Need) :-
    northstar_serves(CapID, PersonaID),
    northstar_need(PersonaID, Need).

# Helper: persona is served by at least one capability
is_served_persona(PersonaID) :-
    northstar_serves(_, PersonaID).

# Helper: capability serves at least one persona
capability_is_linked(CapID) :-
    northstar_serves(CapID, _).

# Unserved persona - has needs but no capability serves them
unserved_persona(PersonaID, Name) :-
    northstar_persona(PersonaID, Name),
    northstar_need(PersonaID, _),
    !is_served_persona(PersonaID).

# Orphan capability - not linked to any persona
orphan_capability(CapID, Desc) :-
    northstar_capability(CapID, Desc, _, _),
    !capability_is_linked(CapID).

# Requirements Traceability

# Must-have requirements (priority = /must_have)
must_have_requirement(ReqID, Desc) :-
    northstar_requirement(ReqID, _, Desc, /must_have).

# Helper: requirement is supported by at least one capability
is_supported_req(ReqID) :-
    northstar_supports(ReqID, _).

# Orphan requirement - not linked to any capability
orphan_requirement(ReqID, Desc) :-
    northstar_requirement(ReqID, _, Desc, _),
    !is_supported_req(ReqID).

# Risk-addressing requirement
risk_addressing_requirement(ReqID, RiskID) :-
    northstar_addresses(ReqID, RiskID),
    high_risk(RiskID).

# Helper: risk is addressed by at least one requirement
risk_is_addressed(RiskID) :-
    northstar_addresses(_, RiskID).

# Unaddressed high risk - no requirement addresses it
unaddressed_high_risk(RiskID, Desc) :-
    high_risk(RiskID),
    northstar_risk(RiskID, Desc, _, _),
    !risk_is_addressed(RiskID).

# Timeline Planning

# Immediate work (timeline = /now)
immediate_capability(CapID, Desc) :-
    northstar_capability(CapID, Desc, /now, _).

# Near-term work (timeline = /6mo)
near_term_capability(CapID, Desc) :-
    northstar_capability(CapID, Desc, /6mo, _).

# Long-term work (timeline = /1yr or /3yr)
long_term_capability(CapID, Desc) :-
    northstar_capability(CapID, Desc, /1yr, _).

long_term_capability(CapID, Desc) :-
    northstar_capability(CapID, Desc, /3yr, _).

# Moonshot capabilities (timeline = /moonshot)
moonshot_capability(CapID, Desc) :-
    northstar_capability(CapID, Desc, /moonshot, _).

# Strategic Warnings

# Warning: critical capability with unmitigated high risk
strategic_warning(/critical_unmitigated_risk, CapID, RiskID) :-
    critical_capability(CapID),
    northstar_supports(ReqID, CapID),
    northstar_addresses(ReqID, RiskID),
    unmitigated_risk(RiskID).

# Warning: immediate work depends on unaddressed risk
strategic_warning(/immediate_risk_gap, CapID, RiskID) :-
    immediate_capability(CapID, _),
    unaddressed_high_risk(RiskID, _).

# Context Injection for Northstar

# Inject mission when planning or deciding actions
injectable_context(/northstar_mission, Mission) :-
    northstar_defined(),
    northstar_mission(_, Mission),
    active_shard(ShardID, _),
    shard_family(ShardID, /planner).

injectable_context(/northstar_mission, Mission) :-
    northstar_defined(),
    northstar_mission(_, Mission),
    active_shard(ShardID, _),
    shard_family(ShardID, /coder).

# Inject critical capabilities during planning
injectable_context(/critical_cap, Desc) :-
    northstar_defined(),
    critical_capability(CapID),
    northstar_capability(CapID, Desc, _, _),
    active_shard(ShardID, _),
    shard_family(ShardID, /planner).

# Inject unmitigated risks as warnings
injectable_context(/unmitigated_risk_warning, Desc) :-
    northstar_defined(),
    unmitigated_risk(RiskID),
    northstar_risk(RiskID, Desc, _, _).

# Inject constraints always
injectable_context(/constraint, Desc) :-
    northstar_defined(),
    northstar_constraint(_, Desc).
