package context_harness

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"codenerd/internal/core"
	internalcontext "codenerd/internal/context"
	"codenerd/internal/perception"
	"codenerd/internal/store"
)

// RealIntegrationEngine uses real codeNERD components for integration testing.
// It wires the real ActivationEngine, Compressor, and Kernel together.
type RealIntegrationEngine struct {
	kernel     *core.RealKernel
	activation *internalcontext.ActivationEngine
	compressor *internalcontext.Compressor
	store      *store.LocalStore
	llmClient  perception.LLMClient

	// Stats tracking
	mu               sync.Mutex
	originalTokens   int
	compressedTokens int
	allFacts         []core.Fact
	factScores       map[string]*ActivationBreakdown
}

// Ensure RealIntegrationEngine implements ContextEngine
var _ ContextEngine = (*RealIntegrationEngine)(nil)

// NewRealIntegrationEngine creates an engine using real codeNERD components.
func NewRealIntegrationEngine(
	kernel *core.RealKernel,
	localStorage *store.LocalStore,
	llmClient perception.LLMClient,
	config internalcontext.CompressorConfig,
) *RealIntegrationEngine {
	return &RealIntegrationEngine{
		kernel:     kernel,
		activation: internalcontext.NewActivationEngine(config),
		compressor: internalcontext.NewCompressor(kernel, localStorage, llmClient),
		store:      localStorage,
		llmClient:  llmClient,
		allFacts:   make([]core.Fact, 0),
		factScores: make(map[string]*ActivationBreakdown),
	}
}

// CompressTurn compresses a single turn into semantic facts using real components.
func (e *RealIntegrationEngine) CompressTurn(ctx context.Context, turn *Turn) ([]core.Fact, int, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Track original tokens
	originalTokens := len(turn.Message) / 4
	e.originalTokens += originalTokens

	// Create Mangle facts from turn
	facts := e.createFactsFromTurn(turn)

	// Load facts into kernel
	if err := e.kernel.LoadFacts(facts); err != nil {
		return nil, 0, fmt.Errorf("failed to load facts: %w", err)
	}

	// Mark new facts for recency tracking
	e.activation.MarkNewFacts(facts)

	// Store facts for later retrieval
	e.allFacts = append(e.allFacts, facts...)

	// Estimate compressed tokens (~20 tokens per fact)
	compressedTokens := len(facts) * 20
	e.compressedTokens += compressedTokens

	return facts, compressedTokens, nil
}

