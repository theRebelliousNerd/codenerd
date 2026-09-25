# Cortex 1.5.0 Schemas (EDB Declarations)
# Version: 1.5.0
# Philosophy: Logic determines Reality; the Model merely describes it.

# Modular Schema: TESTING
# Sections: 35, 36, 51

# =============================================================================
# SECTION 35: TASK QUALITY
# =============================================================================
# The verification loop's own predicates (verification_attempt, corrective_*,
# ...) and its policy (policy/verification.mg) were deleted 2026-09-23: nothing
# asserted them. A chat delegation's attempts are decided by delegation_move
# (policy/delegation.mg); the judge's violations reach the retry prompt.

# current_task(TaskID) - the task currently being executed
Decl current_task(TaskID) bound [/string].

# quality_violation(TaskID, ViolationType) - read by campaign_rules.mg
# (quality_violation_detected); nothing asserts it yet.
# ViolationType: /mock_code, /placeholder, /hallucinated_api, /incomplete,
#                /hardcoded, /empty_function, /missing_errors, /fake_tests
Decl quality_violation(TaskID, ViolationType) bound [/string, /name].

# escalation_required(TaskID, Reason) - derived (campaign_rules.mg): must
# escalate to user
Decl escalation_required(TaskID, Reason) bound [/string, /string].

# =============================================================================
# SECTION 36: REASONING TRACES (Shard LLM Interaction History)
# =============================================================================
# Captures LLM interactions from all 4 shard types for self-learning,
# main agent oversight, and cross-shard learning via Mangle rules.

# -----------------------------------------------------------------------------
# 36.1 Core Trace Facts
# -----------------------------------------------------------------------------

# reasoning_trace(TraceID, ShardType, ShardCategory, SessionID, Success, DurationMs)
# Summary of a reasoning trace for policy decisions
# ShardCategory: /system, /ephemeral, /specialist
Decl reasoning_trace(TraceID, ShardType, ShardCategory, SessionID, Success, DurationMs) bound [/string, /name, /name, /string, /name, /number].

# trace_stats(ShardType, SuccessCount, FailCount, AvgDurationMs)
# Exact per-shard aggregate returned by VirtualStore.QueryTraceStats.
Decl trace_stats(ShardType, SuccessCount, FailCount, AvgDurationMs) bound [/name, /number, /number, /number].

# trace_quality(TraceID, Score)
# Quality score assigned after analysis (0.0-1.0)
Decl trace_quality(TraceID, Score) bound [/string, /number].

# trace_error(TraceID, ErrorType)
# Error categorization for learning
Decl trace_error(TraceID, ErrorType) bound [/string, /name].

# trace_task_type(TraceID, TaskType)
# Task type classification for pattern matching
Decl trace_task_type(TraceID, TaskType) bound [/string, /name].

# -----------------------------------------------------------------------------
# 36.2 Shard Performance Patterns
# -----------------------------------------------------------------------------

# shard_reasoning_pattern(ShardType, PatternType, Frequency)
# Detected patterns in shard reasoning (for learning)
# PatternType: /success_pattern, /failure_pattern, /slow_reasoning, /quality_issue
Decl shard_reasoning_pattern(ShardType, PatternType, Frequency) bound [/name, /name, /number].

# trace_insight(TraceID, InsightType, Insight)
# Extracted insights from trace analysis
# InsightType: /approach, /error_pattern, /optimization, /quality_note
Decl trace_insight(TraceID, InsightType, Insight) bound [/string, /name, /string].

# shard_performance(ShardType, SuccessRate, AvgDurationMs, TraceCount)
# Aggregate performance metrics per shard type
Decl shard_performance(ShardType, SuccessRate, AvgDurationMs, TraceCount) bound [/name, /number, /number, /number].

# -----------------------------------------------------------------------------
# 36.3 Cross-Shard Learning
# -----------------------------------------------------------------------------

# specialist_outperforms(SpecialistName, TaskType)
# Tracks when specialists outperform ephemeral shards
Decl specialist_outperforms(SpecialistName, TaskType) bound [/string, /name].

# shard_can_handle(ShardType, TaskType)
# Capability mapping based on trace history
Decl shard_can_handle(ShardType, TaskType) bound [/name, /name].

# shard_switch_suggestion(TaskType, FromShard, ToShard)
# Suggested shard switches based on performance data
Decl shard_switch_suggestion(TaskType, FromShard, ToShard) bound [/name, /name, /name].

# -----------------------------------------------------------------------------
# 36.4 Derived Predicates for Trace Analysis
# -----------------------------------------------------------------------------

# low_quality_trace(TraceID) - derived: trace quality < 50 (on 0-100 scale)
Decl low_quality_trace(TraceID) bound [/string].

# high_quality_trace(TraceID) - derived: trace quality >= 80 (on 0-100 scale)
Decl high_quality_trace(TraceID) bound [/string].


# shard_success_count(ShardType, N) - derived: count of successes per shard type
Decl shard_success_count(ShardType, N) bound [/name, /number].

# shard_struggling(ShardType) - derived: shard has high failure rate (3+)
Decl shard_struggling(ShardType) bound [/name].

