package codedom

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/tools"
)

func TestCodedomRead_DirectoryReturnsActionableError(t *testing.T) {
	dir := t.TempDir()
	subdir := filepath.Join(dir, "mydir")
	if err := os.Mkdir(subdir, 0755); err != nil {
		t.Fatalf("seed dir: %v", err)
	}

	// extractCodeElements is the direct read path for get_elements/get_element.
	_, err := extractCodeElements(subdir)
	if err == nil {
		t.Fatal("expected error for directory, got nil from extractCodeElements")
	}
	assertActionableDirectoryError(t, err)

	// executeGetElements wraps extractCodeElements and should propagate the actionable error.
	_, err = executeGetElements(context.Background(), map[string]any{"path": subdir})
	if err == nil {
		t.Fatal("expected error for directory, got nil from executeGetElements")
	}
	assertActionableDirectoryError(t, err)

	// executeGetElement also goes through extractCodeElements.
	_, err = executeGetElement(context.Background(), map[string]any{"path": subdir, "name": "Foo"})
	if err == nil {
		t.Fatal("expected error for directory, got nil from executeGetElement")
	}
	assertActionableDirectoryError(t, err)

	// Line tools use projectdoc.ReadFileForTool after workspace containment.
	// Use a workspace context so the path is considered inside the workspace.
	ctx := context.WithValue(context.Background(), tools.CtxKeyWorkspaceRoot, dir)

	_, err = executeEditLines(ctx, map[string]any{
		"path":        subdir,
		"start_line":  1,
		"end_line":    1,
		"new_content": "x",
	})
	if err == nil {
		t.Fatal("expected error for directory, got nil from executeEditLines")
	}
	assertActionableDirectoryError(t, err)

	_, err = executeInsertLines(ctx, map[string]any{
		"path":       subdir,
		"after_line": 0,
		"content":    "x",
	})
	if err == nil {
		t.Fatal("expected error for directory, got nil from executeInsertLines")
	}
	assertActionableDirectoryError(t, err)

	_, err = executeDeleteLines(ctx, map[string]any{
		"path":       subdir,
		"start_line": 1,
		"end_line":   1,
	})
	if err == nil {
		t.Fatal("expected error for directory, got nil from executeDeleteLines")
	}
	assertActionableDirectoryError(t, err)
}

func assertActionableDirectoryError(t *testing.T, err error) {
	t.Helper()
	msg := err.Error()
	lower := strings.ToLower(msg)
	if !strings.Contains(lower, "directory") {
		t.Errorf("expected error to mention directory, got: %v", err)
	}
	if !strings.Contains(msg, "list_files") {
		t.Errorf("expected error to mention list_files, got: %v", err)
	}
	if !strings.Contains(msg, "glob") {
		t.Errorf("expected error to mention glob, got: %v", err)
	}
	if strings.Contains(msg, "Incorrect function") {
		t.Errorf("error should not be raw Windows platform error 'Incorrect function', got: %v", err)
	}
	// Also ensure it's not the raw Unix "is a directory" without guidance.
	// The actionable error must contain guidance, not just the syscall text.
	if lower == "is a directory" || msg == "is a directory" {
		t.Errorf("error should be actionable, not raw 'is a directory', got: %v", err)
	}
}
