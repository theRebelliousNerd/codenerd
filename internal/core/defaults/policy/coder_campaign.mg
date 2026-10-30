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

# Current phase objectives affect coder strategy
campaign_coder_focus(Objective) :-
    current_phase(PhaseID),
    phase_objective(PhaseID, _, Objective, _).

# -----------------------------------------------------------------------------
# 12.2 Campaign Quality Requirements
# -----------------------------------------------------------------------------

# Campaign phase requires tests
campaign_requires_tests() :-
    current_phase(PhaseID),
    phase_checkpoint(PhaseID, /tests, _, _, _).

# Campaign phase requires build pass
campaign_requires_build() :-
    current_phase(PhaseID),
    phase_checkpoint(PhaseID, /build, _, _, _).

# What the coder is told while a campaign runs: the phase's objective, and the
# checks its checkpoints will run. These rows join the campaign's facts, so they
# are derived in the campaign shard; the prompt compiler reads injectable_context
# across shards.
injectable_context("coder", Line) :-
    campaign_coder_focus(Objective),
    Line = fn:string:concat("This work serves the campaign phase objective: ", Objective).

injectable_context("coder", "The campaign phase's checkpoint runs the tests: leave them passing.") :-
    campaign_requires_tests().

injectable_context("coder", "The campaign phase's checkpoint runs the build: leave it passing.") :-
    campaign_requires_build().

# REMOVED: coder_quality_mode (/strict in a campaign, /normal otherwise). Nothing
# read it, and its /normal arm negated a campaign fact with no anchor.

# -----------------------------------------------------------------------------
# 12.3 Campaign Progress Reporting
# -----------------------------------------------------------------------------

