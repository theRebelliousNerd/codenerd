package synth

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestFromResponse(t *testing.T) {
	specJSON := `{"format":"mangle_synth_v1","program":{"clauses":[{"head":{"pred":"next_action","args":[{"kind":"name","value":"/run"}]}}]}}`
	envelope := map[string]any{
		"control_packet": map[string]any{
			"intent_classification": map[string]any{
				"category": "/instruction", "verb": "/generate", "target": "mangle", "confidence": 1.0,
			},
			"mangle_updates": []string{},
		},
		"surface_response": specJSON,
	}
	payload, err := json.Marshal(envelope)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// FromResponse should decode the embedded spec and compile it end-to-end.
	res, err := FromResponse(string(payload), DefaultOptions())
	if err != nil {
		t.Fatalf("FromResponse: %v", err)
	}
	if len(res.Clauses) != 1 {
		t.Fatalf("expected 1 compiled clause, got %d (%v)", len(res.Clauses), res.Clauses)
	}
	if !strings.Contains(res.Clauses[0], "next_action") || !strings.Contains(res.Clauses[0], "/run") {
		t.Errorf("compiled clause missing expected atom: %q", res.Clauses[0])
	}
	if strings.TrimSpace(res.Source) == "" {
		t.Error("expected non-empty compiled Source")
	}

	// Invalid JSON surfaces a decode error rather than panicking.
	if _, err := FromResponse("not json", DefaultOptions()); err == nil {
		t.Error("expected an error for non-JSON input")
	}
}
