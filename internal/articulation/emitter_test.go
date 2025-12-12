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
