package init

import (
	"sync"
	"time"
)

// ETATracker calculates estimated time remaining based on historical phase durations.
type ETATracker struct {
	mu             sync.RWMutex
	startTime      time.Time
	phaseDurations map[string]time.Duration // Historical durations for each phase
	currentPhase   int
	totalPhases    int
	phaseStartTime time.Time
}

// DefaultPhaseDurations returns expected durations for each init phase.
// These are baseline estimates that get refined based on actual performance.
// E2: Updated to include all 22 phases for accurate ETA calculation.
func DefaultPhaseDurations() map[string]time.Duration {
	return map[string]time.Duration{
		"setup":           2 * time.Second,
		"migration":       3 * time.Second,
		"directory":       5 * time.Second,
		"scanning":        20 * time.Second,
		"analysis":        75 * time.Second, // 60-90s average
		"profile":         5 * time.Second,
		"facts":           10 * time.Second,
		"prompt_atoms":    3 * time.Second,
		"prompt_db":       5 * time.Second,
		"agents":          5 * time.Second,
		"shared_kb":       30 * time.Second,
		"kb_creation":     105 * time.Second, // 90-120s average
		"codebase_kb":     20 * time.Second,
		"core_shards_kb":  30 * time.Second,
		"campaign_kb":     15 * time.Second,
		"tool_generation": 10 * time.Second,
		"preferences":     4 * time.Second,
		"session":         2 * time.Second,
		"tools":           20 * time.Second,
		"registry":        5 * time.Second,
		"prompt_sync":     10 * time.Second,
		"complete":        1 * time.Second,
	}
}

// NewETATracker creates a new ETA tracker.
func NewETATracker(totalPhases int) *ETATracker {
	return &ETATracker{
		startTime:      time.Now(),
		phaseDurations: DefaultPhaseDurations(),
		totalPhases:    totalPhases,
		currentPhase:   0,
	}
}

// StartPhase marks the beginning of a new phase.
func (e *ETATracker) StartPhase(phaseNum int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.currentPhase = phaseNum
	e.phaseStartTime = time.Now()
}

// CompletePhase records the actual duration of a completed phase.
func (e *ETATracker) CompletePhase(phaseName string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	actualDuration := time.Since(e.phaseStartTime)
	// Update with actual duration for better future estimates
	e.phaseDurations[phaseName] = actualDuration
}

// GetETARemaining calculates the estimated time remaining.
func (e *ETATracker) GetETARemaining(remainingPhases []string) time.Duration {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var remaining time.Duration
	for _, phase := range remainingPhases {
		if dur, ok := e.phaseDurations[phase]; ok {
			remaining += dur
		} else {
			// Default estimate for unknown phases
			remaining += 10 * time.Second
		}
	}
	return remaining
}

// GetElapsed returns the time elapsed since init started.
func (e *ETATracker) GetElapsed() time.Duration {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return time.Since(e.startTime)
}

// GetCurrentPhase returns the current phase number.
func (e *ETATracker) GetCurrentPhase() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.currentPhase
}

// GetTotalPhases returns the total number of phases.
func (e *ETATracker) GetTotalPhases() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.totalPhases
}

// sendProgressWithETA sends a progress update with ETA information.
func (i *Initializer) sendProgressWithETA(phase, message string, percent float64, remainingPhases []string) {
	if i.config.ProgressChan == nil {
		return
	}

	var eta time.Duration
	var elapsed time.Duration
	var currentPhase, totalPhases int

	if i.etaTracker != nil {
		eta = i.etaTracker.GetETARemaining(remainingPhases)
		elapsed = i.etaTracker.GetElapsed()
		currentPhase = i.etaTracker.GetCurrentPhase()
		totalPhases = i.etaTracker.GetTotalPhases()
	}

	select {
	case i.config.ProgressChan <- InitProgress{
		Phase:          phase,
		Message:        message,
		Percent:        percent,
		ETARemaining:   eta,
		ElapsedTime:    elapsed,
		CurrentPhaseNo: currentPhase,
		TotalPhases:    totalPhases,
	}:
	default:
		// Don't block if channel is full
	}
}

// startPhaseWithETA starts a new phase and sends a progress update.
func (i *Initializer) startPhaseWithETA(phaseNum int, phaseName, message string, percent float64, remainingPhases []string) {
	if i.etaTracker != nil {
		i.etaTracker.StartPhase(phaseNum)
	}
	i.sendProgressWithETA(phaseName, message, percent, remainingPhases)
}

// completePhaseWithETA completes a phase and updates the ETA tracker.
func (i *Initializer) completePhaseWithETA(phaseName string) {
	if i.etaTracker != nil {
		i.etaTracker.CompletePhase(phaseName)
	}
}
