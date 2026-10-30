package northstar

import (
	"codenerd/internal/projectdoc"
	"context"
	"errors"
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
	"northstar_mitigation_state", "northstar_mitigation_evidence", "northstar_risk_waiver",
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
		logging.Get(logging.CategoryNorthstar).Info("Northstar Guardian initialized with vision: %s", types.Ellipsize(vision.Mission, 50))
	} else {
		visionPath := "<unknown>"
		if g.store != nil {
			// The store path is a SQLite DB nobody edits by hand. Point at
			// the import surface the operator actually authors.
			visionPath = filepath.Join(filepath.Dir(g.store.Path()), VisionJSONFileName)
		}
		logging.Get(logging.CategoryNorthstar).Warn(
			"Northstar Guardian initialized without a vision — all alignment checks will be skipped. Configure a vision at %s (or run /northstar) to enable enforcement.",
			visionPath,
		)
	}

	// A failed projection is logged by refreshKernelFacts; Initialize has
	// loaded the authority either way.
	_ = g.refreshKernelFacts()

	return nil
}

// refreshKernelFacts replaces the kernel's northstar_* facts with the current
// vision's projection. It returns every retraction and assertion failure
// together: a kernel left holding stale or partial facts is a projection
// failure the caller has to be able to report.
func (g *Guardian) refreshKernelFacts() error {
	g.mu.RLock()
	kernel := g.kernel
	vision := cloneVision(g.vision)
	g.mu.RUnlock()

	if kernel == nil {
		return nil
	}

	var errs []error

	// Retract all existing northstar facts
	type BatchRetractor interface {
		RemoveFactsByPredicateSet(predicates map[string]struct{}) error
	}

	if br, ok := kernel.(BatchRetractor); ok {
		if err := br.RemoveFactsByPredicateSet(northstarPredicatesMap); err != nil {
			errs = append(errs, fmt.Errorf("retract northstar facts: %w", err))
		}
	} else {
		for _, p := range northstarPredicatesList {
			if err := kernel.Retract(p); err != nil {
				errs = append(errs, fmt.Errorf("retract %s: %w", p, err))
			}
		}
	}

	if vision == nil {
		return errors.Join(errs...)
	}

	// Assert new facts
	type BatchAsserter interface {
		AssertBatch(facts []types.Fact) error
	}

	facts := vision.ToFacts()
	if ba, ok := kernel.(BatchAsserter); ok {
		if err := ba.AssertBatch(facts); err != nil {
			errs = append(errs, fmt.Errorf("assert northstar facts: %w", err))
		}
	} else {
		for _, fact := range facts {
			if err := kernel.Assert(fact); err != nil {
				errs = append(errs, fmt.Errorf("assert %s: %w", fact.Predicate, err))
			}
		}
	}
	if len(errs) > 0 {
		logging.Get(logging.CategoryNorthstar).Warn("Northstar kernel projection incomplete: %v", errors.Join(errs...))
	}
	return errors.Join(errs...)
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

// UpdateVision replaces the stored vision unconditionally. Callers that edit
// a vision they read earlier use UpdateVisionFromBase instead, which refuses to
// overwrite a vision that changed in between.
func (g *Guardian) UpdateVision(vision *Vision) error {
	if vision == nil {
		return fmt.Errorf("vision is nil")
	}

	if err := g.store.SaveVision(vision); err != nil {
		return err
	}

	stateErr, projection := g.publishVision(vision)
	for _, err := range projection {
		logging.Get(logging.CategoryNorthstar).Warn("Vision projection failed: %v", err)
	}
	if stateErr != nil {
		return stateErr
	}

	logging.Get(logging.CategoryNorthstar).Info("Vision updated: %s", types.Ellipsize(vision.Mission, 50))
	return nil
}

// publishVision installs a vision the store has already committed: the
// guardian's cached copy and state, the JSON and .mg export surfaces, and the
// kernel facts. The commit is not undone by a failure here, so failures are
// returned for the caller to report, not hidden: stateErr is a failed state
// refresh, projection the failed exports and kernel writes.
func (g *Guardian) publishVision(vision *Vision) (stateErr error, projection []error) {
	state, err := g.store.GetState()
	if err != nil {
		stateErr = fmt.Errorf("failed to refresh guardian state: %w", err)
	}

	g.mu.Lock()
	g.vision = cloneVision(vision)
	if err == nil {
		g.state = cloneGuardianState(state)
	}
	g.mu.Unlock()

	// Push the new vision back out to the operator-visible surfaces in the same
	// call. Without this the store and .nerd/northstar.json diverge the instant
	// anything updates the vision programmatically, and the next boot's
	// reconciliation has to guess by mtime which half is real.
	nerdDir := filepath.Dir(g.store.Path())
	if _, err := WriteVisionJSON(nerdDir, vision); err != nil {
		projection = append(projection, fmt.Errorf("export %s: %w", VisionJSONFileName, err))
	}
	if err := WriteVisionMangle(nerdDir, vision); err != nil {
		projection = append(projection, fmt.Errorf("export %s: %w", VisionMangleFileName, err))
	}
	if err := g.refreshKernelFacts(); err != nil {
		projection = append(projection, err)
	}
	return stateErr, projection
}

// =============================================================================
// ALIGNMENT CHECKING
// =============================================================================

// CheckAlignment assesses prose against project and explicitly supplied module scope.
func (g *Guardian) CheckAlignment(ctx context.Context, trigger AlignmentTrigger, subject, context string, scope AlignmentScope) (*AlignmentCheck, error) {
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

	normalized, issues := normalizeAlignmentScope(scope)
	check.Coverage = &AlignmentCoverage{Scope: normalized, Outcome: ScopeUnknown, VisionDigest: VisionDigest(vision), Issues: issues}

	// No vision: nothing to align against. Skipped is non-passing and carries
	// no score; a 1.0 here used to read as "perfectly aligned".
	if vision == nil {
		check.Result = AlignmentSkipped
		check.Explanation = "No vision defined - skipping alignment check"
		check.Duration = time.Since(startTime)
		g.persistAlignmentOutcome(check, subject)
		return check, nil
	}

	// Unresolved module scope does not prevent project alignment. Coverage
	// retains its unknown outcome and reason; the model still judges the
	// subject against the project vision without module refinements.
	coverage, module := g.resolveAlignmentScope(scope, vision)
	check.Coverage = coverage

	// No LLM: no assessment happened. This used to be "passed, 0.8, assuming
	// aligned", which let every unassessed subject through every gate.
	if llm == nil {
		markUnknown(check, "LLM not available - alignment was not assessed")
		check.Duration = time.Since(startTime)
		g.persistAlignmentOutcome(check, subject)
		g.logCheck(check, "alignment not assessed")
		return check, nil
	}

	// Build the alignment prompt: no compiled prompt, no assessment.
	systemPrompt, userPrompt, err := g.buildAlignmentPrompts(ctx, vision, subject, context, alignmentScopeSection(coverage), module)
	if err != nil {
		markUnknown(check, fmt.Sprintf("Alignment prompt unavailable: %v", err))
		check.Duration = time.Since(startTime)
		g.persistAlignmentOutcome(check, subject)
		g.logCheck(check, "alignment prompt unavailable")
		return check, nil
	}

	response, err := g.complete(ctx, llm, systemPrompt, userPrompt)
	if err != nil {
		markUnknown(check, fmt.Sprintf("Alignment check could not complete: %v", err))
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
	fields := map[string]any{
		"check_id":    check.ID,
		"subject":     check.Subject,
		"trigger":     string(check.Trigger),
		"result":      string(check.Result),
		"score":       check.Score,
		"duration_ms": check.Duration.Milliseconds(),
		"suggestions": len(check.Suggestions),
		"has_vision":  g.HasVision(),
		"explanation": types.Ellipsize(check.Explanation, 200),
	}
	if c := check.Coverage; c != nil {
		fields["workspace"] = c.Scope.Workspace
		fields["scope"] = c.Scope.Disposition
		fields["scope_outcome"] = c.Outcome
		fields["scope_complete"] = c.Complete
		fields["selected_modules"] = len(c.Modules)
		fields["unmatched_paths"] = len(c.UnmatchedPaths)
		fields["scope_issues"] = len(c.Issues)
		fields["scope_issue_reasons"] = c.Issues
		fields["vision_digest"] = c.VisionDigest
		fields["module_digest"] = c.ModuleDigest
	}
	logging.Get(logging.CategoryNorthstar).StructuredLog("INFO", msg, fields)
}

// buildAlignmentPrompts returns one check's model input: the system prompt
// compiled from the guardian atoms (with the module-refinement rule exactly
// when scoped modules govern the request) and the user message carrying
// the project vision, scope roles, module refinements, subject and context.
func (g *Guardian) buildAlignmentPrompts(ctx context.Context, vision *Vision, subject, extra, scopeSection, module string) (systemPrompt, userPrompt string, err error) {
	systemPrompt, err = compileAlignmentSystemPrompt(ctx, module != "")
	if err != nil {
		return "", "", err
	}
	var sb strings.Builder
	sb.WriteString(alignmentVisionSection(vision))
	sb.WriteString(scopeSection)
	sb.WriteString(module)
	sb.WriteString("## Subject to Evaluate\n")
	sb.WriteString(subject)
	sb.WriteString("\n")
	if extra != "" {
		sb.WriteString("\n## Additional Context\n")
		sb.WriteString(extra)
		sb.WriteString("\n")
	}
	return systemPrompt, sb.String(), nil
}

// alignmentVisionSection renders the project vision the check judges against.
func alignmentVisionSection(vision *Vision) string {
	var sb strings.Builder
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
			// The status travels with the prose so the judge never reads a
			// proposed mitigation as one that exists.
			if r.Mitigation != "" {
				sb.WriteString(fmt.Sprintf(" | Mitigation (%s): %s", r.EffectiveMitigationStatus(), r.Mitigation))
			} else if r.EffectiveMitigationStatus() == MitigationWaived {
				sb.WriteString(" | Waived, not mitigated")
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
	return sb.String()
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
		// Stored whole: an observation is evidence a later drift check reads,
		// and a 500-byte prefix of a result is not what the task returned.
		Content:   fmt.Sprintf("Task: %s\nResult: %s", taskDesc, result),
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
		return g.CheckAlignment(ctx, TriggerPeriodic, taskDesc, "Periodic alignment check", AlignmentScope{Disposition: ScopeUnresolved})
	}

	return nil, nil
}

// =============================================================================
// UTILITY
// =============================================================================

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
	if v.Capabilities != nil {
		clone.Capabilities = make([]Capability, len(v.Capabilities))
		for i, c := range v.Capabilities {
			c.Serves = append([]string(nil), c.Serves...)
			clone.Capabilities[i] = c
		}
	}
	if v.Risks != nil {
		clone.Risks = make([]Risk, len(v.Risks))
		for i, r := range v.Risks {
			clone.Risks[i] = cloneRisk(r)
		}
	}
	if v.Requirements != nil {
		clone.Requirements = make([]Requirement, len(v.Requirements))
		for i, r := range v.Requirements {
			r.Supports = append([]string(nil), r.Supports...)
			r.Addresses = append([]string(nil), r.Addresses...)
			clone.Requirements[i] = r
		}
	}
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

	// A nerd.md critical: entry ("internal/core") covers everything under it,
	// on whole path segments.
	_, ok := projectdoc.MatchPaths([]string{pattern}, path)
	return ok
}
