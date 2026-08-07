package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/tools"
)

func TestNumberLines_StartsAtOne(t *testing.T) {
	got := numberLines("alpha\nbeta\ngamma", 1)
	want := "1\talpha\n2\tbeta\n3\tgamma"
	if got != want {
		t.Errorf("numberLines() = %q, want %q", got, want)
	}
}

// A ranged read numbered from 1 is worse than no numbers: it looks authoritative
// and is wrong by the offset, which is exactly the citation drift this fixes.
func TestNumberLines_RangedReadKeepsRealOffsets(t *testing.T) {
	got := numberLines("func validate()\n\treturn nil\n}", 200)
	want := "200\tfunc validate()\n201\t\treturn nil\n202\t}"
	if got != want {
		t.Errorf("numberLines() = %q, want %q", got, want)
	}
}

func TestNumberLines_EmptyAndSingleLine(t *testing.T) {
	if got := numberLines("", 1); got != "" {
		t.Errorf("numberLines(\"\") = %q, want empty", got)
	}
	if got := numberLines("only", 7); got != "7\tonly" {
		t.Errorf("numberLines single line = %q", got)
	}
}

// Trailing newlines must not silently shift every subsequent number.
func TestNumberLines_TrailingNewlineIsItsOwnLine(t *testing.T) {
	got := numberLines("a\nb\n", 1)
	want := "1\ta\n2\tb\n3\t"
	if got != want {
		t.Errorf("numberLines() = %q, want %q", got, want)
	}
}

// End-to-end: the number a model reads must be the number `sed -n Np` prints,
// for a whole-file read and for a ranged one.
func TestExecuteReadFile_LineNumbersMatchTheRealFile(t *testing.T) {
	dir := t.TempDir()
	// On Windows TempDir hands back a path that may differ from its resolved
	// form; the workspace guard compares resolved paths and would reject a read
	// inside its own temp dir.
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	path := filepath.Join(dir, "sample.go")
	body := "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"hi\")\n}\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write sample: %v", err)
	}

	ctx := context.WithValue(context.Background(), tools.CtxKeyWorkspaceRoot, dir)

	out, err := executeReadFile(ctx, map[string]any{"path": path})
	if err != nil {
		t.Fatalf("executeReadFile: %v", err)
	}
	if !strings.Contains(out, "5\tfunc main() {") {
		t.Errorf("whole-file read did not number func main() as line 5.\ngot:\n%s", out)
	}
	if !strings.Contains(out, "1\tpackage main") {
		t.Errorf("whole-file read did not number the package clause as line 1.\ngot:\n%s", out)
	}

	// JSON delivers integers as float64; the ranged path must survive that.
	ranged, err := executeReadFile(ctx, map[string]any{
		"path":       path,
		"start_line": float64(5),
		"end_line":   float64(7),
	})
	if err != nil {
		t.Fatalf("ranged executeReadFile: %v", err)
	}
	if !strings.HasPrefix(ranged, "5\tfunc main() {") {
		t.Errorf("ranged read must number from start_line, not from 1.\ngot:\n%s", ranged)
	}
	if strings.Contains(ranged, "1\tfunc main() {") {
		t.Errorf("ranged read renumbered from 1, which cites confidently and wrongly.\ngot:\n%s", ranged)
	}
}

func TestStripLineNumberPrefixes(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
		ok   bool
	}{
		{"all lines numbered", "5\tfunc main() {\n6\t\treturn\n7\t}", "func main() {\n\treturn\n}", true},
		{"single numbered line", "42\tx := 1", "x := 1", true},
		{"no prefixes", "func main() {\n}", "func main() {\n}", false},
		// Must not rewrite a genuine edit to tab-separated numeric data.
		{"mixed", "5\tfoo\nbar", "5\tfoo\nbar", false},
		{"non-numeric prefix", "ab\tfoo", "ab\tfoo", false},
		{"empty", "", "", false},
		{"leading tab, no number", "\tindented", "\tindented", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := stripLineNumberPrefixes(tc.in)
			if ok != tc.ok {
				t.Fatalf("ok = %v, want %v", ok, tc.ok)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// A model that pastes what read_file returned must still land its edit.
func TestExecuteEditFile_RecoversFromPastedLineNumbers(t *testing.T) {
	dir := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	path := filepath.Join(dir, "sample.go")
	if err := os.WriteFile(path, []byte("package main\n\nfunc main() {\n}\n"), 0o644); err != nil {
		t.Fatalf("write sample: %v", err)
	}

	ctx := context.WithValue(context.Background(), tools.CtxKeyWorkspaceRoot, dir)
	if _, err := executeEditFile(ctx, map[string]any{
		"path":     path,
		"old_text": "3\tfunc main() {\n4\t}",
		"new_text": "func main() {\n\treturn\n}",
	}); err != nil {
		t.Fatalf("executeEditFile: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if !strings.Contains(string(got), "func main() {\n\treturn\n}") {
		t.Errorf("edit did not land.\ngot:\n%s", got)
	}
	if strings.Contains(string(got), "3\t") {
		t.Errorf("line-number prefix was written into the file.\ngot:\n%s", got)
	}
}
