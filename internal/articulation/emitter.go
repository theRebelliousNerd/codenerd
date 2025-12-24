package articulation

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"codenerd/internal/logging"
)

// =============================================================================
// PIGGYBACK PROTOCOL - Dual-Channel Steganographic Control
// =============================================================================
// The Articulation layer handles LLM output → structured data extraction.
// This implements the "Corpus Callosum" between surface text and logic atoms.

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
	RequireValidJSON     bool
	AllowMarkdownWrapped bool
	MaxSurfaceLength     int
	LogFallbackAsError   bool

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
	ParseMethod string // "json", "fallback", "repair"
	Confidence  float64
	Warnings    []string

	// Original raw response (for debugging)
	RawResponse string
}

// NewResponseProcessor creates a new processor with default settings.
func NewResponseProcessor() *ResponseProcessor {
	logging.ArticulationDebug("Creating new ResponseProcessor with default settings")
	rp := &ResponseProcessor{
		RequireValidJSON:     false, // Allow fallback parsing
		AllowMarkdownWrapped: true,
		MaxSurfaceLength:     50000,
		LogFallbackAsError:   true,
	}
	logging.ArticulationDebug("ResponseProcessor initialized: RequireValidJSON=%v, AllowMarkdownWrapped=%v, MaxSurfaceLength=%d",
		rp.RequireValidJSON, rp.AllowMarkdownWrapped, rp.MaxSurfaceLength)
	return rp
}

