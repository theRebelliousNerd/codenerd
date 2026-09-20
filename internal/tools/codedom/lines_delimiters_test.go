package codedom

import (
	"strings"
	"testing"
)

// Non-Go extensions are judged by whole-file delimiter balance: a balanced
// result is accepted, an unbalanced result over a balanced original is
// refused, and an already-unbalanced original may be repaired.
func TestCheckDelimiterBalance_NonGoBalancedAfterAccepted(t *testing.T) {
	before := "class A {\n void f() {}\n}\n"
	after := "class A {\n void f() {}\n void g() {}\n}\n"
	if err := checkDelimiterBalance("demo.java", 1, []string{"void g() {}"}, before, after); err != nil {
		t.Fatalf("balanced non-Go result was refused: %v", err)
	}
}

func TestCheckDelimiterBalance_NonGoRepairAllowed(t *testing.T) {
	// Both files unbalanced: incremental repair must be allowed.
	before := "class A {\n void f() {\n"
	after := "class A {\n void f() {{\n"
	if err := checkDelimiterBalance("demo.java", 1, []string{"x"}, before, after); err != nil {
		t.Fatalf("repair of unbalanced non-Go file was refused: %v", err)
	}
}

func TestCheckDelimiterBalance_NonGoUnbalancedAfterRefused(t *testing.T) {
	before := "class A {\n void f() {}\n}\n"
	after := "class A {\n void f() {\n}\n"
	old := []string{" void f() {}"}
	err := checkDelimiterBalance("demo.java", 2, old, before, after)
	if err == nil {
		t.Fatal("unbalanced non-Go result was accepted; it must be refused")
	}
	if !strings.Contains(err.Error(), "delimiter balance") {
		t.Errorf("error does not name the cause: %v", err)
	}
	if !strings.Contains(err.Error(), "Replaced lines 2-2 were:") {
		t.Errorf("error missing replaced range:\n%v", err)
	}
}

func TestDelimitersBalanced_Pairs(t *testing.T) {
	balanced := []string{"", "no delimiters", "{}", "[]", "()", "{[()]}", "func f() { x(); }"}
	for _, src := range balanced {
		if !delimitersBalanced(src) {
			t.Errorf("delimitersBalanced(%q) = false, want true", src)
		}
	}
	unbalanced := []string{"}", "{", "(", "[", "{]", "(}", "{[}]", "}{", "{[}", "{"}
	for _, src := range unbalanced {
		if delimitersBalanced(src) {
			t.Errorf("delimitersBalanced(%q) = true, want false", src)
		}
	}
	// Empty-stack close and mismatched close are distinct false paths.
	if delimitersBalanced("}") {
		t.Error("lonely close must be unbalanced")
	}
	if delimitersBalanced("{]") {
		t.Error("mismatched close must be unbalanced")
	}
	// Misordered but net-zero input must still be refused: the stack matters.
	if delimitersBalanced("}{") {
		t.Error("misordered delimiters must be unbalanced")
	}
}

func TestDelimitersBalanced_LineComment(t *testing.T) {
	// Brace hidden inside a line comment; newline ends the comment so the
	// trailing pair keeps the file balanced.
	if !delimitersBalanced("// }\n{}") {
		t.Error("brace in line comment must be ignored")
	}
	// Comment body chars stay inside the comment (inner newline check false).
	if !delimitersBalanced("// } still comment") {
		t.Error("unterminated line comment hiding a brace must be balanced")
	}
	// A lone comment with no newline at all.
	if !delimitersBalanced("// { [ (") {
		t.Error("line comment must hide all delimiters")
	}
}

func TestDelimitersBalanced_BlockComment(t *testing.T) {
	if !delimitersBalanced("/* { */ {}") {
		t.Error("brace in block comment must be ignored")
	}
	// Star inside a comment that does not close it.
	if !delimitersBalanced("/* a * b */ {}") {
		t.Error("star inside block comment must not close it early")
	}
	// Comment body without any star.
	if !delimitersBalanced("/* hello { [ ( */") {
		t.Error("block comment must hide delimiters")
	}
	// Unterminated block comment leaves the file unbalanced.
	if delimitersBalanced("/* {") {
		t.Error("unterminated block comment must be unbalanced")
	}
}

func TestDelimitersBalanced_String(t *testing.T) {
	if !delimitersBalanced("s := \"{\"; {}") {
		t.Error("brace in string must be ignored")
	}
	// Escaped quote inside a string: backslash sets esc, next char clears it.
	if !delimitersBalanced("s := \"a\\\"b\"; {}") {
		t.Error("escaped quote in string must not end the string")
	}
	// Escaped backslash then a brace hidden in the string.
	if !delimitersBalanced("s := \"a\\\\\"; {}") {
		t.Error("escaped backslash must be handled")
	}
	// Ordinary chars inside a string.
	if !delimitersBalanced("s := \"abc\"; {}") {
		t.Error("string body must be skipped")
	}
	// Unterminated string.
	if delimitersBalanced("\"abc") {
		t.Error("unterminated string must be unbalanced")
	}
}

func TestDelimitersBalanced_Rune(t *testing.T) {
	if !delimitersBalanced("x := 'a'; {}") {
		t.Error("rune literal must be skipped")
	}
	if !delimitersBalanced("x := '{'; {}") {
		t.Error("brace in rune must be ignored")
	}
	// Escaped char inside a rune.
	if !delimitersBalanced("x := '\\''; {}") {
		t.Error("escaped quote in rune must not end the rune")
	}
	// Escaped backslash inside a rune.
	if !delimitersBalanced("x := '\\\\'; {}") {
		t.Error("escaped backslash in rune must be handled")
	}
	// Unterminated rune.
	if delimitersBalanced("x := 'a") {
		t.Error("unterminated rune must be unbalanced")
	}
}

func TestDelimitersBalanced_RawString(t *testing.T) {
	if !delimitersBalanced("s := `{`; {}") {
		t.Error("brace in raw string must be ignored")
	}
	// Raw body chars stay hidden until the closing backtick.
	if !delimitersBalanced("s := `a { [ ( b`; {}") {
		t.Error("raw string body must be skipped")
	}
}

func TestDelimitersBalanced_Slashes(t *testing.T) {
	// A slash that starts no comment falls through to normal handling.
	if !delimitersBalanced("a/b; {}") {
		t.Error("lone slash must not affect balance")
	}
	// Slash as the final character has no next rune.
	if !delimitersBalanced("{} /") {
		t.Error("trailing slash must not affect balance")
	}
	if !delimitersBalanced("// c\n{}") {
		t.Error("line comment entry must hide delimiters")
	}
	if !delimitersBalanced("/* c */ {}") {
		t.Error("block comment entry must hide delimiters")
	}
	if !delimitersBalanced("s := \"x\"; {}") {
		t.Error("string entry must hide delimiters")
	}
	if !delimitersBalanced("x := 'q'; {}") {
		t.Error("rune entry must hide delimiters")
	}
	if !delimitersBalanced("s := `x`; {}") {
		t.Error("raw string entry must hide delimiters")
	}
}
