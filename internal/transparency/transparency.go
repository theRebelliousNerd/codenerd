package transparency

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/types"
)

// TransparencyManager implements the operator-visibility surface that
// producers report into.
var _ types.TransparencyManager = (*TransparencyManager)(nil)

// maxOperationHistory bounds the post-operation summary ring.
const maxOperationHistory = 20

// TransparencyManager coordinates all transparency features.
// It provides a unified interface for enabling/disabling visibility
// into codeNERD's internal operations.
type TransparencyManager struct {
	mu sync.RWMutex

	config         *config.TransparencyConfig
	shardObserver  *ShardObserver
	safetyReporter *SafetyReporter
	enabled        bool

	// operations is the post-operation summary ring behind the
	// OperationSummaries flag. Before this existed the flag was printed in
	// GetStatus and read nowhere else.
	operations []*OperationSummary
}

// NewTransparencyManager creates a new transparency manager.
func NewTransparencyManager(cfg *config.TransparencyConfig) *TransparencyManager {
	if cfg == nil {
		cfg = &config.TransparencyConfig{
			Enabled:            false,
			ShardPhases:        true,
			SafetyExplanations: true,
			VerboseErrors:      true,
		}
	}

	tm := &TransparencyManager{
		config:         cfg,
		shardObserver:  NewShardObserver(),
		safetyReporter: NewSafetyReporter(),
		enabled:        cfg.Enabled,
	}

	// Configure sub-components based on config
	if cfg.Enabled && cfg.ShardPhases {
		tm.shardObserver.Enable()
	}

	// Become the process-wide reporter for deny sites and the JIT compiler,
	// which have no reference to this instance. See process.go.
	adoptProcessManager(tm)

	return tm
}

// Enable enables all transparency features.
func (tm *TransparencyManager) Enable() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	tm.enabled = true
	if tm.config.ShardPhases {
		tm.shardObserver.Enable()
	}
	tm.safetyReporter.Enable()
}

// Disable disables all transparency features.
func (tm *TransparencyManager) Disable() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	tm.enabled = false
	tm.shardObserver.Disable()
	tm.safetyReporter.Disable()
}

// Toggle toggles the enabled state and returns the new state.
func (tm *TransparencyManager) Toggle() bool {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	tm.enabled = !tm.enabled
	if tm.enabled {
		if tm.config.ShardPhases {
			tm.shardObserver.Enable()
		}
		tm.safetyReporter.Enable()
	} else {
		tm.shardObserver.Disable()
		tm.safetyReporter.Disable()
	}
	return tm.enabled
}

// IsEnabled returns whether transparency is enabled.
// Nil-safe: producers hold this as an interface that may be unset.
func (tm *TransparencyManager) IsEnabled() bool {
	if tm == nil {
		return false
	}
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.enabled
}

// shardPhasesEnabled reports whether phase tracking is configured.
func (tm *TransparencyManager) shardPhasesEnabled() bool {
	if tm == nil {
		return false
	}
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.config != nil && tm.config.ShardPhases
}

// GetConfig returns the current transparency configuration.
func (tm *TransparencyManager) GetConfig() *config.TransparencyConfig {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.config
}

// ShardObserver returns the shard phase observer.
func (tm *TransparencyManager) ShardObserver() *ShardObserver {
	return tm.shardObserver
}

// SafetyReporter returns the safety reporter.
func (tm *TransparencyManager) SafetyReporter() *SafetyReporter {
	return tm.safetyReporter
}

// StartShard begins tracking a shard execution.
//
// Tracking is gated on ShardPhases only, NOT on the master toggle. Transparency
// defaults to off, so gating the feed on `enabled` meant that by the time an
// operator typed `/transparency on` mid-run, every in-flight shard was invisible
// and "Active Operations" rendered empty — the exact split-brain this wiring is
// meant to close. The ShardObserver already draws the same line internally:
// StartExecution always tracks, and only notifications and phase history are
// gated on enabled. The tracked set is bounded (see pruneTerminalLocked).
func (tm *TransparencyManager) StartShard(shardID, shardType, task string) {
	if tm.shardPhasesEnabled() {
		tm.shardObserver.StartExecution(shardID, shardType, task)
	}
}

// UpdateShardPhase updates the phase of a shard execution.
func (tm *TransparencyManager) UpdateShardPhase(shardID string, phase ShardPhase, message string) {
	if tm.shardPhasesEnabled() {
		tm.shardObserver.UpdatePhase(shardID, phase, message)
	}
}

// EndShard marks a shard execution as complete.
func (tm *TransparencyManager) EndShard(shardID string, failed bool) {
	if tm.shardPhasesEnabled() {
		tm.shardObserver.EndExecution(shardID, failed)
	}
}

// ReportSafetyViolation records a safety gate block.
//
// Unlike the shard feed this stays gated on the master toggle plus
// SafetyExplanations: the violation history is an operator-facing narrative,
// and the durable record of every verdict is the audit log written at the deny
// site itself (logging.Audit().SafetyCheck).
func (tm *TransparencyManager) ReportSafetyViolation(action, target, rule string) *SafetyViolation {
	if tm == nil {
		return nil
	}
	tm.mu.RLock()
	on := tm.enabled && tm.config != nil && tm.config.SafetyExplanations
	tm.mu.RUnlock()
	if on {
		return tm.safetyReporter.ReportViolation(action, target, rule)
	}
	return nil
}

// RecordOperation files a post-operation summary, honoring the
// OperationSummaries flag. Returns the formatted summary (empty when the flag
// is off) so a caller that wants to display it does not have to reformat.
func (tm *TransparencyManager) RecordOperation(rec types.OperationRecord) {
	tm.recordOperation(rec)
}

