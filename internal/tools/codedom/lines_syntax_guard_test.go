package codedom

import (
	"os"
	"path/filepath"
	"testing"
)

const syntaxGuardValidGo = "package x\n\nfunc Ok() {}\n"

func writeSyntaxGuardFixture(t *testing.T) (tmpFile string, before []byte) {
	t.Helper()
	tmpFile = filepath.Join(t.TempDir(), "guard.go")
	before = []byte(syntaxGuardValidGo)
	if err := os.WriteFile(tmpFile, before, 0644); err != nil {
		t.Fatalf("seed fixture: %v", err)
	}
	return tmpFile, before
}

func assertFileUnchanged(t *testing.T, path string, before []byte) {
	t.Helper()
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("fixture unreadable after refused edit: %v", err)
	}
	if string(after) != string(before) {
		t.Errorf("refused edit modified file:\n got: %q\nwant: %q", string(after), string(before))
	}
}

func TestEditLinesSyntaxGuardRefusesUnparseable(t *testing.T) {
	t.Parallel()
	tmpFile, before := writeSyntaxGuardFixture(t)
	_, err := executeEditLines(wsCtxFor(t, tmpFile), map[string]any{
		"path":        tmpFile,
		"start_line":  float64(1),
		"end_line":    float64(1),
		"new_content": "placeholder",
	})
	if err == nil {
		t.Fatalf("edit_lines accepted unparseable Go content")
	}
	assertFileUnchanged(t, tmpFile, before)
}

func TestInsertLinesSyntaxGuardRefusesUnparseable(t *testing.T) {
	t.Parallel()
	tmpFile, before := writeSyntaxGuardFixture(t)
	_, err := executeInsertLines(wsCtxFor(t, tmpFile), map[string]any{
		"path":        tmpFile,
		"line_number": float64(1),
		"content":     "func (",
	})
	if err == nil {
		t.Fatalf("insert_lines accepted unparseable Go content")
	}
	assertFileUnchanged(t, tmpFile, before)
}

func TestDeleteLinesSyntaxGuardRefusesUnparseable(t *testing.T) {
	t.Parallel()
	tmpFile, before := writeSyntaxGuardFixture(t)
	_, err := executeDeleteLines(wsCtxFor(t, tmpFile), map[string]any{
		"path":       tmpFile,
		"start_line": float64(1),
		"end_line":   float64(1),
	})
	if err == nil {
		t.Fatalf("delete_lines accepted deletion that leaves unparseable Go")
	}
	assertFileUnchanged(t, tmpFile, before)
}
