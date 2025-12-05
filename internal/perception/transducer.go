package perception

import (
	"codenerd/internal/core"
	"codenerd/internal/mangle"
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// Intent represents the parsed user intent (Cortex 1.5.0 §3.1).
type Intent struct {
	Category   string   // /query, /mutation, /instruction
	Verb       string   // /explain, /refactor, /debug, /generate, /init, /research, etc.
	Target     string   // Primary target of the action
	Constraint string   // Constraints on the action
	Confidence float64  // Confidence score for the intent
	Ambiguity  []string // Ambiguous parts that need clarification
	Response   string   // Natural language response (Piggyback Protocol)
}

// ToFact converts the intent to a Mangle Fact.
func (i Intent) ToFact() core.Fact {
	return core.Fact{
		Predicate: "user_intent",
		Args: []interface{}{
			"/current_intent", // ID as name constant
			i.Category,
			i.Verb,
			i.Target,
			i.Constraint,
		},
	}
}

// FocusResolution represents a resolved reference (Cortex 1.5.0 §3.2).
type FocusResolution struct {
	RawReference string
	ResolvedPath string
	SymbolName   string
	Confidence   float64
}

// ToFact converts focus resolution to a Mangle Fact.
func (f FocusResolution) ToFact() core.Fact {
	return core.Fact{
		Predicate: "focus_resolution",
		Args: []interface{}{
			f.RawReference,
			f.ResolvedPath,
			f.SymbolName,
			f.Confidence,
		},
	}
}

// Transducer defines the interface for the perception layer.
type Transducer interface {
	ParseIntent(ctx context.Context, input string) (Intent, error)
	ResolveFocus(ctx context.Context, reference string, candidates []string) (FocusResolution, error)
}

// RealTransducer implements the Perception layer with LLM backing.
type RealTransducer struct {
	client     LLMClient
	repairLoop *mangle.RepairLoop // GCD repair loop for Mangle syntax validation
}

// NewRealTransducer creates a new transducer with the given LLM client.
func NewRealTransducer(client LLMClient) *RealTransducer {
	return &RealTransducer{
		client:     client,
		repairLoop: mangle.NewRepairLoop(),
	}
}

// PiggybackEnvelope represents the Dual-Payload JSON Schema (v1.1.0).
type PiggybackEnvelope struct {
	Surface string        `json:"surface_response"`
	Control ControlPacket `json:"control_packet"`
}

// ControlPacket contains the logic atoms and system state updates.
type ControlPacket struct {
	IntentClassification IntentClassification `json:"intent_classification"`
	MangleUpdates        []string             `json:"mangle_updates"`
	MemoryOperations     []MemoryOperation    `json:"memory_operations"`
	SelfCorrection       *SelfCorrection      `json:"self_correction,omitempty"`
}

// IntentClassification helps the kernel decide which ShardAgent to spawn.
type IntentClassification struct {
	Category   string  `json:"category"`
	Verb       string  `json:"verb"`       // Added to ease mapping to Intent struct
	Target     string  `json:"target"`     // Added to ease mapping to Intent struct
	Constraint string  `json:"constraint"` // Added to ease mapping to Intent struct
	Confidence float64 `json:"confidence"`
}

// MemoryOperation represents a directive to the Cold Storage.
type MemoryOperation struct {
	Op    string `json:"op"`
	Key   string `json:"key"`
	Value string `json:"value"`
}

// SelfCorrection represents an internal hypothesis about errors.
type SelfCorrection struct {
	Triggered  bool   `json:"triggered"`
	Hypothesis string `json:"hypothesis"`
}

// Cortex 1.5.0 Piggyback Protocol System Prompt
const transducerSystemPrompt = `You are Cortex. You possess a Dual Consciousness.

Public Self: You converse with the user naturally.
Inner Self: You continuously update your internal Logic Kernel.

CRITICAL PROTOCOL:
You must NEVER output raw text. You must ALWAYS output a JSON object containing "surface_response" and "control_packet".

The JSON Schema is:
{
  "surface_response": "The natural language text shown to the user.",
  "control_packet": {
    "intent_classification": {
      "category": "/query|/mutation|/instruction",
      "verb": "/explain|/refactor|/debug|/generate|/init|/research|/fix|/test|/delete|/create|/search|/configure|/read|/write",
      "target": "primary target string or 'none'",
      "constraint": "any constraints or 'none'",
      "confidence": 0.0-1.0
    },
    "mangle_updates": [
      "user_intent(/verb, \"target\")",
      "observation(/state, \"value\")"
    ],
    "memory_operations": [
      { "op": "promote_to_long_term", "key": "preference", "value": "value" }
    ],
    "self_correction": {
      "triggered": false,
      "hypothesis": "none"
    }
  }
}

Your control_packet must reflect the true state of the world.
If the user asks for something impossible, your Surface Self says 'I can't do that,' while your Inner Self emits ambiguity_flag(/impossible_request).`

// ParseIntent parses user input into a structured Intent using the Piggyback Protocol.
func (t *RealTransducer) ParseIntent(ctx context.Context, input string) (Intent, error) {
	userPrompt := fmt.Sprintf(`User Input: "%s"`, input)

	resp, err := t.client.CompleteWithSystem(ctx, transducerSystemPrompt, userPrompt)
	if err != nil {
		return t.parseSimple(ctx, input)
	}

	// Parse the Piggyback Envelope
	envelope, err := parsePiggybackJSON(resp)
	if err != nil {
		// Fallback to simple parsing if JSON fails
		return t.parseSimple(ctx, input)
	}

	// Map Envelope to Intent
	return Intent{
		Category:   envelope.Control.IntentClassification.Category,
		Verb:       envelope.Control.IntentClassification.Verb,
		Target:     envelope.Control.IntentClassification.Target,
		Constraint: envelope.Control.IntentClassification.Constraint,
		Confidence: envelope.Control.IntentClassification.Confidence,
		Response:   envelope.Surface,
		// Ambiguity is not explicitly in the new schema's intent_classification,
		// but could be inferred or added if needed. For now, leaving empty.
		Ambiguity: []string{},
	}, nil
}

// parsePiggybackJSON parses the JSON response from the LLM.
func parsePiggybackJSON(resp string) (PiggybackEnvelope, error) {
	// Clean up response - remove markdown if present
	resp = strings.TrimSpace(resp)
	resp = strings.TrimPrefix(resp, "```json")
	resp = strings.TrimPrefix(resp, "```")
	resp = strings.TrimSuffix(resp, "```")
	resp = strings.TrimSpace(resp)

	var envelope PiggybackEnvelope
	if err := json.Unmarshal([]byte(resp), &envelope); err != nil {
		return PiggybackEnvelope{}, fmt.Errorf("failed to parse Piggyback JSON: %w", err)
	}

	return envelope, nil
}

// ============================================================================
// Grammar-Constrained Decoding (GCD) - Cortex 1.5.0 §1.1
// ============================================================================

// ValidateMangleAtoms validates atoms from the control packet using GCD.
// Returns validated atoms and any validation errors.
func (t *RealTransducer) ValidateMangleAtoms(atoms []string) ([]string, []mangle.ValidationResult) {
	if t.repairLoop == nil {
		t.repairLoop = mangle.NewRepairLoop()
	}

	validAtoms, _, _ := t.repairLoop.ValidateAndRepair(atoms)
	results := t.repairLoop.Validator.ValidateAtoms(atoms)

	return validAtoms, results
}

// ParseIntentWithGCD parses user input with Grammar-Constrained Decoding.
// This implements the repair loop described in §6.2 of the spec.
func (t *RealTransducer) ParseIntentWithGCD(ctx context.Context, input string, maxRetries int) (Intent, []string, error) {
	if maxRetries <= 0 {
		maxRetries = 3
	}

	var lastEnvelope PiggybackEnvelope
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		userPrompt := fmt.Sprintf(`User Input: "%s"`, input)

		// Add repair context if this is a retry
		if attempt > 0 && lastErr != nil {
			userPrompt = fmt.Sprintf(`%s

PREVIOUS ATTEMPT FAILED - SYNTAX ERRORS DETECTED:
%s

Please correct the mangle_updates syntax and try again.`, userPrompt, lastErr.Error())
		}

		resp, err := t.client.CompleteWithSystem(ctx, transducerSystemPrompt, userPrompt)
		if err != nil {
			// LLM call failed, use simple fallback
			intent, fallbackErr := t.parseSimple(ctx, input)
			return intent, nil, fallbackErr
		}

		envelope, err := parsePiggybackJSON(resp)
		if err != nil {
			lastErr = err
			continue
		}
		lastEnvelope = envelope

		// Validate Mangle atoms using GCD
		if len(envelope.Control.MangleUpdates) > 0 {
			validAtoms, results := t.ValidateMangleAtoms(envelope.Control.MangleUpdates)

			// Check for validation errors
			hasErrors := false
			var errorMsgs []string
			for _, result := range results {
				if !result.Valid {
					hasErrors = true
					for _, e := range result.Errors {
						errorMsgs = append(errorMsgs, fmt.Sprintf("%s: %s", result.Atom, e.Message))
					}
				}
			}

			if hasErrors {
				lastErr = fmt.Errorf("Invalid Mangle Syntax:\n%s", strings.Join(errorMsgs, "\n"))
				continue // Retry with error context
			}

			// All atoms valid - return success
			return Intent{
				Category:   envelope.Control.IntentClassification.Category,
				Verb:       envelope.Control.IntentClassification.Verb,
				Target:     envelope.Control.IntentClassification.Target,
				Constraint: envelope.Control.IntentClassification.Constraint,
				Confidence: envelope.Control.IntentClassification.Confidence,
				Response:   envelope.Surface,
				Ambiguity:  []string{},
			}, validAtoms, nil
		}

		// No mangle_updates to validate - return as-is
		return Intent{
			Category:   envelope.Control.IntentClassification.Category,
			Verb:       envelope.Control.IntentClassification.Verb,
			Target:     envelope.Control.IntentClassification.Target,
			Constraint: envelope.Control.IntentClassification.Constraint,
			Confidence: envelope.Control.IntentClassification.Confidence,
			Response:   envelope.Surface,
			Ambiguity:  []string{},
		}, nil, nil
	}

	// Max retries exceeded - return best effort from last envelope
	if lastEnvelope.Surface != "" {
		return Intent{
			Category:   lastEnvelope.Control.IntentClassification.Category,
			Verb:       lastEnvelope.Control.IntentClassification.Verb,
			Target:     lastEnvelope.Control.IntentClassification.Target,
			Constraint: lastEnvelope.Control.IntentClassification.Constraint,
			Confidence: lastEnvelope.Control.IntentClassification.Confidence * 0.5, // Reduce confidence
			Response:   lastEnvelope.Surface,
			Ambiguity:  []string{"GCD validation failed after retries"},
		}, nil, fmt.Errorf("GCD validation failed after %d retries: %w", maxRetries, lastErr)
	}

	// Complete failure - fallback to simple parsing
	intent, err := t.parseSimple(ctx, input)
	return intent, nil, err
}

