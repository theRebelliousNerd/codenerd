# =============================================================================
# LEARNED LOGIC (Autopoiesis Layer)
# =============================================================================
# This file contains agent-learned rules and patterns.
#
# SECURITY MODEL: Stratified Trust (§Bug #15 Fix)
# - This namespace can ONLY propose candidate_action/1 predicates
# - The Constitution (policy.gl) MUST validate all candidates via safety_check/1
# - Learned rules CANNOT override system-level safety predicates
# - All predicates here use the "candidate_" prefix to prevent namespace collision
#
# The Bridge Rule (in policy.gl):
#   final_action(X) :- candidate_action(X), permitted(X).
#
# This ensures learned logic can suggest, but never execute without approval.
# =============================================================================

# =============================================================================
# LEARNED RULES - CANDIDATE ACTIONS (Stratified Trust Layer)
# =============================================================================
# These rules propose actions to the Constitution for validation.
# They cannot execute directly - must pass safety_check/1 in policy.gl.

# Example 1: Suggest refactoring for long functions
candidate_action(/suggest_refactor) :-
    code_element(Ref, /function, File, StartLine, EndLine),
    file_line_count(File, TotalLines),
    TotalLines > 150,
    !test_coverage(File).

# Example 2: Suggest running tests after code changes
candidate_action(/run_tests) :-
    modified(File),
    test_coverage(File),
    !test_state(/passing).

# Example 3: Suggest code review for high-risk changes
candidate_action(/delegate_reviewer) :-
    modified(File),
    breaking_change_risk(Ref, /high, _),
    code_element(Ref, _, File, _, _).

# Example 4: Suggest documentation for public API changes
candidate_action(/document) :-
    element_modified(Ref, _, _),
    element_visibility(Ref, /public),
    api_handler_function(Ref, _, _).

# Example 5: Suggest integration tests for API clients
candidate_action(/delegate_tester) :-
    api_client_function(Ref, Endpoint, _),
    element_modified(Ref, _, _),
    !requires_integration_test(Ref).

# =============================================================================
# LEARNED PREFERENCES (Hydrated from knowledge.db)
# =============================================================================
# These facts are asserted by VirtualStore.HydrateLearnings() during OODA Observe.
# They represent user preferences learned over time through Autopoiesis.

# Example: User prefers verbose error messages
# learned_preference(/error_verbosity, /verbose).

# Example: User prefers table-driven tests in Go
# learned_preference(/test_style, /table_driven).

# Example: User prefers explicit error handling over panics
# learned_preference(/error_handling, /explicit).

# Example: User prefers conventional commits
# learned_preference(/commit_style, /conventional).

# Example: User prefers detailed explanations
# learned_preference(/explanation_level, /detailed).

# =============================================================================
# LEARNED CONSTRAINTS (Hydrated from knowledge.db)
# =============================================================================
# These facts enforce learned constraints that become safety checks.
# They are derived into constraint_violation/2 by policy.gl Section 17B.

# Example: Never commit without tests passing
# learned_constraint(/commit_requires_tests, /true).

# Example: Never delete files without git history check
# learned_constraint(/delete_requires_git_check, /true).

# Example: Always run linter before commit
# learned_constraint(/commit_requires_lint, /true).

# Example: Require approval for breaking API changes
# learned_constraint(/breaking_requires_approval, /true).

# Example: No hardcoded credentials
# learned_constraint(/no_hardcoded_secrets, /true).

# =============================================================================
# LEARNED FACTS (Hydrated from knowledge.db)
# =============================================================================
# Domain-specific facts learned about the project/user.
# Used by policy.gl Section 17B for context-aware decisions.

# Example: Project uses Go modules
# learned_fact(/build_system, /go_modules).

# Example: Project follows standard Go layout
# learned_fact(/architectural_pattern, /standard_go_layout).

# Example: User prefers reviewer shard for security
# learned_fact(/security_tool, /reviewer).

# Example: User's typical working hours (for scheduling)
# learned_fact(/working_hours, "09:00-17:00").

# Example: CI/CD pipeline requires specific checks
# learned_fact(/required_checks, "build,test,lint").

# =============================================================================
# PATTERN LEARNING - REFACTORING OPPORTUNITIES
# =============================================================================
# These rules learn from repeated code patterns and suggest improvements.

# Suggest extracting duplicated code blocks
candidate_action(/suggest_extract_function) :-
    code_element(Ref1, /function, File, _, _),
    code_element(Ref2, /function, File, _, _),
    Ref1 != Ref2,
    quality_violation(_, /duplicate_code).

