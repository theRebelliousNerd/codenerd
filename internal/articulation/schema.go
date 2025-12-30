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
//	    "reasoning_trace": "step-by-step thinking",
//	    "knowledge_requests": [{ specialist, query, purpose, priority }, ...],
//	    "context_feedback": { overall_usefulness, helpful_facts, noise_facts, missing_context }
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
        },
        "knowledge_requests": {
          "type": "array",
          "items": {
            "type": "object",
            "required": ["specialist", "query"],
            "additionalProperties": false,
            "properties": {
              "specialist": {
                "type": "string",
                "description": "Target agent for consultation (e.g., goexpert, researcher, _any_specialist)"
              },
              "query": {
                "type": "string",
                "description": "Specific question or topic to research"
              },
              "purpose": {
                "type": "string",
                "description": "Why this knowledge is needed"
              },
              "priority": {
                "type": "string",
                "enum": ["required", "optional"],
                "description": "Whether to block until complete or best-effort"
              }
            }
          },
          "description": "Requests for specialist consultation or research"
        },
        "context_feedback": {
          "type": "object",
          "additionalProperties": false,
          "properties": {
            "overall_usefulness": {
              "type": "number",
              "minimum": 0,
              "maximum": 1,
              "description": "How useful was the provided context (0.0 = irrelevant, 1.0 = exactly what was needed)"
            },
            "helpful_facts": {
              "type": "array",
              "items": {"type": "string"},
              "description": "Predicate names that were particularly useful (e.g., file_topology, test_state)"
            },
            "noise_facts": {
              "type": "array",
              "items": {"type": "string"},
              "description": "Predicate names that were irrelevant noise (e.g., dom_node, browser_state)"
            },
            "missing_context": {
              "type": "string",
              "description": "Description of what context would have been helpful"
            }
          },
          "description": "Feedback on context usefulness for improving future context selection"
        },
        "tool_requests": {
          "type": "array",
          "items": {
            "type": "object",
            "required": ["id", "tool_name", "tool_args"],
            "additionalProperties": false,
            "properties": {
              "id": {
                "type": "string",
                "description": "Unique identifier for this tool request (e.g., req_1, tool_req_abc123)"
              },
              "tool_name": {
                "type": "string",
                "description": "Name of the tool to invoke (e.g., read_file, write_file, run_command)"
              },
              "tool_args": {
                "type": "object",
                "description": "Arguments for the tool invocation (structure depends on tool schema)"
              },
              "purpose": {
                "type": "string",
                "description": "Why this tool is being invoked (for debugging/learning)"
              },
              "required": {
                "type": "boolean",
                "description": "Whether this tool call blocks execution (true) or is best-effort (false)"
              }
            }
          },
          "description": "Tool execution requests via structured output (replaces native function calling)"
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