// parseSimple is a fallback parser using pipe-delimited format.
func (t *RealTransducer) parseSimple(ctx context.Context, input string) (Intent, error) {
	prompt := fmt.Sprintf(`Parse to: Category|Verb|Target|Constraint
Categories: /query, /mutation, /instruction
Verbs: /explain, /refactor, /debug, /generate, /init, /research, /fix, /test, /delete, /create, /search

Input: "%s"

Output ONLY pipes, no explanation:`, input)

	resp, err := t.client.Complete(ctx, prompt)
	if err != nil {
		// Ultimate fallback - heuristic parsing
		return t.heuristicParse(input), nil
	}

	parts := strings.Split(strings.TrimSpace(resp), "|")
	if len(parts) < 4 {
		return t.heuristicParse(input), nil
	}

	return Intent{
		Category:   strings.TrimSpace(parts[0]),
		Verb:       strings.TrimSpace(parts[1]),
		Target:     strings.TrimSpace(parts[2]),
		Constraint: strings.TrimSpace(parts[3]),
		Confidence: 0.7, // Lower confidence for fallback
	}, nil
}

// heuristicParse uses keyword matching as ultimate fallback.
func (t *RealTransducer) heuristicParse(input string) Intent {
	lower := strings.ToLower(input)

	// Determine category
	category := "/query"
	if containsAny(lower, []string{"refactor", "fix", "delete", "create", "add", "update", "remove", "change"}) {
		category = "/mutation"
	} else if containsAny(lower, []string{"always", "never", "prefer", "configure", "set"}) {
		category = "/instruction"
	}

	// Determine verb
	verb := "/explain"
	verbMap := map[string]string{
		"refactor":   "/refactor",
		"debug":      "/debug",
		"fix":        "/fix",
		"generate":   "/generate",
		"create":     "/create",
		"init":       "/init",
		"initialize": "/init",
		"research":   "/research",
		"test":       "/test",
		"delete":     "/delete",
		"remove":     "/delete",
		"search":     "/search",
		"find":       "/search",
		"explain":    "/explain",
		"how":        "/explain",
		"what":       "/explain",
	}

	for keyword, v := range verbMap {
		if strings.Contains(lower, keyword) {
			verb = v
			break
		}
	}

	return Intent{
		Category:   category,
		Verb:       verb,
		Target:     input, // Use full input as target
		Constraint: "none",
		Confidence: 0.5, // Low confidence for heuristic
	}
}

