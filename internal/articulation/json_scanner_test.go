package articulation

import (
	"strings"
	"testing"
)

func TestFindJSONCandidates(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "simple",
			input: `prefix {"key": "value"} suffix`,
			want:  []string{`{"key": "value"}`},
		},
		{
			name:  "nested",
			input: `start {"a": {"b": "c"}} end`,
			want:  []string{`{"a": {"b": "c"}}`},
		},
		{
			name:  "multiple",
			input: `obj1 {"id": 1} obj2 {"id": 2}`,
			want:  []string{`{"id": 1}`, `{"id": 2}`},
		},
		{
			name:  "string_with_braces",
			input: `{"key": "value with } inside"}`,
			want:  []string{`{"key": "value with } inside"}`},
		},
		{
			name:  "escaped_quote",
			input: `{"key": "value with \" inside"}`,
			want:  []string{`{"key": "value with \" inside"}`},
		},
		{
			name:  "incomplete",
			input: `prefix { incomplete`,
			want:  nil,
		},
		{
			name:  "malformed_braces",
			input: `} { valid } {`,
			want:  []string{`{ valid }`},
		},
		{
			name:  "escaped_backslash",
			input: `{"key": "value with \\ inside"}`,
			want:  []string{`{"key": "value with \\ inside"}`},
		},
		{
			name:  "empty_object",
			input: `{}`,
			want:  []string{`{}`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findJSONCandidates(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("got %d candidates, want %d", len(got), len(tt.want))
			}
			for i, cand := range got {
				if cand != tt.want[i] {
					t.Errorf("candidate[%d] = %q, want %q", i, cand, tt.want[i])
				}
			}
		})
	}
}

// BenchmarkFindJSONCandidates benchmarks the scanner performance on a large input.
func BenchmarkFindJSONCandidates(b *testing.B) {
	// Create a large input (similar to generateLargeInput)
	var sb strings.Builder
	sb.WriteString("Pre-amble text with some random content...\n")
	sb.WriteString(`{
		"control_packet": {
			"intent_classification": {
				"category": "/code",
				"verb": "/implement",
				"target": "feature",
				"constraint": "none",
				"confidence": 0.95
			},
			"mangle_updates": [`)
	for i := 0; i < 2000; i++ {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(`"update_fact(`)
		sb.WriteString("some_argument")
		sb.WriteString(`)."`)
	}
	sb.WriteString(`], "memory_operations": [] }, "surface_response": "This is the response content..."}`)
	sb.WriteString("\nPost-amble text with more content...")
	input := sb.String()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		candidates := findJSONCandidates(input)
		if len(candidates) == 0 {
			b.Fatal("no candidates found")
		}
	}
}
