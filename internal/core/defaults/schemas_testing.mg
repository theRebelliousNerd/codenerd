# Cortex 1.5.0 Schemas (EDB Declarations)
# Version: 1.5.0
# Philosophy: Logic determines Reality; the Model merely describes it.

# Modular Schema: TESTING
# Sections: 35, 36, 51

# =============================================================================
# SECTION 35: VERIFICATION LOOP (Post-Execution Quality Enforcement)
# =============================================================================
# Tracks task verification attempts, quality violations, and corrective actions.
# Enables the agent to retry with context enrichment until success or escalation.

# -----------------------------------------------------------------------------
# 35.1 Verification State Tracking
# -----------------------------------------------------------------------------

# verification_attempt(TaskID, AttemptNum, Success)
# Tracks each verification attempt for a task
# Success: /success, /failure
Decl verification_attempt(TaskID, AttemptNum, Success) bound [/string, /number, /name].

# current_task(TaskID) - the task currently being executed
Decl current_task(TaskID) bound [/string].

# verification_result(TaskID, AttemptNum, Confidence, Reason)
# Detailed verification result per attempt
Decl verification_result(TaskID, AttemptNum, Confidence, Reason) bound [/string, /number, /number, /string].

# -----------------------------------------------------------------------------
# 35.2 Quality Violation Detection
# -----------------------------------------------------------------------------

# quality_violation(TaskID, ViolationType)
# ViolationType: /mock_code, /placeholder, /hallucinated_api, /incomplete,
#                /hardcoded, /empty_function, /missing_errors, /fake_tests
Decl quality_violation(TaskID, ViolationType) bound [/string, /name].

# quality_violation_evidence(TaskID, ViolationType, Evidence)
# Specific evidence of the violation (e.g., line number, code snippet)
Decl quality_violation_evidence(TaskID, ViolationType, Evidence) bound [/string, /name, /string].

# quality_score(TaskID, AttemptNum, Score)
# Overall quality score (0.0-1.0) for the attempt
Decl quality_score(TaskID, AttemptNum, Score) bound [/string, /number, /number].

# -----------------------------------------------------------------------------
# 35.3 Corrective Action Tracking
# -----------------------------------------------------------------------------

# corrective_action_taken(TaskID, ActionType)
# ActionType: /research, /docs, /tool, /decompose
Decl corrective_action_taken(TaskID, ActionType) bound [/string, /name].

# corrective_context(TaskID, AttemptNum, ContextType, Context)
# Additional context gathered through corrective action
# ContextType: /research_result, /documentation, /tool_output, /decomposition
Decl corrective_context(TaskID, AttemptNum, ContextType, Context) bound [/string, /number, /name, /string].

# corrective_query(TaskID, AttemptNum, Query)
# The query used for corrective action (e.g., research query)
Decl corrective_query(TaskID, AttemptNum, Query) bound [/string, /number, /string].

# -----------------------------------------------------------------------------
# 35.4 Shard Selection Tracking
# -----------------------------------------------------------------------------

# shard_selected(TaskID, AttemptNum, ShardType, SelectionReason)
# Tracks which shard was selected for each attempt
Decl shard_selected(TaskID, AttemptNum, ShardType, SelectionReason) bound [/string, /number, /name, /string].

# shard_selection_confidence(TaskID, AttemptNum, ShardType, Confidence)
# Confidence score for shard selection
Decl shard_selection_confidence(TaskID, AttemptNum, ShardType, Confidence) bound [/string, /number, /name, /number].

# -----------------------------------------------------------------------------
# 35.5 Verification Derived Predicates
# -----------------------------------------------------------------------------

# verification_blocked(TaskID) - derived: max retries reached
Decl verification_blocked(TaskID) bound [/string].

# verification_succeeded(TaskID) - derived: task passed verification
Decl verification_succeeded(TaskID) bound [/string].

# has_quality_violation(TaskID) - derived: task has any quality violation
Decl has_quality_violation(TaskID) bound [/string].

# needs_corrective_action(TaskID) - derived: task needs correction
Decl needs_corrective_action(TaskID) bound [/string].

# escalation_required(TaskID, Reason) - derived: must escalate to user
Decl escalation_required(TaskID, Reason) bound [/string, /string].

# first_attempt_success(TaskID) - derived: task succeeded on first verification attempt
Decl first_attempt_success(TaskID) bound [/string].

# required_retry(TaskID) - derived: task required retries before passing
Decl required_retry(TaskID) bound [/string].

# violation_type_count_high(ViolationType) - derived: violation type occurs frequently (5+)
Decl violation_type_count_high(ViolationType) bound [/name].

# corrective_action_effective(TaskID, ActionType) - derived: corrective action improved result
Decl corrective_action_effective(TaskID, ActionType) bound [/string, /name].

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

# shard_struggling(ShardType) - derived: shard has high failure rate
Decl shard_struggling(ShardType) bound [/name].

# shard_performing_well(ShardType) - derived: shard has high success rate
Decl shard_performing_well(ShardType) bound [/name].

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
