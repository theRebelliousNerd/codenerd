# Campaign Orchestration
# Section 19 of Cortex Executive Policy


# Campaign State Machine

# Current campaign is the one that's active
current_campaign(CampaignID) :-
    campaign(CampaignID, _, _, _, /active).

# Campaign execution strategy activates when a campaign is active
active_strategy(/campaign_execution) :-
    current_campaign(_).

# Phase Eligibility & Sequencing

# Helper: check if a phase has incomplete hard dependencies
has_incomplete_hard_dep(PhaseID) :-
    phase_dependency(PhaseID, DepPhaseID, /hard),
    campaign_phase(DepPhaseID, _, _, _, Status, _),
    /completed != Status.

# A phase is eligible when all hard dependencies are complete
phase_eligible(PhaseID) :-
    campaign_phase(PhaseID, CampaignID, _, _, /pending, _),
    current_campaign(CampaignID),
    !has_incomplete_hard_dep(PhaseID).

# Helper: check if there's an earlier eligible phase
has_earlier_phase(PhaseID) :-
    campaign_phase(PhaseID, CampaignID, _, Order, _, _),
    phase_eligible(OtherPhaseID),
    campaign_phase(OtherPhaseID, CampaignID, _, OtherOrder, _, _),
    OtherPhaseID != PhaseID,
    OtherOrder < Order.

# Current phase: lowest order eligible phase, or the one in progress
current_phase(PhaseID) :-
    campaign_phase(PhaseID, CampaignID, _, _, /in_progress, _),
    current_campaign(CampaignID).

current_phase(PhaseID) :-
    phase_eligible(PhaseID),
    !has_earlier_phase(PhaseID),
    !has_in_progress_phase().

# Helper: check if any phase is in progress
has_in_progress_phase() :-
    campaign_phase(_, CampaignID, _, _, /in_progress, _),
    current_campaign(CampaignID).

# Phase is blocked if it has incomplete hard dependencies
phase_blocked(PhaseID, "hard_dependency_incomplete") :-
    campaign_phase(PhaseID, CampaignID, _, _, /pending, _),
    current_campaign(CampaignID),
    has_incomplete_hard_dep(PhaseID).

# Task Selection & Execution

# Helper: check if task has blocking dependencies
has_blocking_task_dep(TaskID) :-
    task_dependency(TaskID, BlockerID),
    campaign_task(BlockerID, _, _, Status, _),
    /completed != Status,
    /skipped != Status.

# Helper: check if task conflicts with an in-progress task
task_conflict_active(TaskID) :-
    task_conflict(TaskID, OtherTaskID),
    campaign_task(OtherTaskID, _, _, /in_progress, _).

task_conflict_active(TaskID) :-
    task_conflict(OtherTaskID, TaskID),
    campaign_task(OtherTaskID, _, _, /in_progress, _).

# Optional conflict heuristic: same artifact path -> conflict
task_conflict(TaskID, OtherTaskID) :-
    task_artifact(TaskID, _, Path, _),
    task_artifact(OtherTaskID, _, Path, _),
    TaskID != OtherTaskID.

# Helper: check if there's an earlier pending task
has_earlier_task(TaskID, PhaseID) :-
    campaign_task(OtherTaskID, PhaseID, _, /pending, _),
    campaign_task(TaskID, PhaseID, _, /pending, _),
    task_priority(OtherTaskID, OtherPriority),
    task_priority(TaskID, Priority),
    OtherTaskID != TaskID,
    priority_higher(OtherPriority, Priority).

# Priority ordering helper
priority_higher(/critical, /high).
priority_higher(/critical, /normal).
priority_higher(/critical, /low).
priority_higher(/high, /normal).
priority_higher(/high, /low).
priority_higher(/normal, /low).

# Task is in backoff window if retry time is in the future.
task_in_backoff(TaskID) :-
    task_retry_at(TaskID, RetryAt),
    current_time(Now),
    Now < RetryAt.

# Eligible tasks: highest-priority pending tasks in the current phase without blockers or conflicts
eligible_task(TaskID) :-
    current_phase(PhaseID),
    campaign_task(TaskID, PhaseID, _, /pending, _),
    !task_in_backoff(TaskID),
    !has_blocking_task_dep(TaskID),
    !has_earlier_task(TaskID, PhaseID),
    !task_conflict_active(TaskID).

