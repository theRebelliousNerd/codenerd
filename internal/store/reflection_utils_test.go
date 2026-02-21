package store

import (
	"reflect"
	"sort"
	"testing"
)

func TestExtractFileHints(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		max      int
		expected []string
	}{
		{
			name:     "Simple file path",
			input:    "Check internal/store/reflection_utils.go for details.",
			max:      5,
			expected: []string{"internal/store/reflection_utils.go"},
		},
		{
			name:     "Multiple file paths",
			input:    "Files: main.go, internal/config/config.yaml, README.md",
			max:      5,
			expected: []string{"main.go", "internal/config/config.yaml", "README.md"},
		},
		{
			name:     "Limit max hints",
			input:    "a.go b.go c.go d.go e.go f.go",
			max:      3,
			expected: []string{"a.go", "b.go", "c.go"},
		},
		{
			name:     "No file paths",
			input:    "This is just some text with no files.",
			max:      5,
			expected: nil,
		},
		{
			name:     "False positives (URLs, etc)",
			input:    "Check http://google.com/search or user@example.com",
			max:      5,
			expected: nil, // Should hopefully not match URLs as files if regex is strict enough
		},
		{
			name:     "Dashed filenames",
			input:    "my-file-name.ts",
			max:      5,
			expected: []string{"my-file-name.ts"},
		},
		{
			name:     "Relative paths",
			input:    "./cmd/nerd/main.go",
			max:      5,
			expected: []string{"cmd/nerd/main.go"}, // Leading ./ is skipped by \b
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := extractFileHints(tc.input, tc.max)
			sort.Strings(got)
			sort.Strings(tc.expected)

			if !reflect.DeepEqual(got, tc.expected) {
				t.Errorf("extractFileHints(%q, %d) = %v; want %v", tc.input, tc.max, got, tc.expected)
			}
		})
	}
}
