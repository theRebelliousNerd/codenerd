package articulation

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// =============================================================================
// PIGGYBACK PROTOCOL - Dual-Channel Steganographic Control
// =============================================================================
// The Articulation layer handles LLM output → structured data extraction.
// This implements the "Corpus Callosum" between surface text and logic atoms.

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
	ReasoningTrace       string               `json:"reasoning_trace,omitempty"`
}

// IntentClassification helps the kernel decide which ShardAgent to spawn.
type IntentClassification struct {
	Category   string  `json:"category"`
	Verb       string  `json:"verb"`
	Target     string  `json:"target"`
	Constraint string  `json:"constraint"`
	Confidence float64 `json:"confidence"`
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

// =============================================================================
// RESPONSE PROCESSOR - The Full Articulation Pipeline
// =============================================================================

// ResponseProcessor handles the complete articulation pipeline:
// LLM Raw Output → Parse → Validate → Extract → Return structured result
type ResponseProcessor struct {
	// Validation settings
	RequireValidJSON    bool
	AllowMarkdownWrapped bool
	MaxSurfaceLength    int

	// Statistics for monitoring
	stats ProcessorStats
}

// ProcessorStats tracks parsing statistics for monitoring.
type ProcessorStats struct {
	TotalProcessed     int
	SuccessfulParses   int
	FallbackParses     int
	ValidationFailures int
	SelfCorrections    int
}

// ArticulationResult is the complete output of the articulation layer.
type ArticulationResult struct {
	// Surface response for the user
	Surface string

	// Structured control data
	Control ControlPacket

	// Parsing metadata
	ParseMethod string  // "json", "fallback", "repair"
	Confidence  float64
	Warnings    []string

	// Original raw response (for debugging)
	RawResponse string
}

// NewResponseProcessor creates a new processor with default settings.
func NewResponseProcessor() *ResponseProcessor {
	return &ResponseProcessor{
		RequireValidJSON:    false, // Allow fallback parsing
		AllowMarkdownWrapped: true,
		MaxSurfaceLength:    50000,
	}
}

// Process parses an LLM response into a structured ArticulationResult.
// This is the main entry point for the articulation layer.
func (rp *ResponseProcessor) Process(rawResponse string) (*ArticulationResult, error) {
	rp.stats.TotalProcessed++

	result := &ArticulationResult{
		RawResponse: rawResponse,
		ParseMethod: "unknown",
		Confidence:  0.0,
		Warnings:    []string{},
	}

	// 1. Try direct JSON parsing
	envelope, err := rp.parseJSON(rawResponse)
	if err == nil {
		result.Surface = envelope.Surface
		result.Control = envelope.Control
		result.ParseMethod = "json"
		result.Confidence = 1.0
		rp.stats.SuccessfulParses++

		// Check for self-correction trigger
		if envelope.Control.SelfCorrection != nil && envelope.Control.SelfCorrection.Triggered {
			rp.stats.SelfCorrections++
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("Self-correction triggered: %s", envelope.Control.SelfCorrection.Hypothesis))
		}

		return result, nil
	}

	// 2. Try markdown-wrapped JSON
	if rp.AllowMarkdownWrapped {
		envelope, err = rp.parseMarkdownWrappedJSON(rawResponse)
		if err == nil {
			result.Surface = envelope.Surface
			result.Control = envelope.Control
			result.ParseMethod = "json_markdown"
			result.Confidence = 0.95
			rp.stats.SuccessfulParses++
			return result, nil
		}
	}

	// 3. Try to extract JSON from mixed content
	envelope, err = rp.extractEmbeddedJSON(rawResponse)
	if err == nil {
		result.Surface = envelope.Surface
		result.Control = envelope.Control
		result.ParseMethod = "json_extracted"
		result.Confidence = 0.85
		rp.stats.SuccessfulParses++
		result.Warnings = append(result.Warnings, "JSON extracted from mixed content")
		return result, nil
	}

	// 4. Fallback: treat entire response as surface text
	if !rp.RequireValidJSON {
		result.Surface = strings.TrimSpace(rawResponse)
		result.Control = ControlPacket{} // Empty control packet
		result.ParseMethod = "fallback"
		result.Confidence = 0.5
		rp.stats.FallbackParses++
		result.Warnings = append(result.Warnings, "No valid JSON found, using raw response as surface")
		return result, nil
	}

	// 5. Strict mode: fail if no valid JSON
	rp.stats.ValidationFailures++
	return nil, fmt.Errorf("failed to parse Piggyback JSON: %w", err)
}

// parseJSON attempts direct JSON parsing.
func (rp *ResponseProcessor) parseJSON(s string) (PiggybackEnvelope, error) {
	s = strings.TrimSpace(s)

	var envelope PiggybackEnvelope
	if err := json.Unmarshal([]byte(s), &envelope); err != nil {
		return PiggybackEnvelope{}, err
	}

	// Validate required fields
	if envelope.Surface == "" {
		return PiggybackEnvelope{}, fmt.Errorf("missing surface_response field")
	}

	return envelope, nil
}

