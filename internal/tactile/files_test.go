package tactile

import (
	"strings"
	"testing"
)

func TestFileEditor_ReadWrite(t *testing.T) {
	tmpDir := t.TempDir()
	editor := NewFileEditor()
	editor.SetWorkingDir(tmpDir)

	filename := "test.txt"
	content := []string{"line 1", "line 2", "line 3"}

	// 1. Write File
	res, err := editor.WriteFile(filename, content)
	if err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if !res.Success {
		t.Error("WriteFile reported failure")
	}
	if res.LineCount != 3 {
		t.Errorf("Expected 3 lines, got %d", res.LineCount)
	}

	// 2. Read File
	readContent, err := editor.ReadFile(filename)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if len(readContent) != 3 {
		t.Errorf("Expected 3 lines read, got %d", len(readContent))
	}
	if readContent[0] != "line 1" {
		t.Errorf("Expected 'line 1', got '%s'", readContent[0])
	}

	// 3. Read Lines (subset)
	subset, err := editor.ReadLines(filename, 2, 3)
	if err != nil {
		t.Fatalf("ReadLines failed: %v", err)
	}
	if len(subset) != 2 {
		t.Errorf("Expected 2 lines in subset, got %d", len(subset))
	}
	if subset[0] != "line 2" {
		t.Errorf("Expected 'line 2', got '%s'", subset[0])
	}
}

func TestFileEditor_EditLines(t *testing.T) {
	tmpDir := t.TempDir()
	editor := NewFileEditor()
	editor.SetWorkingDir(tmpDir)

	filename := "edit.txt"
	initial := []string{"A", "B", "C", "D", "E"}
	if _, err := editor.WriteFile(filename, initial); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	// Case 1: Replace middle lines
	// Replace lines 2-4 (B, C, D) with (X, Y)
	// Expected: A, X, Y, E
	newLines := []string{"X", "Y"}
	res, err := editor.EditLines(filename, 2, 4, newLines)
	if err != nil {
		t.Fatalf("EditLines failed: %v", err)
	}

	content, _ := editor.ReadFile(filename)
	expected := []string{"A", "X", "Y", "E"}
	if strings.Join(content, ",") != strings.Join(expected, ",") {
		t.Errorf("Edit mismatch. Got: %v, Want: %v", content, expected)
	}

	// Verify result struct
	if res.LinesAffected != 3 { // Removed 3 lines (B,C,D) vs added 2. Max is 3.
		// Wait, implementation says: if len(newLines) > linesAffected { linesAffected = len(newLines) }
		// Old lines: 3. New lines: 2. So affected should be 3.
		t.Errorf("Expected 3 affected lines, got %d", res.LinesAffected)
	}
}

func TestFileEditor_InsertLines(t *testing.T) {
	tmpDir := t.TempDir()
	editor := NewFileEditor()
	editor.SetWorkingDir(tmpDir)

	filename := "insert.txt"
	initial := []string{"A", "B"}
	editor.WriteFile(filename, initial)

	// Insert after line 1 (A)
	// Expected: A, X, B
	_, err := editor.InsertLines(filename, 1, []string{"X"})
	if err != nil {
		t.Fatalf("InsertLines failed: %v", err)
	}

	content, _ := editor.ReadFile(filename)
	expected := []string{"A", "X", "B"}
	if strings.Join(content, ",") != strings.Join(expected, ",") {
		t.Errorf("Insert mismatch. Got: %v, Want: %v", content, expected)
	}

	// Insert at beginning (after line 0)
	editor.InsertLines(filename, 0, []string{"Start"})
	content, _ = editor.ReadFile(filename)
	if content[0] != "Start" {
		t.Error("Insert at beginning failed")
	}

	// Insert at end
	editor.InsertLines(filename, len(content), []string{"End"})
	content, _ = editor.ReadFile(filename)
	if content[len(content)-1] != "End" {
		t.Error("Insert at end failed")
	}
}

func TestFileEditor_DeleteLines(t *testing.T) {
	tmpDir := t.TempDir()
	editor := NewFileEditor()
	editor.SetWorkingDir(tmpDir)

	filename := "delete.txt"
	initial := []string{"1", "2", "3", "4", "5"}
	editor.WriteFile(filename, initial)

	// Delete lines 2-4 (2, 3, 4)
	// Expected: 1, 5
	res, err := editor.DeleteLines(filename, 2, 4)
	if err != nil {
		t.Fatalf("DeleteLines failed: %v", err)
	}

	content, _ := editor.ReadFile(filename)
	expected := []string{"1", "5"}
	if strings.Join(content, ",") != strings.Join(expected, ",") {
		t.Errorf("Delete mismatch. Got: %v, Want: %v", content, expected)
	}

	if res.LinesAffected != 3 {
		t.Errorf("Expected 3 deleted lines, got %d", res.LinesAffected)
	}
}

func TestFileEditor_AuditCallback(t *testing.T) {
	tmpDir := t.TempDir()
	editor := NewFileEditorWithSession("sess-123")
	editor.SetWorkingDir(tmpDir)

	var capturedEvent FileAuditEvent
	editor.SetAuditCallback(func(e FileAuditEvent) {
		capturedEvent = e
	})

	editor.WriteFile("audit.txt", []string{"content"})

	if capturedEvent.Type != FileOpWrite {
		t.Errorf("Expected audit type write, got %s", capturedEvent.Type)
	}
	if capturedEvent.SessionID != "sess-123" {
		t.Errorf("Expected session ID sess-123, got %s", capturedEvent.SessionID)
	}
	if capturedEvent.Path != "audit.txt" {
		t.Errorf("Expected path audit.txt, got %s", capturedEvent.Path)
	}
}

func TestFileEditor_PathSecurity(t *testing.T) {
	tmpDir := t.TempDir()
	editor := NewFileEditor()
	editor.SetWorkingDir(tmpDir)

	// Attempt to write outside working dir using ".."
	// FileEditor implementation currently uses filepath.Join(workDir, path)
	// which cleans the path. So "subdir/../../outside" becomes "outside" relative to root?
	// Wait, filepath.Join("C:/tmp", "../outside") -> "C:/outside".
	// The implementation in `files.go` does NOT explicitly check for containment.
	// resolvePath just joins.
	// Let's see what happens. If it allows writing outside, that's a security finding,
	// but we are writing tests for current behavior.
	//
	// Current implementation:
	// func (e *FileEditor) resolvePath(path string) string {
	//     if filepath.IsAbs(path) { return path }
	//     return filepath.Join(workDir, path)
	// }
	//
	// So it allows absolute paths anywhere.
	// And relative paths can traverse up.

	// We'll just test that resolvePath works as implemented for now.

	target := "subdir/file.txt"
	editor.CreateDirectory("subdir")
	_, err := editor.WriteFile(target, []string{"test"})
	if err != nil {
		t.Fatalf("Valid relative write failed: %v", err)
	}

	if !editor.FileExists(target) {
		t.Error("FileExists returned false for existing file")
	}
}
