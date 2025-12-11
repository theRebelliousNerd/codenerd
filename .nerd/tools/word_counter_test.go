package tools

import (
	"context"
	"testing"
	"time"
)

func TestWordCounter(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		ctx         context.Context
		expected    string
		expectError bool
	}{
		// Happy Path Tests
		{
			name:     "Simple sentence",
			input:    "Hello world",
			ctx:      context.Background(),
			expected: "2",
		},
		{
			name:     "Sentence with punctuation",
			input:    "Go is a great, powerful language!",
			ctx:      context.Background(),
			expected: "6",
		},
		{
			name:     "Multiple spaces between words",
			input:    "a   b    c",
			ctx:      context.Background(),
			expected: "3",
		},
		{
			name:     "Words with numbers",
			input:    "The year is 2024",
			ctx:      context.Background(),
			expected: "4",
		},
		{
			name:     "Alphanumeric words",
			input:    "test123 abc456",
			ctx:      context.Background(),
			expected: "2",
		},
		{
			name:     "Leading and trailing whitespace",
			input:    "  hello world  ",
			ctx:      context.Background(),
			expected: "2",
		},
		{
			name:     "Newline and tab characters",
			input:    "one\ttwo\nthree",
			ctx:      context.Background(),
			expected: "3",
		},

		// Edge Case Tests
		{
			name:     "Empty string",
			input:    "",
			ctx:      context.Background(),
			expected: "0",
		},
		{
			name:     "String with only whitespace",
			input:    " \t\n ",
			ctx:      context.Background(),
			expected: "0",
		},
		{
			name:     "String with only punctuation",
			input:    "!@#$%^&*()",
			ctx:      context.Background(),
			expected: "0",
		},
		{
			name:     "Single word",
			input:    "word",
			ctx:      context.Background(),
			expected: "1",
		},
		{
			name:     "Single character",
			input:    "a",
			ctx:      context.Background(),
			expected: "1",
		},
		{
			name:     "Single number",
			input:    "5",
			ctx:      context.Background(),
			expected: "1",
		},
		{
			name:     "Unicode letters",
			input:    "héllo wörld",
			ctx:      context.Background(),
			expected: "2",
		},
		{
			name:     "Mixed alphanumeric and punctuation",
			input:    "go-version-1.21 is here!",
			ctx:      context.Background(),
			expected: "4",
		},

		// Error Case Tests
		{
			name:        "Cancelled context before execution",
			input:       "some text",
			ctx:         canceledContext(),
			expectError: true,
		},
		{
			name:        "Cancelled context during execution",
			input:       generateLongString(10000),
			ctx:         contextWithTimeout(1 * time.Nanosecond),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := wordCounter(tt.ctx, tt.input)

			if tt.expectError {
				if err == nil {
					t.Errorf("wordCounter() expected error, got none")
				}
				return
			}

			if err != nil {
				t.Errorf("wordCounter() unexpected error = %v", err)
				return
			}

			if got != tt.expected {
				t.Errorf("wordCounter() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// Helper function to create a canceled context for error testing.
func canceledContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

// Helper function to create a context with a timeout for error testing.
func contextWithTimeout(d time.Duration) context.Context {
	ctx, _ := context.WithTimeout(context.Background(), d)
	return ctx
}

// Helper function to generate a long string to test context cancellation during processing.
func generateLongString(n int) string {
	var sb strings.Builder
	for i := 0; i < n; i++ {
		sb.WriteString("word ")
	}
	return sb.String()
}