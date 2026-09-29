package retrieval

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRipgrepRelativeRootIncludesHiddenMembers(t *testing.T) {
	backend, err := NewRipgrepBackend()
	if err != nil {
		t.Skip(err)
	}
	parent := t.TempDir()
	root := filepath.Join(parent, "repo")
	for _, rel := range []string{"plain.go", ".hidden/member.go", "node_modules/excluded.go"} {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("package p\n// membershipneedle\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(parent)
	hits, err := backend.Search(context.Background(), "repo", "membershipneedle", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 2 {
		t.Fatalf("hits = %v, want plain and hidden members", hits)
	}
	for _, hit := range hits {
		if !filepath.IsAbs(hit.FilePath) || strings.Contains(hit.FilePath, "node_modules") {
			t.Errorf("unexpected hit: %+v", hit)
		}
	}
}
