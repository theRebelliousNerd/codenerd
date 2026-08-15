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
	"codenerd/internal/types"
)

// northstarPredicatesList is every predicate Vision.ToFacts can emit. It is the
// retract set for refreshKernelFacts: a predicate missing here survives a vision
// change and leaves the kernel asserting a fact the vision no longer contains.
var northstarPredicatesList = []string{
	"northstar_mission", "northstar_problem", "northstar_vision",
	"northstar_persona", "northstar_pain_point", "northstar_need",
	"northstar_capability", "northstar_serves",
	"northstar_risk", "northstar_mitigation", "northstar_mitigation_text",
	"northstar_requirement", "northstar_supports", "northstar_addresses",
	"northstar_constraint", "northstar_defined",
}

var northstarPredicatesMap = func() map[string]struct{} {
	m := make(map[string]struct{}, len(northstarPredicatesList))
	for _, p := range northstarPredicatesList {
		m[p] = struct{}{}
	}
	return m
}()

// KernelClient provides an interface for asserting facts into the Mangle kernel.
type KernelClient interface {
	Assert(fact types.Fact) error
	Retract(predicate string) error
}

// Guardian is the Northstar vision guardian.
// It monitors project activity and ensures alignment with the defined vision.
// Guardian is the Northstar vision guardian.
// It monitors project activity and ensures alignment with the defined vision.
type Guardian struct {
	store    *Store
	config   GuardianConfig
	llm      LLMClient
	kernel   KernelClient
	querier  FactQuerier
	embedder Embedder
	mu       sync.RWMutex

	// Runtime state
	state  *GuardianState
	vision *Vision

	warnAlignmentModelIgnored sync.Once

	// registryKey is set when this Guardian is owned by the process-wide
	// registry (see registry.go); it is "" for directly constructed guardians.
	registryKey string
}

// LLMClient interface for alignment checks.
type LLMClient interface {
	CompleteWithSystem(ctx context.Context, system, user string) (string, error)
}

// ModelSelectingLLMClient is the optional capability a client advertises when it
// can answer one completion with an explicitly chosen model without mutating
// shared client state.
//
// GuardianConfig.AlignmentModel exists so an operator can run alignment on a
// stronger (or cheaper) model than the session's default. It was previously
// declared, documented, serialized -- and read by nothing, so setting it did
// nothing at all. It is now honoured through this interface. Clients that
// cannot do per-call model selection are detected and warned about once, rather
// than having the guardian silently mutate a client other subsystems share.
type ModelSelectingLLMClient interface {
	CompleteWithSystemModel(ctx context.Context, model, system, user string) (string, error)
}

// NewGuardian creates a new Northstar Guardian.
func NewGuardian(store *Store, config GuardianConfig) *Guardian {
	return &Guardian{
		store:  store,
		config: NormalizeGuardianConfig(config),
		state:  &GuardianState{OverallAlignment: 1.0},
	}
}

// NormalizeGuardianConfig repairs a threshold set that does not satisfy
// block <= failure <= warning, and clamps every threshold into [0,1].
//
// classifyScore walks the thresholds in order warning -> failure -> block and
// returns on the first match. An out-of-order set therefore does not error, it
// silently makes a band unreachable: with warning=0.3 and failure=0.7 every
// score >= 0.3 classifies as passed and nothing is ever marked failed. That is
// a guardian that reports "aligned" for work it was configured to block, which
// is the worst possible failure direction for this subsystem. Repair loudly
// rather than reject, so a bad config degrades to the defaults' ordering
// instead of taking the whole boot down.
func NormalizeGuardianConfig(config GuardianConfig) GuardianConfig {
	clamp := func(v, fallback float64) float64 {
		if v < 0 || v > 1 {
			return fallback
		}
		return v
	}
	defaults := GuardianConfig{WarningThreshold: 0.7, FailureThreshold: 0.5, BlockThreshold: 0.3}
	original := config

	config.WarningThreshold = clamp(config.WarningThreshold, defaults.WarningThreshold)
	config.FailureThreshold = clamp(config.FailureThreshold, defaults.FailureThreshold)
	config.BlockThreshold = clamp(config.BlockThreshold, defaults.BlockThreshold)

	// Sort into the only ordering classifyScore can act on.
	if config.FailureThreshold > config.WarningThreshold {
		config.FailureThreshold, config.WarningThreshold = config.WarningThreshold, config.FailureThreshold
	}
	if config.BlockThreshold > config.FailureThreshold {
		config.BlockThreshold, config.FailureThreshold = config.FailureThreshold, config.BlockThreshold
	}
	if config.FailureThreshold > config.WarningThreshold {
		config.FailureThreshold, config.WarningThreshold = config.WarningThreshold, config.FailureThreshold
	}

	if config.PeriodicCheckInterval <= 0 {
		config.PeriodicCheckInterval = 5
	}

	if original.WarningThreshold != config.WarningThreshold ||
		original.FailureThreshold != config.FailureThreshold ||
		original.BlockThreshold != config.BlockThreshold {
		logging.Get(logging.CategoryNorthstar).Warn(
			"Guardian thresholds were invalid (block=%.2f failure=%.2f warning=%.2f); repaired to block=%.2f failure=%.2f warning=%.2f",
			original.BlockThreshold, original.FailureThreshold, original.WarningThreshold,
			config.BlockThreshold, config.FailureThreshold, config.WarningThreshold)
	}

	return config
}