# Next task remains available for single-dispatch clients
next_campaign_task(TaskID) :-
    eligible_task(TaskID).

# Derive next_action based on campaign task type
next_action(/campaign_create_file) :-
    next_campaign_task(TaskID),
    campaign_task(TaskID, _, _, _, /file_create).

next_action(/campaign_modify_file) :-
    next_campaign_task(TaskID),
    campaign_task(TaskID, _, _, _, /file_modify).

next_action(/campaign_write_test) :-
    next_campaign_task(TaskID),
    campaign_task(TaskID, _, _, _, /test_write).

next_action(/campaign_run_test) :-
    next_campaign_task(TaskID),
    campaign_task(TaskID, _, _, _, /test_run).

next_action(/campaign_research) :-
    next_campaign_task(TaskID),
    campaign_task(TaskID, _, _, _, /research).

next_action(/campaign_verify) :-
    next_campaign_task(TaskID),
    campaign_task(TaskID, _, _, _, /verify).

next_action(/campaign_document) :-
    next_campaign_task(TaskID),
    campaign_task(TaskID, _, _, _, /document).

next_action(/campaign_refactor) :-
    next_campaign_task(TaskID),
    campaign_task(TaskID, _, _, _, /refactor).

next_action(/campaign_integrate) :-
    next_campaign_task(TaskID),
    campaign_task(TaskID, _, _, _, /integrate).

# Auto-spawn researcher shard for research tasks
delegate_task(/researcher, Description, /pending) :-
    next_campaign_task(TaskID),
    campaign_task(TaskID, _, Description, _, /research).

# Auto-spawn coder shard for file creation/modification
delegate_task(/coder, Description, /pending) :-
    next_campaign_task(TaskID),
    campaign_task(TaskID, _, Description, _, /file_create).

delegate_task(/coder, Description, /pending) :-
    next_campaign_task(TaskID),
    campaign_task(TaskID, _, Description, _, /file_modify).

# Auto-spawn tester shard for test tasks
delegate_task(/tester, Description, /pending) :-
    next_campaign_task(TaskID),
    campaign_task(TaskID, _, Description, _, /test_write).

delegate_task(/tester, Description, /pending) :-
    next_campaign_task(TaskID),
    campaign_task(TaskID, _, Description, _, /test_run).

# Context Paging (Phase-Aware Spreading Activation)

# Boost activation for current phase context
activation(Fact, 150) :-
    current_phase(PhaseID),
    phase_context_atom(PhaseID, Fact, _).

# Boost files matching current task's target
activation(Target, 140) :-
    next_campaign_task(TaskID),
    campaign_task(TaskID, _, _, _, _),
    task_artifact(TaskID, _, Target, _).

# Suppress context from completed phases
activation(Fact, -50) :-
    context_compression(PhaseID, _, _, _),
    phase_context_atom(PhaseID, Fact, _).

# Checkpoint & Verification

# Helper: check if phase has pending checkpoint
has_pending_checkpoint(PhaseID) :-
    phase_objective(PhaseID, _, _, VerifyMethod),
    /none != VerifyMethod,
    !has_passed_checkpoint(PhaseID, VerifyMethod).

has_passed_checkpoint(PhaseID, CheckType) :-
    phase_checkpoint(PhaseID, CheckType, /true, _, _).

# Helper: check if all phase tasks are complete
has_incomplete_phase_task(PhaseID) :-
    campaign_task(_, PhaseID, _, Status, _),
    /completed != Status,
    /skipped != Status.

all_phase_tasks_complete(PhaseID) :-
    campaign_phase(PhaseID, _, _, _, _, _),
    !has_incomplete_phase_task(PhaseID).

# Trigger checkpoint when all tasks complete but checkpoint pending
next_action(/run_phase_checkpoint) :-
    current_phase(PhaseID),
    all_phase_tasks_complete(PhaseID),
    has_pending_checkpoint(PhaseID).

# Block phase completion if checkpoint failed
phase_blocked(PhaseID, "checkpoint_failed") :-
    phase_checkpoint(PhaseID, _, /false, _, _).

