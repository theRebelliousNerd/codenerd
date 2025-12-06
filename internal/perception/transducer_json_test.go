package perception

import (
	"codenerd/internal/articulation"
	"testing"
)

func TestParsePiggybackJSON_Robustness(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name: "Clean JSON",
			input: `{"surface_response": "Hello", "control_packet": {}}`,
			wantErr: false,
		},
		{
			name: "Markdown Wrapped",
			input: "```json\n" + `{"surface_response": "Hello", "control_packet": {}}` + "\n```",
			wantErr: false,
		},
		{
			name: "Prefix Text",
			input: `Here is the JSON: {"surface_response": "Hello", "control_packet": {}}`,
			wantErr: false,
		},
		{
			name: "Suffix Text",
			input: `{"surface_response": "Hello", "control_packet": {}} And some text after`,
			wantErr: false,
		},
		{
			name: "Surrounded Text",
			input: `Prefix {"surface_response": "Hello", "control_packet": {}} Suffix`,
			wantErr: false,
		},
		{
			name: "Nested Braces",
			input: `Prefix {"surface_response": "Value {1}", "control_packet": {"mangle_updates": ["a(1)"]}} Suffix`,
			wantErr: false,
		},
		{
			name: "Invalid JSON",
			input: `{"surface_response": "Hello", "control_packet":`,
			wantErr: true,
		},
		{
			name: "No JSON Object",
			input: `Just some text`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			envelope, err := parsePiggybackJSON(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parsePiggybackJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if envelope.Surface != "Hello" && envelope.Surface != "Value {1}" {
					t.Errorf("Unexpected parsed content: %+v", envelope)
				}
			}
		})
	}
}

// Mock types to match articulation package if not imported correctly
// (But we imported articulation, so this is fine)
var _ articulation.PiggybackEnvelope