// recordOperation is the value-returning form of RecordOperation. The
// interface method returns nothing so that internal/types stays free of
// transparency types.
func (tm *TransparencyManager) recordOperation(rec types.OperationRecord) *OperationSummary {
	if tm == nil {
		return nil
	}
	tm.mu.Lock()
	if tm.config == nil || !tm.config.OperationSummaries {
		tm.mu.Unlock()
		return nil
	}

	summary := &OperationSummary{
		Operation:     rec.Operation,
		Outcome:       rec.Outcome,
		Details:       rec.Details,
		Source:        rec.Source,
		FilesAffected: rec.FilesAffected,
		RulesApplied:  rec.RulesApplied,
		NextSteps:     rec.NextSteps,
		CompletedAt:   time.Now(),
	}
	if rec.Duration > 0 {
		summary.Duration = rec.Duration.Round(time.Millisecond).String()
	}

	tm.operations = append(tm.operations, summary)
	if len(tm.operations) > maxOperationHistory {
		tm.operations = tm.operations[1:]
	}
	tm.mu.Unlock()

	return summary
}

// RecentOperations returns the most recent post-operation summaries,
// newest last. Returns nil when OperationSummaries is off.
func (tm *TransparencyManager) RecentOperations(limit int) []*OperationSummary {
	if tm == nil {
		return nil
	}
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	if limit <= 0 || limit > len(tm.operations) {
		limit = len(tm.operations)
	}
	out := make([]*OperationSummary, limit)
	copy(out, tm.operations[len(tm.operations)-limit:])
	return out
}

// FormatLastOperation renders the most recent operation summary with
// FormatOperationSummary, or "" when there is none.
func (tm *TransparencyManager) FormatLastOperation() string {
	recent := tm.RecentOperations(1)
	if len(recent) == 0 {
		return ""
	}
	return FormatOperationSummary(recent[0])
}

// GetStatus returns a summary of the current transparency state.
func (tm *TransparencyManager) GetStatus() string {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	var sb strings.Builder
	sb.WriteString("## Transparency Status\n\n")

	status := "Disabled"
	if tm.enabled {
		status = "Enabled"
	}
	sb.WriteString(fmt.Sprintf("**Status**: %s\n\n", status))

	// The Notes column exists because this table used to advertise three flags
	// (StreamReasoning, JITExplain, OperationSummaries) that nothing in the
	// process read. Two are now wired; the one that is not says so here rather
	// than reporting "Enabled" for a feature that does nothing.
	sb.WriteString("### Feature Flags\n\n")
	sb.WriteString("| Feature | Status | Notes |\n|---------|--------|-------|\n")
	sb.WriteString(fmt.Sprintf("| Shard Phases | %s | fed by ShardManager lifecycle |\n", boolToStatus(tm.config.ShardPhases)))
	sb.WriteString(fmt.Sprintf("| Stream Reasoning | %s | %s |\n",
		boolToStatus(tm.config.StreamReasoning), experimentalNote))
	sb.WriteString(fmt.Sprintf("| Safety Explanations | %s | auto-reported on constitutional deny |\n", boolToStatus(tm.config.SafetyExplanations)))
	sb.WriteString(fmt.Sprintf("| JIT Explain | %s | emits [JIT] Glass Box events |\n", boolToStatus(tm.config.JITExplain)))
	sb.WriteString(fmt.Sprintf("| Operation Summaries | %s | recent operations below |\n", boolToStatus(tm.config.OperationSummaries)))
	sb.WriteString(fmt.Sprintf("| Verbose Errors | %s | classified errors + remediation |\n", boolToStatus(tm.config.VerboseErrors)))

	// Show active shards if any
	active := tm.shardObserver.GetActiveExecutions()
	if len(active) > 0 {
		sb.WriteString("\n### Active Operations\n\n")
		for _, exec := range active {
			sb.WriteString(fmt.Sprintf("- %s\n", exec.StatusLine()))
		}
	}

	// Show recent operation summaries if the flag produced any
	if len(tm.operations) > 0 {
		sb.WriteString("\n### Recent Operations\n\n")
		start := len(tm.operations) - 5
		if start < 0 {
			start = 0
		}
		for _, op := range tm.operations[start:] {
			sb.WriteString(fmt.Sprintf("- %s\n", op.StatusLine()))
		}
	}

	// Show recent violations if any
	violations := tm.safetyReporter.GetRecentViolations(5)
	if len(violations) > 0 {
		sb.WriteString("\n### Recent Safety Blocks\n\n")
		for _, v := range violations {
			sb.WriteString(fmt.Sprintf("- [%s] %s: %s\n",
				v.Timestamp.Format("15:04:05"),
				v.ViolationType.String(),
				v.Summary))
		}
	}

	return sb.String()
}

// FormatError formats an error with transparency context if enabled.
func (tm *TransparencyManager) FormatError(err error) string {
	if err == nil {
		return ""
	}

	classified := ClassifyError(err)
	if tm.IsEnabled() && tm.config.VerboseErrors {
		return classified.Format()
	}

	// Simpler format when transparency is off
	return fmt.Sprintf("%s %s\n\nDetails: %s",
		classified.Category.Prefix(),
		classified.Summary,
		err.Error())
}

// experimentalNote labels a config flag that this process does not act on.
// StreamReasoning would have to be honored inside the LLM streaming path
// (cmd/nerd/chat + internal/session), which reads no transparency config today.
const experimentalNote = "**experimental** — flag is not read by any producer"

// boolToStatus returns a status string for a boolean.
func boolToStatus(b bool) string {
	if b {
		return "Enabled"
	}
	return "Disabled"
}
