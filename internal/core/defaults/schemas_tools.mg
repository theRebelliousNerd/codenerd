# Cortex 1.5.0 Schemas (EDB Declarations)
# Version: 1.5.0
# Philosophy: Logic determines Reality; the Model merely describes it.

# Modular Schema: TOOLS
# Sections: 28, 29, 40

# =============================================================================
# SECTION 28: OUROBOROS / TOOL SELF-GENERATION (§8.3)
# =============================================================================
# Tool registry and lifecycle for self-generating tools

# tool_registered(ToolName, RegisteredAt) - tracks registered tools
Decl tool_registered(ToolName, RegisteredAt) bound [/string, /number].

# registered_tool(ToolName, Command, ShardAffinity) - tool registration details
# ShardAffinity: /coder, /tester, /reviewer, /researcher, /generalist, /all
Decl registered_tool(ToolName, Command, ShardAffinity) bound [/string, /string, /name].

# tool_available(ToolName) - derived: tool is registered and ready
Decl tool_available(ToolName) bound [/string].

# tool_exists(ToolName) - derived: tool is in registry
Decl tool_exists(ToolName) bound [/string].

# tool_ready(ToolName) - derived: tool is compiled and ready
Decl tool_ready(ToolName) bound [/string].

# tool_hash(ToolName, Hash) - content hash for change detection
Decl tool_hash(ToolName, Hash) bound [/string, /string].

# tool_description(ToolName, Description) - human-readable description of what tool does
Decl tool_description(ToolName, Description) bound [/string, /string].

# tool_binary_path(ToolName, BinaryPath) - path to compiled binary
Decl tool_binary_path(ToolName, BinaryPath) bound [/string, /string].

# tool_capability(ToolName, Capability) - capabilities provided by a tool
Decl tool_capability(ToolName, Capability) bound [/string, /name].

# capability_available(Capability) - derived: capability exists
Decl capability_available(Capability) bound [/name].

# Tool generation lifecycle
# tool_source_ready(ToolName) - source code has been generated
Decl tool_source_ready(ToolName) bound [/string].

# tool_safety_verified(ToolName) - passed safety checks
Decl tool_safety_verified(ToolName) bound [/string].

# tool_compiled(ToolName) - successfully compiled
Decl tool_compiled(ToolName) bound [/string].

# generation_state(ToolName, State) - current generation state
# State: /pending, /in_progress, /completed, /failed
Decl generation_state(ToolName, State) bound [/string, /name].

# has_active_generation - helper for safe negation (true if any generation in progress)
Decl has_active_generation().

# is_tool_registered(ToolName) - helper for safe negation in tool registration check
Decl is_tool_registered(ToolName) bound [/string].

# missing_tool_for(Intent, Capability) - detected capability gap
Decl missing_tool_for(Intent, Capability) bound [/name, /name].

# task_failure_reason(TaskID, ReasonType, Detail)
Decl task_failure_reason(TaskID, ReasonType, Detail) bound [/string, /name, /string].

# task_failure_count(Capability, Count) - tracks repeated failures
Decl task_failure_count(Capability, Count) bound [/name, /number].

# tool_generation_blocked(Capability) - capability blocked from generation
Decl tool_generation_blocked(Capability) bound [/name].

# tool_lifecycle(ToolName, State) - tool lifecycle tracking
# State: /detected, /generating, /compiled, /deployed, /deprecated
Decl tool_lifecycle(ToolName, State) bound [/string, /name].

# -----------------------------------------------------------------------------
# 28.2 Ouroboros Derived Predicates (from policy.mg)
# -----------------------------------------------------------------------------

# explicit_tool_request(Capability) - user explicitly requested tool generation
Decl explicit_tool_request(Capability) bound [/name].

# capability_gap_detected(Capability) - repeated failures suggest missing capability
Decl capability_gap_detected(Capability) bound [/name].

# tool_generation_permitted(Capability) - tool generation passes safety checks
Decl tool_generation_permitted(Capability) bound [/name].

