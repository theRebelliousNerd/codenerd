package core

import (
	"os"
	"path/filepath"
	"testing"
)

// =============================================================================
// Tests for VirtualStore Interface Implementation (ReadRaw, ReadFile, WriteFile)
// =============================================================================

// TestVirtualStore_ReadRaw verifies the ReadRaw method returns raw bytes.
func TestVirtualStore_ReadRaw(t *testing.T) {
	// Create temp file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.yaml")
	content := "key: value\nlist:\n  - item1\n  - item2"

	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	vs := NewVirtualStore(nil)

	// Test ReadRaw
	data, err := vs.ReadRaw(testFile)
	if err != nil {
		t.Fatalf("ReadRaw failed: %v", err)
	}

	if string(data) != content {
		t.Errorf("ReadRaw returned wrong content:\ngot: %q\nwant: %q", string(data), content)
	}
}

// TestVirtualStore_ReadRaw_NotFound verifies ReadRaw returns error for missing files.
func TestVirtualStore_ReadRaw_NotFound(t *testing.T) {
	vs := NewVirtualStore(nil)

	_, err := vs.ReadRaw("/nonexistent/file.yaml")
	if err == nil {
		t.Error("Expected error for nonexistent file, got nil")
	}
}

// TestVirtualStore_ReadFile_ReturnsLines verifies ReadFile returns lines as slice.
func TestVirtualStore_ReadFile_ReturnsLines(t *testing.T) {
	// Create temp file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := "line1\nline2\nline3"

	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	vs := NewVirtualStore(nil)

	lines, err := vs.ReadFile(testFile)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	if len(lines) != 3 {
		t.Errorf("Expected 3 lines, got %d", len(lines))
	}

	if lines[0] != "line1" || lines[1] != "line2" || lines[2] != "line3" {
		t.Errorf("Lines don't match: %v", lines)
	}
}

// TestVirtualStore_WriteFile_WritesLines verifies WriteFile writes lines correctly.
func TestVirtualStore_WriteFile_WritesLines(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "output.txt")

	vs := NewVirtualStore(nil)

	lines := []string{"first", "second", "third"}
	err := vs.WriteFile(testFile, lines)
	if err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Verify content
	data, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read written file: %v", err)
	}

	expected := "first\nsecond\nthird"
	if string(data) != expected {
		t.Errorf("Written content doesn't match:\ngot: %q\nwant: %q", string(data), expected)
	}
}

// TestVirtualStore_ReadRaw_BinaryFile verifies ReadRaw works with binary content.
func TestVirtualStore_ReadRaw_BinaryFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "binary.bin")

	// Binary content with null bytes
	content := []byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0x00, 0x03}
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("Failed to create binary file: %v", err)
	}

	vs := NewVirtualStore(nil)

	data, err := vs.ReadRaw(testFile)
	if err != nil {
		t.Fatalf("ReadRaw failed for binary file: %v", err)
	}

	if len(data) != len(content) {
		t.Errorf("Binary content length mismatch: got %d, want %d", len(data), len(content))
	}

	for i, b := range content {
		if data[i] != b {
			t.Errorf("Binary byte mismatch at %d: got %x, want %x", i, data[i], b)
		}
	}
}

// TestVirtualStore_ReadRaw_LargeFile verifies ReadRaw handles larger files.
func TestVirtualStore_ReadRaw_LargeFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "large.txt")

	// Create a 1MB file
	size := 1024 * 1024
	content := make([]byte, size)
	for i := range content {
		content[i] = byte(i % 256)
	}

	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("Failed to create large file: %v", err)
	}

	vs := NewVirtualStore(nil)

	data, err := vs.ReadRaw(testFile)
	if err != nil {
		t.Fatalf("ReadRaw failed for large file: %v", err)
	}

	if len(data) != size {
		t.Errorf("Large file size mismatch: got %d, want %d", len(data), size)
	}
}
