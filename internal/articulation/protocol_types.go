package articulation

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// =============================================================================
// PIGGYBACK PROTOCOL TYPES - The Dual-Payload envelope and control schema
// =============================================================================

// PiggybackEnvelope represents the Dual-Payload JSON Schema (v1.2.0 - Thought-First).
// THOUGHT-FIRST ORDERING (Bug #14 Fix): control_packet MUST appear before surface_response
// in JSON to prevent "Premature Articulation" - where the LLM says it did something before
// actually emitting the action to the kernel. If generation fails mid-stream, the user
// sees nothing (or partial JSON) instead of a false promise.
type PiggybackEnvelope struct {
	Control ControlPacket `json:"control_packet"` // MUST be first in JSON output
	Surface string        `json:"surface_response"`
}

// ControlPacket contains the logic atoms and system state updates.
type ControlPacket struct {
	IntentClassification IntentClassification `json:"intent_classification"`
	MangleUpdates        []string             `json:"mangle_updates"`
	MemoryOperations     []MemoryOperation    `json:"memory_operations"`
	SelfCorrection       *SelfCorrection      `json:"self_correction,omitempty"`
	ReasoningTrace       string               `json:"reasoning_trace,omitempty"`
	// KnowledgeRequests allows the LLM to request specialist consultation or research.
	// When populated, the TUI orchestrator will spawn specialists and re-invoke the LLM
	// with gathered knowledge before generating the final response.
	KnowledgeRequests []KnowledgeRequest `json:"knowledge_requests,omitempty"`
	// ContextFeedback allows the LLM to rate the usefulness of provided context facts.
	// This creates a feedback loop to improve spreading activation scoring over time.
	// The LLM should emit this on every turn to provide continuous learning signal.
	ContextFeedback *ContextFeedback `json:"context_feedback,omitempty"`
	// ToolRequests allows the LLM to request tool execution via structured output.
	// This replaces native LLM function calling, enabling:
	// 1. Coexistence with Gemini's built-in tools (Google Search, URL Context)
	// 2. Dynamic tool discovery (includes Ouroboros-generated tools)
	// 4. Unified tool interface across all providers
	// 5. Full debugging visibility of tool invocations
	ToolRequests []ToolRequest `json:"tool_requests,omitempty"`
}

// UnmarshalJSON implements a custom unmarshaler for ControlPacket to tolerate single string mangle_updates.
func (cp *ControlPacket) UnmarshalJSON(data []byte) error {
	type Alias ControlPacket
	var aux struct {
		MangleUpdates json.RawMessage `json:"mangle_updates"`
		Alias
	}
	aux.Alias = Alias(*cp)
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	*cp = ControlPacket(aux.Alias)
	if len(aux.MangleUpdates) > 0 {
		var slice []string
		if err := json.Unmarshal(aux.MangleUpdates, &slice); err == nil {
			cp.MangleUpdates = slice
		} else {
			var single string
			if err := json.Unmarshal(aux.MangleUpdates, &single); err == nil {
				if single != "" {
					cp.MangleUpdates = []string{single}
				} else {
					cp.MangleUpdates = []string{}
				}
			} else {
				return fmt.Errorf("mangle_updates is neither a string nor a slice of strings: %s", string(aux.MangleUpdates))
			}
		}
	}
	return nil
}

// ToolRequest represents a structured request for tool execution.
// The LLM emits these in the control_packet instead of using native function calling.
// This enables tool use to coexist with Gemini's built-in grounding tools.
type ToolRequest struct {
	// ID is a unique identifier for this tool request (for result correlation).
	// Format: "tool_req_<uuid>" or sequential like "req_1", "req_2"
	ID string `json:"id"`
	// ToolName is the registered name of the tool to invoke.
	// Examples: "read_file", "write_file", "run_command", "web_search"
	ToolName string `json:"tool_name"`
	// ToolArgs contains the arguments for the tool invocation.
	// The structure depends on the tool's schema.
	ToolArgs map[string]any `json:"tool_args"`
	// Purpose explains why this tool is being invoked (for debugging/learning).
	Purpose string `json:"purpose,omitempty"`
	// Required indicates if this tool call is blocking (true) or best-effort (false).
	Required bool `json:"required,omitempty"`
}

