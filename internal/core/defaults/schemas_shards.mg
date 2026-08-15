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
Decl delegate_task(ShardType, TaskDescription, Status) bound [/name, /string, /name].

# delegation_candidate(IntentID, ShardType, Confidence) - EDB asserted by Go.
# Step 4: Go computes the verb->shard lookup (GetShardTypeForVerb, which reads
# the perception taxonomy corpus that is intentionally siloed from this kernel)
# and the perception confidence, then passes BOTH into the kernel as a fact so
# the DELEGATION DECISION (the confidence gate) can live in policy. ShardType is
# /none when no shard maps to the verb. Confidence is a 0-100 integer (Go scales
# the 0.0-1.0 float, matching the action_verified convention).
Decl delegation_candidate(IntentID, ShardType, Confidence) bound [/name, /name, /number].

# should_delegate(ShardType) - derived: the current intent should be delegated to
# ShardType. Replaces the hardcoded `shardType != "" && intent.Confidence >= 0.5`
# boolean at cmd/nerd/chat/process.go. Queried by Go, which falls back to the
# legacy Go boolean if the kernel is unavailable or returns nothing.
Decl should_delegate(ShardType) bound [/name].

# -----------------------------------------------------------------------------
# 6.0.1 Routing Arbitration (single decision point per turn)
# Rules live in policy/routing_arbitration.mg. Go asserts the per-turn EDB
# (user_intent, intent_signal, delegation_candidate, multi_step_signal), queries
# route_decision once, and executes exactly one lane. This replaces the old
# scattered Go gates (follow-up substring detector, stability bypass, ad-hoc
# confidence booleans) that made identical inputs route differently depending
# on hidden session state.
# -----------------------------------------------------------------------------

# route_decision(Route, ShardType) - derived: the single routing outcome for
# the current turn. Route: /respond_directly, /clarify, /multi_step, /delegate.
# ShardType carries the target shard for /delegate and is /none otherwise.
Decl route_decision(Route, ShardType) bound [/name, /name].

# wants_direct_answer() - derived: the user is asking for an answer (question or
# conversation), so the turn must terminate in prose, not shard work.
Decl wants_direct_answer() bound [].

# conversational_verb(Verb) - vocabulary: verbs whose outcome is always prose.
Decl conversational_verb(Verb) bound [/name].

# workhorse_verb(Verb) - vocabulary: verbs that delegate to a shard even when
# the request is phrased as a question ("can you review my code?").
Decl workhorse_verb(Verb) bound [/name].

# spawn_subagent(Persona) - derived: subagent should be spawned
# Persona is the closed persona vocabulary shared with persona/1
# (/researcher, /tester, /nemesis) - see policy/intent_routing_rules.mg.
Decl spawn_subagent(Persona) bound [/name].

# shard_profile(AgentName, Type, KnowledgePath)
Decl shard_profile(AgentName, Type, KnowledgePath) bound [/string, /name, /string].

# -----------------------------------------------------------------------------
# 6.1 Specialist/Advisor Predicates
# -----------------------------------------------------------------------------

# task_complexity(ComplexityLevel) - current task complexity level (1-arg)
# ComplexityLevel: /simple, /complex, /critical
Decl task_complexity(ComplexityLevel) bound [/name].

# task_complexity(Task, ComplexityLevel) - task-specific complexity (2-arg)
Decl task_complexity(Task, ComplexityLevel) bound [/string, /name].

# specialist_classification(Agent, AgentType, SpecialistType) - agent type classification
# Agent: closed specialist vocabulary (/goexpert, /mangleexpert, /northstar, ...)
# AgentType: /advisor, /executor
# SpecialistType: /strategic, /technical
Decl specialist_classification(Agent, AgentType, SpecialistType) bound [/name, /name, /name].

# specialist_knowledge_db(Specialist, DBPath) - specialist's knowledge database
# Specialist is the same closed vocabulary as specialist_classification/3 arg 1;
# DBPath is a filesystem path, so it stays a string.
Decl specialist_knowledge_db(Specialist, DBPath) bound [/name, /string].

# specialist_campaign_role(Specialist, Role) - specialist's role in campaigns
# Role: /phase_executor, /plan_reviewer, /alignment_guardian
Decl specialist_campaign_role(Specialist, Role) bound [/name, /name].