// parseMarkdownWrappedJSON handles ```json ... ``` wrapping.
func (rp *ResponseProcessor) parseMarkdownWrappedJSON(s string) (PiggybackEnvelope, error) {
	s = strings.TrimSpace(s)

	// Remove markdown code block markers
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```JSON")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)

	return rp.parseJSON(s)
}

// extractEmbeddedJSON finds JSON within mixed content.
func (rp *ResponseProcessor) extractEmbeddedJSON(s string) (PiggybackEnvelope, error) {
	// Pattern to find JSON objects
	jsonPattern := regexp.MustCompile(`\{[\s\S]*"surface_response"[\s\S]*"control_packet"[\s\S]*\}`)

	match := jsonPattern.FindString(s)
	if match == "" {
		// Try alternative pattern
		jsonPattern = regexp.MustCompile(`\{[\s\S]*\}`)
		matches := jsonPattern.FindAllString(s, -1)

		// Try each match, largest first
		for i := len(matches) - 1; i >= 0; i-- {
			envelope, err := rp.parseJSON(matches[i])
			if err == nil {
				return envelope, nil
			}
		}

		return PiggybackEnvelope{}, fmt.Errorf("no embedded JSON found")
	}

	return rp.parseJSON(match)
}

// GetStats returns current processing statistics.
func (rp *ResponseProcessor) GetStats() ProcessorStats {
	return rp.stats
}

// ResetStats resets the processing statistics.
func (rp *ResponseProcessor) ResetStats() {
	rp.stats = ProcessorStats{}
}

// =============================================================================
// EMITTER - Output Generation
// =============================================================================

// Emitter handles output generation and formatting.
type Emitter struct {
	processor *ResponseProcessor

	// Output settings
	PrettyPrint bool
	IncludeRaw  bool
}

// NewEmitter creates a new Emitter with default settings.
func NewEmitter() *Emitter {
	return &Emitter{
		processor:   NewResponseProcessor(),
		PrettyPrint: true,
		IncludeRaw:  false,
	}
}

// Emit outputs the dual payload as JSON.
func (e *Emitter) Emit(payload PiggybackEnvelope) error {
	var data []byte
	var err error

	if e.PrettyPrint {
		data, err = json.MarshalIndent(payload, "", "  ")
	} else {
		data, err = json.Marshal(payload)
	}

	if err != nil {
		return err
	}

	fmt.Println(string(data))
	return nil
}

// EmitSurface outputs only the surface response.
func (e *Emitter) EmitSurface(payload PiggybackEnvelope) {
	fmt.Println(strings.TrimSpace(payload.Surface))
}

// ParseAndProcess parses raw LLM output and returns structured result.
func (e *Emitter) ParseAndProcess(rawResponse string) (*ArticulationResult, error) {
	return e.processor.Process(rawResponse)
}

// CreateEnvelope creates a PiggybackEnvelope from components.
func (e *Emitter) CreateEnvelope(surface string, intent IntentClassification, mangleUpdates []string, memOps []MemoryOperation) PiggybackEnvelope {
	return PiggybackEnvelope{
		Surface: surface,
		Control: ControlPacket{
			IntentClassification: intent,
			MangleUpdates:        mangleUpdates,
			MemoryOperations:     memOps,
		},
	}
}

// MarshalEnvelope converts an envelope to JSON bytes.
func (e *Emitter) MarshalEnvelope(envelope PiggybackEnvelope) ([]byte, error) {
	if e.PrettyPrint {
		return json.MarshalIndent(envelope, "", "  ")
	}
	return json.Marshal(envelope)
}

// =============================================================================
// CONSTITUTIONAL OVERRIDE - Safety Layer
// =============================================================================

// ConstitutionalOverride represents a kernel-mandated response modification.
type ConstitutionalOverride struct {
	OriginalSurface string
	ModifiedSurface string
	Reason          string
	BlockedAtoms    []string
}

// ApplyConstitutionalOverride modifies the surface response based on kernel rules.
// This allows the kernel to block or rewrite unsafe surface responses.
func ApplyConstitutionalOverride(envelope *PiggybackEnvelope, blocked []string, reason string) *ConstitutionalOverride {
	if len(blocked) == 0 && reason == "" {
		return nil // No override needed
	}

	override := &ConstitutionalOverride{
		OriginalSurface: envelope.Surface,
		BlockedAtoms:    blocked,
		Reason:          reason,
	}

	// Modify surface if there's a safety concern
	if reason != "" {
		override.ModifiedSurface = fmt.Sprintf("[SAFETY NOTICE: %s]\n\n%s", reason, envelope.Surface)
		envelope.Surface = override.ModifiedSurface
	}

	// Filter blocked atoms from mangle_updates
	if len(blocked) > 0 {
		filtered := make([]string, 0, len(envelope.Control.MangleUpdates))
		blockedSet := make(map[string]bool)
		for _, b := range blocked {
			blockedSet[b] = true
		}

		for _, atom := range envelope.Control.MangleUpdates {
			if !blockedSet[atom] {
				filtered = append(filtered, atom)
			}
		}
		envelope.Control.MangleUpdates = filtered
	}

	return override
}

// =============================================================================
// UTILITY FUNCTIONS
// =============================================================================

// ExtractSurfaceOnly extracts just the surface response, ignoring control packet.
// Useful for display purposes when only the user-facing text is needed.
func ExtractSurfaceOnly(rawResponse string) string {
	processor := NewResponseProcessor()
	processor.RequireValidJSON = false

	result, err := processor.Process(rawResponse)
	if err != nil {
		return strings.TrimSpace(rawResponse)
	}

	return result.Surface
}

// HasSelfCorrection checks if the response indicates self-correction was triggered.
func HasSelfCorrection(envelope PiggybackEnvelope) bool {
	return envelope.Control.SelfCorrection != nil && envelope.Control.SelfCorrection.Triggered
}

// HasMemoryOperations checks if there are memory operations to process.
func HasMemoryOperations(envelope PiggybackEnvelope) bool {
	return len(envelope.Control.MemoryOperations) > 0
}

// GetMemoryOperationsByType filters memory operations by operation type.
func GetMemoryOperationsByType(envelope PiggybackEnvelope, opType string) []MemoryOperation {
	result := make([]MemoryOperation, 0)
	for _, op := range envelope.Control.MemoryOperations {
		if op.Op == opType {
			result = append(result, op)
		}
	}
	return result
}
