package northstar

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"codenerd/internal/logging"
)

// Guardian is the Northstar vision guardian.
// It monitors project activity and ensures alignment with the defined vision.
type Guardian struct {
	store   *Store
	config  GuardianConfig
	llm     LLMClient
	mu      sync.RWMutex

	// Runtime state
	state   *GuardianState
	vision  *Vision
}

// LLMClient interface for alignment checks.
type LLMClient interface {
	CompleteWithSystem(ctx context.Context, system, user string) (string, error)
}

// NewGuardian creates a new Northstar Guardian.
func NewGuardian(store *Store, config GuardianConfig) *Guardian {
	return &Guardian{
		store:  store,
		config: config,
	}
}

// SetLLMClient sets the LLM client for alignment checks.
func (g *Guardian) SetLLMClient(client LLMClient) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.llm = client
}

// Initialize loads the vision and state from the store.
func (g *Guardian) Initialize() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	vision, err := g.store.LoadVision()
	if err != nil {
		return fmt.Errorf("failed to load vision: %w", err)
	}
	g.vision = vision

	state, err := g.store.GetState()
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}
	g.state = state

	if g.vision != nil {
		logging.Get(logging.CategoryNorthstar).Info("Northstar Guardian initialized with vision: %s", truncate(g.vision.Mission, 50))
	} else {
		logging.Get(logging.CategoryNorthstar).Info("Northstar Guardian initialized (no vision defined)")
	}

	return nil
}

// HasVision returns true if a vision is defined.
func (g *Guardian) HasVision() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.vision != nil
}

// GetVision returns the current vision.
func (g *Guardian) GetVision() *Vision {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.vision
}

// GetState returns the current guardian state.
func (g *Guardian) GetState() *GuardianState {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.state
}

// UpdateVision updates the stored vision.
func (g *Guardian) UpdateVision(vision *Vision) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if err := g.store.SaveVision(vision); err != nil {
		return err
	}
	g.vision = vision
	g.state.VisionDefined = true

	logging.Get(logging.CategoryNorthstar).Info("Vision updated: %s", truncate(vision.Mission, 50))
	return nil
}

// =============================================================================
// ALIGNMENT CHECKING
// =============================================================================

// CheckAlignment performs an alignment check for the given subject.
func (g *Guardian) CheckAlignment(ctx context.Context, trigger AlignmentTrigger, subject, context string) (*AlignmentCheck, error) {
	g.mu.RLock()
	vision := g.vision
	llm := g.llm
	g.mu.RUnlock()

	check := &AlignmentCheck{
		ID:        fmt.Sprintf("check-%d", time.Now().UnixNano()),
		Timestamp: time.Now(),
		Trigger:   trigger,
		Subject:   subject,
		Context:   context,
	}

	// If no vision, skip
	if vision == nil {
		check.Result = AlignmentSkipped
		check.Score = 1.0
		check.Explanation = "No vision defined - skipping alignment check"
		return check, nil
	}

	// If no LLM, do basic check
	if llm == nil {
		check.Result = AlignmentPassed
		check.Score = 0.8
		check.Explanation = "LLM not available for deep analysis - assuming aligned"
		return check, nil
	}

	startTime := time.Now()

	// Build the alignment prompt
	systemPrompt := g.buildAlignmentSystemPrompt(vision)
	userPrompt := g.buildAlignmentUserPrompt(subject, context)

	response, err := llm.CompleteWithSystem(ctx, systemPrompt, userPrompt)
	if err != nil {
		check.Result = AlignmentWarning
		check.Score = 0.7
		check.Explanation = fmt.Sprintf("Failed to complete alignment check: %v", err)
		check.Duration = time.Since(startTime)
		return check, nil
	}

	// Parse the response
	g.parseAlignmentResponse(response, check)
	check.Duration = time.Since(startTime)

	// Record the check
	if err := g.store.RecordAlignmentCheck(check); err != nil {
		logging.Get(logging.CategoryNorthstar).Debug("Failed to record alignment check: %v", err)
	}

	// Record drift event if needed
	if check.Result == AlignmentFailed || check.Result == AlignmentBlocked {
		drift := &DriftEvent{
			Timestamp:    time.Now(),
			Severity:     g.scoreToSeverity(check.Score),
			Category:     "alignment",
			Description:  check.Explanation,
			Evidence:     []string{subject},
			RelatedCheck: check.ID,
		}
		g.store.RecordDriftEvent(drift)
	}

	logging.Get(logging.CategoryNorthstar).Info("Alignment check: %s [%s] score=%.2f",
		subject, check.Result, check.Score)

	return check, nil
}