# specialist_can_execute(Specialist) - derived: specialist can execute tasks
# Projection of specialist_classification/3 arg 1, so it carries name constants.
Decl specialist_can_execute(Specialist) bound [/name].

# consultation_request(FromSpec, ToSpec, Question, Timestamp) - specialist consultation request
Decl consultation_request(FromSpec, ToSpec, Question, Timestamp) bound [/string, /string, /string, /number].

# specialist_should_execute(Specialist, Task) - derived: specialist should execute task directly
Decl specialist_should_execute(Specialist, Task) bound [/string, /string].

# specialist_should_advise(Specialist, Task) - derived: specialist should advise on task
Decl specialist_should_advise(Specialist, Task) bound [/string, /string].

# strategic_advisor_required(Task) - derived: task requires strategic advisor
Decl strategic_advisor_required(Task) bound [/string].

# specialist_context_source(Specialist, DBPath) - derived: route to specialist knowledge DB
# Straight projection of specialist_knowledge_db/2.
Decl specialist_context_source(Specialist, DBPath) bound [/name, /string].

# activate_specialist_for_phase(Specialist, Phase) - derived: activate specialist for campaign phase
# Specialist comes from specialist_campaign_role/2 (name constant); Phase is
# campaign_phase/6 arg 1, which is a PhaseID string, not a name constant.
Decl activate_specialist_for_phase(Specialist, Phase) bound [/name, /string].

# specialist_assists(Advisor, Executor) - derived: advisor can assist executor
# Both slots are specialist_classification/3 arg 1.
Decl specialist_assists(Advisor, Executor) bound [/name, /name].

# specialist_consultation_route(FromSpec, ToSpec, Question) - derived: consultation routing
Decl specialist_consultation_route(FromSpec, ToSpec, Question) bound [/string, /string, /string].

# specialist_allowed_tools(Specialist, Tool) - derived: tools specialist can use
# Specialist is specialist_classification/3 arg 1; Tool is the /write_file,
# /edit_file, /run_command ... tool vocabulary.
Decl specialist_allowed_tools(Specialist, Tool) bound [/name, /name].

# persona_tool_allowed(Persona, Tool) - derived: tools allowed for a persona
Decl persona_tool_allowed(Persona, Tool) bound [/name, /name].

# =============================================================================
# SECTION 30: CODER SHARD HELPERS
# =============================================================================

# file_content(FilePath, Content) - cached file content
Decl file_content(FilePath, Content) bound [/string, /string].

# coder_state(State) - current coder state
# State: /idle, /context_ready, /code_generated, /edit_applied, /build_passed, /build_failed
Decl coder_state(State) bound [/name].

# pending_edit(FilePath, Content) - pending edits
Decl pending_edit(FilePath, Content) bound [/string, /string].

# coder_block_write(FilePath, Reason) - derived: write is blocked
Decl coder_block_write(FilePath, Reason) bound [/string, /string].

# coder_safe_to_write(FilePath) - derived: safe to write
Decl coder_safe_to_write(FilePath) bound [/string].

# is_binary_file(FilePath) - file is binary (cannot edit)
Decl is_binary_file(FilePath) bound [/string].

# is_core_file(FilePath) - file is in core/critical path (higher risk)
Decl is_core_file(FilePath) bound [/string].

# dependent_count(Target, Count) - number of files depending on Target
# Computed by Go from dependency_link graph
Decl dependent_count(Target, Count) bound [/string, /number].

# is_interface_file(FilePath) - file contains interface/type definitions
Decl is_interface_file(FilePath) bound [/string].

# instruction_contains(Instruction, Substring) - checks if user instruction contains pattern
# Implemented by Go string search
Decl instruction_contains(Instruction, Substring) bound [/string, /string].

# instruction_contains_write(FilePath) - instruction mentions writing to file
Decl instruction_contains_write(FilePath) bound [/string].

# file_package(FilePath, PackageName) - maps file to its Go package
Decl file_package(FilePath, PackageName) bound [/string, /string].

# same_package(File1, File2) - files are in the same Go package
# Computed by Go: checks if file_package(File1, P) and file_package(File2, P)
Decl same_package(File1, File2) bound [/string, /string].

# tdd_state(State) - current TDD phase state
# State: /red (tests failing), /green (tests passing), /refactor
Decl tdd_state(State) bound [/name].

