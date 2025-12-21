# Cortex 1.5.0 Schemas (EDB Declarations)
# Version: 1.5.0
# Philosophy: Logic determines Reality; the Model merely describes it.

# Modular Schema: SHARDS
# Sections: 6, 30, 31, 32, 33

# =============================================================================
# SECTION 6: SHARD DELEGATION (§7.0)
# =============================================================================

# delegate_task(ShardType, TaskDescription, Status)
# ShardType: /researcher, /coder, /reviewer, /tester, /generalist, /specialist
# Status: /pending, /in_progress, /completed, /failed
Decl delegate_task(ShardType, TaskDescription, Status).

# shard_profile(AgentName, Type, KnowledgePath)
Decl shard_profile(AgentName, Type, KnowledgePath).

# =============================================================================
# SECTION 30: CODER SHARD HELPERS
# =============================================================================

# file_content(FilePath, Content) - cached file content
Decl file_content(FilePath, Content).

# coder_state(State) - current coder state
# State: /idle, /context_ready, /code_generated, /edit_applied, /build_passed, /build_failed
Decl coder_state(State).

# pending_edit(FilePath, Content) - pending edits
Decl pending_edit(FilePath, Content).

# coder_block_write(FilePath, Reason) - derived: write is blocked
Decl coder_block_write(FilePath, Reason).

# coder_safe_to_write(FilePath) - derived: safe to write
Decl coder_safe_to_write(FilePath).

# is_binary_file(FilePath) - file is binary (cannot edit)
Decl is_binary_file(FilePath).

# is_core_file(FilePath) - file is in core/critical path (higher risk)
Decl is_core_file(FilePath).

# dependent_count(Target, Count) - number of files depending on Target
# Computed by Go from dependency_link graph
Decl dependent_count(Target, Count).

# is_interface_file(FilePath) - file contains interface/type definitions
Decl is_interface_file(FilePath).

# instruction_contains(Instruction, Substring) - checks if user instruction contains pattern
# Implemented by Go string search
Decl instruction_contains(Instruction, Substring).

# instruction_contains_write(FilePath) - instruction mentions writing to file
Decl instruction_contains_write(FilePath).

# file_package(FilePath, PackageName) - maps file to its Go package
Decl file_package(FilePath, PackageName).

# same_package(File1, File2) - files are in the same Go package
# Computed by Go: checks if file_package(File1, P) and file_package(File2, P)
Decl same_package(File1, File2).

# tdd_state(State) - current TDD phase state
# State: /red (tests failing), /green (tests passing), /refactor
Decl tdd_state(State).

# tdd_retry_count(Count) - number of retries in current TDD loop
Decl tdd_retry_count(Count).

# type_definition_file(FilePath) - file contains type/struct definitions
Decl type_definition_file(FilePath).

# edit_analysis(FilePath, Property) - analysis results for pending edits
# Property: /handles_errors, /has_context, /has_waitgroup, /has_context_cancel,
#           /spawns_goroutine, /public_function, /does_io
Decl edit_analysis(FilePath, Property).

# interface_definition(FilePath, Name, MethodCount) - interface definition info
Decl interface_definition(FilePath, Name, MethodCount).

# edit_operation(FilePath, Operation) - operations in pending edit
Decl edit_operation(FilePath, Operation).

# function_metrics(FilePath, FuncName, Lines, Complexity) - function metrics
Decl function_metrics(FilePath, FuncName, Lines, Complexity).

# function_params(FilePath, FuncName, ParamCount) - parameter count
Decl function_params(FilePath, FuncName, ParamCount).

# function_nesting(FilePath, FuncName, Depth) - nesting depth
Decl function_nesting(FilePath, FuncName, Depth).

# diagnostic_count(FilePath, Severity, Count) - diagnostics per file
Decl diagnostic_count(FilePath, Severity, Count).

# previous_coder_state(State) - previous state for progress tracking
Decl previous_coder_state(State).

# state_unchanged_count(Count) - how many times state hasn't changed
Decl state_unchanged_count(Count).

# path_contains(Path, Pattern) - checks if file path contains substring
Decl path_contains(Path, Pattern).

# is_test_file(FilePath) - file is a test file (e.g., *_test.go)
# NOTE: Also declared in tester.mg for module separation
Decl is_test_file(FilePath).