# Suggest splitting large files
candidate_action(/suggest_split_file) :-
    file_line_count(File, LineCount),
    LineCount > 1000,
    code_element(_, _, File, _, _).

# Suggest adding error handling for API clients
candidate_action(/add_error_handling) :-
    api_client_function(Ref, _, _),
    quality_violation(_, /missing_errors),
    code_element(Ref, _, _, _, _).

# =============================================================================
# AUTOPOIESIS - SELF-LEARNING FROM QUALITY VIOLATIONS
# =============================================================================
# These rules are synthesized from repeated quality violations.
# See policy.gl Section 23 (Verification Loop) for quality_violation sources.

# Avoid mock code in production (learned from quality violations)
candidate_action(/corrective_research) :-
    quality_violation(TaskID, /mock_code),
    current_task(TaskID).

# Require documentation lookup for hallucinated APIs
candidate_action(/corrective_docs) :-
    quality_violation(TaskID, /hallucinated_api),
    current_task(TaskID).

# Decompose tasks when implementations are incomplete
candidate_action(/corrective_decompose) :-
    quality_violation(TaskID, /incomplete),
    verification_attempt(TaskID, AttemptNum, /failure),
    AttemptNum >= 2.

# =============================================================================
# TOOL GENERATION PATTERNS (Ouroboros)
# =============================================================================
# Learned patterns for when to generate new tools.

# Generate tool when capability gap detected repeatedly
candidate_action(/ouroboros_generate) :-
    missing_tool_for(_, Cap),
    task_failure_count(Cap, Count),
    Count >= 2,
    !dangerous_capability(Cap).

# Refine tool when quality is poor
candidate_action(/refine_tool) :-
    tool_quality_poor(ToolName),
    !active_refinement(ToolName).

# =============================================================================
# SHARD SELECTION PATTERNS (Cross-Shard Learning)
# =============================================================================
# Learned preferences for which shards to use for specific tasks.
# See policy.gl Section 24 (Reasoning Traces) for learning signals.

# Prefer specialist over ephemeral for complex tasks
candidate_action(/delegate_specialist) :-
    specialist_outperforms(SpecialistName, TaskType),
    user_intent(_, _, _, Task, _),
    shard_can_handle(SpecialistName, TaskType).

# Switch shard when current one struggles
candidate_action(/switch_shard) :-
    shard_struggling(CurrentShard),
    shard_switch_suggestion(TaskType, CurrentShard, AlternateShard),
    current_task(TaskID).

# =============================================================================
# CAMPAIGN LEARNING (Multi-Phase Goal Patterns)
# =============================================================================
# Patterns learned from successful campaign executions.

# Replicate successful phase patterns
candidate_action(/create_phase) :-
    phase_success_pattern(PhaseType),
    current_campaign(CampaignID),
    promote_to_long_term(/phase_success, PhaseType).

# Avoid failed task patterns
candidate_action(/skip_task_pattern) :-
    campaign_learning(CampaignID, /failure_pattern, TaskType, ErrorMsg, _),
    current_campaign(CampaignID),
    next_campaign_task(TaskID),
    campaign_task(TaskID, _, _, _, TaskType).

# =============================================================================
# COMMIT SAFETY PATTERNS (Git-Aware Learning)
# =============================================================================
# Learned rules for safe commit practices.

# Block commit for recent changes by others (Chesterton's Fence)
candidate_action(/interrogative_mode) :-
    user_intent(_, /mutation, /commit, _, _),
    chesterton_fence_warning(File, "recent_change_by_other"),
    modified(File).

# Require review for high-churn files
candidate_action(/delegate_reviewer) :-
    user_intent(_, /mutation, _, File, _),
    churn_rate(File, Freq),
    Freq > 5.0.

# =============================================================================
# NOTES FOR AUTOPOIESIS INTEGRATION
# =============================================================================
# How new rules are added to this file:
#
# 1. System detects repeated patterns via promote_to_long_term/2 in policy.gl
# 2. LLM synthesizes new Mangle rule from pattern
# 3. Kernel.HotLoadRule() validates rule in sandbox (Bug #8 Fix)
# 4. If safe, rule is appended to learned.gl via file write
# 5. Rule is loaded into kernel and takes effect immediately
#
# All learned rules must:
# - Use candidate_action/1 prefix (stratified trust)
# - Only reference declared predicates from schemas.gl (Bug #18 Fix)
# - Be safe for negation (all negated vars bound elsewhere)
# - Pass stratification analysis (no paradoxes)
#
# See internal/core/kernel.go:557-593 for HotLoadRule implementation.
