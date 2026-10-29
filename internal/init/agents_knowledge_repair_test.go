package init

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"codenerd/internal/store"
)

func TestEnsureAllAgentKnowledge_HealsEmptyAndUnembedded(t *testing.T) {
	ws := t.TempDir()
	shardsDir := filepath.Join(ws, ".nerd", "shards")
	if err := os.MkdirAll(shardsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// Empty agent DB (the 12 zero-atom agents).
	emptyPath := filepath.Join(shardsDir, "goexpert_knowledge.db")
	emptyDB, err := store.NewLocalStore(emptyPath)
	if err != nil {
		t.Fatalf("NewLocalStore empty: %v", err)
	}
	emptyDB.Close()

	// Unembedded agent DB (the bubbleteaexpert 73/0 case, scaled down).
	vecPath := filepath.Join(shardsDir, "bubbleteaexpert_knowledge.db")
	vecDB, err := store.NewLocalStore(vecPath)
	if err != nil {
		t.Fatalf("NewLocalStore vec: %v", err)
	}
	if err := vecDB.StoreKnowledgeAtom("guide/a", "legacy content a", 0.9, "test"); err != nil {
		t.Fatalf("StoreKnowledgeAtom: %v", err)
	}
	if err := vecDB.StoreKnowledgeAtom("guide/b", "legacy content b", 0.8, "test"); err != nil {
		t.Fatalf("StoreKnowledgeAtom: %v", err)
	}
	vecDB.Close()

	healed, added := EnsureAllAgentKnowledge(context.Background(), ws, seedStubEngine{dim: 8})
	if healed != 2 {
		t.Fatalf("healed = %d, want 2", healed)
	}
	if added == 0 {
		t.Fatalf("added = 0, want >0 (baseline seed + vector backfill)")
	}

	for _, p := range []string{emptyPath, vecPath} {
		db, err := store.NewLocalStore(p)
		if err != nil {
			t.Fatalf("reopen %s: %v", p, err)
		}
		atoms, err := db.GetAllKnowledgeAtoms()
		if err != nil || len(atoms) == 0 {
			db.Close()
			t.Fatalf("%s atoms = %d, err=%v, want >0", p, len(atoms), err)
		}
		n, err := db.BackfillKnowledgeAtomVectors(context.Background())
		db.Close()
		if err != nil {
			t.Fatalf("final backfill %s: %v", p, err)
		}
		if n != 0 {
			t.Fatalf("%s still missing %d vectors after Ensure", p, n)
		}
	}
}
