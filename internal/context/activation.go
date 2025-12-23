package context

import (
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"fmt"
	"math"
	"sort"
	"strings"
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
type ActivationEngine struct {
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

	// Session tracking
	sessionID      string
	sessionStarted time.Time
	sessionFacts   map[string]bool // Facts added this session

	// Corpus-based priorities (loaded from predicate_corpus.db)
	// These take precedence over config.PredicatePriorities
	corpusPriorities map[string]int
}

// CampaignActivationContext holds campaign-specific activation state.
type CampaignActivationContext struct {
	CampaignID    string
	CurrentPhase  string
	CurrentTask   string
	PhaseGoals    []string
	RelevantFiles []string
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
	ae.campaignContext = ctx
}

// ClearCampaignContext clears the campaign context.
func (ae *ActivationEngine) ClearCampaignContext() {
	ae.campaignContext = nil
}

// SetIssueContext sets the current issue context for issue-driven activation boosting.
// Works with any issue source: GitHub issues, Jira tickets, bug reports, or benchmarks.
func (ae *ActivationEngine) SetIssueContext(ctx *IssueActivationContext) {
	ae.issueContext = ctx
}

// ClearIssueContext clears the issue context.
func (ae *ActivationEngine) ClearIssueContext() {
	ae.issueContext = nil
}

// SetCorpusPriorities sets priorities from the predicate corpus.
// These take precedence over hardcoded config.PredicatePriorities.
// Call this after kernel initialization to use corpus as single source of truth.
func (ae *ActivationEngine) SetCorpusPriorities(priorities map[string]int) {
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
	ae.corpusPriorities = priorities
	return nil
}

// =============================================================================
// Core Scoring
// =============================================================================

// ScoreFacts computes activation scores for all facts.
// Returns facts sorted by score in descending order.
func (ae *ActivationEngine) ScoreFacts(facts []core.Fact, currentIntent *core.Fact) []ScoredFact {
	timer := logging.StartTimer(logging.CategoryContext, "ScoreFacts")
	defer timer.Stop()

	ae.state.ActiveIntent = currentIntent
	ae.state.LastUpdate = time.Now()

	intentStr := "<none>"
	if currentIntent != nil {
		intentStr = currentIntent.String()
	}
	logging.ContextDebug("Scoring %d facts with intent: %s", len(facts), intentStr)

	// Build symbol graph from facts (for dependency spreading)
	ae.buildSymbolGraph(facts)

	scored := make([]ScoredFact, 0, len(facts))

	for _, fact := range facts {
		score := ae.computeScore(fact)
		scored = append(scored, ScoredFact{
			Fact:            fact,
			Score:           score.Total(),
			BaseScore:       score.base,
			RecencyScore:    score.recency,
			RelevanceScore:  score.relevance,
			DependencyScore: score.dependency,
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

// buildSymbolGraph extracts symbol relationships from symbol_graph and dependency_link facts.
func (ae *ActivationEngine) buildSymbolGraph(facts []core.Fact) {
	for _, f := range facts {
		switch f.Predicate {
		case "dependency_link":
			// dependency_link(CallerID, CalleeID, ImportPath)
			if len(f.Args) >= 2 {
				caller, _ := f.Args[0].(string)
				callee, _ := f.Args[1].(string)
				if caller != "" && callee != "" {
					ae.symbolGraph[caller] = append(ae.symbolGraph[caller], callee)
					// Also add reverse dependency
					ae.reverseDependencies[callee] = append(ae.reverseDependencies[callee], caller)
				}
			}
		case "symbol_graph":
			// symbol_graph(SymbolID, Type, Visibility, DefinedAt, Signature)
			if len(f.Args) >= 4 {
				symbolID, _ := f.Args[0].(string)
				definedAt, _ := f.Args[3].(string)
				if symbolID != "" && definedAt != "" {
					// Link symbol to its file
					ae.dependencies[symbolID] = append(ae.dependencies[symbolID], definedAt)
				}
			}
		}
	}
}

// FilterByThreshold returns only facts above the activation threshold.
func (ae *ActivationEngine) FilterByThreshold(scored []ScoredFact) []ScoredFact {
	threshold := ae.config.ActivationThreshold
	filtered := make([]ScoredFact, 0)

	for _, sf := range scored {
		if sf.Score >= threshold {
			filtered = append(filtered, sf)
		}
	}

	return filtered
}

// SelectWithinBudget selects facts that fit within the token budget.
func (ae *ActivationEngine) SelectWithinBudget(scored []ScoredFact, budget int) []ScoredFact {
	counter := NewTokenCounter()
	selected := make([]ScoredFact, 0)
	usedTokens := 0

	for _, sf := range scored {
		tokens := counter.CountFact(sf.Fact)
		if usedTokens+tokens <= budget {
			selected = append(selected, sf)
			usedTokens += tokens
		}
	}

	logging.ContextDebug("SelectWithinBudget: selected %d/%d facts, using %d/%d tokens",
		len(selected), len(scored), usedTokens, budget)

	return selected
}

// UpdateFocusedPaths updates the focused paths from focus_resolution facts.
func (ae *ActivationEngine) UpdateFocusedPaths(facts []core.Fact) {
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
	key := factKey(fact)
	ae.factTimestamps[key] = time.Now()
	ae.sessionFacts[key] = true
}

// AddDependency records a dependency between facts.
func (ae *ActivationEngine) AddDependency(dependent, dependency core.Fact) {
	depKey := factKey(dependent)
	depsKey := factKey(dependency)
	ae.dependencies[depKey] = append(ae.dependencies[depKey], depsKey)
	ae.reverseDependencies[depsKey] = append(ae.reverseDependencies[depsKey], depKey)
}

// =============================================================================
// Enhanced Scoring Components
// =============================================================================

type scoreComponents struct {
	base       float64
	recency    float64
	relevance  float64
	dependency float64
	campaign   float64 // Campaign-specific boost
	session    float64 // Session-specific boost
	issue      float64 // Issue/SWE-bench specific boost
}

func (s *scoreComponents) Total() float64 {
	return s.base + s.recency + s.relevance + s.dependency + s.campaign + s.session + s.issue
}

// computeScore calculates the activation score for a fact.
func (ae *ActivationEngine) computeScore(fact core.Fact) scoreComponents {
	return scoreComponents{
		base:       ae.computeBaseScore(fact),
		recency:    ae.computeRecencyScore(fact),
		relevance:  ae.computeRelevanceScore(fact),
		dependency: ae.computeDependencyScore(fact),
		campaign:   ae.computeCampaignScore(fact),
		session:    ae.computeSessionScore(fact),
		issue:      ae.computeIssueScore(fact),
	}
}

// computeBaseScore returns the base priority score for a predicate.
// This implements the predicate priority system from policy.mg §1.
// Priority sources (checked in order):
// 1. Corpus-based priorities (from predicate_corpus.db if loaded)
// 2. Config-based priorities (hardcoded fallback in types.go)
// 3. Default (50)
func (ae *ActivationEngine) computeBaseScore(fact core.Fact) float64 {
	// Check corpus-based priorities first (single source of truth)
	if ae.corpusPriorities != nil {
		if priority, ok := ae.corpusPriorities[fact.Predicate]; ok {
			return float64(priority)
		}
	}
	// Fall back to config-based priorities (hardcoded)
	if priority, ok := ae.config.PredicatePriorities[fact.Predicate]; ok {
		return float64(priority)
	}
	return 50.0 // Default priority
}

// computeRecencyScore applies recency bias to facts.
// More recently added facts get higher scores.
func (ae *ActivationEngine) computeRecencyScore(fact core.Fact) float64 {
	key := factKey(fact)
	timestamp, ok := ae.factTimestamps[key]
	if !ok {
		return 0.0 // Unknown timestamp
	}

	age := time.Since(timestamp)

	// Decay function: score decreases with age
	// New facts (< 1 minute): +50
	// Recent facts (< 5 minutes): +30
	// Older facts (< 30 minutes): +10
	// Very old facts: 0
	switch {
	case age < time.Minute:
		return 50.0
	case age < 5*time.Minute:
		return 30.0
	case age < 30*time.Minute:
		return 10.0
	default:
		return 0.0
	}
}

// computeRelevanceScore scores based on relevance to current intent.
func (ae *ActivationEngine) computeRelevanceScore(fact core.Fact) float64 {
	if ae.state.ActiveIntent == nil {
		return 0.0
	}

	score := 0.0

	// Extract target from intent
	var intentTarget string
	if len(ae.state.ActiveIntent.Args) >= 4 {
		if t, ok := ae.state.ActiveIntent.Args[3].(string); ok {
			intentTarget = strings.ToLower(t)
		}
	}

	// Check if fact relates to the target
	factStr := strings.ToLower(fact.String())
	if intentTarget != "" && strings.Contains(factStr, intentTarget) {
		score += 40.0
	}

	// Check if fact relates to focused paths
	for _, path := range ae.state.FocusedPaths {
		if strings.Contains(factStr, strings.ToLower(path)) {
			score += 30.0
			break
		}
	}

	// Check if fact relates to focused symbols
	for _, symbol := range ae.state.FocusedSymbols {
		if strings.Contains(factStr, strings.ToLower(symbol)) {
			score += 20.0
			break
		}
	}

	// Special boosting for certain predicates related to active intent
	intentVerb := ""
	if len(ae.state.ActiveIntent.Args) >= 3 {
		if v, ok := ae.state.ActiveIntent.Args[2].(string); ok {
			intentVerb = v
		}
	}

	// Enhanced verb-predicate relevance boosting
	verbPredicateBoosts := map[string]map[string]float64{
		"/fix": {
			"diagnostic":      35.0,
			"test_state":      30.0,
			"impacted":        25.0,
			"file_content":    20.0,
			"error_context":   40.0,
			"knowledge_atom":  30.0, // Architecture knowledge helps fix decisions
		},
		"/debug": {
			"diagnostic":     40.0,
			"test_state":     35.0,
			"impacted":       30.0,
			"symbol_graph":   25.0,
			"stack_trace":    45.0,
			"knowledge_atom": 25.0, // Pattern knowledge helps debugging
		},
		"/refactor": {
			"dependency_link":    40.0,
			"impacted":           35.0,
			"unsafe_to_refactor": 50.0,
			"block_refactor":     50.0,
			"symbol_graph":       30.0,
			"knowledge_atom":     40.0, // Architecture knowledge is critical for refactoring
		},
		"/test": {
			"test_state":     45.0,
			"test_coverage":  40.0,
			"diagnostic":     30.0,
			"test_result":    40.0,
			"knowledge_atom": 25.0, // Testing patterns from docs
		},
		"/explain": {
			"symbol_graph":    35.0,
			"dependency_link": 30.0,
			"file_topology":   25.0,
			"documentation":   30.0,
			"knowledge_atom":  40.0, // Strategic knowledge helps explanations
		},
		"/research": {
			"knowledge_atom": 45.0,
			"vector_recall":  40.0,
			"knowledge_link": 35.0,
			"documentation":  35.0,
		},
		"/review": {
			"diagnostic":      35.0,
			"security_issue":  45.0,
			"code_smell":      30.0,
			"complexity":      25.0,
			"knowledge_atom":  35.0, // Pattern knowledge for code review
		},
		"/security": {
			"security_issue":    50.0,
			"vulnerability":     50.0,
			"diagnostic":        30.0,
			"security_pattern":  40.0,
			"knowledge_atom":    30.0, // Security constraints from docs
		},
		"/create": {
			"file_topology":   30.0,
			"symbol_graph":    25.0,
			"template":        35.0,
			"dependency_link": 20.0,
			"knowledge_atom":  45.0, // Architecture knowledge is critical for new code
		},
	}

	if boosts, ok := verbPredicateBoosts[intentVerb]; ok {
		if boost, found := boosts[fact.Predicate]; found {
			score += boost
		}
	}

	return score
}

// computeDependencyScore applies spreading activation through the dependency graph.
// Enhanced with bidirectional spreading and depth-limited traversal.
func (ae *ActivationEngine) computeDependencyScore(fact core.Fact) float64 {
	key := factKey(fact)
	score := 0.0

	// Forward dependencies (what I depend on)
	if deps, ok := ae.dependencies[key]; ok {
		for _, depKey := range deps {
			pred := extractPredicate(depKey)
			priority := ae.lookupPriority(pred)
			score += float64(priority) * 0.3 // 30% inheritance
		}
	}

	// Reverse dependencies (what depends on me)
	// If many things depend on me, I'm more important
	if rdeps, ok := ae.reverseDependencies[key]; ok {
		score += float64(len(rdeps)) * 5.0 // 5 points per dependent
	}

	// Symbol graph spreading
	// If this fact relates to a file that's in the focused paths
	factStr := fact.String()
	for _, focusPath := range ae.state.FocusedPaths {
		if strings.Contains(factStr, focusPath) {
			// Check if any symbols in this file are called by other symbols
			for symbol, callees := range ae.symbolGraph {
				if strings.Contains(symbol, focusPath) {
					score += float64(len(callees)) * 2.0 // Points for outgoing calls
				}
				for _, callee := range callees {
					if strings.Contains(callee, focusPath) {
						score += 3.0 // Points for incoming calls
					}
				}
			}
		}
	}

	return math.Min(score, 40.0) // Cap at 40
}

// computeCampaignScore adds campaign-specific activation boost.
// Facts related to the current phase/task get higher scores.
func (ae *ActivationEngine) computeCampaignScore(fact core.Fact) float64 {
	if ae.campaignContext == nil {
		return 0.0
	}

	score := 0.0
	factStr := strings.ToLower(fact.String())

	// Boost facts related to current campaign
	if ae.campaignContext.CampaignID != "" {
		if strings.Contains(factStr, strings.ToLower(ae.campaignContext.CampaignID)) {
			score += 25.0
		}
	}

	// Boost facts related to current phase
	if ae.campaignContext.CurrentPhase != "" {
		if strings.Contains(factStr, strings.ToLower(ae.campaignContext.CurrentPhase)) {
			score += 30.0
		}
	}

	// Boost facts related to current task
	if ae.campaignContext.CurrentTask != "" {
		if strings.Contains(factStr, strings.ToLower(ae.campaignContext.CurrentTask)) {
			score += 35.0
		}
	}

	// Boost facts related to relevant files
	for _, file := range ae.campaignContext.RelevantFiles {
		if strings.Contains(factStr, strings.ToLower(file)) {
			score += 20.0
			break
		}
	}

	// Boost facts related to relevant symbols
	for _, symbol := range ae.campaignContext.RelevantSymbols {
		if strings.Contains(factStr, strings.ToLower(symbol)) {
			score += 15.0
			break
		}
	}

	// Campaign-specific predicates get extra boost
	campaignPredicates := map[string]float64{
		"campaign":          40.0,
		"campaign_phase":    35.0,
		"campaign_task":     35.0,
		"current_campaign":  50.0,
		"current_phase":     45.0,
		"phase_goal":        30.0,
		"task_dependency":   25.0,
		"phase_requirement": 25.0,
	}

	if boost, ok := campaignPredicates[fact.Predicate]; ok {
		score += boost
	}

	return math.Min(score, 60.0) // Cap at 60
}

// computeSessionScore boosts facts that were added during the current session.
func (ae *ActivationEngine) computeSessionScore(fact core.Fact) float64 {
	key := factKey(fact)

	// Check if this fact was added during the current session
	if ae.sessionFacts[key] {
		return 15.0 // Session bonus
	}

	return 0.0
}

// computeIssueScore adds issue-driven activation boost.
// Facts related to the issue keywords, mentioned files, or expected tests get higher scores.
// This works with ANY issue source: GitHub, Jira, bug reports, or benchmarks.
func (ae *ActivationEngine) computeIssueScore(fact core.Fact) float64 {
	if ae.issueContext == nil {
		return 0.0
	}

	score := 0.0
	factStr := strings.ToLower(fact.String())

	// Boost facts mentioning issue ID
	if ae.issueContext.IssueID != "" {
		if strings.Contains(factStr, strings.ToLower(ae.issueContext.IssueID)) {
			score += 30.0
		}
	}

	// Boost facts matching keywords (weighted)
	for keyword, weight := range ae.issueContext.Keywords {
		if strings.Contains(factStr, strings.ToLower(keyword)) {
			// Scale weight (0.0-1.0) to score points (0-50)
			score += weight * 50.0
		}
	}

	// Boost facts related to mentioned files (Tier 1 files)
	for _, file := range ae.issueContext.MentionedFiles {
		if strings.Contains(factStr, strings.ToLower(file)) {
			score += 40.0
			break
		}
	}

	// Boost facts related to tiered files based on tier
	// Tier 1: +50, Tier 2: +35, Tier 3: +20, Tier 4: +10
	tierBoosts := map[int]float64{1: 50.0, 2: 35.0, 3: 20.0, 4: 10.0}
	for file, tier := range ae.issueContext.TieredFiles {
		if strings.Contains(factStr, strings.ToLower(file)) {
			if boost, ok := tierBoosts[tier]; ok {
				score += boost
			}
			break
		}
	}

	// Boost facts mentioning error types
	for _, errorType := range ae.issueContext.ErrorTypes {
		if strings.Contains(factStr, strings.ToLower(errorType)) {
			score += 35.0
			break
		}
	}

	// Boost facts related to expected tests (tests that should pass after fix)
	for _, testName := range ae.issueContext.ExpectedTests {
		if strings.Contains(factStr, strings.ToLower(testName)) {
			score += 45.0
			break
		}
	}

	// Issue-related predicates get extra boost
	// Organized by category for clarity:
	issuePredicates := map[string]float64{
		// General issue tracking predicates
		"issue_keyword":    40.0,
		"keyword_hit":      35.0,
		"candidate_file":   30.0,
		"context_tier":     25.0,
		"activation_boost": 20.0,

		// Test/diagnostic predicates (general-purpose)
		"pytest_failure":       50.0,
		"assertion_mismatch":   45.0,
		"traceback_frame":      35.0,
		"pytest_root_cause":    55.0,
		"source_file_failure":  50.0,
		"test_failure":         50.0,
		"diagnostic":           45.0,
		"error_context":        40.0,

		// Benchmark-specific predicates (loaded only when running benchmarks)
		// These are in benchmarks.mg and only relevant during benchmark evaluation
		"swebench_instance":          50.0,
		"swebench_environment":       40.0,
		"swebench_test_result":       45.0,
		"swebench_evaluation_result": 45.0,
	}

	if boost, ok := issuePredicates[fact.Predicate]; ok {
		score += boost
	}

	return math.Min(score, 80.0) // Cap at 80
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
	idx := strings.Index(key, "(")
	if idx == -1 {
		return key
	}
	return key[:idx]
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
	// Update focus from focus_resolution facts
	ae.UpdateFocusedPaths(facts)

	// Score all facts
	scored := ae.ScoreFacts(facts, intent)

	// Filter by threshold
	filtered := ae.FilterByThreshold(scored)

	return filtered
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

	// Score with the seed boost
	scored := ae.ScoreFacts(facts, intent)

	// Apply depth-limited spreading
	if depth > 0 {
		for d := 0; d < depth; d++ {
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

	return scored
}

// =============================================================================
// Activation State Management
// =============================================================================

// GetState returns the current activation state.
func (ae *ActivationEngine) GetState() ActivationState {
	return ae.state
}

// SetState sets the activation state.
func (ae *ActivationEngine) SetState(state ActivationState) {
	ae.state = state
}

// ClearState resets the activation state.
func (ae *ActivationEngine) ClearState() {
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
	ae.sessionID = fmt.Sprintf("sess_%d", time.Now().UnixNano())
	ae.sessionStarted = time.Now()
	ae.sessionFacts = make(map[string]bool)
}

// GetSessionStats returns statistics about the current session.
func (ae *ActivationEngine) GetSessionStats() map[string]interface{} {
	return map[string]interface{}{
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