# shard_performing_well(ShardType) - derived: shard has high success rate (5+)
Decl shard_performing_well(ShardType) bound [/name].

# has_strategic_advisor - derived: at least one strategic advisor exists
Decl has_strategic_advisor() bound [].

# slow_reasoning_detected(ShardType) - derived: average duration > threshold
Decl slow_reasoning_detected(ShardType) bound [/name].

# learning_from_traces(SignalType, ShardType) - derived: learning signals
# SignalType: /avoid_pattern, /success_pattern, /shard_needs_help
Decl learning_from_traces(SignalType, ShardType) bound [/name, /name].

# suggest_use_specialist(TaskType, SpecialistName) - derived: use specialist
Decl suggest_use_specialist(TaskType, SpecialistName) bound [/name, /string].

# specialist_recommended(ShardName, FilePath, Confidence) - reviewer output
# Emitted when reviewer detects technology patterns matching a specialist shard
Decl specialist_recommended(ShardName, FilePath, Confidence) bound [/string, /string, /number].

# =============================================================================
# SECTION 51: PYTEST DIAGNOSTIC SCHEMA
# =============================================================================
# General-purpose pytest output parsing for any Python project.
# Used by TDD loop, code review, and debugging workflows.

# -----------------------------------------------------------------------------
# 51.1 Core Failure Tracking
# -----------------------------------------------------------------------------

# pytest_failure(TestName, ErrorCategory, RootFile, RootLine, Message)
# ErrorCategory: /assertion, /type, /import, /fixture, /timeout, /attribute, /value, /other
Decl pytest_failure(TestName, ErrorCategory, RootFile, RootLine, Message) bound [/string, /name, /string, /number, /string].

# pytest_error_type(TestName, ErrorTypeString, ErrorCategory)
# Maps Python exception types to categories.
Decl pytest_error_type(TestName, ErrorTypeString, ErrorCategory) bound [/string, /string, /name].

# -----------------------------------------------------------------------------
# 51.2 Assertion Context
# -----------------------------------------------------------------------------

# assertion_mismatch(TestName, Expected, Actual)
# Captures expected vs actual values from assertion failures.
Decl assertion_mismatch(TestName, Expected, Actual) bound [/string, /string, /string].

# assertion_operator(TestName, Operator)
# The comparison operator: ==, !=, in, is, etc.
Decl assertion_operator(TestName, Operator) bound [/string, /string].

# -----------------------------------------------------------------------------
# 51.3 Traceback Analysis
# -----------------------------------------------------------------------------

# traceback_frame(TestName, Depth, File, Line, Function, IsTestFile)
# Depth: 0 = innermost (where exception was raised)
# IsTestFile: /true or /false
Decl traceback_frame(TestName, Depth, File, Line, Function, IsTestFile) bound [/string, /number, /string, /number, /string, /name].

# pytest_root_cause(TestName, FilePath, Line, Function)
# The first non-test file in the traceback (likely source of bug).
Decl pytest_root_cause(TestName, FilePath, Line, Function) bound [/string, /string, /number, /string].

# =============================================================================
# SECTION 52: TEST FRAMEWORK DETECTION
# =============================================================================

# test_framework(FrameworkAtom)
# Framework: /gotest, /pytest, /jest, /junit, /xunit, etc.
Decl test_framework(FrameworkAtom) bound [/name].

# =============================================================================
# SECTION 53: CONTAINERIZED PYTHON ENVIRONMENTS
# =============================================================================
# Asserted by the VirtualStore python_* handlers (internal/core/
# virtual_store_python.go) from a python.Environment running in a persistent
# container. Every fact records work that was done: the handlers fail instead
# of asserting when no environment or container runtime exists.

# python_environment(Project, ContainerID, State, Timestamp)
# State: /ready, /patched, /error, /terminated
Decl python_environment(Project, ContainerID, State, Timestamp) bound [/string, /string, /name, /number].

# python_project_source(Project, GitURL, Commit, Branch)
Decl python_project_source(Project, GitURL, Commit, Branch) bound [/string, /string, /string, /string].

# python_command_executed(Project, Command, ExitCode, Timestamp)
Decl python_command_executed(Project, Command, ExitCode, Timestamp) bound [/string, /string, /number, /number].

# pytest_execution(Project, ArgCount, Timestamp)
Decl pytest_execution(Project, ArgCount, Timestamp) bound [/string, /number, /number].

# python_pytest_result(Project, Passed, ExitCode, Timestamp)
# Passed: /true or /false
Decl python_pytest_result(Project, Passed, ExitCode, Timestamp) bound [/string, /name, /number, /number].

# python_patch_applied(Project, PatchSize, Timestamp)
Decl python_patch_applied(Project, PatchSize, Timestamp) bound [/string, /number, /number].

# python_snapshot(Project, SnapshotName, Timestamp)
Decl python_snapshot(Project, SnapshotName, Timestamp) bound [/string, /string, /number].

# python_restored(Project, SnapshotName, Timestamp)
Decl python_restored(Project, SnapshotName, Timestamp) bound [/string, /string, /number].

# python_teardown_complete(Project, Timestamp)
Decl python_teardown_complete(Project, Timestamp) bound [/string, /number].
