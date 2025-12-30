package northstar

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"codenerd/internal/logging"
)

// CampaignObserver observes campaign execution and performs alignment checks.
type CampaignObserver struct {
	guardian *Guardian
	mu       sync.RWMutex

	// Campaign state
	campaignID    string
	currentPhase  string
	tasksInPhase  int
	phaseChecks   map[string]*AlignmentCheck

	// Configuration
	checkOnPhaseTransition bool
	checkEveryNTasks       int
}

// NewCampaignObserver creates a new campaign observer.
func NewCampaignObserver(guardian *Guardian) *CampaignObserver {
	return &CampaignObserver{
		guardian:               guardian,
		phaseChecks:            make(map[string]*AlignmentCheck),
		checkOnPhaseTransition: true,
		checkEveryNTasks:       5,
	}
}

// StartCampaign initializes observation for a new campaign.
func (o *CampaignObserver) StartCampaign(ctx context.Context, campaignID, goal string) error {
	o.mu.Lock()
	o.campaignID = campaignID
	o.currentPhase = ""
	o.tasksInPhase = 0
	o.phaseChecks = make(map[string]*AlignmentCheck)
	o.mu.Unlock()

	// Perform initial alignment check for the campaign goal
	if o.guardian.HasVision() {
		check, err := o.guardian.CheckAlignment(ctx, TriggerCampaignStart, goal,
			fmt.Sprintf("Campaign %s starting with goal: %s", campaignID, goal))
		if err != nil {
			logging.Get(logging.CategoryNorthstar).Debug("Campaign start alignment check failed: %v", err)
		} else if check.Result == AlignmentBlocked {
			return fmt.Errorf("campaign goal does not align with vision: %s", check.Explanation)
		}
	}

	logging.Get(logging.CategoryNorthstar).Info("Campaign observer started for %s", campaignID)
	return nil
}

// OnPhaseStart is called when a new campaign phase begins.
func (o *CampaignObserver) OnPhaseStart(ctx context.Context, phaseName, phaseGoal string) (*AlignmentCheck, error) {
	o.mu.Lock()
	previousPhase := o.currentPhase
	o.currentPhase = phaseName
	o.tasksInPhase = 0
	o.mu.Unlock()

	// Skip check for first phase (already checked at campaign start)
	if previousPhase == "" {
		return nil, nil
	}

	if !o.checkOnPhaseTransition || !o.guardian.HasVision() {
		return nil, nil
	}

	// Perform phase gate check
	check, err := o.guardian.CheckAlignment(ctx, TriggerPhaseGate, phaseGoal,
		fmt.Sprintf("Phase transition: %s -> %s", previousPhase, phaseName))
	if err != nil {
		return nil, err
	}

	o.mu.Lock()
	o.phaseChecks[phaseName] = check
	o.mu.Unlock()

	if check.Result == AlignmentBlocked {
		return check, fmt.Errorf("phase %s does not align with vision: %s", phaseName, check.Explanation)
	}

	return check, nil
}

// OnPhaseComplete is called when a campaign phase completes.
func (o *CampaignObserver) OnPhaseComplete(ctx context.Context, phaseName string, success bool, summary string) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	// Record observation
	obs := &Observation{
		SessionID: o.campaignID,
		Timestamp: time.Now(),
		Type:      ObsTaskCompleted,
		Subject:   fmt.Sprintf("phase:%s", phaseName),
		Content:   fmt.Sprintf("Phase %s completed (success=%t): %s", phaseName, success, summary),
		Relevance: 0.8,
		Tags:      []string{"phase", phaseName},
		Metadata:  map[string]string{"success": fmt.Sprintf("%t", success)},
	}

	return o.guardian.store.RecordObservation(obs)
}

