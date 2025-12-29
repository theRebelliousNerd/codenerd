// Package shards provides specialist agent management including background observers.
// This file implements the BackgroundObserverManager for running observer specialists
// (like Northstar) that monitor activities and provide alignment assessments.
package shards

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// =============================================================================
// BACKGROUND OBSERVER INFRASTRUCTURE
// =============================================================================

// ObserverEvent represents an event that background observers can react to.
type ObserverEvent struct {
	Type      ObserverEventType
	Source    string            // Who generated this event (shard name, system, user)
	Target    string            // Target of the event (file, task, campaign)
	Details   map[string]string // Event-specific details
	Timestamp time.Time
}

// ObserverEventType categorizes events that observers care about.
type ObserverEventType string

const (
	EventTaskStarted     ObserverEventType = "task_started"
	EventTaskCompleted   ObserverEventType = "task_completed"
	EventTaskFailed      ObserverEventType = "task_failed"
	EventCampaignStarted ObserverEventType = "campaign_started"
	EventCampaignPhase   ObserverEventType = "campaign_phase"
	EventCampaignEnded   ObserverEventType = "campaign_ended"
	EventFileModified    ObserverEventType = "file_modified"
	EventUserIntent      ObserverEventType = "user_intent"
	EventAlignmentCheck  ObserverEventType = "alignment_check" // Periodic check request
)

// ObserverAssessment represents an observer's assessment of an event/activity.
type ObserverAssessment struct {
	ObserverName string
	EventID      string
	Score        int               // 0-100 alignment/quality score
	Level        AssessmentLevel   // proceed, note, clarify, block
	VisionMatch  string            // How this serves the mission
	Deviations   []string          // Detected deviations
	Suggestions  []string          // Recommendations
	Metadata     map[string]string // Additional observer-specific data
	Timestamp    time.Time
}

// AssessmentLevel indicates the severity of an observer's assessment.
type AssessmentLevel string

const (
	LevelProceed  AssessmentLevel = "proceed"  // Score >= 80: All good
	LevelNote     AssessmentLevel = "note"     // Score 60-79: Proceed with note
	LevelClarify  AssessmentLevel = "clarify"  // Score 40-59: Needs justification
	LevelBlock    AssessmentLevel = "block"    // Score < 40: Block for review
)

// GetAssessmentLevel returns the level based on score.
func GetAssessmentLevel(score int) AssessmentLevel {
	switch {
	case score >= 80:
		return LevelProceed
	case score >= 60:
		return LevelNote
	case score >= 40:
		return LevelClarify
	default:
		return LevelBlock
	}
}

// ObserverCallback is the function signature for observer assessment handlers.
type ObserverCallback func(assessment ObserverAssessment)

// NorthstarHandler processes Northstar-specific events directly.
// This bypasses the generic spawner for more efficient alignment checks.
type NorthstarHandler interface {
	// HandleEvent processes an observer event and returns an assessment.
	HandleEvent(ctx context.Context, event ObserverEvent) (*ObserverAssessment, error)
}

// BackgroundObserverManager manages observer specialists running in the background.
type BackgroundObserverManager struct {
	mu sync.RWMutex

	// Active observers (observer name -> observer state)
	observers map[string]*ObserverState

	// Event channel for observers to receive events
	eventChan chan ObserverEvent

	// Assessment callbacks
	callbacks []ObserverCallback

	// Configuration
	enabled          bool
	checkInterval    time.Duration
	assessmentBuffer []ObserverAssessment

	// Spawner for creating observer tasks
	spawner ObserverSpawner

	// Northstar-specific handler for direct integration
	northstarHandler NorthstarHandler

	// Context for lifecycle management
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// ObserverState tracks the state of a background observer.
type ObserverState struct {
	Name           string
	Classification SpecialistClassification
	Active         bool
	LastCheck      time.Time
	LastAssessment *ObserverAssessment
	EventsReceived int
}

// ObserverSpawner interface for spawning observer tasks.
type ObserverSpawner interface {
	SpawnObserver(ctx context.Context, observerName, task string) (string, error)
}

// NewBackgroundObserverManager creates a new background observer manager.
func NewBackgroundObserverManager(spawner ObserverSpawner) *BackgroundObserverManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &BackgroundObserverManager{
		observers:     make(map[string]*ObserverState),
		eventChan:     make(chan ObserverEvent, 100),
		callbacks:     make([]ObserverCallback, 0),
		enabled:       false,
		checkInterval: 5 * time.Minute, // Default periodic check interval
		spawner:       spawner,
		ctx:           ctx,
		cancel:        cancel,
	}
}

