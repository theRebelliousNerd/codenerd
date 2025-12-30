package context_harness

import (
	"context"
	"fmt"

	"codenerd/internal/core"
	internalcontext "codenerd/internal/context"
)

// scoredFact represents a fact with an activation score for sorting.
type scoredFact struct {
	fact  core.Fact
	score float64
}

// MockContextEngine provides fast mock implementations for CI testing.
// It uses simplified scoring instead of the real 7-component ActivationEngine.
// Facts persist within a scenario but don't use real compression.
type MockContextEngine struct {
	kernel *core.RealKernel
	facts  []core.Fact // All facts across turns

	// Stats tracking
	originalTokens   int
	compressedTokens int
}

// Ensure MockContextEngine implements ContextEngine
var _ ContextEngine = (*MockContextEngine)(nil)

// NewMockContextEngine creates a mock engine for fast testing.
func NewMockContextEngine(kernel *core.RealKernel) *MockContextEngine {
	return &MockContextEngine{
		kernel: kernel,
		facts:  make([]core.Fact, 0),
	}
}

// CompressTurn compresses a single turn into semantic facts.
// Uses simplified fact generation (no real LLM compression).
func (e *MockContextEngine) CompressTurn(ctx context.Context, turn *Turn) ([]core.Fact, int, error) {
	// Track original tokens (~4 chars per token)
	originalTokens := len(turn.Message) / 4
	e.originalTokens += originalTokens

	// Create a fact representing this turn
	turnFact := core.Fact{
		Predicate: "conversation_turn",
		Args: []interface{}{
			turn.TurnID,
			turn.Speaker,
			turn.Message,
			turn.Intent,
		},
	}

	facts := []core.Fact{turnFact}

	// Files referenced
	for _, file := range turn.Metadata.FilesReferenced {
		facts = append(facts, core.Fact{
			Predicate: "turn_references_file",
			Args:      []interface{}{turn.TurnID, file},
		})
	}

	// Error messages
	for _, errMsg := range turn.Metadata.ErrorMessages {
		facts = append(facts, core.Fact{
			Predicate: "turn_error_message",
			Args:      []interface{}{turn.TurnID, errMsg},
		})
	}

	// Topics
	for _, topic := range turn.Metadata.Topics {
		facts = append(facts, core.Fact{
			Predicate: "turn_topic",
			Args:      []interface{}{turn.TurnID, topic},
		})
	}

	// Symbols referenced
	for _, symbol := range turn.Metadata.SymbolsReferenced {
		facts = append(facts, core.Fact{
			Predicate: "turn_references_symbol",
			Args:      []interface{}{turn.TurnID, symbol},
		})
	}

	// Reference-back tracking
	if turn.Metadata.IsQuestionReferringBack && turn.Metadata.ReferencesBackToTurn != nil {
		facts = append(facts, core.Fact{
			Predicate: "turn_references_back",
			Args:      []interface{}{turn.TurnID, *turn.Metadata.ReferencesBackToTurn},
		})
	}

	// Store facts
	e.facts = append(e.facts, facts...)

	// Load into kernel if available
	if e.kernel != nil {
		if err := e.kernel.LoadFacts(facts); err != nil {
			return nil, 0, fmt.Errorf("failed to load facts: %w", err)
		}
	}

	// Estimate compressed size (~20 tokens per fact for semantic enrichment)
	compressedTokens := len(facts) * 20
	e.compressedTokens += compressedTokens

	return facts, compressedTokens, nil
}

// RetrieveContext retrieves relevant facts for a query.
// Uses simplified scoring (not the real 7-component ActivationEngine).
// Applies threshold filtering to match production behavior.
func (e *MockContextEngine) RetrieveContext(ctx context.Context, query string, tokenBudget int) ([]core.Fact, error) {
	// Activation threshold - facts need meaningful activation to be included
	// Base(50) + recency(40) = 90, so facts need relevance boost (+10+) to pass
	// Set at 100 to allow facts with relevance matches to pass even at low recency
	const activationThreshold = 100.0

	// Score all facts with simplified logic
	scoredFacts := make([]scoredFact, 0, len(e.facts))
	maxTurnID := 0

	// Find max turn ID for recency scoring
	for _, fact := range e.facts {
		if fact.Predicate == "conversation_turn" && len(fact.Args) > 0 {
			if turnID, ok := fact.Args[0].(int); ok && turnID > maxTurnID {
				maxTurnID = turnID
			}
		}
	}

	// CRITICAL FIX: Collect back-referenced turns for boosting
	// When turn N references back to turn M, facts from turn M should be boosted
	// This enables "What was the original error?" queries to retrieve old context
	referencedTurns := make(map[int]bool)
	for _, fact := range e.facts {
		if fact.Predicate == "turn_references_back" && len(fact.Args) >= 2 {
			if referencedTurn, ok := fact.Args[1].(int); ok {
				referencedTurns[referencedTurn] = true
			}
		}
	}

	// Score each fact
	for _, fact := range e.facts {
		score := 0.0

		// Base score: all facts start with 50 (matches real ActivationEngine)
		score += 50

		// Recency score: newer facts score higher (max +40)
		factTurnID := -1
		if len(fact.Args) > 0 {
			if turnID, ok := fact.Args[0].(int); ok {
				factTurnID = turnID
				if maxTurnID > 0 {
					recency := float64(turnID) / float64(maxTurnID)
					score += recency * 40
				}
			}
		}

		// CRITICAL FIX: Back-reference boost (max +50)
		// Facts from referenced turns get a major boost to overcome recency penalty
		if factTurnID >= 0 && referencedTurns[factTurnID] {
			score += 50 // Significant boost to overcome low recency
		}

		// Relevance score: keyword matching with query (max +30)
		for _, arg := range fact.Args {
			if str, ok := arg.(string); ok {
				if containsKeyword(query, str) {
					score += 30
					break // Only count once per fact
				}
			}
		}

		// Predicate priority boost
		switch fact.Predicate {
		case "turn_error_message":
			score += 25 // Errors are important
		case "turn_topic":
			score += 20 // Topics provide context
		case "turn_references_file":
			score += 15 // File refs are useful
		case "turn_references_symbol":
			score += 15 // Symbol refs are useful
		case "conversation_turn":
			score += 10 // Base conversational context
		case "turn_references_back":
			score += 30 // Back-references are highly relevant
		}

		scoredFacts = append(scoredFacts, scoredFact{fact: fact, score: score})
	}

	// Sort by score descending
	sortByScore(scoredFacts)

	// CRITICAL: Apply threshold filtering BEFORE budget selection
	// This is the key fix - only facts with meaningful activation pass
	filtered := make([]scoredFact, 0, len(scoredFacts))
	pruned := 0
	for _, sf := range scoredFacts {
		if sf.score >= activationThreshold {
			filtered = append(filtered, sf)
		} else {
			pruned++
		}
	}

	// Trim filtered facts to budget
	result := make([]core.Fact, 0, len(filtered))
	tokens := 0
	const avgTokensPerFact = 20

	for _, sf := range filtered {
		if tokens+avgTokensPerFact > tokenBudget {
			break
		}
		result = append(result, sf.fact)
		tokens += avgTokensPerFact
	}

	return result, nil
}

