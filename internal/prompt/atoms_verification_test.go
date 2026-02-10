package prompt

import (
	"testing"
)

func TestNewDebuggingAtoms(t *testing.T) {
	// Load the embedded corpus which should include our new files
	// logic: The //go:embed directive in embedded.go includes atoms/* recursively.
	corpus, err := LoadEmbeddedCorpus()
	if err != nil {
		t.Fatalf("Failed to load embedded corpus: %v", err)
	}

	expectedIDs := []string{
		"methodology/debugging/go",
		"methodology/debugging/python",
		"methodology/debugging/typescript",
	}

	for _, id := range expectedIDs {
		atom, found := corpus.Get(id)
		if !found {
			t.Errorf("Atom %s not found in embedded corpus", id)
			continue
		}
		if atom.ID != id {
			t.Errorf("Atom ID mismatch: got %s, want %s", atom.ID, id)
		}

		// Verify basic content sanity
		if len(atom.Content) == 0 {
			t.Errorf("Atom %s has empty content", id)
		}

		if atom.Category != CategoryMethodology {
			t.Errorf("Atom %s has wrong category: got %s, want %s", id, atom.Category, CategoryMethodology)
		}
	}
}
