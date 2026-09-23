# Campaign Phases Logic
# Phase eligibility and sequencing, checkpoints, replanning triggers.
# Stratification: Depends on schemas_campaign.mg
#
# One definition per head. Until 2026-09-23 campaign_planning.mg carried a
# second copy of most of this file (and this file one of campaign_core.mg's
# state machine), the copies differed -- unscoped here, campaign-scoped there
# -- and what derived was their union (sweep finding F3). The campaign-scoped
# forms are kept; the state machine and the helpers campaign_core.mg reads
# live there only.

# =============================================================================
# Phase Eligibility & Sequencing
# =============================================================================

# Helper: a hard dependency of the phase, in the same campaign, is not
# completed. A phase closed /unverified is not completed: its dependents wait.
has_incomplete_hard_dep(PhaseID) :-
    phase_dependency(PhaseID, DepPhaseID, /hard),
    campaign_phase(PhaseID, CampaignID, _, _, _, _),
    campaign_phase(DepPhaseID, CampaignID, _, _, Status, _),
    /completed != Status.

# Helper: incomplete hard dependency scoped to campaign
has_incomplete_hard_dep_in_campaign(PhaseID, CampaignID) :-
    has_incomplete_hard_dep(PhaseID),
    campaign_phase(PhaseID, CampaignID, _, _, _, _).

# Helper: phase eligibility scoped to campaign
phase_eligible_in_campaign(PhaseID, CampaignID) :-
    campaign_phase(PhaseID, CampaignID, _, _, /pending, _),
    !has_incomplete_hard_dep_in_campaign(PhaseID, CampaignID),
    current_campaign(CampaignID).

# A phase is eligible when all hard dependencies are complete
phase_eligible(PhaseID) :-
    phase_eligible_in_campaign(PhaseID, _).

# Helper: an eligible phase of the same campaign comes earlier
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
    current_campaign(CampaignID),
    has_incomplete_hard_dep(PhaseID).

# =============================================================================
# Checkpoint & Verification
# =============================================================================

# Helper: a verification method the phase's objectives name has not passed.
has_pending_checkpoint(PhaseID) :-
    phase_objective(PhaseID, _, _, VerifyMethod),
    /none != VerifyMethod,
    !has_passed_checkpoint(PhaseID, VerifyMethod).

has_passed_checkpoint(PhaseID, CheckType) :-
    phase_checkpoint(PhaseID, CheckType, /true, _, _).

# Phase has pending tasks currently waiting in retry/backoff window
phase_waiting_for_retry(PhaseID) :-
    current_phase(PhaseID),
    phase_has_backoff_task(PhaseID).

# Block phase completion if checkpoint failed
phase_blocked(PhaseID, /checkpoint_failed) :-
    phase_checkpoint(PhaseID, _, /false, _, _).

# A phase whose checkpoint never passed within its attempts closes /unverified
# (orchestrator closePhaseUnverified): its tasks ran, its verification did not.
# It is not /completed, so every hard dependent stays blocked
# (has_incomplete_hard_dep), and when nothing else can run the campaign is
# blocked on it by name rather than on "no eligible phases". It used to close
# /completed and unlock the phases built on it (external audit N03).
has_unverified_phase(CampaignID) :-
    campaign_phase(_, CampaignID, _, _, /unverified, _).

campaign_blocked(CampaignID, /phase_unverified) :-
    current_campaign(CampaignID),
    has_unverified_phase(CampaignID),
    !has_eligible_phase(),
    !has_in_progress_phase().

# =============================================================================
# Replanning Triggers
# =============================================================================

# Helper: identify failed tasks
failed_campaign_task(CampaignID, TaskID) :-
    current_campaign(CampaignID),
    campaign_phase(PhaseID, CampaignID, PhaseName, Seq, Status, Profile),
    campaign_task(TaskID, PhaseID, Desc, /failed, TaskType).

# Trigger replan if user provides new instruction during campaign
replan_needed(CampaignID, /user_instruction) :-
    current_campaign(CampaignID),
    user_intent(/current_intent, /instruction, _, _, _).

# Trigger replan if explicit trigger exists
replan_needed(CampaignID, Reason) :-
    replan_trigger(CampaignID, Reason, _).
