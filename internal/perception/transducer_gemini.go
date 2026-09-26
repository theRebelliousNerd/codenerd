package perception

import (
	"codenerd/internal/logging"
	"codenerd/internal/types"
	"context"
)

// understandingSchema is the JSON schema for Gemini structured output.
// This ensures the LLM returns a valid UnderstandingEnvelope every time.
// Note: Gemini 3 has a max schema depth of 6, so signals and
// suggested_approach stay shallow (one object level each) and optional:
// older prompts that omit them still validate.
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
        "confidence": {"type": "number"},
        "signals": {
          "type": "object",
          "properties": {
            "is_question": {"type": "boolean"},
            "is_hypothetical": {"type": "boolean"},
            "is_multi_step": {"type": "boolean"},
            "is_negated": {"type": "boolean"},
            "requires_confirmation": {"type": "boolean"},
            "urgency": {"type": "string"}
          }
        },
        "suggested_approach": {
          "type": "object",
          "properties": {
            "mode": {"type": "string"},
            "primary_shard": {"type": "string"}
          }
        },
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

// Compile-time proof that the wrapper still exposes the kernel port: the
// factory wires routing through TransducerWithKernel regardless of which
// concrete transducer NewUnderstandingTransducer returned.
var _ TransducerWithKernel = (*GeminiThinkingTransducer)(nil)

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
// It honors the same contracts as the base: blank input short-circuits,
// oversized input truncates, failures degrade to /explain with a nil error
// (callers do not expect an error here), and routing derives from Mangle.
func (t *GeminiThinkingTransducer) ParseIntentWithContext(ctx context.Context, input string, history []ConversationTurn) (Intent, error) {
	if intent, done := emptyInputFallback(input); done {
		return intent, nil
	}
	if truncated, didTruncate := truncateClassificationInput(input); didTruncate {
		logging.Perception("[GeminiTransducer] input truncated from %d to %d", len(input), len(truncated))
		input = truncated
	}

	// 1. Initialize logic (same as base)
	t.initialize(ctx)

	// 2. Semantic grounding (same as base)
	var semanticMatches []SemanticMatch
	if SharedSemanticClassifier != nil {
		matches, err := SharedSemanticClassifier.Classify(ctx, input)
		if err != nil {
			_ = matches
			logging.PerceptionDebug("[GeminiTransducer] semantic classification failed: %v (continuing LLM-only)", err)
		} else {
			semanticMatches = matches
		}
	}

	// 3. History is already in ConversationTurn format

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

	// 5. Build Final Prompt
	userPrompt := t.llmTransducer.BuildPrompt(input, history, semanticMatches, types.GetSessionContext(ctx), t.getStrategicContext())

	// 6a. Try structured output first (most reliable for JSON)
	if schemaClient, ok := t.client.(schemaCapableClient); ok && schemaClient.SchemaCapable() {
		logging.Perception("[GeminiTransducer] Client type=%T, attempting structured output", t.client)
		if u, err := t.classifyViaSchema(ctx, schemaClient, thinkingWrapper, userPrompt); err == nil {
			return t.finishUnderstanding(ctx, u), nil
		} else {
			logging.PerceptionWarn("Structured classification failed: %v, falling back to free-form", err)
		}
	}

	// 6b. Fallback: free-form completion with JSON extraction.
	// parseResponse finds the LAST valid JSON object, which is exactly the
	// "[Thoughts...] { JSON }" shape Thinking models produce.
	rawResponse, err := t.client.CompleteWithSystem(ctx, thinkingWrapper, userPrompt)
	if err != nil {
		logging.Get(logging.CategoryPerception).Warn("Gemini classification failed: %v", err)
		return degradedClassificationIntent(err), nil
	}
	logging.PerceptionDebug("Raw Gemini Thinking Response: %s", rawResponse)

	u, err := t.llmTransducer.parseResponse(rawResponse)
	if err != nil {
		logging.Get(logging.CategoryPerception).Warn("Gemini response parse failed: %v", err)
		return degradedClassificationIntent(err), nil
	}
	return t.finishUnderstanding(ctx, u), nil
}

// classifyViaSchema runs one schema-enforced classification call and parses
// the result through the shared parseResponse path (envelope detection,
// normalization), so the schema and free-form paths cannot disagree on shape.
func (t *GeminiThinkingTransducer) classifyViaSchema(ctx context.Context, schemaClient schemaCapableClient, systemPrompt, userPrompt string) (*Understanding, error) {
	rawResponse, err := schemaClient.CompleteWithSchema(ctx, systemPrompt, userPrompt, understandingSchema)
	if err != nil {
		return nil, err
	}
	logging.Perception("[GeminiTransducer] Raw structured response (len=%d)", len(rawResponse))

	// Even structured output might carry thinking preamble; ExtractCleanJSON
	// inside parseResponse finds the JSON. Fall back to raw when extraction
	// finds nothing so parse errors surface the real payload.
	if cleanJSON := ExtractCleanJSON(rawResponse); cleanJSON != "" {
		rawResponse = cleanJSON
	}
	return t.llmTransducer.parseResponse(rawResponse)
}

// finishUnderstanding derives Mangle routing for a parsed understanding,
// caches it for debugging, and converts it to the legacy Intent. Shared by
// the schema and free-form paths so neither can skip routing.
func (t *GeminiThinkingTransducer) finishUnderstanding(ctx context.Context, u *Understanding) Intent {
	t.llmTransducer.deriveRouting(ctx, u)
	t.mu.Lock()
	t.lastUnderstanding = u
	t.mu.Unlock()
	return t.understandingToIntent(u)
}
