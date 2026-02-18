# Cortex 1.5.0 Schemas (EDB Declarations)
# Version: 1.5.0
# Philosophy: Logic determines Reality; the Model merely describes it.

# Modular Schema: ANALYSIS
# Sections: 12, 13, 14, 15, 16, 37, 39

# =============================================================================
# SECTION 12: SPREADING ACTIVATION (§8.1)
# =============================================================================

# activation(FactID, Score) - declared in Section 7C

# active_goal(Goal)
Decl active_goal(Goal) bound [/string].

# tool_capabilities(Tool, Cap)
# Tool: /fs_read, /fs_write, /exec_cmd, /browser, /code_graph
# Cap: /read, /write, /execute, /navigate, /click, /type, /analyze, /dependencies
Decl tool_capabilities(Tool, Cap) bound [/name, /name].

# has_capability(Cap) - helper for safe negation in missing tool detection
Decl has_capability(Cap) bound [/name].

# goal_requires(Goal, Cap)
Decl goal_requires(Goal, Cap) bound [/string, /name].

# context_atom(Fact) - derived predicate
Decl context_atom(Fact) bound [/string].

# =============================================================================
# SECTION 13: STRATEGY SELECTION (§3.1)
# =============================================================================

# active_strategy(Strategy)
# Strategy: /tdd_repair_loop, /breadth_first_survey, /project_init, /refactor_guard
Decl active_strategy(Strategy) bound [/name].

# target_is_large(Target) - true if target references multiple files/features (Go builtin)
Decl target_is_large(Target) bound [/string].

# target_is_complex(Target) - true if target requires multiple phases (Go builtin)
Decl target_is_complex(Target) bound [/string].

# =============================================================================
# SECTION 14: IMPACT ANALYSIS (§3.3)
# =============================================================================

# impacted(FilePath) - derived predicate
Decl impacted(FilePath) bound [/string].

# unsafe_to_refactor(Target) - derived predicate
Decl unsafe_to_refactor(Target) bound [/string].

# block_refactor(Target, Reason) - derived predicate
Decl block_refactor(Target, Reason) bound [/string, /string].

# block_commit(Reason) - derived predicate
Decl block_commit(Reason) bound [/string].

# =============================================================================
# SECTION 15: ABDUCTIVE REASONING (§8.2)
# =============================================================================

# missing_hypothesis(RootCause)
Decl missing_hypothesis(RootCause) bound [/string].

# clarification_needed(Ref) - derived predicate
Decl clarification_needed(Ref) bound [/string].

# ambiguity_detected(Param) - derived predicate
Decl ambiguity_detected(Param) bound [/string].

# symptom(Context, SymptomType)
Decl symptom(Context, SymptomType) bound [/string, /name].

# known_cause(SymptomType, Cause)
Decl known_cause(SymptomType, Cause) bound [/name, /string].

# has_known_cause(SymptomType) - helper for safe negation
Decl has_known_cause(SymptomType) bound [/name].

# =============================================================================
# SECTION 16: AUTOPOIESIS / LEARNING (§8.3)
# =============================================================================

# rejection_count(Pattern, Count)
Decl rejection_count(Pattern, Count) bound [/string, /number].

# preference_signal(Pattern) - derived predicate
Decl preference_signal(Pattern) bound [/string].

# derived_rule(Pattern, FactType, FactValue) - maps rejection patterns to facts for promotion
Decl derived_rule(Pattern, FactType, FactValue) bound [/string, /name, /string].

# promote_to_long_term(FactType, FactValue) - derived predicate for Autopoiesis (§8.3)
# FactType is a name constant (e.g., /style_preference, /avoid_pattern)
# FactValue is the specific value to learn
Decl promote_to_long_term(FactType, FactValue) bound [/name, /string].

# =============================================================================
# SECTION 37: HOLOGRAPHIC CODE GRAPH (Cartographer)
# =============================================================================
# Rich structural facts extracted by Cartographer (§NextGen-1)

# code_defines(File, Symbol, Type, StartLine, EndLine)
# Type: /function, /struct, /interface, /type
Decl code_defines(File, Symbol, Type, StartLine, EndLine) bound [/string, /string, /name, /number, /number].

