package codedom

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The defect these guard (F-DOM-4, observed live): an edit_lines range that
// ended on a closing brace was replaced by content that omitted it. The write
// succeeded, the tool reported success, and the package stopped compiling
// several declarations further down -- so the error surfaced detached from its
// cause. Under an unattended run, more edits get layered on top of the broken
// file before anything notices.

const balGoSrc = `package demo

type Config struct {
	A string
	B string
}

func Helper() {}
`

func writeGo(t *testing.T, src string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "demo.go")
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return path
}

func TestEditLines_RefusesEditThatDropsClosingBrace(t *testing.T) {
	path := writeGo(t, balGoSrc)
	before, _ := os.ReadFile(path)

	// Lines 3-6 are the struct through its closing brace. The replacement
	// keeps the fields but drops the "}" -- exactly the live failure.
	_, err := executeEditLines(context.Background(), map[string]any{
		"path":        path,
		"start_line":  3,
		"end_line":    6,
		"new_content": "type Config struct {\n\tA string\n\tB string\n\tC string",
	})
	if err == nil {
		t.Fatal("edit that drops a closing brace was accepted; it must be refused")
	}
	if !strings.Contains(err.Error(), "delimiter balance") {
		t.Errorf("error does not name the cause: %v", err)
	}

	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Error("file was modified despite the edit being refused")
	}
}

func TestEditLines_AllowsBalancedEdit(t *testing.T) {
	path := writeGo(t, balGoSrc)

	// Same edit, but correctly reproducing the closing brace.
	out, err := executeEditLines(context.Background(), map[string]any{
		"path":        path,
		"start_line":  3,
		"end_line":    6,
		"new_content": "type Config struct {\n\tA string\n\tB string\n\tC string\n}",
	})
	if err != nil {
		t.Fatalf("balanced edit was refused: %v", err)
	}
	if !strings.Contains(out, "Replaced lines 3-6") {
		t.Errorf("unexpected result: %s", out)
	}

	got, _ := os.ReadFile(path)
	if !strings.Contains(string(got), "C string") {
		t.Error("edit did not apply")
	}
}

// A brace inside a comment or string is not structural and must not make a
// legitimate edit look unbalanced -- a guard that cries wolf gets worked around.
func TestEditLines_IgnoresDelimitersInCommentsAndStrings(t *testing.T) {
	path := writeGo(t, balGoSrc)

	out, err := executeEditLines(context.Background(), map[string]any{
		"path":       path,
		"start_line": 8,
		"end_line":   8,
		"new_content": "func Helper() {\n" +
			"\t// a lone } in a comment\n" +
			"\ts := \"an unmatched { in a string\"\n" +
			"\t_ = s\n" +
			"}",
	})
	if err != nil {
		t.Fatalf("edit with delimiters inside a comment/string was wrongly refused: %v", err)
	}
	if !strings.Contains(out, "Replaced lines 8-8") {
		t.Errorf("unexpected result: %s", out)
	}
}

// Non-brace languages must pass through untouched.
func TestEditLines_SkipsBalanceCheckForNonBraceFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notes.md")
	if err := os.WriteFile(path, []byte("# Title\n\nsome { text\n"), 0644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if _, err := executeEditLines(context.Background(), map[string]any{
		"path":        path,
		"start_line":  3,
		"end_line":    3,
		"new_content": "replaced with an unmatched } brace",
	}); err != nil {
		t.Fatalf("markdown edit was refused by the brace check: %v", err)
	}
}

func TestNetDelimiters(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want map[rune]int
	}{
		{"balanced", "func f() {}", map[rune]int{'{': 0, '[': 0, '(': 0}},
		{"open brace", "func f() {", map[rune]int{'{': 1, '[': 0, '(': 0}},
		{"close brace", "}", map[rune]int{'{': -1, '[': 0, '(': 0}},
		{"brace in line comment", "// }", map[rune]int{'{': 0, '[': 0, '(': 0}},
		{"brace in block comment", "/* { { */", map[rune]int{'{': 0, '[': 0, '(': 0}},
		{"brace in string", `s := "{"`, map[rune]int{'{': 0, '[': 0, '(': 0}},
		{"brace in raw string", "s := `{`", map[rune]int{'{': 0, '[': 0, '(': 0}},
		{"escaped quote then brace", `s := "\"" + "{"`, map[rune]int{'{': 0, '[': 0, '(': 0}},
		{"slice literal", "x := []int{1}", map[rune]int{'{': 0, '[': 0, '(': 0}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := netDelimiters(tc.src)
			for k, want := range tc.want {
				if got[k] != want {
					t.Errorf("net[%q] = %d, want %d (src: %s)", k, got[k], want, tc.src)
				}
			}
		})
	}
}
