package store

import (
	"testing"
)

func TestLocalStore_PromptAtoms_Extra(t *testing.T) {
	s, err := NewLocalStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer s.Close()

	atom := &PromptAtom{
		AtomID:           "test-atom-1",
		Version:          1,
		Content:          "Hello World",
		TokenCount:       2,
		Category:         "test-cat",
		Subcategory:      "test-subcat",
		OperationalModes: []string{"/active"},
		IsExclusive:      "group1",
		EmbeddingTask:    "RETRIEVAL_DOCUMENT",
	}

	// 1. StorePromptAtom
	err = s.StorePromptAtom(atom)
	if err != nil {
		t.Errorf("StorePromptAtom failed: %v", err)
	}

	// Update
	atom.Version = 2
	err = s.StorePromptAtom(atom)
	if err != nil {
		t.Errorf("StorePromptAtom update failed: %v", err)
	}

	// 2. GetPromptAtom
	got, err := s.GetPromptAtom("test-atom-1")
	if err != nil {
		t.Errorf("GetPromptAtom failed: %v", err)
	}
	if got == nil || got.Version != 2 {
		t.Errorf("GetPromptAtom unexpected result")
	}

	gotNone, _ := s.GetPromptAtom("non-existent")
	if gotNone != nil {
		t.Errorf("Expected nil for non-existent atom")
	}

	// 3. LoadPromptAtoms
	atoms, err := s.LoadPromptAtoms()
	if err != nil {
		t.Errorf("LoadPromptAtoms failed: %v", err)
	}
	if len(atoms) != 1 {
		t.Errorf("Expected 1 atom, got %d", len(atoms))
	}

	// 4. LoadPromptAtomsByCategory
	atomsByCat, err := s.LoadPromptAtomsByCategory("test-cat")
	if err != nil {
		t.Errorf("LoadPromptAtomsByCategory failed: %v", err)
	}
	if len(atomsByCat) != 1 {
		t.Errorf("Expected 1 atom by cat, got %d", len(atomsByCat))
	}

	atomsByCatEmpty, _ := s.LoadPromptAtomsByCategory("non-existent")
	if len(atomsByCatEmpty) != 0 {
		t.Errorf("Expected 0 atoms")
	}

	// 5. DeletePromptAtom
	err = s.DeletePromptAtom("test-atom-1")
	if err != nil {
		t.Errorf("DeletePromptAtom failed: %v", err)
	}

	// Delete non-existent
	err = s.DeletePromptAtom("test-atom-1")
	if err != nil {
		t.Errorf("DeletePromptAtom non-existent failed: %v", err)
	}

	gotAfterDelete, _ := s.GetPromptAtom("test-atom-1")
	if gotAfterDelete != nil {
		t.Errorf("Expected nil after deletion")
	}
}
