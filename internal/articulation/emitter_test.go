package articulation

import (
	"strings"
	"testing"
)

func TestResponseProcessor_Process_JSON(t *testing.T) {
	rp := NewResponseProcessor()

	raw := `{
	  "control_packet": {
	    "intent_classification": {
	      "category": "/query",
	      "verb": "/explain",
	      "target": "x",
	      "constraint": "none",
	      "confidence": 0.9
	    },
	    "mangle_updates": ["a()."],
	    "memory_operations": []
	  },
	  "surface_response": "hello"
	}`

	res, err := rp.Process(raw)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if res.ParseMethod != "json" {
		t.Fatalf("ParseMethod = %q, want json", res.ParseMethod)
	}
	if res.Surface != "hello" {
		t.Fatalf("Surface = %q, want hello", res.Surface)
	}
	if len(res.Control.MangleUpdates) != 1 {
		t.Fatalf("MangleUpdates = %d, want 1", len(res.Control.MangleUpdates))
	}
}

func TestResponseProcessor_Process_MarkdownWrapped(t *testing.T) {
	rp := NewResponseProcessor()

	raw := "```json\n" + `{"control_packet":{"intent_classification":{"category":"/query","verb":"/explain","target":"x","constraint":"none","confidence":1},"mangle_updates":[],"memory_operations":[]},"surface_response":"ok"}` + "\n```"

	res, err := rp.Process(raw)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if res.ParseMethod != "json_markdown" && res.ParseMethod != "json" {
		t.Fatalf("ParseMethod = %q, want json_markdown or json", res.ParseMethod)
	}
	if res.Surface != "ok" {
		t.Fatalf("Surface = %q, want ok", res.Surface)
	}
}

func TestResponseProcessor_extractEmbeddedJSON_OrderAgnostic(t *testing.T) {
	rp := NewResponseProcessor()

	raw := `prefix {"control_packet":{"intent_classification":{"category":"/query","verb":"/explain","target":"x","constraint":"none","confidence":1},"mangle_updates":[],"memory_operations":[]},"surface_response":"hi"} suffix`

	env, err := rp.extractEmbeddedJSON(raw)
	if err != nil {
		t.Fatalf("extractEmbeddedJSON() error = %v", err)
	}
	if env.Surface != "hi" {
		t.Fatalf("Surface = %q, want hi", env.Surface)
	}
}

func TestResponseProcessor_Process_StrictValidation(t *testing.T) {
	rp := NewResponseProcessor()
	rp.RequireValidJSON = true

	_, err := rp.Process(`{"control_packet":{},"surface_response":"hi"}`)
	if err == nil {
		t.Fatal("expected error in strict mode, got nil")
	}
}

func TestResponseProcessor_Process_SurfaceTruncation(t *testing.T) {
	rp := NewResponseProcessor()
	rp.MaxSurfaceLength = 5

	raw := `{"control_packet":{"intent_classification":{"category":"/query","verb":"/explain","target":"x","constraint":"none","confidence":1},"mangle_updates":[],"memory_operations":[]},"surface_response":"123456789"}`
	res, err := rp.Process(raw)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if !strings.HasPrefix(res.Surface, "12345") || !strings.Contains(res.Surface, "[TRUNCATED]") {
		t.Fatalf("Surface not truncated as expected: %q", res.Surface)
	}
}

func TestResponseProcessor_Process_ControlCaps(t *testing.T) {
	rp := NewResponseProcessor()

	updates := make([]string, 2001)
	for i := range updates {
		updates[i] = "a()."
	}
	var sb strings.Builder
	sb.WriteString(`{"control_packet":{"intent_classification":{"category":"/query","verb":"/explain","target":"x","constraint":"none","confidence":1},"mangle_updates":[`)
	for i, u := range updates {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(`"` + u + `"`)
	}
	sb.WriteString(`],"memory_operations":[]},"surface_response":"ok"}`)

	res, err := rp.Process(sb.String())
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if len(res.Control.MangleUpdates) != 2000 {
		t.Fatalf("MangleUpdates = %d, want 2000", len(res.Control.MangleUpdates))
	}
	foundWarn := false
	for _, w := range res.Warnings {
		if strings.Contains(w, "Mangle updates truncated") {
			foundWarn = true
			break
		}
	}
	if !foundWarn {
		t.Fatalf("expected truncation warning, got %v", res.Warnings)
	}
}

func FuzzResponseProcessor_Process(f *testing.F) {
	seeds := []string{
		`{"control_packet":{"intent_classification":{"category":"/query","verb":"/explain","target":"x","constraint":"none","confidence":1},"mangle_updates":[],"memory_operations":[]},"surface_response":"ok"}`,
		"```json\n{\"control_packet\":{},\"surface_response\":\"hi\"}\n```",
		"noise {\"control_packet\":{},\"surface_response\":\"mixed\"} tail",
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		rp := NewResponseProcessor()
		rp.RequireValidJSON = false
		_, _ = rp.Process(raw)
	})
}

