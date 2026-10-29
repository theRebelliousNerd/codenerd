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

func TestBaselineAgentAtoms_OfflineSeed(t *testing.T) {
	agent := RecommendedAgent{Name: "GoExpert", Description: "Go specialist", Topics: []string{"go concurrency", "go generics"}}
	atoms := baselineAgentAtoms(agent)
	if len(atoms) < 4 {
		t.Fatalf("baseline atoms = %d, want >=4 (identity+mission+methodology+domains)", len(atoms))
	}
	seen := map[string]bool{}
	for _, a := range atoms {
		if a.Concept == "" || a.Content == "" {
			t.Fatalf("empty baseline atom: %+v", a)
		}
		seen[a.Concept] = true
	}
	for _, want := range []string{"agent_identity", "agent_mission", "methodology/overview"} {
		if !seen[want] {
			t.Fatalf("baseline missing %s: %v", want, atoms)
		}
	}
}

func TestSeedBaselineKnowledge_PopulatesEmptyKB(t *testing.T) {
	dir := t.TempDir()
	db, err := store.NewLocalStore(dir + "/goexpert_knowledge.db")
	if err != nil {
		t.Fatalf("NewLocalStore: %v", err)
	}
	defer db.Close()
	db.SetEmbeddingEngine(seedStubEngine{dim: 8})

	agent := RecommendedAgent{Name: "GoExpert", Description: "Go specialist", Topics: []string{"go concurrency"}}
	added := seedBaselineKnowledge(context.Background(), db, agent, map[string]bool{})
	if added == 0 {
		t.Fatalf("seedBaselineKnowledge added 0, want >0 for empty agent")
	}
	atoms, err := db.GetAllKnowledgeAtoms()
	if err != nil || len(atoms) == 0 {
		t.Fatalf("GetAllKnowledgeAtoms = %d, err=%v, want >0", len(atoms), err)
	}
	// Every seeded atom must already have a vector (the bubbletea 73/0 fix).
	n, err := db.BackfillKnowledgeAtomVectors(context.Background())
	if err != nil {
		t.Fatalf("Backfill: %v", err)
	}
	if n != 0 {
		t.Fatalf("backfill after seed added %d, want 0 (seed writes embeddings)", n)
	}
	hits, err := db.SearchKnowledgeAtomsSemantic(context.Background(), "Go specialist", 3)
	if err != nil {
		t.Fatalf("SearchKnowledgeAtomsSemantic: %v", err)
	}
	if len(hits) == 0 {
		t.Fatalf("seeded KB unsearchable: 0 semantic hits")
	}
}

func TestAppendKnowledgeAtom_Embeds(t *testing.T) {
	dir := t.TempDir()
	db, err := store.NewLocalStore(dir + "/kb.db")
	if err != nil {
		t.Fatalf("NewLocalStore: %v", err)
	}
	defer db.Close()
	db.SetEmbeddingEngine(seedStubEngine{dim: 8})

	hashes := map[string]bool{}
	ok, err := appendKnowledgeAtom(context.Background(), db, "guide/x", "embedded content", 0.9, hashes)
	if err != nil || !ok {
		t.Fatalf("appendKnowledgeAtom = %v,%v, want true,nil", ok, err)
	}
	// Duplicate is skipped.
	ok, err = appendKnowledgeAtom(context.Background(), db, "guide/x", "embedded content", 0.9, hashes)
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
