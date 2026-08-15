package campaign

import "sort"

// OrchestratorEvent.Type used to be a free string written at ~35 call sites and
// switched on by name in three UIs (cmd/nerd/cmd_campaign.go, the chat model,
// cmd/nerd/ui/campaign_page.go). A typo produced an event that every consumer
// silently dropped through its default branch, which is invisible: the campaign
// still runs, the operator just never sees the step.
//
// These constants close the set. TestOrchestratorEventTypes_AreClosedSet keeps
// the constants and the emit sites in step, so a new event type has to be named
// here — and therefore has to be considered by the UIs — before it can ship.
const (
	// Task lifecycle.
	EventTaskStarted   = "task_started"
	EventTaskCompleted = "task_completed"
	EventTaskFailed    = "task_failed"

	// Phase lifecycle.
	EventPhaseStarted   = "phase_started"
	EventPhaseCompleted = "phase_completed"

	// Campaign lifecycle.
	EventCampaignCompleted = "campaign_completed"
	EventCampaignBlocked   = "campaign_blocked"

	// Verification.
	EventCheckpointFailed    = "checkpoint_failed"
	EventCheckpointExhausted = "checkpoint_exhausted"

	// Planning.
	EventReplan                 = "replan"
	EventReplanTriggered        = "replan_triggered"
	EventReplanFailed           = "replan_failed"
	EventNewRequirementReceived = "new_requirement_received"
	EventNewRequirementDone     = "new_requirement_integrated"
	EventNewRequirementFailed   = "new_requirement_failed"

	// Context paging.
	EventContextError     = "context_error"
	EventCompressionError = "compression_error"

	// Scheduling and durability.
	EventTaskLockTimeout     = "task_lock_timeout"
	EventTaskWriteSetMissing = "task_write_set_missing"
	EventArtifactPersisted   = "artifact_persisted"

	// Diagnostics and self-repair.
	EventDiagnosticTaskInserted  = "diagnostic_task_inserted"
	EventLogicFailureEscalated   = "logic_failure_escalated"
	EventGenerationDegraded      = "generation_degraded"
	EventResearchEmpty           = "research_empty"
	EventShardResultEmpty        = "shard_result_empty"
	EventToolGenerationRequested = "tool_generation_requested"
	EventSubCampaignReferenced   = "sub_campaign_referenced"

	// Risk preflight audit trail. These carry the RiskGateEvaluation /
	// RiskFinding in Data so a UI can render the gate report, not just a string.
	EventRiskSnapshotPinned    = "risk_snapshot_pinned"
	EventRiskScoreComputed     = "risk_score_computed"
	EventRiskGateResult        = "risk_gate_result"
	EventRiskGateSkipped       = "risk_gate_skipped"
	EventRiskGatePassed        = "risk_gate_passed"
	EventRiskGateAdvisory      = "risk_gate_advisory"
	EventRiskGateBlocked       = "risk_gate_blocked"
	EventRiskIntelligenceError = "risk_intelligence_error"
)

// orchestratorEventTypes is the closed set. Keep it sorted.
var orchestratorEventTypes = []string{
	EventArtifactPersisted,
	EventCampaignBlocked,
	EventCampaignCompleted,
	EventCheckpointExhausted,
	EventCheckpointFailed,
	EventCompressionError,
	EventContextError,
	EventDiagnosticTaskInserted,
	EventGenerationDegraded,
	EventLogicFailureEscalated,
	EventNewRequirementFailed,
	EventNewRequirementDone,
	EventNewRequirementReceived,
	EventPhaseCompleted,
	EventPhaseStarted,
	EventReplan,
	EventReplanFailed,
	EventReplanTriggered,
	EventResearchEmpty,
	EventRiskGateAdvisory,
	EventRiskGateBlocked,
	EventRiskGatePassed,
	EventRiskGateResult,
	EventRiskGateSkipped,
	EventRiskIntelligenceError,
	EventRiskScoreComputed,
	EventRiskSnapshotPinned,
	EventShardResultEmpty,
	EventSubCampaignReferenced,
	EventTaskCompleted,
	EventTaskFailed,
	EventTaskLockTimeout,
	EventTaskStarted,
	EventTaskWriteSetMissing,
	EventToolGenerationRequested,
}

// OrchestratorEventTypes returns the closed set of event types an orchestrator
// can emit. UIs use it to assert they handle every event they can receive.
func OrchestratorEventTypes() []string {
	out := make([]string, len(orchestratorEventTypes))
	copy(out, orchestratorEventTypes)
	return out
}

// IsKnownOrchestratorEventType reports whether t is in the closed set.
func IsKnownOrchestratorEventType(t string) bool {
	i := sort.SearchStrings(orchestratorEventTypes, t)
	return i < len(orchestratorEventTypes) && orchestratorEventTypes[i] == t
}