// Process parses an LLM response into a structured ArticulationResult.
// This is the main entry point for the articulation layer.
func (rp *ResponseProcessor) Process(rawResponse string) (*ArticulationResult, error) {
	timer := logging.StartTimer(logging.CategoryArticulation, "Process")
	defer timer.Stop()

	rp.stats.TotalProcessed++
	logging.Articulation("Processing LLM response (attempt #%d, length=%d bytes)",
		rp.stats.TotalProcessed, len(rawResponse))
	logging.ArticulationDebug("Raw response preview: %.200s...", rawResponse)

	result := &ArticulationResult{
		RawResponse: rawResponse,
		ParseMethod: "unknown",
		Confidence:  0.0,
		Warnings:    []string{},
	}

	// Track parse errors for diagnostic logging on fallback
	var parseErrors []string

	// 1. Try direct JSON parsing
	logging.ArticulationDebug("Attempting direct JSON parsing")
	envelope, err := rp.parseJSON(rawResponse)
	if err == nil {
		result.Surface = envelope.Surface
		result.Control = envelope.Control
		result.ParseMethod = "json"
		result.Confidence = 1.0
		rp.stats.SuccessfulParses++

		logging.Articulation("Direct JSON parse successful (confidence=1.0, surface_length=%d)",
			len(result.Surface))
		logging.ArticulationDebug("Control packet: intent=%s/%s, mangle_updates=%d, memory_ops=%d",
			envelope.Control.IntentClassification.Category,
			envelope.Control.IntentClassification.Verb,
			len(envelope.Control.MangleUpdates),
			len(envelope.Control.MemoryOperations))

		// Check for self-correction trigger
		if envelope.Control.SelfCorrection != nil && envelope.Control.SelfCorrection.Triggered {
			rp.stats.SelfCorrections++
			logging.Articulation("Self-correction triggered: %s", envelope.Control.SelfCorrection.Hypothesis)
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("Self-correction triggered: %s", envelope.Control.SelfCorrection.Hypothesis))
		}

		rp.applyCaps(result)
		return result, nil
	}
	parseErrors = append(parseErrors, fmt.Sprintf("direct: %v", err))
	logging.ArticulationDebug("Direct JSON parse failed: %v", err)

	// 2. Try markdown-wrapped JSON
	if rp.AllowMarkdownWrapped {
		logging.ArticulationDebug("Attempting markdown-wrapped JSON parsing")
		envelope, err = rp.parseMarkdownWrappedJSON(rawResponse)
		if err == nil {
			result.Surface = envelope.Surface
			result.Control = envelope.Control
			result.ParseMethod = "json_markdown"
			result.Confidence = 0.95
			rp.stats.SuccessfulParses++
			logging.Articulation("Markdown-wrapped JSON parse successful (confidence=0.95, surface_length=%d)",
				len(result.Surface))
			rp.applyCaps(result)
			return result, nil
		}
		parseErrors = append(parseErrors, fmt.Sprintf("markdown: %v", err))
		logging.ArticulationDebug("Markdown-wrapped JSON parse failed: %v", err)
	}

	// 3. Try to extract JSON from mixed content
	logging.ArticulationDebug("Attempting embedded JSON extraction")
	envelope, err = rp.extractEmbeddedJSON(rawResponse)
	if err == nil {
		result.Surface = envelope.Surface
		result.Control = envelope.Control
		result.ParseMethod = "json_extracted"
		result.Confidence = 0.85
		rp.stats.SuccessfulParses++
		result.Warnings = append(result.Warnings, "JSON extracted from mixed content")
		logging.Articulation("Embedded JSON extraction successful (confidence=0.85, surface_length=%d)",
			len(result.Surface))
		logging.Get(logging.CategoryArticulation).Warn("JSON extracted from mixed content - LLM response was not clean")
		rp.applyCaps(result)
		return result, nil
	}
	parseErrors = append(parseErrors, fmt.Sprintf("embedded: %v", err))
	logging.ArticulationDebug("Embedded JSON extraction failed: %v", err)

	// 4. Fallback: treat entire response as surface text
	if !rp.RequireValidJSON {
		result.Surface = strings.TrimSpace(rawResponse)
		result.Control = ControlPacket{} // Empty control packet
		result.ParseMethod = "fallback"
		result.Confidence = 0.5
		rp.stats.FallbackParses++
		result.Warnings = append(result.Warnings, "No valid JSON found, using raw response as surface")

		// Log comprehensive diagnostic info for debugging Piggyback failures.
		responsePreview := rawResponse
		if len(responsePreview) > 300 {
			responsePreview = responsePreview[:300] + "..."
		}
		if rp.LogFallbackAsError {
			logging.Get(logging.CategoryArticulation).Error(
				"Fallback parse: no valid Piggyback JSON found (len=%d, errors=[%s], preview=%q)",
				len(rawResponse),
				strings.Join(parseErrors, "; "),
				responsePreview,
			)
		} else if strings.TrimSpace(rawResponse) == "" {
			logging.Get(logging.CategoryArticulation).Warn(
				"Fallback parse: empty response with no Piggyback JSON (errors=[%s])",
				strings.Join(parseErrors, "; "),
			)
		} else {
			logging.ArticulationDebug(
				"Fallback parse: using raw response (len=%d, errors=[%s])",
				len(rawResponse),
				strings.Join(parseErrors, "; "),
			)
		}
		logging.ArticulationDebug("Fallback surface length: %d bytes", len(result.Surface))
		rp.applyCaps(result)
		return result, nil
	}

	// 5. Strict mode: fail if no valid JSON
	rp.stats.ValidationFailures++
	logging.Get(logging.CategoryArticulation).Error("Strict mode: failed to parse Piggyback JSON after all attempts")
	logging.ArticulationDebug("Validation failure stats: total=%d, failures=%d",
		rp.stats.TotalProcessed, rp.stats.ValidationFailures)
	return nil, fmt.Errorf("failed to parse Piggyback JSON: %w", err)
}

// applyCaps enforces surface/control size limits to avoid runaway payloads.
func (rp *ResponseProcessor) applyCaps(result *ArticulationResult) {
	if result == nil {
		return
	}

	// Surface length cap
	if rp.MaxSurfaceLength > 0 && len(result.Surface) > rp.MaxSurfaceLength {
		result.Surface = result.Surface[:rp.MaxSurfaceLength] + "\n\n[TRUNCATED]"
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("Surface response truncated to %d chars", rp.MaxSurfaceLength))
	}

	// Control packet caps (defensive)
	const maxMangleUpdates = 2000
	if len(result.Control.MangleUpdates) > maxMangleUpdates {
		result.Control.MangleUpdates = result.Control.MangleUpdates[:maxMangleUpdates]
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("Mangle updates truncated to %d atoms", maxMangleUpdates))
	}

	const maxMemoryOps = 500
	if len(result.Control.MemoryOperations) > maxMemoryOps {
		result.Control.MemoryOperations = result.Control.MemoryOperations[:maxMemoryOps]
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("Memory operations truncated to %d items", maxMemoryOps))
	}
}