# tdd_retry_count(Count) - number of retries in current TDD loop
Decl tdd_retry_count(Count) bound [/number].

# type_definition_file(FilePath) - file contains type/struct definitions
Decl type_definition_file(FilePath) bound [/string].

# edit_analysis(FilePath, Property) - analysis results for pending edits
# Property: /handles_errors, /has_context, /has_waitgroup, /has_context_cancel,
#           /spawns_goroutine, /public_function, /does_io
Decl edit_analysis(FilePath, Property) bound [/string, /name].

# interface_definition(FilePath, Name, MethodCount) - interface definition info
Decl interface_definition(FilePath, Name, MethodCount) bound [/string, /string, /number].

# edit_operation(FilePath, Operation) - operations in pending edit
Decl edit_operation(FilePath, Operation) bound [/string, /name].

# function_metrics(FilePath, FuncName, Lines, Complexity) - function metrics
Decl function_metrics(FilePath, FuncName, Lines, Complexity) bound [/string, /string, /number, /number].

# function_params(FilePath, FuncName, ParamCount) - parameter count
Decl function_params(FilePath, FuncName, ParamCount) bound [/string, /string, /number].

# function_nesting(FilePath, FuncName, Depth) - nesting depth
Decl function_nesting(FilePath, FuncName, Depth) bound [/string, /string, /number].

# diagnostic_count(FilePath, Severity, Count) - diagnostics per file
Decl diagnostic_count(FilePath, Severity, Count) bound [/string, /name, /number].

# previous_coder_state(State) - previous state for progress tracking
Decl previous_coder_state(State) bound [/name].

# state_unchanged_count(Count) - how many times state hasn't changed
Decl state_unchanged_count(Count) bound [/number].

# path_contains(Path, Pattern) - checks if file path contains substring
Decl path_contains(Path, Pattern) bound [/string, /string].

# is_test_file(FilePath) - file is a test file (e.g., *_test.go)
# NOTE: Also declared in tester.mg for module separation
Decl is_test_file(FilePath) bound [/string].

# detected_language(FilePath, Language) - detected programming language of file
Decl detected_language(FilePath, Language) bound [/string, /name].

# testable_language(Language) - language supports automated testing
Decl testable_language(Language) bound [/name].

# is_public_api(FilePath) - file contains public API definitions
Decl is_public_api(FilePath) bound [/string].

# doc_exists_for(FilePath) - documentation exists for this file
Decl doc_exists_for(FilePath) bound [/string].

# test_file_for(TestFile, SourceFile) - TestFile contains tests for SourceFile
Decl test_file_for(TestFile, SourceFile) bound [/string, /string].

# build_state(State) - current build state
# State: /passing, /failing, /unknown
Decl build_state(State) bound [/name].

# build_result(Success, Output) - result of build action (added for Bug 10)
Decl build_result(Success, Output) bound [/name, /string].

# =============================================================================
# SECTION 31: REVIEWER SHARD HELPERS
# =============================================================================

# file_line_count(FilePath, LineCount) - line count per file
Decl file_line_count(FilePath, LineCount) bound [/string, /number].

# finding_count(Severity, Count) - count of findings by severity
Decl finding_count(Severity, Count) bound [/name, /number].

# style_rule(RuleID, RuleName, Threshold) - style rule definitions
# Threshold is the numeric budget the rule allows (max line length, max
# nesting depth, 0 for "none permitted"). Regex-shaped rules carry their
# pattern in style_rule_pattern/2, NOT in this slot.
Decl style_rule(RuleID, RuleName, Threshold) bound [/string, /string, /number].

# style_rule_pattern(RuleID, Pattern) - regex a style rule matches on
# Split out of style_rule/3 because a pattern is not a threshold: STY003 had
# "TODO|FIXME" sitting in the /number Threshold slot, which is the exact
# shape that aborts whole-program evaluation the moment anything compares it.
Decl style_rule_pattern(RuleID, Pattern) bound [/string, /string].

# permission_denied(Action, Reason) - derived: why permission was denied (Bug 12)
Decl permission_denied(Action, Reason) bound [/name, /string].

# checks_passed() - derived: positive confirmation of checks (Bug 10)
Decl checks_passed() bound [].

# safe_to_commit() - derived: all checks pass, safe to commit
Decl safe_to_commit() bound [].

