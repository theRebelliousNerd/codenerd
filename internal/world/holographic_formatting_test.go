package world

import (
	"testing"
)

func TestCountTODOs(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected int
	}{
		{
			name:     "empty string",
			content:  "",
			expected: 0,
		},
		{
			name:     "no todos",
			content:  `func main() { fmt.Println("Hello World") }`,
			expected: 0,
		},
		{
			name:     "single TODO",
			content:  "// TODO: implement this",
			expected: 1,
		},
		{
			name:     "single FIXME without colon",
			content:  "// FIXME implement this",
			expected: 1,
		},
		{
			name:     "multiple different tags",
			content:  "// TODO: one\n// FIXME: two\n// HACK: three\n// XXX: four\n// BUG: five",
			expected: 5,
		},
		{
			name:     "case insensitivity",
			content:  "// todo: one\n// fixme two\n// Hack three",
			expected: 3,
		},
		{
			name:     "inline within code",
			content:  "func foo() { /* TODO: refactor */ return 0 } // BUG here",
			expected: 2,
		},
		{
			name:     "mixed with other text",
			content:  "TODAY is a good day, but TODO: make it better. The bug is here but BUG: fix it",
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CountTODOs(tt.content)
			if result != tt.expected {
				t.Errorf("CountTODOs() = %v, want %v", result, tt.expected)
			}
		})
	}
}
