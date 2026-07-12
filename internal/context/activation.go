package context

import (
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

// =============================================================================
// Spreading Activation Engine - Enhanced
// =============================================================================
// Implements §8.1: Logic-Directed Context (Spreading Activation)
// Energy flows from the user's intent through the graph of known facts.
//
// This enhanced version adds:
// - Campaign-aware activation (boost facts related to current phase/task)
// - Dependency-based spreading (use symbol_graph and dependency_link)
// - Session-aware activation (boost facts from current session)
// - Verb-based contextual boosting

// ActivationEngine computes activation scores for facts.
//
// All map-backed state is guarded by mu. Concurrent ScoreFacts /
// GetHighActivationFacts (e.g. session save vs live turn) previously raced on
// reverseDependencies/symbolGraph and crashed with concurrent map read/write.
type ActivationEngine struct {
	mu sync.RWMutex

	config CompressorConfig

	// State tracking
	state ActivationState

	// Fact timestamps for recency scoring
	factTimestamps map[string]time.Time

	// Dependency graph for spreading
	dependencies map[string][]string // fact -> depends on

	// Reverse dependency graph (who depends on me)
	reverseDependencies map[string][]string

	// Symbol graph cache (extracted from symbol_graph facts)
	symbolGraph map[string][]string // symbol -> calls

	// Campaign context for phase-aware activation
	campaignContext *CampaignActivationContext

	// Issue context for issue-driven activation (GitHub issues, bug reports, etc.)
	issueContext *IssueActivationContext

	// Back-reference context for follow-up questions referring to previous turns
	backReferenceContext *BackReferenceActivationContext

	// Session tracking
	sessionID      string
	sessionStarted time.Time
	sessionFacts   map[string]bool // Facts added this session

	// Corpus-based priorities (loaded from predicate_corpus.db)
	// These take precedence over config.PredicatePriorities
	corpusPriorities map[string]int

	// Feedback store for learned predicate usefulness
	// Provides historical feedback on which predicates help for each intent type
	feedbackStore *ContextFeedbackStore

	// Current intent verb for intent-specific feedback lookup
	currentIntentVerb string
}

// CampaignActivationContext holds campaign-specific activation state.
type CampaignActivationContext struct {
	CampaignID      string
	CurrentPhase    string
	CurrentTask     string
	PhaseGoals      []string
	RelevantFiles   []string
	RelevantSymbols []string
}

// IssueActivationContext holds issue-specific activation state.
// Used for ANY issue-driven workflow: GitHub issues, bug reports, support tickets,
// Jira tasks, or benchmark instances (SWE-bench, HumanEval, etc.).
//
// This is a GENERAL-PURPOSE context that works with any issue tracking system.
// The key insight: all issue-driven development shares common patterns:
// - A problem description with extractable keywords
// - Files that are likely relevant (mentioned, suspected, or discovered)
// - Error signatures that help identify root cause
// - Tests that validate the fix
type IssueActivationContext struct {
	// IssueID is the unique identifier for this issue.
	// Examples: "GH-1234", "JIRA-5678", "django__django-12345", "BUG-99"
	IssueID string

	// IssueText is the full problem description / issue body.
	// Used for keyword extraction and semantic matching.
	IssueText string

	// Keywords are extracted terms with relevance weights (0.0-1.0).
	// Higher weights indicate stronger relevance to the issue.
	Keywords map[string]float64

	// MentionedFiles are files explicitly referenced in the issue text.
	// These are Tier 1 files with highest relevance.
	MentionedFiles []string

	// TieredFiles maps file paths to their relevance tier (1-4).
	// Tier 1: Directly mentioned in issue
	// Tier 2: High keyword match score
	// Tier 3: Import/dependency neighbors of Tier 1-2
	// Tier 4: Semantic similarity matches
	TieredFiles map[string]int

	// ErrorTypes are error/exception types mentioned in the issue.
	// Examples: "TypeError", "NullPointerException", "ENOENT", "404"
	ErrorTypes []string

	// ExpectedTests are tests that should pass after the fix.
	// For bug fixes: tests that currently fail and should pass.
	// For features: new tests that validate the implementation.
	ExpectedTests []string

	// Source identifies where this issue came from (optional metadata).
	// Examples: "github", "jira", "swebench", "manual"
	Source string
}

// BackReferenceActivationContext holds back-reference activation state.
// Used when users ask follow-up questions referring to previous context.
// Examples: "What was the original error?", "List all solutions we tried"
//
// This enables "infinite context" by boosting facts from referenced turns
// even when they have low recency scores due to being many turns ago.
type BackReferenceActivationContext struct {
	// ReferencedTurnIDs are the turns being referred back to.
	// Multiple turns can be referenced (e.g., "all the errors we saw")
	ReferencedTurnIDs []int

	// ReferenceStrength indicates how explicit the reference is (0.0-1.0).
	// 1.0 = explicit ("What was the error in turn 5?")
	// 0.5 = implicit ("What was the original problem?")
	ReferenceStrength float64

	// ReferencedTopics are topics/entities from the referenced turns.
	// Extracted from turn_topic facts of referenced turns.
	ReferencedTopics []string

	// ReferencedFiles are files mentioned in the referenced turns.
	// Extracted from turn_references_file facts of referenced turns.
	ReferencedFiles []string

	// ReferencedSymbols are symbols mentioned in the referenced turns.
	// Extracted from turn_references_symbol facts of referenced turns.
	ReferencedSymbols []string

	// ReferencedErrors are error messages from the referenced turns.
	// Extracted from turn_error_message facts of referenced turns.
	ReferencedErrors []string
}

// NewActivationEngine creates a new activation engine.
func NewActivationEngine(config CompressorConfig) *ActivationEngine {
	return &ActivationEngine{
		config:              config,
		factTimestamps:      make(map[string]time.Time),
		dependencies:        make(map[string][]string),
		reverseDependencies: make(map[string][]string),
		symbolGraph:         make(map[string][]string),
		sessionFacts:        make(map[string]bool),
		sessionStarted:      time.Now(),
		sessionID:           fmt.Sprintf("sess_%d", time.Now().UnixNano()),
	}
}

// SetCampaignContext sets the current campaign context for activation boosting.
func (ae *ActivationEngine) SetCampaignContext(ctx *CampaignActivationContext) {
	ae.mu.Lock()
	defer ae.mu.Unlock()
	ae.campaignContext = ctx
}

// ClearCampaignContext clears the campaign context.
func (ae *ActivationEngine) ClearCampaignContext() {
	ae.mu.Lock()
	defer ae.mu.Unlock()
	ae.campaignContext = nil
}

// SetIssueContext sets the current issue context for issue-driven activation boosting.
// Works with any issue source: GitHub issues, Jira tickets, bug reports, or benchmarks.
func (ae *ActivationEngine) SetIssueContext(ctx *IssueActivationContext) {
	ae.mu.Lock()
	defer ae.mu.Unlock()
	ae.issueContext = ctx
}

// ClearIssueContext clears the issue context.
func (ae *ActivationEngine) ClearIssueContext() {
	ae.mu.Lock()
	defer ae.mu.Unlock()
	ae.issueContext = nil
}

// SetBackReferenceContext sets the current back-reference context.
// Call this when the user asks a follow-up question referring to previous turns.
func (ae *ActivationEngine) SetBackReferenceContext(ctx *BackReferenceActivationContext) {
	ae.mu.Lock()
	defer ae.mu.Unlock()
	ae.backReferenceContext = ctx
}

// ClearBackReferenceContext clears the back-reference context.
func (ae *ActivationEngine) ClearBackReferenceContext() {
	ae.mu.Lock()
	defer ae.mu.Unlock()
	ae.backReferenceContext = nil
}

// SetCorpusPriorities sets priorities from the predicate corpus.
// These take precedence over hardcoded config.PredicatePriorities.
// Call this after kernel initialization to use corpus as single source of truth.
func (ae *ActivationEngine) SetCorpusPriorities(priorities map[string]int) {
	ae.mu.Lock()
	defer ae.mu.Unlock()
	ae.corpusPriorities = priorities
}

// LoadPrioritiesFromCorpus loads priorities from a PredicateCorpus.
// This is a convenience method that calls GetPriorities() on the corpus.
func (ae *ActivationEngine) LoadPrioritiesFromCorpus(corpus *core.PredicateCorpus) error {
	if corpus == nil {
		return nil // No-op if no corpus
	}
	priorities, err := corpus.GetPriorities()
	if err != nil {
		return err
	}
	ae.mu.Lock()
	ae.corpusPriorities = priorities
	ae.mu.Unlock()
	return nil
}

// SetFeedbackStore sets the context feedback store for learned predicate usefulness.
// When set, the scoring system will use historical feedback to boost/penalize predicates
// based on how useful they've been for each task type.
func (ae *ActivationEngine) SetFeedbackStore(store *ContextFeedbackStore) {
	ae.mu.Lock()
	defer ae.mu.Unlock()
	ae.feedbackStore = store
}

// =============================================================================
// Core Scoring
// =============================================================================

// ScoreFacts computes activation scores for all facts.
// Returns facts sorted by score in descending order.
func (ae *ActivationEngine) ScoreFacts(facts []core.Fact, currentIntent *core.Fact) []ScoredFact {
	timer := logging.StartTimer(logging.CategoryContext, "ScoreFacts")
	defer timer.Stop()

	ae.mu.Lock()
	defer ae.mu.Unlock()
	return ae.scoreFactsLocked(facts, currentIntent)
}

// scoreFactsLocked is the locked implementation of ScoreFacts.
// Caller must hold ae.mu for writing (buildSymbolGraph mutates maps).
func (ae *ActivationEngine) scoreFactsLocked(facts []core.Fact, currentIntent *core.Fact) []ScoredFact {
	ae.state.ActiveIntent = currentIntent
	ae.state.LastUpdate = time.Now()

	// Extract intent verb for intent-specific feedback lookup
	ae.currentIntentVerb = ""
	if currentIntent != nil && len(currentIntent.Args) >= 3 {
		if v, ok := currentIntent.Args[2].(string); ok {
			ae.currentIntentVerb = v
		}
	}

	intentStr := "<none>"
	if currentIntent != nil {
		intentStr = currentIntent.String()
	}
	logging.ContextDebug("Scoring %d facts with intent: %s", len(facts), intentStr)

	// Build symbol graph from facts (for dependency spreading)
	ae.buildSymbolGraphLocked(facts)

	scored := make([]ScoredFact, 0, len(facts))

	for _, fact := range facts {
		score := ae.computeScore(fact)
		scored = append(scored, ScoredFact{
			Fact:               fact,
			Score:              score.Total(),
			BaseScore:          score.base,
			RecencyScore:       score.recency,
			RelevanceScore:     score.relevance,
			DependencyScore:    score.dependency,
			CampaignScore:      score.campaign,
			SessionScore:       score.session,
			IssueScore:         score.issue,
			FeedbackScore:      score.feedback,
			BackReferenceScore: score.backReference,
		})
	}

	// Sort by total score descending
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].Score > scored[j].Score
	})

	// Log top scorers
	if len(scored) > 0 {
		topScore := scored[0].Score
		aboveThreshold := 0
		for _, sf := range scored {
			if sf.Score >= ae.config.ActivationThreshold {
				aboveThreshold++
			}
		}
		logging.ContextDebug("Activation scoring: top_score=%.1f, above_threshold=%d/%d",
			topScore, aboveThreshold, len(scored))
	}

	return scored
}