# file_truncated(Path, MaxSize) - file content truncated (Bug 6)
Decl file_truncated(Path, MaxSize) bound [/string, /number].

# =============================================================================
# SECTION 32: SAFE NEGATION HELPERS
# =============================================================================
# These helpers support safe negation patterns in Mangle rules

# has_block_commit() - helper: true if any block_commit exists
Decl has_block_commit() bound [].

# has_active_refinement() - helper: true if any refinement in progress
Decl has_active_refinement() bound [].

# has_eligible_phase() - helper: true if any phase is eligible
Decl has_eligible_phase() bound [].

# has_next_campaign_task() - helper: true if there's a next task
Decl has_next_campaign_task() bound [].

# has_in_progress_phase() - helper: true if any phase in progress
Decl has_in_progress_phase() bound [].

# has_incomplete_phase(CampaignID) - helper: campaign has incomplete phases
Decl has_incomplete_phase(CampaignID) bound [/string].

# has_incomplete_phase_task(PhaseID) - helper: phase has incomplete tasks
Decl has_incomplete_phase_task(PhaseID) bound [/string].

# has_incomplete_hard_dep(TaskID) - helper: task has incomplete hard dependencies
Decl has_incomplete_hard_dep(TaskID) bound [/string].

# has_earlier_phase(PhaseID) - helper: there are earlier phases to complete
Decl has_earlier_phase(PhaseID) bound [/string].

# has_earlier_task(PhaseID, TaskID) - helper: there are earlier tasks in phase
Decl has_earlier_task(PhaseID, TaskID) bound [/string, /string].

# all_phase_tasks_complete(PhaseID) - derived: all tasks in phase are complete
Decl all_phase_tasks_complete(PhaseID) bound [/string].

# campaign_complete(CampaignID) - derived: entire campaign is complete
Decl campaign_complete(CampaignID) bound [/string].

# -----------------------------------------------------------------------------
# 32.2 Campaign Derived Predicates (from policy.mg Section 19)
# -----------------------------------------------------------------------------

# failed_campaign_task(CampaignID, TaskID) - derived: task failed during campaign
Decl failed_campaign_task(CampaignID, TaskID) bound [/string, /string].

# priority_higher(PriorityA, PriorityB) - priority ordering helper
# Returns true if PriorityA is higher than PriorityB.
# The six ground facts in policy/campaign_tasks.mg are name constants
# (/critical, /high, /normal, /low) and they join task_priority/2, whose Go
# producer emits campaign.TaskPriority values that already carry the leading
# slash ("/critical") and therefore land as name constants. This is an
# ordering table over an enum, not arithmetic - nothing sums or compares it.
Decl priority_higher(PriorityA, PriorityB) bound [/name, /name].

# has_blocking_task_dep(TaskID) - helper: task has incomplete blocking dependencies
Decl has_blocking_task_dep(TaskID) bound [/string].

# task_conflict_active(TaskID) - helper: task conflicts with an in-progress task
Decl task_conflict_active(TaskID) bound [/string].

# has_passed_checkpoint(PhaseID, CheckType) - helper: phase has passed checkpoint
Decl has_passed_checkpoint(PhaseID, CheckType) bound [/string, /name].

# phase_success_pattern(PhaseType) - tracks successful phase types for learning
Decl phase_success_pattern(PhaseType) bound [/name].

# phase_tool_permitted(Tool) - derived: tool is permitted in current phase profile
Decl phase_tool_permitted(Tool) bound [/name].

# tool_advisory_block(Tool, Reason) - advisory: tool not in phase profile
Decl tool_advisory_block(Tool, Reason) bound [/name, /string].

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
Decl system_shard(ShardName, Type) bound [/string, /name].

# system_shard_state(ShardName, State) - current shard state
# State: /idle, /starting, /running, /stopping, /stopped, /error
Decl system_shard_state(ShardName, State) bound [/string, /name].

# system_heartbeat(ShardName, Timestamp) - last heartbeat from shard
# ShardName is the system shard's TYPE name from the fixed registry
# (/perception_firewall, /executive_policy, /constitution_gate, /tactile_router,
# /session_planner, /world_model_ingestor, ...), NOT a spawn instance id. It is
# the same closed vocabulary that activate_shard/1 and shard_startup/2 hold, so
# it is /name. BaseSystemShard.EmitHeartbeat wraps its plain ID accordingly.
Decl system_heartbeat(ShardName, Timestamp) bound [/name, /number].