// Start starts the background observer processing loop.
func (m *BackgroundObserverManager) Start() error {
	m.mu.Lock()
	if m.enabled {
		m.mu.Unlock()
		return fmt.Errorf("observer manager already running")
	}
	m.enabled = true
	m.mu.Unlock()

	// Start the event processing goroutine
	m.wg.Add(1)
	go m.eventLoop()

	// Start the periodic check goroutine
	m.wg.Add(1)
	go m.periodicCheckLoop()

	return nil
}

// Stop stops the background observer processing.
func (m *BackgroundObserverManager) Stop() {
	m.mu.Lock()
	m.enabled = false
	m.mu.Unlock()

	m.cancel()
	m.wg.Wait()
}

// RegisterObserver registers an observer specialist for background monitoring.
func (m *BackgroundObserverManager) RegisterObserver(name string) error {
	class, ok := GetSpecialistClassification(name)
	if !ok {
		return fmt.Errorf("unknown specialist: %s", name)
	}

	if class.ExecutionMode != SpecialistModeObserver {
		return fmt.Errorf("specialist %s is not an observer (mode: %s)", name, class.ExecutionMode)
	}

	if !class.BackgroundCapable {
		return fmt.Errorf("specialist %s is not background-capable", name)
	}

	m.mu.Lock()
	m.observers[strings.ToLower(name)] = &ObserverState{
		Name:           name,
		Classification: class,
		Active:         true,
		LastCheck:      time.Time{},
	}
	m.mu.Unlock()

	return nil
}

// UnregisterObserver removes an observer from background monitoring.
func (m *BackgroundObserverManager) UnregisterObserver(name string) {
	m.mu.Lock()
	delete(m.observers, strings.ToLower(name))
	m.mu.Unlock()
}

// GetActiveObservers returns the list of active background observers.
func (m *BackgroundObserverManager) GetActiveObservers() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var names []string
	for name, state := range m.observers {
		if state.Active {
			names = append(names, name)
		}
	}
	return names
}

// SendEvent sends an event to all background observers.
func (m *BackgroundObserverManager) SendEvent(event ObserverEvent) {
	m.mu.RLock()
	enabled := m.enabled
	m.mu.RUnlock()

	if !enabled {
		return
	}

	event.Timestamp = time.Now()

	select {
	case m.eventChan <- event:
	default:
		// Channel full, drop event (could log this)
	}
}

// AddCallback registers a callback for assessment notifications.
func (m *BackgroundObserverManager) AddCallback(cb ObserverCallback) {
	m.mu.Lock()
	m.callbacks = append(m.callbacks, cb)
	m.mu.Unlock()
}

// SetNorthstarHandler sets a specialized handler for Northstar alignment checks.
// This enables direct integration with the Northstar Guardian for efficient processing.
func (m *BackgroundObserverManager) SetNorthstarHandler(handler NorthstarHandler) {
	m.mu.Lock()
	m.northstarHandler = handler
	m.mu.Unlock()
}

// GetRecentAssessments returns recent observer assessments.
func (m *BackgroundObserverManager) GetRecentAssessments(limit int) []ObserverAssessment {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.assessmentBuffer) {
		limit = len(m.assessmentBuffer)
	}

	start := len(m.assessmentBuffer) - limit
	if start < 0 {
		start = 0
	}

	result := make([]ObserverAssessment, limit)
	copy(result, m.assessmentBuffer[start:])
	return result
}

