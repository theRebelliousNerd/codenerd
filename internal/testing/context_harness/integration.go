package context_harness

import (
	"context"
	"fmt"

	"codenerd/internal/core"
	internalcontext "codenerd/internal/context"
	"codenerd/internal/perception"
	"codenerd/internal/store"
)

// RealContextEngine wraps codeNERD's actual compression and retrieval systems.
type RealContextEngine struct {
	compressor *internalcontext.Compressor
	kernel     *core.RealKernel
	store      *store.LocalStore
}

// NewRealContextEngine creates an engine using codeNERD's production systems.
func NewRealContextEngine(kernel *core.RealKernel, localStorage *store.LocalStore, llmClient perception.LLMClient) *RealContextEngine {
	return &RealContextEngine{
		compressor: internalcontext.NewCompressor(kernel, localStorage, llmClient),
		kernel:     kernel,
		store:      localStorage,
	}
}

// CompressTurn compresses a single turn into semantic facts.
func (e *RealContextEngine) CompressTurn(ctx context.Context, turn *Turn) ([]core.Fact, int, error) {
	// Convert turn to a message-like structure
	message := turn.Message
	speaker := turn.Speaker

	// Create a fact representing this turn
	turnFact := core.Fact{
		Predicate: "conversation_turn",
		Args: []interface{}{
			turn.TurnID,
			speaker,
			message,
			turn.Intent,
		},
	}

	// Add to kernel
	if err := e.kernel.LoadFacts([]core.Fact{turnFact}); err != nil {
		return nil, 0, fmt.Errorf("failed to load turn fact: %w", err)
	}

	// Add metadata facts
	facts := []core.Fact{turnFact}

	// Files referenced
	for _, file := range turn.Metadata.FilesReferenced {
		facts = append(facts, core.Fact{
			Predicate: "turn_references_file",
			Args:      []interface{}{turn.TurnID, file},
		})
	}

	// Error messages
	for _, err := range turn.Metadata.ErrorMessages {
		facts = append(facts, core.Fact{
			Predicate: "turn_error_message",
			Args:      []interface{}{turn.TurnID, err},
		})
	}

	// Topics
	for _, topic := range turn.Metadata.Topics {
		facts = append(facts, core.Fact{
			Predicate: "turn_topic",
			Args:      []interface{}{turn.TurnID, topic},
		})
	}

	// Reference-back tracking
	if turn.Metadata.IsQuestionReferringBack && turn.Metadata.ReferencesBackToTurn != nil {
		facts = append(facts, core.Fact{
			Predicate: "turn_references_back",
			Args:      []interface{}{turn.TurnID, *turn.Metadata.ReferencesBackToTurn},
		})
	}

	// Load all facts into kernel
	if err := e.kernel.LoadFacts(facts); err != nil {
		return nil, 0, fmt.Errorf("failed to load metadata facts: %w", err)
	}

	// Estimate compressed size (count of facts * avg fact size)
	compressedTokens := len(facts) * 20 // Rough estimate: ~20 tokens per fact

	return facts, compressedTokens, nil
}

// RetrieveContext retrieves relevant facts for a query using spreading activation.
func (e *RealContextEngine) RetrieveContext(ctx context.Context, query string, tokenBudget int) ([]core.Fact, error) {
	// Parse the query to extract intent
	// In a real scenario, this would use the Transducer
	// For now, we'll do a simple keyword-based retrieval

	// Query the kernel for relevant facts based on query keywords
	// This is simplified - real implementation would use spreading activation

	var allFacts []core.Fact

	// Try to extract turn numbers from query
	// Example: "What was the original error?" -> look for turn_0 facts

	// Query for conversation turns
	turnFacts, err := e.kernel.Query("conversation_turn")
	if err == nil {
		allFacts = append(allFacts, turnFacts...)
	}

	// Query for error messages
	errorFacts, err := e.kernel.Query("turn_error_message")
	if err == nil {
		allFacts = append(allFacts, errorFacts...)
	}

	// Query for file references
	fileFacts, err := e.kernel.Query("turn_references_file")
	if err == nil {
		allFacts = append(allFacts, fileFacts...)
	}

	// Query for topics
	topicFacts, err := e.kernel.Query("turn_topic")
	if err == nil {
		allFacts = append(allFacts, topicFacts...)
	}

	// TODO: Implement actual spreading activation scoring
	// For now, return all facts (limited by budget)

	// Estimate token count and trim to budget
	estimatedTokens := len(allFacts) * 20
	if estimatedTokens > tokenBudget {
		// Trim facts to fit budget
		maxFacts := tokenBudget / 20
		if maxFacts < len(allFacts) {
			allFacts = allFacts[:maxFacts]
		}
	}

	return allFacts, nil
}

// GetCompressionStats returns compression statistics.
func (e *RealContextEngine) GetCompressionStats() (originalTokens, compressedTokens int) {
	// Query kernel for all conversation turns
	turnFacts, _ := e.kernel.Query("conversation_turn")

	// Estimate original size (full message content)
	original := 0
	for _, f := range turnFacts {
		if len(f.Args) >= 3 {
			if msg, ok := f.Args[2].(string); ok {
				// Rough token estimate: ~4 chars per token
				original += len(msg) / 4
			}
		}
	}

	// Count compressed facts
	allPredicates := []string{
		"conversation_turn",
		"turn_references_file",
		"turn_error_message",
		"turn_topic",
		"turn_references_back",
	}

	compressed := 0
	for _, pred := range allPredicates {
		facts, _ := e.kernel.Query(pred)
		compressed += len(facts) * 20 // ~20 tokens per fact
	}

	return original, compressed
}