# Campaign Runner supervision facts
Decl campaign_runner_heartbeat(Timestamp) bound [/number].
Decl campaign_runner_active(CampaignID, Timestamp) bound [/string, /number].
Decl campaign_runner_success(CampaignID, Timestamp) bound [/string, /number].
Decl campaign_runner_failure(CampaignID, Reason, Timestamp) bound [/string, /string, /number].

# -----------------------------------------------------------------------------
# 33.2 Intent Processing Flow
# -----------------------------------------------------------------------------

# processed_intent(IntentID) - intent has been processed by perception
# IntentID is the same /name value user_intent/5 carries: every producer
# (chat/process.go, chat/process_seed.go, shards/system/perception.go) asserts
# it alongside user_intent from the same intentID variable, "/current_intent",
# which Fact.ToAtom promotes to a name constant.
Decl processed_intent(IntentID) bound [/name].

# executive_processed_intent(IntentID) - intent has been consumed by the executive
# (i.e., executive_policy emitted the first action envelope for it).
# Same /name intent id. shards/system/executive.go asserts the literal
# "/current_intent"; the 37 negated body literals in policy/capabilities.mg all
# match !executive_processed_intent(/current_intent).
Decl executive_processed_intent(IntentID) bound [/name].

# pending_intent(IntentID) - derived: intent waiting to be processed
# IntentID is the singleton name constant /current_intent: the only rule head
# is pending_intent(/current_intent) and the Go producers of user_intent/5
# emit "/current_intent", which Fact.ToAtom turns into a name constant.
Decl pending_intent(IntentID) bound [/name].

# -----------------------------------------------------------------------------
# 33.3 Action Flow (Executive → Constitution → Router)
# -----------------------------------------------------------------------------

# pending_action(ActionID, ActionType, Target, Payload, Timestamp) - action awaiting permission check
# ActionID is a unique string generated by the executive (or shard) for correlation.
# Payload may be a map or additional args to pass to VirtualStore.
Decl pending_action(ActionID, ActionType, Target, Payload, Timestamp) bound [/string, /name, /string, /string, /number].

# permitted_action(ActionID, ActionType, Target, Payload, Timestamp) - action cleared by constitution gate
# This is the primary action stream for the tactile router today.
Decl permitted_action(ActionID, ActionType, Target, Payload, Timestamp) bound [/string, /name, /string, /string, /number].

# pending_permission_check(ActionID) - derived: action needs constitution check
Decl pending_permission_check(ActionID) bound [/string].

# action_permitted(ActionID) - action passed constitution gate (derived from permission_check_result)
Decl action_permitted(ActionID) bound [/string].

# ready_for_routing(ActionID) - derived: action ready for router
Decl ready_for_routing(ActionID) bound [/string].

# route_to(Target, Reason) - Routing decision record
# Records where an action was routed and why (for transparency display)
Decl route_to(Target, Reason) bound [/name, /string].

# exec_request(ToolName, Target, Timeout, CallID, Timestamp) - router output
Decl exec_request(ToolName, Target, Timeout, CallID, Timestamp) bound [/string, /string, /number, /string, /number].

# -----------------------------------------------------------------------------
# 33.4 Safety & Violations
# -----------------------------------------------------------------------------

# security_violation(ActionType, Reason, Timestamp) - blocked by constitution
Decl security_violation(ActionType, Reason, Timestamp) bound [/name, /string, /number].

# escalation_needed(Target, Subject, Reason) - needs human intervention
# Target: system component, a closed vocabulary of name constants
#         (/system_health, /session_planner, /ooda_loop, /campaign,
#          /constitution_gate). Go producers must use types.MangleAtom.
# Subject: entity being escalated - an open identifier (ItemID, PhaseID) or a
#          composite like "write_file:/some/path.go" from escalationSubject().
#          Open text, so /string.
# Reason: why escalation is needed - free text
Decl escalation_needed(Target, Subject, Reason) bound [/name, /string, /string].

# rule_proposal_pending(ShardName, MangleCode, Rationale, Confidence, Timestamp)
Decl rule_proposal_pending(ShardName, MangleCode, Rationale, Confidence, Timestamp) bound [/string, /string, /string, /number, /number].