func (g *Guardian) buildAlignmentSystemPrompt(vision *Vision) string {
	var sb strings.Builder
	sb.WriteString("You are the Northstar Alignment Guardian for a software project.\n\n")
	sb.WriteString("## Project Vision\n")
	sb.WriteString(fmt.Sprintf("**Mission:** %s\n", vision.Mission))
	sb.WriteString(fmt.Sprintf("**Problem:** %s\n", vision.Problem))
	sb.WriteString(fmt.Sprintf("**Vision:** %s\n\n", vision.VisionStmt))

	if len(vision.Personas) > 0 {
		sb.WriteString("## Target Users\n")
		for _, p := range vision.Personas {
			sb.WriteString(fmt.Sprintf("- **%s**: Needs: %s\n", p.Name, strings.Join(p.Needs, ", ")))
		}
		sb.WriteString("\n")
	}

	if len(vision.Capabilities) > 0 {
		sb.WriteString("## Planned Capabilities\n")
		for _, c := range vision.Capabilities {
			sb.WriteString(fmt.Sprintf("- [%s/%s] %s\n", c.Priority, c.Timeline, c.Description))
		}
		sb.WriteString("\n")
	}

	if len(vision.Constraints) > 0 {
		sb.WriteString("## Constraints\n")
		for _, c := range vision.Constraints {
			sb.WriteString(fmt.Sprintf("- %s\n", c))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## Your Task\n")
	sb.WriteString("Evaluate whether the given subject/change aligns with this vision.\n")
	sb.WriteString("Respond in this EXACT format:\n")
	sb.WriteString("SCORE: <0.0-1.0>\n")
	sb.WriteString("RESULT: <passed|warning|failed|blocked>\n")
	sb.WriteString("EXPLANATION: <one sentence explanation>\n")
	sb.WriteString("SUGGESTIONS: <comma-separated suggestions, or 'none'>\n")

	return sb.String()
}

func (g *Guardian) buildAlignmentUserPrompt(subject, context string) string {
	var sb strings.Builder
	sb.WriteString("## Subject to Evaluate\n")
	sb.WriteString(subject)
	sb.WriteString("\n\n")
	if context != "" {
		sb.WriteString("## Additional Context\n")
		sb.WriteString(context)
		sb.WriteString("\n\n")
	}
	sb.WriteString("Evaluate alignment with the project vision.")
	return sb.String()
}

func (g *Guardian) parseAlignmentResponse(response string, check *AlignmentCheck) {
	// Default values
	check.Score = 0.7
	check.Result = AlignmentWarning
	check.Explanation = "Unable to parse alignment response"

	lines := strings.Split(response, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "SCORE:") {
			var score float64
			fmt.Sscanf(strings.TrimPrefix(line, "SCORE:"), "%f", &score)
			if score >= 0 && score <= 1 {
				check.Score = score
			}
		} else if strings.HasPrefix(line, "RESULT:") {
			result := strings.TrimSpace(strings.TrimPrefix(line, "RESULT:"))
			switch strings.ToLower(result) {
			case "passed":
				check.Result = AlignmentPassed
			case "warning":
				check.Result = AlignmentWarning
			case "failed":
				check.Result = AlignmentFailed
			case "blocked":
				check.Result = AlignmentBlocked
			}
		} else if strings.HasPrefix(line, "EXPLANATION:") {
			check.Explanation = strings.TrimSpace(strings.TrimPrefix(line, "EXPLANATION:"))
		} else if strings.HasPrefix(line, "SUGGESTIONS:") {
			sugStr := strings.TrimSpace(strings.TrimPrefix(line, "SUGGESTIONS:"))
			if sugStr != "none" && sugStr != "" {
				for _, s := range strings.Split(sugStr, ",") {
					if s = strings.TrimSpace(s); s != "" {
						check.Suggestions = append(check.Suggestions, s)
					}
				}
			}
		}
	}

	// Derive result from score if not explicitly set
	if check.Score >= g.config.WarningThreshold {
		check.Result = AlignmentPassed
	} else if check.Score >= g.config.FailureThreshold {
		check.Result = AlignmentWarning
	} else if check.Score >= g.config.BlockThreshold {
		check.Result = AlignmentFailed
	} else {
		check.Result = AlignmentBlocked
	}
}

func (g *Guardian) scoreToSeverity(score float64) DriftSeverity {
	if score >= 0.7 {
		return DriftMinor
	} else if score >= 0.5 {
		return DriftModerate
	} else if score >= 0.3 {
		return DriftMajor
	}
	return DriftCritical
}

// =============================================================================
// OBSERVATION RECORDING
// =============================================================================

// ObserveTaskCompletion records an observation about a completed task.
func (g *Guardian) ObserveTaskCompletion(sessionID, taskType, taskDesc, result string) error {
	obs := &Observation{
		SessionID: sessionID,
		Timestamp: time.Now(),
		Type:      ObsTaskCompleted,
		Subject:   taskType,
		Content:   fmt.Sprintf("Task: %s\nResult: %s", taskDesc, truncate(result, 500)),
		Relevance: 0.5, // Will be updated based on vision relevance
		Tags:      []string{taskType},
	}

	// Calculate relevance if vision exists
	if g.vision != nil {
		obs.Relevance = g.calculateRelevance(taskDesc + " " + result)
	}

	return g.store.RecordObservation(obs)
}

// ObserveFileChange records an observation about a file change.
func (g *Guardian) ObserveFileChange(sessionID, filePath, changeType string) error {
	obs := &Observation{
		SessionID: sessionID,
		Timestamp: time.Now(),
		Type:      ObsFileChanged,
		Subject:   filePath,
		Content:   fmt.Sprintf("File %s: %s", changeType, filePath),
		Relevance: g.calculatePathRelevance(filePath),
		Tags:      []string{changeType, filepath.Ext(filePath)},
	}

	return g.store.RecordObservation(obs)
}

// ObserveDecision records an observation about a decision made.
func (g *Guardian) ObserveDecision(sessionID, decision, rationale string) error {
	obs := &Observation{
		SessionID: sessionID,
		Timestamp: time.Now(),
		Type:      ObsDecisionMade,
		Subject:   "decision",
		Content:   fmt.Sprintf("Decision: %s\nRationale: %s", decision, rationale),
		Relevance: 0.8, // Decisions are always relevant
		Tags:      []string{"decision"},
	}

	return g.store.RecordObservation(obs)
}

func (g *Guardian) calculateRelevance(text string) float64 {
	if g.vision == nil {
		return 0.5
	}

	// Simple keyword matching for relevance
	// In production, this would use embeddings
	textLower := strings.ToLower(text)
	matches := 0
	total := 0

	checkKeywords := func(source string) {
		words := strings.Fields(strings.ToLower(source))
		for _, word := range words {
			if len(word) > 3 { // Skip short words
				total++
				if strings.Contains(textLower, word) {
					matches++
				}
			}
		}
	}

	checkKeywords(g.vision.Mission)
	checkKeywords(g.vision.Problem)
	checkKeywords(g.vision.VisionStmt)

	if total == 0 {
		return 0.5
	}
	return float64(matches) / float64(total)
}

func (g *Guardian) calculatePathRelevance(path string) float64 {
	for _, highImpact := range g.config.HighImpactPaths {
		if matched, _ := filepath.Match(highImpact, path); matched {
			return 0.9
		}
		if strings.HasPrefix(path, strings.TrimSuffix(highImpact, "*")) {
			return 0.9
		}
	}
	return 0.5
}

// =============================================================================
// PERIODIC AND INTELLIGENT CHECKS
// =============================================================================

// ShouldCheckNow determines if an alignment check should be performed.
func (g *Guardian) ShouldCheckNow(trigger AlignmentTrigger, filePaths []string) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if g.vision == nil {
		return false
	}

	switch trigger {
	case TriggerPhaseGate:
		return g.config.EnablePhaseGates

	case TriggerPeriodic:
		if !g.config.EnablePeriodicCheck {
			return false
		}
		if g.state != nil && g.state.TasksSinceCheck >= g.config.PeriodicCheckInterval {
			return true
		}
		return false

	case TriggerHighImpact:
		if !g.config.EnableHighImpact {
			return false
		}
		for _, path := range filePaths {
			for _, pattern := range g.config.HighImpactPaths {
				if matched, _ := filepath.Match(pattern, path); matched {
					return true
				}
				if strings.HasPrefix(path, strings.TrimSuffix(pattern, "*")) {
					return true
				}
			}
		}
		return false

	case TriggerManual:
		return true

	default:
		return false
	}
}

// OnTaskComplete should be called after each task completion.
// It increments the counter and may trigger a periodic check.
func (g *Guardian) OnTaskComplete(ctx context.Context, taskDesc string) (*AlignmentCheck, error) {
	count, err := g.store.IncrementTaskCount()
	if err != nil {
		return nil, err
	}

	g.mu.Lock()
	if g.state != nil {
		g.state.TasksSinceCheck = count
	}
	g.mu.Unlock()

	// Check if periodic check is due
	if g.ShouldCheckNow(TriggerPeriodic, nil) {
		return g.CheckAlignment(ctx, TriggerPeriodic, taskDesc, "Periodic alignment check")
	}

	return nil, nil
}

// =============================================================================
// UTILITY
// =============================================================================

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
