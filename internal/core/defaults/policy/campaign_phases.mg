# Campaign Phases Logic
# Extracted from campaign.mg
# Stratification: Depends on schemas_campaign.mg

# =============================================================================
# Campaign State Machine
# =============================================================================

# Current campaign is the one that's active
current_campaign(CampaignID) :-
    campaign(CampaignID, _, _, _, /active).

# Campaign execution strategy activates when a campaign is active
active_strategy(/campaign_execution) :-
    current_campaign(_).

# =============================================================================
# Phase Eligibility & Sequencing
# =============================================================================

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
    campaign_phase(PhaseID, _, _, Order, _, _),
    phase_eligible(OtherPhaseID),
    OtherPhaseID != PhaseID,
    campaign_phase(OtherPhaseID, _, _, OtherOrder, _, _),
    OtherOrder < Order.

# Helper: check if any phase is in progress
has_in_progress_phase() :-
    campaign_phase(_, CampaignID, _, _, /in_progress, _),
    current_campaign(CampaignID).

# Current phase: lowest order eligible phase, or the one in progress
current_phase(PhaseID) :-
    campaign_phase(PhaseID, CampaignID, _, _, /in_progress, _),
    current_campaign(CampaignID).

current_phase(PhaseID) :-
    phase_eligible(PhaseID),
    !has_earlier_phase(PhaseID),
    !has_in_progress_phase().

# Phase is blocked if it has incomplete hard dependencies
phase_blocked(PhaseID, "hard_dependency_incomplete") :-
    campaign_phase(PhaseID, CampaignID, _, _, /pending, _),
    current_campaign(CampaignID),
    has_incomplete_hard_dep(PhaseID).

# =============================================================================
# Checkpoint & Verification
# =============================================================================

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

# =============================================================================
# Replanning Triggers
# =============================================================================

# Helper: identify failed tasks (for counting in Go runtime)
failed_campaign_task(CampaignID, TaskID) :-
    current_campaign(CampaignID),
    campaign_task(TaskID, PhaseID, Desc, /failed, TaskType),
    campaign_phase(PhaseID, CampaignID, PhaseName, Seq, Status, Profile).

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

# =============================================================================
# Campaign Helpers & Blocking
# =============================================================================

# Helper: true if any phase is eligible to start
has_eligible_phase() :-
    phase_eligible(_).

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

# Campaign blocked if no eligible phases and none in progress
campaign_blocked(CampaignID, "no_eligible_phases") :-
    current_campaign(CampaignID),
    !has_eligible_phase(),
    !has_in_progress_phase(),
    has_incomplete_phase(CampaignID).
