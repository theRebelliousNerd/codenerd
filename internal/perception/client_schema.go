package perception

import (
	"encoding/json"
	"sync"

	"codenerd/internal/articulation"
)

var (
	piggybackSchemaOnce sync.Once
	piggybackSchemaRaw  map[string]any
)

// piggybackEnvelopeRawSchema returns the raw JSON schema for PiggybackEnvelope.
// This is the base schema used by providers that support JSON schema enforcement.
//
// IMPORTANT: This schema MUST match articulation/schema.go PiggybackEnvelopeSchema.
// We parse the canonical schema constant to avoid schema drift (wiring gap class).
func piggybackEnvelopeRawSchema() map[string]any {
	piggybackSchemaOnce.Do(func() {
		// Best-effort parse; fallback to a minimal schema if something goes wrong.
		if err := json.Unmarshal([]byte(articulation.PiggybackEnvelopeSchema), &piggybackSchemaRaw); err != nil {
			piggybackSchemaRaw = map[string]any{
				"type": "object",
				"properties": map[string]any{
					"control_packet": map[string]any{"type": "object"},
					"surface_response": map[string]any{
						"type": "string",
					},
				},
				"required": []string{"control_packet", "surface_response"},
			}
		}
	})

	return piggybackSchemaRaw
}

// strictObjectSchema returns a deep copy of raw with "additionalProperties":
// false stamped on every object node that declares properties.
//
// Strict structured output requires this. OpenAI's strict mode documents it,
// and Meta enforces it hard: without the key the request is rejected outright
// with 400 `'additionalProperties' is required to be supplied and to be false.`
// (observed on ~1% of a cold start's batches, and NOT caught by the
// retry-without-response_format fallback, whose matcher looks for
// "response_format"/"json_schema" while Meta reports `"param":"schema"`).
//
// The copy is mandatory: piggybackEnvelopeRawSchema caches one shared map
// behind a sync.Once, and concurrent callers must not mutate it.
func strictObjectSchema(raw map[string]any) map[string]any {
	out := make(map[string]any, len(raw)+1)
	for k, v := range raw {
		switch typed := v.(type) {
		case map[string]any:
			out[k] = strictObjectSchema(typed)
		case []any:
			cp := make([]any, len(typed))
			for i, item := range typed {
				if m, ok := item.(map[string]any); ok {
					cp[i] = strictObjectSchema(m)
				} else {
					cp[i] = item
				}
			}
			out[k] = cp
		default:
			out[k] = v
		}
	}
	// Only object nodes take additionalProperties, and only those that actually
	// declare a property set — stamping it on a free-form object would forbid
	// every field it is meant to accept.
	if out["type"] == "object" {
		if _, hasProps := out["properties"]; hasProps {
			out["additionalProperties"] = false
		}
	}
	return out
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
			Schema: strictObjectSchema(piggybackEnvelopeRawSchema()),
		},
	}
}

// BuildGeminiPiggybackEnvelopeSchema returns the raw schema for Gemini's responseJsonSchema.
// Gemini uses generationConfig.responseMimeType = "application/json" with a separate
// responseJsonSchema field that takes the raw schema object.
// See: https://ai.google.dev/gemini-api/docs/structured-output
func BuildGeminiPiggybackEnvelopeSchema() map[string]any {
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
			Schema: strictObjectSchema(piggybackEnvelopeRawSchema()),
		},
	}
}
