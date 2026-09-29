package codedom

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/tools"
)

const calcSrc = "package calc\n\nfunc Add(a, b int) int {\n\treturn a + b\n}\n\nfunc Sub(a, b int) int {\n\treturn a - b\n}\n"

func resolvedTemp(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	return dir
}

func editRecordFixture(t *testing.T, name, content string) (*tools.Registry, string) {
	t.Helper()
	dir := resolvedTemp(t)
	abs := filepath.Join(dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	reg := tools.NewRegistry()
	if err := RegisterAll(reg); err != nil {
		t.Fatal(err)
	}
	reg.SetWorkspaceRoot(dir)
	return reg, name
}

func lineNo(t *testing.T, src, frag string) int {
	t.Helper()
	for i, line := range strings.Split(src, "\n") {
		if strings.Contains(line, frag) {
			return i + 1
		}
	}
	t.Fatalf("no line contains %q", frag)
	return 0
}

// declarationLines is the 1-based span of the function whose signature line
// contains decl. It is a test aid for addressing a fixture, not a parser.
func declarationLines(t *testing.T, src, decl string) (int, int) {
	t.Helper()
	lines := strings.Split(src, "\n")
	start := 0
	for i, line := range lines {
		if strings.Contains(line, decl) {
			start = i + 1
			break
		}
	}
	if start == 0 {
		t.Fatalf("declaration %q not found", decl)
	}
	depth := 0
	seen := false
	for i := start - 1; i < len(lines); i++ {
		depth += strings.Count(lines[i], "{") - strings.Count(lines[i], "}")
		if strings.Contains(lines[i], "{") {
			seen = true
		}
		if seen && depth == 0 {
			return start, i + 1
		}
	}
	t.Fatalf("declaration %q has no closing brace", decl)
	return 0, 0
}

func declsOf(edits []tools.EditedElement) []tools.EditedElement {
	var out []tools.EditedElement
	for _, e := range edits {
		if e.Kind == "header" || e.Kind == "syntax_error" {
			continue
		}
		out = append(out, e)
	}
	return out
}

func TestEditLines_RecordsTheElementWhoseBodyChanged(t *testing.T) {
	reg, rel := editRecordFixture(t, "calc.go", calcSrc)
	n := lineNo(t, calcSrc, "return a + b")
	_, edits, err := execTool(reg, "edit_lines", map[string]any{
		"path": rel, "start_line": n, "end_line": n, "new_content": "\treturn a + b + 1",
	})
	if err != nil {
		t.Fatal(err)
	}
	got := declsOf(edits)
	if len(got) != 1 || got[0].Name != "Add" || got[0].Kind != "function" || got[0].Package != "calc" || got[0].Receiver != "" || got[0].File != "calc.go" || got[0].Removed {
		t.Fatalf("got %+v", edits)
	}
}

func TestEditLines_CommentBetweenFunctionsRecordsNoFunction(t *testing.T) {
	reg, rel := editRecordFixture(t, "calc.go", calcSrc)
	n := lineNo(t, calcSrc, "return a + b") + 1 // the closing brace
	_, edits, err := execTool(reg, "insert_lines", map[string]any{
		"path": rel, "after_line": n, "content": "// note\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := declsOf(edits); len(got) != 0 {
		t.Fatalf("comment between functions recorded %+v", got)
	}
}

func TestInsertLines_RecordsTheNewFunctionOnly(t *testing.T) {
	reg, rel := editRecordFixture(t, "calc.go", calcSrc)
	n := lineNo(t, calcSrc, "return a + b") + 1
	_, edits, err := execTool(reg, "insert_lines", map[string]any{
		"path": rel, "after_line": n, "content": "\nfunc Mul(a, b int) int {\n\treturn a * b\n}\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	got := declsOf(edits)
	if len(got) != 1 || got[0].Name != "Mul" || got[0].Kind != "function" || got[0].Removed {
		t.Fatalf("got %+v", edits)
	}
}

func TestDeleteLines_RecordsTheRemovedFunction(t *testing.T) {
	reg, rel := editRecordFixture(t, "calc.go", calcSrc)
	start, end := declarationLines(t, calcSrc, "func Sub")
	_, edits, err := execTool(reg, "delete_lines", map[string]any{
		"path": rel, "start_line": start, "end_line": end,
	})
	if err != nil {
		t.Fatal(err)
	}
	got := declsOf(edits)
	if len(got) != 1 || got[0].Name != "Sub" || !got[0].Removed || got[0].Kind != "function" {
		t.Fatalf("got %+v", edits)
	}
}

func TestEditLines_GenericReceiverRecordsTheBaseType(t *testing.T) {
	src := "package elemprobe\n\ntype Box[T any] struct{ v T }\n\nfunc Get() int { return 1 }\n\nfunc (b *Box[T]) Get() T {\n\treturn b.v\n}\n"
	reg, rel := editRecordFixture(t, "box.go", src)
	n := lineNo(t, src, "return b.v")
	_, edits, err := execTool(reg, "edit_lines", map[string]any{
		"path": rel, "start_line": n, "end_line": n, "new_content": "\treturn b.v /*t*/",
	})
	if err != nil {
		t.Fatal(err)
	}
	got := declsOf(edits)
	if len(got) != 1 || got[0].Kind != "method" || got[0].Name != "Get" || got[0].Receiver != "Box" || got[0].Package != "elemprobe" {
		t.Fatalf("got %+v", edits)
	}
}

func TestEditLines_MangleRuleIsRecorded(t *testing.T) {
	src := "# a doc line\nDecl p(X) bound [/string].\n\np(\"a\").\nq(X) :- p(X).\n"
	reg, rel := editRecordFixture(t, "x.mg", src)
	n := lineNo(t, src, "q(X) :- p(X).")
	_, edits, err := execTool(reg, "edit_lines", map[string]any{
		"path": rel, "start_line": n, "end_line": n, "new_content": "q(X) :- p(X), p(X).",
	})
	if err != nil {
		t.Fatal(err)
	}
	var rules int
	for _, e := range edits {
		if e.Kind == "decl" || e.Kind == "fact" {
			t.Errorf("unchanged statement recorded: %+v", e)
		}
		if e.Kind == "rule" && e.Name == "q" && e.Language == "mangle" {
			rules++
		}
	}
	if rules == 0 {
		t.Fatalf("mangle rule edit recorded nothing: %+v", edits)
	}
}

func TestEditLines_RefusedEditRecordsNothing(t *testing.T) {
	reg, rel := editRecordFixture(t, "calc.go", calcSrc)
	n := lineNo(t, calcSrc, "return a + b")
	var edits []tools.EditedElement
	called := false
	reg.SetFactSink(func(_ context.Context, rec tools.ExecutionRecord) {
		called = true
		edits = rec.Edits
	})
	_, err := reg.Execute(context.Background(), "edit_lines", map[string]any{
		"path": rel, "start_line": n, "end_line": n, "new_content": "\treturn (",
	})
	if err == nil {
		t.Fatal("expected the syntax guard to refuse the edit")
	}
	if !called {
		t.Fatal("the completion sink did not see the refused execution")
	}
	if len(edits) != 0 {
		t.Fatalf("refused edit recorded %+v", edits)
	}
}
