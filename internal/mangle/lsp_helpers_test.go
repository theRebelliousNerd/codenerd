package mangle

import "testing"

func TestIsWordChar(t *testing.T) {
	for _, c := range []byte{'a', 'Z', '0', '_', '/', ':'} {
		if !isWordChar(c) {
			t.Errorf("isWordChar(%q) = false, want true", c)
		}
	}
	for _, c := range []byte{' ', '(', ',', '.', '"'} {
		if isWordChar(c) {
			t.Errorf("isWordChar(%q) = true, want false", c)
		}
	}
}

func TestGetWordAtPosition(t *testing.T) {
	line := "user_intent(/query)."
	cases := []struct {
		col  int
		want string
	}{
		{0, "user_intent"},  // start of predicate
		{5, "user_intent"},  // middle of predicate
		{12, "/query"},      // inside the name constant (after '(')
		{-1, ""},            // out of range low
		{len(line) + 1, ""}, // out of range high
	}
	for _, c := range cases {
		if got := getWordAtPosition(line, c.col); got != c.want {
			t.Errorf("getWordAtPosition(col=%d)=%q, want %q", c.col, got, c.want)
		}
	}
}

func TestGetWordPrefixAtPosition(t *testing.T) {
	line := "focus_res"
	if got := getWordPrefixAtPosition(line, 5); got != "focus" {
		t.Errorf("prefix at col 5 = %q, want focus", got)
	}
	if got := getWordPrefixAtPosition(line, len(line)); got != "focus_res" {
		t.Errorf("prefix at end = %q, want focus_res", got)
	}
	if got := getWordPrefixAtPosition(line, -1); got != "" {
		t.Errorf("prefix at col -1 = %q, want empty", got)
	}
}

func TestCountArity(t *testing.T) {
	cases := []struct {
		line string
		want int
	}{
		{"pred().", 0},             // no args
		{"pred(a).", 1},            // single arg
		{"pred(a, b, c).", 3},      // three args
		{`pred("a, b", c).`, 2},    // comma inside quotes ignored
		{"pred(foo(x, y), z).", 2}, // nested parens: only depth-1 commas count
		{"no_parens_here", 0},      // no parens at all
	}
	for _, c := range cases {
		if got := countArity(c.line); got != c.want {
			t.Errorf("countArity(%q)=%d, want %d", c.line, got, c.want)
		}
	}
}
