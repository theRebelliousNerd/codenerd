package codedom

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// =============================================================================
// EDIT LINES TOOL TESTS
// =============================================================================

func TestEditLinesTool_Definition(t *testing.T) {
	t.Parallel()

	tool := EditLinesTool()

	if tool.Name != "edit_lines" {
		t.Errorf("Name mismatch: got %q", tool.Name)
	}
	if tool.Execute == nil {
		t.Error("Execute should be set")
	}
	if len(tool.Schema.Required) != 4 {
		t.Errorf("expected 4 required fields, got %d", len(tool.Schema.Required))
	}
}

func TestEditLinesTool_Execute_MissingPath(t *testing.T) {
	t.Parallel()

	_, err := executeEditLines(context.Background(), map[string]any{
		"start_line":  1,
		"end_line":    2,
		"new_content": "test",
	})
	if err == nil {
		t.Error("expected error for missing path")
	}
}

func TestEditLinesTool_Execute_MissingStartLine(t *testing.T) {
	t.Parallel()

	_, err := executeEditLines(context.Background(), map[string]any{
		"path":        "/some/file.txt",
		"end_line":    2,
		"new_content": "test",
	})
	if err == nil {
		t.Error("expected error for missing start_line")
	}
}

func TestEditLinesTool_Execute_MissingEndLine(t *testing.T) {
	t.Parallel()

	_, err := executeEditLines(context.Background(), map[string]any{
		"path":        "/some/file.txt",
		"start_line":  1,
		"new_content": "test",
	})
	if err == nil {
		t.Error("expected error for missing end_line")
	}
}

func TestEditLinesTool_Execute_Success(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")
	content := "line1\nline2\nline3\nline4\nline5"
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	result, err := executeEditLines(context.Background(), map[string]any{
		"path":        tmpFile,
		"start_line":  float64(2), // JSON numbers are float64
		"end_line":    float64(3),
		"new_content": "replaced2\nreplaced3",
	})
	if err != nil {
		t.Fatalf("executeEditLines error: %v", err)
	}

	if !strings.Contains(result, "Replaced") {
		t.Errorf("unexpected result: %s", result)
	}

	// Verify file contents
	newContent, _ := os.ReadFile(tmpFile)
	if !strings.Contains(string(newContent), "replaced2") {
		t.Error("file content not updated correctly")
	}
}

func TestEditLinesTool_Execute_InvalidRange(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")
	content := "line1\nline2\nline3"
	os.WriteFile(tmpFile, []byte(content), 0644)

	// Start line out of range
	_, err := executeEditLines(context.Background(), map[string]any{
		"path":        tmpFile,
		"start_line":  float64(10),
		"end_line":    float64(11),
		"new_content": "test",
	})
	if err == nil {
		t.Error("expected error for out of range start_line")
	}
}

// =============================================================================
// INSERT LINES TOOL TESTS
// =============================================================================

func TestInsertLinesTool_Definition(t *testing.T) {
	t.Parallel()

	tool := InsertLinesTool()

	if tool.Name != "insert_lines" {
		t.Errorf("Name mismatch: got %q", tool.Name)
	}
	if len(tool.Schema.Required) != 3 {
		t.Errorf("expected 3 required fields, got %d", len(tool.Schema.Required))
	}
}

func TestInsertLinesTool_Execute_MissingPath(t *testing.T) {
	t.Parallel()

	_, err := executeInsertLines(context.Background(), map[string]any{
		"after_line": 1,
		"content":    "test",
	})
	if err == nil {
		t.Error("expected error for missing path")
	}
}

func TestInsertLinesTool_Execute_MissingContent(t *testing.T) {
	t.Parallel()

	_, err := executeInsertLines(context.Background(), map[string]any{
		"path":       "/some/file.txt",
		"after_line": 1,
	})
	if err == nil {
		t.Error("expected error for missing content")
	}
}

func TestInsertLinesTool_Execute_Success(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")
	content := "line1\nline2\nline3"
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	result, err := executeInsertLines(context.Background(), map[string]any{
		"path":       tmpFile,
		"after_line": float64(1),
		"content":    "inserted",
	})
	if err != nil {
		t.Fatalf("executeInsertLines error: %v", err)
	}

	if !strings.Contains(result, "Inserted") {
		t.Errorf("unexpected result: %s", result)
	}

	// Verify file contents
	newContent, _ := os.ReadFile(tmpFile)
	lines := strings.Split(string(newContent), "\n")
	if len(lines) != 4 {
		t.Errorf("expected 4 lines, got %d", len(lines))
	}
	if lines[1] != "inserted" {
		t.Errorf("inserted line not at correct position: %v", lines)
	}
}

func TestInsertLinesTool_Execute_AtBeginning(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")
	content := "line1\nline2"
	os.WriteFile(tmpFile, []byte(content), 0644)

	_, err := executeInsertLines(context.Background(), map[string]any{
		"path":       tmpFile,
		"after_line": float64(0), // Insert at beginning
		"content":    "first_line",
	})
	if err != nil {
		t.Fatalf("executeInsertLines error: %v", err)
	}

	newContent, _ := os.ReadFile(tmpFile)
	if !strings.HasPrefix(string(newContent), "first_line") {
		t.Error("content not inserted at beginning")
	}
}

// =============================================================================
// DELETE LINES TOOL TESTS
// =============================================================================

func TestDeleteLinesTool_Definition(t *testing.T) {
	t.Parallel()

	tool := DeleteLinesTool()

	if tool.Name != "delete_lines" {
		t.Errorf("Name mismatch: got %q", tool.Name)
	}
	if len(tool.Schema.Required) != 3 {
		t.Errorf("expected 3 required fields, got %d", len(tool.Schema.Required))
	}
}

func TestDeleteLinesTool_Execute_MissingPath(t *testing.T) {
	t.Parallel()

	_, err := executeDeleteLines(context.Background(), map[string]any{
		"start_line": 1,
		"end_line":   2,
	})
	if err == nil {
		t.Error("expected error for missing path")
	}
}

func TestDeleteLinesTool_Execute_Success(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")
	content := "line1\nline2\nline3\nline4"
	os.WriteFile(tmpFile, []byte(content), 0644)

	result, err := executeDeleteLines(context.Background(), map[string]any{
		"path":       tmpFile,
		"start_line": float64(2),
		"end_line":   float64(3),
	})
	if err != nil {
		t.Fatalf("executeDeleteLines error: %v", err)
	}

	if !strings.Contains(result, "Deleted") {
		t.Errorf("unexpected result: %s", result)
	}

	newContent, _ := os.ReadFile(tmpFile)
	lines := strings.Split(string(newContent), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines after deletion, got %d", len(lines))
	}
}

func TestDeleteLinesTool_Execute_InvalidRange(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")
	content := "line1\nline2"
	os.WriteFile(tmpFile, []byte(content), 0644)

	// End line beyond file length
	_, err := executeDeleteLines(context.Background(), map[string]any{
		"path":       tmpFile,
		"start_line": float64(1),
		"end_line":   float64(10),
	})
	if err == nil {
		t.Error("expected error for out of range end_line")
	}
}