# code_calls(Caller, Callee)
# Represents dynamic call graph
Decl code_calls(Caller, Callee) bound [/string, /string].

# code_implements(Struct, Interface)
# Represents structural typing relationships
Decl code_implements(Struct, Interface) bound [/string, /string].

# relevant_context(Content) - derived: content relevant to current intent target
# Used by Holographic Retrieval (Cartographer) for X-Ray Vision
Decl relevant_context(Content) bound [/string].

# =============================================================================
# SECTION 39: EXTENDED METRICS (Aggregation)
# =============================================================================
# These facts capture shard execution results and make them available to the
# main agent's context in subsequent turns. Solves the "lost context" problem
# where shard outputs were displayed but not persisted for later reference.

# -----------------------------------------------------------------------------
# 34B.1 Shard Execution Facts
# -----------------------------------------------------------------------------

# shard_executed(ShardID, ShardType, Task, Timestamp)
# Records that a shard was executed with a specific task
Decl shard_executed(ShardID, ShardType, Task, Timestamp) bound [/string, /name, /string, /number].

# shard_output(ShardID, Output)
# The raw output from shard execution (may be truncated)
Decl shard_output(ShardID, Output) bound [/string, /string].

# shard_success(ShardID)
# Marks successful shard execution
Decl shard_success(ShardID) bound [/string].

# shard_error(ShardID, ErrorMessage)
# Records shard execution failure
Decl shard_error(ShardID, ErrorMessage) bound [/string, /string].

# -----------------------------------------------------------------------------
# 34B.2 Review Findings (from ReviewerShard)
# -----------------------------------------------------------------------------

# review_finding(File, Line, Severity, Category, Message)
# Individual findings from code review (emitted by Go ReviewerShard)
# Severity: /critical, /error, /warning, /info
# NOTE: There's also a 6-arg version in Section 41.12 for feedback loop:
# review_finding(ReviewID, File, Line, Severity, Category, Message)
Decl review_finding(File, Line, Severity, Category, Message) bound [/string, /number, /name, /name, /string].

# review_summary(ShardID, Critical, Errors, Warnings, Info)
# Summary counts from a review execution
Decl review_summary(ShardID, Critical, Errors, Warnings, Info) bound [/string, /number, /number, /number, /string].

# review_metrics(ShardID, TotalLines, CodeLines, CommentLines, FunctionCount)
# Code metrics from review
Decl review_metrics(ShardID, TotalLines, CodeLines, CommentLines, FunctionCount) bound [/string, /number, /number, /number, /number].

# security_finding(ShardID, Severity, FilePath, Line, RuleID, Message)
# Security-specific findings
Decl security_finding(ShardID, Severity, FilePath, Line, RuleID, Message) bound [/string, /name, /string, /number, /string, /string].

# -----------------------------------------------------------------------------
# 34B.3 Test Results (from TesterShard)
# -----------------------------------------------------------------------------

# test_result(ShardID, TestName, Passed, Duration)
# Individual test results
Decl test_result(ShardID, TestName, Passed, Duration) bound [/string, /string, /name, /number].

# test_summary(ShardID, Total, Passed, Failed, Skipped)
# Summary of test execution
Decl test_summary(ShardID, Total, Passed, Failed, Skipped) bound [/string, /number, /number, /number, /number].

# -----------------------------------------------------------------------------
# 34B.4 Recent Shard Context (for LLM injection)
# -----------------------------------------------------------------------------

# recent_shard_context(ShardType, Task, Summary, Timestamp)
# Compressed context from recent shard executions for LLM injection
Decl recent_shard_context(ShardType, Task, Summary, Timestamp) bound [/name, /string, /string, /number].

# last_shard_execution(ShardID, ShardType, Task)
# The most recent shard execution (for quick reference)
Decl last_shard_execution(ShardID, ShardType, Task) bound [/string, /name, /string].

# -----------------------------------------------------------------------------
# 34B.5 Derived Predicates
# -----------------------------------------------------------------------------

# has_recent_shard_output(ShardType) - derived: there's recent output from this shard type
Decl has_recent_shard_output(ShardType) bound [/name].

# shard_findings_available() - derived: there are findings to reference
Decl shard_findings_available().

