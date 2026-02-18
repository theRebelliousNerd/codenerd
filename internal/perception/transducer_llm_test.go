package perception

import (
	"strings"
	"testing"
)

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Simple JSON",
			input:    `{"key": "value"}`,
			expected: `{"key": "value"}`,
		},
		{
			name:     "With Preamble",
			input:    `Here is the JSON: {"key": "value"}`,
			expected: `{"key": "value"}`,
		},
		{
			name:     "With Postamble",
			input:    `{"key": "value"} is the JSON`,
			expected: `{"key": "value"}`,
		},
		{
			name:     "With Both",
			input:    `Start {"key": "value"} End`,
			expected: `{"key": "value"}`,
		},
		{
			name:     "Nested JSON",
			input:    `{"outer": {"inner": "value"}}`,
			expected: `{"outer": {"inner": "value"}}`,
		},
		{
			name:     "Multiple JSON objects - return last",
			input:    `{"first": 1} ... {"second": 2}`,
			expected: `{"second": 2}`,
		},
		{
			name:     "Valid inside Invalid",
			input:    `{ invalid json { "valid": "inside" } }`, // "valid" is inside invalid braces
			expected: `{ "valid": "inside" }`,
		},
		{
			name:     "Valid followed by Invalid",
			input:    `{"valid": 1} { invalid }`,
			expected: `{"valid": 1}`,
		},
		{
			name:     "Malformed JSON",
			input:    `{ "key": "value"`,
			expected: ``,
		},
		{
			name:     "Deeply Nested",
			input:    `{"a":{"b":{"c":{"d":1}}}}`,
			expected: `{"a":{"b":{"c":{"d":1}}}}`,
		},
		{
			name:     "Brace In String - Closing",
			input:    `{"a": "}"}`,
			expected: `{"a": "}"}`,
		},
		{
			name:     "Brace In String - Opening",
			input:    `{"a": "{"}`,
			expected: `{"a": "{"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractJSON(tt.input)
			if got != tt.expected {
				// Special handling for "Valid inside Invalid" case if behaviors differ,
				// but let's see what the current implementation does first.
				if tt.name == "Valid inside Invalid" {
					// Current implementation might return `{"valid": "inside"}`.
					// My implementation will return `{"valid": "inside"}`.
					// So they should match.
				}
				t.Errorf("extractJSON() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func BenchmarkExtractJSON(b *testing.B) {
	// Create a large input
	var sb strings.Builder
	sb.WriteString("Here is some text preamble.\n")
	for i := 0; i < 1000; i++ {
		sb.WriteString("Some noise { invalid } more noise.\n")
	}
	sb.WriteString(`{"final": "json", "data": [`)
	for i := 0; i < 1000; i++ {
		sb.WriteString(`{"id": 1},`)
	}
	sb.WriteString(`{"id": 2}]}`)
	sb.WriteString("\nAnd some trailing text.")
	input := sb.String()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		extractJSON(input)
	}
}

// TestNormalizeLLMFields_WhenMixedCase_ShouldLowercase verifies that
// LLM-generated field values are normalized to lowercase for Mangle vocabulary matching.
func TestNormalizeLLMFields_WhenMixedCase_ShouldLowercase(t *testing.T) {
	tests := []struct {
		name     string
		input    Understanding
		expected Understanding
	}{
		{
			name: "Mixed case fields",
			input: Understanding{
				SemanticType: "Code_Generation",
				ActionType:   "IMPLEMENT",
				Domain:       "Testing",
				Scope:        Scope{Level: "METHOD"},
				SuggestedApproach: SuggestedApproach{
					Mode: "NORMAL",
				},
			},
			expected: Understanding{
				SemanticType: "code_generation",
				ActionType:   "implement",
				Domain:       "testing",
				Scope:        Scope{Level: "method"},
				SuggestedApproach: SuggestedApproach{
					Mode: "normal",
				},
			},
		},
		{
			name: "Already lowercase",
			input: Understanding{
				SemanticType: "code_generation",
				ActionType:   "implement",
				Domain:       "testing",
			},
			expected: Understanding{
				SemanticType: "code_generation",
				ActionType:   "implement",
				Domain:       "testing",
			},
		},
		{
			name: "Empty fields preserved",
			input: Understanding{
				SemanticType: "Query",
				ActionType:   "",
				Domain:       "DATABASE",
				Scope:        Scope{Level: ""},
				SuggestedApproach: SuggestedApproach{
					Mode: "",
				},
			},
			expected: Understanding{
				SemanticType: "query",
				ActionType:   "",
				Domain:       "database",
				Scope:        Scope{Level: ""},
				SuggestedApproach: SuggestedApproach{
					Mode: "",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := tt.input
			normalizeLLMFields(&u)
			if u.SemanticType != tt.expected.SemanticType {
				t.Errorf("SemanticType = %q, want %q", u.SemanticType, tt.expected.SemanticType)
			}
			if u.ActionType != tt.expected.ActionType {
				t.Errorf("ActionType = %q, want %q", u.ActionType, tt.expected.ActionType)
			}
			if u.Domain != tt.expected.Domain {
				t.Errorf("Domain = %q, want %q", u.Domain, tt.expected.Domain)
			}
			if u.Scope.Level != tt.expected.Scope.Level {
				t.Errorf("Scope.Level = %q, want %q", u.Scope.Level, tt.expected.Scope.Level)
			}
			if u.SuggestedApproach.Mode != tt.expected.SuggestedApproach.Mode {
				t.Errorf("Mode = %q, want %q", u.SuggestedApproach.Mode, tt.expected.SuggestedApproach.Mode)
			}
		})
	}
}

// TestNormalizeLLMFields_WhenNil_ShouldNotPanic verifies nil safety.
func TestNormalizeLLMFields_WhenNil_ShouldNotPanic(t *testing.T) {
	// Should not panic
	normalizeLLMFields(nil)
}
