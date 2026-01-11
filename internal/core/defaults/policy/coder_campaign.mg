# Coder Policy - Campaign Integration
# Version: 1.0.0
# Extracted from coder.mg Section 12

# =============================================================================
# SECTION 12: CAMPAIGN INTEGRATION
# =============================================================================
# Rules for coder behavior during campaigns.

# -----------------------------------------------------------------------------
# 12.1 Campaign Context Awareness
# -----------------------------------------------------------------------------

# Coder is operating within campaign
in_campaign_context() :-
    current_campaign(_).

# Current phase objectives affect coder strategy
campaign_coder_focus(Objective) :-
    in_campaign_context(),
    current_phase(PhaseID),
    phase_objective(PhaseID, _, Objective, _).

# -----------------------------------------------------------------------------
# 12.2 Campaign Quality Requirements
# -----------------------------------------------------------------------------

# Campaign phase requires tests
campaign_requires_tests() :-
    in_campaign_context(),
    current_phase(PhaseID),
    phase_checkpoint(PhaseID, /tests, _, _, _).

# Campaign phase requires build pass
campaign_requires_build() :-
    in_campaign_context(),
    current_phase(PhaseID),
    phase_checkpoint(PhaseID, /build, _, _, _).

# Stricter quality during campaigns
coder_quality_mode(/strict) :-
    in_campaign_context().

coder_quality_mode(/normal) :-
    !in_campaign_context().

# -----------------------------------------------------------------------------
# 12.3 Campaign Progress Reporting
# -----------------------------------------------------------------------------

# Report coder completion to campaign
coder_task_completed(TaskID) :-
    coder_task(TaskID, _, _, _),
    coder_state(/build_passed).

coder_task_completed(TaskID) :-
    coder_task(TaskID, _, _, _),
    coder_state(/tests_passed).

# Report coder failure to campaign
coder_task_failed(TaskID, Reason) :-
    coder_task(TaskID, _, _, _),
    coder_state(/build_failed),
    retry_count(N),
    N >= 3,
    Reason = "max_retries_exceeded".
