package store

import (
	"testing"
)

func TestSanitizeDescriptor(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "API Key",
			input:    "api_key: secretvalue123",
			expected: "api_key: [redacted]",
		},
		{
			name:     "API Key Equals",
			input:    "api_key=secretvalue123",
			expected: "api_key=[redacted]",
		},
		{
			name:     "Secret",
			input:    "secret: supersecret",
			expected: "secret: [redacted]",
		},
		{
			name:     "Token",
			input:    "token: abcdefg",
			expected: "token: [redacted]",
		},
		{
			name:     "Password",
			input:    "password: mypass",
			expected: "password: [redacted]",
		},
		{
			name:     "Bearer Token",
			input:    "Authorization: Bearer my-token-123",
			expected: "Authorization: Bearer [redacted]",
		},
		{
			name:     "AIza Key",
			input:    "AIzaSyB_1234567890abcdef",
			expected: "[redacted]",
		},
		{
			name:     "sk Key",
			input:    "sk-1234567890abcdef",
			expected: "[redacted]",
		},
		{
			name:     "ctx7sk Key",
			input:    "ctx7sk-12345678",
			expected: "[redacted]",
		},
		{
			name:     "Multiple Secrets",
			input:    "api_key: val1, secret=val2, Bearer token3",
			expected: "api_key: [redacted], secret=[redacted], Bearer [redacted]",
		},
		{
			name:     "No Secrets",
			input:    "Just some normal text here.",
			expected: "Just some normal text here.",
		},
		{
			name:     "Empty",
			input:    "",
			expected: "",
		},
		{
			name:     "Whitespace Only",
			input:    "   ",
			expected: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizeDescriptor(tc.input)
			if got != tc.expected {
				t.Errorf("sanitizeDescriptor(%q) = %q; want %q", tc.input, got, tc.expected)
			}
		})
	}
}

func BenchmarkSanitizeDescriptor(b *testing.B) {
	input := "Here is a log message with api_key: secret123 and also a Bearer token-xyz and maybe a sk-abcdef123456 inside. It also has some normal text context to make it longer and more realistic."
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sanitizeDescriptor(input)
	}
}