// SetLLMClient sets the LLM client for alignment checks.
func (g *Guardian) SetLLMClient(client LLMClient) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.llm = client
}

// SetParentKernel sets the Mangle kernel for fact injection.
func (g *Guardian) SetParentKernel(client KernelClient) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.kernel = client
}

// SetQuerier sets the FactQuerier for module northstar resolution.
func (g *Guardian) SetQuerier(q FactQuerier) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.querier = q
}

// Initialize loads the vision and state from the store.
//
// It first reconciles the store with .nerd/northstar.json (see bridge.go). That
// happens here rather than at each call site because every consumer of a vision
// -- chat boot, shared boot, /alignment, the campaign risk gate, the CLI --
// goes through Initialize, and the dual-store divergence bug was precisely that
// each of them remembered a different half of the truth.
func (g *Guardian) Initialize() error {
	if g.store != nil {
		if _, err := SyncVisionAuthority(g.store, filepath.Dir(g.store.Path())); err != nil {
			// Reconciliation failure must not block the guardian: the store is
			// still a usable authority on its own.
			logging.Get(logging.CategoryNorthstar).Warn("Vision reconciliation failed: %v", err)
		}
	}

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
		visionPath := "<unknown>"
		if g.store != nil {
			visionPath = g.store.Path()
		}
		logging.Get(logging.CategoryNorthstar).Warn(
			"Northstar Guardian initialized without a vision file — all alignment checks will be skipped. Configure vision at %s to enable enforcement.",
			visionPath,
		)
	}

	g.refreshKernelFacts()

	return nil
}

func (g *Guardian) refreshKernelFacts() {
	g.mu.RLock()
	kernel := g.kernel
	vision := cloneVision(g.vision)
	g.mu.RUnlock()

	if kernel == nil {
		return
	}

	// Retract all existing northstar facts
	type BatchRetractor interface {
		RemoveFactsByPredicateSet(predicates map[string]struct{}) error
	}

	if br, ok := kernel.(BatchRetractor); ok {
		_ = br.RemoveFactsByPredicateSet(northstarPredicatesMap)
	} else {
		for _, p := range northstarPredicatesList {
			_ = kernel.Retract(p)
		}
	}

	if vision == nil {
		return
	}

	// Assert new facts
	type BatchAsserter interface {
		AssertBatch(facts []types.Fact) error
	}

	facts := vision.ToFacts()
	if ba, ok := kernel.(BatchAsserter); ok {
		if err := ba.AssertBatch(facts); err != nil {
			logging.Get(logging.CategoryNorthstar).Debug("Failed to assert batch northstar facts: %v", err)
		}
	} else {
		for _, fact := range facts {
			if err := kernel.Assert(fact); err != nil {
				logging.Get(logging.CategoryNorthstar).Debug("Failed to assert northstar fact %s: %v", fact.Predicate, err)
			}
		}
	}
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

	// Push the new vision back out to the operator-visible surfaces in the same
	// call. Without this the store and .nerd/northstar.json diverge the instant
	// anything updates the vision programmatically, and the next boot's
	// reconciliation has to guess by mtime which half is real.
	if g.store != nil {
		nerdDir := filepath.Dir(g.store.Path())
		if _, err := WriteVisionJSON(nerdDir, vision); err != nil {
			logging.Get(logging.CategoryNorthstar).Warn("Failed to export vision JSON: %v", err)
		}
		if err := WriteVisionMangle(nerdDir, vision); err != nil {
			logging.Get(logging.CategoryNorthstar).Warn("Failed to export vision facts: %v", err)
		}
	}

	g.refreshKernelFacts()

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
		ID:        newID("check"),
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
	systemPrompt := g.buildAlignmentSystemPrompt(vision, subject)
	userPrompt := g.buildAlignmentUserPrompt(subject, context)

	response, err := g.complete(ctx, llm, systemPrompt, userPrompt)
	if err != nil {
		check.Result = AlignmentWarning
		check.Score = 0.7
		check.Explanation = fmt.Sprintf("Failed to complete alignment check: %v", err)
		check.Duration = time.Since(startTime)
		g.persistAlignmentOutcome(check, subject)
		g.logCheck(check, "alignment check failed")
		return check, nil
	}

	// Parse the response
	g.parseAlignmentResponse(response, check)
	check.Duration = time.Since(startTime)

	g.persistAlignmentOutcome(check, subject)
	g.logCheck(check, "alignment check")

	return check, nil
}