# -----------------------------------------------------------------------------
# 33.5 System Health
# -----------------------------------------------------------------------------

# system_shard_healthy(ShardName) - derived: shard heartbeat recent
# ShardName is inherited from system_heartbeat/2, so it is the /name-shaped
# shard type. The on-demand activation rules in policy/system_shards.mg negate
# this with literals (!system_shard_healthy(/session_planner)); while this was
# declared /string and Go asserted a bare "session_planner", the negation could
# never be satisfied and every activate_shard guard was permanently open.
Decl system_shard_healthy(ShardName) bound [/name].

# system_shard_unhealthy(ShardName) - derived: shard heartbeat stale
Decl system_shard_unhealthy(ShardName) bound [/string].

# world_model_heartbeat(ShardID, FileCount, Timestamp) - world model status
Decl world_model_heartbeat(ShardID, FileCount, Timestamp) bound [/string, /number, /number].

# world_model_updating(UpdateType, Scope) - trigger for incremental world model updates
Decl world_model_updating(UpdateType, Scope) bound [/name, /name].

# session_planner_status(Total, Pending, InProgress, Completed, Blocked, Timestamp)
Decl session_planner_status(Total, Pending, InProgress, Completed, Blocked, Timestamp) bound [/number, /number, /number, /number, /number, /number].

# plan_task(TaskID, Description, Status, ProgressPct) - individual task state
# Status: /pending, /in_progress, /completed, /blocked
Decl plan_task(TaskID, Description, Status, ProgressPct) bound [/string, /string, /name, /number].

# plan_progress(CampaignID, TotalTasks, CompletedTasks, ProgressPct) - overall plan progress
Decl plan_progress(CampaignID, TotalTasks, CompletedTasks, ProgressPct) bound [/string, /number, /number, /number].

# -----------------------------------------------------------------------------
# 33.6 Routing & Tool Management
# -----------------------------------------------------------------------------

# routing_error(ActionType, Reason, Timestamp) - router couldn't find handler
Decl routing_error(ActionType, Reason, Timestamp) bound [/name, /string, /number].

# route_added(ActionPattern, ToolName, Timestamp) - new route added via autopoiesis
Decl route_added(ActionPattern, ToolName, Timestamp) bound [/string, /string, /number].

# system_event_handled(ActionType, Target, Timestamp) - internal kernel lifecycle event acknowledged
Decl system_event_handled(ActionType, Target, Timestamp) bound [/name, /string, /number].

# -----------------------------------------------------------------------------
# 33.7 Agenda & Planning
# -----------------------------------------------------------------------------

# agenda_item(ItemID, Description, Priority, Status, Timestamp)
Decl agenda_item(ItemID, Description, Priority, Status, Timestamp) bound [/string, /string, /number, /name, /number].

# session_checkpoint(CheckpointID, ItemsRemaining, Timestamp)
Decl session_checkpoint(CheckpointID, ItemsRemaining, Timestamp) bound [/string, /number, /number].

# task_completed(TaskID) - task marked complete
Decl task_completed(TaskID) bound [/string].

# task_blocked(TaskID) - task is blocked
Decl task_blocked(TaskID) bound [/string].

# -----------------------------------------------------------------------------
# 33.8 Perception Errors & Stats
# -----------------------------------------------------------------------------

# perception_error(Message, Timestamp) - perception shard error
Decl perception_error(Message, Timestamp) bound [/string, /number].

# world_model_error(Message, Timestamp) - world model shard error
Decl world_model_error(Message, Timestamp) bound [/string, /number].

# executive_error(Message, Timestamp) - executive shard error
Decl executive_error(Message, Timestamp) bound [/string, /number].

# executive_trace(Action, FromRule, Rationale, Timestamp) - debug trace
Decl executive_trace(Action, FromRule, Rationale, Timestamp) bound [/name, /string, /string, /number].

# strategy_activated(StrategyName, Timestamp) - strategy change
Decl strategy_activated(StrategyName, Timestamp) bound [/string, /number].

# executive_blocked(Reason, Timestamp) - executive policy blocked
Decl executive_blocked(Reason, Timestamp) bound [/string, /number].

# -----------------------------------------------------------------------------
# 33.8b Tactile Execution Audit Facts
# -----------------------------------------------------------------------------
# Facts generated by tactile/audit.go for execution event tracking

