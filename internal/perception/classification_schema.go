package perception

import "context"

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