// GetCompressionStats returns original and compressed token counts.
func (e *MockContextEngine) GetCompressionStats() (originalTokens, compressedTokens int) {
	return e.originalTokens, e.compressedTokens
}

// GetActivationBreakdown returns nil in mock mode (no real activation scoring).
func (e *MockContextEngine) GetActivationBreakdown(factID string) *ActivationBreakdown {
	return nil // Not available in mock mode
}

// SetCampaignContext is a no-op in mock mode.
func (e *MockContextEngine) SetCampaignContext(ctx *internalcontext.CampaignActivationContext) {
	// No-op in mock mode
}

// SetIssueContext is a no-op in mock mode.
func (e *MockContextEngine) SetIssueContext(ctx *internalcontext.IssueActivationContext) {
	// No-op in mock mode
}

// Reset clears all state for a fresh test run.
func (e *MockContextEngine) Reset() error {
	e.facts = make([]core.Fact, 0)
	e.originalTokens = 0
	e.compressedTokens = 0
	return nil
}

// GetMode returns MockMode.
func (e *MockContextEngine) GetMode() EngineMode {
	return MockMode
}

// containsKeyword checks for semantic relevance between query and target.
// It uses bidirectional matching:
// 1. Short targets (topics): check if query contains target words
// 2. Long targets (messages): check if target contains query keywords
func containsKeyword(query, target string) bool {
	if len(query) == 0 || len(target) == 0 {
		return false
	}

	queryLower := toLower(query)
	targetLower := toLower(target)

	// For short targets (topics, keywords), check if query contains them
	// Also normalize hyphens to spaces for matching "original-error" with "original error"
	targetNormalized := normalizeForMatch(targetLower)
	queryNormalized := normalizeForMatch(queryLower)

	// Direct substring check
	if findSubstring(queryNormalized, targetNormalized) {
		return true
	}

	// For longer targets (error messages, etc.), check if target contains query keywords
	// Extract significant words from query (skip common words)
	queryWords := extractKeywords(queryNormalized)
	for _, word := range queryWords {
		if len(word) >= 4 && findSubstring(targetNormalized, word) {
			return true
		}
	}

	return false
}

// normalizeForMatch replaces hyphens with spaces for flexible matching
func normalizeForMatch(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '-' {
			result[i] = ' '
		} else {
			result[i] = s[i]
		}
	}
	return string(result)
}

// extractKeywords splits a string into significant words
func extractKeywords(s string) []string {
	// Common stop words to skip
	stopWords := map[string]bool{
		"the": true, "was": true, "what": true, "is": true, "a": true,
		"an": true, "and": true, "or": true, "to": true, "in": true,
		"of": true, "for": true, "we": true, "our": true, "all": true,
		"that": true, "this": true, "with": true, "how": true, "list": true,
	}

	var words []string
	var current []byte

	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' {
			current = append(current, c)
		} else if len(current) > 0 {
			word := string(current)
			if !stopWords[word] {
				words = append(words, word)
			}
			current = current[:0]
		}
	}

	if len(current) > 0 {
		word := string(current)
		if !stopWords[word] {
			words = append(words, word)
		}
	}

	return words
}

// toLower converts string to lowercase (simple ASCII)
func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		result[i] = c
	}
	return string(result)
}

// findSubstring checks if target is in query
func findSubstring(query, target string) bool {
	if len(target) > len(query) {
		return false
	}
	for i := 0; i <= len(query)-len(target); i++ {
		if query[i:i+len(target)] == target {
			return true
		}
	}
	return false
}

// sortByScore sorts scored facts by score descending (in-place)
func sortByScore(facts []scoredFact) {
	// Simple bubble sort (good enough for test harness)
	n := len(facts)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if facts[j].score < facts[j+1].score {
				facts[j], facts[j+1] = facts[j+1], facts[j]
			}
		}
	}
}