func TestResponseProcessor_Process_NullFields(t *testing.T) {
	rp := NewResponseProcessor()
	rp.RequireValidJSON = true

	// JSON with explicit nulls for array/pointer fields
	raw := `{
	  "control_packet": {
	    "intent_classification": {
	      "category": "/query",
	      "verb": "/explain",
	      "target": "x",
	      "constraint": "none",
	      "confidence": 0.9
	    },
	    "mangle_updates": null,
	    "memory_operations": null,
	    "tool_requests": null,
	    "self_correction": null,
        "context_feedback": null,
        "knowledge_requests": null
	  },
	  "surface_response": "hello"
	}`

	res, err := rp.Process(raw)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	if res.Control.MangleUpdates != nil && len(res.Control.MangleUpdates) != 0 {
		t.Errorf("Expected MangleUpdates to be nil or empty, got %v", res.Control.MangleUpdates)
	}
	if res.Control.MemoryOperations != nil && len(res.Control.MemoryOperations) != 0 {
		t.Errorf("Expected MemoryOperations to be nil or empty, got %v", res.Control.MemoryOperations)
	}
	if res.Control.ToolRequests != nil && len(res.Control.ToolRequests) != 0 {
		t.Errorf("Expected ToolRequests to be nil or empty, got %v", res.Control.ToolRequests)
	}
	if res.Control.SelfCorrection != nil {
		t.Errorf("Expected SelfCorrection to be nil, got %v", res.Control.SelfCorrection)
	}
	if res.Control.ContextFeedback != nil {
		t.Errorf("Expected ContextFeedback to be nil, got %v", res.Control.ContextFeedback)
	}
	if res.Control.KnowledgeRequests != nil && len(res.Control.KnowledgeRequests) != 0 {
		t.Errorf("Expected KnowledgeRequests to be nil or empty, got %v", res.Control.KnowledgeRequests)
	}

	if res.Surface != "hello" {
		t.Errorf("Surface = %q, want hello", res.Surface)
	}
}

func TestResponseProcessor_Process_TypeCoercion(t *testing.T) {
	rp := NewResponseProcessor()
	rp.RequireValidJSON = true

	// Case 1: String for float
	raw1 := `{
	  "control_packet": {
	    "intent_classification": {
	      "category": "/query",
	      "verb": "/explain",
	      "target": "x",
	      "constraint": "none",
	      "confidence": "0.9"
	    },
	    "mangle_updates": [],
	    "memory_operations": []
	  },
	  "surface_response": "hello"
	}`

	_, err := rp.Process(raw1)
	if err == nil {
		t.Fatal("Expected error for stringified float, got nil")
	}
	// Check for unmarshal error
	if !strings.Contains(err.Error(), "cannot unmarshal") {
		t.Errorf("Unexpected error message: %v", err)
	}

	// Case 2: String for array
	raw2 := `{
	  "control_packet": {
	    "intent_classification": {
	      "category": "/query",
	      "verb": "/explain",
	      "target": "x",
	      "constraint": "none",
	      "confidence": 0.9
	    },
	    "mangle_updates": "a().",
	    "memory_operations": []
	  },
	  "surface_response": "hello"
	}`

	_, err = rp.Process(raw2)
	if err == nil {
		t.Fatal("Expected error for stringified array, got nil")
	}
}

// TODO: TEST_GAP: User Request Extremes - Verify behavior with massive 'reasoning_trace' (>50MB) to ensure OOM protection.
// TODO: TEST_GAP: User Request Extremes - Verify recursion depth limits for nested JSON objects to prevent stack overflow.
// TODO: TEST_GAP: State Conflicts - Verify behavior when JSON contains duplicate keys (e.g. multiple 'surface_response' fields) - which one wins?
// TODO: TEST_GAP: Performance - Benchmark 'extractEmbeddedJSON' regex against massive inputs with near-matches to detect catastrophic backtracking.

// TODO: TEST_GAP: Decoy Injection / State Conflict - Verify which candidate is selected when multiple valid-looking JSONs exist.
// Scenario: Input contains `Example: {"control_packet": ...}` followed by `Real: {"control_packet": ...}`.
// Currently, Pass 1 picks the FIRST one (Decoy). We need a test to enforce a safer selection policy (e.g. Last One Wins, or strict wrapper).

// TODO: TEST_GAP: Hallucinated Keys - Verify behavior when keys are present but slightly wrong (e.g. "control_packets").
// The heuristic scanner might pick it up, but `json.Unmarshal` will produce a default zero-value struct.
// Test should confirm that strict mode (`RequireValidJSON`) correctly rejects this, while loose mode creates a safe fallback.

// TODO: TEST_GAP: DoS / Resource Exhaustion - Verify `extractEmbeddedJSON` performance when input contains 10,000+ candidates.
// Each candidate triggers `json.Unmarshal`. This test should measure the time taken and ensure it fails if it exceeds a threshold (e.g. 100ms).
// Mitigation: implement a candidate limit or size limit.

// TODO: TEST_GAP: Malformed Hiding - Verify behavior when a malformed real response (missing brace) hides a subsequent decoy.
// Scenario: `{"real": ... (missing brace) ... {"decoy": ...}`.
// The scanner might skip the real response but pick up the decoy if depth resets or aligns.