// OnTaskComplete is called when a campaign task completes.
func (o *CampaignObserver) OnTaskComplete(ctx context.Context, taskID, taskDesc, result string, filePaths []string) (*AlignmentCheck, error) {
	o.mu.Lock()
	o.tasksInPhase++
	tasksInPhase := o.tasksInPhase
	phase := o.currentPhase
	o.mu.Unlock()

	// Record observation
	o.guardian.ObserveTaskCompletion(o.campaignID, "campaign_task", taskDesc, result)

	// Record file changes
	for _, path := range filePaths {
		o.guardian.ObserveFileChange(o.campaignID, path, "modified")
	}

	// Check if periodic check is due
	if tasksInPhase > 0 && tasksInPhase%o.checkEveryNTasks == 0 && o.guardian.HasVision() {
		check, err := o.guardian.CheckAlignment(ctx, TriggerTaskComplete, taskDesc,
			fmt.Sprintf("After %d tasks in phase %s", tasksInPhase, phase))
		if err != nil {
			return nil, err
		}
		return check, nil
	}

	// Check for high-impact changes
	if o.guardian.ShouldCheckNow(TriggerHighImpact, filePaths) {
		check, err := o.guardian.CheckAlignment(ctx, TriggerHighImpact, taskDesc,
			fmt.Sprintf("High-impact files modified: %v", filePaths))
		return check, err
	}

	return nil, nil
}

// EndCampaign finalizes campaign observation.
func (o *CampaignObserver) EndCampaign(ctx context.Context, success bool, summary string) error {
	o.mu.Lock()
	campaignID := o.campaignID
	o.mu.Unlock()

	// Final observation
	obs := &Observation{
		SessionID: campaignID,
		Timestamp: time.Now(),
		Type:      ObsTaskCompleted,
		Subject:   "campaign:end",
		Content:   fmt.Sprintf("Campaign ended (success=%t): %s", success, summary),
		Relevance: 0.9,
		Tags:      []string{"campaign", "end"},
		Metadata:  map[string]string{"success": fmt.Sprintf("%t", success)},
	}

	if err := o.guardian.store.RecordObservation(obs); err != nil {
		return err
	}

	logging.Get(logging.CategoryNorthstar).Info("Campaign observer ended for %s (success=%t)", campaignID, success)
	return nil
}

// GetPhaseCheck returns the alignment check for a specific phase.
func (o *CampaignObserver) GetPhaseCheck(phaseName string) *AlignmentCheck {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.phaseChecks[phaseName]
}

// GetAllPhaseChecks returns all phase alignment checks.
func (o *CampaignObserver) GetAllPhaseChecks() map[string]*AlignmentCheck {
	o.mu.RLock()
	defer o.mu.RUnlock()

	result := make(map[string]*AlignmentCheck)
	for k, v := range o.phaseChecks {
		result[k] = v
	}
	return result
}

// =============================================================================
// TASK OBSERVER (For Standard Task Execution)
// =============================================================================

// TaskObserver observes standard task execution (non-campaign).
type TaskObserver struct {
	guardian  *Guardian
	sessionID string
	mu        sync.Mutex
}

// NewTaskObserver creates a task observer for standard execution.
func NewTaskObserver(guardian *Guardian, sessionID string) *TaskObserver {
	return &TaskObserver{
		guardian:  guardian,
		sessionID: sessionID,
	}
}

// OnTaskStart records the start of a task.
func (t *TaskObserver) OnTaskStart(taskType, taskDesc string) {
	// Lightweight - just log
	logging.Get(logging.CategoryNorthstar).Debug("Task started: %s - %s", taskType, truncate(taskDesc, 50))
}

// OnTaskComplete records task completion and may trigger alignment check.
func (t *TaskObserver) OnTaskComplete(ctx context.Context, taskType, taskDesc, result string, filePaths []string) (*AlignmentCheck, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Record observation
	t.guardian.ObserveTaskCompletion(t.sessionID, taskType, taskDesc, result)

	// Record file changes
	for _, path := range filePaths {
		t.guardian.ObserveFileChange(t.sessionID, path, "modified")
	}

	// Delegate to guardian for periodic check logic
	return t.guardian.OnTaskComplete(ctx, taskDesc)
}

// OnError records an error observation.
func (t *TaskObserver) OnError(taskType, taskDesc, errorMsg string) {
	obs := &Observation{
		SessionID: t.sessionID,
		Timestamp: time.Now(),
		Type:      ObsPatternDetected,
		Subject:   "error:" + taskType,
		Content:   fmt.Sprintf("Error in %s: %s\nTask: %s", taskType, errorMsg, taskDesc),
		Relevance: 0.7,
		Tags:      []string{"error", taskType},
	}

	if err := t.guardian.store.RecordObservation(obs); err != nil {
		logging.Get(logging.CategoryNorthstar).Debug("Failed to record error observation: %v", err)
	}
}

// =============================================================================
// BACKGROUND EVENT HANDLER (For BackgroundObserverManager Integration)
// =============================================================================

