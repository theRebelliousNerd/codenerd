# Shard Types, Trace Analysis & Specialist Classification
# Section 18, 18B, 24 of Cortex Executive Policy


# Section 18: Shard Type Classification

# Type 1: System Level - Always on, high reliability
shard_type(/system, /permanent, /high_reliability).

# Type 2: Ephemeral - Fast spawning, RAM only
shard_type(/ephemeral, /spawn_die, /speed_optimized).

# Type 3: Persistent LLM-Created - Background tasks, SQLite
shard_type(/persistent, /long_running, /adaptive).

# Type 4: User Configured - Deep domain knowledge
shard_type(/user, /explicit, /user_defined).

# Model capability mapping for shards
shard_model_config(/system, /high_reasoning).
shard_model_config(/ephemeral, /high_speed).
shard_model_config(/persistent, /balanced).
shard_model_config(/user, /high_reasoning).

# Helper predicates for safe negation
# Use has_active_shard(Type) instead of "!active_shard(Type, _)" to avoid unbound variables
has_active_shard(ShardType) :- active_shard(_, ShardType).

# Section 24: Reasoning Trace Policy

# Trace Quality Tracking

# Low quality trace (needs review) - score on 0-100 scale
low_quality_trace(TraceID) :-
    trace_quality(TraceID, Score),
    Score < 50.

# High quality trace (good for learning) - score on 0-100 scale
high_quality_trace(TraceID) :-
    trace_quality(TraceID, Score),
    Score >= 80.

# Shard Performance Patterns

# Shard has high failure rate (3+ consecutive failures)
shard_struggling(ShardType) :-
    reasoning_trace(T1, ShardType, _, _, /false, _),
    reasoning_trace(T2, ShardType, _, _, /false, _),
    reasoning_trace(T3, ShardType, _, _, /false, _),
    T1 != T2,
    T2 != T3,
    T1 != T3.

# Shard is performing well (5+ consecutive successes)
shard_performing_well(ShardType) :-
    reasoning_trace(T1, ShardType, _, _, /true, _),
    reasoning_trace(T2, ShardType, _, _, /true, _),
    reasoning_trace(T3, ShardType, _, _, /true, _),
    reasoning_trace(T4, ShardType, _, _, /true, _),
    reasoning_trace(T5, ShardType, _, _, /true, _),
    T1 != T2,
    T2 != T3,
    T3 != T4,
    T4 != T5.

# Detect slow reasoning (> 30 seconds)
slow_reasoning_detected(ShardType) :-
    reasoning_trace(_, ShardType, _, _, _, DurationMs),
    DurationMs > 30000.

# Learning Signals from Traces

# Learn from repeated failures - shard needs help
learning_from_traces(/shard_needs_help, ShardType) :-
    shard_struggling(ShardType).

# Learn from success patterns
learning_from_traces(/success_pattern, ShardType) :-
    shard_performing_well(ShardType).

# Learn from slow traces (performance issue)
learning_from_traces(/slow_reasoning, ShardType) :-
    slow_reasoning_detected(ShardType).

# Promote learning signals to long-term memory
promote_to_long_term(/shard_pattern, ShardType) :-
    learning_from_traces(_, ShardType).

# Cross-Shard Learning (Specialist vs Ephemeral)

# Specialist outperforms ephemeral for same task type
specialist_outperforms(SpecialistName, TaskType) :-
    reasoning_trace(T1, SpecialistName, /specialist, _, /true, _),
    reasoning_trace(T2, /coder, /ephemeral, _, /false, _),
    trace_task_type(T1, TaskType),
    trace_task_type(T2, TaskType).

# Suggest using specialist instead of ephemeral
suggest_use_specialist(TaskType, SpecialistName) :-
    specialist_outperforms(SpecialistName, TaskType).

# Suggest switching shard when current one struggles
shard_switch_suggestion(TaskType, CurrentShard, AlternateShard) :-
    shard_struggling(CurrentShard),
    shard_performing_well(AlternateShard),
    shard_can_handle(AlternateShard, TaskType).

# Trace-Based Context Enhancement

# Boost activation for successful trace patterns in current session
activation(TraceID, 80) :-
    high_quality_trace(TraceID),
    reasoning_trace(TraceID, ShardType, _, SessionID, /true, _),
    session_state(SessionID, /active, _).

# Suppress failed trace patterns
activation(TraceID, -30) :-
    low_quality_trace(TraceID),
    reasoning_trace(TraceID, _, _, _, /false, _).

# Corrective Actions Based on Traces

# Escalate if multiple shards struggling
escalation_needed(/system_health, /shard_performance, "Multiple shards struggling") :-
    shard_struggling(Shard1),
    shard_struggling(Shard2),
    Shard1 != Shard2.

# Suggest spawning researcher for failed traces with unknown errors
delegate_task(/researcher, TaskContext, /pending) :-
    reasoning_trace(TraceID, _, _, _, /false, _),
    trace_error(TraceID, /unknown),
    trace_task_type(TraceID, TaskContext).

# System Shard Trace Monitoring

# System shard traces get special attention
activation(TraceID, 90) :-
    reasoning_trace(TraceID, _, /system, _, _, _).

# Alert on system shard failures
escalation_needed(/system_health, ShardType, "System shard failure") :-
    reasoning_trace(_, ShardType, /system, _, /false, _).

# Specialist Knowledge Hydration from Traces

# Specialist with good traces should be preferred for similar tasks
delegate_task(SpecialistName, Task, /pending) :-
    shard_performing_well(SpecialistName),
    shard_profile(SpecialistName, /specialist, _),
    trace_task_type(_, TaskType),
    shard_can_handle(SpecialistName, TaskType),
    user_intent(/current_intent, _, _, Task, _).

