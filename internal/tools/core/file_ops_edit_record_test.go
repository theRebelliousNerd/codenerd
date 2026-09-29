package core

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

func execFileTool(t *testing.T, dir, name string, args map[string]any) ([]tools.EditedElement, bool, error) {
	t.Helper()
	reg := tools.NewRegistry()
	if err := RegisterAll(reg); err != nil {
		t.Fatal(err)
	}
	reg.SetWorkspaceRoot(dir)
	var edits []tools.EditedElement
	called := false
	reg.SetFactSink(func(_ context.Context, rec tools.ExecutionRecord) {
		called = true
		edits = append([]tools.EditedElement(nil), rec.Edits...)
	})
	_, err := reg.Execute(context.Background(), name, args)
	return edits, called, err
}

func writeCalc(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "calc.go"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
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

func TestEditFile_RecordsTheElementWhoseBodyChanged(t *testing.T) {
	dir := resolvedTemp(t)
	writeCalc(t, dir, calcSrc)
	edits, called, err := execFileTool(t, dir, "edit_file", map[string]any{
		"path": "calc.go", "old_text": "return a + b", "new_text": "return a + b + 1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("completion sink was not called")
	}
	got := declsOf(edits)
	if len(got) != 1 || got[0].Name != "Add" || got[0].Kind != "function" || got[0].Package != "calc" || got[0].File != "calc.go" || got[0].Removed {
		t.Fatalf("got %+v", edits)
	}
}

func TestEditFile_UnchangedReplacementRecordsNothing(t *testing.T) {
	dir := resolvedTemp(t)
	writeCalc(t, dir, calcSrc)
	edits, called, err := execFileTool(t, dir, "edit_file", map[string]any{
		"path": "calc.go", "old_text": "return a + b", "new_text": "return a + b",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !called || len(edits) != 0 {
		t.Fatalf("called=%v edits=%+v", called, edits)
	}
}

func TestWriteFile_NewFileRecordsItsElements(t *testing.T) {
	dir := resolvedTemp(t)
	edits, called, err := execFileTool(t, dir, "write_file", map[string]any{
		"path": "calc.go", "content": calcSrc,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("completion sink was not called")
	}
	names := map[string]bool{}
	var headers int
	for _, e := range edits {
		if e.Removed {
			t.Fatalf("new file element marked removed: %+v", e)
		}
		if e.Kind == "header" {
			headers++
		}
		if e.Kind == "function" {
			names[e.Name] = true
		}
	}
	if headers != 1 || !names["Add"] || !names["Sub"] {
		t.Fatalf("got %+v", edits)
	}
}

func TestWriteFile_OverwriteRecordsOnlyTheChangedFunction(t *testing.T) {
	dir := resolvedTemp(t)
	writeCalc(t, dir, calcSrc)
	body := strings.Replace(calcSrc, "return a - b", "return a - b - 1", 1)
	edits, _, err := execFileTool(t, dir, "write_file", map[string]any{
		"path": "calc.go", "content": body,
	})
	if err != nil {
		t.Fatal(err)
	}
	got := declsOf(edits)
	if len(got) != 1 || got[0].Name != "Sub" || got[0].Removed {
		t.Fatalf("got %+v", edits)
	}
}

func TestWriteFile_CRLFRewriteOfTheSameTextRecordsNothing(t *testing.T) {
	dir := resolvedTemp(t)
	crlf := strings.ReplaceAll(calcSrc, "\n", "\r\n")
	writeCalc(t, dir, crlf)
	edits, called, err := execFileTool(t, dir, "write_file", map[string]any{
		"path": "calc.go", "content": calcSrc,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !called || len(edits) != 0 {
		t.Fatalf("called=%v edits=%+v", called, edits)
	}
}

func TestDeleteFile_RecordsTheElementsItRemoved(t *testing.T) {
	dir := resolvedTemp(t)
	writeCalc(t, dir, calcSrc)
	edits, called, err := execFileTool(t, dir, "delete_file", map[string]any{
		"path": "calc.go", "confirmed": true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("completion sink was not called")
	}
	if _, err := os.Stat(filepath.Join(dir, "calc.go")); !os.IsNotExist(err) {
		t.Fatalf("file still present: %v", err)
	}
	names := map[string]bool{}
	for _, e := range declsOf(edits) {
		if !e.Removed {
			t.Fatalf("not removed: %+v", e)
		}
		names[e.Name] = true
	}
	if !names["Add"] || !names["Sub"] {
		t.Fatalf("got %+v", edits)
	}
}

// Python is a CodeDOM language: a write records the elements it created and a
// delete records them removed, the same as Go.
func TestWriteFile_PythonRecordsItsElements(t *testing.T) {
	dir := resolvedTemp(t)
	body := "def add(a, b):\n    return a + b\n"
	edits, called, err := execFileTool(t, dir, "write_file", map[string]any{
		"path": "calc.py", "content": body,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !called || len(edits) != 1 || edits[0].Name != "add" || edits[0].Language != "python" || edits[0].Removed {
		t.Fatalf("python write recorded %+v (called=%v)", edits, called)
	}
	edits, called, err = execFileTool(t, dir, "delete_file", map[string]any{
		"path": "calc.py", "confirmed": true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !called || len(edits) != 1 || edits[0].Name != "add" || !edits[0].Removed {
		t.Fatalf("python delete recorded %+v (called=%v)", edits, called)
	}
}

func TestWriteFile_RefusedSyntaxRecordsNothing(t *testing.T) {
	dir := resolvedTemp(t)
	edits, called, err := execFileTool(t, dir, "write_file", map[string]any{
		"path": "calc.go", "content": "package calc\nfunc (",
	})
	if err == nil {
		t.Fatal("expected the syntax guard to refuse the write")
	}
	if !called {
		t.Fatal("the completion sink did not see the refused execution")
	}
	if len(edits) != 0 {
		t.Fatalf("refused write recorded %+v", edits)
	}
}