// UnmarshalJSON implements a custom unmarshaler for ToolRequest to tolerate stringified booleans.
func (tr *ToolRequest) UnmarshalJSON(data []byte) error {
	type Alias ToolRequest
	var aux struct {
		Required json.RawMessage `json:"required"`
		Alias
	}
	aux.Alias = Alias(*tr)
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	*tr = ToolRequest(aux.Alias)
	if len(aux.Required) > 0 {
		var b bool
		if err := json.Unmarshal(aux.Required, &b); err == nil {
			tr.Required = b
		} else {
			var s string
			if err := json.Unmarshal(aux.Required, &s); err == nil {
				s = strings.Trim(strings.ToLower(s), `"`+"\t\n\r ")
				switch s {
				case "true", "1", "yes", "y", "on":
					tr.Required = true
				case "false", "0", "no", "n", "off":
					tr.Required = false
				default:
					return fmt.Errorf("invalid string value for required: %q", s)
				}
			} else {
				var i int
				if err := json.Unmarshal(aux.Required, &i); err == nil {
					tr.Required = (i != 0)
				} else {
					return fmt.Errorf("required is neither a boolean, string, nor integer: %s", string(aux.Required))
				}
			}
		}
	}
	return nil
}

// KnowledgeRequest represents a request for specialist consultation or research.
// This enables LLM-first knowledge discovery where the agent can proactively
// gather information from domain specialists or web research.
type KnowledgeRequest struct {
	// Specialist is the target agent for consultation.
	// Values: agent name (e.g., "goexpert"), "researcher" for web research,
	// or "_any_specialist" for auto-selection based on query content.
	Specialist string `json:"specialist"`
	// Query is the specific question or topic to research.
	Query string `json:"query"`
	// Purpose explains why this knowledge is needed (helps with context handoff).
	Purpose string `json:"purpose,omitempty"`
	// Priority: "required" (block until complete) or "optional" (best-effort).
	Priority string `json:"priority,omitempty"`
}

// IntentClassification helps the kernel decide which ShardAgent to spawn.
type IntentClassification struct {
	Category   string  `json:"category"`
	Verb       string  `json:"verb"`
	Target     string  `json:"target"`
	Constraint string  `json:"constraint"`
	Confidence float64 `json:"confidence"`
}

// UnmarshalJSON implements a custom unmarshaler for IntentClassification to tolerate stringified floats/confidence levels.
func (ic *IntentClassification) UnmarshalJSON(data []byte) error {
	type Alias IntentClassification
	var aux struct {
		Confidence json.RawMessage `json:"confidence"`
		Alias
	}
	aux.Alias = Alias(*ic)
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	*ic = IntentClassification(aux.Alias)
	if len(aux.Confidence) > 0 {
		var f float64
		if err := json.Unmarshal(aux.Confidence, &f); err == nil {
			ic.Confidence = f
		} else {
			var s string
			if err := json.Unmarshal(aux.Confidence, &s); err == nil {
				s = strings.Trim(strings.ToLower(s), `"`+"\t\n\r ")
				switch s {
				case "high":
					ic.Confidence = 0.9
				case "medium", "med":
					ic.Confidence = 0.5
				case "low":
					ic.Confidence = 0.1
				default:
					if parsed, err := strconv.ParseFloat(s, 64); err == nil {
						ic.Confidence = parsed
					} else {
						return fmt.Errorf("invalid string value for confidence: %q", s)
					}
				}
			} else {
				return fmt.Errorf("confidence is neither a float nor a string: %s", string(aux.Confidence))
			}
		}
	}
	return nil
}

// MemoryOperation represents a directive to the Cold Storage.
type MemoryOperation struct {
	Op    string `json:"op"`    // promote_to_long_term, forget, store_vector, note
	Key   string `json:"key"`   // Fact predicate or preference key
	Value string `json:"value"` // Fact value or preference value
}

// SelfCorrection represents an internal hypothesis about errors.
type SelfCorrection struct {
	Triggered  bool   `json:"triggered"`
	Hypothesis string `json:"hypothesis"`
}

// ContextFeedback represents LLM feedback on context usefulness.
// This enables self-improving context selection via spreading activation.
// The LLM rates which predicate types were helpful vs noise, creating
// training signal to improve future context retrieval.
type ContextFeedback struct {
	// OverallUsefulness: 0.0-1.0 rating of how useful the provided context was.
	// 1.0 = context was exactly what was needed
	// 0.5 = context was partially helpful
	// 0.0 = context was irrelevant or confusing
	OverallUsefulness float64 `json:"overall_usefulness"`
	// HelpfulFacts: Predicate names that were particularly useful.
	// Examples: ["file_topology", "test_state", "symbol_graph"]
	HelpfulFacts []string `json:"helpful_facts,omitempty"`
	// NoiseFacts: Predicate names that were irrelevant noise.
	// Examples: ["dom_node", "browser_state"]
	NoiseFacts []string `json:"noise_facts,omitempty"`
	// MissingContext: Description of what context would have been helpful.
	// Example: "needed dependency graph for this module"
	MissingContext string `json:"missing_context,omitempty"`
}