// GetLastAssessment returns the most recent assessment from a specific observer.
func (m *BackgroundObserverManager) GetLastAssessment(observerName string) *ObserverAssessment {
	m.mu.RLock()
	defer m.mu.RUnlock()

	state, ok := m.observers[strings.ToLower(observerName)]
	if !ok {
		return nil
	}
	return state.LastAssessment
}

// eventLoop processes events and dispatches to observers.
func (m *BackgroundObserverManager) eventLoop() {
	defer m.wg.Done()

	for {
		select {
		case <-m.ctx.Done():
			return
		case event := <-m.eventChan:
			m.processEvent(event)
		}
	}
}

// periodicCheckLoop triggers periodic alignment checks.
func (m *BackgroundObserverManager) periodicCheckLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.SendEvent(ObserverEvent{
				Type:    EventAlignmentCheck,
				Source:  "system",
				Details: map[string]string{"trigger": "periodic"},
			})
		}
	}
}

// processEvent processes a single event by dispatching to relevant observers.
func (m *BackgroundObserverManager) processEvent(event ObserverEvent) {
	m.mu.RLock()
	observers := make([]*ObserverState, 0, len(m.observers))
	for _, state := range m.observers {
		if state.Active {
			observers = append(observers, state)
		}
	}
	spawner := m.spawner
	northstarHandler := m.northstarHandler
	m.mu.RUnlock()

	// Dispatch to each observer
	for _, obs := range observers {
		obs.EventsReceived++

		// Check for Northstar-specific handler
		if strings.ToLower(obs.Name) == "northstar" && northstarHandler != nil {
			go func(observerState *ObserverState) {
				ctx, cancel := context.WithTimeout(m.ctx, 2*time.Minute)
				defer cancel()

				assessment, err := northstarHandler.HandleEvent(ctx, event)
				if err != nil {
					// Log error but continue
					return
				}
				if assessment != nil {
					m.recordAssessment(observerState.Name, *assessment)
				}
			}(obs)
			continue
		}

		// Use generic spawner for other observers
		if spawner == nil {
			continue
		}

		// Build the assessment task
		task := m.buildAssessmentTask(event, obs)

		// Spawn the observer task (async)
		go func(observerState *ObserverState, assessTask string) {
			ctx, cancel := context.WithTimeout(m.ctx, 2*time.Minute)
			defer cancel()

			result, err := spawner.SpawnObserver(ctx, observerState.Name, assessTask)
			if err != nil {
				// Log error but continue
				return
			}

			// Parse the assessment from the result
			assessment := m.parseAssessment(observerState.Name, event, result)

			// Store and notify
			m.recordAssessment(observerState.Name, assessment)
		}(obs, task)
	}
}