// complete routes the alignment completion through the configured
// AlignmentModel when the injected client can select a model per call.
func (g *Guardian) complete(ctx context.Context, llm LLMClient, systemPrompt, userPrompt string) (string, error) {
	g.mu.RLock()
	model := strings.TrimSpace(g.config.AlignmentModel)
	g.mu.RUnlock()

	if model == "" {
		return llm.CompleteWithSystem(ctx, systemPrompt, userPrompt)
	}
	if selector, ok := llm.(ModelSelectingLLMClient); ok {
		return selector.CompleteWithSystemModel(ctx, model, systemPrompt, userPrompt)
	}
	g.warnAlignmentModelIgnored.Do(func() {
		logging.Get(logging.CategoryNorthstar).Warn(
			"GuardianConfig.AlignmentModel=%q is set but the injected LLM client cannot select a model per call; alignment checks run on the client's default model",
			model)
	})
	return llm.CompleteWithSystem(ctx, systemPrompt, userPrompt)
}

// logCheck emits the alignment outcome with structured fields.
//
// The previous single Info line interpolated subject/result/score into prose,
// so nothing downstream could filter on "blocked" or aggregate scores without
// parsing English. Fields make the guardian's decisions queryable in the log
// pipeline; the human-readable line is kept as the message.
func (g *Guardian) logCheck(check *AlignmentCheck, msg string) {
	if check == nil {
		return
	}
	logging.Get(logging.CategoryNorthstar).StructuredLog("INFO", msg, map[string]any{
		"check_id":    check.ID,
		"subject":     check.Subject,
		"trigger":     string(check.Trigger),
		"result":      string(check.Result),
		"score":       check.Score,
		"duration_ms": check.Duration.Milliseconds(),
		"suggestions": len(check.Suggestions),
		"has_vision":  g.HasVision(),
		"explanation": truncate(check.Explanation, 200),
	})
}

func (g *Guardian) buildAlignmentSystemPrompt(vision *Vision, subject ...string) string {
	var sb strings.Builder
	sb.WriteString(AlignmentAtom(atomGuardianRole))
	sb.WriteString("\n\n")
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

	// Module northstar refinement: when the subject lives inside a module that
	// declares its own purpose, include that purpose and its requirements
	// ALONGSIDE the project vision. A module refines the project northstar; it
	// must never be able to opt out of it. When no querier is set or no module
	// governs the subject, behaviour is exactly as before.
	subjectStr := ""
	if len(subject) > 0 {
		subjectStr = subject[0]
	}
	var q FactQuerier
	g.mu.RLock()
	q = g.querier
	g.mu.RUnlock()
	if q != nil && strings.TrimSpace(subjectStr) != "" {
		mod, err := ModuleForPath(q, subjectStr)
		if err == nil && mod != "" {
			purpose, err := EffectiveModulePurpose(q, mod)
			if err == nil && purpose != "" {
				sb.WriteString(fmt.Sprintf("## Module Northstar (%s)\n", mod))
				sb.WriteString(fmt.Sprintf("**Purpose:** %s\n\n", purpose))
				sb.WriteString(AlignmentAtom(atomGuardianModuleRefinement))
				sb.WriteString("\n\n")
				reqs, err := ModuleRequirementsFor(q, mod)
				if err == nil && len(reqs) > 0 {
					sb.WriteString("### Module Requirements\n")
					for _, r := range reqs {
						sev := strings.TrimSpace(r.Severity)
						if sev != "" {
							sev = strings.TrimPrefix(sev, "/")
							sb.WriteString(fmt.Sprintf("- [%s/%s] %s\n", r.ID, sev, r.Statement))
						} else {
							sb.WriteString(fmt.Sprintf("- [%s] %s\n", r.ID, r.Statement))
						}
					}
					sb.WriteString("\n")
				}
			}
		}
	}

	sb.WriteString(AlignmentAtom(atomGuardianTask))
	sb.WriteString("\n\n")
	sb.WriteString(AlignmentAtom(atomGuardianOutputContract))
	sb.WriteString("\n")

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
	sb.WriteString(AlignmentAtom(atomGuardianUserInstruction))
	return sb.String()
}

