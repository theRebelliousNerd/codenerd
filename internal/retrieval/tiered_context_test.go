package retrieval

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestKeywordHitCacheClear(t *testing.T) {
	c := NewKeywordHitCache(8, time.Hour)
	c.Set("kw", []KeywordHit{{FilePath: "a.go"}})
	if _, ok := c.Get("kw"); !ok {
		t.Fatal("entry should be present after Set")
	}
	c.Clear()
	if _, ok := c.Get("kw"); ok {
		t.Error("entry should be gone after Clear")
	}
}

func TestTieredExtractMentionedFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "alpha.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	b := NewTieredContextBuilder(DefaultTieredContextConfig(dir))

	kw := &IssueKeywords{MentionedFiles: []string{"alpha.go"}}
	added := map[string]bool{}
	resolved := map[string]string{}
	files := b.extractMentionedFiles(context.Background(), kw, added, resolved)
	if len(files) != 1 {
		t.Fatalf("expected 1 mentioned file resolved, got %d", len(files))
	}
	if !strings.HasSuffix(files[0].FilePath, "alpha.go") || files[0].Tier != 1 {
		t.Errorf("unexpected mentioned-file entry: %+v", files[0])
	}
	// A second pass with the same addedFiles set must not duplicate.
	again := b.extractMentionedFiles(context.Background(), kw, added, resolved)
	if len(again) != 0 {
		t.Errorf("already-added file should not be returned again, got %d", len(again))
	}
}

func TestTieredBuildContextEndToEnd(t *testing.T) {
	dir := t.TempDir()
	src := "package main\n\nfunc handle_error() error { return nil }\n"
	if err := os.WriteFile(filepath.Join(dir, "alpha.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	b := NewTieredContextBuilder(DefaultTieredContextConfig(dir))

	// Crosses ExtractKeywords -> sparse search -> tiered assembly.
	tc, err := b.BuildContext(context.Background(), "The handle_error() function in alpha.go misbehaves")
	if err != nil {
		t.Fatalf("BuildContext error: %v", err)
	}
	if tc == nil || tc.Keywords == nil {
		t.Fatal("expected a populated TieredContext with keywords")
	}
	if tc.TotalFiles != len(tc.Files) {
		t.Errorf("TotalFiles=%d but len(Files)=%d", tc.TotalFiles, len(tc.Files))
	}
	found := false
	for _, f := range tc.Files {
		if strings.HasSuffix(f.FilePath, "alpha.go") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected alpha.go among tiered context files, got %+v", tc.Files)
	}
}
