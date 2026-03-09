package prompt

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMinifyWhitespace_Extended(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no change",
			input:    "Hello world",
			expected: "Hello world",
		},
		{
			name:     "single newline",
			input:    "Hello\nworld",
			expected: "Hello\nworld",
		},
		{
			name:     "double newline",
			input:    "Hello\n\nworld",
			expected: "Hello\n\nworld",
		},
		{
			name:     "triple newline",
			input:    "Hello\n\n\nworld",
			expected: "Hello\n\nworld",
		},
		{
			name:     "many newlines",
			input:    "Hello\n\n\n\n\n\nworld",
			expected: "Hello\n\nworld",
		},
		{
			name:     "trailing spaces on lines",
			input:    "line1  \nline2\t\nline3 \t ",
			expected: "line1\nline2\nline3",
		},
		{
			name:     "newlines with spaces between them",
			input:    "line1\n  \n\n\nline2",
			expected: "line1\n\n\nline2",
			// Trace:
			// 1. \n\n\n found? No (it's \n  \n\n\n). Wait. \n\n\n is at the end.
			// 2. ReplaceAll(\n\n\n, \n\n) -> line1\n  \n\nline2
			// 3. \n\n\n found? No.
			// 4. Split -> ["line1", "  ", "", "line2"]
			// 5. TrimRight -> ["line1", "", "", "line2"]
			// 6. Join -> line1\n\n\nline2
		},
		{
			name:     "mixed spaces and newlines",
			input:    "a\n \n \n \nb",
			expected: "a\n\n\n\nb",
			// Trace:
			// 1. \n\n\n found? No.
			// 2. Split -> ["a", " ", " ", " ", "b"]
			// 3. TrimRight -> ["a", "", "", "", "b"]
			// 4. Join -> a\n\n\n\nb
		},
		{
			name:     "leading and trailing newlines",
			input:    "\n\n\nstart\n\n\nend\n\n\n",
			expected: "\n\nstart\n\nend\n\n",
		},
		{
			name:     "tabs as trailing whitespace",
			input:    "text\t\t\nmore text\t",
			expected: "text\nmore text",
		},
		{
			name:     "UTF-8 content",
			input:    "こんにちは  \n世界  \n\n\n",
			expected: "こんにちは\n世界\n\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := minifyWhitespace(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func BenchmarkMinifyWhitespace_Large(b *testing.B) {
	// Create a large input: 1000 lines, each with trailing spaces and many newlines
	var sb strings.Builder
	for i := 0; i < 1000; i++ {
		sb.WriteString("This is a line with trailing spaces    ")
		sb.WriteString("\n")
		if i%10 == 0 {
			sb.WriteString("\n\n\n\n\n")
		} else {
			sb.WriteString("\n")
		}
	}
	input := sb.String()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		minifyWhitespace(input)
	}
}
