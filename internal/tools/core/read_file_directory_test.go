package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadFile_DirectoryReturnsHelpfulError(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("CODENERD_WORKSPACE_ROOT", tmpDir)
	dir := filepath.Join(tmpDir, "subdir")
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatalf("seed dir: %v", err)
	}
	_, err := executeReadFile(context.Background(), map[string]any{"path": dir})
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
	if strings.Contains(msg, "Incorrect function") {
		t.Errorf("error should not be raw Windows platform error 'Incorrect function', got: %v", err)
	}
}

func TestReadFile_DirectoryErrorNotRawPlatform(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("CODENERD_WORKSPACE_ROOT", tmpDir)
	dir := filepath.Join(tmpDir, "another")
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatalf("seed dir: %v", err)
	}
	_, err := executeReadFile(context.Background(), map[string]any{"path": filepath.Join(tmpDir, "another")})
	if err == nil {
		t.Fatal("expected error for directory, got nil")
	}
	// Raw os.ReadFile on a directory would be platform-dependent:
	// Windows: "Incorrect function", Unix: "is a directory" with no guidance.
	// Our helper must wrap with actionable guidance.
	if !strings.Contains(err.Error(), "list_files") || !strings.Contains(strings.ToLower(err.Error()), "directory") {
		t.Fatalf("directory error not actionable, got: %v", err)
	}
}