# dangerous_capability(Capability) - capabilities that should never be auto-generated
# e.g., /exec_arbitrary, /network_unconstrained, /system_admin, /credential_access
Decl dangerous_capability(Capability) bound [/name].

# =============================================================================
# SECTION 29: TOOL LEARNING / REFINEMENT (Autopoiesis)
# =============================================================================
# Predicates for tool quality tracking and refinement

# refinement_state(ToolName, State) - tracks refinement lifecycle
# State: /idle, /in_progress, /completed, /failed
Decl refinement_state(ToolName, State) bound [/string, /name].

# tool_known_issue(ToolName, IssueType) - known issues with a tool
# IssueType: /pagination, /incomplete, /rate_limit, /timeout
Decl tool_known_issue(ToolName, IssueType) bound [/string, /name].

# issue_occurrence_count(ToolName, IssueType, Count) - how often issue occurs
Decl issue_occurrence_count(ToolName, IssueType, Count) bound [/string, /name, /number].

# capability_similar_to(Capability, SimilarCapability) - capability relationships
Decl capability_similar_to(Capability, SimilarCapability) bound [/name, /name].

# tool_refined(ToolName, OldVersion, NewVersion) - refinement history
Decl tool_refined(ToolName, OldVersion, NewVersion) bound [/string, /string, /string].

# version_quality(ToolName, Version, QualityScore) - quality per version
Decl version_quality(ToolName, Version, QualityScore) bound [/string, /string, /number].

# tool_quality_poor(ToolName) - derived: tool has low quality
Decl tool_quality_poor(ToolName) bound [/string].

# refinement_count(ToolName, Count) - number of refinements attempted
Decl refinement_count(ToolName, Count) bound [/string, /number].

# tool_learning(ToolName, Executions, SuccessRate, AvgQuality) - learning metrics
Decl tool_learning(ToolName, Executions, SuccessRate, AvgQuality) bound [/string, /number, /number, /number].

# active_generation(ToolName) - tool is being generated
Decl active_generation(ToolName) bound [/string].

# Tool Execution Tracking (for VirtualStore integration)
# tool_executed(ToolName, Output) - tool was executed successfully
Decl tool_executed(ToolName, Output) bound [/string, /string].

# tool_exec_success(ToolName) - marks successful tool execution
Decl tool_exec_success(ToolName) bound [/string].

# tool_exec_failed(ToolName, Reason) - marks failed tool execution
Decl tool_exec_failed(ToolName, Reason) bound [/string, /string].

# tool_not_found(ToolName) - tool was requested but not in registry
Decl tool_not_found(ToolName) bound [/string].

# tool_execution_count(ToolName, Count) - total executions per tool
Decl tool_execution_count(ToolName, Count) bound [/string, /number].

# tool_last_execution(ToolName, Timestamp) - last execution time
Decl tool_last_execution(ToolName, Timestamp) bound [/string, /number].

# tool_quality_acceptable(ToolName) - derived: tool has acceptable quality
Decl tool_quality_acceptable(ToolName) bound [/string].

# tool_quality_good(ToolName) - derived: tool has good quality
Decl tool_quality_good(ToolName) bound [/string].

# tool_generation_hint(ToolName, Hint) - hints for tool generation
Decl tool_generation_hint(ToolName, Hint) bound [/string, /string].

# tool_needs_refinement(ToolName) - derived: tool quality is poor and needs refinement
Decl tool_needs_refinement(ToolName) bound [/string].

# active_refinement(ToolName) - tool is currently being refined
Decl active_refinement(ToolName) bound [/string].

# learning_pattern_detected(ToolName, IssueType) - recurring issue pattern found
Decl learning_pattern_detected(ToolName, IssueType) bound [/string, /name].

# refinement_effective(ToolName) - derived: refinement improved tool quality
Decl refinement_effective(ToolName) bound [/string].

# escalate_to_user(Subject, Reason) - escalation needed for user decision
Decl escalate_to_user(Subject, Reason) bound [/string, /string].

