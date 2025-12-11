package tools

import (
	"context"
	"errors"
	"testing"
)

func TestStringReverser(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    string
		expectError bool
		errorMsg    string
	}{
		// Happy path tests
		{
			name:        "Simple ASCII string",
			input:       "hello",
			expected:    "olleh",
			expectError: false,
		},
		{
			name:        "String with numbers",
			input:       "12345",
			expected:    "54321",
			expectError: false,
		},
		{
			name:        "String with special characters",
			input:       "a!b@c#",
			expected:    "#c@b!a",
			expectError: false,
		},
		{
			name:        "String with mixed case",
			input:       "GoLang",
			expected:    "gnaLoG",
			expectError: false,
		},
		{
			name:        "String with spaces",
			input:       "hello world",
			expected:    "dlrow olleh",
			expectError: false,
		},

		// Edge case tests
		{
			name:        "Single character",
			input:       "a",
			expected:    "a",
			expectError: false,
		},
		{
			name:        "Two characters",
			input:       "ab",
			expected:    "ba",
			expectError: false,
		},
		{
			name:        "Unicode characters (emoji)",
			input:       "🙂🙃",
			expected:    "🙃🙂",
			expectError: false,
		},
		{
			name:        "Unicode characters (Chinese)",
			input:       "你好",
			expected:    "好你",
			expectError: false,
		},
		{
			name:        "Mixed ASCII and Unicode",
			input:       "a🙂b",
			expected:    "b🙂a",
			expectError: false,
		},

		// Error case tests
		{
			name:        "Empty string",
			input:       "",
			expected:    "",
			expectError: true,
			errorMsg:    "input string cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			result, err := stringReverser(ctx, tt.input)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected an error, but got none")
				} else if !errors.Is(err, errors.New(tt.errorMsg)) {
					// Using errors.Is is tricky with new errors, so we compare the string.
					if err.Error() != tt.errorMsg {
						t.Errorf("Expected error message '%s', but got '%s'", tt.errorMsg, err.Error())
					}
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("Expected result '%s', but got '%s'", tt.expected, result)
				}
			}
		})
	}
}

func TestStringReverser_ContextCancellation(t *testing.T) {
	// Create a context that is already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := stringReverser(ctx, "some input")
	if err == nil {
		t.Error("Expected an error due to context cancellation, but got none")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context.Canceled error, but got %v", err)
	}
}