# execution_started(SessionID, RequestID, Binary, Timestamp) - command started
Decl execution_started(SessionID, RequestID, Binary, Timestamp) bound [/string, /string, /string, /number].

# execution_command(RequestID, CommandString) - full command string
Decl execution_command(RequestID, CommandString) bound [/string, /string].

# execution_working_dir(RequestID, WorkingDir) - working directory
Decl execution_working_dir(RequestID, WorkingDir) bound [/string, /string].

# execution_completed(RequestID, ExitCode, DurationMs, Timestamp) - command finished
Decl execution_completed(RequestID, ExitCode, DurationMs, Timestamp) bound [/string, /number, /number, /number].

# execution_output(RequestID, StdoutLen, StderrLen) - output lengths
Decl execution_output(RequestID, StdoutLen, StderrLen) bound [/string, /number, /number].

# execution_success(RequestID) - successful execution (exit code 0)
Decl execution_success(RequestID) bound [/string].

# execution_nonzero(RequestID, ExitCode) - non-zero exit code
Decl execution_nonzero(RequestID, ExitCode) bound [/string, /number].

# execution_failure(RequestID, Error) - infrastructure failure
Decl execution_failure(RequestID, Error) bound [/string, /string].

# execution_resource_usage(RequestID, CPUTimeMs, MemoryBytes) - resource metrics
Decl execution_resource_usage(RequestID, CPUTimeMs, MemoryBytes) bound [/string, /number, /number].

# execution_io(RequestID, ReadBytes, WriteBytes) - I/O metrics
Decl execution_io(RequestID, ReadBytes, WriteBytes) bound [/string, /number, /number].

# execution_sandbox(RequestID, SandboxMode) - sandbox mode used
Decl execution_sandbox(RequestID, SandboxMode) bound [/string, /name].

# execution_killed(RequestID, Reason, DurationMs) - command was killed
Decl execution_killed(RequestID, Reason, DurationMs) bound [/string, /string, /number].

# execution_error(RequestID, ErrorMessage) - execution error
Decl execution_error(RequestID, ErrorMessage) bound [/string, /string].

# execution_blocked(RequestID, Reason, Timestamp) - command was blocked by policy/sandbox
Decl execution_blocked(RequestID, Reason, Timestamp) bound [/string, /string, /number].

# Structured summaries parsed from test and build output. These names are
# deliberately execution-scoped so they cannot collide with tester/world facts
# that use test_result, failed_test, test_coverage, build_result, or diagnostic
# with different meanings and arities.
Decl execution_test_summary(RequestID, Passed, Failed, Skipped) bound [/string, /number, /number, /number].
Decl execution_test_state(RequestID, State) bound [/string, /name].
Decl execution_failed_test(RequestID, TestName) bound [/string, /string].
Decl execution_test_coverage(RequestID, CoveragePercent) bound [/string, /number].
Decl execution_build_summary(RequestID, Success, Errors, Warnings) bound [/string, /name, /number, /number].
Decl execution_diagnostic(RequestID, Severity, FilePath, Line, Column, Message) bound [/string, /name, /string, /number, /number, /string].

# -----------------------------------------------------------------------------
# 33.9 Policy Derived Predicates (Section 21 Support)
# -----------------------------------------------------------------------------

# Intent processing derived predicates
Decl intent_processed(IntentID) bound [/string].
Decl focus_needs_resolution(Ref) bound [/string].
# IntentID matches pending_intent/1: the singleton name constant /current_intent.
Decl intent_ready_for_executive(IntentID) bound [/name].

# Action flow derived predicates
Decl action_pending_permission(ActionID) bound [/string].
Decl permission_checked(ActionID) bound [/string].
Decl permission_check_result(ActionID, Result, Reason, Timestamp) bound [/string, /name, /string, /number].
Decl action_blocked(ActionID, Reason) bound [/string, /string].
Decl action_routed(ActionID) bound [/string].
Decl routing_result(ActionID, Result, Details, Timestamp) bound [/string, /name, /string, /number].
Decl routing_succeeded(ActionID) bound [/string].
Decl routing_failed(ActionID, Error) bound [/string, /string].

# Health monitoring derived predicates
Decl shard_heartbeat_stale(ShardName) bound [/string].