# =============================================================================
# SECTION 40: INTELLIGENT TOOL ROUTING (§40)
# =============================================================================
# Predicates for smart tool-to-shard routing based on capabilities, intent,
# domain matching, and usage history. Enables context-window-aware tool injection.

# -----------------------------------------------------------------------------
# 40.1 Tool Capability Categories
# -----------------------------------------------------------------------------
# Categories: /validation, /generation, /inspection, /transformation,
#             /analysis, /execution, /knowledge, /debugging, /general

# tool_domain(ToolName, Domain) - tool's primary domain
# Domains: /go, /python, /mangle, /filesystem, /git, /testing, /build, /web
Decl tool_domain(ToolName, Domain) bound [/string, /name].

# tool_usage_stats(ToolName, ExecuteCount, SuccessCount, LastUsed)
# Tracks tool execution history for learning-based prioritization
Decl tool_usage_stats(ToolName, ExecuteCount, SuccessCount, LastUsed) bound [/string, /number, /number, /number].

# tool_priority_score(ToolName, Score)
# Derived score 0.0-1.0 based on combined relevance factors
Decl tool_priority_score(ToolName, Score) bound [/string, /number].

# -----------------------------------------------------------------------------
# 40.2 Shard-Tool Affinity Mapping
# -----------------------------------------------------------------------------

# shard_capability_affinity(ShardType, CapabilityCategory, AffinityScore)
# Score 0-100 (integer) indicating how relevant a capability category is to a shard type
# NOTE: Must use integers because Mangle comparison operators don't support floats
# ShardType: /coder, /tester, /reviewer, /researcher, /generalist
# CapabilityCategory: /validation, /generation, /inspection, /transformation,
#                     /analysis, /execution, /knowledge, /debugging
Decl shard_capability_affinity(ShardType, CapabilityCategory, AffinityScore) bound [/name, /name, /number].

# current_shard_type(ShardType) - the shard type being configured
# Used for context during tool routing derivation
Decl current_shard_type(ShardType) bound [/name].

# -----------------------------------------------------------------------------
# 40.3 Intent-Capability Mapping
# -----------------------------------------------------------------------------

# intent_requires_capability(IntentVerb, CapabilityCategory, Weight)
# Maps user intent verbs to required tool capabilities with importance weights
# IntentVerb: /implement, /refactor, /fix, /test, /review, /explain, /research, /explore
# Weight: 0.0-1.0 (higher = more important)
Decl intent_requires_capability(IntentVerb, CapabilityCategory, Weight) bound [/name, /name, /number].

# current_intent(IntentID) - the active intent for routing context
Decl current_intent(IntentID) bound [/string].

# -----------------------------------------------------------------------------
# 40.4 Tool Routing Derived Predicates
# -----------------------------------------------------------------------------

# tool_base_relevance(ShardType, ToolName, Score)
# Base relevance from shard-capability affinity
Decl tool_base_relevance(ShardType, ToolName, Score) bound [/name, /string, /number].

# tool_intent_relevance(ToolName, Score)
# Boost from matching current intent's required capabilities
Decl tool_intent_relevance(ToolName, Score) bound [/string, /number].

# tool_domain_relevance(ToolName, Score)
# Boost from matching target file's language/domain
Decl tool_domain_relevance(ToolName, Score) bound [/string, /number].

# tool_success_relevance(ToolName, Score)
# Boost based on historical success rate
Decl tool_success_relevance(ToolName, Score) bound [/string, /number].

# tool_recency_relevance(ToolName, Score)
# Boost for recently used tools (likely still relevant)
Decl tool_recency_relevance(ToolName, Score) bound [/string, /number].

# tool_combined_score(ShardType, ToolName, TotalScore)
# Weighted combination of all relevance factors
Decl tool_combined_score(ShardType, ToolName, TotalScore) bound [/name, /string, /number].

# relevant_tool(ShardType, ToolName)
# Derived: tool is relevant for this shard type (above threshold)
Decl relevant_tool(ShardType, ToolName) bound [/name, /string].