# detected_language(FilePath, Language) - detected programming language of file
Decl detected_language(FilePath, Language).

# testable_language(Language) - language supports automated testing
Decl testable_language(Language).

# is_public_api(FilePath) - file contains public API definitions
Decl is_public_api(FilePath).

# doc_exists_for(FilePath) - documentation exists for this file
Decl doc_exists_for(FilePath).

# test_file_for(TestFile, SourceFile) - TestFile contains tests for SourceFile
Decl test_file_for(TestFile, SourceFile).

# build_state(State) - current build state
# State: /passing, /failing, /unknown
Decl build_state(State).

# build_result(Success, Output) - result of build action (added for Bug 10)
Decl build_result(Success, Output).

# =============================================================================
# SECTION 31: REVIEWER SHARD HELPERS
# =============================================================================

# file_line_count(FilePath, LineCount) - line count per file
Decl file_line_count(FilePath, LineCount).

# finding_count(Severity, Count) - count of findings by severity
Decl finding_count(Severity, Count).

# style_rule(RuleID, RuleName, Threshold) - style rule definitions
Decl style_rule(RuleID, RuleName, Threshold).

# permission_denied(Action, Reason) - derived: why permission was denied (Bug 12)
Decl permission_denied(Action, Reason).

# checks_passed() - derived: positive confirmation of checks (Bug 10)
Decl checks_passed().

# safe_to_commit() - derived: all checks pass, safe to commit
Decl safe_to_commit().

# file_truncated(Path, MaxSize) - file content truncated (Bug 6)
Decl file_truncated(Path, MaxSize).

# =============================================================================
# SECTION 32: SAFE NEGATION HELPERS
# =============================================================================
# These helpers support safe negation patterns in Mangle rules

# has_block_commit() - helper: true if any block_commit exists
Decl has_block_commit().

# has_active_refinement() - helper: true if any refinement in progress
Decl has_active_refinement().

# has_eligible_phase() - helper: true if any phase is eligible
Decl has_eligible_phase().

# has_next_campaign_task() - helper: true if there's a next task
Decl has_next_campaign_task().

# has_in_progress_phase() - helper: true if any phase in progress
Decl has_in_progress_phase().

# has_incomplete_phase(CampaignID) - helper: campaign has incomplete phases
Decl has_incomplete_phase(CampaignID).

# has_incomplete_phase_task(PhaseID) - helper: phase has incomplete tasks
Decl has_incomplete_phase_task(PhaseID).

# has_incomplete_hard_dep(TaskID) - helper: task has incomplete hard dependencies
Decl has_incomplete_hard_dep(TaskID).

# has_earlier_phase(PhaseID) - helper: there are earlier phases to complete
Decl has_earlier_phase(PhaseID).

# has_earlier_task(PhaseID, TaskID) - helper: there are earlier tasks in phase
Decl has_earlier_task(PhaseID, TaskID).

# all_phase_tasks_complete(PhaseID) - derived: all tasks in phase are complete
Decl all_phase_tasks_complete(PhaseID).

# campaign_complete(CampaignID) - derived: entire campaign is complete
Decl campaign_complete(CampaignID).

# -----------------------------------------------------------------------------
# 32.2 Campaign Derived Predicates (from policy.mg Section 19)
# -----------------------------------------------------------------------------

# failed_campaign_task(CampaignID, TaskID) - derived: task failed during campaign
Decl failed_campaign_task(CampaignID, TaskID).

# priority_higher(PriorityA, PriorityB) - priority ordering helper
# Returns true if PriorityA is higher than PriorityB
Decl priority_higher(PriorityA, PriorityB).

# has_blocking_task_dep(TaskID) - helper: task has incomplete blocking dependencies
Decl has_blocking_task_dep(TaskID).

# task_conflict_active(TaskID) - helper: task conflicts with an in-progress task
Decl task_conflict_active(TaskID).

# has_passed_checkpoint(PhaseID, CheckType) - helper: phase has passed checkpoint
Decl has_passed_checkpoint(PhaseID, CheckType).

# phase_success_pattern(PhaseType) - tracks successful phase types for learning
Decl phase_success_pattern(PhaseType).

# phase_tool_permitted(Tool) - derived: tool is permitted in current phase profile
Decl phase_tool_permitted(Tool).