// buildSymbolGraphLocked extracts symbol relationships from symbol_graph and
// dependency_link facts. Caller must hold ae.mu.
//
// Rebuilds graph maps from the fact set each call so we do not concurrently
// append into shared slices and so maps do not grow unboundedly across scores.
// Explicit AddDependency edges recorded earlier in the session are preserved
// by merging them into the rebuilt maps.
func (ae *ActivationEngine) buildSymbolGraphLocked(facts []core.Fact) {
	// Preserve edges added via AddDependency (not present as code-graph facts).
	preservedDeps := ae.dependencies
	preservedRev := ae.reverseDependencies

	ae.symbolGraph = make(map[string][]string)
	ae.dependencies = make(map[string][]string)
	ae.reverseDependencies = make(map[string][]string)

	// Re-apply preserved edges first.
	for k, vs := range preservedDeps {
		ae.dependencies[k] = append([]string(nil), vs...)
	}
	for k, vs := range preservedRev {
		ae.reverseDependencies[k] = append([]string(nil), vs...)
	}

	for _, f := range facts {
		switch f.Predicate {
		case "dependency_link":
			// dependency_link(CallerID, CalleeID, ImportPath)
			if len(f.Args) >= 2 {
				caller, _ := f.Args[0].(string)
				callee, _ := f.Args[1].(string)
				if caller != "" && callee != "" {
					ae.symbolGraph[caller] = append(ae.symbolGraph[caller], callee)
					ae.reverseDependencies[callee] = append(ae.reverseDependencies[callee], caller)
				}
			}
		case "symbol_graph":
			// symbol_graph(SymbolID, Type, Visibility, DefinedAt, Signature)
			if len(f.Args) >= 4 {
				symbolID, _ := f.Args[0].(string)
				definedAt, _ := f.Args[3].(string)
				if symbolID != "" && definedAt != "" {
					ae.dependencies[symbolID] = append(ae.dependencies[symbolID], definedAt)
				}
			}
		}
	}
}

