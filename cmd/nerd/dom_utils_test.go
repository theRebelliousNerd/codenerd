package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRunGoFmtFiles_HyphenFile(t *testing.T) {
	tmpDir := t.TempDir()

	file1 := filepath.Join(tmpDir, "-someflag.go")
	content := []byte("package main\nfunc main(){}\n")
	if err := os.WriteFile(file1, content, 0644); err != nil {
		t.Fatalf("failed to write file1: %v", err)
	}

	file2 := filepath.Join(tmpDir, "normal.go")
	if err := os.WriteFile(file2, content, 0644); err != nil {
		t.Fatalf("failed to write file2: %v", err)
	}

	files := []string{file1, file2}
	err := runGoFmtFiles(context.Background(), tmpDir, files)
	if err != nil {
		t.Fatalf("runGoFmtFiles failed: %v", err)
	}

	for _, f := range files {
		out, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("failed to read %s: %v", f, err)
		}
		expected := "package main\n\nfunc main() {}\n"
		if string(out) != expected {
			t.Errorf("file %s not formatted correctly: got %q, want %q", f, string(out), expected)
		}
	}
}