# Replanning Triggers

# Helper: identify failed tasks (for counting in Go runtime)
failed_campaign_task(CampaignID, TaskID) :-
    current_campaign(CampaignID),
    campaign_phase(PhaseID, CampaignID, PhaseName, Seq, Status, Profile),
    campaign_task(TaskID, PhaseID, Desc, /failed, TaskType).

# Trigger replan on repeated failures (configurable threshold).
replan_needed(CampaignID, "task_failure_cascade") :-
    current_campaign(CampaignID),
    campaign_config(CampaignID, _, Threshold, /true, _),
    failed_campaign_task_count_computed(CampaignID, Count),
    Count >= Threshold.

# Trigger replan if user provides new instruction during campaign
replan_needed(CampaignID, "user_instruction") :-
    current_campaign(CampaignID),
    user_intent(/current_intent, /instruction, _, _, _).

# Trigger replan if explicit trigger exists
replan_needed(CampaignID, Reason) :-
    replan_trigger(CampaignID, Reason, _).

# Pause and replan action
next_action(/pause_and_replan) :-
    replan_needed(_, _).

# Campaign Helpers for Safe Negation

# Helper: true if any phase is eligible to start
has_eligible_phase() :-
    phase_eligible(_).

# Helper: true if there's a next campaign task available
has_next_campaign_task() :-
    next_campaign_task(_).

# Helper: check if any phase is not complete
has_incomplete_phase(CampaignID) :-
    campaign_phase(_, CampaignID, _, _, Status, _),
    /completed != Status,
    /skipped != Status.

# Campaign complete when all phases complete
campaign_complete(CampaignID) :-
    current_campaign(CampaignID),
    !has_incomplete_phase(CampaignID).

next_action(/campaign_complete) :-
    campaign_complete(_).

# Campaign Blocking Conditions

# Campaign blocked if no eligible phases and none in progress
campaign_blocked(CampaignID, "no_eligible_phases") :-
    current_campaign(CampaignID),
    !has_eligible_phase(),
    !has_in_progress_phase(),
    has_incomplete_phase(CampaignID).

# Campaign blocked if all remaining tasks are blocked
campaign_blocked(CampaignID, "all_tasks_blocked") :-
    current_campaign(CampaignID),
    campaign_phase(PhaseID, CampaignID, _, _, _, _),
    current_phase(PhaseID),
    !has_next_campaign_task(),
    has_incomplete_phase_task(PhaseID).

# Autopoiesis During Campaign

# Track successful phase types for learning (Go runtime extracts from kernel)
phase_success_pattern(PhaseType) :-
    campaign_phase(PhaseID, CampaignID, PhaseName, Seq, /completed, Profile),
    phase_objective(PhaseID, PhaseType, Desc, Priority),
    phase_checkpoint(PhaseID, CheckpointID, /true, ValidatedAt, ValidatorShard).

# Learn from phase completion - promotes success pattern for phase type
promote_to_long_term(/phase_success, PhaseType) :-
    phase_success_pattern(PhaseType).

# Learn from task failures for future avoidance
campaign_learning(CampaignID, /failure_pattern, TaskType, ErrorMsg, Now) :-
    current_campaign(CampaignID),
    campaign_phase(PhaseID, CampaignID, _, _, _, _),
    campaign_task(TaskID, PhaseID, _, /failed, TaskType),
    task_error(TaskID, _, ErrorMsg),
    current_time(Now).

# Campaign-Aware Tool Permissions

# During campaigns, only permit tools in the phase's context profile
phase_tool_permitted(Tool) :-
    current_phase(PhaseID),
    campaign_phase(PhaseID, _, _, _, _, ContextProfile),
    context_profile(ContextProfile, _, RequiredTools, _),
    tool_in_list(Tool, RequiredTools).

# Block tools not in phase profile during active campaign
# NOTE: E023 warning is false positive - current_campaign/current_phase are singletons (1×1×N)
tool_advisory_block(Tool, "not_in_phase_profile") :-
    current_campaign(CampaignID),
    campaign_phase(PhaseID, CampaignID, _, _, _, _),
    current_phase(PhaseID),
    tool_capabilities(Tool, _),
    !phase_tool_permitted(Tool).
