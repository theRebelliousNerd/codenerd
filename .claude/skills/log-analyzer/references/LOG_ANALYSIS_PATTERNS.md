# Log Analysis Patterns Reference

Extended Mangle patterns for analyzing codeNERD system logs. This reference supplements the patterns in SKILL.md with advanced techniques for debugging complex issues.

## Table of Contents

1. [Core Patterns](#core-patterns)
2. [Temporal Analysis](#temporal-analysis)
3. [Category-Specific Patterns](#category-specific-patterns)
4. [Performance Analysis](#performance-analysis)
5. [Error Diagnosis](#error-diagnosis)
6. [Session Reconstruction](#session-reconstruction)
7. [Anomaly Detection](#anomaly-detection)

---

## Core Patterns

### Pattern: Log Entry Basics

```mangle
# Core fact structure
# log_entry(Timestamp_ms, Category, Level, Message, Filename, LineNumber)

# Filter by level
errors_only(T, C, M) :- log_entry(T, C, /error, M, _, _).
warnings_only(T, C, M) :- log_entry(T, C, /warn, M, _, _).
info_only(T, C, M) :- log_entry(T, C, /info, M, _, _).
debug_only(T, C, M) :- log_entry(T, C, /debug, M, _, _).

# Filter by category
kernel_logs(T, L, M) :- log_entry(T, /kernel, L, M, _, _).
shard_logs(T, L, M) :- log_entry(T, /shards, L, M, _, _).
perception_logs(T, L, M) :- log_entry(T, /perception, L, M, _, _).
```

### Pattern: Category Statistics

```mangle
# Entry count per category
category_stats(Cat, Total, Errors, Warnings) :-
    log_entry(_, Cat, _, _, _, _) |>
    do fn:group_by(Cat),
    let Total = fn:Count(),
    # Count errors
    error_entry(_, Cat, _) |>
    do fn:group_by(Cat),
    let Errors = fn:Count(),
    # Count warnings
    warning_entry(_, Cat, _) |>
    do fn:group_by(Cat),
    let Warnings = fn:Count().

# Most active categories
active_categories(Cat, Count) :-
    log_entry(_, Cat, _, _, _, _) |>
    do fn:group_by(Cat),
    let Count = fn:Count().
```

### Pattern: Level Distribution

```mangle
# Count by level across all categories
level_distribution(Level, Count) :-
    log_entry(_, _, Level, _, _, _) |>
    do fn:group_by(Level),
    let Count = fn:Count().

# Level distribution per category
category_level_dist(Cat, Level, Count) :-
    log_entry(_, Cat, Level, _, _, _) |>
    do fn:group_by(Cat, Level),
    let Count = fn:Count().
```

---

## Temporal Analysis

### Pattern: Time Windows

```mangle
# Events in a specific time window (timestamps in ms)
in_window(T, C, L, M, Start, End) :-
    log_entry(T, C, L, M, _, _),
    T >= Start,
    T <= End.

# Events in the last N milliseconds from max timestamp
recent_events(T, C, L, M, WindowMs) :-
    log_entry(MaxT, _, _, _, _, _) |>
    let MaxT = fn:Max(T),
    log_entry(T, C, L, M, _, _),
    fn:minus(MaxT, T) <= WindowMs.
```

### Pattern: Event Ordering

```mangle
# Consecutive events (same category)
consecutive_in_category(T1, M1, T2, M2, Cat) :-
    log_entry(T1, Cat, _, M1, _, _),
    log_entry(T2, Cat, _, M2, _, _),
    T2 > T1,
    !between_event(T1, T2, Cat).

between_event(T1, T2, Cat) :-
    log_entry(T, Cat, _, _, _, _),
    T > T1,
    T < T2.
```

### Pattern: Session Timeline

```mangle
# Session boundaries (boot events mark starts)
session_start(T) :-
    log_entry(T, /boot, _, M, _, _),
    :string_contains(M, "Initialized").

# Session duration
session_duration(StartT, EndT, DurationMs) :-
    session_start(StartT),
    log_entry(_, _, _, _, _, _) |>
    let EndT = fn:Max(T),
    DurationMs = fn:minus(EndT, StartT).
```

### Pattern: Event Gaps

```mangle
# Find gaps larger than threshold (potential hangs/blocks)
event_gap(T1, C1, T2, C2, GapMs) :-
    log_entry(T1, C1, _, _, _, _),
    log_entry(T2, C2, _, _, _, _),
    T2 > T1,
    GapMs = fn:minus(T2, T1),
    GapMs > 5000.  # 5 second threshold
```

---

## Category-Specific Patterns

### Pattern: Kernel Derivation Tracking

```mangle
# Fact assertions
kernel_assert(T, Predicate) :-
    log_entry(T, /kernel, _, M, _, _),
    :string_contains(M, "Asserting fact"),
    :extract_predicate(M, Predicate).

# Rule derivations
kernel_derive(T, Predicate) :-
    log_entry(T, /kernel, _, M, _, _),
    :string_contains(M, "Derived"),
    :extract_predicate(M, Predicate).

# Failed derivations
kernel_fail(T, Predicate, Reason) :-
    log_entry(T, /kernel, /error, M, _, _),
    :string_contains(M, "Failed to derive"),
    :extract_predicate(M, Predicate),
    :extract_reason(M, Reason).
```

### Pattern: Shard Lifecycle

```mangle
# Shard spawns
shard_spawn(T, ShardType, ShardId) :-
    log_entry(T, /shards, _, M, _, _),
    :string_contains(M, "Spawning"),
    :extract_shard_type(M, ShardType),
    :extract_shard_id(M, ShardId).

# Shard completions
shard_complete(T, ShardType, ShardId, DurationMs) :-
    log_entry(T, /shards, _, M, _, _),
    :string_contains(M, "completed"),
    :extract_shard_type(M, ShardType),
    :extract_shard_id(M, ShardId),
    :extract_duration(M, DurationMs).

# Shard failures
shard_fail(T, ShardType, ShardId, Error) :-
    log_entry(T, /shards, /error, M, _, _),
    :extract_shard_type(M, ShardType),
    :extract_shard_id(M, ShardId),
    :extract_error(M, Error).

# Shard execution time
shard_execution(ShardType, SpawnT, CompleteT, DurationMs) :-
    shard_spawn(SpawnT, ShardType, Id),
    shard_complete(CompleteT, ShardType, Id, _),
    DurationMs = fn:minus(CompleteT, SpawnT).
```

### Pattern: Perception/Articulation Flow

```mangle
# Intent extraction
intent_extracted(T, Category, Verb, Target) :-
    log_entry(T, /perception, _, M, _, _),
    :string_contains(M, "Extracted intent"),
    :extract_intent(M, Category, Verb, Target).

# Atom generation
atoms_generated(T, Count) :-
    log_entry(T, /perception, _, M, _, _),
    :string_contains(M, "Generated"),
    :string_contains(M, "atoms"),
    :extract_count(M, Count).

# Response articulation
response_articulated(T, TokenCount) :-
    log_entry(T, /articulation, _, M, _, _),
    :string_contains(M, "Articulated"),
    :extract_token_count(M, TokenCount).

# Full request flow
request_flow(PerceptionT, ArticulationT, LatencyMs) :-
    intent_extracted(PerceptionT, _, _, _),
    response_articulated(ArticulationT, _),
    ArticulationT > PerceptionT,
    LatencyMs = fn:minus(ArticulationT, PerceptionT).
```

### Pattern: API Call Tracking

```mangle
# API requests
api_request(T, Model, TokenCount) :-
    log_entry(T, /api, _, M, _, _),
    :string_contains(M, "Request"),
    :extract_model(M, Model),
    :extract_tokens(M, TokenCount).

# API responses
api_response(T, Model, DurationMs, OutputTokens) :-
    log_entry(T, /api, _, M, _, _),
    :string_contains(M, "Response"),
    :extract_model(M, Model),
    :extract_duration(M, DurationMs),
    :extract_output_tokens(M, OutputTokens).

# API errors
api_error(T, Model, ErrorCode, ErrorMsg) :-
    log_entry(T, /api, /error, M, _, _),
    :extract_model(M, Model),
    :extract_error_code(M, ErrorCode),
    :extract_error_msg(M, ErrorMsg).
```

---

## Performance Analysis

### Pattern: Slow Operations

```mangle
# Operations exceeding threshold
slow_op(T, Cat, Op, DurationMs, ThresholdMs) :-
    log_entry(T, Cat, _, M, _, _),
    :extract_operation(M, Op),
    :extract_duration(M, DurationMs),
    DurationMs > ThresholdMs.

# Categorize by severity
perf_severity(T, Cat, Op, Duration, /critical) :-
    slow_op(T, Cat, Op, Duration, _), Duration > 10000.
perf_severity(T, Cat, Op, Duration, /warning) :-
    slow_op(T, Cat, Op, Duration, _), Duration > 1000, Duration <= 10000.
perf_severity(T, Cat, Op, Duration, /minor) :-
    slow_op(T, Cat, Op, Duration, _), Duration > 100, Duration <= 1000.
```

### Pattern: Bottleneck Detection

```mangle
# Average operation time by category
avg_time_by_category(Cat, AvgMs) :-
    log_entry(T, Cat, _, M, _, _),
    :extract_duration(M, Duration) |>
    do fn:group_by(Cat),
    let AvgMs = fn:Avg(Duration).

# Operations blocking others (gap analysis)
blocking_operation(T1, Cat1, T2, Cat2, BlockMs) :-
    log_entry(T1, Cat1, _, M1, _, _),
    :extract_operation(M1, _),
    log_entry(T2, Cat2, _, _, _, _),
    T2 > T1,
    BlockMs = fn:minus(T2, T1),
    BlockMs > 1000,
    Cat1 != Cat2.
```

### Pattern: Throughput Analysis

```mangle
# Events per second (bucket by second)
events_per_second(BucketSec, Count) :-
    log_entry(T, _, _, _, _, _),
    BucketSec = fn:div(T, 1000) |>
    do fn:group_by(BucketSec),
    let Count = fn:Count().

# Peak activity detection
peak_activity(BucketSec, Count) :-
    events_per_second(BucketSec, Count),
    events_per_second(_, MaxCount) |>
    let MaxCount = fn:Max(Count),
    Count = MaxCount.
```

---

## Error Diagnosis

### Pattern: Error Classification

```mangle
# Classify errors by type
error_type(T, Cat, /timeout) :-
    error_entry(T, Cat, M),
    :string_contains(M, "timeout").

error_type(T, Cat, /connection) :-
    error_entry(T, Cat, M),
    :string_contains(M, "connection").

error_type(T, Cat, /parse) :-
    error_entry(T, Cat, M),
    :string_contains(M, "parse").

error_type(T, Cat, /derivation) :-
    error_entry(T, Cat, M),
    :string_contains(M, "derive").

error_type(T, Cat, /other) :-
    error_entry(T, Cat, M),
    !error_type(T, Cat, /timeout),
    !error_type(T, Cat, /connection),
    !error_type(T, Cat, /parse),
    !error_type(T, Cat, /derivation).

# Error type frequency
error_type_freq(Type, Count) :-
    error_type(_, _, Type) |>
    do fn:group_by(Type),
    let Count = fn:Count().
```

### Pattern: Error Chains

```mangle
# Errors that follow each other (potential chain reactions)
error_chain(E1T, E1Cat, E2T, E2Cat, GapMs) :-
    error_entry(E1T, E1Cat, _),
    error_entry(E2T, E2Cat, _),
    E2T > E1T,
    GapMs = fn:minus(E2T, E1T),
    GapMs < 500.  # Within 500ms

# Transitive error propagation
error_propagates(E1T, E1Cat, E3T, E3Cat) :-
    error_chain(E1T, E1Cat, E2T, _),
    error_chain(E2T, _, E3T, E3Cat).

# Root cause (first error in chain)
root_error(T, Cat, Msg) :-
    error_entry(T, Cat, Msg),
    !error_chain(_, _, T, Cat).
```

### Pattern: Error Context Window

```mangle
# All events in the 1 second before an error
pre_error_context(ErrorT, ErrorCat, EventT, EventCat, EventLevel, EventMsg) :-
    error_entry(ErrorT, ErrorCat, _),
    log_entry(EventT, EventCat, EventLevel, EventMsg, _, _),
    EventT < ErrorT,
    fn:minus(ErrorT, EventT) < 1000.

# Specific category context before error
kernel_before_error(ErrorT, ErrorCat, KernelT, KernelMsg) :-
    error_entry(ErrorT, ErrorCat, _),
    log_entry(KernelT, /kernel, _, KernelMsg, _, _),
    KernelT < ErrorT,
    fn:minus(ErrorT, KernelT) < 1000.
```

---

## Session Reconstruction

### Pattern: Request-Response Pairs

```mangle
# Match perception (request start) with articulation (response end)
request_response(ReqT, RespT, LatencyMs) :-
    log_entry(ReqT, /perception, _, ReqMsg, _, _),
    :string_contains(ReqMsg, "Processing"),
    log_entry(RespT, /articulation, _, RespMsg, _, _),
    :string_contains(RespMsg, "Complete"),
    RespT > ReqT,
    LatencyMs = fn:minus(RespT, ReqT).
```

### Pattern: Turn Reconstruction

```mangle
# User turn boundaries
turn_start(T, TurnId) :-
    log_entry(T, /session, _, M, _, _),
    :string_contains(M, "Turn"),
    :extract_turn_id(M, TurnId).

# Events within a turn
turn_event(TurnId, T, Cat, Level, Msg) :-
    turn_start(StartT, TurnId),
    turn_start(NextT, NextTurnId),
    NextTurnId = fn:plus(TurnId, 1),
    log_entry(T, Cat, Level, Msg, _, _),
    T >= StartT,
    T < NextT.
```

### Pattern: Campaign Tracking

```mangle
# Campaign phase transitions
campaign_phase(T, CampaignId, Phase) :-
    log_entry(T, /campaign, _, M, _, _),
    :extract_campaign_id(M, CampaignId),
    :extract_phase(M, Phase).

# Campaign completion
campaign_complete(CampaignId, StartT, EndT, DurationMs) :-
    campaign_phase(StartT, CampaignId, /started),
    campaign_phase(EndT, CampaignId, /completed),
    DurationMs = fn:minus(EndT, StartT).

# Failed campaigns
campaign_failed(CampaignId, T, Reason) :-
    campaign_phase(T, CampaignId, /failed),
    log_entry(T, /campaign, /error, M, _, _),
    :extract_reason(M, Reason).
```

---

## Anomaly Detection

### Pattern: Unusual Event Frequency

```mangle
# Categories with unusually high error rates
high_error_rate(Cat, ErrorRate) :-
    entry_count(Cat, Total),
    error_count(Cat, Errors),
    ErrorRate = fn:div(fn:mult(Errors, 100), Total),
    ErrorRate > 10.  # More than 10% errors
```

### Pattern: Missing Expected Events

```mangle
# Categories that should have events but don't
expected_category(/kernel).
expected_category(/perception).
expected_category(/articulation).
expected_category(/shards).

missing_category(Cat) :-
    expected_category(Cat),
    !log_entry(_, Cat, _, _, _, _).
```

### Pattern: Out-of-Order Events

```mangle
# Articulation before perception (shouldn't happen)
out_of_order_articulation(ArtT, PercT) :-
    log_entry(ArtT, /articulation, _, _, _, _),
    log_entry(PercT, /perception, _, _, _, _),
    ArtT < PercT.

# Response before request
response_before_request(T) :-
    log_entry(T, /api, _, M, _, _),
    :string_contains(M, "Response"),
    !api_request(ReqT, _, _),
    ReqT < T.
```

### Pattern: Repeated Errors

```mangle
# Same error message repeated (potential loop)
repeated_error(Msg, Count) :-
    error_entry(_, _, Msg) |>
    do fn:group_by(Msg),
    let Count = fn:Count(),
    Count > 3.

# Rapid-fire errors (more than 5 in 1 second)
error_burst(BucketSec, Count) :-
    error_entry(T, _, _),
    BucketSec = fn:div(T, 1000) |>
    do fn:group_by(BucketSec),
    let Count = fn:Count(),
    Count > 5.
```

---

## Debugging Workflows

### Workflow 1: "Why did this fail?"

1. Find the error: `?error_entry(T, Cat, Msg).`
2. Get context: `?pre_error_context(T, Cat, EventT, EventCat, Level, EventMsg).`
3. Check for cascades: `?error_chain(_, _, T, Cat).`
4. Find root cause: `?root_error(RootT, RootCat, RootMsg).`

### Workflow 2: "Why is it slow?"

1. Find slow operations: `?slow_op(T, Cat, Op, Duration, 1000).`
2. Check bottlenecks: `?blocking_operation(T1, Cat1, T2, Cat2, BlockMs).`
3. Analyze throughput: `?events_per_second(Sec, Count).`
4. Find peak load: `?peak_activity(Sec, Count).`

### Workflow 3: "What happened during this session?"

1. Get session boundaries: `?session_start(T).`
2. List all categories: `?category_event(Cat, _, _).`
3. Reconstruct turns: `?turn_event(TurnId, T, Cat, Level, Msg).`
4. Check for anomalies: `?missing_category(Cat).`

### Workflow 4: "Is the kernel working correctly?"

1. Check assertions: `?kernel_assert(T, Pred).`
2. Check derivations: `?kernel_derive(T, Pred).`
3. Find failures: `?kernel_fail(T, Pred, Reason).`
4. Correlate with other systems: `?correlated(KernelT, /kernel, OtherT, OtherCat).`

---

## Helper Predicates

These predicates require custom implementation in the parser or virtual store:

```mangle
# String operations (implemented in parser)
Decl :string_contains(Str.Type<string>, Substr.Type<string>).
Decl :extract_predicate(Str.Type<string>, Pred.Type<name>).
Decl :extract_duration(Str.Type<string>, DurationMs.Type<int>).
Decl :extract_count(Str.Type<string>, Count.Type<int>).
Decl :extract_shard_type(Str.Type<string>, ShardType.Type<name>).
Decl :extract_shard_id(Str.Type<string>, ShardId.Type<string>).
Decl :extract_error(Str.Type<string>, Error.Type<string>).
Decl :extract_intent(Str.Type<string>, Cat.Type<name>, Verb.Type<name>, Target.Type<string>).
Decl :extract_model(Str.Type<string>, Model.Type<string>).
Decl :extract_tokens(Str.Type<string>, Tokens.Type<int>).
Decl :extract_turn_id(Str.Type<string>, TurnId.Type<int>).
Decl :extract_campaign_id(Str.Type<string>, CampaignId.Type<string>).
Decl :extract_phase(Str.Type<string>, Phase.Type<name>).
Decl :extract_reason(Str.Type<string>, Reason.Type<string>).
```

---

## See Also

- [SKILL.md](../SKILL.md) - Main skill documentation
- [log-schema.mg](../assets/log-schema.mg) - Complete Mangle schema
- [mangle-programming](../../mangle-programming/SKILL.md) - Full Mangle reference
