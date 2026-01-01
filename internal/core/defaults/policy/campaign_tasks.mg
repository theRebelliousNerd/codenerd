# Campaign Tasks Logic
# Task selection, execution, and tool permissions

# Task Selection & Execution

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

# --- Helpers ---

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
pending_task_priority(TaskID, PhaseID, Priority) :-
    campaign_task(TaskID, PhaseID, _, /pending, _),
    task_priority(TaskID, Priority).

has_earlier_task(TaskID, PhaseID) :-
    pending_task_priority(OtherTaskID, PhaseID, OtherPriority),
    pending_task_priority(TaskID, PhaseID, Priority),
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
    current_time(Now),
    task_retry_at(TaskID, RetryAt),
    Now < RetryAt.

# --- Tool Permissions ---

# During campaigns, only permit tools in the phase's context profile
phase_tool_permitted(Tool) :-
    current_phase(PhaseID),
    campaign_phase(PhaseID, _, _, _, _, ContextProfile),
    context_profile(ContextProfile, _, RequiredTools, _),
    tool_in_list(Tool, RequiredTools).

# Block tools not in phase profile during active campaign
# NOTE: E023 warning is false positive - current_campaign/current_phase are singletons (1×1×N)
tool_advisory_block(Tool, /not_in_phase_profile) :-
    current_campaign(CampaignID),
    campaign_phase(PhaseID, CampaignID, _, _, _, _),
    current_phase(PhaseID),
    tool_capabilities(Tool, _),
    !phase_tool_permitted(Tool).