# Learn which tasks specialists handle well
shard_can_handle(ShardType, TaskType) :-
    reasoning_trace(TraceID, ShardType, /specialist, _, /true, _),
    trace_task_type(TraceID, TaskType),
    high_quality_trace(TraceID).


# =============================================================================
# Section 18B: Specialist Classification System
# =============================================================================
# Specialists are classified by execution mode and knowledge tier.
# This determines whether they can execute directly or must advise.

# Specialist Execution Modes
# /executor - Can write/modify code in their domain
# /advisor  - Provides guidance, cannot execute
# /observer - Background monitoring only

# Specialist Knowledge Tiers
# /technical - Implementation expertise (how to code)
# /strategic - Architectural guidance (what to code)
# /domain    - Project-specific knowledge (why we code this way)

# Executor Specialists (Technical Tier) - Can write code directly
specialist_classification(/goexpert, /executor, /technical).
specialist_classification(/bubbleteaexpert, /executor, /technical).
specialist_classification(/cobraexpert, /executor, /technical).
specialist_classification(/rodexpert, /executor, /technical).
specialist_classification(/mangleexpert, /executor, /technical).

# Advisor Specialists (Strategic Tier) - Guide but don't execute
specialist_classification(/securityauditor, /advisor, /strategic).
specialist_classification(/testarchitect, /advisor, /strategic).

# Observer Specialists (Strategic Tier) - Background monitoring
specialist_classification(/northstar, /observer, /strategic).

# Specialist Knowledge DB Paths (for JIT context hydration)
specialist_knowledge_db(/goexpert, "goexpert_knowledge.db").
specialist_knowledge_db(/bubbleteaexpert, "bubbleteaexpert_knowledge.db").
specialist_knowledge_db(/cobraexpert, "cobraexpert_knowledge.db").
specialist_knowledge_db(/rodexpert, "rodexpert_knowledge.db").
specialist_knowledge_db(/mangleexpert, "mangleexpert_knowledge.db").
specialist_knowledge_db(/securityauditor, "securityauditor_knowledge.db").
specialist_knowledge_db(/testarchitect, "testarchitect_knowledge.db").
specialist_knowledge_db(/northstar, "northstar_knowledge.db").

# Campaign Integration Roles
# /phase_executor  - Executes during implementation phases
# /plan_reviewer   - Reviews campaign plans
# /background_mon  - Background monitoring

specialist_campaign_role(/goexpert, /phase_executor).
specialist_campaign_role(/bubbleteaexpert, /phase_executor).
specialist_campaign_role(/cobraexpert, /phase_executor).
specialist_campaign_role(/rodexpert, /phase_executor).
specialist_campaign_role(/mangleexpert, /phase_executor).
specialist_campaign_role(/securityauditor, /plan_reviewer).
specialist_campaign_role(/testarchitect, /plan_reviewer).
specialist_campaign_role(/northstar, /alignment_guardian).

# Determine if specialist can execute (not just advise)
specialist_can_execute(Specialist) :-
    specialist_classification(Specialist, /executor, _).

# Determine if specialist should execute directly
# High confidence (>0.8) on an executor specialist triggers direct execution
specialist_should_execute(Specialist, Task) :-
    specialist_match(Specialist, Task, Confidence),
    specialist_can_execute(Specialist),
    Confidence > 80.

# Determine if specialist should advise instead
specialist_should_advise(Specialist, Task) :-
    specialist_match(Specialist, Task, Confidence),
    Confidence > 40,
    Confidence <= 80.

# Always consult strategic advisors for complex tasks
strategic_advisor_required(Task) :-
    task_complexity(Task, /high),
    specialist_classification(Advisor, /advisor, /strategic).

# Route to specialist's knowledge DB for context
specialist_context_source(Specialist, DBPath) :-
    specialist_knowledge_db(Specialist, DBPath).

# Activate specialist for campaign phase based on role
# campaign_phase(PhaseID, CampaignID, Name, Order, Status, ContextProfile) - 6 args
activate_specialist_for_phase(Specialist, Phase) :-
    specialist_campaign_role(Specialist, /phase_executor),
    campaign_phase(Phase, _, /implementation, _, _, _).

activate_specialist_for_phase(Specialist, Phase) :-
    specialist_campaign_role(Specialist, /plan_reviewer),
    campaign_phase(Phase, _, /planning, _, _, _).

# Cross-Specialist Collaboration
# Strategic advisors can assist technical executors

specialist_assists(Advisor, Executor) :-
    specialist_classification(Advisor, /advisor, /strategic),
    specialist_classification(Executor, /executor, /technical).

# Specialist consultation request routing
specialist_consultation_route(FromSpec, ToSpec, Question) :-
    consultation_request(FromSpec, ToSpec, Question, _),
    specialist_classification(ToSpec, _, _).

# Derive specialist tools from classification
specialist_allowed_tools(Specialist, /write_file) :-
    specialist_can_execute(Specialist).

specialist_allowed_tools(Specialist, /edit_file) :-
    specialist_can_execute(Specialist).

specialist_allowed_tools(Specialist, /run_command) :-
    specialist_classification(Specialist, _, _).

specialist_allowed_tools(Specialist, /read_file) :-
    specialist_classification(Specialist, _, _).

specialist_allowed_tools(Specialist, /list_files) :-
    specialist_classification(Specialist, _, _).

specialist_allowed_tools(Specialist, /glob) :-
    specialist_classification(Specialist, _, _).
