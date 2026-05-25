package feedback

import (
	"testing"
)

func TestNormalizeRuleInput(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "trim whitespace",
			input:    "  foo :- bar.  ",
			expected: "foo :- bar.",
		},
		{
			name:     "unquote JSON string",
			input:    `"foo :- bar."`,
			expected: "foo :- bar.",
		},
		{
			name:     "unquote JSON string with escaped quotes",
			input:    `"foo :- \"bar\"."`,
			expected: `foo :- "bar".`,
		},
		{
			name:     "prolog negation with space",
			input:    `foo :- \+ bar.`,
			expected: `foo :- !bar.`,
		},
		{
			name:     "prolog negation without space",
			input:    `foo :- \+bar.`,
			expected: `foo :- !bar.`,
		},
		{
			name:     "prolog negation with multiple spaces",
			input:    `foo :- \+   bar.`,
			expected: `foo :- !bar.`,
		},
		{
			name:     "multiple prolog negations",
			input:    `foo :- \+ bar, \+ baz.`,
			expected: `foo :- !bar, !baz.`,
		},
		{
			name:     "backslash inside string - windows path",
			// \t and \f are known escapes, so they are not double-escaped
			input:    `foo :- bar("C:\path\to\file").`,
			expected: `foo :- bar("C:\\path\to\file").`,
		},
		{
			name:     "already escaped backslashes inside string",
			input:    `foo :- bar("C:\\path\\to\\file").`,
			expected: `foo :- bar("C:\\path\\to\\file").`,
		},
		{
			name:     "known escapes inside string",
			input:    `foo :- bar("hello\n\t\r\b\f\0world").`,
			expected: `foo :- bar("hello\n\t\r\b\f\0world").`,
		},
		{
			name:     "unknown escapes inside string",
			input:    `foo :- bar("unknown\escape").`,
			expected: `foo :- bar("unknown\\escape").`,
		},
		{
			name:     "backslash outside string",
			input:    `foo \ bar.`,
			expected: `foo \ bar.`, // no quotes, so it doesn't try to escape
		},
		{
			name:     "complex quoted string with mixed escapes",
			input:    `foo :- \+ bar("val1:\n", "C:\dir", "tab\t", "unk\own").`,
			expected: `foo :- !bar("val1:\n", "C:\\dir", "tab\t", "unk\\own").`,
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "empty unquoted string",
			input:    `""`,
			expected: "",
		},
		{
			name:     "invalid JSON quoted string",
			input:    `"foo`, // missing end quote
			expected: `"foo`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeRuleInput(tt.input)
			if got != tt.expected {
				t.Errorf("NormalizeRuleInput(%q) = %q; want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestIsKnownEscape(t *testing.T) {
	known := []byte{'\\', '"', 'n', 'r', 't', 'b', 'f', '0'}
	for _, b := range known {
		if !isKnownEscape(b) {
			t.Errorf("expected isKnownEscape(%q) to be true", b)
		}
	}

	unknown := []byte{'a', 'c', 'x', 'y', '1', 'z'}
	for _, b := range unknown {
		if isKnownEscape(b) {
			t.Errorf("expected isKnownEscape(%q) to be false", b)
		}
	}
}
