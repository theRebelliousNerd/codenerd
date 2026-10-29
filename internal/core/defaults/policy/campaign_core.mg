# Campaign Core Logic
# State machine and basic helpers

# Current campaign is the one that's active
current_campaign(CampaignID) :-
    campaign(CampaignID, _, _, _, /active).

# Campaign execution strategy activates when a campaign is active
active_strategy(/campaign_execution) :-
    current_campaign(_).

# Every phase of the campaign is completed or skipped. The orchestrator asks
# this when no phase is current, and then settles the acceptance command; it
# used to decide it with an in-memory scan beside this rule (sweep finding F3).
campaign_phases_done(CampaignID) :-
    current_campaign(CampaignID),
    !has_incomplete_phase(CampaignID).

# Campaign complete when all phases complete and, where the user declared an
# acceptance command, that command has passed. This is the only definition of
# campaign_complete: a second copy without the acceptance premise would be a
# way round it.
campaign_complete(CampaignID) :-
    campaign_phases_done(CampaignID),
    !campaign_acceptance_unmet(CampaignID).

# --- Acceptance: the campaign's deterministic witness ---

# Failed acceptance rounds a campaign may remediate before it is blocked:
# campaign.acceptance_rounds in the user's config.
config_param_required(/campaign, /campaign_acceptance_rounds).

campaign_accepted(CampaignID) :-
    campaign_acceptance_result(CampaignID, _, /pass).

campaign_acceptance_unmet(CampaignID) :-
    campaign_acceptance(CampaignID, _),
    !campaign_accepted(CampaignID).

campaign_acceptance_exhausted(CampaignID) :-
    campaign_acceptance_unmet(CampaignID),
    campaign_acceptance_result(CampaignID, Round, /fail),
    config_param(/campaign_acceptance_rounds, Limit),
    Round >= Limit.

# Every phase is done and the witness has not passed: run it.
campaign_acceptance_due(CampaignID) :-
    campaign_phases_done(CampaignID),
    campaign_acceptance_unmet(CampaignID),
    !campaign_acceptance_exhausted(CampaignID).

campaign_blocked(CampaignID, /acceptance_failed) :-
    current_campaign(CampaignID),
    campaign_acceptance_exhausted(CampaignID).

# Campaign Blocking Conditions

# Campaign blocked if no eligible phases and none in progress -- unless a phase
# closed /unverified explains it, which campaign_phases.mg names instead
# (/phase_unverified).
campaign_blocked(CampaignID, /no_eligible_phases) :-
    current_campaign(CampaignID),
    !has_eligible_phase(),
    !has_in_progress_phase(),
    has_incomplete_phase(CampaignID),
    !has_unverified_phase(CampaignID).


# Campaign blocked if the current phase has incomplete tasks and none can run:
# no next task, and none waiting out a retry backoff (a backoff is a wait, not
# a block).
campaign_blocked(CampaignID, /all_tasks_blocked) :-
    current_campaign(CampaignID),
    campaign_phase(PhaseID, CampaignID, _, _, _, _),
    current_phase(PhaseID),
    !has_next_campaign_task(),
    !phase_has_backoff_task(PhaseID),
    has_incomplete_phase_task(PhaseID).

# --- Helpers ---

# Helper: true if any phase is eligible to start
has_eligible_phase() :-
    phase_eligible_in_campaign(_, _).

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
