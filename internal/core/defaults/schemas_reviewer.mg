# Cortex 1.5.0 Schemas (EDB Declarations)
# Version: 1.5.0
# Philosophy: Logic determines Reality; the Model merely describes it.

# Modular Schema: REVIEWER
# Sections: 41, 47

# =============================================================================
# SECTION 41: MISSING DECLARATIONS (Cross-Module Support)
# =============================================================================
# Predicates used by policy.mg rules or Go code that were previously undeclared.

# -----------------------------------------------------------------------------
# 41.1 Policy Helper Predicates
# -----------------------------------------------------------------------------

# has_projection_violation(ActionID) - helper for safe negation in shadow mode
Decl has_projection_violation(ActionID).

# is_mutation_approved(MutationID) - helper for safe negation in diff approval
Decl is_mutation_approved(MutationID).

# has_pending_checkpoint(PhaseID) - helper for checkpoint verification
Decl has_pending_checkpoint(PhaseID).

# action_ready_for_routing(ActionID) - derived: action ready for tactile router
Decl action_ready_for_routing(ActionID).

# -----------------------------------------------------------------------------
# 41.2 Shard Configuration Predicates
# -----------------------------------------------------------------------------

# shard_type(Type, Lifecycle, Characteristic) - shard taxonomy configuration
# Type: /system, /ephemeral, /persistent, /user
# Lifecycle: /permanent, /spawn_die, /long_running, /explicit
# Characteristic: /high_reliability, /speed_optimized, /adaptive, /user_defined
Decl shard_type(Type, Lifecycle, Characteristic).

# shard_model_config(ShardType, ModelType) - model capability mapping for shards
# ShardType: /system, /ephemeral, /persistent, /user
# ModelType: /high_reasoning, /high_speed, /balanced
Decl shard_model_config(ShardType, ModelType).

# -----------------------------------------------------------------------------
# 41.3 Perception / Taxonomy Predicates
# -----------------------------------------------------------------------------

# NOTE: context_token(Token) is declared in inference.mg

# user_input_string(Input) - raw user input string for NL processing
Decl user_input_string(Input).

# is_relevant(Path) - derived: path is relevant to current campaign/intent
Decl is_relevant(Path).

# -----------------------------------------------------------------------------
# 41.4 Reviewer Shard Predicates
# -----------------------------------------------------------------------------

# active_finding(File, Line, Severity, Category, RuleID, Message)
# Filtered findings after Mangle rules suppress noisy or irrelevant entries
Decl active_finding(File, Line, Severity, Category, RuleID, Message).

# raw_finding(File, Line, Severity, Category, RuleID, Message)
# Unfiltered findings from static analysis before Mangle processing
Decl raw_finding(File, Line, Severity, Category, RuleID, Message).

# -----------------------------------------------------------------------------
# 41.5 Tool Generator / Ouroboros Predicates
# -----------------------------------------------------------------------------

# tool_generated(ToolName, Timestamp) - successfully generated tool
Decl tool_generated(ToolName, Timestamp).

# tool_trace(ToolName, TraceID) - reasoning trace for tool generation
Decl tool_trace(ToolName, TraceID).

# tool_generation_failed(ToolName, ErrorMessage) - tool generation failure record
Decl tool_generation_failed(ToolName, ErrorMessage).

# tool_issue_pattern(ToolName, IssueType, Occurrences, Confidence)
# Detected patterns from tool learning (pagination, incomplete, rate_limit, timeout)
Decl tool_issue_pattern(ToolName, IssueType, Occurrences, Confidence).

# -----------------------------------------------------------------------------
# 41.6 Campaign / Requirement Predicates
# -----------------------------------------------------------------------------

# requirement_task_link(RequirementID, TaskID, Strength)
# Links requirements to tasks that fulfill them with strength score
Decl requirement_task_link(RequirementID, TaskID, Strength).

# -----------------------------------------------------------------------------
# 41.7 Git Context Predicates (Chesterton's Fence)
# -----------------------------------------------------------------------------

# git_branch(Branch) - current git branch name
Decl git_branch(Branch).

# recent_commit(Hash, Message, Author, Timestamp)
# Recent commit history for Chesterton's Fence analysis
Decl recent_commit(Hash, Message, Author, Timestamp).

# -----------------------------------------------------------------------------
# 41.8 Test State Predicates
# -----------------------------------------------------------------------------