// createFactsFromTurn converts turn metadata into Mangle facts.
func (e *RealIntegrationEngine) createFactsFromTurn(turn *Turn) []core.Fact {
	facts := []core.Fact{
		{
			Predicate: "conversation_turn",
			Args: []interface{}{
				turn.TurnID,
				turn.Speaker,
				turn.Message,
				turn.Intent,
			},
		},
	}

	// Files referenced
	for _, file := range turn.Metadata.FilesReferenced {
		facts = append(facts, core.Fact{
			Predicate: "turn_references_file",
			Args:      []interface{}{turn.TurnID, file},
		})
	}

	// Symbols referenced
	for _, symbol := range turn.Metadata.SymbolsReferenced {
		facts = append(facts, core.Fact{
			Predicate: "turn_references_symbol",
			Args:      []interface{}{turn.TurnID, symbol},
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

	// Reference-back tracking
	if turn.Metadata.IsQuestionReferringBack && turn.Metadata.ReferencesBackToTurn != nil {
		facts = append(facts, core.Fact{
			Predicate: "turn_references_back",
			Args:      []interface{}{turn.TurnID, *turn.Metadata.ReferencesBackToTurn},
		})
	}

	// Campaign phase (for integration scenarios)
	if turn.CampaignPhase != "" {
		facts = append(facts, core.Fact{
			Predicate: "turn_campaign_phase",
			Args:      []interface{}{turn.TurnID, turn.CampaignPhase},
		})
	}

	return facts
}

// RetrieveContext retrieves relevant facts using real 7-component activation.
func (e *RealIntegrationEngine) RetrieveContext(ctx context.Context, query string, tokenBudget int) ([]core.Fact, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if len(e.allFacts) == 0 {
		return nil, nil
	}

	// CRITICAL FIX: Collect back-referenced turns for boosting
	// When turn N references back to turn M, facts from turn M should be boosted
	// This enables "What was the original error?" queries to retrieve old context
	referencedTurns := make(map[int]bool)
	for _, fact := range e.allFacts {
		if fact.Predicate == "turn_references_back" && len(fact.Args) >= 2 {
			if referencedTurn, ok := fact.Args[1].(int); ok {
				referencedTurns[referencedTurn] = true
			}
		}
	}

	// Create intent fact from query for activation scoring
	intentFact := &core.Fact{
		Predicate: "user_intent",
		Args:      []interface{}{"query", "retrieve", query, ""},
	}

	// Score all facts with real 7-component activation
	scored := e.activation.ScoreFacts(e.allFacts, intentFact)

	// CRITICAL FIX: Apply back-reference boost after activation scoring
	// Facts from referenced turns get a significant boost to overcome recency penalty
	const backRefBoost = 0.5 // Add 50% to score for referenced turns
	for i := range scored {
		if len(scored[i].Fact.Args) > 0 {
			if turnID, ok := scored[i].Fact.Args[0].(int); ok {
				if referencedTurns[turnID] {
					scored[i].Score += scored[i].Score * backRefBoost
				}
			}
		}
	}

	// Re-sort after applying back-reference boost
	sortScoredFacts(scored)

	// Store activation breakdowns for later inspection
	// Note: ScoredFact only exposes 4 of the 7 components currently
	for _, sf := range scored {
		factID := e.factID(sf.Fact)
		e.factScores[factID] = &ActivationBreakdown{
			FactID:          factID,
			BaseScore:       sf.BaseScore,
			RecencyBoost:    sf.RecencyScore,
			RelevanceBoost:  sf.RelevanceScore,
			DependencyBoost: sf.DependencyScore,
			// Campaign, Session, Issue scores are computed internally
			// but not currently exposed in ScoredFact. When needed,
			// ScoredFact can be extended to include these.
			CampaignBoost: 0, // TODO: expose from ScoredFact
			SessionBoost:  0, // TODO: expose from ScoredFact
			IssueBoost:    0, // TODO: expose from ScoredFact
			TotalScore:    sf.Score,
		}
	}

	// Select within budget
	selected := e.activation.SelectWithinBudget(scored, tokenBudget)

	// Convert back to facts
	result := make([]core.Fact, len(selected))
	for i, sf := range selected {
		result[i] = sf.Fact
	}

	return result, nil
}

// sortScoredFacts sorts scored facts by score descending
func sortScoredFacts(facts []internalcontext.ScoredFact) {
	n := len(facts)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if facts[j].Score < facts[j+1].Score {
				facts[j], facts[j+1] = facts[j+1], facts[j]
			}
		}
	}
}

// GetCompressionStats returns original and compressed token counts.
func (e *RealIntegrationEngine) GetCompressionStats() (originalTokens, compressedTokens int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.originalTokens, e.compressedTokens
}

// GetActivationBreakdown returns the 7-component scoring breakdown for a fact.
func (e *RealIntegrationEngine) GetActivationBreakdown(factID string) *ActivationBreakdown {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.factScores[factID]
}

// SetCampaignContext sets the campaign context for campaign-aware activation.
func (e *RealIntegrationEngine) SetCampaignContext(ctx *internalcontext.CampaignActivationContext) {
	e.activation.SetCampaignContext(ctx)
}

// SetIssueContext sets the issue context for issue-driven activation.
func (e *RealIntegrationEngine) SetIssueContext(ctx *internalcontext.IssueActivationContext) {
	e.activation.SetIssueContext(ctx)
}

// Reset clears all state for a fresh test run.
func (e *RealIntegrationEngine) Reset() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.originalTokens = 0
	e.compressedTokens = 0
	e.allFacts = make([]core.Fact, 0)
	e.factScores = make(map[string]*ActivationBreakdown)

	// Clear activation engine state
	e.activation.ClearCampaignContext()
	e.activation.ClearIssueContext()

	return nil
}

// GetMode returns RealMode.
func (e *RealIntegrationEngine) GetMode() EngineMode {
	return RealMode
}

// factID creates a unique identifier for a fact.
func (e *RealIntegrationEngine) factID(fact core.Fact) string {
	if len(fact.Args) > 0 {
		if turnID, ok := fact.Args[0].(int); ok {
			return fmt.Sprintf("turn_%d_%s", turnID, fact.Predicate)
		}
	}
	return fact.Predicate
}

// LiveLLMResponse represents a response from the live LLM.
type LiveLLMResponse struct {
	SurfaceText     string
	ContextFeedback *ContextFeedback
	ResponseTokens  int
}

