package prompt

import (
	"strings"
	"testing"
)

func TestLoadEmbeddedCorpus(t *testing.T) {
	corpus, err := LoadEmbeddedCorpus()
	if err != nil {
		t.Fatalf("LoadEmbeddedCorpus failed: %v", err)
	}

	count := corpus.Count()
	if count == 0 {
		t.Error("Expected non-zero atom count, got 0")
	}

	t.Logf("Loaded %d atoms from embedded corpus", count)

	// Verify we have atoms from different categories
	categories := make(map[AtomCategory]int)
	for _, atom := range corpus.All() {
		categories[atom.Category]++
	}

	t.Logf("Categories found: %v", categories)

	// Check for expected categories
	expectedCategories := []AtomCategory{
		CategoryIdentity,
		CategoryProtocol,
		CategoryLanguage,
		CategoryMethodology,
	}

	for _, cat := range expectedCategories {
		if categories[cat] == 0 {
			t.Errorf("Expected atoms in category %s, found none", cat)
		}
	}
}

func TestEmbeddedCorpusHasSystemAtoms(t *testing.T) {
	corpus, err := LoadEmbeddedCorpus()
	if err != nil {
		t.Fatalf("LoadEmbeddedCorpus failed: %v", err)
	}

	// Check for specific system atoms we created
	systemAtomIDs := []string{
		"system/legislator/identity",
		"system/perception/identity",
		"system/autopoiesis/executive",
		"system/autopoiesis/router",
		"system/autopoiesis/world_model",
	}

	for _, id := range systemAtomIDs {
		atom, found := corpus.Get(id)
		if !found {
			t.Errorf("Expected system atom %s not found in corpus", id)
			continue
		}
		if atom.Content == "" {
			t.Errorf("System atom %s has empty content", id)
		}
		t.Logf("Found system atom: %s (%d tokens)", id, atom.TokenCount)
	}
}

func TestMustLoadEmbeddedCorpus(t *testing.T) {
	// Should not panic
	corpus := MustLoadEmbeddedCorpus()
	if corpus == nil {
		t.Error("MustLoadEmbeddedCorpus returned nil")
	}
	if corpus.Count() == 0 {
		t.Error("MustLoadEmbeddedCorpus returned empty corpus")
	}
}

func TestEmbeddedCorpusLoadsMangleDocAtoms(t *testing.T) {
	corpus, err := LoadEmbeddedCorpus()
	if err != nil {
		t.Fatalf("LoadEmbeddedCorpus failed: %v", err)
	}

	// This atom is generated from the Mangle doc corpus under internal/prompt/atoms/mangle/.
	const id = "language/mangle/docs/aggregation_syntax"

	atom, found := corpus.Get(id)
	if !found {
		t.Fatalf("Expected atom %s not found in corpus", id)
	}
	if atom.Content == "" {
		t.Fatalf("Atom %s has empty content", id)
	}
	if atom.Description != "Mangle Aggregation Syntax" {
		t.Fatalf("Atom %s description mismatch: got %q", id, atom.Description)
	}
	if !strings.Contains(atom.Content, "Pipeline Operator") {
		t.Fatalf("Atom %s content did not appear to load from file", id)
	}
}
