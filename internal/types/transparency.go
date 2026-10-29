package types

import "time"

// ShardPhase represents the current execution phase of a shard.
//
// The canonical definition lives here rather than in internal/transparency so
// that producers (ShardManager) can report phases through the
// TransparencyManager interface below without importing the transparency
// package. internal/transparency aliases this type, so
// transparency.PhaseExecuting and types.PhaseExecuting are the same constant.
type ShardPhase int

const (
	PhaseIdle ShardPhase = iota
	PhaseInitializing
	PhaseLoading
	PhaseAnalyzing
	PhaseGenerating
	PhaseExecuting
	PhaseComplete
	PhaseFailed
)

// String returns the display name for a phase.
func (p ShardPhase) String() string {
	names := []string{
		"Idle",
		"Initializing",
		"Loading context",
		"Analyzing",
		"Generating",
		"Executing",
		"Complete",
		"Failed",
	}
	if int(p) < len(names) && p >= 0 {
		return names[p]
	}
	return "Unknown"
}

// OperationRecord is a completed unit of work reported to the transparency
// layer for post-operation summaries. It carries only plain types so that
// producers do not need to import internal/transparency.
type OperationRecord struct {
	Operation     string        // What ran (e.g. "coder shard", "exec_cmd")
	Outcome       string        // "Success" / "Failed" / free text
	Duration      time.Duration // Wall time
	Details       string        // Optional body (error text, result excerpt)
	Source        string        // Producer identity (shard ID, action ID)
	FilesAffected []string      // Files read/modified, when known
	RulesApplied  []string      // Mangle rules that fired, when known
	NextSteps     []string      // Suggested follow-ups
}

// TransparencyManager is the narrow operator-visibility surface that long-lived
// subsystems report into. ShardManager previously stored the concrete manager
// as `any` with a "to be added later" comment, which meant the shard lifecycle
// could not call it at all: Active Operations in `/transparency` rendered from a
// ShardObserver that nothing ever fed.
//
// Implemented by *transparency.TransparencyManager. Every method must be safe
// on a nil implementation value and must not block the caller.
type TransparencyManager interface {
	// IsEnabled reports whether the operator has transparency switched on.
	IsEnabled() bool

	// StartShard begins tracking a shard execution.
	StartShard(shardID, shardType, task string)

	// UpdateShardPhase moves a tracked execution to a new phase.
	UpdateShardPhase(shardID string, phase ShardPhase, message string)

	// EndShard marks a tracked execution terminal.
	EndShard(shardID string, failed bool)

	// RecordOperation files a post-operation summary. Honors the
	// OperationSummaries config flag.
	RecordOperation(rec OperationRecord)
}
