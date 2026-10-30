package northstar

import (
	"context"
	"fmt"
	"maps"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"codenerd/internal/logging"
)

// CampaignObserver observes campaign execution and performs alignment checks.
type CampaignObserver struct {
	guardian  *Guardian
	mu        sync.RWMutex
	closeOnce sync.Once

	// Campaign state
	campaignID    string
	currentPhase  string
	tasksInPhase  int
	phaseChecks   map[string]*AlignmentCheck
	startCoverage *AlignmentCoverage

	// Configuration
	checkOnPhaseTransition bool
	checkEveryNTasks       int
}

// NewCampaignObserver creates a new campaign observer.
func NewCampaignObserver(guardian *Guardian) *CampaignObserver {
	checkOnPhaseTransition := true
	checkEveryNTasks := 5
	if guardian != nil {
		guardian.mu.RLock()
		checkOnPhaseTransition = guardian.config.EnablePhaseGates
		if guardian.config.PeriodicCheckInterval > 0 {
			checkEveryNTasks = guardian.config.PeriodicCheckInterval
		}
		guardian.mu.RUnlock()
	}

	return &CampaignObserver{
		guardian:               guardian,
		phaseChecks:            make(map[string]*AlignmentCheck),
		checkOnPhaseTransition: checkOnPhaseTransition,
		checkEveryNTasks:       checkEveryNTasks,
	}
}

// StartCampaign initializes observation for a new campaign and returns the
// goal's alignment check. It does not decide whether the campaign may start:
// the campaign risk gate grades the check (blocked, unknown, failed, passed)
// under the kernel's hard/soft contract. It used to return an error only for
// blocked, which collapsed "unassessed" and "failed" into "passed".
//
// With no vision defined the check is the guardian's skipped result.
func (o *CampaignObserver) StartCampaign(ctx context.Context, campaignID, goal string, scope AlignmentScope) (*AlignmentCheck, error) {
	o.mu.Lock()
	o.campaignID = campaignID
	o.currentPhase = ""
	o.tasksInPhase = 0
	o.phaseChecks = make(map[string]*AlignmentCheck)
	o.startCoverage = nil
	o.mu.Unlock()

	check, err := o.guardian.CheckAlignment(ctx, TriggerCampaignStart, goal,
		fmt.Sprintf("Campaign %s starting with goal: %s", campaignID, goal), scope)
	if err != nil {
		return nil, fmt.Errorf("campaign start alignment check: %w", err)
	}

	o.mu.Lock()
	if check.Result.Assessed() {
		o.startCoverage = cloneAlignmentCoverage(check.Coverage)
	}
	o.mu.Unlock()

	logging.Get(logging.CategoryNorthstar).Info("Campaign observer started for %s (alignment %s)", campaignID, check.Result)
	return check, nil
}

