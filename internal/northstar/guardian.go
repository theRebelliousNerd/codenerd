package northstar

import (
	"context"
	"fmt"
	pathpkg "path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"codenerd/internal/logging"
)

// Guardian is the Northstar vision guardian.
// It monitors project activity and ensures alignment with the defined vision.
type Guardian struct {
	store  *Store
	config GuardianConfig
	llm    LLMClient
	mu     sync.RWMutex

	// Runtime state
	state  *GuardianState
	vision *Vision
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
		state:  &GuardianState{OverallAlignment: 1.0},
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
	vision, err := g.store.LoadVision()
	if err != nil {
		return fmt.Errorf("failed to load vision: %w", err)
	}

	state, err := g.store.GetState()
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	g.mu.Lock()
	g.vision = cloneVision(vision)
	g.state = cloneGuardianState(state)
	g.mu.Unlock()

	if vision != nil {
		logging.Get(logging.CategoryNorthstar).Info("Northstar Guardian initialized with vision: %s", truncate(vision.Mission, 50))
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
	return cloneVision(g.vision)
}

// GetState returns the current guardian state.
func (g *Guardian) GetState() *GuardianState {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return cloneGuardianState(g.state)
}

// UpdateVision updates the stored vision.
func (g *Guardian) UpdateVision(vision *Vision) error {
	if vision == nil {
		return fmt.Errorf("vision is nil")
	}

	if err := g.store.SaveVision(vision); err != nil {
		return err
	}

	state, err := g.store.GetState()
	if err != nil {
		return fmt.Errorf("failed to refresh guardian state: %w", err)
	}

	g.mu.Lock()
	g.vision = cloneVision(vision)
	g.state = cloneGuardianState(state)
	g.mu.Unlock()

	logging.Get(logging.CategoryNorthstar).Info("Vision updated: %s", truncate(vision.Mission, 50))
	return nil
}

// =============================================================================
// ALIGNMENT CHECKING
// =============================================================================

// CheckAlignment performs an alignment check for the given subject.
func (g *Guardian) CheckAlignment(ctx context.Context, trigger AlignmentTrigger, subject, context string) (*AlignmentCheck, error) {
	g.mu.RLock()
	vision := cloneVision(g.vision)
	llm := g.llm
	g.mu.RUnlock()

	startTime := time.Now()
	check := &AlignmentCheck{
		ID:        fmt.Sprintf("check-%d", time.Now().UnixNano()),
		Timestamp: startTime,
		Trigger:   trigger,
		Subject:   subject,
		Context:   context,
	}

	// If no vision, skip
	if vision == nil {
		check.Result = AlignmentSkipped
		check.Score = 1.0
		check.Explanation = "No vision defined - skipping alignment check"
		check.Duration = time.Since(startTime)
		g.persistAlignmentOutcome(check, subject)
		return check, nil
	}

	// If no LLM, do basic check
	if llm == nil {
		check.Result = AlignmentPassed
		check.Score = 0.8
		check.Explanation = "LLM not available for deep analysis - assuming aligned"
		check.Duration = time.Since(startTime)
		g.persistAlignmentOutcome(check, subject)
		return check, nil
	}

	// Build the alignment prompt
	systemPrompt := g.buildAlignmentSystemPrompt(vision)
	userPrompt := g.buildAlignmentUserPrompt(subject, context)

	response, err := llm.CompleteWithSystem(ctx, systemPrompt, userPrompt)
	if err != nil {
		check.Result = AlignmentWarning
		check.Score = 0.7
		check.Explanation = fmt.Sprintf("Failed to complete alignment check: %v", err)
		check.Duration = time.Since(startTime)
		g.persistAlignmentOutcome(check, subject)
		return check, nil
	}

	// Parse the response
	g.parseAlignmentResponse(response, check)
	check.Duration = time.Since(startTime)

	g.persistAlignmentOutcome(check, subject)

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

	if len(vision.Requirements) > 0 {
		sb.WriteString("## Requirements\n")
		for _, r := range vision.Requirements {
			sb.WriteString(fmt.Sprintf("- [%s/%s] %s\n", r.Priority, r.Type, r.Description))
		}
		sb.WriteString("\n")
	}

	if len(vision.Risks) > 0 {
		sb.WriteString("## Risks To Avoid\n")
		for _, r := range vision.Risks {
			sb.WriteString(fmt.Sprintf("- [%s/%s] %s", r.Likelihood, r.Impact, r.Description))
			if r.Mitigation != "" {
				sb.WriteString(fmt.Sprintf(" | Mitigation: %s", r.Mitigation))
			}
			sb.WriteString("\n")
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
	explicitResult := false

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
				explicitResult = true
			case "warning":
				check.Result = AlignmentWarning
				explicitResult = true
			case "failed":
				check.Result = AlignmentFailed
				explicitResult = true
			case "blocked":
				check.Result = AlignmentBlocked
				explicitResult = true
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

	// Derive result from score only when the model did not explicitly provide one.
	if !explicitResult {
		check.Result = g.classifyScore(check.Score)
	}
}

func (g *Guardian) scoreToSeverity(score float64) DriftSeverity {
	switch g.classifyScore(score) {
	case AlignmentPassed:
		return DriftMinor
	case AlignmentWarning:
		return DriftModerate
	case AlignmentFailed:
		return DriftMajor
	default:
		return DriftCritical
	}
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

	obs.Relevance = g.calculateRelevance(taskDesc + " " + result)

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
	vision := g.GetVision()
	if vision == nil {
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

	checkKeywords(vision.Mission)
	checkKeywords(vision.Problem)
	checkKeywords(vision.VisionStmt)

	if total == 0 {
		return 0.5
	}
	return float64(matches) / float64(total)
}

func (g *Guardian) calculatePathRelevance(path string) float64 {
	for _, highImpact := range g.config.HighImpactPaths {
		if matchesHighImpactPath(highImpact, path) {
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
				if matchesHighImpactPath(pattern, path) {
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

func (g *Guardian) persistAlignmentOutcome(check *AlignmentCheck, subject string) {
	if err := g.store.RecordAlignmentCheck(check); err != nil {
		logging.Get(logging.CategoryNorthstar).Debug("Failed to record alignment check: %v", err)
		return
	}

	if err := g.refreshState(); err != nil {
		logging.Get(logging.CategoryNorthstar).Debug("Failed to refresh guardian state: %v", err)
	}

	if check.Result != AlignmentFailed && check.Result != AlignmentBlocked {
		return
	}

	drift := &DriftEvent{
		Timestamp:    time.Now(),
		Severity:     g.scoreToSeverity(check.Score),
		Category:     "alignment",
		Description:  check.Explanation,
		Evidence:     []string{subject},
		RelatedCheck: check.ID,
	}
	if err := g.store.RecordDriftEvent(drift); err != nil {
		logging.Get(logging.CategoryNorthstar).Debug("Failed to record drift event: %v", err)
		return
	}

	if err := g.refreshState(); err != nil {
		logging.Get(logging.CategoryNorthstar).Debug("Failed to refresh guardian state after drift update: %v", err)
	}
}

func (g *Guardian) refreshState() error {
	state, err := g.store.GetState()
	if err != nil {
		return err
	}

	g.mu.Lock()
	g.state = cloneGuardianState(state)
	g.mu.Unlock()
	return nil
}

func (g *Guardian) classifyScore(score float64) AlignmentResult {
	if score >= g.config.WarningThreshold {
		return AlignmentPassed
	}
	if score >= g.config.FailureThreshold {
		return AlignmentWarning
	}
	if score >= g.config.BlockThreshold {
		return AlignmentFailed
	}
	return AlignmentBlocked
}

func cloneVision(v *Vision) *Vision {
	if v == nil {
		return nil
	}

	clone := *v
	if len(v.Personas) > 0 {
		clone.Personas = make([]Persona, len(v.Personas))
		for i, persona := range v.Personas {
			clone.Personas[i] = Persona{
				Name:       persona.Name,
				PainPoints: append([]string(nil), persona.PainPoints...),
				Needs:      append([]string(nil), persona.Needs...),
			}
		}
	}
	clone.Capabilities = append([]Capability(nil), v.Capabilities...)
	clone.Risks = append([]Risk(nil), v.Risks...)
	clone.Requirements = append([]Requirement(nil), v.Requirements...)
	clone.Constraints = append([]string(nil), v.Constraints...)
	return &clone
}

func cloneGuardianState(state *GuardianState) *GuardianState {
	if state == nil {
		return nil
	}
	clone := *state
	return &clone
}

func matchesHighImpactPath(pattern, path string) bool {
	normalizedPattern := filepath.ToSlash(pattern)
	normalizedPath := filepath.ToSlash(path)

	if matched, err := pathpkg.Match(normalizedPattern, normalizedPath); err == nil && matched {
		return true
	}

	if strings.HasSuffix(normalizedPattern, "/") {
		return strings.HasPrefix(normalizedPath, normalizedPattern)
	}

	if strings.ContainsAny(normalizedPattern, "*?[") {
		if matched, err := pathpkg.Match(normalizedPattern, pathpkg.Base(normalizedPath)); err == nil && matched {
			return true
		}

		prefix := strings.TrimSuffix(normalizedPattern, "*")
		if prefix != normalizedPattern {
			return strings.HasPrefix(normalizedPath, prefix)
		}
	}

	return normalizedPath == normalizedPattern
}