// FilterByThreshold returns only facts above the activation threshold.
// Facts below the threshold are considered insufficiently activated for the current context.
func (ae *ActivationEngine) FilterByThreshold(scored []ScoredFact) []ScoredFact {
	threshold := ae.config.ActivationThreshold
	filtered := make([]ScoredFact, 0)
	pruned := 0

	for _, sf := range scored {
		if sf.Score >= threshold {
			filtered = append(filtered, sf)
		} else {
			pruned++
		}
	}

	logging.ContextDebug("FilterByThreshold: %d input, %d passed (≥%.1f), %d pruned",
		len(scored), len(filtered), threshold, pruned)

	return filtered
}

// SelectWithinBudget selects facts that fit within the token budget.
// IMPORTANT: This function automatically applies threshold filtering first.
// This defensive design ensures callers cannot accidentally skip filtering.
func (ae *ActivationEngine) SelectWithinBudget(scored []ScoredFact, budget int) []ScoredFact {
	// ALWAYS apply threshold filter first - defensive design
	// This ensures no caller can bypass activation filtering
	filtered := ae.FilterByThreshold(scored)

	counter := NewTokenCounter()
	selected := make([]ScoredFact, 0)
	usedTokens := 0

	for _, sf := range filtered {
		tokens := counter.CountFact(sf.Fact)
		if usedTokens+tokens <= budget {
			selected = append(selected, sf)
			usedTokens += tokens
		}
	}

	logging.ContextDebug("SelectWithinBudget: %d input → %d above threshold (%.1f) → %d selected (%d/%d tokens)",
		len(scored), len(filtered), ae.config.ActivationThreshold, len(selected), usedTokens, budget)

	return selected
}

