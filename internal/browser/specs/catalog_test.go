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

func TestCatalogDedupesIndexAndRootsAndBoundsMatchExcerpts(t *testing.T) {
	root := t.TempDir()
	specRoot := filepath.Join(root, "specs")
	if err := os.MkdirAll(specRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	filler := strings.Repeat("lorem ipsum dolor sit amet ", 200)
	body := "# Same\n\n" + filler + " needle-term\n"
	if err := os.WriteFile(filepath.Join(specRoot, "a.md"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(specRoot, "b.md"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	indexContent := "[A](specs/a.md)\n[A again](specs/a.md)\n[B](specs/b.md)\n"
	if err := os.WriteFile(filepath.Join(root, "catalog.md"), []byte(indexContent), 0o600); err != nil {
		t.Fatal(err)
	}
	catalog, err := NewCatalog(root, Config{Sources: []Source{{Name: "product", Roots: []string{"specs"}, Indexes: []string{"catalog.md"}}}})
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := catalog.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Specs) != 2 {
		t.Fatalf("deduped load specs = %+v", loaded)
	}
	if loaded.Truncated {
		t.Fatalf("deduped load should not truncate: %+v", loaded)
	}
	if loaded.Specs[0].Path != "specs/a.md" || loaded.Specs[1].Path != "specs/b.md" {
		t.Fatalf("index-first order = %q, %q", loaded.Specs[0].Path, loaded.Specs[1].Path)
	}
	if loaded.EntriesScanned < 3 {
		t.Fatalf("entries scanned = %d, want at least index links", loaded.EntriesScanned)
	}
	if loaded.FilesScanned != 2 || loaded.BytesLoaded <= 0 {
		t.Fatalf("files scanned = %d bytes = %d", loaded.FilesScanned, loaded.BytesLoaded)
	}
	unfiltered := MatchSpecs(loaded.Specs, MatchInput{}, 600)
	if len(unfiltered) != 2 || unfiltered[0].Path != "specs/a.md" || unfiltered[1].Path != "specs/b.md" {
		t.Fatalf("stable tie-break matches = %+v", unfiltered)
	}
	again := MatchSpecs(loaded.Specs, MatchInput{}, 600)
	if again[0].Path != unfiltered[0].Path || again[1].Path != unfiltered[1].Path {
		t.Fatalf("unstable match order: %+v vs %+v", unfiltered, again)
	}
	capped := MatchSpecs(loaded.Specs, MatchInput{Max: 1}, 600)
	if len(capped) != 1 {
		t.Fatalf("capped matches = %+v", capped)
	}
	if got := CountMatchingSpecs(loaded.Specs, MatchInput{Max: 1}); got != 2 {
		t.Fatalf("count before cap = %d, want 2", got)
	}
	bounded := MatchSpecs(loaded.Specs, MatchInput{}, 64)
	if len(bounded) != 2 {
		t.Fatalf("bounded matches = %+v", bounded)
	}
	for _, match := range bounded {
		if len(match.Excerpt) > 64 {
			t.Fatalf("excerpt exceeds bound: %d bytes", len(match.Excerpt))
		}
		if len(match.Summary) > 32 {
			t.Fatalf("summary exceeds half bound: %d bytes", len(match.Summary))
		}
	}
}

func TestMatchSpecsDeterministicTiebreakCorpusAndName(t *testing.T) {
	specs := []Spec{
		{Path: "same.md", Corpus: "b", Name: "second"},
		{Path: "same.md", Corpus: "a", Name: "zeta"},
		{Path: "same.md", Corpus: "a", Name: "alpha"},
	}
	got := MatchSpecs(specs, MatchInput{}, 1200)
	if len(got) != 3 {
		t.Fatalf("expected 3 matches, got %d", len(got))
	}
	if got[0].Name != "alpha" || got[0].Corpus != "a" {
		t.Fatalf("expected first match alpha/corpus-a, got %q/%q", got[0].Name, got[0].Corpus)
	}
	if got[1].Name != "zeta" || got[1].Corpus != "a" {
		t.Fatalf("expected second match zeta/corpus-a, got %q/%q", got[1].Name, got[1].Corpus)
	}
	if got[2].Name != "second" || got[2].Corpus != "b" {
		t.Fatalf("expected third match second/corpus-b, got %q/%q", got[2].Name, got[2].Corpus)
	}
}

func TestCatalogLoadNilContextUsesBackground(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "specs"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "specs", "a.md"), []byte("# Hello\n\nBody.\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg := DefaultConfig()
	cfg.Sources = []Source{{Name: "test", Roots: []string{"specs"}}}
	cat, err := NewCatalog(dir, cfg)
	if err != nil {
		t.Fatal(err)
	}
	result, err := cat.Load(nil)
	if err != nil {
		t.Fatalf("Load(nil) error = %v", err)
	}
	if len(result.Specs) == 0 && result.FilesScanned == 0 {
		t.Fatalf("Load(nil) loaded nothing: %+v", result)
	}
	bgResult, err := cat.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(bgResult.Specs) != len(result.Specs) {
		t.Fatalf("Load(nil) specs %d != Load(Background) specs %d", len(result.Specs), len(bgResult.Specs))
	}
}

func TestCatalogCollectSourceNilContextAndGuards(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "specs"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "specs", "a.md"), []byte("# Hello\n\nBody.\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg := DefaultConfig()
	cfg.Sources = []Source{{Name: "test", Roots: []string{"specs"}}}
	cat, err := NewCatalog(dir, cfg)
	if err != nil {
		t.Fatal(err)
	}
	src := Source{Name: "test", Roots: []string{"specs"}}

	candidates := []specCandidate{}
	seen := map[string]struct{}{}
	scanned := 0
	truncated, _ := cat.collectSource(nil, src, &candidates, seen, 10, &scanned, 1000)
	if len(candidates) == 0 {
		t.Fatalf("collectSource(nil ctx) collected nothing (truncated=%v scanned=%d)", truncated, scanned)
	}
	bgCandidates := []specCandidate{}
	bgSeen := map[string]struct{}{}
	bgScanned := 0
	bgTruncated, _ := cat.collectSource(context.Background(), src, &bgCandidates, bgSeen, 10, &bgScanned, 1000)
	if len(bgCandidates) != len(candidates) {
		t.Fatalf("collectSource(nil) candidates %d != Background candidates %d", len(candidates), len(bgCandidates))
	}
	if truncated != bgTruncated {
		t.Fatalf("collectSource(nil) truncated %v != Background truncated %v", truncated, bgTruncated)
	}

	for _, tc := range []struct {
		name  string
		cands *[]specCandidate
		seen  map[string]struct{}
		scan  *int
	}{
		{"nil candidates", nil, map[string]struct{}{}, func() *int { v := 0; return &v }()},
		{"nil seen", &[]specCandidate{}, nil, func() *int { v := 0; return &v }()},
		{"nil scanned", &[]specCandidate{}, map[string]struct{}{}, nil},
	} {
		truncated, warnings := cat.collectSource(context.Background(), src, tc.cands, tc.seen, 10, tc.scan, 1000)
		if !truncated {
			t.Fatalf("%s: expected truncated=true", tc.name)
		}
		found := false
		for _, w := range warnings {
			if strings.Contains(w, "state is unavailable") {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("%s: expected state-unavailable warning, got %v", tc.name, warnings)
		}
	}

	cands := []specCandidate{}
	seen2 := map[string]struct{}{}
	scan2 := 0
	truncated, warnings := cat.collectSource(context.Background(), src, &cands, seen2, 0, &scan2, 1000)
	if !truncated {
		t.Fatalf("zero limit: expected truncated=true")
	}
	if len(warnings) != 0 {
		t.Fatalf("zero limit: expected no warnings, got %v", warnings)
	}
	cands = []specCandidate{}
	seen2 = map[string]struct{}{}
	scan2 = 0
	truncated, warnings = cat.collectSource(context.Background(), src, &cands, seen2, 10, &scan2, 0)
	if !truncated {
		t.Fatalf("zero maxEntries: expected truncated=true")
	}
	if len(warnings) != 0 {
		t.Fatalf("zero maxEntries: expected no warnings, got %v", warnings)
	}
}

func TestCatalogResolveReadPathUnavailableWorkspace(t *testing.T) {
	var nilCat *Catalog
	if _, _, err := nilCat.resolveReadPath("a.md"); err == nil || !strings.Contains(err.Error(), "workspace is unavailable") {
		t.Fatalf("nil catalog: expected workspace-unavailable error, got %v", err)
	}
	empty := &Catalog{}
	if _, _, err := empty.resolveReadPath("a.md"); err == nil || !strings.Contains(err.Error(), "workspace is unavailable") {
		t.Fatalf("empty workspace: expected workspace-unavailable error, got %v", err)
	}
	blank := &Catalog{workspace: "   "}
	if _, _, err := blank.resolveReadPath("a.md"); err == nil || !strings.Contains(err.Error(), "workspace is unavailable") {
		t.Fatalf("blank workspace: expected workspace-unavailable error, got %v", err)
	}
}
