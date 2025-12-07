// Package chat provides the interactive TUI chat interface for codeNERD.
// This file implements knowledge persistence to populate knowledge.db tables.
package chat

import (
	"codenerd/internal/core"
	ctxcompress "codenerd/internal/context"
	"codenerd/internal/perception"
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// =============================================================================
// KNOWLEDGE PERSISTENCE - Populate knowledge.db tables
// =============================================================================

// persistTurnToKnowledge populates all knowledge.db tables with turn data
// This implements the complete learning loop: Execute → Store → Learn
//
// Populates 5 tables in knowledge.db:
// 1. session_history - Full conversation history
// 2. vectors - User inputs and responses for semantic search
// 3. knowledge_graph - Entity relationships from memory operations
// 4. cold_storage - Learned Mangle facts
// 5. knowledge_atoms - High-level semantic insights
func (m Model) persistTurnToKnowledge(turn ctxcompress.Turn, intent perception.Intent, response string) {
	if m.localDB == nil {
		fmt.Printf("[Knowledge] Warning: knowledge database not configured; skipping persistence\n")
		return
	}

	// Get session ID from Model
	sessionID := m.sessionID
	if sessionID == "" {
		sessionID = "default"
	}

	// Get turn number from Model
	turnNumber := m.turnCount

	// 1. SESSION HISTORY: Store the conversation turn
	// This provides complete audit trail of all interactions
	intentJSON, _ := json.Marshal(intent)
	atomsJSON, _ := json.Marshal(turn.ControlPacket.MangleUpdates)

	if err := m.localDB.StoreSessionTurn(
		sessionID,
		turnNumber,
		turn.UserInput,
		string(intentJSON),
		response,
		string(atomsJSON),
	); err != nil {
		fmt.Printf("[Knowledge] Warning: Failed to store session turn: %v\n", err)
	}

	// 2. VECTORS: Store user input and response with semantic embeddings
	// Uses StoreVectorWithEmbedding for true semantic search when embedding engine available
	ctx := context.Background()
	userMeta := map[string]interface{}{
		"type":         "user_input",
		"session_id":   sessionID,
		"turn":         turnNumber,
		"verb":         intent.Verb,
		"category":     intent.Category,
		"content_type": "conversation", // For intelligent task type selection
	}
	if err := m.localDB.StoreVectorWithEmbedding(ctx, turn.UserInput, userMeta); err != nil {
		// Fallback to keyword-only storage if embedding fails
		m.localDB.StoreVector(turn.UserInput, userMeta)
	}

	responseMeta := map[string]interface{}{
		"type":         "assistant_response",
		"session_id":   sessionID,
		"turn":         turnNumber,
		"content_type": "conversation",
	}
	if err := m.localDB.StoreVectorWithEmbedding(ctx, response, responseMeta); err != nil {
		// Fallback to keyword-only storage if embedding fails
		m.localDB.StoreVector(response, responseMeta)
	}

	// 3. KNOWLEDGE GRAPH: Extract relationships from memory operations
	// Memory operations represent explicit knowledge the LLM wants to persist
	// Format examples: "user prefers X", "project uses Y", "concept relates_to Z"
	for _, memOp := range turn.ControlPacket.MemoryOperations {
		if memOp.Op == "store" || memOp.Op == "link" {
			// Parse memory operation to extract entity relationships
			parts := strings.SplitN(memOp.Value, " ", 3)
			if len(parts) >= 3 {
				entityA := parts[0]
				relation := parts[1]
				entityB := strings.Join(parts[2:], " ")

				meta := map[string]interface{}{
					"session_id": sessionID,
					"turn":       turnNumber,
					"source":     "memory_operation",
				}

				if err := m.localDB.StoreLink(entityA, relation, entityB, 1.0, meta); err != nil {
					fmt.Printf("[Knowledge] Warning: Failed to store knowledge link: %v\n", err)
				}
			}
		}
	}

	// 4. COLD STORAGE: Persist learned facts from Mangle updates
	// These are the logic atoms the system has derived during OODA execution
	for _, factStr := range turn.ControlPacket.MangleUpdates {
		// Parse fact string into predicate and args
		if fact, err := core.ParseSingleFact(factStr); err == nil {
			// Skip temporary/transient facts (those with low priority)
			priority := 5 // Default priority
			factType := "learned"

			// Higher priority for certain predicates
			if strings.Contains(fact.Predicate, "preference") {
				priority = 10
				factType = "preference"
			} else if strings.Contains(fact.Predicate, "constraint") {
				priority = 8
				factType = "constraint"
			} else if strings.Contains(fact.Predicate, "user_") {
				priority = 9
				factType = "user_fact"
			} else if strings.Contains(fact.Predicate, "final_action") {
				priority = 7
				factType = "action"
			}

			if err := m.localDB.StoreFact(fact.Predicate, fact.Args, factType, priority); err != nil {
				fmt.Printf("[Knowledge] Warning: Failed to store fact: %v\n", err)
			}
		}
	}

	// 5. KNOWLEDGE ATOMS: Store high-level insights from the turn
	// This captures semantic meaning beyond raw facts
	// Only store for actions, not queries (reduces noise)
	if intent.Category != "/query" {
		concept := fmt.Sprintf("%s_%s", intent.Verb, intent.Category)
		content := fmt.Sprintf("User intent: %s on %s. Response: %s",
			intent.Verb, intent.Target, truncateForStorage(response, 500))

		if err := m.localDB.StoreKnowledgeAtom(concept, content, 0.8); err != nil {
			fmt.Printf("[Knowledge] Warning: Failed to store knowledge atom: %v\n", err)
		}
	}

	fmt.Printf("[Knowledge] ✓ Persisted turn to knowledge.db: session=%s, turn=%d, facts=%d, mem_ops=%d\n",
		sessionID, turnNumber, len(turn.ControlPacket.MangleUpdates), len(turn.ControlPacket.MemoryOperations))
}

// truncateForStorage truncates long strings for storage efficiency
func truncateForStorage(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// persistActivationScores logs activation scores to activation_log table
// This enables tracking which facts are most relevant over time
func (m Model) persistActivationScores(scoredFacts []ScoredFact) {
	if m.localDB == nil {
		return
	}

	// Only log facts above activation threshold (reduces noise)
	threshold := 5.0

	for _, sf := range scoredFacts {
		if sf.Score >= threshold {
			// Create unique fact ID
			factID := fmt.Sprintf("%s_%v", sf.Fact.Predicate, sf.Fact.Args)

			if err := m.localDB.LogActivation(factID, sf.Score); err != nil {
				// Silently ignore logging errors (non-critical)
				continue
			}
		}
	}
}

// ScoredFact represents a fact with its activation score
// This mirrors the internal/context/types.go ScoredFact
type ScoredFact struct {
	Fact            core.Fact
	Score           float64
	BaseScore       float64
	RecencyScore    float64
	RelevanceScore  float64
	DependencyScore float64
}