# failing_test(TestName, ErrorMessage) - details of failing tests
Decl failing_test(TestName, ErrorMessage).

# -----------------------------------------------------------------------------
# 41.9 Constitutional Safety Predicates
# -----------------------------------------------------------------------------

# blocked_action(Action) - action blocked by constitutional rules
Decl blocked_action(Action).

# safety_warning(Warning) - active safety concern/warning message
Decl safety_warning(Warning).

# -----------------------------------------------------------------------------
# 41.10 Execution & Context Predicates
# -----------------------------------------------------------------------------

# execution_result(ActionID, Type, Target, Success, Output, Timestamp) - from virtual store
Decl execution_result(ActionID, Type, Target, Success, Output, Timestamp).

# context_to_inject(Fact) - derived: facts selected for LLM context injection
Decl context_to_inject(Fact).

# final_system_prompt(Prompt) - derived: assembled system prompt for LLM
Decl final_system_prompt(Prompt).

# -----------------------------------------------------------------------------
# 41.11 Recursive Helper Predicates
# -----------------------------------------------------------------------------

# parent(Child, Parent) - direct parent-child relationship (recursive base case)
Decl parent(Child, Parent).

# ancestor(Descendant, Ancestor) - transitive ancestor relationship (recursive closure)
Decl ancestor(Descendant, Ancestor).

# =============================================================================
# SECTION 47: STATIC ANALYSIS - DATA FLOW PREDICATES (ReviewerShard Beyond-SOTA)
# =============================================================================
# Advanced static analysis predicates for differential nil-pointer detection,
# error handling verification, and data flow tracking. These enable the
# ReviewerShard to perform precise diff-aware analysis using guard-based
# reasoning.

# -----------------------------------------------------------------------------
# 47.1 Data Flow Predicates - Variable Tracking
# -----------------------------------------------------------------------------

# assigns(Var, ValueType, File, Line) - Variable assignment tracking
# Var: Variable name (string)
# ValueType: Type of value assigned (e.g., /pointer, /interface, /value, /error)
# File: Source file path
# Line: Line number of assignment
Decl assigns(Var, ValueType, File, Line).

# uses(File, Func, Var, Line) - Variable read sites
# File: Source file path
# Func: Function containing the use
# Var: Variable being read
# Line: Line number of use
Decl uses(File, Func, Var, Line).

# call_arg(CallSite, ArgPos, VarRef, File, Line) - Function call argument tracking
# CallSite: Identifier for the call (e.g., "foo.Bar")
# ArgPos: Argument position (0-indexed integer)
# VarRef: Variable reference passed as argument
# File: Source file path
# Line: Line number
Decl call_arg(CallSite, ArgPos, VarRef, File, Line).

# -----------------------------------------------------------------------------
# 47.2 Guard Predicates - Two Types for Go's Idiomatic Patterns
# -----------------------------------------------------------------------------
# Go uses two distinct guard patterns:
# 1. Block guards: if x != nil { /* x is safe here */ }
# 2. Return guards: if x == nil { return } /* x is safe after */

# guards_block(Var, CheckType, File, ScopeStart, ScopeEnd) - Block-scoped guards
# Var: Variable being guarded
# CheckType: Type of check (/nil_check, /len_check, /type_assert, /ok_check)
# File: Source file path
# ScopeStart: Starting line of guarded scope
# ScopeEnd: Ending line of guarded scope
Decl guards_block(Var, CheckType, File, ScopeStart, ScopeEnd).

# guards_return(Var, CheckType, File, Line) - Return-based guards (dominator guards)
# Var: Variable being guarded
# CheckType: Type of check (/nil_check, /zero_check, /err_check)
# File: Source file path
# Line: Line of the guard check (all lines after are guarded)
Decl guards_return(Var, CheckType, File, Line).

# -----------------------------------------------------------------------------
# 47.3 Error Handling Predicates
# -----------------------------------------------------------------------------

# error_checked_block(Var, File, ScopeStart, ScopeEnd) - Block-scoped error handling
# Var: Error variable being checked
# File: Source file path
# ScopeStart: Start of error handling scope
# ScopeEnd: End of error handling scope
Decl error_checked_block(Var, File, ScopeStart, ScopeEnd).