# tool_advisory_block(Tool, Reason) - advisory: tool not in phase profile
Decl tool_advisory_block(Tool, Reason).

# =============================================================================
# SECTION 33: SYSTEM SHARD COORDINATION
# =============================================================================
# Predicates for coordinating the 6 Type 1 system shards:
# - perception_firewall: NL → atoms transduction
# - world_model_ingestor: file_topology, symbol_graph maintenance
# - executive_policy: next_action derivation
# - constitution_gate: safety enforcement
# - tactile_router: action → tool routing
# - session_planner: agenda/campaign orchestration

# -----------------------------------------------------------------------------
# 33.1 System Shard Registry
# -----------------------------------------------------------------------------

# system_shard(ShardName, Type) - registered system shards with type
Decl system_shard(ShardName, Type).

# system_shard_state(ShardName, State) - current shard state
# State: /idle, /starting, /running, /stopping, /stopped, /error
Decl system_shard_state(ShardName, State).

# system_heartbeat(ShardName, Timestamp) - last heartbeat from shard
Decl system_heartbeat(ShardName, Timestamp).

# Campaign Runner supervision facts
Decl campaign_runner_heartbeat(Timestamp).
Decl campaign_runner_active(CampaignID, Timestamp).
Decl campaign_runner_success(CampaignID, Timestamp).
Decl campaign_runner_failure(CampaignID, Reason, Timestamp).

# -----------------------------------------------------------------------------
# 33.2 Intent Processing Flow
# -----------------------------------------------------------------------------

# processed_intent(IntentID) - intent has been processed by perception
Decl processed_intent(IntentID).

# executive_processed_intent(IntentID) - intent has been consumed by the executive
# (i.e., executive_policy emitted the first action envelope for it).
Decl executive_processed_intent(IntentID).

# pending_intent(IntentID) - derived: intent waiting to be processed
Decl pending_intent(IntentID).

# -----------------------------------------------------------------------------
# 33.3 Action Flow (Executive → Constitution → Router)
# -----------------------------------------------------------------------------

# pending_action(ActionID, ActionType, Target, Payload, Timestamp) - action awaiting permission check
# ActionID is a unique string generated by the executive (or shard) for correlation.
# Payload may be a map or additional args to pass to VirtualStore.
Decl pending_action(ActionID, ActionType, Target, Payload, Timestamp).

# permitted_action(ActionID, ActionType, Target, Payload, Timestamp) - action cleared by constitution gate
# This is the primary action stream for the tactile router today.
Decl permitted_action(ActionID, ActionType, Target, Payload, Timestamp).

# pending_permission_check(ActionID) - derived: action needs constitution check
Decl pending_permission_check(ActionID).

# action_permitted(ActionID) - action passed constitution gate (derived from permission_check_result)
Decl action_permitted(ActionID).

# ready_for_routing(ActionID) - derived: action ready for router
Decl ready_for_routing(ActionID).

# exec_request(ToolName, Target, Timeout, CallID, Timestamp) - router output
Decl exec_request(ToolName, Target, Timeout, CallID, Timestamp).

# -----------------------------------------------------------------------------
# 33.4 Safety & Violations
# -----------------------------------------------------------------------------

# security_violation(ActionType, Reason, Timestamp) - blocked by constitution
Decl security_violation(ActionType, Reason, Timestamp).

# escalation_needed(Target, Subject, Reason) - needs human intervention
# Target: system component (e.g., /system_health, /session_planner, /ooda_loop)
# Subject: entity being escalated (e.g., ShardName, ItemID)
# Reason: why escalation is needed
Decl escalation_needed(Target, Subject, Reason).

# rule_proposal_pending(ShardName, MangleCode, Rationale, Confidence, Timestamp)
Decl rule_proposal_pending(ShardName, MangleCode, Rationale, Confidence, Timestamp).

# -----------------------------------------------------------------------------
# 33.5 System Health
# -----------------------------------------------------------------------------

# system_shard_healthy(ShardName) - derived: shard heartbeat recent
Decl system_shard_healthy(ShardName).

# system_shard_unhealthy(ShardName) - derived: shard heartbeat stale
Decl system_shard_unhealthy(ShardName).

