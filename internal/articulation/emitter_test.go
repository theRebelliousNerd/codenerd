package articulation

import (
	"fmt"
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

	res1, err := rp.Process(raw1)
	if err != nil {
		t.Fatalf("Expected no error for stringified float, got: %v", err)
	}
	if res1.Control.IntentClassification.Confidence != 0.9 {
		t.Errorf("Expected confidence 0.9, got %v", res1.Control.IntentClassification.Confidence)
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

	res2, err := rp.Process(raw2)
	if err != nil {
		t.Fatalf("Expected no error for stringified array, got: %v", err)
	}
	if len(res2.Control.MangleUpdates) != 1 || res2.Control.MangleUpdates[0] != "a()." {
		t.Errorf("Expected mangle_updates [a().], got %v", res2.Control.MangleUpdates)
	}

	// Case 3: Descriptive string for float
	raw3 := `{
	  "control_packet": {
	    "intent_classification": {
	      "category": "/query",
	      "verb": "/explain",
	      "confidence": "high"
	    }
	  },
	  "surface_response": "hello"
	}`
	res3, err := rp.Process(raw3)
	if err != nil {
		t.Fatalf("Expected no error for descriptive float, got: %v", err)
	}
	if res3.Control.IntentClassification.Confidence != 0.9 {
		t.Errorf("Expected confidence 0.9 for 'high', got %v", res3.Control.IntentClassification.Confidence)
	}

	// Case 4: Stringified boolean in ToolRequests
	raw4 := `{
	  "control_packet": {
	    "intent_classification": {
	      "category": "/query",
	      "verb": "/explain"
	    },
	    "tool_requests": [
	      {
	        "id": "req_1",
	        "tool_name": "read_file",
	        "tool_args": {},
	        "required": "true"
	      }
	    ]
	  },
	  "surface_response": "hello"
	}`
	res4, err := rp.Process(raw4)
	if err != nil {
		t.Fatalf("Expected no error for stringified bool, got: %v", err)
	}
	if len(res4.Control.ToolRequests) != 1 || !res4.Control.ToolRequests[0].Required {
		t.Errorf("Expected ToolRequest Required to be true, got false or missing")
	}
}

func TestResponseProcessor_Process_MassiveReasoningTrace(t *testing.T) {
	rp := NewResponseProcessor()
	
	// Create a 50MB reasoning trace
	massiveTrace := strings.Repeat("A", 50*1024*1024)
	raw := fmt.Sprintf(`{"control_packet":{"intent_classification":{"category":"/query","verb":"/explain","target":"x","constraint":"none","confidence":1},"reasoning_trace":"%s"},"surface_response":"hi"}`, massiveTrace)
	
	res, err := rp.Process(raw)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	
	if len(res.Control.ReasoningTrace) > 60000 {
		t.Fatalf("ReasoningTrace was not truncated: len=%d", len(res.Control.ReasoningTrace))
	}
	if !strings.HasSuffix(res.Control.ReasoningTrace, "[TRUNCATED]") {
		t.Fatalf("ReasoningTrace did not end with [TRUNCATED]")
	}
}

func TestResponseProcessor_Process_RecursionDepth(t *testing.T) {
	rp := NewResponseProcessor()
	
	// Create deeply nested JSON
	depth := 10000
	var sb strings.Builder
	for i := 0; i < depth; i++ {
		sb.WriteString(`{"a":`)
	}
	sb.WriteString(`1`)
	for i := 0; i < depth; i++ {
		sb.WriteString(`}`)
	}
	
	// This will likely fail to parse, but shouldn't stack overflow
	raw := fmt.Sprintf(`{"control_packet":{"intent_classification":{"category":"/query","verb":"/explain","target":"x","constraint":"none","confidence":1},"reasoning_trace":%s},"surface_response":"hi"}`, sb.String())
	
	_, _ = rp.Process(raw) // We just want to ensure it doesn't crash
}

func TestResponseProcessor_Process_DuplicateKeys(t *testing.T) {
	rp := NewResponseProcessor()
	
	// Standard JSON decoding in Go uses last-wins for duplicate keys
	raw := `{"control_packet":{"intent_classification":{"category":"/query","verb":"/explain","target":"x","constraint":"none","confidence":1}},"surface_response":"first","surface_response":"second"}`
	
	res, err := rp.Process(raw)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	
	if res.Surface != "second" {
		t.Fatalf("Expected last-wins for duplicate keys, got %q", res.Surface)
	}
}

func TestResponseProcessor_ExtractEmbeddedJSON_CatastrophicBacktracking(t *testing.T) {
	rp := NewResponseProcessor()
	
	// Benchmark extractEmbeddedJSON with lots of braces
	massiveNoise := strings.Repeat("{ } ", 10000)
	raw := massiveNoise + `{"control_packet":{"intent_classification":{"category":"/query","verb":"/explain","target":"x","constraint":"none","confidence":1}},"surface_response":"hi"}` + massiveNoise
	
	_, err := rp.extractEmbeddedJSON(raw)
	if err != nil {
		t.Fatalf("extractEmbeddedJSON() error = %v", err)
	}
}

func TestResponseProcessor_ExtractEmbeddedJSON_DecoyInjection(t *testing.T) {
	rp := NewResponseProcessor()
	
	// Real comes second, Decoy comes first
	raw := `Example: {"control_packet":{"intent_classification":{"category":"/decoy","verb":"/decoy","target":"x","constraint":"none","confidence":1}},"surface_response":"decoy"}
Real: {"control_packet":{"intent_classification":{"category":"/real","verb":"/real","target":"x","constraint":"none","confidence":1}},"surface_response":"real"}`
	
	env, err := rp.extractEmbeddedJSON(raw)
	if err != nil {
		t.Fatalf("extractEmbeddedJSON() error = %v", err)
	}
	
	if env.Surface != "real" {
		t.Fatalf("Expected real surface to win, got %q", env.Surface)
	}
	if env.Control.IntentClassification.Category != "/real" {
		t.Fatalf("Expected real intent to win, got %q", env.Control.IntentClassification.Category)
	}
}

func TestResponseProcessor_Process_HallucinatedKeys(t *testing.T) {
	rpStrict := NewResponseProcessor()
	rpStrict.RequireValidJSON = true
	
	// "control_packets" instead of "control_packet"
	raw := `{"control_packets":{"intent_classification":{"category":"/query","verb":"/explain","target":"x","constraint":"none","confidence":1}},"surface_response":"hi"}`
	
	_, err := rpStrict.Process(raw)
	if err == nil {
		t.Fatalf("Expected strict processor to fail on hallucinated keys")
	}
	
	rpLoose := NewResponseProcessor()
	rpLoose.RequireValidJSON = false
	res, err := rpLoose.Process(raw)
	if err != nil {
		t.Fatalf("Loose processor should not fail")
	}
	// In loose mode, it parses successfully but the missing fields are default-zero.
	if res.ParseMethod != "json" {
		t.Fatalf("Expected json parsing for loose processor, got %s", res.ParseMethod)
	}
	if res.Control.IntentClassification.Category != "" {
		t.Fatalf("Expected empty category due to hallucinated key, got %q", res.Control.IntentClassification.Category)
	}
}

func TestResponseProcessor_ExtractEmbeddedJSON_DOS(t *testing.T) {
	rp := NewResponseProcessor()
	
	// Create 10,000 JSON objects that are NOT valid Piggyback Envelopes
	var sb strings.Builder
	for i := 0; i < 10000; i++ {
		sb.WriteString(`{"a": 1} `)
	}
	sb.WriteString(`{"control_packet":{"intent_classification":{"category":"/query","verb":"/explain","target":"x","constraint":"none","confidence":1}},"surface_response":"hi"}`)
	
	// Must not take too long
	_, err := rp.extractEmbeddedJSON(sb.String())
	if err != nil {
		t.Fatalf("extractEmbeddedJSON() error = %v", err)
	}
}

func TestResponseProcessor_Process_MalformedHiding(t *testing.T) {
	rp := NewResponseProcessor()
	rp.RequireValidJSON = false // Allow fallback
	
	// Real is missing a brace, so it's malformed. Decoy follows it.
	raw := `Real: {"control_packet":{"intent_classification":{"category":"/query","verb":"/explain","target":"x","constraint":"none","confidence":1}},"surface_response":"hi"
Decoy: {"control_packet":{"intent_classification":{"category":"/decoy","verb":"/decoy","target":"x","constraint":"none","confidence":1}},"surface_response":"decoy"}`
	
	res, err := rp.Process(raw)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	
	// Because of brace counting, the entire block is seen as one malformed candidate.
	// It should fallback to parsing the entire text as surface response rather than 
	// getting confused and picking up the decoy.
	if res.ParseMethod != "fallback" {
		t.Fatalf("Expected fallback for malformed hiding, got %q", res.ParseMethod)
	}
}

func TestResponseProcessor_Process_NonASCII(t *testing.T) {
	rp := NewResponseProcessor()
	
	raw := `{"control_packet":{"intent_classification":{"category":"/query","verb":"/explain","target":"x","constraint":"none","confidence":1},"reasoning_trace":"🤔💡"},"surface_response":"こんにちは"}`
	
	res, err := rp.Process(raw)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	
	if res.Surface != "こんにちは" {
		t.Fatalf("Expected Japanese surface, got %q", res.Surface)
	}
	if res.Control.ReasoningTrace != "🤔💡" {
		t.Fatalf("Expected Emoji reasoning trace, got %q", res.Control.ReasoningTrace)
	}
}

func TestResponseProcessor_ExtractEmbeddedJSON_BraceImbalance(t *testing.T) {
	rp := NewResponseProcessor()
	
	raw := strings.Repeat("{", 1000) + `{"control_packet":{"intent_classification":{"category":"/query","verb":"/explain","target":"x","constraint":"none","confidence":1}},"surface_response":"hi"}` + strings.Repeat("}", 1000)
	
	// The scanner may or may not find it depending on how it handles depth. 
	// The goal is just to ensure it doesn't hang/OOM.
	_, _ = rp.extractEmbeddedJSON(raw)
}

func TestResponseProcessor_Process_TypeCoercionResilience(t *testing.T) {
	rp := NewResponseProcessor()
	rp.RequireValidJSON = false
	
	raw := `{"control_packet":{"intent_classification":{"category":"/query","verb":"/explain","target":"x","constraint":"none","confidence":"0.95"}},"surface_response":"hi"}`
	
	res, err := rp.Process(raw)
	if err != nil {
		t.Fatalf("Expected process to succeed, got: %v", err)
	}
	if res.ParseMethod != "json" {
		t.Fatalf("Expected json parsing for string-to-float coercion, got %s", res.ParseMethod)
	}
	if res.Control.IntentClassification.Confidence != 0.95 {
		t.Fatalf("Expected coerced confidence 0.95, got %v", res.Control.IntentClassification.Confidence)
	}
}

func TestResponseProcessor_StrictSchemaUnknownFields(t *testing.T) {
	rp := NewResponseProcessor()
	rp.RequireValidJSON = true

	// Strict mode with unknown fields inside control_packet
	raw := `{
	  "control_packet": {
	    "intent_classification": {
	      "category": "/query",
	      "verb": "/explain"
	    },
	    "hacker_injected_field": "exploit"
	  },
	  "surface_response": "hello"
	}`

	_, err := rp.Process(raw)
	if err == nil {
		t.Fatal("Expected strict mode to fail with unknown fields, but it succeeded")
	}
	if !strings.Contains(err.Error(), "unknown field") {
		t.Errorf("Expected error to mention unknown field, got: %v", err)
	}
}
