package articulation

// =============================================================================
// PIGGYBACK PROTOCOL JSON SCHEMA
// =============================================================================
// This schema defines the structure for Claude CLI's --json-schema flag.
// When using SchemaCapableLLMClient.CompleteWithSchema(), passing this schema
// guarantees the LLM returns properly structured Piggyback Protocol output.

// PiggybackEnvelopeSchema is the JSON Schema for validating Piggyback Protocol output.
// Used with Claude CLI's --json-schema flag to guarantee structured output.
//
// Structure:
//
//	{
//	  "control_packet": {
//	    "intent_classification": { category, verb, target, constraint, confidence },
//	    "mangle_updates": ["atom1", "atom2", ...],
//	    "memory_operations": [{ op, key, value }, ...],
//	    "self_correction": { triggered, hypothesis },
//	    "reasoning_trace": "step-by-step thinking"
//	  },
//	  "surface_response": "User-facing text response"
//	}
const PiggybackEnvelopeSchema = `{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "required": ["control_packet", "surface_response"],
  "additionalProperties": false,
  "properties": {
    "control_packet": {
      "type": "object",
      "required": ["intent_classification", "mangle_updates"],
      "additionalProperties": false,
      "properties": {
        "intent_classification": {
          "type": "object",
          "required": ["category", "verb", "target", "constraint", "confidence"],
          "additionalProperties": false,
          "properties": {
            "category": {
              "type": "string",
              "description": "Intent category (e.g., /query, /mutation, /instruction)"
            },
            "verb": {
              "type": "string",
              "description": "Action verb (e.g., /read, /write, /explain)"
            },
            "target": {
              "type": "string",
              "description": "Target of the action (file path, concept, etc.)"
            },
            "constraint": {
              "type": "string",
              "description": "Any constraints on the action"
            },
            "confidence": {
              "type": "number",
              "minimum": 0,
              "maximum": 1,
              "description": "Confidence score for intent classification (0-1)"
            }
          }
        },
        "mangle_updates": {
          "type": "array",
          "items": {"type": "string"},
          "description": "Mangle logic atoms to assert into the kernel"
        },
        "memory_operations": {
          "type": "array",
          "items": {
            "type": "object",
            "required": ["op", "key", "value"],
            "additionalProperties": false,
            "properties": {
              "op": {
                "type": "string",
                "enum": ["promote_to_long_term", "forget", "store_vector", "note"],
                "description": "Memory operation type"
              },
              "key": {
                "type": "string",
                "description": "Fact predicate or preference key"
              },
              "value": {
                "type": "string",
                "description": "Fact value or preference value"
              }
            }
          },
          "description": "Directives for Cold Storage operations"
        },
        "self_correction": {
          "type": "object",
          "additionalProperties": false,
          "properties": {
            "triggered": {
              "type": "boolean",
              "description": "Whether self-correction was triggered"
            },
            "hypothesis": {
              "type": "string",
              "description": "Hypothesis about what went wrong"
            }
          },
          "description": "Self-correction signals for autopoiesis"
        },
        "reasoning_trace": {
          "type": "string",
          "description": "Step-by-step reasoning (for debugging)"
        }
      }
    },
    "surface_response": {
      "type": "string",
      "minLength": 1,
      "description": "User-facing natural language response"
    }
  }
}`

// SimpleEnvelopeSchema is a minimal schema for basic validation.
// Use this for testing or when full schema validation is too strict.
const SimpleEnvelopeSchema = `{
  "type": "object",
  "required": ["control_packet", "surface_response"],
  "properties": {
    "control_packet": {"type": "object"},
    "surface_response": {"type": "string"}
  }
}`

// GetPiggybackSchema returns the appropriate schema based on strictness level.
// strict=true returns the full schema, strict=false returns the simple schema.
func GetPiggybackSchema(strict bool) string {
	if strict {
		return PiggybackEnvelopeSchema
	}
	return SimpleEnvelopeSchema
}
