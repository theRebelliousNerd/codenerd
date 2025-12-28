# Shard Types & Trace Analysis
# Section 18, 24 of Cortex Executive Policy


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
