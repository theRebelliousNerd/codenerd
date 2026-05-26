package autopoiesis

import (
	"testing"
)

func TestNormalizePackage(t *testing.T) {
	td := NewThunderdome()

	cases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Normal package main",
			input:    "package main\n\nfunc main() {}",
			expected: "package tools\n\nfunc main() {}",
		},
		{
			name:     "Leading spaces",
			input:    "   package main\n\nfunc main() {}",
			expected: "   package tools\n\nfunc main() {}",
		},
		{
			name:     "Leading tabs",
			input:    "\tpackage main\n\nfunc main() {}",
			expected: "\tpackage tools\n\nfunc main() {}",
		},
		{
			name:     "Package already tools",
			input:    "package tools\n\nfunc main() {}",
			expected: "package tools\n\nfunc main() {}",
		},
		{
			name:     "No package declaration",
			input:    "func main() {}",
			expected: "package tools\n\nfunc main() {}",
		},
		{
			name:     "Comments before package",
			input:    "// some comment\npackage main\n\nfunc main() {}",
			expected: "// some comment\npackage tools\n\nfunc main() {}",
		},
		{
			name:     "String containing package main",
			input:    "package tools\n\nfunc main() {\n\t// package main\n}",
			expected: "package tools\n\nfunc main() {\n\t// package main\n}",
		},
		{
			name:     "String containing package main with different initial package",
			input:    "package main\n\nfunc main() {\n\t// package main\n}",
			expected: "package tools\n\nfunc main() {\n\t// package main\n}",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			actual := td.normalizePackage(tc.input)
			if actual != tc.expected {
				t.Errorf("Expected:\n%s\n\nGot:\n%s", tc.expected, actual)
			}
		})
	}
}
