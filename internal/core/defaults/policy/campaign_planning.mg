# Campaign Planning Logic
# Phases, checkpoints, replanning, and learning

# --- Phase Eligibility & Sequencing ---

# A phase is eligible when all hard dependencies are complete
phase_eligible(PhaseID) :-
    phase_eligible_in_campaign(PhaseID, _).

# Helper: phase eligibility scoped to campaign
phase_eligible_in_campaign(PhaseID, CampaignID) :-
    campaign_phase(PhaseID, CampaignID, _, _, /pending, _),
    !has_incomplete_hard_dep_in_campaign(PhaseID, CampaignID),
    current_campaign(CampaignID).

# Helper: incomplete hard dependency scoped to campaign
has_incomplete_hard_dep_in_campaign(PhaseID, CampaignID) :-
    has_incomplete_hard_dep(PhaseID),
    campaign_phase(PhaseID, CampaignID, _, _, _, _).

# Helper: check if a phase has incomplete hard dependencies
has_incomplete_hard_dep(PhaseID) :-
    phase_dependency(PhaseID, DepPhaseID, /hard),
    campaign_phase(PhaseID, CampaignID, _, _, _, _),
    campaign_phase(DepPhaseID, CampaignID, _, _, Status, _),
    /completed != Status.

# Helper: check if there's an earlier eligible phase
has_earlier_phase(PhaseID) :-
    campaign_phase(PhaseID, CampaignID, _, Order, _, _),
    campaign_phase(OtherPhaseID, CampaignID, _, OtherOrder, _, _),
    phase_eligible_in_campaign(OtherPhaseID, CampaignID),
    OtherPhaseID != PhaseID,
    OtherOrder < Order.

# Current phase: lowest order eligible phase, or the one in progress
current_phase(PhaseID) :-
    campaign_phase(PhaseID, CampaignID, _, _, /in_progress, _),
    current_campaign(CampaignID).

current_phase(PhaseID) :-
    phase_eligible_in_campaign(PhaseID, _),
    !has_earlier_phase(PhaseID),
    !has_in_progress_phase().

# Phase is blocked if it has incomplete hard dependencies
phase_blocked(PhaseID, /hard_dependency_incomplete) :-
    campaign_phase(PhaseID, CampaignID, _, _, /pending, _),
    has_incomplete_hard_dep(PhaseID),
    current_campaign(CampaignID).

# --- Checkpoint & Verification ---

# Trigger checkpoint when all tasks complete but checkpoint pending
next_action(/run_phase_checkpoint) :-
    current_phase(PhaseID),
    all_phase_tasks_complete(PhaseID),
    has_pending_checkpoint(PhaseID).

# Block phase completion if checkpoint failed
phase_blocked(PhaseID, /checkpoint_failed) :-
    phase_checkpoint(PhaseID, _, /false, _, _).

# Helper: check if phase has pending checkpoint
has_pending_checkpoint(PhaseID) :-
    phase_objective(PhaseID, _, _, VerifyMethod),
    /none != VerifyMethod,
    !has_passed_checkpoint(PhaseID, VerifyMethod).

has_passed_checkpoint(PhaseID, CheckType) :-
    phase_checkpoint(PhaseID, CheckType, /true, _, _).

# --- Replanning Triggers ---

# Trigger replan on repeated failures (configurable threshold).
replan_needed(CampaignID, /task_failure_cascade) :-
    campaign_config(CampaignID, _, Threshold, /true, _),
    failed_campaign_task_count_computed(CampaignID, Count),
    Count >= Threshold,
    current_campaign(CampaignID).

# Trigger replan if user provides new instruction during campaign
replan_needed(CampaignID, /user_instruction) :-
    user_intent(/current_intent, /instruction, _, _, _),
    current_campaign(CampaignID).

# Trigger replan if explicit trigger exists
replan_needed(CampaignID, Reason) :-
    replan_trigger(CampaignID, Reason, _).

# Pause and replan action
next_action(/pause_and_replan) :-
    replan_needed(_, _).

# Helper: identify failed tasks (for counting in Go runtime)
failed_campaign_task(CampaignID, TaskID) :-
    current_campaign(CampaignID),
    campaign_phase(PhaseID, CampaignID, PhaseName, Seq, Status, Profile),
    campaign_task(TaskID, PhaseID, Desc, /failed, TaskType).

# --- Autopoiesis During Campaign ---

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
