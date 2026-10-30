package init

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"codenerd/internal/store"
)

// Healing an existing workspace prunes the removed seed's placeholder rows,
// keeps real knowledge, backfills vectors, and writes nothing of its own: an
// empty KB stays empty instead of being filled with the agent's own name.
func TestEnsureAllAgentKnowledge_PrunesPlaceholdersAndSeedsNothing(t *testing.T) {
	ws := t.TempDir()
	shardsDir := filepath.Join(ws, ".nerd", "shards")
	if err := os.MkdirAll(shardsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	emptyPath := filepath.Join(shardsDir, "goexpert_knowledge.db")
	emptyDB, err := store.NewLocalStore(emptyPath)
	if err != nil {
		t.Fatalf("NewLocalStore empty: %v", err)
	}
	emptyDB.Close()

	thinPath := filepath.Join(shardsDir, "bubbleteaexpert_knowledge.db")
	thinDB, err := store.NewLocalStore(thinPath)
	if err != nil {
		t.Fatalf("NewLocalStore thin: %v", err)
	}
	rows := [][2]string{
		{"agent_identity", "You are bubbleteaexpert, a specialist agent in the codeNERD ecosystem."},
		{"agent_mission", "bubbleteaexpert exists to carry out bubbleteaexpert tasks in this workspace."},
		{"methodology/overview", "bubbleteaexpert methodology: decompose the task, consult the knowledge base, verify."},
		{"domain/tui", "bubbleteaexpert domain knowledge (1 of 1): tui. When working on tui, check the repository's own docs and code for project-specific rules."},
		{"guide/a", "Update returns the next model and a command; never mutate the model from a goroutine."},
	}
	for _, r := range rows {
		if err := thinDB.StoreKnowledgeAtom(r[0], r[1], 0.9, "test"); err != nil {
			t.Fatalf("StoreKnowledgeAtom: %v", err)
		}
	}
	thinDB.Close()

	healed, _ := EnsureAllAgentKnowledge(context.Background(), ws, seedStubEngine{dim: 8})
	if healed != 2 {
		t.Fatalf("healed = %d, want 2", healed)
	}

	empty, err := store.NewLocalStore(emptyPath)
	if err != nil {
		t.Fatal(err)
	}
	atoms, err := empty.GetAllKnowledgeAtoms()
	empty.Close()
	if err != nil || len(atoms) != 0 {
		t.Fatalf("the empty KB was filled: %d atom(s), err %v", len(atoms), err)
	}

	thin, err := store.NewLocalStore(thinPath)
	if err != nil {
		t.Fatal(err)
	}
	defer thin.Close()
	atoms, err = thin.GetAllKnowledgeAtoms()
	if err != nil || len(atoms) != 1 || atoms[0].Concept != "guide/a" {
		t.Fatalf("after healing the thin KB holds %+v (err %v), want only guide/a", atoms, err)
	}
	if n, err := thin.BackfillKnowledgeAtomVectors(context.Background()); err != nil || n != 0 {
		t.Fatalf("%d vector(s) still missing after healing, err %v", n, err)
	}
}
