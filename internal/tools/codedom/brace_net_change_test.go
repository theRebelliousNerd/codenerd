package codedom

import "testing"

func TestBraceNetChange_Behaviour(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  int
	}{
		{name: "empty", input: "", want: 0},
		{name: "open", input: "{", want: 1},
		{name: "close", input: "}", want: -1},
		{name: "balanced", input: "{{}}", want: 0},
		{name: "two open", input: "{{", want: 2},
		{name: "code with brace", input: "func Foo() {", want: 1},

		// Single-quoted literals: braces inside do not count.
		{name: "brace in single quotes", input: "'{'", want: 0},
		{name: "close brace in single quotes", input: "'}'", want: 0},
		{name: "single quoted then real open", input: "'}' {", want: 1},
		{name: "double quote in single quotes", input: `'"+{'`, want: 0},
		// Backslash escapes the next char inside single quotes, so an
		// escaped quote does not end the literal.
		{name: "escaped quote in single quotes", input: "'\\''", want: 0},
		{name: "escaped quote then real open", input: "'\\''{", want: 1},
		{name: "escaped letter in single quotes", input: "'\\n'", want: 0},
		{name: "escaped letter then real open", input: "'\\n'{", want: 1},

		// Double-quoted literals: braces inside do not count.
		{name: "brace in double quotes", input: `"{"`, want: 0},
		{name: "close brace in double quotes", input: `"}"`, want: 0},
		{name: "single quote in double quotes", input: `"'"`, want: 0},
		{name: "double quoted then real open", input: `"}" {`, want: 1},
		// Backslash escapes the next char inside double quotes.
		{name: "escaped quote in double quotes", input: `"\""`, want: 0},
		{name: "escaped quote then real open", input: `"\""{`, want: 1},
		{name: "escaped letter in double quotes", input: `"\n"`, want: 0},
		{name: "brace after escaped letter then close", input: `"\n"{`, want: 1},

		// Backtick literals: braces inside do not count.
		{name: "brace in backticks", input: "`{`", want: 0},
		{name: "close brace in backticks", input: "`}`", want: 0},
		{name: "letter in backticks", input: "`a`", want: 0},
		{name: "backticks then real open", input: "`}` {", want: 1},
		// Backslash escapes the next char inside backticks too.
		{name: "escaped letter in backticks", input: "`\\x`", want: 0},
		{name: "escaped letter then real open", input: "`\\x`{", want: 1},
		{name: "escaped backtick in backticks", input: "`\\``", want: 0},
		{name: "escaped backtick then real open", input: "`\\``{", want: 1},

		// Line comments: everything after // is ignored.
		{name: "comment with open brace", input: "//{", want: 0},
		{name: "comment with close brace", input: "//}", want: 0},
		{name: "open before comment with close", input: "{//}", want: 1},
		{name: "code brace then comment brace", input: "x { // }", want: 1},
		{name: "comment after string", input: `"{" // }`, want: 0},
		// A single slash is not a comment.
		{name: "single slash with brace", input: "/{", want: 1},
		{name: "slash between names with brace", input: "a/b {", want: 1},
		{name: "trailing slash", input: "a/", want: 0},
		{name: "slash alone", input: "/", want: 0},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := braceNetChange(tc.input); got != tc.want {
				t.Errorf("braceNetChange(%q) = %d, want %d", tc.input, got, tc.want)
			}
		})
	}
}

func TestBraceNetChange_CommentTerminatesLine(t *testing.T) {
	t.Parallel()

	// The close brace after // must not cancel the open brace before it.
	if got := braceNetChange("{ // }"); got != 1 {
		t.Errorf("braceNetChange({ // }) = %d, want 1", got)
	}
	// Without the comment the braces balance out.
	if got := braceNetChange("{ }"); got != 0 {
		t.Errorf("braceNetChange({ }) = %d, want 0", got)
	}
}

func TestBraceNetChange_EscapedQuoteDoesNotEndLiteral(t *testing.T) {
	t.Parallel()

	// Inside '...', the sequence \' stays in the literal: the brace that
	// follows is still quoted, so only the trailing real open counts.
	if got := braceNetChange("'\\''{"); got != 1 {
		t.Errorf("single-quoted escaped close: got %d, want 1", got)
	}
	// Inside "...", the sequence \" stays in the literal.
	if got := braceNetChange(`"\""{`); got != 1 {
		t.Errorf("double-quoted escaped close: got %d, want 1", got)
	}
	// Inside `...`, the sequence \` stays in the literal.
	if got := braceNetChange("`\\``{"); got != 1 {
		t.Errorf("backtick escaped close: got %d, want 1", got)
	}
}