// BackgroundEventHandler implements the shards.NorthstarHandler interface.
// It processes events from the BackgroundObserverManager using the Northstar Guardian.
type BackgroundEventHandler struct {
	guardian  *Guardian
	sessionID string
}

// NewBackgroundEventHandler creates a handler for BackgroundObserverManager integration.
func NewBackgroundEventHandler(guardian *Guardian, sessionID string) *BackgroundEventHandler {
	return &BackgroundEventHandler{
		guardian:  guardian,
		sessionID: sessionID,
	}
}

// ObserverAssessment mirrors shards.ObserverAssessment to avoid import cycle.
type ObserverAssessment struct {
	ObserverName string
	EventID      string
	Score        int
	Level        string
	VisionMatch  string
	Deviations   []string
	Suggestions  []string
	Metadata     map[string]string
	Timestamp    time.Time
}

// ObserverEvent mirrors shards.ObserverEvent to avoid import cycle.
type ObserverEvent struct {
	Type      string
	Source    string
	Target    string
	Details   map[string]string
	Timestamp time.Time
}

// HandleEvent processes an observer event and returns an assessment.
func (h *BackgroundEventHandler) HandleEvent(ctx context.Context, eventType, source, target string, details map[string]string, timestamp time.Time) (*ObserverAssessment, error) {
	if !h.guardian.HasVision() {
		return nil, nil // No vision defined, skip
	}

	// Determine trigger type based on event
	var trigger AlignmentTrigger
	switch eventType {
	case "task_completed":
		trigger = TriggerTaskComplete
	case "campaign_phase":
		trigger = TriggerPhaseGate
	case "file_modified":
		trigger = TriggerHighImpact
	case "alignment_check":
		trigger = TriggerPeriodic
	default:
		trigger = TriggerPeriodic
	}

	// Build context from event details
	contextStr := h.buildEventContext(eventType, source, target, details)

	// Build subject from event
	subject := h.buildSubject(eventType, source, target, details)

	// Perform alignment check
	check, err := h.guardian.CheckAlignment(ctx, trigger, subject, contextStr)
	if err != nil {
		return nil, err
	}

	// Convert to ObserverAssessment
	return &ObserverAssessment{
		ObserverName: "northstar",
		EventID:      check.ID,
		Score:        int(check.Score * 100),
		Level:        h.resultToLevel(check.Result),
		VisionMatch:  check.Explanation,
		Deviations:   []string{}, // Not tracked separately
		Suggestions:  check.Suggestions,
		Metadata:     map[string]string{"trigger": string(trigger)},
		Timestamp:    check.Timestamp,
	}, nil
}

func (h *BackgroundEventHandler) buildSubject(eventType, source, target string, details map[string]string) string {
	switch eventType {
	case "task_completed":
		if desc, ok := details["task_description"]; ok {
			return desc
		}
		return fmt.Sprintf("%s completed task on %s", source, target)
	case "campaign_phase":
		if phase, ok := details["phase"]; ok {
			return fmt.Sprintf("Campaign phase: %s", phase)
		}
		return "Campaign phase transition"
	case "file_modified":
		return fmt.Sprintf("File modified: %s", target)
	case "alignment_check":
		if reason, ok := details["trigger"]; ok {
			return fmt.Sprintf("Periodic check (%s)", reason)
		}
		return "Periodic alignment check"
	default:
		return fmt.Sprintf("%s event on %s", eventType, target)
	}
}

func (h *BackgroundEventHandler) buildEventContext(eventType, source, target string, details map[string]string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Event Type: %s\n", eventType))
	sb.WriteString(fmt.Sprintf("Source: %s\n", source))
	if target != "" {
		sb.WriteString(fmt.Sprintf("Target: %s\n", target))
	}
	if len(details) > 0 {
		sb.WriteString("\nDetails:\n")
		for k, v := range details {
			sb.WriteString(fmt.Sprintf("  %s: %s\n", k, v))
		}
	}
	return sb.String()
}

func (h *BackgroundEventHandler) resultToLevel(result AlignmentResult) string {
	switch result {
	case AlignmentPassed:
		return "proceed"
	case AlignmentWarning:
		return "note"
	case AlignmentFailed:
		return "clarify"
	case AlignmentBlocked:
		return "block"
	default:
		return "note"
	}
}
