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
