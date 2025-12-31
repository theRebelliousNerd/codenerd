package synth

import (
	"encoding/json"
	"testing"
)

func TestDecodeSpecFromPiggybackSurface(t *testing.T) {
	specJSON := `{"format":"mangle_synth_v1","program":{"clauses":[{"head":{"pred":"next_action","args":[{"kind":"name","value":"/run"}]}}]}}`

	envelope := map[string]interface{}{
		"control_packet": map[string]interface{}{
			"intent_classification": map[string]interface{}{
				"category":   "/instruction",
				"verb":       "/generate",
				"target":     "mangle",
				"constraint": "",
				"confidence": 1.0,
			},
			"mangle_updates": []string{},
		},
		"surface_response": specJSON,
	}

	payload, err := json.Marshal(envelope)
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}

	spec, err := DecodeSpec(string(payload))
	if err != nil {
		t.Fatalf("DecodeSpec: %v", err)
	}
	if spec.Format != FormatV1 {
		t.Fatalf("unexpected format: %q", spec.Format)
	}
	if len(spec.Program.Clauses) != 1 {
		t.Fatalf("expected 1 clause, got %d", len(spec.Program.Clauses))
	}
	if spec.Program.Clauses[0].Head.Pred != "next_action" {
		t.Fatalf("unexpected head predicate: %q", spec.Program.Clauses[0].Head.Pred)
	}
}
