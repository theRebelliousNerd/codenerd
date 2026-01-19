package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// =============================================================================
// READ FILE TOOL TESTS
// =============================================================================

func TestReadFileTool_Definition(t *testing.T) {
	t.Parallel()

	tool := ReadFileTool()

	if tool.Name != "read_file" {
		t.Errorf("Name mismatch: got %q", tool.Name)
	}
	if tool.Description == "" {
		t.Error("Description should not be empty")
	}
	if tool.Execute == nil {
		t.Error("Execute should be set")
	}
}

func TestReadFileTool_Execute_MissingPath(t *testing.T) {
	t.Parallel()

	_, err := executeReadFile(context.Background(), map[string]any{})
	if err == nil {
		t.Error("expected error for missing path")
	}
}

func TestReadFileTool_Execute_FileNotFound(t *testing.T) {
	t.Parallel()

	_, err := executeReadFile(context.Background(), map[string]any{
		"path": "/nonexistent/file.txt",
	})
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestReadFileTool_Execute_Success(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")
	content := "Hello, World!\nSecond line."
	os.WriteFile(tmpFile, []byte(content), 0644)

	result, err := executeReadFile(context.Background(), map[string]any{
		"path": tmpFile,
	})
	if err != nil {
		t.Fatalf("executeReadFile error: %v", err)
	}

	if !strings.Contains(result, "Hello, World!") {
		t.Error("expected result to contain file content")
	}
}

func TestReadFileTool_Execute_WithLineRange(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")
	content := "line1\nline2\nline3\nline4\nline5"
	os.WriteFile(tmpFile, []byte(content), 0644)

	result, err := executeReadFile(context.Background(), map[string]any{
		"path":       tmpFile,
		"start_line": float64(2),
		"end_line":   float64(4),
	})
	if err != nil {
		t.Fatalf("executeReadFile error: %v", err)
	}

	if !strings.Contains(result, "line2") {
		t.Error("expected result to contain line2")
	}
}

// =============================================================================
// WRITE FILE TOOL TESTS
// =============================================================================

func TestWriteFileTool_Definition(t *testing.T) {
	t.Parallel()

	tool := WriteFileTool()

	if tool.Name != "write_file" {
		t.Errorf("Name mismatch: got %q", tool.Name)
	}
}

func TestWriteFileTool_Execute_MissingPath(t *testing.T) {
	t.Parallel()

	_, err := executeWriteFile(context.Background(), map[string]any{
		"content": "test",
	})
	if err == nil {
		t.Error("expected error for missing path")
	}
}

func TestWriteFileTool_Execute_Success(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "new_file.txt")

	result, err := executeWriteFile(context.Background(), map[string]any{
		"path":    tmpFile,
		"content": "Test content",
	})
	if err != nil {
		t.Fatalf("executeWriteFile error: %v", err)
	}

	if !strings.Contains(result, "Wrote") {
		t.Errorf("unexpected result: %s", result)
	}

	// Verify file was created
	content, _ := os.ReadFile(tmpFile)
	if string(content) != "Test content" {
		t.Errorf("file content mismatch: got %q", string(content))
	}
}

func TestWriteFileTool_Execute_CreatesDirs(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "subdir", "nested", "file.txt")

	_, err := executeWriteFile(context.Background(), map[string]any{
		"path":    tmpFile,
		"content": "Nested content",
	})
	if err != nil {
		t.Fatalf("executeWriteFile error: %v", err)
	}

	if _, err := os.Stat(tmpFile); os.IsNotExist(err) {
		t.Error("file should have been created in nested directory")
	}
}

// =============================================================================
// EDIT FILE TOOL TESTS
// =============================================================================

func TestEditFileTool_Definition(t *testing.T) {
	t.Parallel()

	tool := EditFileTool()

	if tool.Name != "edit_file" {
		t.Errorf("Name mismatch: got %q", tool.Name)
	}
}

func TestEditFileTool_Execute_MissingPath(t *testing.T) {
	t.Parallel()

	_, err := executeEditFile(context.Background(), map[string]any{
		"search":  "old",
		"replace": "new",
	})
	if err == nil {
		t.Error("expected error for missing path")
	}
}

