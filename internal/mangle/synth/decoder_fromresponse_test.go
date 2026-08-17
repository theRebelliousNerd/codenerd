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

	invalidSpecJSON := `{"format":"mangle_synth_v2","program":{"clauses":[{"head":{"pred":"next_action","args":[{"kind":"name","value":"/run"}]}}]}}`

	tests := []struct {
		name        string
		input       string
		options     Options
		expectError bool
		errContains string
		verifyRes   func(*testing.T, Result)
	}{
		{
			name:        "Piggyback Envelope Success",
			input:       string(payload),
			options:     DefaultOptions(),
			expectError: false,
			verifyRes: func(t *testing.T, res Result) {
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
			name:        "Raw Valid JSON Success",
			input:       specJSON,
			options:     DefaultOptions(),
			expectError: false,
			verifyRes: func(t *testing.T, res Result) {
				if len(res.Clauses) != 1 {
					t.Fatalf("expected 1 compiled clause, got %d (%v)", len(res.Clauses), res.Clauses)
				}
				if !strings.Contains(res.Clauses[0], "next_action") || !strings.Contains(res.Clauses[0], "/run") {
					t.Errorf("compiled clause missing expected atom: %q", res.Clauses[0])
				}
			},
		},
		{
			name:        "Invalid JSON",
			input:       "not json",
			options:     DefaultOptions(),
			expectError: true,
			errContains: ErrMissingJSON.Error(),
		},
		{
			name:        "Compile Failure",
			input:       invalidSpecJSON,
			options:     DefaultOptions(),
			expectError: true,
			errContains: "format: expected \"mangle_synth_v1\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := FromResponse(tt.input, tt.options)
			if tt.expectError {
				if err == nil {
					t.Errorf("expected an error but got none")
				} else if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("expected error containing %q, got: %v", tt.errContains, err)
				}
			} else {
				if err != nil {
					t.Fatalf("did not expect error, got: %v", err)
				}
				if tt.verifyRes != nil {
					tt.verifyRes(t, res)
				}
			}
		})
	}
}
