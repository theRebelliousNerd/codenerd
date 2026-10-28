package store

import (
	"codenerd/internal/embedding"
	"context"
)

// Test seams: conveniences only tests call. They live here, not in
// production files, so the dead-code budget measures production code.

// NewLearnedCorpusStore creates or opens the learned corpus store.
// Creates the database and schema if it doesn't exist.
//
// Parameters:
//   - dbPath: Path to the SQLite database file (e.g., ".nerd/learned_corpus.db")
//   - engine: Embedding engine for generating pattern embeddings
func NewLearnedCorpusStore(dbPath string, engine embedding.EmbeddingEngine) (*LearnedCorpusStore, error) {
	return NewLearnedCorpusStoreWithContext(context.Background(), dbPath, engine)
}
