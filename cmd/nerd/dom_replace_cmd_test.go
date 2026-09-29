package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestDomReplaceMaxFilesDefaultIsUnlimited(t *testing.T) {
	flag := domReplaceCmd.Flags().Lookup("max-files")
	if flag == nil {
		t.Fatal("no --max-files flag")
	}
	if flag.DefValue != "0" {
		t.Errorf("--max-files default = %s, want 0 (no cap)", flag.DefValue)
	}
}

func TestDomReplaceOverCap(t *testing.T) {
	prevMax, prevAllow := domReplaceMaxFiles, domReplaceAllowLarge
	t.Cleanup(func() {
		domReplaceMaxFiles = prevMax
		domReplaceAllowLarge = prevAllow
	})

	domReplaceAllowLarge = false
	domReplaceMaxFiles = 0
	if domReplaceOverCap(1000) {
		t.Fatal("--max-files 0 refused a batch")
	}
	domReplaceMaxFiles = -5
	if domReplaceOverCap(1) {
		t.Fatal("negative --max-files refused a batch")
	}
	domReplaceMaxFiles = 2
	if !domReplaceOverCap(3) {
		t.Fatal("--max-files 2 accepted 3 files")
	}
	if domReplaceOverCap(2) {
		t.Fatal("--max-files 2 refused a batch of exactly 2")
	}
	domReplaceAllowLarge = true
	if domReplaceOverCap(100) {
		t.Fatal("--allow-large still refused the batch")
	}
}

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
