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
			name: "backslash inside string - windows path",
			// \t is a real Mangle escape and passes through. \p and \f are
			// not (Mangle rejects \f at lex time), so they are doubled and
			// parse as literal text.
			input:    `foo :- bar("C:\path\to\file").`,
			expected: `foo :- bar("C:\\path\to\\file").`,
		},
		{
			name:     "already escaped backslashes inside string",
			input:    `foo :- bar("C:\\path\\to\\file").`,
			expected: `foo :- bar("C:\\path\\to\\file").`,
		},
		{
			name:  "known escapes inside string",
			input: `foo :- bar("hello\n\tworld").`,
			// Only Mangle's real escapes (\n \t \\ " ' ` \xHH \u{h..})
			// pass through; the rest are doubled below.
			expected: `foo :- bar("hello\n\tworld").`,
		},
		{
			name:  "non-mangle escapes are doubled to literal text",
			input: `foo :- bar("a\rb\fc\0d").`,
			// \r \b \f \0 are Go escapes, not Mangle escapes: Mangle's
			// lexer rejects them with "token recognition error" (proven
			// against mangle-go's parser), so normalize doubles them and the
			// rule parses with literal backslash text.
			expected: `foo :- bar("a\\rb\\fc\\0d").`,
		},
		{
			name:     "well-formed hex and unicode escapes pass through",
			input:    `foo :- bar("A\x41B\u{00e9}C").`,
			expected: `foo :- bar("A\x41B\u{00e9}C").`,
		},
		{
			name:  "malformed hex escape is doubled",
			input: `foo :- bar("C:\xerox").`,
			// \x without two hex digits is not a Mangle escape; doubling
			// keeps Windows-style paths parseable as literal text.
			expected: `foo :- bar("C:\\xerox").`,
		},
		{
			name:  "uppercase and out-of-range hex escapes are doubled",
			input: `foo :- bar("\xE9\xe9").`,
			// The grammar's HEXDIGIT is lowercase-only, and Unescape rejects
			// bytes above 0x7F in strings: neither form can survive, so both
			// become literal text.
			expected: `foo :- bar("\\xE9\\xe9").`,
		},
		{
			name:  "short unicode escape is doubled",
			input: `foo :- bar("\u{e9}").`,
			// The grammar demands 4-6 hex digits in \u{...}; fewer fail the lex.
			expected: `foo :- bar("\\u{e9}").`,
		},
		{
			name:     "prolog negation inside string is data",
			input:    `foo :- bar("a\+b").`,
			expected: `foo :- bar("a\\+b").`,
		},
		{
			name:  "escaped backslash before quote keeps boundary",
			input: `foo :- bar("a\\"), baz(X).`,
			// The \\" is an escaped backslash plus the closing quote: the
			// string ends there and the rest normalizes outside strings.
			expected: `foo :- bar("a\\"), baz(X).`,
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
	known := []byte{'\\', '"', 'n', 't', '\'', '\n'}
	for _, b := range known {
		if !isKnownEscape(b) {
			t.Errorf("expected isKnownEscape(%q) to be true", b)
		}
	}

	// r b f 0 are Go escapes Mangle's lexer rejects; x and u need shape
	// validation in copyStringEscape and must never be copied blindly.
	unknown := []byte{'a', 'c', 'x', 'y', '1', 'z', 'r', 'b', 'f', '0', 'u', 'v', '`'}
	for _, b := range unknown {
		if isKnownEscape(b) {
			t.Errorf("expected isKnownEscape(%q) to be false", b)
		}
	}
}