# world_model_heartbeat(ShardID, FileCount, Timestamp) - world model status
Decl world_model_heartbeat(ShardID, FileCount, Timestamp).

# world_model_updating(UpdateType, Scope) - trigger for incremental world model updates
Decl world_model_updating(UpdateType, Scope).

# session_planner_status(Total, Pending, InProgress, Completed, Blocked, Timestamp)
Decl session_planner_status(Total, Pending, InProgress, Completed, Blocked, Timestamp).

# plan_task(TaskID, Description, Status, ProgressPct) - individual task state
# Status: /pending, /in_progress, /completed, /blocked
Decl plan_task(TaskID, Description, Status, ProgressPct).

# plan_progress(CampaignID, TotalTasks, CompletedTasks, ProgressPct) - overall plan progress
Decl plan_progress(CampaignID, TotalTasks, CompletedTasks, ProgressPct).

# -----------------------------------------------------------------------------
# 33.6 Routing & Tool Management
# -----------------------------------------------------------------------------

# routing_error(ActionType, Reason, Timestamp) - router couldn't find handler
Decl routing_error(ActionType, Reason, Timestamp).

# route_added(ActionPattern, ToolName, Timestamp) - new route added via autopoiesis
Decl route_added(ActionPattern, ToolName, Timestamp).

# system_event_handled(ActionType, Target, Timestamp) - internal kernel lifecycle event acknowledged
Decl system_event_handled(ActionType, Target, Timestamp).

# -----------------------------------------------------------------------------
# 33.7 Agenda & Planning
# -----------------------------------------------------------------------------

# agenda_item(ItemID, Description, Priority, Status, Timestamp)
Decl agenda_item(ItemID, Description, Priority, Status, Timestamp).

# session_checkpoint(CheckpointID, ItemsRemaining, Timestamp)
Decl session_checkpoint(CheckpointID, ItemsRemaining, Timestamp).

# task_completed(TaskID) - task marked complete
Decl task_completed(TaskID).

# task_blocked(TaskID) - task is blocked
Decl task_blocked(TaskID).

# -----------------------------------------------------------------------------
# 33.8 Perception Errors & Stats
# -----------------------------------------------------------------------------

# perception_error(Message, Timestamp) - perception shard error
Decl perception_error(Message, Timestamp).

# world_model_error(Message, Timestamp) - world model shard error
Decl world_model_error(Message, Timestamp).

# executive_error(Message, Timestamp) - executive shard error
Decl executive_error(Message, Timestamp).

# executive_trace(Action, FromRule, Rationale, Timestamp) - debug trace
Decl executive_trace(Action, FromRule, Rationale, Timestamp).

# strategy_activated(StrategyName, Timestamp) - strategy change
Decl strategy_activated(StrategyName, Timestamp).

# executive_blocked(Reason, Timestamp) - executive policy blocked
Decl executive_blocked(Reason, Timestamp).

# -----------------------------------------------------------------------------
# 33.8b Tactile Execution Audit Facts
# -----------------------------------------------------------------------------
# Facts generated by tactile/audit.go for execution event tracking

# execution_started(SessionID, RequestID, Binary, Timestamp) - command started
Decl execution_started(SessionID, RequestID, Binary, Timestamp).

# execution_command(RequestID, CommandString) - full command string
Decl execution_command(RequestID, CommandString).

# execution_working_dir(RequestID, WorkingDir) - working directory
Decl execution_working_dir(RequestID, WorkingDir).

# execution_completed(RequestID, ExitCode, DurationMs, Timestamp) - command finished
Decl execution_completed(RequestID, ExitCode, DurationMs, Timestamp).

# execution_output(RequestID, StdoutLen, StderrLen) - output lengths
Decl execution_output(RequestID, StdoutLen, StderrLen).

# execution_success(RequestID) - successful execution (exit code 0)
Decl execution_success(RequestID).

# execution_nonzero(RequestID, ExitCode) - non-zero exit code
Decl execution_nonzero(RequestID, ExitCode).

# execution_failure(RequestID, Error) - infrastructure failure
Decl execution_failure(RequestID, Error).

# execution_resource_usage(RequestID, CPUTimeMs, MemoryBytes) - resource metrics
Decl execution_resource_usage(RequestID, CPUTimeMs, MemoryBytes).

