package campaign

import "slices"

// OrchestratorEvent.Type used to be a free string written at ~35 call sites and
// switched on by name in three UIs (cmd/nerd/cmd_campaign.go, the chat model,
// cmd/nerd/ui/campaign_page.go). A typo produced an event that every consumer
// silently dropped through its default branch, which is invisible: the campaign
// still runs, the operator just never sees the step.
//
// OrchestratorEventType is a defined type, not an alias for string, and that is
// the load-bearing part.
//
// TestOrchestratorEventTypes_AreClosedSet scans emitEvent call sites for string
// literals and checks each against the constants below. That guard has a hole
// while the parameter is a plain string: an untyped string constant compiles
// fine, so
//
//	const zzPhantom = "zz_phantom_event"
//	o.emitEvent(zzPhantom, ...)
//
// passes the scan and produces exactly the defect the guard exists to catch —
// an event type every consumer drops through its default branch, invisible
// because the campaign still runs and the operator just never sees the step. A
// defined type moves that check from a regexp to the compiler, which cannot be
// routed around by naming the literal.
//
// The constants still close the set for the AST scan, which remains useful for
// the reverse direction: a new event type must be named here, and therefore
// considered by the UIs, before it can ship.
type OrchestratorEventType string

const (
	// Task lifecycle.
	EventTaskStarted   OrchestratorEventType = "task_started"
	EventTaskCompleted OrchestratorEventType = "task_completed"
	EventTaskFailed    OrchestratorEventType = "task_failed"

	// Phase lifecycle.
	EventPhaseStarted   OrchestratorEventType = "phase_started"
	EventPhaseCompleted OrchestratorEventType = "phase_completed"

	// Campaign lifecycle.
	EventCampaignCompleted OrchestratorEventType = "campaign_completed"
	EventCampaignBlocked   OrchestratorEventType = "campaign_blocked"

	// Verification.
	EventCheckpointFailed    OrchestratorEventType = "checkpoint_failed"
	EventCheckpointExhausted OrchestratorEventType = "checkpoint_exhausted"

	// Planning.
	EventReplan                 OrchestratorEventType = "replan"
	EventReplanTriggered        OrchestratorEventType = "replan_triggered"
	EventReplanFailed           OrchestratorEventType = "replan_failed"
	EventNewRequirementReceived OrchestratorEventType = "new_requirement_received"
	EventNewRequirementDone     OrchestratorEventType = "new_requirement_integrated"
	EventNewRequirementFailed   OrchestratorEventType = "new_requirement_failed"

	// Context paging.
	EventContextError     OrchestratorEventType = "context_error"
	EventCompressionError OrchestratorEventType = "compression_error"

	// Scheduling and durability.
	EventTaskLockTimeout     OrchestratorEventType = "task_lock_timeout"
	EventTaskWriteSetMissing OrchestratorEventType = "task_write_set_missing"
	EventArtifactPersisted   OrchestratorEventType = "artifact_persisted"

	// Diagnostics and self-repair.
	EventDiagnosticTaskInserted  OrchestratorEventType = "diagnostic_task_inserted"
	EventLogicFailureEscalated   OrchestratorEventType = "logic_failure_escalated"
	EventGenerationDegraded      OrchestratorEventType = "generation_degraded"
	EventResearchEmpty           OrchestratorEventType = "research_empty"
	EventShardResultEmpty        OrchestratorEventType = "shard_result_empty"
	EventToolGenerationRequested OrchestratorEventType = "tool_generation_requested"
	EventSubCampaignReferenced   OrchestratorEventType = "sub_campaign_referenced"

	// Risk preflight audit trail. These carry the RiskGateEvaluation /
	// RiskFinding in Data so a UI can render the gate report, not just a string.
	EventRiskSnapshotPinned    OrchestratorEventType = "risk_snapshot_pinned"
	EventRiskScoreComputed     OrchestratorEventType = "risk_score_computed"
	EventRiskGateResult        OrchestratorEventType = "risk_gate_result"
	EventRiskGateSkipped       OrchestratorEventType = "risk_gate_skipped"
	EventRiskGatePassed        OrchestratorEventType = "risk_gate_passed"
	EventRiskGateAdvisory      OrchestratorEventType = "risk_gate_advisory"
	EventRiskGateBlocked       OrchestratorEventType = "risk_gate_blocked"
	EventRiskIntelligenceError OrchestratorEventType = "risk_intelligence_error"
)

// orchestratorEventTypes is the closed set. Keep it sorted.
var orchestratorEventTypes = []OrchestratorEventType{
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
func OrchestratorEventTypes() []OrchestratorEventType {
	out := make([]OrchestratorEventType, len(orchestratorEventTypes))
	copy(out, orchestratorEventTypes)
	return out
}

// IsKnownOrchestratorEventType reports whether t is in the closed set.
func IsKnownOrchestratorEventType(t OrchestratorEventType) bool {
	i, found := slices.BinarySearch(orchestratorEventTypes, t)
	return found && i < len(orchestratorEventTypes)
}