// GenerateAssistantResponse generates a real assistant response using the LLM.
// This is used in live LLM mode to get real context_feedback from Gemini.
func (e *RealIntegrationEngine) GenerateAssistantResponse(ctx context.Context, userMessage string, intent string, facts []core.Fact) (*LiveLLMResponse, error) {
	if e.llmClient == nil {
		return nil, fmt.Errorf("no LLM client configured for live mode")
	}

	// Build context from compressed facts
	factContext := buildFactContext(facts)

	// System prompt that requests piggyback-style response with context feedback
	systemPrompt := fmt.Sprintf(`You are a coding assistant in a context harness test.

## Context Facts
%s

## Instructions
Respond to the user message. After your response, provide feedback on which context facts were helpful vs noise.

Your response MUST be valid JSON with this structure:
{
  "surface_response": "Your helpful response to the user",
  "control_packet": {
    "intent_classification": {
      "category": "code",
      "verb": "%s",
      "target": "",
      "confidence": 0.95
    },
    "context_feedback": {
      "overall_usefulness": 0.8,
      "helpful_facts": ["fact_name_1", "fact_name_2"],
      "noise_facts": ["noisy_fact_1"],
      "missing_context": ""
    }
  }
}

Rate context usefulness from 0.0 to 1.0. List predicate names (like "turn_error_message", "file_topology", "test_state") that were helpful or noise.`, factContext, intent)

	// Call the LLM
	response, err := e.llmClient.CompleteWithSystem(ctx, systemPrompt, userMessage)
	if err != nil {
		return nil, fmt.Errorf("LLM call failed: %w", err)
	}

	// Parse the response
	return parseLiveLLMResponse(response)
}

// buildFactContext formats facts for LLM context.
func buildFactContext(facts []core.Fact) string {
	if len(facts) == 0 {
		return "(no context facts)"
	}

	var sb strings.Builder
	for _, fact := range facts {
		sb.WriteString(fmt.Sprintf("- %s(", fact.Predicate))
		for i, arg := range fact.Args {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(fmt.Sprintf("%v", arg))
		}
		sb.WriteString(")\n")
	}
	return sb.String()
}

// parseLiveLLMResponse parses the JSON response from the LLM.
func parseLiveLLMResponse(response string) (*LiveLLMResponse, error) {
	// Try to extract JSON from response (may have markdown wrapper)
	jsonStr := extractJSON(response)
	if jsonStr == "" {
		// Fallback: treat entire response as surface text
		return &LiveLLMResponse{
			SurfaceText:    response,
			ResponseTokens: len(response) / 4,
		}, nil
	}

	// Parse the JSON
	var parsed struct {
		SurfaceResponse string `json:"surface_response"`
		ControlPacket   struct {
			IntentClassification struct {
				Category   string  `json:"category"`
				Verb       string  `json:"verb"`
				Target     string  `json:"target"`
				Confidence float64 `json:"confidence"`
			} `json:"intent_classification"`
			ContextFeedback *struct {
				OverallUsefulness float64  `json:"overall_usefulness"`
				HelpfulFacts      []string `json:"helpful_facts"`
				NoiseFacts        []string `json:"noise_facts"`
				MissingContext    string   `json:"missing_context"`
			} `json:"context_feedback"`
		} `json:"control_packet"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		// Fallback on parse error
		return &LiveLLMResponse{
			SurfaceText:    response,
			ResponseTokens: len(response) / 4,
		}, nil
	}

	result := &LiveLLMResponse{
		SurfaceText:    parsed.SurfaceResponse,
		ResponseTokens: len(response) / 4,
	}

	if parsed.ControlPacket.ContextFeedback != nil {
		result.ContextFeedback = &ContextFeedback{
			OverallUsefulness: parsed.ControlPacket.ContextFeedback.OverallUsefulness,
			HelpfulFacts:      parsed.ControlPacket.ContextFeedback.HelpfulFacts,
			NoiseFacts:        parsed.ControlPacket.ContextFeedback.NoiseFacts,
			MissingContext:    parsed.ControlPacket.ContextFeedback.MissingContext,
		}
	}

	return result, nil
}

// extractJSON extracts JSON from a potentially markdown-wrapped response.
func extractJSON(response string) string {
	// Try to find JSON in code blocks
	if start := strings.Index(response, "```json"); start != -1 {
		start += 7
		if end := strings.Index(response[start:], "```"); end != -1 {
			return strings.TrimSpace(response[start : start+end])
		}
	}

	// Try to find raw JSON (starts with {)
	if start := strings.Index(response, "{"); start != -1 {
		// Find matching closing brace
		depth := 0
		for i := start; i < len(response); i++ {
			switch response[i] {
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					return response[start : i+1]
				}
			}
		}
	}

	return ""
}