func TestEditFileTool_Execute_MissingSearch(t *testing.T) {
	t.Parallel()

	_, err := executeEditFile(context.Background(), map[string]any{
		"path":     "/some/file.txt",
		"new_text": "new",
	})
	if err == nil {
		t.Error("expected error for missing old_text")
	}
}

func TestEditFileTool_Execute_Success(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")
	content := "Hello, OLD, goodbye OLD"
	os.WriteFile(tmpFile, []byte(content), 0644)

	result, err := executeEditFile(context.Background(), map[string]any{
		"path":        tmpFile,
		"old_text":    "OLD",
		"new_text":    "NEW",
		"replace_all": true,
	})
	if err != nil {
		t.Fatalf("executeEditFile error: %v", err)
	}

	if !strings.Contains(result, "2 occurrence") {
		t.Errorf("unexpected result: %s", result)
	}

	newContent, _ := os.ReadFile(tmpFile)
	if !strings.Contains(string(newContent), "NEW") {
		t.Error("file content not updated")
	}
}

func TestEditFileTool_Execute_NoMatch(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(tmpFile, []byte("Hello, World"), 0644)

	_, err := executeEditFile(context.Background(), map[string]any{
		"path":     tmpFile,
		"old_text": "NOTFOUND",
		"new_text": "NEW",
	})
	if err == nil {
		t.Error("expected error when old_text not found")
	}
}

// =============================================================================
// DELETE FILE TOOL TESTS
// =============================================================================

func TestDeleteFileTool_Definition(t *testing.T) {
	t.Parallel()

	tool := DeleteFileTool()

	if tool.Name != "delete_file" {
		t.Errorf("Name mismatch: got %q", tool.Name)
	}
}

func TestDeleteFileTool_Execute_MissingPath(t *testing.T) {
	t.Parallel()

	_, err := executeDeleteFile(context.Background(), map[string]any{})
	if err == nil {
		t.Error("expected error for missing path")
	}
}

func TestDeleteFileTool_Execute_Success(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "to_delete.txt")
	os.WriteFile(tmpFile, []byte("delete me"), 0644)

	result, err := executeDeleteFile(context.Background(), map[string]any{
		"path": tmpFile,
	})
	if err != nil {
		t.Fatalf("executeDeleteFile error: %v", err)
	}

	if !strings.Contains(result, "Deleted") {
		t.Errorf("unexpected result: %s", result)
	}

	if _, err := os.Stat(tmpFile); !os.IsNotExist(err) {
		t.Error("file should have been deleted")
	}
}

func TestDeleteFileTool_Execute_NotFound(t *testing.T) {
	t.Parallel()

	_, err := executeDeleteFile(context.Background(), map[string]any{
		"path": "/nonexistent/file.txt",
	})
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

// =============================================================================
// LIST FILES TOOL TESTS
// =============================================================================

func TestListFilesTool_Definition(t *testing.T) {
	t.Parallel()

	tool := ListFilesTool()

	if tool.Name != "list_files" {
		t.Errorf("Name mismatch: got %q", tool.Name)
	}
}

func TestListFilesTool_Execute_MissingPath(t *testing.T) {
	t.Parallel()

	// list_files path is optional with default ".", so this should succeed
	result, err := executeListFiles(context.Background(), map[string]any{})
	if err != nil {
		t.Errorf("expected no error for missing path (uses default): %v", err)
	}
	_ = result // result depends on current directory
}

func TestListFilesTool_Execute_Success(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	// Create some files
	os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte(""), 0644)
	os.WriteFile(filepath.Join(tmpDir, "file2.go"), []byte(""), 0644)
	os.Mkdir(filepath.Join(tmpDir, "subdir"), 0755)

	result, err := executeListFiles(context.Background(), map[string]any{
		"path": tmpDir,
	})
	if err != nil {
		t.Fatalf("executeListFiles error: %v", err)
	}

	if !strings.Contains(result, "file1.txt") {
		t.Error("expected to find file1.txt in listing")
	}
	if !strings.Contains(result, "subdir") {
		t.Error("expected to find subdir in listing")
	}
}

func TestListFilesTool_Execute_NotFound(t *testing.T) {
	t.Parallel()

	_, err := executeListFiles(context.Background(), map[string]any{
		"path": "/nonexistent/directory",
	})
	if err == nil {
		t.Error("expected error for nonexistent directory")
	}
}