func (g *Guardian) parseAlignmentResponse(response string, check *AlignmentCheck) {
	// Default values
	check.Score = 0.7
	check.Result = AlignmentWarning
	check.Explanation = "Unable to parse alignment response"
	explicitResult := false

	lines := strings.SplitSeq(response, "\n")
	for line := range lines {
		line = strings.TrimSpace(line)
		line = strings.ReplaceAll(line, "\"", "")
		line = strings.TrimSuffix(line, ",")

		if strings.HasPrefix(line, "SCORE:") {
			var score float64
			fmt.Sscanf(strings.TrimPrefix(line, "SCORE:"), "%f", &score)
			if score >= 0 && score <= 1 {
				check.Score = score
			}
		} else if after, ok := strings.CutPrefix(line, "RESULT:"); ok {
			result := strings.TrimSpace(after)
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
		} else if after, ok := strings.CutPrefix(line, "EXPLANATION:"); ok {
			check.Explanation = strings.TrimSpace(after)
		} else if after, ok := strings.CutPrefix(line, "SUGGESTIONS:"); ok {
			sugStr := strings.TrimSpace(after)
			if sugStr != "none" && sugStr != "" {
				for s := range strings.SplitSeq(sugStr, ",") {
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
		words := strings.FieldsSeq(strings.ToLower(source))
		for word := range words {
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

	visionScore := 0.5
	if total > 0 {
		visionScore = float64(matches) / float64(total)
	}

	// Ingested project documents are a second, independent opinion on
	// relevance. Three vision sentences are a very small vocabulary: work that
	// is obviously on-mission but phrased differently scores near zero against
	// them. Take the stronger of the two signals rather than averaging, because
	// a miss on one channel is evidence of nothing, while a hit on either is
	// evidence of relevance.
	if docScore, ok := g.DocumentRelevance(text); ok && docScore > visionScore {
		return docScore
	}
	return visionScore
}

func (g *Guardian) calculatePathRelevance(path string) float64 {
	for _, highImpact := range g.config.HighImpactPaths {
		if matchesHighImpactPath(highImpact, path) {
			return 0.9
		}
	}
	// Module northstar tier (NS-6): deterministic, LLM-free signal that
	// decides WHEN to spend an expensive LLM alignment check. A module's
	// own severity is the cheapest honest signal available.
	//
	// Must be checked AFTER HighImpactPath so existing behaviour never
	// regresses, and must fall through to the original 0.5 when the
	// querier is absent, no module governs the path, or any lookup
	// errors. No background cache, no LLM call.
	var q FactQuerier
	g.mu.RLock()
	q = g.querier
	g.mu.RUnlock()
	if q == nil {
		return 0.5
	}
	mod, err := ModuleForPath(q, path)
	if err != nil || mod == "" {
		return 0.5
	}
	reqs, err := ModuleRequirementsFor(q, mod)
	if err != nil {
		return 0.5
	}
	if len(reqs) > 0 {
		hasBlocker := false
		hasMajor := false
		for _, r := range reqs {
			sev := strings.TrimSpace(strings.ToLower(r.Severity))
			sev = strings.TrimPrefix(sev, "/")
			switch sev {
			case "blocker":
				hasBlocker = true
			case "major":
				hasMajor = true
			}
		}
		if hasBlocker {
			return 0.9
		}
		if hasMajor {
			return 0.7
		}
		// Only /minor or /unspecified (including empty/unknown) requirements.
		return 0.6
	}
	// No requirements: module with a purpose but no requirements scores 0.6
	// (it cared enough to declare something). Lookup error falls back to 0.5.
	purpose, err := EffectiveModulePurpose(q, mod)
	if err != nil {
		return 0.5
	}
	if strings.TrimSpace(purpose) != "" {
		return 0.6
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