# Safety derived predicates
Decl block_all_actions(Reason) bound [/string].
Decl security_anomaly(AnomalyID, Type, Details) bound [/string, /name, /string].
Decl anomaly_investigated(AnomalyID) bound [/string].
Decl investigation_result(AnomalyID, Result) bound [/string, /string].
Decl violation_count(Pattern, Count) bound [/string, /number].
Decl propose_safety_rule(Pattern) bound [/string].
Decl repeated_violation_pattern(Pattern) bound [/string].
Decl safety_violation(ViolationID, Pattern, ActionType, Timestamp) bound [/string, /string, /name, /number].

# World model derived predicates
Decl world_model_stale(File) bound [/string].
Decl file_in_project(File) bound [/string].
Decl symbol_reachable(From, To) bound [/string, /string].
# Priority: 70
Decl symbol_reachable_bounded(From, To, MaxDepth) bound [/string, /string, /number].
# Priority: 70
Decl symbol_reachable_safe(From, To) bound [/string, /string].
# Depth generator for bounded recursion (1-20)
Decl depth_option(Depth) bound [/number].

# Routing derived predicates
Decl routing_table(ActionType, Tool, RiskLevel) bound [/name, /name, /name].
Decl tool_allowlist(Tool, Timestamp) bound [/name, /number].
Decl tool_allowed(Tool, ActionType) bound [/name, /name].
Decl route_action(ActionID, Tool) bound [/string, /name].
Decl action_type(ActionID, ActionType) bound [/string, /name].
Decl routing_blocked(ActionID, Reason) bound [/string, /string].
Decl has_tool_for_action(ActionType) bound [/name].

# Agenda/Planning derived predicates
Decl agenda_item_ready(ItemID) bound [/string].
Decl has_incomplete_dependency(ItemID) bound [/string].
Decl agenda_dependency(ItemID, DepID) bound [/string, /string].
Decl next_agenda_item(ItemID) bound [/string].
Decl has_higher_priority_item(ItemID) bound [/string].
Decl checkpoint_due() bound [].
Decl last_checkpoint_time(Timestamp) bound [/number].
Decl agenda_item_escalate(ItemID, Reason) bound [/string, /string].
Decl item_retry_count(ItemID, Count) bound [/string, /number].

# Shard activation derived predicates
# ShardName is the closed system-shard vocabulary (/perception_firewall,
# /executive_policy, /constitution_gate, /world_model_ingestor,
# /tactile_router, /session_planner). The Go consumer of activate_shard
# (ShardManager.StartSystemShards) normalizes with TrimLeft("/"), so it reads
# name constants correctly.
Decl activate_shard(ShardName) bound [/name].
Decl system_startup(State) bound [/name].
Decl shard_startup(ShardName, Mode) bound [/name, /name].

# Autopoiesis derived predicates
Decl unhandled_case_count(ShardName, Count) bound [/string, /number].
Decl unhandled_cases(ShardName, Cases) bound [/string, /string].
# ShardName here is a policy-module / shard identity name constant. The only
# live producer is propose_new_rule(/verification_policy) in policy/verification.mg;
# the system_autopoiesis.mg rule that would bind it from a variable is inert
# because nothing (Go or .mg) ever asserts system_shard/2.
Decl propose_new_rule(ShardName) bound [/name].
Decl proposed_rule(RuleID, ShardName, MangleCode, Confidence) bound [/string, /string, /string, /number].
Decl rule_needs_approval(RuleID) bound [/string].
Decl auto_apply_rule(RuleID) bound [/string].
Decl rule_applied(RuleID) bound [/string].
Decl applied_rule(RuleID, Timestamp) bound [/string, /number].
Decl learning_signal(SignalType, RuleID) bound [/name, /string].
# 1-arg variant renamed to avoid arity conflict (Mangle doesn't support overloading)
Decl quality_signal(SignalType) bound [/name].
Decl rule_outcome(RuleID, Outcome, Details) bound [/string, /name, /string].

# OODA loop derived predicates
Decl ooda_phase(Phase) bound [/name].
Decl has_next_action() bound [].
Decl current_ooda_phase(Phase) bound [/name].
Decl ooda_stalled(Reason) bound [/string].
Decl last_action_time(Timestamp) bound [/number].

# Builtin helper predicates
# Note: time_diff removed - use fn:minus(Now, Timestamp) inline in rules instead.
# Note: list_length is DEPRECATED - use fn:list:length(List) in transform pipelines instead.