// UpdateFocusedPaths updates the focused paths from focus_resolution facts.
func (ae *ActivationEngine) UpdateFocusedPaths(facts []core.Fact) {
	ae.mu.Lock()
	defer ae.mu.Unlock()
	ae.updateFocusedPathsLocked(facts)
}

// updateFocusedPathsLocked is the locked implementation of UpdateFocusedPaths.
func (ae *ActivationEngine) updateFocusedPathsLocked(facts []core.Fact) {
	ae.state.FocusedPaths = nil
	ae.state.FocusedSymbols = nil

	for _, f := range facts {
		if f.Predicate == "focus_resolution" && len(f.Args) >= 3 {
			if path, ok := f.Args[1].(string); ok && path != "" {
				ae.state.FocusedPaths = append(ae.state.FocusedPaths, path)
			}
			if symbol, ok := f.Args[2].(string); ok && symbol != "" {
				ae.state.FocusedSymbols = append(ae.state.FocusedSymbols, symbol)
			}
		}
	}
}

// RecordFactTimestamp records when a fact was added.
func (ae *ActivationEngine) RecordFactTimestamp(fact core.Fact) {
	ae.mu.Lock()
	defer ae.mu.Unlock()
	key := factKey(fact)
	ae.factTimestamps[key] = time.Now()
	ae.sessionFacts[key] = true
}

