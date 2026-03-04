# Campaign Tasks Logic
# Extracted from campaign.mg
# Stratification: Depends on campaign_phases.mg (for current_phase)

# Local helper declaration for clock-availability gating in isolated policy tests.
Decl has_current_time().

# =============================================================================
# Task Selection & Execution
# =============================================================================

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

# Canonical task write path:
# 1) explicit runtime write contract via task_write_target
# 2) fallback to legacy task_artifact path
task_write_path(TaskID, Path) :-
    task_write_target(TaskID, Path).

task_write_path(TaskID, Path) :-
    task_artifact(TaskID, _, Path, _).

# Conflict heuristic: same canonical write path -> conflict
task_conflict(TaskID, OtherTaskID) :-
    task_write_path(TaskID, Path),
    task_write_path(OtherTaskID, Path),
    TaskID != OtherTaskID.

# Helper: check if there's an earlier pending task
has_earlier_task(TaskID, PhaseID) :-
    campaign_task(OtherTaskID, PhaseID, _, /pending, _),
    OtherTaskID != TaskID,
    task_priority(OtherTaskID, OtherPriority),
    task_priority(TaskID, Priority),
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

has_current_time() :-
    current_time(_).

# Fail closed: if no clock fact is available, keep retry tasks in backoff.
task_in_backoff(TaskID) :-
    task_retry_at(TaskID, _),
    !has_current_time().

# A current-phase task is waiting for retry window after lock timeout/backoff.
phase_has_backoff_task(PhaseID) :-
    current_phase(PhaseID),
    campaign_task(TaskID, PhaseID, _, /pending, _),
    task_in_backoff(TaskID).

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

# Helper: true if there's a next campaign task available
has_next_campaign_task() :-
    next_campaign_task(_).

# Backoff tasks keep campaign routing alive while waiting for retry.
# This prevents transient lock contention from being misclassified as hard blocking.
has_next_campaign_task() :-
    phase_has_backoff_task(_).

# Campaign blocked if all remaining tasks are blocked
campaign_blocked(CampaignID, "all_tasks_blocked") :-
    current_campaign(CampaignID),
    current_phase(PhaseID),
    !has_next_campaign_task(),
    !phase_has_backoff_task(PhaseID),
    has_incomplete_phase_task(PhaseID).

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

# =============================================================================
# Context Paging (Phase-Aware Spreading Activation)
# =============================================================================

# Boost activation for current phase context
activation(Fact, 150) :-
    current_phase(PhaseID),
    phase_context_atom(PhaseID, Fact, _).

# Boost files matching current task's target
activation(Target, 140) :-
    next_campaign_task(TaskID),
    campaign_task(TaskID, _, _, _, _),
    task_write_path(TaskID, Target).

# Suppress context from completed phases
activation(Fact, -50) :-
    context_compression(PhaseID, _, _, _),
    phase_context_atom(PhaseID, Fact, _).

# =============================================================================
# Campaign-Aware Tool Permissions
# =============================================================================

# During campaigns, only permit tools in the phase's context profile
phase_tool_permitted(Tool) :-
    current_phase(PhaseID),
    campaign_phase(PhaseID, _, _, _, _, ContextProfile),
    context_profile(ContextProfile, _, RequiredTools, _),
    tool_in_list(Tool, RequiredTools).

# Block tools not in phase profile during active campaign
tool_advisory_block(Tool, "not_in_phase_profile") :-
    current_campaign(_),
    current_phase(_),
    tool_capabilities(Tool, _),
    !phase_tool_permitted(Tool).
