# Campaign Core Logic
# State machine and basic helpers

# Current campaign is the one that's active
current_campaign(CampaignID) :-
    campaign(CampaignID, _, _, _, /active).

# Campaign execution strategy activates when a campaign is active
active_strategy(/campaign_execution) :-
    current_campaign(_).

# Campaign complete when all phases complete and, where the user declared an
# acceptance command, that command has passed. This is the only definition of
# campaign_complete: a second copy without the acceptance premise would be a
# way round it.
campaign_complete(CampaignID) :-
    current_campaign(CampaignID),
    !has_incomplete_phase(CampaignID),
    !campaign_acceptance_unmet(CampaignID).

next_action(/campaign_complete) :-
    campaign_complete(_).

# --- Acceptance: the campaign's deterministic witness ---

# Failed acceptance rounds a campaign may remediate before it is blocked.
campaign_acceptance_limit(3).

campaign_accepted(CampaignID) :-
    campaign_acceptance_result(CampaignID, _, /pass).

campaign_acceptance_unmet(CampaignID) :-
    campaign_acceptance(CampaignID, _),
    !campaign_accepted(CampaignID).

campaign_acceptance_exhausted(CampaignID) :-
    campaign_acceptance_unmet(CampaignID),
    campaign_acceptance_result(CampaignID, Round, /fail),
    campaign_acceptance_limit(Limit),
    Round >= Limit.

# Every phase is done and the witness has not passed: run it.
campaign_acceptance_due(CampaignID) :-
    current_campaign(CampaignID),
    !has_incomplete_phase(CampaignID),
    campaign_acceptance_unmet(CampaignID),
    !campaign_acceptance_exhausted(CampaignID).

campaign_blocked(CampaignID, /acceptance_failed) :-
    current_campaign(CampaignID),
    campaign_acceptance_exhausted(CampaignID).

# Campaign Blocking Conditions

# Campaign blocked if no eligible phases and none in progress -- unless a phase
# closed /unverified explains it, which campaign_phases.mg names instead
# (/phase_unverified). This rule is duplicated in campaign_phases.mg; both
# copies carry the exclusion.
campaign_blocked(CampaignID, /no_eligible_phases) :-
    current_campaign(CampaignID),
    !has_eligible_phase(),
    !has_in_progress_phase(),
    has_incomplete_phase(CampaignID),
    !has_unverified_phase(CampaignID).


# Campaign blocked if all remaining tasks are blocked
campaign_blocked(CampaignID, /all_tasks_blocked) :-
    current_campaign(CampaignID),
    !has_next_campaign_task(),
    campaign_phase(PhaseID, CampaignID, _, _, _, _),
    current_phase(PhaseID),
    has_incomplete_phase_task(PhaseID).

# --- Helpers ---

# Helper: true if any phase is eligible to start
has_eligible_phase() :-
    phase_eligible_in_campaign(_, _).

# Helper: true if there's a next campaign task available
has_next_campaign_task() :-
    next_campaign_task(_).

# Helper: check if any phase is not complete
has_incomplete_phase(CampaignID) :-
    campaign_phase(_, CampaignID, _, _, Status, _),
    /completed != Status,
    /skipped != Status.

# Helper: check if any phase is in progress
has_in_progress_phase() :-
    campaign_phase(_, CampaignID, _, _, /in_progress, _),
    current_campaign(CampaignID).

# Helper: check if all phase tasks are complete
has_incomplete_phase_task(PhaseID) :-
    campaign_task(_, PhaseID, _, Status, _),
    /completed != Status,
    /skipped != Status.

all_phase_tasks_complete(PhaseID) :-
    campaign_phase(PhaseID, _, _, _, _, _),
    !has_incomplete_phase_task(PhaseID).
