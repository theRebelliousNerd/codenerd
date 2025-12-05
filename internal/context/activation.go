package context

import (
	"codenerd/internal/core"
	"math"
	"sort"
	"strings"
	"time"
)

// =============================================================================
// Spreading Activation Engine
// =============================================================================
// Implements §8.1: Logic-Directed Context (Spreading Activation)
// Energy flows from the user's intent through the graph of known facts.

// ActivationEngine computes activation scores for facts.
type ActivationEngine struct {
	config CompressorConfig

	// State tracking
	state ActivationState

	// Fact timestamps for recency scoring
	factTimestamps map[string]time.Time

	// Dependency graph for spreading
	dependencies map[string][]string // fact -> depends on
}

// NewActivationEngine creates a new activation engine.
func NewActivationEngine(config CompressorConfig) *ActivationEngine {
	return &ActivationEngine{
		config:         config,
		factTimestamps: make(map[string]time.Time),
		dependencies:   make(map[string][]string),
	}
}

// ScoreFacts computes activation scores for all facts.
// Returns facts sorted by score in descending order.
func (ae *ActivationEngine) ScoreFacts(facts []core.Fact, currentIntent *core.Fact) []ScoredFact {
	ae.state.ActiveIntent = currentIntent
	ae.state.LastUpdate = time.Now()

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

	return scored
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
}

// AddDependency records a dependency between facts.
func (ae *ActivationEngine) AddDependency(dependent, dependency core.Fact) {
	depKey := factKey(dependent)
	depsKey := factKey(dependency)
	ae.dependencies[depKey] = append(ae.dependencies[depKey], depsKey)
}

// =============================================================================
// Scoring Components
// =============================================================================

type scoreComponents struct {
	base       float64
	recency    float64
	relevance  float64
	dependency float64
}

func (s *scoreComponents) Total() float64 {
	return s.base + s.recency + s.relevance + s.dependency
}

// computeScore calculates the activation score for a fact.
func (ae *ActivationEngine) computeScore(fact core.Fact) scoreComponents {
	return scoreComponents{
		base:       ae.computeBaseScore(fact),
		recency:    ae.computeRecencyScore(fact),
		relevance:  ae.computeRelevanceScore(fact),
		dependency: ae.computeDependencyScore(fact),
	}
}

// computeBaseScore returns the base priority score for a predicate.
// This implements the predicate priority system from policy.gl §1.
func (ae *ActivationEngine) computeBaseScore(fact core.Fact) float64 {
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

	// Verb-predicate relevance boosting
	verbPredicateBoosts := map[string][]string{
		"/fix":      {"diagnostic", "test_state", "impacted"},
		"/debug":    {"diagnostic", "test_state", "impacted", "symbol_graph"},
		"/refactor": {"dependency_link", "impacted", "unsafe_to_refactor", "block_refactor"},
		"/test":     {"test_state", "test_coverage", "diagnostic"},
		"/explain":  {"symbol_graph", "dependency_link", "file_topology"},
		"/research": {"knowledge_atom", "vector_recall", "knowledge_link"},
	}

	if boosts, ok := verbPredicateBoosts[intentVerb]; ok {
		for _, pred := range boosts {
			if fact.Predicate == pred {
				score += 25.0
				break
			}
		}
	}

	return score
}

// computeDependencyScore applies spreading activation through the dependency graph.
func (ae *ActivationEngine) computeDependencyScore(fact core.Fact) float64 {
	// Simple spreading: if this fact depends on a high-priority fact,
	// it inherits some of that priority
	key := factKey(fact)
	deps, ok := ae.dependencies[key]
	if !ok || len(deps) == 0 {
		return 0.0
	}

	// Sum partial scores from dependencies (with decay)
	totalScore := 0.0
	for _, depKey := range deps {
		// Look up the dependency's priority (simplified - use predicate)
		pred := extractPredicate(depKey)
		if priority, ok := ae.config.PredicatePriorities[pred]; ok {
			totalScore += float64(priority) * 0.3 // 30% inheritance
		}
	}

	return math.Min(totalScore, 30.0) // Cap at 30
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

// =============================================================================
// Predefined Activation Patterns
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
	scored := ae.ApplyIntentActivation(facts, intent)
	return ae.SelectWithinBudget(scored, budget)
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
}

// MarkNewFacts marks a set of facts as newly added (high recency).
func (ae *ActivationEngine) MarkNewFacts(facts []core.Fact) {
	now := time.Now()
	for _, f := range facts {
		key := factKey(f)
		ae.factTimestamps[key] = now
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
