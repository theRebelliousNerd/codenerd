package articulation

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestProcessLLMResponse_DropsPlaceholderSurface ensures parroted schema
// example text is never surfaced to the user. Both historical placeholders
// and the new angle-bracket form must yield Surface "" plus a warning so the
// existing empty-surface handling and the executor's hollow logic apply.
func TestProcessLLMResponse_DropsPlaceholderSurface(t *testing.T) {
	placeholders := []string{
		"Human-readable response to the user",
		"I cannot delete that directory because it's protected by the kernel's safety rules. If you need to delete it, you'll need to do so manually or grant explicit permission with /override.",
		"<SURFACE_RESPONSE: your reply to the user, plain language, never this placeholder>",
		"<SURFACE_RESPONSE>",
	}

	buildRaw := func(surface string) string {
		envelope := map[string]any{
			"control_packet": map[string]any{
				"intent_classification": map[string]any{
					"category":   "/query",
					"verb":       "/explain",
					"target":     "x",
					"constraint": "none",
					"confidence": 0.9,
				},
				"mangle_updates":    []string{},
				"memory_operations": []any{},
			},
			"surface_response": surface,
		}
		raw, err := json.Marshal(envelope)
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}
		return string(raw)
	}

	for _, surface := range placeholders {
		raw := buildRaw(surface)

		for _, tc := range []struct {
			name string
			fn   func(string) *ProcessedLLMResponse
		}{
			{"ProcessLLMResponse", ProcessLLMResponse},
			{"ProcessLLMResponseAllowPlain", ProcessLLMResponseAllowPlain},
		} {
			processed := tc.fn(raw)
			if processed == nil {
				t.Fatalf("%s(%q) returned nil", tc.name, surface)
			}
			if processed.Surface != "" {
				t.Errorf("%s(%q): Surface = %q, want empty string", tc.name, surface, processed.Surface)
			}
			joined := strings.Join(processed.Warnings, "\n")
			if !strings.Contains(joined, "placeholder surface_response dropped") {
				t.Errorf("%s(%q): Warnings = %v, want placeholder surface_response dropped", tc.name, surface, processed.Warnings)
			}
		}
	}
}

// TestProcessLLMResponse_DropsParrotedFeedback ensures the schema's example
// feedback values are never learned from. When helpful_facts equals the
// historical example set or missing_context carries the angle-bracket
// placeholder, ContextFeedback must be nilled with a warning.
func TestProcessLLMResponse_DropsParrotedFeedback(t *testing.T) {
	buildRaw := func(fb map[string]any) string {
		envelope := map[string]any{
			"control_packet": map[string]any{
				"intent_classification": map[string]any{
					"category":   "/query",
					"verb":       "/explain",
					"target":     "x",
					"constraint": "none",
					"confidence": 0.9,
				},
				"mangle_updates":    []string{},
				"memory_operations": []any{},
				"context_feedback":  fb,
			},
			"surface_response": "a real reply that must survive",
		}
		raw, err := json.Marshal(envelope)
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}
		return string(raw)
	}

	cases := []struct {
		name string
		fb   map[string]any
	}{
		{
			name: "example helpful facts",
			fb: map[string]any{
				"overall_usefulness": 0.8,
				"helpful_facts":      []string{"file_topology", "test_state"},
				"noise_facts":        []string{"dom_node"},
				"missing_context":    "",
			},
		},
		{
			name: "angle bracket missing context",
			fb: map[string]any{
				"overall_usefulness": 0.0,
				"helpful_facts":      []string{"symbol_graph"},
				"noise_facts":        []any{},
				"missing_context":    "<MISSING_CONTEXT: what would have helped, or empty string>",
			},
		},
	}

	for _, tt := range cases {
		raw := buildRaw(tt.fb)
		for _, fn := range []struct {
			name string
			call func(string) *ProcessedLLMResponse
		}{
			{"ProcessLLMResponse", ProcessLLMResponse},
			{"ProcessLLMResponseAllowPlain", ProcessLLMResponseAllowPlain},
		} {
			processed := fn.call(raw)
			if processed == nil {
				t.Fatalf("%s/%s returned nil", tt.name, fn.name)
			}
			if processed.Control == nil {
				t.Fatalf("%s/%s: Control is nil, want non-nil with nilled feedback", tt.name, fn.name)
			}
			if processed.Control.ContextFeedback != nil {
				t.Errorf("%s/%s: ContextFeedback = %+v, want nil", tt.name, fn.name, processed.Control.ContextFeedback)
			}
			joined := strings.Join(processed.Warnings, "\n")
			if !strings.Contains(joined, "placeholder context_feedback dropped") {
				t.Errorf("%s/%s: Warnings = %v, want placeholder context_feedback dropped", tt.name, fn.name, processed.Warnings)
			}
			if processed.Surface != "a real reply that must survive" {
				t.Errorf("%s/%s: Surface = %q, want real reply to survive", tt.name, fn.name, processed.Surface)
			}
		}
	}
}

// TestProcessLLMResponse_KeepsRealSurface ensures a genuine reply passes
// through untouched, with no placeholder warnings.
func TestProcessLLMResponse_KeepsRealSurface(t *testing.T) {
	envelope := map[string]any{
		"control_packet": map[string]any{
			"intent_classification": map[string]any{
				"category":   "/query",
				"verb":       "/explain",
				"target":     "x",
				"constraint": "none",
				"confidence": 0.9,
			},
			"mangle_updates":    []string{},
			"memory_operations": []any{},
			"context_feedback": map[string]any{
				"overall_usefulness": 0.85,
				"helpful_facts":      []string{"symbol_graph"},
				"noise_facts":        []string{"browser_state"},
				"missing_context":    "needed the call graph for foo",
			},
		},
		"surface_response": "Implemented validation with email checks and tests.",
	}
	raw, err := json.Marshal(envelope)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	for _, tc := range []struct {
		name string
		fn   func(string) *ProcessedLLMResponse
	}{
		{"ProcessLLMResponse", ProcessLLMResponse},
		{"ProcessLLMResponseAllowPlain", ProcessLLMResponseAllowPlain},
	} {
		processed := tc.fn(string(raw))
		if processed == nil {
			t.Fatalf("%s returned nil", tc.name)
		}
		if processed.Surface != "Implemented validation with email checks and tests." {
			t.Errorf("%s: Surface = %q, want real sentence to survive", tc.name, processed.Surface)
		}
		for _, w := range processed.Warnings {
			if strings.Contains(w, "placeholder surface_response dropped") {
				t.Errorf("%s: unexpected placeholder surface warning: %v", tc.name, processed.Warnings)
			}
			if strings.Contains(w, "placeholder context_feedback dropped") {
				t.Errorf("%s: unexpected placeholder feedback warning: %v", tc.name, processed.Warnings)
			}
		}
		if processed.Control == nil || processed.Control.ContextFeedback == nil {
			t.Fatalf("%s: expected real ContextFeedback to survive, got %+v", tc.name, processed.Control)
		}
	}
}
