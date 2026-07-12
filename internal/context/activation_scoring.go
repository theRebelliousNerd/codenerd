package context

import (
	"codenerd/internal/core"
	"math"
	"slices"
	"strings"
	"time"
)

// =============================================================================
// Enhanced Scoring Components
// =============================================================================

type scoreComponents struct {
	base          float64
	recency       float64
	relevance     float64
	dependency    float64
	campaign      float64 // Campaign-specific boost
	session       float64 // Session-specific boost
	issue         float64 // Issue/SWE-bench specific boost
	feedback      float64 // Learned predicate usefulness from LLM feedback
	backReference float64 // Back-reference boost for follow-up questions
}

func (s *scoreComponents) Total() float64 {
	return s.base + s.recency + s.relevance + s.dependency + s.campaign + s.session + s.issue + s.feedback + s.backReference
}

// NERD-EVOLVE-START: context_scoring_engine
// Target: Replace 9-component Go heuristic with Mangle-derived context_score(Fact, Score) rules.
// The kernel already has working_file, dependency_link, symbol_graph facts.
// C1+C4: ScoreFactsWithKernelOverride uses kernel-derived priorities when available.

// computeScore calculates the activation score for a fact.
func (ae *ActivationEngine) computeScore(fact core.Fact) scoreComponents {
	return scoreComponents{
		base:          ae.computeBaseScore(fact),
		recency:       ae.computeRecencyScore(fact),
		relevance:     ae.computeRelevanceScore(fact),
		dependency:    ae.computeDependencyScore(fact),
		campaign:      ae.computeCampaignScore(fact),
		session:       ae.computeSessionScore(fact),
		issue:         ae.computeIssueScore(fact),
		feedback:      ae.computeFeedbackScore(fact),
		backReference: ae.computeBackReferenceScore(fact),
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
			"diagnostic":     35.0,
			"test_state":     30.0,
			"impacted":       25.0,
			"file_content":   20.0,
			"error_context":  40.0,
			"knowledge_atom": 30.0, // Architecture knowledge helps fix decisions
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
			"diagnostic":     35.0,
			"security_issue": 45.0,
			"code_smell":     30.0,
			"complexity":     25.0,
			"knowledge_atom": 35.0, // Pattern knowledge for code review
		},
		"/security": {
			"security_issue":   50.0,
			"vulnerability":    50.0,
			"diagnostic":       30.0,
			"security_pattern": 40.0,
			"knowledge_atom":   30.0, // Security constraints from docs
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
			// Clamp weight to [0.0, 1.0] to prevent adversarial facts from
			// reaching uncapped scores (e.g., weight=100 → score=5000) that
			// push safety rules out of the context window.
			clampedWeight := weight
			if clampedWeight < 0 {
				clampedWeight = 0
			}
			if clampedWeight > 1.0 {
				clampedWeight = 1.0
			}
			// Scale clamped weight to score points (0-50)
			score += clampedWeight * 50.0
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
		"pytest_failure":      50.0,
		"assertion_mismatch":  45.0,
		"traceback_frame":     35.0,
		"pytest_root_cause":   55.0,
		"source_file_failure": 50.0,
		"test_failure":        50.0,
		"diagnostic":          45.0,
		"error_context":       40.0,

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

	return math.Min(score, 100.0) // Cap at 100 to prevent any single fact from dominating context
}

// computeFeedbackScore applies learned predicate usefulness from LLM feedback.
// This implements the third feedback loop: LLM-driven context learning.
// Score range: -20 (consistently noise) to +20 (consistently helpful)
func (ae *ActivationEngine) computeFeedbackScore(fact core.Fact) float64 {
	if ae.feedbackStore == nil {
		return 0.0 // No feedback store configured
	}

	// Try intent-specific feedback first for more targeted learning
	var usefulnessScore float64
	if ae.currentIntentVerb != "" {
		usefulnessScore = ae.feedbackStore.GetPredicateUsefulnessForIntent(fact.Predicate, ae.currentIntentVerb)
	}

	// Fall back to general usefulness if no intent-specific feedback
	if usefulnessScore == 0.0 {
		usefulnessScore = ae.feedbackStore.GetPredicateUsefulness(fact.Predicate)
	}

	// usefulnessScore is -1.0 (always noise) to +1.0 (always helpful)
	// Scale to -20 to +20 activation boost/penalty
	// This is a significant but not overwhelming factor in total scoring
	return usefulnessScore * 20.0
}

// computeBackReferenceScore adds back-reference activation boost.
// Facts related to previously referenced turns get boosted when the user
// asks follow-up questions like "What was the original error?"
func (ae *ActivationEngine) computeBackReferenceScore(fact core.Fact) float64 {
	if ae.backReferenceContext == nil {
		return 0.0
	}

	score := 0.0
	factStr := strings.ToLower(fact.String())

	// Primary boost: facts from referenced turns
	// Check if fact's turn ID matches any referenced turn.
	//
	// The bare `.(int)` assertion silently fails when the kernel returns
	// turn IDs as `int64` (which is the common case for facts loaded
	// from SQLite or asserted via AssertString — both produce
	// ast.Number, which materialises as int64 on the Go side). Use the
	// same int/int64/float64 normalization the compressor uses for
	// turn IDs.
	if turnID, ok := factArgAsInt(fact); ok {
		if slices.Contains(ae.backReferenceContext.ReferencedTurnIDs, turnID) {
			score += 50.0
		}
	}

	// Topic boost: facts matching topics from referenced turns
	for _, topic := range ae.backReferenceContext.ReferencedTopics {
		if strings.Contains(factStr, strings.ToLower(topic)) {
			score += 30.0
			break
		}
	}

	// File boost: facts mentioning files from referenced turns
	for _, file := range ae.backReferenceContext.ReferencedFiles {
		if strings.Contains(factStr, strings.ToLower(file)) {
			score += 20.0
			break
		}
	}

	// Symbol boost: facts mentioning symbols from referenced turns
	for _, symbol := range ae.backReferenceContext.ReferencedSymbols {
		if strings.Contains(factStr, strings.ToLower(symbol)) {
			score += 25.0
			break
		}
	}

	// Error boost: facts mentioning errors from referenced turns
	for _, errMsg := range ae.backReferenceContext.ReferencedErrors {
		if strings.Contains(factStr, strings.ToLower(errMsg)) {
			score += 35.0
			break
		}
	}

	// Reference strength multiplier (0.0-1.0)
	// Explicit references get full boost, implicit get partial
	if ae.backReferenceContext.ReferenceStrength > 0 && ae.backReferenceContext.ReferenceStrength < 1.0 {
		score *= ae.backReferenceContext.ReferenceStrength
	}

	// Back-reference specific predicates get extra boost
	backRefPredicates := map[string]float64{
		"turn_references_back":   50.0, // Explicit back-reference tracking
		"turn_error_message":     30.0, // Errors are often referenced back
		"turn_topic":             25.0, // Topics help identify context
		"turn_references_file":   20.0, // File references
		"turn_references_symbol": 20.0, // Symbol references
	}

	if boost, ok := backRefPredicates[fact.Predicate]; ok {
		// Only apply predicate boost if this fact is from a referenced turn
		if turnID, ok := factArgAsInt(fact); ok {
			if slices.Contains(ae.backReferenceContext.ReferencedTurnIDs, turnID) {
				score += boost
			}
		}
	}

	return math.Min(score, 70.0) // Cap at 70
}

// factArgAsInt extracts the first argument of a fact as an int, handling
// the int / int64 / float64 type drift between Go-emitted facts and
// kernel-derived facts. The kernel typically returns Number constants
// as int64; Go-side asserts may use int. Both must match for back-ref
// scoring to fire.
func factArgAsInt(fact core.Fact) (int, bool) {
	if len(fact.Args) == 0 {
		return 0, false
	}
	switch v := fact.Args[0].(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	default:
		return 0, false
	}
}

// ScoreFactsWithKernelOverride scores facts using kernel-derived scores as primary
// source when available, falling back to the Go heuristic for facts not covered.
//
// C1+C4 integration: when kernelScores is non-empty (a map of fact string ->
// priority score derived from should_include_context), those scores take precedence
// over the 9-component Go heuristic for matching facts. Facts not present in
// kernelScores receive Go-derived scores as before.
//
// Backward compatible: if kernelScores is nil or empty, behaves identically to ScoreFacts.
func (ae *ActivationEngine) ScoreFactsWithKernelOverride(facts []core.Fact, intent *core.Fact, kernelScores map[string]float64) []ScoredFact {
	if len(kernelScores) == 0 {
		return ae.ScoreFacts(facts, intent)
	}

	ae.mu.Lock()
	defer ae.mu.Unlock()

	// Update focus from facts for Go-side fallback scoring
	ae.updateFocusedPathsLocked(facts)
	ae.buildSymbolGraphLocked(facts)

	scored := make([]ScoredFact, 0, len(facts))
	for _, fact := range facts {
		key := factKey(fact)
		if kScore, ok := kernelScores[key]; ok {
			// Kernel-derived score takes precedence over Go heuristic
			scored = append(scored, ScoredFact{
				Fact:  fact,
				Score: kScore,
			})
		} else {
			// Go heuristic fallback for facts not covered by kernel derivation
			components := ae.computeScore(fact)
			scored = append(scored, ScoredFact{
				Fact:               fact,
				Score:              components.Total(),
				BaseScore:          components.base,
				RecencyScore:       components.recency,
				RelevanceScore:     components.relevance,
				DependencyScore:    components.dependency,
				CampaignScore:      components.campaign,
				SessionScore:       components.session,
				IssueScore:         components.issue,
				FeedbackScore:      components.feedback,
				BackReferenceScore: components.backReference,
			})
		}
	}

	return scored
}

// NERD-EVOLVE-END: context_scoring_engine
