package init

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"codenerd/internal/embedding"
	"codenerd/internal/logging"
	"codenerd/internal/store"
)

// EnsureAllAgentKnowledge heals every existing agent DB in .nerd/shards: empty
// KBs get baseline atoms from the agent name, and atoms without vectors get
// embedded. It runs on every init so existing agents (not just the ones
// created this run) converge to populated and searchable without --force.
func EnsureAllAgentKnowledge(ctx context.Context, workspace string, engine embedding.EmbeddingEngine) (healed int, added int) {
	shardsDir := filepath.Join(workspace, ".nerd", "shards")
	entries, err := os.ReadDir(shardsDir)
	if err != nil {
		return 0, 0
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, "_knowledge.db") {
			continue
		}
		agentName := strings.TrimSuffix(name, "_knowledge.db")
		dbPath := filepath.Join(shardsDir, name)
		db, err := store.NewLocalStore(dbPath)
		if err != nil {
			continue
		}
		if engine != nil {
			db.SetEmbeddingEngine(engine)
		}
		func() {
			defer db.Close()
			atoms, err := db.GetAllKnowledgeAtoms()
			if err != nil {
				return
			}
			if len(atoms) == 0 {
				// Seed from the agent name so a brand-new or never-researched
				// agent still has searchable knowledge. Richer topics come
				// from the next createAgentKnowledgeBase run.
				n := seedBaselineKnowledge(ctx, db, RecommendedAgent{Name: agentName}, map[string]bool{})
				added += n
			}
			n, err := db.BackfillKnowledgeAtomVectors(ctx)
			if err == nil {
				added += n
			}
			healed++
		}()
	}
	if healed > 0 {
		logging.Boot("EnsureAllAgentKnowledge: healed %d agent DBs, added %d atoms/vectors", healed, added)
	}
	return healed, added
}
