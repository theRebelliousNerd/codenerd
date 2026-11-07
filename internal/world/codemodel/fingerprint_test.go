package codemodel

import "testing"

func TestFingerprintIgnoresLayoutAndNothingElse(t *testing.T) {
	base := "func Add(a, b int) int {\n\treturn a + b\n}\n"
	same := []string{
		"func Add(a,  b int) int {\r\n    return a +\tb\r\n}",
		"\n\nfunc Add(a, b int) int {\n\n\treturn a + b   \n}\n\n",
		"func Add(a, b int) int { return a + b }",
		// what a formatter changes: spaces around operators, commas and braces
		"func Add(a,b int)int{return a+b}",
		"func   Add ( a , b int ) int { return a + b }",
	}
	want := Fingerprint(LangGo, base)
	for _, s := range same {
		if got := Fingerprint(LangGo, s); got != want {
			t.Errorf("whitespace-only change moved the fingerprint:\n%q", s)
		}
		// Revision is the exact-text precondition and must keep telling layouts apart.
		if Revision(s) == Revision(base) {
			t.Errorf("Revision folded layout it must keep: %q", s)
		}
	}
	for _, s := range []string{
		"func Add(a, b int) int {\n\treturn a - b\n}\n",
		"func Add(a, b int64) int {\n\treturn a + b\n}\n",
		"func Add(a, b int) int {\n\treturna + b\n}\n",
	} {
		if Fingerprint(LangGo, s) == want {
			t.Errorf("a token change left the fingerprint alone:\n%q", s)
		}
	}
}

func TestFingerprintKeepsPythonBlockStructure(t *testing.T) {
	a := "def f(x):\n    if x:\n        return 1\n    return 2\n"
	reflowed := "def f(x):   \n\n    if x:\n        return  1\n    return 2"
	moved := "def f(x):\n    if x:\n        return 1\n        return 2\n"
	if Fingerprint(LangPython, a) != Fingerprint(LangPython, reflowed) {
		t.Error("trailing whitespace and blank lines are not Python layout")
	}
	if Fingerprint(LangPython, a) == Fingerprint(LangPython, moved) {
		t.Error("moving a statement into another block is a change")
	}
	// The same two texts are one text in a language without significant indentation.
	if Fingerprint(LangGo, a) != Fingerprint(LangGo, moved) {
		t.Error("control: outside Python the indentation is folded")
	}
}

func TestFingerprintKeepsASpaceThatSeparatesTwoTokens(t *testing.T) {
	if Fingerprint(LangGo, "x := 1") != Fingerprint(LangGo, "x:=1") {
		t.Error("spaces around an operator are layout")
	}
	if Fingerprint(LangGo, "func F() {}") != Fingerprint(LangGo, "func F() {\n}") {
		t.Error("a line break inside braces is layout")
	}
	if Fingerprint(LangGo, "var a int") == Fingerprint(LangGo, "vara int") {
		t.Error("joining two identifiers is a change")
	}
	if Fingerprint(LangMarkdown, "Run it  twice.") != Fingerprint(LangMarkdown, "Run it\ntwice.") {
		t.Error("reflowed prose is layout")
	}
	if Fingerprint(LangMarkdown, "Run it twice.") == Fingerprint(LangMarkdown, "Run it once.") {
		t.Error("rewording is a change")
	}
}