// parseJSON attempts direct JSON parsing.
func (rp *ResponseProcessor) parseJSON(s string) (PiggybackEnvelope, error) {
	timer := logging.StartTimer(logging.CategoryArticulation, "parseJSON")
	defer timer.Stop()

	s = strings.TrimSpace(s)
	logging.ArticulationDebug("parseJSON: input length=%d bytes", len(s))

	var envelope PiggybackEnvelope
	if err := json.Unmarshal([]byte(s), &envelope); err != nil {
		logging.ArticulationDebug("parseJSON: direct unmarshal failed: %v", err)
		// Be tolerant of leading text or trailing decorations by decoding the
		// first JSON object we can find (streaming decoder stops at end of object).
		if idx := strings.Index(s, "{"); idx >= 0 {
			logging.ArticulationDebug("parseJSON: found '{' at index %d, trying streaming decoder", idx)
			decoder := json.NewDecoder(strings.NewReader(s[idx:]))
			if derr := decoder.Decode(&envelope); derr != nil {
				logging.ArticulationDebug("parseJSON: streaming decode also failed: %v", derr)
				return PiggybackEnvelope{}, err
			}
			logging.ArticulationDebug("parseJSON: streaming decode succeeded")
		} else {
			logging.ArticulationDebug("parseJSON: no '{' found in input")
			return PiggybackEnvelope{}, err
		}
	}

	// Validate required fields (surface always required)
	if envelope.Surface == "" {
		logging.ArticulationDebug("parseJSON: validation failed - missing surface_response field")
		return PiggybackEnvelope{}, fmt.Errorf("missing surface_response field")
	}

	// In strict mode, also require a minimally valid control packet.
	if rp.RequireValidJSON {
		ic := envelope.Control.IntentClassification
		if ic.Category == "" || ic.Verb == "" {
			return PiggybackEnvelope{}, fmt.Errorf("missing intent_classification fields")
		}
		if envelope.Control.MangleUpdates == nil {
			return PiggybackEnvelope{}, fmt.Errorf("missing mangle_updates field")
		}
	}

	logging.ArticulationDebug("parseJSON: successfully parsed envelope with surface_length=%d", len(envelope.Surface))
	return envelope, nil
}

