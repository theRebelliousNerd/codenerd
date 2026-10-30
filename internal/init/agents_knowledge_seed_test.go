package init

import (
	"context"
	"testing"

	"codenerd/internal/store"
)

type seedStubEngine struct{ dim int }

func (s seedStubEngine) Embed(_ context.Context, _ string) ([]float32, error) {
	out := make([]float32, s.dim)
	for i := range out {
		out[i] = 0.2
	}
	return out, nil
}
func (s seedStubEngine) EmbedBatch(_ context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i := range out {
		vec := make([]float32, s.dim)
		for j := range vec {
			vec[j] = 0.2
		}
		out[i] = vec
	}
	return out, nil
}
func (s seedStubEngine) Dimensions() int { return s.dim }
func (s seedStubEngine) Name() string    { return "stub" }

func TestAppendKnowledgeAtom_Embeds(t *testing.T) {
	dir := t.TempDir()
	db, err := store.NewLocalStore(dir + "/kb.db")
	if err != nil {
		t.Fatalf("NewLocalStore: %v", err)
	}
	defer db.Close()
	db.SetEmbeddingEngine(seedStubEngine{dim: 8})

	hashes := map[string]bool{}
	ok, err := appendKnowledgeAtom(context.Background(), db, "guide/x", "embedded content", 0.9, "test", hashes)
	if err != nil || !ok {
		t.Fatalf("appendKnowledgeAtom = %v,%v, want true,nil", ok, err)
	}
	// Duplicate is skipped.
	ok, err = appendKnowledgeAtom(context.Background(), db, "guide/x", "embedded content", 0.9, "test", hashes)
	if err != nil || ok {
		t.Fatalf("duplicate append = %v,%v, want false,nil", ok, err)
	}
	n, err := db.BackfillKnowledgeAtomVectors(context.Background())
	if err != nil {
		t.Fatalf("Backfill: %v", err)
	}
	if n != 0 {
		t.Fatalf("append without embedding: backfill added %d, want 0", n)
	}
}