# error_checked_return(Var, File, Line) - Return-based error handling
# Var: Error variable being checked
# File: Source file path
# Line: Line of error check (typically: if err != nil { return err })
Decl error_checked_return(Var, File, Line).

# -----------------------------------------------------------------------------
# 47.4 Function Metadata
# -----------------------------------------------------------------------------

# nil_returns(File, Func, Line) - Functions that can return nil
# File: Source file path
# Func: Function name
# Line: Line number where nil is returned
Decl nil_returns(File, Func, Line).

# modified_function(Func, File) - Functions changed in the current diff
# Func: Function name
# File: Source file path
Decl modified_function(Func, File).

# modified_interface(Interface, File) - Interfaces changed in the current diff
# Interface: Interface name
# File: Source file path
Decl modified_interface(Interface, File).

# -----------------------------------------------------------------------------
# 47.5 Scope Tracking (for dominator guard analysis)
# -----------------------------------------------------------------------------

# same_scope(Var, File, Line1, Line2) - Lines in same function scope
# Var: Variable name (for context)
# File: Source file path
# Line1: First line number
# Line2: Second line number
# Used to determine if a guard at Line1 protects a use at Line2
Decl same_scope(Var, File, Line1, Line2).

# -----------------------------------------------------------------------------
# 47.6 Suppression Predicates (for Autopoiesis - False Positive Learning)
# -----------------------------------------------------------------------------

# suppression(DiagnosticID, Reason) - User-suppressed diagnostic warnings
# DiagnosticID: ID of the diagnostic being suppressed
# Reason: User-provided reason for suppression
Decl suppression(DiagnosticID, Reason).

# suppressed_rule(RuleType, File, Line, Reason) - Manually suppressed findings
# RuleType: Type of rule suppressed (e.g., /nil_deref, /unchecked_error)
# File: Source file path
# Line: Line number of suppression
# Reason: User-provided reason for suppression
Decl suppressed_rule(RuleType, File, Line, Reason).

# suppression_confidence(RuleType, File, Line, Score) - Learned suppression confidence
# RuleType: Type of rule
# File: Source file path
# Line: Line number
# Score: Confidence score (0-100) that this is a false positive
Decl suppression_confidence(RuleType, File, Line, Score).

# -----------------------------------------------------------------------------
# 47.7 Priority and Risk Predicates
# -----------------------------------------------------------------------------

# type_priority(Type, Priority) - Severity priority by type
# Type: Finding type (e.g., /nil_deref, /unchecked_error, /race_condition)
# Priority: Priority level (1=critical, 2=high, 3=medium, 4=low)
Decl type_priority(Type, Priority).

# bug_history(File, Count) - Historical bug count per file
# File: Source file path
# Count: Number of bugs historically found in this file
# Used for risk-based prioritization
Decl bug_history(File, Count).

# -----------------------------------------------------------------------------
# 47.8 Derived Predicates for Data Flow Analysis
# -----------------------------------------------------------------------------

# guarded_use(Var, File, Line) - Derived: variable use is protected by a guard
Decl guarded_use(Var, File, Line).

# unguarded_use(Var, File, Line) - Derived: variable use lacks guard protection
Decl unguarded_use(Var, File, Line).

# error_ignored(Var, File, Line) - Derived: error variable is not checked
Decl error_ignored(Var, File, Line).

# nil_deref_risk(Var, File, Line, RiskLevel) - Derived: potential nil dereference
# RiskLevel: /high (no guard), /medium (conditional guard), /low (likely safe)
Decl nil_deref_risk(Var, File, Line, RiskLevel).

# in_modified_code(File, Line) - Derived: line is within modified diff hunks
Decl in_modified_code(File, Line).

# diff_introduces_risk(File, Line, RiskType) - Derived: diff introduces new risk
# RiskType: /nil_deref, /unchecked_error, /race_condition
Decl diff_introduces_risk(File, Line, RiskType).

# has_guard(Var, File, Line) - Helper: variable has any guard at this point
Decl has_guard(Var, File, Line).

# is_suppressed(RuleType, File, Line) - Helper: finding is suppressed
Decl is_suppressed(RuleType, File, Line).

# -----------------------------------------------------------------------------
# 47.9 Data Flow Safety Derived Predicates (IDB - from policy.mg Section 47)
# -----------------------------------------------------------------------------