# tool_priority_rank(ShardType, ToolName, Rank)
# Integer rank for ordering (higher = more relevant)
Decl tool_priority_rank(ShardType, ToolName, Rank) bound [/name, /string, /number].

# -----------------------------------------------------------------------------
# -----------------------------------------------------------------------------
# 40.4b Modular Tool Routing (Intent-based)
# -----------------------------------------------------------------------------

# modular_tool_allowed(ToolName, Intent) - derived: modular tool permitted for intent
Decl modular_tool_allowed(ToolName, Intent) bound [/string, /name].

# modular_tool_priority(ToolName, Priority) - derived: tool priority
Decl modular_tool_priority(ToolName, Priority) bound [/string, /number].

# 40.5 Tool Execution Tracking (for learning feedback)
# -----------------------------------------------------------------------------

# tool_execution(ToolName, Success, Timestamp)
# Individual execution record for aggregation
# Success: /true, /false
Decl tool_execution(ToolName, Success, Timestamp) bound [/string, /name, /number].

# tool_compilation_failed(ToolName, ErrorMessage)
# Emitted when tool generation fails to compile
Decl tool_compilation_failed(ToolName, ErrorMessage) bound [/string, /string].

# -----------------------------------------------------------------------------
# 40.6 Helper Predicates for Safe Negation
# -----------------------------------------------------------------------------

# has_current_intent() - true if any current intent exists
Decl has_current_intent().

# has_tool_domain(ToolName) - true if tool has a domain specified
Decl has_tool_domain(ToolName) bound [/string].

# has_tool_usage(ToolName) - true if tool has usage stats
Decl has_tool_usage(ToolName) bound [/string].

# -----------------------------------------------------------------------------
# 36.5 Virtual Predicate for Trace Queries
# -----------------------------------------------------------------------------

# query_traces(ShardType, Limit, TraceID, Success, DurationMs)
# Queries reasoning_traces table via VirtualStore FFI
# External predicate: ShardType required as input
Decl query_traces(ShardType, Limit, TraceID, Success, DurationMs) descr [external(), mode('+', '-', '-', '-', '-')] bound [/name, /number, /string, /name, /number].

# query_trace_stats(ShardType, SuccessCount, FailCount, AvgDuration)
# Retrieves aggregate stats for a shard type
# External predicate: ShardType required as input
Decl query_trace_stats(ShardType, SuccessCount, FailCount, AvgDuration) descr [external(), mode('+', '-', '-', '-')] bound [/name, /number, /number, /number].

# -----------------------------------------------------------------------------
# 41.12 Reviewer Feedback Loop Predicates (Self-Correction)
# -----------------------------------------------------------------------------

# review_finding_with_id(ReviewID, File, Line, Severity, Category, Message)
# A finding from a specific review session (6-arg variant with ReviewID)
# NOTE: The 5-arg review_finding/5 is declared in Section 29 and emitted by Go.
# This 6-arg variant is derived via bridge rule when active_review(ReviewID) is set.
Decl review_finding_with_id(ReviewID, File, Line, Severity, Category, Message) bound [/string, /string, /number, /name, /name, /string].

# user_rejected_finding(ReviewID, File, Line, Reason, Timestamp)
# User explicitly rejected a finding as incorrect
Decl user_rejected_finding(ReviewID, File, Line, Reason, Timestamp) bound [/string, /string, /number, /string, /number].

# user_accepted_finding(ReviewID, File, Line, Timestamp)
# User explicitly accepted a finding (applied the suggestion)
Decl user_accepted_finding(ReviewID, File, Line, Timestamp) bound [/string, /string, /number, /number].

# review_accuracy(ReviewID, TotalFindings, Accepted, Rejected, Score)
# Computed accuracy score for a review session
Decl review_accuracy(ReviewID, TotalFindings, Accepted, Rejected, Score) bound [/string, /number, /number, /number, /number].

# review_rejection_rate_high(ReviewID) - True when rejection rate > 50%
# Computed by Go: (Rejected * 2) > Total
Decl review_rejection_rate_high(ReviewID) bound [/string].

