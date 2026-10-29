package store

import (
	"context"
	"testing"
)

type backfillStubEngine struct{ dim int }

func (s backfillStubEngine) Embed(_ context.Context, _ string) ([]float32, error) {
	out := make([]float32, s.dim)
	for i := range out {
		out[i] = 0.1
	}
	return out, nil
}
func (s backfillStubEngine) EmbedBatch(_ context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i := range out {
		vec := make([]float32, s.dim)
		for j := range vec {
			vec[j] = 0.1
		}
		out[i] = vec
	}
	return out, nil
}
func (s backfillStubEngine) Dimensions() int { return s.dim }
func (s backfillStubEngine) Name() string    { return "stub" }

func TestBackfillKnowledgeAtomVectors_EmbedsMissing(t *testing.T) {
	dir := t.TempDir()
	dbPath := dir + "/kb.db"
	s, err := NewLocalStore(dbPath)
	if err != nil {
		t.Fatalf("NewLocalStore: %v", err)
	}
	defer s.Close()
	s.SetEmbeddingEngine(backfillStubEngine{dim: 8})

	// Write atoms without embeddings (legacy path).
	if err := s.StoreKnowledgeAtom("guide/test", "unembedded content one", 0.9, "test"); err != nil {
		t.Fatalf("StoreKnowledgeAtom: %v", err)
	}
	if err := s.StoreKnowledgeAtom("guide/test2", "unembedded content two", 0.8, "test"); err != nil {
		t.Fatalf("StoreKnowledgeAtom: %v", err)
	}

	added, err := s.BackfillKnowledgeAtomVectors(context.Background())
	if err != nil {
		t.Fatalf("Backfill: %v", err)
	}
	if added != 2 {
		t.Fatalf("added = %d, want 2 (bubbletea-style 73/0 case)", added)
	}

	// Second run is idempotent: nothing missing.
	added, err = s.BackfillKnowledgeAtomVectors(context.Background())
	if err != nil {
		t.Fatalf("Backfill second: %v", err)
	}
	if added != 0 {
		t.Fatalf("second added = %d, want 0 (no duplicates)", added)
	}

	// Semantic search finds the backfilled atoms.
	hits, err := s.SearchKnowledgeAtomsSemantic(context.Background(), "unembedded content", 5)
	if err != nil {
		t.Fatalf("SearchKnowledgeAtomsSemantic: %v", err)
	}
	if len(hits) == 0 {
		t.Fatalf("semantic search found 0 hits after backfill, want >0")
	}
}

func TestStoreKnowledgeAtomWithEmbedding_WritesBothHalves(t *testing.T) {
	dir := t.TempDir()
	s, err := NewLocalStore(dir + "/kb2.db")
	if err != nil {
		t.Fatalf("NewLocalStore: %v", err)
	}
	defer s.Close()
	s.SetEmbeddingEngine(backfillStubEngine{dim: 8})

	if err := s.StoreKnowledgeAtomWithEmbedding(context.Background(), "guide/seed", "searchable content", 0.9, "test"); err != nil {
		t.Fatalf("StoreKnowledgeAtomWithEmbedding: %v", err)
	}
	atoms, err := s.GetAllKnowledgeAtoms()
	if err != nil || len(atoms) != 1 {
		t.Fatalf("GetAllKnowledgeAtoms = %d, err=%v, want 1", len(atoms), err)
	}
	added, err := s.BackfillKnowledgeAtomVectors(context.Background())
	if err != nil {
		t.Fatalf("Backfill: %v", err)
	}
	if added != 0 {
		t.Fatalf("backfill after embedded write added %d, want 0", added)
	}
}
