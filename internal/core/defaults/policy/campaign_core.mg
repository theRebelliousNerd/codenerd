# Campaign Core Logic
# State machine and basic helpers

# Current campaign is the one that's active
current_campaign(CampaignID) :-
    campaign(CampaignID, _, _, _, /active).

# Campaign execution strategy activates when a campaign is active
active_strategy(/campaign_execution) :-
    current_campaign(_).

# Campaign complete when all phases complete
campaign_complete(CampaignID) :-
    current_campaign(CampaignID),
    !has_incomplete_phase(CampaignID).

next_action(/campaign_complete) :-
    campaign_complete(_).

# Campaign Blocking Conditions

# Campaign blocked if no eligible phases and none in progress
campaign_blocked(CampaignID, /no_eligible_phases) :-
    current_campaign(CampaignID),
    !has_eligible_phase(),
    !has_in_progress_phase(),
    has_incomplete_phase(CampaignID).

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
