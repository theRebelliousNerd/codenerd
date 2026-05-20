package store

import (
	"context"
	"testing"
	"time"
)

func TestLocalStore_Knowledge_Extra(t *testing.T) {
	s, err := NewLocalStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer s.Close()

	// 1. StoreKnowledgeAtom
	err = s.StoreKnowledgeAtom("concept1", "content1", 0.9)
	if err != nil {
		t.Errorf("StoreKnowledgeAtom failed: %v", err)
	}

	// 2. GetKnowledgeAtoms
	atoms, err := s.GetKnowledgeAtoms("concept1")
	if err != nil {
		t.Errorf("GetKnowledgeAtoms failed: %v", err)
	}
	if len(atoms) != 1 || atoms[0].Content != "content1" {
		t.Errorf("Expected 1 atom with content1, got %v", atoms)
	}

	// 3. GetAllKnowledgeAtoms
	allAtoms, err := s.GetAllKnowledgeAtoms()
	if err != nil {
		t.Errorf("GetAllKnowledgeAtoms failed: %v", err)
	}
	if len(allAtoms) != 1 {
		t.Errorf("Expected 1 total atom, got %d", len(allAtoms))
	}

	// 4. GetKnowledgeAtomsByPrefix
	prefixAtoms, err := s.GetKnowledgeAtomsByPrefix("conc")
	if err != nil {
		t.Errorf("GetKnowledgeAtomsByPrefix failed: %v", err)
	}
	if len(prefixAtoms) != 1 {
		t.Errorf("Expected 1 atom for prefix, got %d", len(prefixAtoms))
	}

	// 5. ensureContentHashes (trigger it by manually inserting a row with NULL hash)
	_, err = s.db.Exec("INSERT INTO knowledge_atoms (concept, content, confidence) VALUES ('concept2', 'content2', 0.8)")
	if err != nil {
		t.Fatalf("Manual insert failed: %v", err)
	}
	err = s.ensureContentHashes()
	if err != nil {
		t.Errorf("ensureContentHashes failed: %v", err)
	}

	// 6. KnowledgeStore wrapper
	ks, err := NewKnowledgeStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create KnowledgeStore: %v", err)
	}
	defer ks.Close()

	atom := KnowledgeAtom{
		Concept:    "concept3",
		Content:    "content3",
		Source:     "test",
		Confidence: 0.99,
		Tags:       []string{"tag1"},
		CreatedAt:  time.Now(),
	}
	err = ks.StoreAtom(atom)
	if err != nil {
		t.Errorf("KnowledgeStore.StoreAtom failed: %v", err)
	}
	
	// Ensure table existence check for prefix
	emptyKs, _ := NewKnowledgeStore(":memory:")
	emptyKs.db.Exec("DROP TABLE knowledge_atoms")
	_, err = emptyKs.GetKnowledgeAtomsByPrefix("test")
	if err != nil {
		t.Errorf("Expected no error when table doesn't exist, got %v", err)
	}

	// 7. StoreKnowledgeAtomWithEmbedding / SearchKnowledgeAtomsSemantic (with nil embeddingEngine)
	ctx := context.Background()
	err = s.StoreKnowledgeAtomWithEmbedding(ctx, "concept-embed", "content-embed", 0.95)
	if err != nil {
		t.Errorf("StoreKnowledgeAtomWithEmbedding failed: %v", err)
	}

	semAtoms, err := s.SearchKnowledgeAtomsSemantic(ctx, "query", 10)
	if err != nil {
		t.Errorf("SearchKnowledgeAtomsSemantic failed: %v", err)
	}
	if len(semAtoms) != 0 {
		t.Errorf("Expected 0 semantic atoms when engine is nil, got %d", len(semAtoms))
	}
}
