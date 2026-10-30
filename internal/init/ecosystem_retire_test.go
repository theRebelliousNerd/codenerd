package init

import (
	"context"
	"path/filepath"
	"testing"

	"codenerd/internal/store"
)

// A file that left the workspace (here, a checkout since deleted) kept its
// ecosystem rows in the agent KB for good: every init appended the current
// import and nothing retired the previous one, so its blank paragraph was
// re-sent to the embedder on every boot. An init's ecosystem import now owns
// its rows: one it no longer produces is retired.
func TestCreateAgentKnowledgeBase_RetiresEcosystemRowsTheImportNoLongerHas(t *testing.T) {
	ws := t.TempDir()
	t.Cleanup(func() { clearEcosystemSeed(ws) })
	ini := &Initializer{config: InitConfig{Workspace: ws, SkipResearch: true}, embedEngine: fixedInitEngine{}}
	kbPath := filepath.Join(t.TempDir(), "goexpert_knowledge.db")
	agent := RecommendedAgent{Name: "GoExpert"}
	kept := ecosystemAtom{Concept: "ecosystem paragraph 1/2 tool=claude path=agents.md bytes=9", Content: "Run tests"}
	gone := ecosystemAtom{Concept: "ecosystem paragraph 1/1 tool=claude path=.claude/worktrees/old/agents.md bytes=0", Content: ""}

	publishEcosystemSeed(ws, &ecosystemWorkspace{knowledge: map[string][]ecosystemAtom{"GoExpert": {kept, gone}}})
	if _, err := ini.createAgentKnowledgeBase(context.Background(), kbPath, agent, false); err != nil {
		t.Fatal(err)
	}
	// The next init: the deleted checkout's file is no longer in the corpus.
	publishEcosystemSeed(ws, &ecosystemWorkspace{knowledge: map[string][]ecosystemAtom{"GoExpert": {kept}}})
	if _, err := ini.createAgentKnowledgeBase(context.Background(), kbPath, agent, true); err != nil {
		t.Fatal(err)
	}

	db, err := store.NewLocalStore(kbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	atoms, err := db.GetAllKnowledgeAtoms()
	if err != nil {
		t.Fatal(err)
	}
	var sawKept bool
	for _, a := range atoms {
		if a.Concept == gone.Concept {
			t.Fatalf("the deleted file's row survived the re-import: %q", a.Concept)
		}
		if a.Concept == kept.Concept {
			sawKept = true
		}
	}
	if !sawKept {
		t.Fatal("a row the import still has was retired")
	}

	// No published import (the ecosystem phase did not run) retires nothing.
	clearEcosystemSeed(ws)
	if _, err := ini.createAgentKnowledgeBase(context.Background(), kbPath, agent, true); err != nil {
		t.Fatal(err)
	}
	db2, err := store.NewLocalStore(kbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db2.Close()
	if atoms, _ := db2.GetAllKnowledgeAtoms(); len(atoms) == 0 {
		t.Fatal("an init with no ecosystem import retired the KB's rows")
	}
}
