package projectdoc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadFileForTool_DirectoryReturnsHelpfulError(t *testing.T) {
	dir := t.TempDir()
	_, err := ReadFileForTool(dir)
	if err == nil {
		t.Fatal("expected error for directory, got nil")
	}
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
	// Must not be the raw platform error.
	if strings.Contains(msg, "Incorrect function") {
		t.Errorf("error should not be raw Windows platform error, got: %v", err)
	}
}

func TestReadFileForTool_FileReturnsBytes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hello.txt")
	want := []byte("hello world\nsecond line")
	if err := os.WriteFile(path, want, 0644); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	got, err := ReadFileForTool(path)
	if err != nil {
		t.Fatalf("ReadFileForTool unexpected error: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("bytes mismatch: got %q want %q", string(got), string(want))
	}
}

func TestReadFileForTool_MissingReturnsNotExist(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nope.txt")
	_, err := ReadFileForTool(path)
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
	if !os.IsNotExist(err) {
		t.Fatalf("expected os.IsNotExist error, got: %v", err)
	}
}