# false_positive_pattern(Pattern, Category, Occurrences, Confidence)
# Learned patterns that cause false positives
Decl false_positive_pattern(Pattern, Category, Occurrences, Confidence) bound [/string, /name, /number, /number].

# review_suspect(ReviewID, Reason)
# Derived: Review flagged as potentially inaccurate
Decl review_suspect(ReviewID, Reason) bound [/string, /string].

# reviewer_needs_validation(ReviewID)
# Derived: This review should be spot-checked by main agent
Decl reviewer_needs_validation(ReviewID) bound [/string].

# =============================================================================
# MULTI-SHARD REVIEW ORCHESTRATION (§20.1)
# =============================================================================
# Schemas for tracking orchestrated multi-shard reviews where multiple
# specialist agents review code in parallel.

# multi_shard_review(ReviewID, Target, Participants, IsComplete, TotalFindings, Timestamp)
# Main record for a multi-shard orchestrated review
Decl multi_shard_review(ReviewID, Target, Participants, IsComplete, TotalFindings, Timestamp) bound [/string, /string, /number, /name, /number, /number].

# multi_review_participant(ReviewID, ShardName, FileCount, FindingCount)
# Tracks which specialists participated in a review
Decl multi_review_participant(ReviewID, ShardName, FileCount, FindingCount) bound [/string, /string, /number, /number].

# multi_review_finding(ReviewID, ShardName, FilePath, Line, Severity, Message)
# Individual findings from a multi-shard review, attributed to source shard
Decl multi_review_finding(ReviewID, ShardName, FilePath, Line, Severity, Message) bound [/string, /string, /string, /number, /name, /string].

# cross_shard_insight(ReviewID, InsightType, Description)
# Holistic insights derived from cross-shard analysis
# InsightType: /hot_spot, /pattern, /critical_attention, /cross_domain
Decl cross_shard_insight(ReviewID, InsightType, Description) bound [/string, /name, /string].

# review_insight(Index, Insight)
# Individual review insights stored for learning/retrieval
Decl review_insight(Index, Insight) bound [/number, /string].

# specialist_match(ReviewID, AgentName, Score, Reason)
# Records which specialists were matched for a review and why
Decl specialist_match(ReviewID, AgentName, Score, Reason) bound [/string, /string, /number, /string].

# specialist_match(Specialist, Task, Confidence)
# BUG FIX: 3-arg overload for task matching rules in shards.mg
Decl specialist_match(Specialist, Task, Confidence) bound [/string, /string, /number].

# symbol_verified_exists(Symbol, File, VerifiedAt)
# Symbol was verified to exist (counters false "undefined" claims)
Decl symbol_verified_exists(Symbol, File, VerifiedAt) bound [/string, /string, /number].

# =============================================================================
# SECTION 41.13: LANGUAGE-AGNOSTIC SECURITY FLOW ANALYSIS
# =============================================================================
# These predicates enable security detection across any programming language
# by abstracting sink/source relationships rather than language-specific patterns.

# detected_security_flow(File, Line, SinkType, SourceType, Confidence)
# Flow analysis result: untrusted data flows to security-sensitive sink
# SinkType: /sql_sink, /command_sink, /dom_sink, /hardcoded_secret, /weak_crypto
# SourceType: /user_input, /file_read, /network, /env_var
# Confidence: 0-100 integer
Decl detected_security_flow(File, Line, SinkType, SourceType, Confidence) bound [/string, /number, /name, /name, /number].

# security_sink_type(Language, SinkType, Pattern, Description)
# Configuration: Maps language-specific patterns to abstract sink types
# Emitted by language-specific analyzers (Go, Python, JS, Rust, etc.)
Decl security_sink_type(Language, SinkType, Pattern, Description) bound [/name, /name, /string, /string].

# flow_security_rule(RuleID, Severity, SinkType, Message)
# Abstract security rules keyed by sink type (language-agnostic)
Decl flow_security_rule(RuleID, Severity, SinkType, Message) bound [/string, /name, /name, /string].
