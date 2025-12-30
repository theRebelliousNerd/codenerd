package transparency

import (
	"fmt"
	"strings"
	"sync"

	"codenerd/internal/config"
)

// TransparencyManager coordinates all transparency features.
// It provides a unified interface for enabling/disabling visibility
// into codeNERD's internal operations.
type TransparencyManager struct {
	mu sync.RWMutex

	config         *config.TransparencyConfig
	shardObserver  *ShardObserver
	safetyReporter *SafetyReporter
	enabled        bool
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
func (tm *TransparencyManager) IsEnabled() bool {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.enabled
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
func (tm *TransparencyManager) StartShard(shardID, shardType, task string) {
	if tm.IsEnabled() && tm.config.ShardPhases {
		tm.shardObserver.StartExecution(shardID, shardType, task)
	}
}

// UpdateShardPhase updates the phase of a shard execution.
func (tm *TransparencyManager) UpdateShardPhase(shardID string, phase ShardPhase, message string) {
	if tm.IsEnabled() && tm.config.ShardPhases {
		tm.shardObserver.UpdatePhase(shardID, phase, message)
	}
}

// EndShard marks a shard execution as complete.
func (tm *TransparencyManager) EndShard(shardID string, failed bool) {
	if tm.IsEnabled() && tm.config.ShardPhases {
		tm.shardObserver.EndExecution(shardID, failed)
	}
}

// ReportSafetyViolation records a safety gate block.
func (tm *TransparencyManager) ReportSafetyViolation(action, target, rule string) *SafetyViolation {
	if tm.IsEnabled() && tm.config.SafetyExplanations {
		return tm.safetyReporter.ReportViolation(action, target, rule)
	}
	return nil
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

	sb.WriteString("### Feature Flags\n\n")
	sb.WriteString("| Feature | Status |\n|---------|--------|\n")
	sb.WriteString(fmt.Sprintf("| Shard Phases | %s |\n", boolToStatus(tm.config.ShardPhases)))
	sb.WriteString(fmt.Sprintf("| Stream Reasoning | %s |\n", boolToStatus(tm.config.StreamReasoning)))
	sb.WriteString(fmt.Sprintf("| Safety Explanations | %s |\n", boolToStatus(tm.config.SafetyExplanations)))
	sb.WriteString(fmt.Sprintf("| JIT Explain | %s |\n", boolToStatus(tm.config.JITExplain)))
	sb.WriteString(fmt.Sprintf("| Operation Summaries | %s |\n", boolToStatus(tm.config.OperationSummaries)))
	sb.WriteString(fmt.Sprintf("| Verbose Errors | %s |\n", boolToStatus(tm.config.VerboseErrors)))

	// Show active shards if any
	active := tm.shardObserver.GetActiveExecutions()
	if len(active) > 0 {
		sb.WriteString("\n### Active Operations\n\n")
		for _, exec := range active {
			sb.WriteString(fmt.Sprintf("- %s\n", exec.StatusLine()))
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

// boolToStatus returns a status string for a boolean.
func boolToStatus(b bool) string {
	if b {
		return "Enabled"
	}
	return "Disabled"
}
