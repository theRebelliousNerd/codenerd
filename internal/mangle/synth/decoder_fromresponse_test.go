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

	tests := []struct {
		name        string
		input       string
		expectError bool
		errContains string
		checkResult func(*testing.T, Result)
	}{
		{
			name:        "Piggyback Envelope",
			input:       string(payload),
			expectError: false,
			checkResult: func(t *testing.T, res Result) {
				if len(res.Clauses) != 1 {
					t.Fatalf("expected 1 compiled clause, got %d (%v)", len(res.Clauses), res.Clauses)
				}
				if !strings.Contains(res.Clauses[0], "next_action") || !strings.Contains(res.Clauses[0], "/run") {
					t.Errorf("compiled clause missing expected atom: %q", res.Clauses[0])
				}
				if strings.TrimSpace(res.Source) == "" {
					t.Error("expected non-empty compiled Source")
				}
			},
		},
		{
			name:        "Basic JSON Decoding",
			input:       specJSON,
			expectError: false,
			checkResult: func(t *testing.T, res Result) {
				if len(res.Clauses) != 1 {
					t.Fatalf("expected 1 compiled clause, got %d (%v)", len(res.Clauses), res.Clauses)
				}
			},
		},
		{
			name:        "Decoding Error Delegation",
			input:       "not json",
			expectError: true,
			errContains: ErrMissingJSON.Error(),
		},
		{
			name:        "Rule Compilation Delegation Error",
			input:       `{"format":"mangle_synth_v1","program":{"package":{"name":"   "}}}`,
			expectError: true,
			errContains: "package name is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := FromResponse(tt.input, DefaultOptions())
			if tt.expectError {
				if err == nil {
					t.Errorf("expected an error, got nil")
				} else if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("expected error to contain %q, got %q", tt.errContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got: %v", err)
				}
				if tt.checkResult != nil {
					tt.checkResult(t, res)
				}
			}
		})
	}
}
