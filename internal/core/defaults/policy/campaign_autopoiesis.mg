# Campaign Autopoiesis Logic
# Extracted from campaign.mg
# Stratification: Depends on campaign_phases.mg
#
# The only copy of these rules: campaign_planning.mg carried a second one until
# 2026-09-23.

# =============================================================================
# Autopoiesis During Campaign
# =============================================================================

# Track successful phase types for learning (Go runtime extracts from kernel)
phase_success_pattern(PhaseType) :-
    campaign_phase(PhaseID, CampaignID, PhaseName, Seq, /completed, Profile),
    phase_objective(PhaseID, PhaseType, Desc, Priority),
    phase_checkpoint(PhaseID, CheckpointID, /true, ValidatedAt, ValidatorShard).

# Learn from phase completion - promotes success pattern for phase type
promote_to_long_term(/phase_success, PhaseType) :-
    phase_success_pattern(PhaseType).

# Learn from task failures for future avoidance: this campaign's failed tasks,
# not a failed task of any campaign the kernel holds.
campaign_learning(CampaignID, /failure_pattern, TaskType, ErrorMsg, Now) :-
    current_campaign(CampaignID),
    campaign_phase(PhaseID, CampaignID, _, _, _, _),
    campaign_task(TaskID, PhaseID, _, /failed, TaskType),
    task_error(TaskID, _, ErrorMsg),
    current_time(Now).
