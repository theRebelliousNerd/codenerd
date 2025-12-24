package perception

// BuildPiggybackEnvelopeSchema creates the response format for Z.AI structured output.
// Z.AI only supports basic JSON mode: {"type": "json_object"}
// Schema enforcement must happen via prompt instructions, not API-level constraints.
// See: https://docs.z.ai/guides/capabilities/struct-output
func BuildPiggybackEnvelopeSchema() *ZAIResponseFormat {
	return &ZAIResponseFormat{
		Type: "json_object", // Z.AI only supports "json_object" or "text"
		// Note: Z.AI does NOT support json_schema with strict mode like OpenAI
		// The PiggybackEnvelope schema is enforced via system prompt instructions
	}
}

// BuildOpenAIPiggybackEnvelopeSchema creates full JSON schema for OpenAI-compatible APIs.
// Use this for OpenAI, OpenRouter with OpenAI models, and other providers that support
// the json_schema response format with strict mode.
func BuildOpenAIPiggybackEnvelopeSchema() *ZAIResponseFormat {
	return &ZAIResponseFormat{
		Type: "json_schema",
		JSONSchema: &ZAIJSONSchema{
			Name:   "PiggybackEnvelope",
			Strict: true,
			Schema: map[string]interface{}{
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
								"required":             []string{"triggered", "hypothesis"},
								"additionalProperties": false,
							},
						},
						"required":             []string{"intent_classification", "mangle_updates", "memory_operations", "self_correction"},
						"additionalProperties": false,
					},
					"surface_response": map[string]interface{}{
						"type": "string",
					},
				},
				"required":             []string{"control_packet", "surface_response"},
				"additionalProperties": false,
			},
		},
	}
}