// buildAssessmentTask creates the task prompt for an observer.
func (m *BackgroundObserverManager) buildAssessmentTask(event ObserverEvent, obs *ObserverState) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("ALIGNMENT CHECK REQUEST\n\n"))
	sb.WriteString(fmt.Sprintf("Event Type: %s\n", event.Type))
	sb.WriteString(fmt.Sprintf("Source: %s\n", event.Source))
	sb.WriteString(fmt.Sprintf("Target: %s\n", event.Target))
	sb.WriteString(fmt.Sprintf("Timestamp: %s\n\n", event.Timestamp.Format(time.RFC3339)))

	if len(event.Details) > 0 {
		sb.WriteString("Details:\n")
		for k, v := range event.Details {
			sb.WriteString(fmt.Sprintf("  %s: %s\n", k, v))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(`Provide your alignment assessment:
1. Score (0-100): How well does this align with project vision?
2. Vision Match: How does this serve the mission?
3. Deviations: Any scope creep or drift detected?
4. Recommendations: Course corrections if needed

Format your response as:
SCORE: <number>
VISION: <one line>
DEVIATIONS: <comma-separated list or "none">
RECOMMENDATIONS: <comma-separated list or "none">`)

	return sb.String()
}

// parseAssessment parses an observer's response into an assessment.
func (m *BackgroundObserverManager) parseAssessment(observerName string, event ObserverEvent, result string) ObserverAssessment {
	assessment := ObserverAssessment{
		ObserverName: observerName,
		EventID:      fmt.Sprintf("%s-%s-%d", event.Type, event.Target, event.Timestamp.UnixNano()),
		Score:        50, // Default neutral score
		Level:        LevelNote,
		Timestamp:    time.Now(),
		Deviations:   make([]string, 0),
		Suggestions:  make([]string, 0),
		Metadata:     make(map[string]string),
	}

	// Parse the structured response
	lines := strings.Split(result, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "SCORE:") {
			var score int
			if _, err := fmt.Sscanf(line, "SCORE: %d", &score); err == nil {
				assessment.Score = score
				assessment.Level = GetAssessmentLevel(score)
			}
		} else if strings.HasPrefix(line, "VISION:") {
			assessment.VisionMatch = strings.TrimPrefix(line, "VISION:")
			assessment.VisionMatch = strings.TrimSpace(assessment.VisionMatch)
		} else if strings.HasPrefix(line, "DEVIATIONS:") {
			devStr := strings.TrimPrefix(line, "DEVIATIONS:")
			devStr = strings.TrimSpace(devStr)
			if devStr != "none" && devStr != "" {
				assessment.Deviations = strings.Split(devStr, ",")
				for i := range assessment.Deviations {
					assessment.Deviations[i] = strings.TrimSpace(assessment.Deviations[i])
				}
			}
		} else if strings.HasPrefix(line, "RECOMMENDATIONS:") {
			recStr := strings.TrimPrefix(line, "RECOMMENDATIONS:")
			recStr = strings.TrimSpace(recStr)
			if recStr != "none" && recStr != "" {
				assessment.Suggestions = strings.Split(recStr, ",")
				for i := range assessment.Suggestions {
					assessment.Suggestions[i] = strings.TrimSpace(assessment.Suggestions[i])
				}
			}
		}
	}

	return assessment
}

// recordAssessment stores an assessment and notifies callbacks.
func (m *BackgroundObserverManager) recordAssessment(observerName string, assessment ObserverAssessment) {
	m.mu.Lock()

	// Update observer state
	if state, ok := m.observers[strings.ToLower(observerName)]; ok {
		state.LastCheck = time.Now()
		state.LastAssessment = &assessment
	}

	// Add to buffer (keep last 100)
	m.assessmentBuffer = append(m.assessmentBuffer, assessment)
	if len(m.assessmentBuffer) > 100 {
		m.assessmentBuffer = m.assessmentBuffer[1:]
	}

	// Copy callbacks
	callbacks := make([]ObserverCallback, len(m.callbacks))
	copy(callbacks, m.callbacks)

	m.mu.Unlock()

	// Notify callbacks
	for _, cb := range callbacks {
		cb(assessment)
	}
}

// FormatAssessment returns a human-readable string for an assessment.
func FormatAssessment(a ObserverAssessment) string {
	var sb strings.Builder

	levelIcon := map[AssessmentLevel]string{
		LevelProceed: "✅",
		LevelNote:    "📝",
		LevelClarify: "❓",
		LevelBlock:   "🚫",
	}

	sb.WriteString(fmt.Sprintf("%s **%s Assessment** (Score: %d/100)\n",
		levelIcon[a.Level], a.ObserverName, a.Score))
	sb.WriteString(fmt.Sprintf("Level: %s\n", a.Level))

	if a.VisionMatch != "" {
		sb.WriteString(fmt.Sprintf("Vision: %s\n", a.VisionMatch))
	}

	if len(a.Deviations) > 0 {
		sb.WriteString(fmt.Sprintf("Deviations: %s\n", strings.Join(a.Deviations, "; ")))
	}

	if len(a.Suggestions) > 0 {
		sb.WriteString(fmt.Sprintf("Recommendations: %s\n", strings.Join(a.Suggestions, "; ")))
	}

	return sb.String()
}
