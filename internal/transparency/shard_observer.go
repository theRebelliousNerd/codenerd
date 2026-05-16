package transparency

import (
	"fmt"
	"sync"
	"time"
)

// ShardPhase represents the current execution phase of a shard.
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
	if int(p) < len(names) {
		return names[p]
	}
	return "Unknown"
}

// ShardExecution represents the state of a shard's execution.
type ShardExecution struct {
	ShardID   string
	ShardType string
	Task      string
	Phase     ShardPhase
	StartTime time.Time
	PhaseTime time.Time // When the current phase started
	Message   string    // Optional phase-specific message
	Progress  float64   // 0.0-1.0 progress within current phase (optional)
}

// Duration returns how long the shard has been executing.
func (e *ShardExecution) Duration() time.Duration {
	return time.Since(e.StartTime)
}

// PhaseDuration returns how long the current phase has been running.
func (e *ShardExecution) PhaseDuration() time.Duration {
	return time.Since(e.PhaseTime)
}

// StatusLine returns a one-line status suitable for display.
func (e *ShardExecution) StatusLine() string {
	duration := e.Duration().Round(100 * time.Millisecond)
	if e.Message != "" {
		return fmt.Sprintf("[%s] %s: %s (%.1fs)", e.ShardType, e.Phase.String(), e.Message, duration.Seconds())
	}
	return fmt.Sprintf("[%s] %s (%.1fs)", e.ShardType, e.Phase.String(), duration.Seconds())
}

// PhaseUpdate represents a notification about phase changes.
type PhaseUpdate struct {
	ShardID   string
	OldPhase  ShardPhase
	NewPhase  ShardPhase
	Message   string
	Timestamp time.Time
}

// PhaseObserver observes shard execution phases.
type PhaseObserver interface {
	OnPhaseChange(update PhaseUpdate)
}

// ShardObserver tracks shard execution phases and notifies observers.
type ShardObserver struct {
	mu           sync.RWMutex
	executions   map[string]*ShardExecution
	observers    []PhaseObserver
	enabled      bool
	phaseHistory []PhaseUpdate
	maxHistory   int
}

// NewShardObserver creates a new shard observer.
func NewShardObserver() *ShardObserver {
	return &ShardObserver{
		executions: make(map[string]*ShardExecution),
		enabled:    false, // Disabled by default
		maxHistory: 100,
	}
}

// Enable enables phase observation.
func (o *ShardObserver) Enable() {
	o.mu.Lock()
	o.enabled = true
	o.mu.Unlock()
}

// Disable disables phase observation.
func (o *ShardObserver) Disable() {
	o.mu.Lock()
	o.enabled = false
	o.mu.Unlock()
}

// IsEnabled returns whether observation is enabled.
func (o *ShardObserver) IsEnabled() bool {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.enabled
}

// AddObserver registers an observer for phase updates.
func (o *ShardObserver) AddObserver(obs PhaseObserver) {
	o.mu.Lock()
	o.observers = append(o.observers, obs)
	o.mu.Unlock()
}

// StartExecution begins tracking a shard execution.
func (o *ShardObserver) StartExecution(shardID, shardType, task string) {
	o.mu.Lock()
	defer o.mu.Unlock()

	now := time.Now()
	o.executions[shardID] = &ShardExecution{
		ShardID:   shardID,
		ShardType: shardType,
		Task:      task,
		Phase:     PhaseInitializing,
		StartTime: now,
		PhaseTime: now,
	}

	if o.enabled {
		o.notifyObservers(PhaseUpdate{
			ShardID:   shardID,
			OldPhase:  PhaseIdle,
			NewPhase:  PhaseInitializing,
			Timestamp: now,
		})
	}
}

// UpdatePhase updates the current phase of a shard execution.
func (o *ShardObserver) UpdatePhase(shardID string, phase ShardPhase, message string) {
	o.mu.Lock()
	defer o.mu.Unlock()

	exec, ok := o.executions[shardID]
	if !ok {
		return
	}

	oldPhase := exec.Phase
	now := time.Now()

	exec.Phase = phase
	exec.PhaseTime = now
	exec.Message = message

	if o.enabled {
		update := PhaseUpdate{
			ShardID:   shardID,
			OldPhase:  oldPhase,
			NewPhase:  phase,
			Message:   message,
			Timestamp: now,
		}
		o.phaseHistory = append(o.phaseHistory, update)
		if len(o.phaseHistory) > o.maxHistory {
			o.phaseHistory = o.phaseHistory[1:]
		}
		o.notifyObservers(update)
	}
}

// SetProgress updates the progress within the current phase.
func (o *ShardObserver) SetProgress(shardID string, progress float64) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if exec, ok := o.executions[shardID]; ok {
		exec.Progress = progress
	}
}

// EndExecution marks a shard execution as complete.
func (o *ShardObserver) EndExecution(shardID string, failed bool) {
	o.mu.Lock()
	defer o.mu.Unlock()

	exec, ok := o.executions[shardID]
	if !ok {
		return
	}

	oldPhase := exec.Phase
	newPhase := PhaseComplete
	if failed {
		newPhase = PhaseFailed
	}

	now := time.Now()
	exec.Phase = newPhase
	exec.PhaseTime = now

	if o.enabled {
		update := PhaseUpdate{
			ShardID:   shardID,
			OldPhase:  oldPhase,
			NewPhase:  newPhase,
			Timestamp: now,
		}
		o.phaseHistory = append(o.phaseHistory, update)
		o.notifyObservers(update)
	}
}

// GetExecution returns the current execution state for a shard.
func (o *ShardObserver) GetExecution(shardID string) *ShardExecution {
	o.mu.RLock()
	defer o.mu.RUnlock()

	if exec, ok := o.executions[shardID]; ok {
		// Return a copy
		copy := *exec
		return &copy
	}
	return nil
}

// GetActiveExecutions returns all currently active executions.
func (o *ShardObserver) GetActiveExecutions() []*ShardExecution {
	o.mu.RLock()
	defer o.mu.RUnlock()

	var active []*ShardExecution
	for _, exec := range o.executions {
		if exec.Phase != PhaseIdle && exec.Phase != PhaseComplete && exec.Phase != PhaseFailed {
			copy := *exec
			active = append(active, &copy)
		}
	}
	return active
}

// GetPhaseHistory returns recent phase changes.
func (o *ShardObserver) GetPhaseHistory(limit int) []PhaseUpdate {
	o.mu.RLock()
	defer o.mu.RUnlock()

	if limit <= 0 || limit > len(o.phaseHistory) {
		limit = len(o.phaseHistory)
	}

	// Return most recent
	start := len(o.phaseHistory) - limit
	result := make([]PhaseUpdate, limit)
	copy(result, o.phaseHistory[start:])
	return result
}

// ClearHistory clears the phase history.
func (o *ShardObserver) ClearHistory() {
	o.mu.Lock()
	o.phaseHistory = nil
	o.mu.Unlock()
}

// notifyObservers sends updates to all registered observers.
func (o *ShardObserver) notifyObservers(update PhaseUpdate) {
	for _, obs := range o.observers {
		obs.OnPhaseChange(update)
	}
}

// FormatExecutionSummary returns a formatted summary of all active executions.
func (o *ShardObserver) FormatExecutionSummary() string {
	active := o.GetActiveExecutions()
	if len(active) == 0 {
		return ""
	}

	var lines []string
	for _, exec := range active {
		lines = append(lines, exec.StatusLine())
	}

	result := ""
	for i, line := range lines {
		if i > 0 {
			result += "\n"
		}
		result += line
	}
	return result
}