# is_guarded(Var, File, Line) - Derived: variable is protected at this point
# Computed from guards_block and guards_return
Decl is_guarded(Var, File, Line).

# unsafe_deref(File, Var, Line) - Derived: nullable dereference without guard
Decl unsafe_deref(File, Var, Line).

# is_error_checked(Var, File, Line) - Derived: error variable is checked
Decl is_error_checked(Var, File, Line).

# unchecked_error(File, Func, Line) - Derived: error assigned but not checked
Decl unchecked_error(File, Func, Line).

# -----------------------------------------------------------------------------
# 47.10 Impact Analysis Derived Predicates (IDB - from policy.mg Section 48)
# -----------------------------------------------------------------------------

# impact_caller(TargetFunc, CallerFunc) - Direct callers of modified function
Decl impact_caller(TargetFunc, CallerFunc).

# impact_implementer(ImplFile, Struct) - Implementers of modified interface
Decl impact_implementer(ImplFile, Struct).

# impact_graph(Target, Caller, Depth) - Transitive impact with depth (max 3)
Decl impact_graph(Target, Caller, Depth).

# relevant_context_file(File) - Files to fetch for review context
Decl relevant_context_file(File).

# context_priority_file(File, Func, Priority) - Priority-ordered context files
Decl context_priority_file(File, Func, Priority).

# -----------------------------------------------------------------------------
# 47.11 Hypothesis Management (IDB - from policy.mg Sections 49-50)
# -----------------------------------------------------------------------------

# active_hypothesis(Type, File, Line, Var) - Post-suppression hypotheses
Decl active_hypothesis(Type, File, Line, Var).

# priority_boost(File, Boost) - Additional priority for risky files
Decl priority_boost(File, Boost).

# prioritized_hypothesis(Type, File, Line, Var, Priority) - Final prioritized findings
Decl prioritized_hypothesis(Type, File, Line, Var, Priority).

# -----------------------------------------------------------------------------
# 47.12 Helper Predicates for Safe Negation (IDB)
# -----------------------------------------------------------------------------
# These helpers enable safe negation by ensuring variables are bound before
# negation is applied. Required by Mangle's safety constraints.

# has_guard_at(Var, File, Line) - Helper for guarded variable check
Decl has_guard_at(Var, File, Line).

# has_error_check_at(Var, File, Line) - Helper for error check presence
Decl has_error_check_at(Var, File, Line).

# has_suppression_unsafe_deref(File, Line) - Helper for suppression check
Decl has_suppression_unsafe_deref(File, Line).

# has_suppression_unchecked_error(File, Line) - Helper for suppression check
Decl has_suppression_unchecked_error(File, Line).

# has_test_coverage(File) - Helper: file has test coverage
Decl has_test_coverage(File).

# has_bug_history(File) - Helper: file has bug history (count > 0)
Decl has_bug_history(File).

# has_priority_boost(File) - Helper: file has any priority boost
Decl has_priority_boost(File).

# -----------------------------------------------------------------------------
# 47.13 Multi-Language Data Flow Predicates
# -----------------------------------------------------------------------------
# These predicates support data flow analysis across multiple languages
# (Go, Python, TypeScript, JavaScript, Rust) using Tree-sitter parsing.

# function_scope(File, Func, Start, End) - Function scope boundaries
# File: Source file path
# Func: Function name
# Start: Starting line number
# End: Ending line number
# Used to determine scope for guard domination
Decl function_scope(File, Func, Start, End).

# guard_dominates(File, Func, GuardLine, EndLine) - Guard domination for early returns
# File: Source file path
# Func: Function containing the guard
# GuardLine: Line of the guard check (e.g., if x == nil { return })
# EndLine: Last line of the function (guard protects all subsequent lines)
# Early return guards dominate all code after them in the same scope
Decl guard_dominates(File, Func, GuardLine, EndLine).

# safe_access(Var, AccessType, File, Line) - Language-specific safe access patterns
# Var: Variable being accessed
# AccessType: Type of safe access pattern:
#   /optional_chain - JavaScript/TypeScript x?.foo
#   /if_let - Rust if let Some(x) = ...
#   /match_exhaustive - Rust match expression (exhaustive)
#   /walrus - Python x := (assignment expression)
# File: Source file path
# Line: Line number
# These accesses are inherently safe by the language's semantics
Decl safe_access(Var, AccessType, File, Line).

