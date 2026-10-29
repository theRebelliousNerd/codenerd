package sync

import (
	"context"
	"database/sql"

	"codenerd/internal/embedding"
	"codenerd/internal/logging"
	"codenerd/internal/store"
)

// backfillKnowledgeVectors ensures every row in knowledge_atoms has a matching
// vectors row with content_type=knowledge_atom. KBs written through
// StoreKnowledgeAtom (no embedding) are otherwise unsearchable: 73 atoms with
// 0 vectors embeds nothing. It is idempotent: atoms that already have a vector
// are skipped, so every boot heals without duplicating rows.
func backfillKnowledgeVectors(ctx context.Context, db *sql.DB, dbPath string, engine embedding.EmbeddingEngine) error {
	_ = db
	if engine == nil {
		return nil
	}
	agentDB, err := store.NewLocalStore(dbPath)
	if err != nil {
		return nil
	}
	defer agentDB.Close()
	agentDB.SetEmbeddingEngine(engine)
	n, err := agentDB.BackfillKnowledgeAtomVectors(ctx)
	if err != nil {
		return err
	}
	if n > 0 {
		logging.Get(logging.CategoryStore).Info("Knowledge backfill: embedded %d atoms in %s", n, dbPath)
	}
	return nil
}
