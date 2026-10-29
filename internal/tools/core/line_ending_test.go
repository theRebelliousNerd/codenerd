package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/tools"
)

func assertToolFileCRLFOnly(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lfCount := strings.Count(string(data), "\n")
	crlfCount := strings.Count(string(data), "\r\n")
	if crlfCount == 0 || lfCount != crlfCount {
		t.Fatalf("expected CRLF-only file, got lf=%d crlf=%d", lfCount, crlfCount)
	}
	return string(data)
}

func TestExecuteWriteFilePreservesCRLF(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "write.go")
	if err := os.WriteFile(path, []byte("package p\r\n\r\nfunc A() {}\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := context.WithValue(context.Background(), tools.CtxKeyWorkspaceRoot, dir)
	if _, err := executeWriteFile(ctx, map[string]any{
		"path":    filepath.Base(path),
		"content": "package p\n\nfunc B() {}\n",
	}); err != nil {
		t.Fatal(err)
	}
	assertToolFileCRLFOnly(t, path)
}

func TestExecuteEditFileMatchesMultilineLFInCRLFFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "edit.go")
	if err := os.WriteFile(path, []byte("package p\r\n\r\nfunc A() {}\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := context.WithValue(context.Background(), tools.CtxKeyWorkspaceRoot, dir)
	if _, err := executeEditFile(ctx, map[string]any{
		"path":     filepath.Base(path),
		"old_text": "package p\n\nfunc A() {}",
		"new_text": "package p\n\nfunc B() {\n\treturn\n}",
	}); err != nil {
		t.Fatal(err)
	}
	got := assertToolFileCRLFOnly(t, path)
	if !strings.Contains(got, "func B() {\r\n\treturn\r\n}") {
		t.Fatalf("multi-line edit did not land:\n%s", got)
	}
}
