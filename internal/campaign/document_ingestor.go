package campaign

import (
	"codenerd/internal/embedding"
	"codenerd/internal/logging"
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

	logging.CampaignDebug("DocumentIngestor.Ingest: starting campaign=%s files=%d", campaignID, len(fileContents))
	totalChunks := 0

	for path, text := range fileContents {
		chunks := chunkText(text, 2000)
		if len(chunks) == 0 {
			continue
		}

		if err := di.store.StoreLink(campaignID, "/has_source_doc", path, 1.0, map[string]interface{}{
			"path": path,
		}); err != nil {
			logging.StoreWarn("DocumentIngestor: failed to store source doc link for %s: %v", path, err)
		}

		for idx, chunk := range chunks {
			meta := map[string]interface{}{
				"campaign_id": campaignID,
				"path":        path,
				"chunk_index": idx,
				"total":       len(chunks),
			}
			// Vector + metadata
			if err := di.store.StoreVectorWithEmbedding(ctx, chunk, meta); err != nil {
				logging.StoreWarn("DocumentIngestor: failed to store vector for %s chunk %d: %v", path, idx, err)
			}
			// Knowledge atom for lightweight recall
			if err := di.store.StoreKnowledgeAtom(path, chunk, 0.9); err != nil {
				logging.StoreWarn("DocumentIngestor: failed to store knowledge atom for %s: %v", path, err)
			}
			// Graph link for chunk navigation
			chunkID := fmt.Sprintf("%s#%d", path, idx)
			if err := di.store.StoreLink(path, "/has_chunk", chunkID, 0.5, meta); err != nil {
				logging.StoreWarn("DocumentIngestor: failed to store chunk link for %s: %v", chunkID, err)
			}
			totalChunks++
		}
	}

	logging.Campaign("DocumentIngestor.Ingest: completed campaign=%s total_chunks=%d", campaignID, totalChunks)
	return totalChunks, nil
}