// AddDependency records a dependency between facts.
func (ae *ActivationEngine) AddDependency(dependent, dependency core.Fact) {
	ae.mu.Lock()
	defer ae.mu.Unlock()
	depKey := factKey(dependent)
	depsKey := factKey(dependency)
	ae.dependencies[depKey] = append(ae.dependencies[depKey], depsKey)
	ae.reverseDependencies[depsKey] = append(ae.reverseDependencies[depsKey], depKey)
}

// =============================================================================
// Helper Functions
// =============================================================================

// factKey creates a unique key for a fact.
func factKey(f core.Fact) string {
	return f.String()
}

// extractPredicate extracts the predicate name from a fact key.
func extractPredicate(key string) string {
	before, _, ok := strings.Cut(key, "(")
	if !ok {
		return key
	}
	return before
}

// lookupPriority returns the priority for a predicate name.
// Checks corpus first, then config, then returns default.
func (ae *ActivationEngine) lookupPriority(pred string) int {
	// Check corpus-based priorities first
	if ae.corpusPriorities != nil {
		if priority, ok := ae.corpusPriorities[pred]; ok {
			return priority
		}
	}
	// Fall back to config-based priorities
	if priority, ok := ae.config.PredicatePriorities[pred]; ok {
		return priority
	}
	return 50 // Default
}

// =============================================================================
// Advanced Activation Patterns
// =============================================================================

// ApplyIntentActivation applies activation boosts based on the current intent.
// This is a high-level function that combines multiple activation strategies.
func (ae *ActivationEngine) ApplyIntentActivation(facts []core.Fact, intent *core.Fact) []ScoredFact {
	ae.mu.Lock()
	defer ae.mu.Unlock()

	// Update focus from focus_resolution facts
	ae.updateFocusedPathsLocked(facts)

	// Score all facts (already under lock)
	scored := ae.scoreFactsLocked(facts, intent)

	// Filter by threshold
	return ae.FilterByThreshold(scored)
}

// GetHighActivationFacts returns facts above the threshold sorted by score.
func (ae *ActivationEngine) GetHighActivationFacts(facts []core.Fact, intent *core.Fact, budget int) []ScoredFact {
	timer := logging.StartTimer(logging.CategoryContext, "GetHighActivationFacts")
	defer timer.Stop()

	logging.ContextDebug("GetHighActivationFacts: %d input facts, budget=%d tokens", len(facts), budget)

	scored := ae.ApplyIntentActivation(facts, intent)
	selected := ae.SelectWithinBudget(scored, budget)

	logging.ContextDebug("GetHighActivationFacts: selected %d high-activation facts", len(selected))

	return selected
}

// SpreadFromSeeds spreads activation from a set of seed facts.
// This implements the spreading activation algorithm described in §8.1.
func (ae *ActivationEngine) SpreadFromSeeds(facts []core.Fact, seeds []core.Fact, depth int) []ScoredFact {
	timer := logging.StartTimer(logging.CategoryContext, "SpreadFromSeeds")
	defer timer.Stop()

	logging.ContextDebug("SpreadFromSeeds: %d facts, %d seeds, depth=%d", len(facts), len(seeds), depth)

	ae.mu.Lock()
	// Mark seeds with high recency
	now := time.Now()
	for _, seed := range seeds {
		key := factKey(seed)
		ae.factTimestamps[key] = now
		ae.sessionFacts[key] = true
	}

	// Create a synthetic intent from the first seed if it's a user_intent
	var intent *core.Fact
	for _, seed := range seeds {
		if seed.Predicate == "user_intent" {
			intent = &seed
			break
		}
	}

	// Score with the seed boost (under same lock)
	scored := ae.scoreFactsLocked(facts, intent)

	// Apply depth-limited spreading
	if depth > 0 {
		for d := range depth {
			for i := range scored {
				// Spread activation to dependencies
				key := factKey(scored[i].Fact)
				if deps, ok := ae.dependencies[key]; ok {
					for _, depKey := range deps {
						// Find the dependent fact and boost it
						for j := range scored {
							if factKey(scored[j].Fact) == depKey {
								// Spread 50% of activation, decaying with depth
								spread := scored[i].Score * 0.5 * math.Pow(0.7, float64(d))
								scored[j].Score += spread
								scored[j].DependencyScore += spread
							}
						}
					}
				}
			}
		}

		// Re-sort after spreading
		sort.Slice(scored, func(i, j int) bool {
			return scored[i].Score > scored[j].Score
		})
	}
	ae.mu.Unlock()

	return scored
}

