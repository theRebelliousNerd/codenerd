package perception

import (
	"codenerd/internal/logging"
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// understandingSchema is the JSON schema for Gemini structured output.
// This ensures the LLM returns a valid UnderstandingEnvelope every time.
// Note: Gemini 3 has a max schema depth of 6, so we use a flattened version.
const understandingSchema = `{
  "type": "object",
  "properties": {
    "understanding": {
      "type": "object",
      "properties": {
        "primary_intent": {"type": "string"},
        "semantic_type": {"type": "string"},
        "action_type": {"type": "string"},
        "domain": {"type": "string"},
        "scope": {
          "type": "object",
          "properties": {
            "level": {"type": "string"},
            "target": {"type": "string"},
            "file": {"type": "string"},
            "symbol": {"type": "string"}
          }
        },
        "user_constraints": {"type": "array", "items": {"type": "string"}},
        "implicit_assumptions": {"type": "array", "items": {"type": "string"}},
        "confidence": {"type": "number"},
        "surface_response": {"type": "string"}
      },
      "required": ["primary_intent", "semantic_type", "action_type", "domain", "confidence"]
    },
    "surface_response": {"type": "string"}
  },
  "required": ["understanding"]
}`

// schemaCapableClient is a local interface for clients that support schema-enforced completion.
// This avoids import cycles with the core package.
type schemaCapableClient interface {
	SchemaCapable() bool
	CompleteWithSchema(ctx context.Context, systemPrompt, userPrompt, jsonSchema string) (string, error)
}

// GeminiThinkingTransducer is a specialized transducer for Gemini models with Thinking enabled.
// It handles the unique output format (interspersed thoughts + JSON) cleanly.
type GeminiThinkingTransducer struct {
	*UnderstandingTransducer
}

// NewGeminiThinkingTransducer creates a new specialized transducer.
func NewGeminiThinkingTransducer(base *UnderstandingTransducer) *GeminiThinkingTransducer {
	return &GeminiThinkingTransducer{
		UnderstandingTransducer: base,
	}
}

// ParseIntent overrides the base implementation to ensure our ParseIntentWithContext is called.
// This is required because Go struct embedding doesn't provide virtual dispatch.
func (t *GeminiThinkingTransducer) ParseIntent(ctx context.Context, input string) (Intent, error) {
	return t.ParseIntentWithContext(ctx, input, nil)
}

// ParseIntentWithContext overrides the generic implementation to handle Gemini Thinking output.
func (t *GeminiThinkingTransducer) ParseIntentWithContext(ctx context.Context, input string, history []ConversationTurn) (Intent, error) {
	// 1. Initialize logic (same as base)
	t.initialize(ctx)

	// 2. Semantic grounding (same as base)
	if SharedSemanticClassifier != nil {
		matches, err := SharedSemanticClassifier.Classify(ctx, input)
		if err != nil {
			_ = matches
		}
	}

	// 3. Prepare History
	var turns []Turn
	for _, h := range history {
		turns = append(turns, Turn{
			Role:    h.Role,
			Content: h.Content,
		})
	}

	// 4. Specialized Gemini Thinking Prompt
	// We wrap the standard prompt with specific instructions for Thinking models
	basePrompt := getUnderstandingPrompt(ctx, t.promptAssembler)
	thinkingWrapper := `
IMPORTANT: You are a model with "Thinking" capabilities enabled.
1. You MUST first think about the user's request, analyzing the nuance, intent, and constraints.
2. Your output MUST contain your thoughts followed by the valid JSON object.
3. The JSON object must be the LAST thing you output.
4. Do NOT output markdown code blocks for the JSON (e.g. no triple backticks). Just the raw JSON object at the end.

` + basePrompt

	// 5. Call LLM directly via client to avoid double-wrapping in LLMTransducer if possible,
	// but reusing LLMTransducer logic is safer for consistency. We just need to handle the output.
	// Actually, we can reuse LLMTransducer but we need to intercept the *parsing*.
	// Since LLMTransducer.Understand calls parseResponse which we can't easily override without
	// changing the struct, we will reimplement the 'Understand' logic here using the client directly.

	// 5. Build Final Prompt
	// We reconstruct the user prompt manually here since LLMTransducer doesn't expose it publically.
	var sb strings.Builder

	// Include relevant history for context
	if len(turns) > 0 {
		sb.WriteString("## Recent Conversation\n\n")
		// Only include last few turns to stay focused
		start := 0
		if len(turns) > 5 {
			start = len(turns) - 5
		}
		for _, turn := range turns[start:] {
			sb.WriteString(fmt.Sprintf("**%s**: %s\n\n", turn.Role, turn.Content))
		}
		sb.WriteString("---\n\n")
	}

	sb.WriteString("## Current Request\n\n")
	sb.WriteString(input) // Use original input, not formatted USER: line, to match LLMTransducer style

	userPrompt := sb.String()

	// 5a. Try structured output first (most reliable for JSON)
	var envelope UnderstandingEnvelope
	schemaClient, ok := t.client.(schemaCapableClient)
	logging.Perception("[GeminiTransducer] Client type=%T, schemaCapableClient check: ok=%t, schema_capable=%t", t.client, ok, ok && schemaClient.SchemaCapable())
	if ok && schemaClient.SchemaCapable() {
		rawResponse, err := schemaClient.CompleteWithSchema(ctx, thinkingWrapper, userPrompt, understandingSchema)
		if err == nil {
			logging.Perception("[GeminiTransducer] Raw structured response (len=%d): %s", len(rawResponse), rawResponse)

			// Even structured output might have markdown or extra text due to thinking mode
			// Try to extract clean JSON first
			cleanJSON := extractLastJSON(rawResponse)
			if cleanJSON == "" {
				cleanJSON = rawResponse // Fallback to raw if no JSON found
			}

			// Structured output should already be valid JSON
			if err := json.Unmarshal([]byte(cleanJSON), &envelope); err != nil {
				logging.PerceptionDebug("Structured envelope parse failed: %v, trying understanding", err)
				// Try just the understanding object
				var understanding Understanding
				if err2 := json.Unmarshal([]byte(cleanJSON), &understanding); err2 == nil {
					envelope.Understanding = understanding
					envelope.SurfaceResponse = understanding.SurfaceResponse
				} else {
					logging.PerceptionWarn("Structured output parse failed, falling back to free-form: %v", err)
					goto fallback
				}
			}
			t.lastUnderstanding = &envelope.Understanding
			return t.understandingToIntent(&envelope.Understanding), nil
		}
		logging.PerceptionWarn("CompleteWithSchema failed: %v, falling back to free-form", err)
	}

fallback:
	// 5b. Fallback: free-form completion with manual JSON extraction
	rawResponse, err := t.client.CompleteWithSystem(ctx, thinkingWrapper, userPrompt)
	if err != nil {
		return Intent{}, fmt.Errorf("Gemini classification failed: %w", err)
	}

	logging.PerceptionDebug("Raw Gemini Thinking Response: %s", rawResponse)

	// 6. Specialized Parsing for Thinking Output
	// We expect: [Thoughts...] { JSON }
	// We find the *last* valid JSON object.
	jsonStr := extractLastJSON(rawResponse)
	if jsonStr == "" {
		return Intent{}, fmt.Errorf("failed to extract JSON from thinking response")
	}

	logging.PerceptionDebug("JSON String: %s", jsonStr)
	if err := json.Unmarshal([]byte(jsonStr), &envelope); err != nil {
		logging.PerceptionDebug("Envelope parse failed: %v", err)
		// Fallback: try unmarshaling just Understanding if Envelope fails
		var understanding Understanding
		if err2 := json.Unmarshal([]byte(jsonStr), &understanding); err2 == nil {
			envelope.Understanding = understanding
			envelope.SurfaceResponse = understanding.SurfaceResponse // Backfill
			logging.PerceptionDebug("Fallback parse succeeded")
		} else {
			return Intent{}, fmt.Errorf("failed to parse JSON formatting: %w", err)
		}
	} else {
		logging.PerceptionDebug("Envelope parsed: %+v", envelope)
	}

	// 7. Store for debugging
	t.lastUnderstanding = &envelope.Understanding

	// 8. Convert to Intent
	return t.understandingToIntent(&envelope.Understanding), nil
}

// extractLastJSON finds the last valid JSON object in a string.
// It handles cases where the LLM wraps JSON in markdown code fences.
func extractLastJSON(s string) string {
	// First, try to strip markdown code fences if present
	cleaned := stripMarkdownCodeFences(s)

	// Find the last closing brace
	end := strings.LastIndex(cleaned, "}")
	if end == -1 {
		return ""
	}

	// Scan backwards to find the matching opening brace
	balance := 0
	for i := end; i >= 0; i-- {
		switch cleaned[i] {
		case '}':
			balance++
		case '{':
			balance--
		}

		if balance == 0 && cleaned[i] == '{' {
			// Found the matching opening brace
			candidate := cleaned[i : end+1]
			if json.Valid([]byte(candidate)) {
				return candidate
			}
			// If invalid (e.g. valid brace pair but content is garbage),
			// it usually means we're inside a string or comment, or the JSON is malformed.
			// But since we are looking for the OUTERMOST object that ends at 'end',
			// this must be it. If it's not valid, then no valid JSON ends at 'end'.
			return ""
		}
	}

	return ""
}

// stripMarkdownCodeFences removes markdown code fence wrapping from a string.
// Handles ```json, ```, and variations with language specifiers.
func stripMarkdownCodeFences(s string) string {
	// Check for ```json or ``` at start and end
	trimmed := strings.TrimSpace(s)

	// Common patterns: ```json\n...\n``` or just ```\n...\n```
	if strings.HasPrefix(trimmed, "```") {
		// Find end of first line (after opening fence)
		firstNewline := strings.Index(trimmed, "\n")
		if firstNewline != -1 {
			// Find closing fence
			lastFence := strings.LastIndex(trimmed, "```")
			if lastFence > firstNewline {
				// Extract content between fences
				content := trimmed[firstNewline+1 : lastFence]
				return strings.TrimSpace(content)
			}
		}
	}

	return s
}
