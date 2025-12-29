package perception

// piggybackEnvelopeRawSchema returns the raw JSON schema for PiggybackEnvelope.
// This is the base schema used by providers that support JSON schema enforcement.
// IMPORTANT: This schema must match articulation/schema.go PiggybackEnvelopeSchema.
func piggybackEnvelopeRawSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"control_packet": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"intent_classification": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"category":   map[string]interface{}{"type": "string"},
							"verb":       map[string]interface{}{"type": "string"},
							"target":     map[string]interface{}{"type": "string"},
							"constraint": map[string]interface{}{"type": "string"},
							"confidence": map[string]interface{}{"type": "number"},
						},
						"required":             []string{"category", "verb", "target", "constraint", "confidence"},
						"additionalProperties": false,
					},
					"mangle_updates": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "string",
						},
					},
					"memory_operations": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"op":    map[string]interface{}{"type": "string"},
								"key":   map[string]interface{}{"type": "string"},
								"value": map[string]interface{}{"type": "string"},
							},
							"required":             []string{"op", "key", "value"},
							"additionalProperties": false,
						},
					},
					"self_correction": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"triggered":  map[string]interface{}{"type": "boolean"},
							"hypothesis": map[string]interface{}{"type": "string"},
						},
						"additionalProperties": false,
					},
					"reasoning_trace": map[string]interface{}{
						"type":        "string",
						"description": "Step-by-step reasoning for debugging",
					},
					"knowledge_requests": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"specialist": map[string]interface{}{"type": "string"},
								"query":      map[string]interface{}{"type": "string"},
								"purpose":    map[string]interface{}{"type": "string"},
								"priority":   map[string]interface{}{"type": "string"},
							},
							"required":             []string{"specialist", "query"},
							"additionalProperties": false,
						},
					},
					"context_feedback": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"overall_usefulness": map[string]interface{}{
								"type":        "number",
								"description": "How useful was the provided context (0.0-1.0)",
							},
							"helpful_facts": map[string]interface{}{
								"type":  "array",
								"items": map[string]interface{}{"type": "string"},
							},
							"noise_facts": map[string]interface{}{
								"type":  "array",
								"items": map[string]interface{}{"type": "string"},
							},
							"missing_context": map[string]interface{}{
								"type":        "string",
								"description": "What context would have been helpful",
							},
						},
						"additionalProperties": false,
					},
				},
				"required":             []string{"intent_classification", "mangle_updates"},
				"additionalProperties": false,
			},
			"surface_response": map[string]interface{}{
				"type": "string",
			},
		},
		"required":             []string{"control_packet", "surface_response"},
		"additionalProperties": false,
	}
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
