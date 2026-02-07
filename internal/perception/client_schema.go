package perception

import (
	"encoding/json"
	"sync"

	"codenerd/internal/articulation"
)

var (
	piggybackSchemaOnce sync.Once
	piggybackSchemaRaw  map[string]interface{}
)

// piggybackEnvelopeRawSchema returns the raw JSON schema for PiggybackEnvelope.
// This is the base schema used by providers that support JSON schema enforcement.
//
// IMPORTANT: This schema MUST match articulation/schema.go PiggybackEnvelopeSchema.
// We parse the canonical schema constant to avoid schema drift (wiring gap class).
func piggybackEnvelopeRawSchema() map[string]interface{} {
	piggybackSchemaOnce.Do(func() {
		// Best-effort parse; fallback to a minimal schema if something goes wrong.
		if err := json.Unmarshal([]byte(articulation.PiggybackEnvelopeSchema), &piggybackSchemaRaw); err != nil {
			piggybackSchemaRaw = map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"control_packet": map[string]interface{}{"type": "object"},
					"surface_response": map[string]interface{}{
						"type": "string",
					},
				},
				"required": []string{"control_packet", "surface_response"},
			}
		}
	})

	return piggybackSchemaRaw
}

// BuildZAIPiggybackEnvelopeSchema creates the response format for Z.AI structured output.
// Z.AI only supports basic JSON mode: {"type": "json_object"}
// Schema enforcement must happen via prompt instructions, not API-level constraints.
// See: https://docs.z.ai/guides/capabilities/struct-output
func BuildZAIPiggybackEnvelopeSchema() *ZAIResponseFormat {
	return &ZAIResponseFormat{
		Type: "json_object", // Z.AI only supports "json_object" or "text"
	}
}

// BuildOpenAIPiggybackEnvelopeSchema creates full JSON schema for OpenAI-compatible APIs.
// Use this for OpenAI, xAI, and other providers that support the json_schema response
// format with strict mode.
// See: https://platform.openai.com/docs/guides/structured-outputs
func BuildOpenAIPiggybackEnvelopeSchema() *ZAIResponseFormat {
	return &ZAIResponseFormat{
		Type: "json_schema",
		JSONSchema: &ZAIJSONSchema{
			Name:   "PiggybackEnvelope",
			Strict: true,
			Schema: piggybackEnvelopeRawSchema(),
		},
	}
}

// BuildGeminiPiggybackEnvelopeSchema returns the raw schema for Gemini's responseJsonSchema.
// Gemini uses generationConfig.responseMimeType = "application/json" with a separate
// responseJsonSchema field that takes the raw schema object.
// See: https://ai.google.dev/gemini-api/docs/structured-output
func BuildGeminiPiggybackEnvelopeSchema() map[string]interface{} {
	return piggybackEnvelopeRawSchema()
}

// BuildOpenRouterPiggybackEnvelopeSchema creates the response format for OpenRouter.
// OpenRouter proxies to various providers. It uses OpenAI-compatible format but behavior
// depends on the underlying model. Most frontier models support json_schema.
// Falls back gracefully if the model doesn't support strict schema.
// See: https://openrouter.ai/docs/responses
func BuildOpenRouterPiggybackEnvelopeSchema() *ZAIResponseFormat {
	return &ZAIResponseFormat{
		Type: "json_schema",
		JSONSchema: &ZAIJSONSchema{
			Name:   "PiggybackEnvelope",
			Strict: true,
			Schema: piggybackEnvelopeRawSchema(),
		},
	}
}