# execution_io(RequestID, ReadBytes, WriteBytes) - I/O metrics
Decl execution_io(RequestID, ReadBytes, WriteBytes).

# execution_sandbox(RequestID, SandboxMode) - sandbox mode used
Decl execution_sandbox(RequestID, SandboxMode).

# execution_killed(RequestID, Reason, DurationMs) - command was killed
Decl execution_killed(RequestID, Reason, DurationMs).

# execution_error(RequestID, ErrorMessage) - execution error
Decl execution_error(RequestID, ErrorMessage).

# execution_blocked(RequestID, Reason, Timestamp) - command was blocked by policy/sandbox
Decl execution_blocked(RequestID, Reason, Timestamp).

# -----------------------------------------------------------------------------
# 33.9 Policy Derived Predicates (Section 21 Support)
# -----------------------------------------------------------------------------

# Intent processing derived predicates
Decl intent_processed(IntentID).
Decl focus_needs_resolution(Ref).
Decl intent_ready_for_executive(IntentID).

# Action flow derived predicates
Decl action_pending_permission(ActionID).
Decl permission_checked(ActionID).
Decl permission_check_result(ActionID, Result, Reason, Timestamp).
Decl action_blocked(ActionID, Reason).
Decl action_routed(ActionID).
Decl routing_result(ActionID, Result, Details, Timestamp).
Decl routing_succeeded(ActionID).
Decl routing_failed(ActionID, Error).

# Health monitoring derived predicates
Decl shard_heartbeat_stale(ShardName).

# Safety derived predicates
Decl block_all_actions(Reason).
Decl security_anomaly(AnomalyID, Type, Details).
Decl anomaly_investigated(AnomalyID).
Decl investigation_result(AnomalyID, Result).
Decl violation_count(Pattern, Count).
Decl propose_safety_rule(Pattern).
Decl repeated_violation_pattern(Pattern).
Decl safety_violation(ViolationID, Pattern, ActionType, Timestamp).

# World model derived predicates
Decl world_model_stale(File).
Decl file_in_project(File).
Decl symbol_reachable(From, To).
# Priority: 70
Decl symbol_reachable_bounded(From, To, MaxDepth).
# Priority: 70
Decl symbol_reachable_safe(From, To).

# Routing derived predicates
Decl routing_table(ActionType, Tool, RiskLevel).
Decl tool_allowlist(Tool, Timestamp).
Decl tool_allowed(Tool, ActionType).
Decl route_action(ActionID, Tool).
Decl action_type(ActionID, ActionType).
Decl routing_blocked(ActionID, Reason).
Decl has_tool_for_action(ActionType).

# Agenda/Planning derived predicates
Decl agenda_item_ready(ItemID).
Decl has_incomplete_dependency(ItemID).
Decl agenda_dependency(ItemID, DepID).
Decl next_agenda_item(ItemID).
Decl has_higher_priority_item(ItemID).
Decl checkpoint_due().
Decl last_checkpoint_time(Timestamp).
Decl agenda_item_escalate(ItemID, Reason).
Decl item_retry_count(ItemID, Count).

# Shard activation derived predicates
Decl activate_shard(ShardName).
Decl system_startup(State).
Decl shard_startup(ShardName, Mode).

# Autopoiesis derived predicates
Decl unhandled_case_count(ShardName, Count).
Decl unhandled_cases(ShardName, Cases).
Decl propose_new_rule(ShardName).
Decl proposed_rule(RuleID, ShardName, MangleCode, Confidence).
Decl rule_needs_approval(RuleID).
Decl auto_apply_rule(RuleID).
Decl rule_applied(RuleID).
Decl applied_rule(RuleID, Timestamp).
Decl learning_signal(SignalType, RuleID).
# 1-arg variant renamed to avoid arity conflict (Mangle doesn't support overloading)
Decl quality_signal(SignalType).
Decl rule_outcome(RuleID, Outcome, Details).

# OODA loop derived predicates
Decl ooda_phase(Phase).
Decl has_next_action().
Decl current_ooda_phase(Phase).
Decl ooda_stalled(Reason).
Decl last_action_time(Timestamp).

# Builtin helper predicates
# Note: time_diff removed - use fn:minus(Now, Timestamp) inline in rules instead.
# Note: list_length is DEPRECATED - use fn:list:length(List) in transform pipelines instead.

