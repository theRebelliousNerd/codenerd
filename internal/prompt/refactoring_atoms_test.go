package prompt

import (
	"testing"
)

func TestRefactoringAtoms(t *testing.T) {
	// Load the embedded corpus which should include our new files
	corpus, err := LoadEmbeddedCorpus()
	if err != nil {
		t.Fatalf("Failed to load embedded corpus: %v", err)
	}

	expectedIDs := []string{
		"methodology/refactoring/go_workflow",
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

        // Check language specific fields
        if len(atom.Languages) == 0 {
             t.Errorf("Atom %s has no languages defined", id)
        }

        foundGo := false
        for _, lang := range atom.Languages {
            if lang == "/go" || lang == "/golang" {
                foundGo = true
                break
            }
        }
        if !foundGo {
             t.Errorf("Atom %s should have /go or /golang language", id)
        }
	}
}