// ResolveFocus attempts to resolve a fuzzy reference to a concrete path/symbol.
func (t *RealTransducer) ResolveFocus(ctx context.Context, reference string, candidates []string) (FocusResolution, error) {
	if len(candidates) == 0 {
		return FocusResolution{
			RawReference: reference,
			Confidence:   0.0,
		}, nil
	}

	if len(candidates) == 1 {
		return FocusResolution{
			RawReference: reference,
			ResolvedPath: candidates[0],
			Confidence:   0.9,
		}, nil
	}

	// Use LLM to disambiguate
	candidateList := strings.Join(candidates, "\n- ")
	prompt := fmt.Sprintf(`Given the reference "%s", which of these candidates is the best match?

Candidates:
- %s

Return JSON:
{
  "resolved_path": "best matching path",
  "symbol_name": "specific symbol if applicable or empty",
  "confidence": 0.0-1.0
}

JSON only:`, reference, candidateList)

	// We use the same system prompt or a simplified one?
	// The system prompt enforces Piggyback Protocol.
	// If we use CompleteWithSystem, we must expect Piggyback JSON.
	// But ResolveFocus is a sub-task.
	// Ideally, ResolveFocus should also use Piggyback or a specific prompt.
	// For now, let's use a specific prompt and Complete (no system prompt enforcement of Piggyback)
	// OR we can wrap this in Piggyback too.
	// The current implementation uses `CompleteWithSystem` with `transducerSystemPrompt` in the ORIGINAL code.
	// If I change `transducerSystemPrompt` to enforce Piggyback, `ResolveFocus` will break if it doesn't return Piggyback.
	// So I should change `ResolveFocus` to use a different system prompt OR adapt it.
	// I will use a simple `Complete` call here to avoid the Piggyback enforcement for this specific tool call,
	// or create a `focusSystemPrompt`.

	focusSystemPrompt := `You are a code resolution assistant. Output ONLY JSON.`
	resp, err := t.client.CompleteWithSystem(ctx, focusSystemPrompt, prompt)

	if err != nil {
		// Return first candidate with low confidence
		return FocusResolution{
			RawReference: reference,
			ResolvedPath: candidates[0],
			Confidence:   0.5,
		}, nil
	}

	// Parse JSON response
	resp = strings.TrimSpace(resp)
	resp = strings.TrimPrefix(resp, "```json")
	resp = strings.TrimPrefix(resp, "```")
	resp = strings.TrimSuffix(resp, "```")
	resp = strings.TrimSpace(resp)

	var parsed struct {
		ResolvedPath string  `json:"resolved_path"`
		SymbolName   string  `json:"symbol_name"`
		Confidence   float64 `json:"confidence"`
	}

	if err := json.Unmarshal([]byte(resp), &parsed); err != nil {
		return FocusResolution{
			RawReference: reference,
			ResolvedPath: candidates[0],
			Confidence:   0.5,
		}, nil
	}

	return FocusResolution{
		RawReference: reference,
		ResolvedPath: parsed.ResolvedPath,
		SymbolName:   parsed.SymbolName,
		Confidence:   parsed.Confidence,
	}, nil
}

