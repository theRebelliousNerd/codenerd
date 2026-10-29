package synth

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDecodeSpec(t *testing.T) {
	validSpecJSON := `{"format":"mangle_synth_v1","program":{"clauses":[{"head":{"pred":"next_action","args":[{"kind":"name","value":"/run"}]}}]}}`

	piggybackMap := map[string]any{
		"surface_response": validSpecJSON,
	}
	piggybackPayload, _ := json.Marshal(piggybackMap)

	// Since decoding checks for top level valid JSON if missing, and Piggyback might have control packet info.
	// We want to test the case: `embedded, ok := findJSONObject(surface)` succeeds inside extractPiggybackSurface fallback path.
	// BUT to get there, `extractJSONPayload(raw)` must return an error OR extract something that fails decodeSpecPayload.
	// AND `extractPiggybackSurface(payload)` must extract surface, BUT `decodeSpecPayload(surface)` must fail,
	// AND `findJSONObject(surface)` must extract valid JSON.

	// How to construct this payload?
	// 1. `raw` is a valid piggyback envelope (so extractJSONPayload returns the envelope).
	// 2. `decodeSpecPayload(envelope)` fails (unknown field "surface_response").
	// 3. `extractPiggybackSurface(envelope)` extracts the `surface_response` content.
	// 4. `decodeSpecPayload(surface_response_content)` fails.
	// 5. `findJSONObject(surface_response_content)` returns valid spec JSON.

	// Envelope:
	// {"surface_response": "Some unstructured text before JSON {\"format\":\"mangle_synth_v1\",\"program\":{\"clauses\":[{\"head\":{\"pred\":\"next_action\",\"args\":[{\"kind\":\"name\",\"value\":\"/run\"}]}}]}}"}

	embeddedPiggybackMap := map[string]any{
		"surface_response": "Here is the plan:\n" + validSpecJSON + "\nHope that helps!",
	}
	embeddedPiggybackPayload, _ := json.Marshal(embeddedPiggybackMap)

	tests := []struct {
		name        string
		input       string
		expectError bool
		errString   string
		verifySpec  func(*testing.T, *Spec)
	}{
		{
			name:        "Empty Response",
			input:       "   \n\t  ",
			expectError: true,
			errString:   ErrEmptyResponse.Error(),
		},
		{
			name:        "Valid JSON",
			input:       validSpecJSON,
			expectError: false,
			verifySpec: func(t *testing.T, s *Spec) {
				if s.Format != FormatV1 {
					t.Errorf("expected format %s, got %s", FormatV1, s.Format)
				}
			},
		},
		{
			name:        "Markdown Wrapped JSON",
			input:       "```json\n" + validSpecJSON + "\n```",
			expectError: false,
		},
		{
			name:        "Markdown Wrapped JSON No Newline",
			input:       "```" + validSpecJSON + "```",
			expectError: false,
		},
		{
			name:        "Invalid JSON",
			input:       "invalid json string that is not an object",
			expectError: true,
			errString:   ErrMissingJSON.Error(),
		},
		{
			name:        "Piggyback Envelope Direct",
			input:       string(piggybackPayload),
			expectError: false,
		},
		{
			name:        "Piggyback Envelope Embedded",
			input:       string(embeddedPiggybackPayload),
			expectError: false,
		},
		{
			name:        "Trailing Data",
			input:       `{"format": "mangle_synth_v1", "program": {"clauses": []}}`,
			expectError: false,
		},
		{
			name:        "Invalid Piggyback Envelope Content",
			input:       `{"surface_response": []}`,
			expectError: true,
		},
		{
			name:        "Invalid JSON Object Inside findJSONObject",
			input:       `{"format":"mangle_synth_v1",`,
			expectError: true,
			errString:   ErrMissingJSON.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec, err := DecodeSpec(tt.input)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected an error but got none")
				} else if tt.errString != "" && !strings.Contains(err.Error(), tt.errString) {
					t.Errorf("expected error to contain %q, got %q", tt.errString, err)
				}
			} else {
				if err != nil {
					t.Errorf("did not expect error, got: %v", err)
				}
				if tt.verifySpec != nil {
					tt.verifySpec(t, &spec)
				}
			}
		})
	}
}

func TestEnsureEOF(t *testing.T) {
	// Directly test ensureEOF using decodeSpecPayload
	_, err := decodeSpecPayload(`{"format":"mangle_synth_v1","program":{"clauses":[]}} []`)
	if err == nil || !strings.Contains(err.Error(), "unexpected trailing JSON content") {
		t.Errorf("expected unexpected trailing JSON content error, got: %v", err)
	}
}

func TestFindJSONObject(t *testing.T) {
	input := `some text before {"key": "value", "nested": {"a": 1}} some text after`
	obj, ok := findJSONObject(input)
	if !ok {
		t.Errorf("expected to find JSON object")
	}
	if obj != `{"key": "value", "nested": {"a": 1}}` {
		t.Errorf("extracted wrong object: %s", obj)
	}

	// String containing braces
	inputStringBrace := `{"key": "value { inside } string"}`
	objStr, okStr := findJSONObject(inputStringBrace)
	if !okStr || objStr != inputStringBrace {
		t.Errorf("failed to handle braces inside strings: %s", objStr)
	}

	// Escaped quote
	inputEscapedQuote := `{"key": "value \" escaped"}`
	objEsc, okEsc := findJSONObject(inputEscapedQuote)
	if !okEsc || objEsc != inputEscapedQuote {
		t.Errorf("failed to handle escaped quotes: %s", objEsc)
	}

	// findJSONObject incomplete
	inputIncomplete := `{"key": "value"`
	_, okInc := findJSONObject(inputIncomplete)
	if okInc {
		t.Errorf("expected to fail finding JSON object for incomplete input")
	}
}

func TestExtractJSONPayload(t *testing.T) {
	// Code coverage for branches in extractJSONPayload not covered by standard flows.
	// Empty response
	_, err := extractJSONPayload("  ")
	if err != ErrEmptyResponse {
		t.Errorf("expected ErrEmptyResponse")
	}

	// Markdown block with no valid JSON inside
	_, err = extractJSONPayload("```json\nnot json\n```")
	if err != ErrMissingJSON {
		t.Errorf("expected ErrMissingJSON")
	}

	// Plain text no json
	_, err = extractJSONPayload("not json at all")
	if err != ErrMissingJSON {
		t.Errorf("expected ErrMissingJSON")
	}
}

func TestExtractPiggybackSurface(t *testing.T) {
	// Invalid envelope
	_, ok := extractPiggybackSurface(`{"surface_response": {]}`)
	if ok {
		t.Errorf("expected failure on invalid envelope")
	}

	// Missing surface
	_, ok = extractPiggybackSurface(`{"other_field": "value"}`)
	if ok {
		t.Errorf("expected failure on missing surface")
	}

	// empty surface raw string
	_, ok = extractPiggybackSurface(`{"surface_response": ""}`)
	if ok {
		t.Errorf("expected failure on empty surface")
	}
}

func TestDecodeSpecFromPiggybackSurface(t *testing.T) {
	specJSON := `{"format":"mangle_synth_v1","program":{"clauses":[{"head":{"pred":"next_action","args":[{"kind":"name","value":"/run"}]}}]}}`

	envelope := map[string]any{
		"control_packet": map[string]any{
			"intent_classification": map[string]any{
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
