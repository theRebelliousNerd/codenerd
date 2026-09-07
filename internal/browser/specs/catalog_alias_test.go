package specs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCatalogResolvesAliasBeforeCheckingContainment(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "login.md"), []byte(sampleSpec), 0600); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	catalog, err := NewCatalog(alias, Config{Sources: []Source{{Name: "alias", Roots: []string{"."}}}})
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := catalog.Load(t.Context())
	if err != nil || len(loaded.Specs) != 1 {
		t.Fatalf("physical in-workspace document rejected: %+v %v", loaded, err)
	}
	if _, _, err := catalog.resolveReadPath(filepath.Join(root, "login.md")); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside.md")
	if err := os.WriteFile(outside, []byte(sampleSpec), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := catalog.resolveReadPath(outside); err == nil {
		t.Fatal("outside document accepted")
	}
}
