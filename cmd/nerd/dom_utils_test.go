package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRunGoFmtFiles(t *testing.T) {
	ctx := context.Background()

	tempDir := t.TempDir()
	exploitFile := filepath.Join(tempDir, "exploit.txt")
	testFile := filepath.Join(tempDir, "test.go")
	os.WriteFile(testFile, []byte("package main\n\nfunc main() {}\n"), 0644)

	// Verify that filenames starting with "-" are correctly handled without injecting flags
	err := runGoFmtFiles(ctx, tempDir, []string{"-cpuprofile=" + exploitFile, testFile})

	// gofmt will fail because "-cpuprofile=..." as a file path doesn't exist
	if err == nil {
		t.Fatalf("Expected runGoFmtFiles to fail due to missing file, but it succeeded")
	}

	// Confirm exploit file was not created by gofmt command injection
	if _, statErr := os.Stat(exploitFile); !os.IsNotExist(statErr) {
		t.Fatalf("Vulnerability triggered: exploit file %s was created", exploitFile)
	}
}

// The complement of the case above: a file whose name legitimately begins with
// a dash must still be formatted, not rejected. A guard that blocks the exploit
// by refusing every dash-leading name would pass the test above and break this.
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
