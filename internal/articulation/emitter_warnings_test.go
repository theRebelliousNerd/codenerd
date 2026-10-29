package articulation

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestProcessLLMResponseAllowPlain_MangleUpdateWarnings feeds an envelope with
// one valid atom, one missing its trailing period, and one containing a shell
// metacharacter. The valid atom must survive; both drops must be reported in
// Warnings with the reason named. No LLM is involved.
//
// Note: internal/logging offers no capture hook or test writer (Get returns a
// no-op logger when no logs dir is configured), so log output is not asserted
// here — only the surviving atom and the Warnings entries. The WARN logging
// added in applyCaps mirrors each of these Warnings.
func TestProcessLLMResponseAllowPlain_MangleUpdateWarnings(t *testing.T) {
	valid := "task_done(x)."
	noPeriod := `checkpoint_verdict("Retrieval Scaffold Inventory", /pass)`
	withDollar := "inject($PATH)."

	envelope := map[string]any{
		"control_packet": map[string]any{
			"intent_classification": map[string]any{
				"category":   "/query",
				"verb":       "/explain",
				"target":     "x",
				"constraint": "none",
				"confidence": 0.9,
			},
			"mangle_updates":    []string{valid, noPeriod, withDollar},
			"memory_operations": []any{},
		},
		"surface_response": "hello",
	}
	raw, err := json.Marshal(envelope)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	processed := ProcessLLMResponseAllowPlain(string(raw))
	if processed == nil {
		t.Fatal("ProcessLLMResponseAllowPlain() returned nil")
	}
	if processed.Control == nil {
		t.Fatalf("Control is nil, ParseMethod=%q Surface=%q", processed.ParseMethod, processed.Surface)
	}
	if len(processed.Control.MangleUpdates) != 1 {
		t.Fatalf("MangleUpdates = %v, want exactly [%q]", processed.Control.MangleUpdates, valid)
	}
	if processed.Control.MangleUpdates[0] != valid {
		t.Fatalf("MangleUpdates[0] = %q, want %q", processed.Control.MangleUpdates[0], valid)
	}
	if len(processed.Warnings) != 2 {
		t.Fatalf("Warnings = %v, want 2 entries", processed.Warnings)
	}
	joined := strings.Join(processed.Warnings, "\n")
	if !strings.Contains(joined, "Invalid Mangle update syntax") {
		t.Errorf("expected a warning naming the syntax reason, got %v", processed.Warnings)
	}
	if !strings.Contains(joined, "shell metacharacters") {
		t.Errorf("expected a warning naming the shell-metacharacter reason, got %v", processed.Warnings)
	}
}