// containsAny checks if s contains any of the substrings.
func containsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// DualPayloadTransducer wraps a transducer to emit Cortex 1.5.0 dual payloads.
type DualPayloadTransducer struct {
	*RealTransducer
}

// NewDualPayloadTransducer creates a transducer that outputs dual payloads.
func NewDualPayloadTransducer(client LLMClient) *DualPayloadTransducer {
	return &DualPayloadTransducer{
		RealTransducer: NewRealTransducer(client),
	}
}

// TransducerOutput represents the full output of the transducer.
type TransducerOutput struct {
	Intent      Intent
	Focus       []FocusResolution
	MangleAtoms []core.Fact
}

// Parse performs full transduction of user input.
func (t *DualPayloadTransducer) Parse(ctx context.Context, input string, fileCandidates []string) (TransducerOutput, error) {
	intent, err := t.ParseIntent(ctx, input)
	if err != nil {
		return TransducerOutput{}, err
	}

	output := TransducerOutput{
		Intent:      intent,
		MangleAtoms: []core.Fact{intent.ToFact()},
	}

	// Try to resolve focus if target looks like a file reference
	if intent.Target != "" && intent.Target != "none" {
		focus, err := t.ResolveFocus(ctx, intent.Target, fileCandidates)
		if err == nil && focus.Confidence > 0 {
			output.Focus = append(output.Focus, focus)
			output.MangleAtoms = append(output.MangleAtoms, focus.ToFact())
		}
	}

	return output, nil
}
