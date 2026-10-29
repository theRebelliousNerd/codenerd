package init

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"codenerd/internal/embedding"
	"codenerd/internal/knowledge"
	"codenerd/internal/logging"
	"codenerd/internal/store"
)

// EnsureAllAgentKnowledge heals every existing agent DB in .nerd/shards:
// legacy placeholder rows are pruned and missing vectors are backfilled. It
// writes no knowledge of its own; an empty KB stays empty until a source
// teaches it (init's acquisition, the background refresh).
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
			// Legacy placeholder rows (the removed baseline seed's
			// templates) teach nothing and made topics read as covered;
			// they go. An agent with no knowledge stays empty and says so
			// (knowledge_acquisitions, `nerd check-kb`) until a source
			// teaches it.
			if len(atoms) > 0 {
				if n, err := knowledge.PrunePlaceholders(ctx, db.GetDB()); err != nil {
					logging.Boot("EnsureAllAgentKnowledge: placeholder prune for %s failed: %v", agentName, err)
				} else if n > 0 {
					logging.Boot("EnsureAllAgentKnowledge: pruned %d placeholder atom(s) from %s", n, agentName)
				}
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
