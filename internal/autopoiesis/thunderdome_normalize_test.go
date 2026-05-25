package autopoiesis

import (
	"testing"
)

func TestThunderdome_NormalizePackage(t *testing.T) {
	td := NewThunderdome()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no package",
			input:    "func main() {}",
			expected: "package tools\n\nfunc main() {}",
		},
		{
			name:     "package main",
			input:    "package main\n\nfunc main() {}",
			expected: "package tools\n\nfunc main() {}",
		},
		{
			name:     "package tools",
			input:    "package tools\n\nfunc main() {}",
			expected: "package tools\n\nfunc main() {}",
		},
		{
			name:     "package with comments",
			input:    "// some comment\npackage main\n\nfunc main() {}",
			expected: "// some comment\npackage tools\n\nfunc main() {}",
		},
		{
			name:     "multiline string with package",
			input:    "package main\n\nvar x = `\npackage main\n`",
			expected: "package tools\n\nvar x = `\npackage main\n`",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := td.normalizePackage(tt.input)
			if result != tt.expected {
				t.Errorf("Expected:\n%s\nGot:\n%s", tt.expected, result)
			}
		})
	}
}
