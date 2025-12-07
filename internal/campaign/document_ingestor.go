package campaign

import (
	"codenerd/internal/embedding"
	"codenerd/internal/store"
	"context"
	"fmt"
)

// DocumentIngestor writes campaign source docs into a campaign-scoped knowledge store:
// - Vectors with embeddings for semantic recall
// - Knowledge graph links (campaign -> doc -> chunk)
// - Knowledge atoms per chunk for lightweight retrieval
type DocumentIngestor struct {
	store *store.LocalStore
}

// NewDocumentIngestor creates an ingestor backed by the given knowledge DB path.
func NewDocumentIngestor(dbPath string, embedCfg embedding.Config) (*DocumentIngestor, error) {
	ls, err := store.NewLocalStore(dbPath)
	if err != nil {
		return nil, err
	}

	// Best-effort embedding engine (falls back to keyword search if unavailable)
	if eng, err := embedding.NewEngine(embedCfg); err == nil && eng != nil {
		ls.SetEmbeddingEngine(eng)
	}

	return &DocumentIngestor{store: ls}, nil
}

// Close closes the underlying store.
func (di *DocumentIngestor) Close() error {
	if di.store == nil {
		return nil
	}
	return di.store.Close()
}

// Ingest persists document content into vectors + knowledge graph.
// Returns number of chunks ingested.
func (di *DocumentIngestor) Ingest(ctx context.Context, campaignID string, fileContents map[string]string) (int, error) {
	if di.store == nil {
		return 0, fmt.Errorf("ingestor store not initialized")
	}

	totalChunks := 0

	for path, text := range fileContents {
		chunks := chunkText(text, 2000)
		if len(chunks) == 0 {
			continue
		}

		_ = di.store.StoreLink(campaignID, "/has_source_doc", path, 1.0, map[string]interface{}{
			"path": path,
		})

		for idx, chunk := range chunks {
			meta := map[string]interface{}{
				"campaign_id": campaignID,
				"path":        path,
				"chunk_index": idx,
				"total":       len(chunks),
			}
			// Vector + metadata
			_ = di.store.StoreVectorWithEmbedding(ctx, chunk, meta)
			// Knowledge atom for lightweight recall
			_ = di.store.StoreKnowledgeAtom(path, chunk, 0.9)
			// Graph link for chunk navigation
			chunkID := fmt.Sprintf("%s#%d", path, idx)
			_ = di.store.StoreLink(path, "/has_chunk", chunkID, 0.5, meta)
			totalChunks++
		}
	}

	return totalChunks, nil
}