// =============================================================================
// Activation State Management
// =============================================================================

// GetState returns the current activation state.
func (ae *ActivationEngine) GetState() ActivationState {
	ae.mu.RLock()
	defer ae.mu.RUnlock()
	return ae.state
}

// SetState sets the activation state.
func (ae *ActivationEngine) SetState(state ActivationState) {
	ae.mu.Lock()
	defer ae.mu.Unlock()
	ae.state = state
}

// ClearState resets the activation state.
func (ae *ActivationEngine) ClearState() {
	ae.mu.Lock()
	defer ae.mu.Unlock()
	ae.state = ActivationState{}
	ae.factTimestamps = make(map[string]time.Time)
	ae.dependencies = make(map[string][]string)
	ae.reverseDependencies = make(map[string][]string)
	ae.symbolGraph = make(map[string][]string)
	ae.sessionFacts = make(map[string]bool)
	ae.campaignContext = nil
	ae.issueContext = nil
}

// MarkNewFacts marks a set of facts as newly added (high recency).
func (ae *ActivationEngine) MarkNewFacts(facts []core.Fact) {
	ae.mu.Lock()
	defer ae.mu.Unlock()
	now := time.Now()
	for _, f := range facts {
		key := factKey(f)
		ae.factTimestamps[key] = now
		ae.sessionFacts[key] = true
	}
	ae.state.RecentFacts = append(ae.state.RecentFacts, facts...)
}

// DecayRecency reduces the recency score of old facts.
// Called periodically to allow older facts to fade.
func (ae *ActivationEngine) DecayRecency(maxAge time.Duration) {
	ae.mu.Lock()
	defer ae.mu.Unlock()
	cutoff := time.Now().Add(-maxAge)

	// Remove timestamps older than maxAge
	for key, ts := range ae.factTimestamps {
		if ts.Before(cutoff) {
			delete(ae.factTimestamps, key)
		}
	}

	// Clear recent facts that are too old
	var filtered []core.Fact
	for _, f := range ae.state.RecentFacts {
		key := factKey(f)
		if ts, ok := ae.factTimestamps[key]; ok && ts.After(cutoff) {
			filtered = append(filtered, f)
		}
	}
	ae.state.RecentFacts = filtered
}

// NewSession starts a new session, resetting session-specific tracking.
func (ae *ActivationEngine) NewSession() {
	ae.mu.Lock()
	defer ae.mu.Unlock()
	ae.sessionID = fmt.Sprintf("sess_%d", time.Now().UnixNano())
	ae.sessionStarted = time.Now()
	ae.sessionFacts = make(map[string]bool)
}

// GetSessionStats returns statistics about the current session.
func (ae *ActivationEngine) GetSessionStats() map[string]any {
	ae.mu.RLock()
	defer ae.mu.RUnlock()
	return map[string]any{
		"session_id":      ae.sessionID,
		"session_started": ae.sessionStarted,
		"session_facts":   len(ae.sessionFacts),
		"total_facts":     len(ae.factTimestamps),
		"dependencies":    len(ae.dependencies),
		"symbols":         len(ae.symbolGraph),
		"has_campaign":    ae.campaignContext != nil,
		"has_issue":       ae.issueContext != nil,
	}
}
