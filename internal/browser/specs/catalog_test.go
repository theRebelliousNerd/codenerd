package specs

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCatalogLoadsIndexFirstRanksAndSelects(t *testing.T) {
	root := t.TempDir()
	specRoot := filepath.Join(root, "docs", "specs")
	indexRoot := filepath.Join(root, "docs", "indexes")
	if err := os.MkdirAll(specRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(indexRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(specRoot, "login.md"), []byte(sampleSpec), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(specRoot, "billing.md"), []byte("---\ntitle: Billing\ntags: [invoice]\n---\n# Billing\nInvoices only."), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(indexRoot, "catalog.md"), []byte("[Login](../specs/login.md)"), 0o600); err != nil {
		t.Fatal(err)
	}
	catalog, err := NewCatalog(root, Config{Sources: []Source{{
		Name: "product", Roots: []string{"docs/specs"}, Indexes: []string{"docs/indexes/catalog.md"},
		Include: []string{"**/*.md"}, Exclude: []string{"**/archive/**"},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := catalog.Load(context.Background())
	if err != nil || len(loaded.Specs) != 2 || loaded.Specs[0].Name != "Login form" {
		t.Fatalf("loaded catalog = %+v, %v", loaded, err)
	}
	matches := MatchSpecs(loaded.Specs, MatchInput{Route: "/login/reset", Terms: []string{"credentials"}}, 600)
	if len(matches) != 1 || matches[0].Name != "Login form" || matches[0].Score < 100 || matches[0].Excerpt == "" {
		t.Fatalf("ranked matches = %+v", matches)
	}
	invariants, truncated := SelectInvariants(loaded.Specs, MatchInput{File: "src/utils/validate.ts", From: 10, To: 12}, 100)
	if truncated || len(invariants) != 1 || invariants[0].Name != "validation-no-error" {
		t.Fatalf("selected invariants = %+v truncated=%v", invariants, truncated)
	}
}

func TestCatalogBoundsFilesAndRejectsWorkspaceEscapes(t *testing.T) {
	root := t.TempDir()
	specRoot := filepath.Join(root, "specs")
	if err := os.Mkdir(specRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 3; index++ {
		name := filepath.Join(specRoot, string(rune('a'+index))+".md")
		if err := os.WriteFile(name, []byte("# bounded"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	catalog, err := NewCatalog(root, Config{Sources: []Source{{Name: "bounded", Roots: []string{"specs"}}}, MaxFiles: 2})
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := catalog.Load(context.Background())
	if err != nil || len(loaded.Specs) != 2 || !loaded.Truncated {
		t.Fatalf("bounded load = %+v, %v", loaded, err)
	}

	out := t.TempDir()
	escape, err := NewCatalog(root, Config{Sources: []Source{{Name: "escape", Roots: []string{out}}}})
	if err != nil {
		t.Fatal(err)
	}
	escaped, err := escape.Load(context.Background())
	if err != nil || len(escaped.Specs) != 0 || len(escaped.Warnings) == 0 || !strings.Contains(escaped.Warnings[0], "outside workspace") {
		t.Fatalf("escape load = %+v, %v", escaped, err)
	}
}

func TestCatalogRejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.md"), []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "linked")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	catalog, err := NewCatalog(root, Config{Sources: []Source{{Name: "linked", Roots: []string{"linked"}}}})
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := catalog.Load(context.Background())
	if err != nil || len(loaded.Specs) != 0 || len(loaded.Warnings) == 0 {
		t.Fatalf("symlink escape load = %+v, %v", loaded, err)
	}
}

func TestConfigNormalizationHardCaps(t *testing.T) {
	config := (Config{MaxFiles: 999999, MaxFileBytes: 1 << 30, MaxResults: 999, MaxExcerptBytes: 1 << 20}).Normalize()
	if config.MaxFiles != hardMaxFiles || config.MaxFileBytes != hardMaxFileBytes || config.MaxResults != hardMaxResults || config.MaxExcerptBytes != hardMaxExcerptBytes {
		t.Fatalf("normalized config = %+v", config)
	}
	if len(config.Sources) != 1 || config.Sources[0].Roots[0] != DefaultRoot {
		t.Fatalf("default source = %+v", config.Sources)
	}
}

func TestCatalogBoundsDirectoryTraversalEntries(t *testing.T) {
	root := t.TempDir()
	specRoot := filepath.Join(root, "wide")
	if err := os.Mkdir(specRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 25; index++ {
		name := filepath.Join(specRoot, "entry-"+string(rune('a'+index))+".txt")
		if err := os.WriteFile(name, []byte("ignored"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(specRoot, "z.md"), []byte("# too late"), 0o600); err != nil {
		t.Fatal(err)
	}
	catalog, err := NewCatalog(root, Config{Sources: []Source{{Name: "wide", Roots: []string{"wide"}}}, MaxFiles: 1})
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := catalog.Load(context.Background())
	if err != nil || !loaded.Truncated || loaded.EntriesScanned <= 20 || len(loaded.Specs) != 0 {
		t.Fatalf("entry-bounded load = %+v, %v", loaded, err)
	}
}