// parseMarkdownWrappedJSON handles ```json ... ``` wrapping.
func (rp *ResponseProcessor) parseMarkdownWrappedJSON(s string) (PiggybackEnvelope, error) {
	timer := logging.StartTimer(logging.CategoryArticulation, "parseMarkdownWrappedJSON")
	defer timer.Stop()

	s = strings.TrimSpace(s)
	originalLen := len(s)

	// Remove markdown code block markers
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```JSON")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)

	strippedLen := len(s)
	if strippedLen != originalLen {
		logging.ArticulationDebug("parseMarkdownWrappedJSON: stripped markdown markers (original=%d, stripped=%d)",
			originalLen, strippedLen)
	} else {
		logging.ArticulationDebug("parseMarkdownWrappedJSON: no markdown markers found")
	}

	return rp.parseJSON(s)
}

// extractEmbeddedJSON finds JSON within mixed content.
func (rp *ResponseProcessor) extractEmbeddedJSON(s string) (PiggybackEnvelope, error) {
	timer := logging.StartTimer(logging.CategoryArticulation, "extractEmbeddedJSON")
	defer timer.Stop()

	logging.ArticulationDebug("extractEmbeddedJSON: searching in %d bytes of content", len(s))

	// Pattern to find JSON objects containing both keys, regardless of order
	jsonPattern := regexp.MustCompile(`\{[\s\S]*("surface_response"[\s\S]*"control_packet"|"control_packet"[\s\S]*"surface_response")[\s\S]*\}`)

	match := jsonPattern.FindString(s)
	if match == "" {
		logging.ArticulationDebug("extractEmbeddedJSON: primary pattern (surface_response/control_packet) not found, trying fallback")
		// Try alternative pattern
		jsonPattern = regexp.MustCompile(`\{[\s\S]*\}`)
		matches := jsonPattern.FindAllString(s, -1)
		logging.ArticulationDebug("extractEmbeddedJSON: fallback pattern found %d potential JSON objects", len(matches))

		// Try each match, largest first
		for i := len(matches) - 1; i >= 0; i-- {
			logging.ArticulationDebug("extractEmbeddedJSON: trying match %d (length=%d)", i, len(matches[i]))
			envelope, err := rp.parseJSON(matches[i])
			if err == nil {
				logging.ArticulationDebug("extractEmbeddedJSON: match %d parsed successfully", i)
				return envelope, nil
			}
			logging.ArticulationDebug("extractEmbeddedJSON: match %d failed: %v", i, err)
		}

		logging.ArticulationDebug("extractEmbeddedJSON: all matches failed")
		return PiggybackEnvelope{}, fmt.Errorf("no embedded JSON found")
	}

	logging.ArticulationDebug("extractEmbeddedJSON: primary pattern matched (length=%d)", len(match))
	return rp.parseJSON(match)
}

// GetStats returns current processing statistics.
func (rp *ResponseProcessor) GetStats() ProcessorStats {
	logging.ArticulationDebug("GetStats: total=%d, success=%d, fallback=%d, failures=%d, self_corrections=%d",
		rp.stats.TotalProcessed, rp.stats.SuccessfulParses, rp.stats.FallbackParses,
		rp.stats.ValidationFailures, rp.stats.SelfCorrections)
	return rp.stats
}

// ResetStats resets the processing statistics.
func (rp *ResponseProcessor) ResetStats() {
	logging.ArticulationDebug("ResetStats: clearing processor statistics")
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
	logging.Articulation("Initializing new Emitter")
	e := &Emitter{
		processor:   NewResponseProcessor(),
		PrettyPrint: true,
		IncludeRaw:  false,
	}
	logging.ArticulationDebug("Emitter initialized: PrettyPrint=%v, IncludeRaw=%v", e.PrettyPrint, e.IncludeRaw)
	return e
}

// Emit outputs the dual payload as JSON.
func (e *Emitter) Emit(payload PiggybackEnvelope) error {
	timer := logging.StartTimer(logging.CategoryArticulation, "Emit")
	defer timer.Stop()

	logging.Articulation("Emitting Piggyback envelope (surface_length=%d, mangle_updates=%d, memory_ops=%d)",
		len(payload.Surface), len(payload.Control.MangleUpdates), len(payload.Control.MemoryOperations))
	logging.ArticulationDebug("Emit: intent=%s/%s/%s, confidence=%.2f",
		payload.Control.IntentClassification.Category,
		payload.Control.IntentClassification.Verb,
		payload.Control.IntentClassification.Target,
		payload.Control.IntentClassification.Confidence)

	var data []byte
	var err error

	if e.PrettyPrint {
		logging.ArticulationDebug("Emit: using pretty-print format")
		data, err = json.MarshalIndent(payload, "", "  ")
	} else {
		logging.ArticulationDebug("Emit: using compact format")
		data, err = json.Marshal(payload)
	}

	if err != nil {
		logging.Get(logging.CategoryArticulation).Error("Emit: failed to marshal envelope: %v", err)
		return err
	}

	logging.ArticulationDebug("Emit: marshaled %d bytes", len(data))
	fmt.Println(string(data))
	logging.Articulation("Emit: successfully output %d bytes", len(data))
	return nil
}

// EmitSurface outputs only the surface response.
func (e *Emitter) EmitSurface(payload PiggybackEnvelope) {
	logging.Articulation("EmitSurface: outputting surface channel only (length=%d)", len(payload.Surface))
	logging.ArticulationDebug("EmitSurface: control packet discarded (mangle_updates=%d, memory_ops=%d)",
		len(payload.Control.MangleUpdates), len(payload.Control.MemoryOperations))
	fmt.Println(strings.TrimSpace(payload.Surface))
}

// ParseAndProcess parses raw LLM output and returns structured result.
func (e *Emitter) ParseAndProcess(rawResponse string) (*ArticulationResult, error) {
	logging.Articulation("ParseAndProcess: delegating to processor (input_length=%d)", len(rawResponse))
	result, err := e.processor.Process(rawResponse)
	if err != nil {
		logging.Get(logging.CategoryArticulation).Error("ParseAndProcess: processing failed: %v", err)
		return nil, err
	}
	logging.Articulation("ParseAndProcess: completed (method=%s, confidence=%.2f, warnings=%d)",
		result.ParseMethod, result.Confidence, len(result.Warnings))
	return result, nil
}

// CreateEnvelope creates a PiggybackEnvelope from components.
func (e *Emitter) CreateEnvelope(surface string, intent IntentClassification, mangleUpdates []string, memOps []MemoryOperation) PiggybackEnvelope {
	logging.Articulation("CreateEnvelope: assembling Piggyback envelope")
	logging.ArticulationDebug("CreateEnvelope: surface_length=%d, intent=%s/%s, mangle_updates=%d, memory_ops=%d",
		len(surface), intent.Category, intent.Verb, len(mangleUpdates), len(memOps))

	envelope := PiggybackEnvelope{
		Surface: surface,
		Control: ControlPacket{
			IntentClassification: intent,
			MangleUpdates:        mangleUpdates,
			MemoryOperations:     memOps,
		},
	}

	logging.ArticulationDebug("CreateEnvelope: envelope assembled successfully")
	return envelope
}

// MarshalEnvelope converts an envelope to JSON bytes.
func (e *Emitter) MarshalEnvelope(envelope PiggybackEnvelope) ([]byte, error) {
	timer := logging.StartTimer(logging.CategoryArticulation, "MarshalEnvelope")
	defer timer.Stop()

	logging.ArticulationDebug("MarshalEnvelope: serializing envelope (pretty=%v)", e.PrettyPrint)

	var data []byte
	var err error

	if e.PrettyPrint {
		data, err = json.MarshalIndent(envelope, "", "  ")
	} else {
		data, err = json.Marshal(envelope)
	}

	if err != nil {
		logging.Get(logging.CategoryArticulation).Error("MarshalEnvelope: failed to marshal: %v", err)
		return nil, err
	}

	logging.ArticulationDebug("MarshalEnvelope: serialized %d bytes", len(data))
	return data, nil
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
		logging.ArticulationDebug("ApplyConstitutionalOverride: no override needed (blocked=%d, reason empty)", len(blocked))
		return nil // No override needed
	}

	logging.Articulation("ApplyConstitutionalOverride: applying constitutional override (blocked=%d, reason=%q)",
		len(blocked), reason)

	override := &ConstitutionalOverride{
		OriginalSurface: envelope.Surface,
		BlockedAtoms:    blocked,
		Reason:          reason,
	}

	// Modify surface if there's a safety concern
	if reason != "" {
		logging.Get(logging.CategoryArticulation).Warn("Constitutional safety override: %s", reason)
		override.ModifiedSurface = fmt.Sprintf("[SAFETY NOTICE: %s]\n\n%s", reason, envelope.Surface)
		envelope.Surface = override.ModifiedSurface
		logging.ArticulationDebug("ApplyConstitutionalOverride: surface modified with safety notice")
	}

	// Filter blocked atoms from mangle_updates
	if len(blocked) > 0 {
		originalCount := len(envelope.Control.MangleUpdates)
		filtered := make([]string, 0, len(envelope.Control.MangleUpdates))
		blockedSet := make(map[string]bool)
		for _, b := range blocked {
			blockedSet[b] = true
			logging.ArticulationDebug("ApplyConstitutionalOverride: blocking atom: %s", b)
		}

		for _, atom := range envelope.Control.MangleUpdates {
			if !blockedSet[atom] {
				filtered = append(filtered, atom)
			}
		}
		envelope.Control.MangleUpdates = filtered
		logging.Articulation("ApplyConstitutionalOverride: filtered mangle_updates from %d to %d atoms",
			originalCount, len(filtered))
	}

	return override
}

// =============================================================================
// REASONING TRACE DIRECTIVE
// =============================================================================
// Standard directive appended to shard prompts to mandate structured reasoning
// output. This enables trace capture and self-learning across all shard types.

// ReasoningTraceDirective is the standard suffix for shard system prompts.
// It instructs the LLM to include structured reasoning in its output.
const ReasoningTraceDirective = `

## REASONING TRACE (MANDATORY)
You MUST include a "reasoning_trace" field in your output with your step-by-step thinking process:
1. What is the task asking for?
2. What approach will you take?
3. What are the key considerations/constraints?
4. What is your confidence level and why?

Format your response as JSON with:
{
  "reasoning_trace": "Step-by-step reasoning...",
  "result": <your actual output>
}

The reasoning_trace captures your thinking for learning and improvement.`

// ShardReasoningDirective is a shorter directive for simpler shards.
const ShardReasoningDirective = `

## REASONING OUTPUT
Include a brief "reasoning" field explaining your approach and confidence.`

// AppendReasoningDirective appends the standard reasoning directive to a prompt.
func AppendReasoningDirective(systemPrompt string, full bool) string {
	if full {
		logging.ArticulationDebug("AppendReasoningDirective: appending full reasoning trace directive")
		return systemPrompt + ReasoningTraceDirective
	}
	logging.ArticulationDebug("AppendReasoningDirective: appending short shard reasoning directive")
	return systemPrompt + ShardReasoningDirective
}

// =============================================================================
// UTILITY FUNCTIONS
// =============================================================================

// ExtractSurfaceOnly extracts just the surface response, ignoring control packet.
// Useful for display purposes when only the user-facing text is needed.
func ExtractSurfaceOnly(rawResponse string) string {
	timer := logging.StartTimer(logging.CategoryArticulation, "ExtractSurfaceOnly")
	defer timer.Stop()

	logging.ArticulationDebug("ExtractSurfaceOnly: extracting surface from %d bytes", len(rawResponse))

	processor := NewResponseProcessor()
	processor.RequireValidJSON = false

	result, err := processor.Process(rawResponse)
	if err != nil {
		logging.ArticulationDebug("ExtractSurfaceOnly: processing failed, returning raw response")
		return strings.TrimSpace(rawResponse)
	}

	logging.ArticulationDebug("ExtractSurfaceOnly: extracted surface (length=%d, method=%s)",
		len(result.Surface), result.ParseMethod)
	return result.Surface
}

// HasSelfCorrection checks if the response indicates self-correction was triggered.
func HasSelfCorrection(envelope PiggybackEnvelope) bool {
	triggered := envelope.Control.SelfCorrection != nil && envelope.Control.SelfCorrection.Triggered
	if triggered {
		logging.ArticulationDebug("HasSelfCorrection: self-correction detected (hypothesis=%s)",
			envelope.Control.SelfCorrection.Hypothesis)
	}
	return triggered
}

// HasMemoryOperations checks if there are memory operations to process.
func HasMemoryOperations(envelope PiggybackEnvelope) bool {
	hasOps := len(envelope.Control.MemoryOperations) > 0
	if hasOps {
		logging.ArticulationDebug("HasMemoryOperations: %d memory operations present", len(envelope.Control.MemoryOperations))
	}
	return hasOps
}

// GetMemoryOperationsByType filters memory operations by operation type.
func GetMemoryOperationsByType(envelope PiggybackEnvelope, opType string) []MemoryOperation {
	logging.ArticulationDebug("GetMemoryOperationsByType: filtering for op_type=%q", opType)
	result := make([]MemoryOperation, 0)
	for _, op := range envelope.Control.MemoryOperations {
		if op.Op == opType {
			result = append(result, op)
		}
	}
	logging.ArticulationDebug("GetMemoryOperationsByType: found %d operations of type %q", len(result), opType)
	return result
}

// =============================================================================
// SHARED PIGGYBACK PROCESSING FOR SHARDS
// =============================================================================
// All shards that use LLM responses MUST process them through this layer to:
// 1. Extract and route control_packet data to the kernel
// 2. Return only surface_response to the user
// This prevents control data from leaking into user-facing output.

// ProcessedLLMResponse contains the separated components of a Piggyback response.
type ProcessedLLMResponse struct {
	Surface     string         // User-facing text (safe for display)
	Control     *ControlPacket // Control packet (route to kernel)
	ParseMethod string         // How the response was parsed
	Confidence  float64        // Parsing confidence
}

// ProcessLLMResponse is a convenience function for shards to process LLM responses.
// It extracts the surface_response and control_packet from a Piggyback-formatted
// LLM response. The surface is safe for user display; the control should be
// routed to the kernel.
//
// Usage in any shard:
//
//	rawResponse, err := llmClient.Complete(ctx, prompt)
//	processed := articulation.ProcessLLMResponse(rawResponse)
//	// Display: processed.Surface
//	// Route to kernel: processed.Control
func ProcessLLMResponse(rawResponse string) *ProcessedLLMResponse {
	logging.ArticulationDebug("ProcessLLMResponse: processing %d bytes", len(rawResponse))

	processor := NewResponseProcessor()
	processor.RequireValidJSON = false // Allow fallback to raw text

	result, err := processor.Process(rawResponse)
	if err != nil {
		logging.Get(logging.CategoryArticulation).Warn("ProcessLLMResponse: parse failed, using raw: %v", err)
		return &ProcessedLLMResponse{
			Surface:     strings.TrimSpace(rawResponse),
			Control:     nil,
			ParseMethod: "fallback",
			Confidence:  0.0,
		}
	}

	logging.Articulation("ProcessLLMResponse: method=%s, confidence=%.2f, surface_len=%d",
		result.ParseMethod, result.Confidence, len(result.Surface))

	processed := &ProcessedLLMResponse{
		Surface:     result.Surface,
		ParseMethod: result.ParseMethod,
		Confidence:  result.Confidence,
	}

	// Only include control packet if we actually parsed it
	if result.ParseMethod != "fallback" {
		processed.Control = &result.Control
	}

	return processed
}

// ProcessLLMResponseAllowPlain treats non-Piggyback responses as expected output.
// It avoids emitting error logs when the response is intentionally plain text.
func ProcessLLMResponseAllowPlain(rawResponse string) *ProcessedLLMResponse {
	logging.ArticulationDebug("ProcessLLMResponseAllowPlain: processing %d bytes", len(rawResponse))

	processor := NewResponseProcessor()
	processor.RequireValidJSON = false
	processor.LogFallbackAsError = false

	result, err := processor.Process(rawResponse)
	if err != nil {
		logging.Get(logging.CategoryArticulation).Warn("ProcessLLMResponseAllowPlain: parse failed, using raw: %v", err)
		return &ProcessedLLMResponse{
			Surface:     strings.TrimSpace(rawResponse),
			Control:     nil,
			ParseMethod: "fallback",
			Confidence:  0.0,
		}
	}

	logging.Articulation("ProcessLLMResponseAllowPlain: method=%s, confidence=%.2f, surface_len=%d",
		result.ParseMethod, result.Confidence, len(result.Surface))

	processed := &ProcessedLLMResponse{
		Surface:     result.Surface,
		ParseMethod: result.ParseMethod,
		Confidence:  result.Confidence,
	}

	if result.ParseMethod != "fallback" {
		processed.Control = &result.Control
	}

	return processed
}

// MustExtractSurface extracts only the surface response, returning raw on failure.
// Use this when you only need the user-facing text and don't care about control.
func MustExtractSurface(rawResponse string) string {
	processed := ProcessLLMResponse(rawResponse)
	return processed.Surface
}