// OnPhaseStart is called when a new campaign phase begins.
func (o *CampaignObserver) OnPhaseStart(ctx context.Context, phaseName, phaseGoal string, scope AlignmentScope) (*AlignmentCheck, error) {
	o.mu.Lock()
	previousPhase := o.currentPhase
	startCoverage := o.startCoverage
	o.currentPhase = phaseName
	o.tasksInPhase = 0
	o.mu.Unlock()

	if !o.checkOnPhaseTransition || !o.guardian.HasVision() {
		return nil, nil
	}

	// Only reuse campaign-start coverage for a first phase it actually covered.
	if previousPhase == "" && coverageContainsScope(startCoverage, scope) {
		current, _ := o.guardian.resolveAlignmentScope(scope, o.guardian.GetVision())
		if current.Complete && current.VisionDigest == startCoverage.VisionDigest && current.ModuleDigest == startCoverage.ModuleDigest {
			return nil, nil
		}
	}

	// Perform phase gate check
	check, err := o.guardian.CheckAlignment(ctx, TriggerPhaseGate, phaseGoal,
		fmt.Sprintf("Phase transition: %s -> %s", previousPhase, phaseName), scope)
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
	o.mu.RLock()
	campaignID := o.campaignID
	o.mu.RUnlock()

	// Record observation
	obs := &Observation{
		SessionID: campaignID,
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
func (o *CampaignObserver) OnTaskComplete(ctx context.Context, taskID, taskDesc, result string, scope AlignmentScope) (*AlignmentCheck, error) {
	o.mu.Lock()
	o.tasksInPhase++
	tasksInPhase := o.tasksInPhase
	phase := o.currentPhase
	campaignID := o.campaignID
	o.mu.Unlock()

	taskSubject := taskDesc
	if taskID != "" {
		taskSubject = fmt.Sprintf("%s [%s]", taskDesc, taskID)
	}

	// Record observation
	if err := o.guardian.ObserveTaskCompletion(campaignID, "campaign_task", taskSubject, result); err != nil {
		logging.Get(logging.CategoryNorthstar).Debug("Failed to record campaign task observation: %v", err)
	}

	// Only observed effects are recorded as modifications. Artifact paths may
	// still trigger an advisory check, but are labelled as artifacts in it.
	normalized, _ := normalizeAlignmentScope(scope)
	filePaths := make([]string, 0, len(normalized.Paths))
	for _, entry := range normalized.Paths {
		filePaths = append(filePaths, entry.Path)
		if entry.Role != PathObservedEffect {
			continue
		}
		if err := o.guardian.ObserveFileChange(campaignID, entry.Path, "modified"); err != nil {
			logging.Get(logging.CategoryNorthstar).Debug("Failed to record campaign file change: %v", err)
		}
	}

	// Check if periodic check is due
	if tasksInPhase > 0 && tasksInPhase%o.checkEveryNTasks == 0 && o.guardian.HasVision() {
		check, err := o.guardian.CheckAlignment(ctx, TriggerTaskComplete, taskSubject,
			fmt.Sprintf("After %d tasks in phase %s (task_id=%s)", tasksInPhase, phase, taskID), scope)
		if err != nil {
			return nil, err
		}
		return check, nil
	}

	// Check for high-impact changes
	if o.guardian.ShouldCheckNow(TriggerHighImpact, filePaths) {
		check, err := o.guardian.CheckAlignment(ctx, TriggerHighImpact, taskSubject,
			fmt.Sprintf("High-impact scope after task %s: %v", taskID, filePaths), scope)
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
	maps.Copy(result, o.phaseChecks)
	return result
}

// Close releases the guardian reference BuildCampaignObserver acquired.
// The observer must not be used afterwards. Safe on a nil observer and
// safe to call more than once: only the first call releases.
func (o *CampaignObserver) Close() error {
	if o == nil {
		return nil
	}
	var err error
	o.closeOnce.Do(func() {
		err = ReleaseGuardian(o.guardian)
	})
	return err
}

// =============================================================================
// TASK OBSERVER (For Standard Task Execution)
// =============================================================================

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
	subject := h.buildSubject(eventType, source, target, details)
	contextStr := h.buildEventContext(eventType, source, target, details, timestamp)
	h.recordEventObservation(eventType, source, target, details, timestamp, subject, contextStr)

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

	// Perform alignment check
	scope := AlignmentScope{Disposition: ScopeUnresolved}
	if h.guardian.store != nil {
		scope.Workspace = filepath.Dir(filepath.Dir(h.guardian.store.Path()))
	}
	if eventType == "file_modified" && target != "" {
		scope.Disposition = ScopePaths
		scope.Paths = []AlignmentPath{{Path: target, Role: PathObservedEffect}}
	}
	check, err := h.guardian.CheckAlignment(ctx, trigger, subject, contextStr, scope)
	if err != nil {
		return nil, err
	}

	metadata := map[string]string{"trigger": string(trigger), "result": string(check.Result)}
	if c := check.Coverage; c != nil {
		metadata["workspace"] = c.Scope.Workspace
		metadata["scope"] = string(c.Scope.Disposition)
		metadata["scope_outcome"] = string(c.Outcome)
		metadata["scope_reason"] = strings.Join(c.Issues, "; ")
		metadata["scope_complete"] = strconv.FormatBool(c.Complete)
		metadata["selected_modules"] = strconv.Itoa(len(c.Modules))
		metadata["unmatched_paths"] = strconv.Itoa(len(c.UnmatchedPaths))
		metadata["scope_issues"] = strconv.Itoa(len(c.Issues))
		metadata["vision_digest"] = c.VisionDigest
		metadata["module_digest"] = c.ModuleDigest
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
		Metadata:     metadata,
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

func (h *BackgroundEventHandler) buildEventContext(eventType, source, target string, details map[string]string, timestamp time.Time) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Event Type: %s\n", eventType))
	sb.WriteString(fmt.Sprintf("Source: %s\n", source))
	if target != "" {
		sb.WriteString(fmt.Sprintf("Target: %s\n", target))
	}
	if !timestamp.IsZero() {
		sb.WriteString(fmt.Sprintf("Timestamp: %s\n", timestamp.Format(time.RFC3339)))
	}
	if len(details) > 0 {
		sb.WriteString("\nDetails:\n")
		keys := make([]string, 0, len(details))
		for k := range details {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			v := details[k]
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
	case AlignmentUnknown:
		// Nobody assessed the event. "note" read as "proceed, with a remark";
		// an unassessed event needs a human to look, which is "clarify".
		return "clarify"
	default:
		return "note"
	}
}

func (h *BackgroundEventHandler) recordEventObservation(eventType, source, target string, details map[string]string, timestamp time.Time, subject, context string) {
	metadata := map[string]string{
		"event_type": eventType,
		"source":     source,
	}
	if target != "" {
		metadata["target"] = target
	}
	for k, v := range details {
		metadata["detail."+k] = v
	}

	relevance := h.guardian.calculateRelevance(subject + "\n" + context)
	if eventType == "file_modified" && target != "" {
		relevance = h.guardian.calculatePathRelevance(target)
	}
	if timestamp.IsZero() {
		timestamp = time.Now()
	}

	obs := &Observation{
		SessionID: h.sessionID,
		Timestamp: timestamp,
		Type:      h.observationTypeForEvent(eventType),
		Subject:   subject,
		Content:   context,
		Relevance: relevance,
		Tags:      []string{"background_observer", eventType},
		Metadata:  metadata,
	}

	if err := h.guardian.store.RecordObservation(obs); err != nil {
		logging.Get(logging.CategoryNorthstar).Debug("Failed to record background observer event: %v", err)
	}
}

func (h *BackgroundEventHandler) observationTypeForEvent(eventType string) ObservationType {
	switch eventType {
	case "task_completed":
		return ObsTaskCompleted
	case "file_modified":
		return ObsFileChanged
	case "campaign_phase":
		return ObsDecisionMade
	case "alignment_check":
		return ObsPatternDetected
	default:
		return ObsPatternDetected
	}
}
