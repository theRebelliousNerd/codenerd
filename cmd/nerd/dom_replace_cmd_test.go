package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestCollectReplaceFilesOneHopDerivesOpenPermission(t *testing.T) {
	workspace := t.TempDir()
	root := filepath.Join(workspace, "main.go")
	if err := os.WriteFile(filepath.Join(workspace, "go.mod"), []byte("module example.com/domreplace\n\ngo 1.22\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(root, []byte("package main\n\nfunc main() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	previousScope := domReplaceScope
	previousIncludeTests := domReplaceIncludeTest
	domReplaceScope = "one-hop"
	domReplaceIncludeTest = false
	t.Cleanup(func() {
		domReplaceScope = previousScope
		domReplaceIncludeTest = previousIncludeTests
	})

	files, err := collectReplaceFiles(context.Background(), workspace, root)
	if err != nil {
		t.Fatalf("one-hop CodeDOM scope failed: %v", err)
	}
	if len(files) != 1 || files[0] != root {
		t.Fatalf("files = %v, want [%s]", files, root)
	}
}
