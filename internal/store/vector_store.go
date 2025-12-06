// Package store - Vector embedding support for LocalStore
// This file extends LocalStore with real vector embeddings using the embedding engine.
package store

import (
	"codenerd/internal/embedding"
	"context"
	"encoding/json"
	"fmt"
	"math"
)

// =============================================================================
// VECTOR STORE WITH REAL EMBEDDINGS
// =============================================================================

// SetEmbeddingEngine configures the embedding engine for this LocalStore.
// Must be called before StoreVectorWithEmbedding.
func (s *LocalStore) SetEmbeddingEngine(engine embedding.EmbeddingEngine) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.embeddingEngine = engine
}

// StoreVectorWithEmbedding stores content with a real vector embedding.
// This is the new method that replaces StoreVector for semantic search.
func (s *LocalStore) StoreVectorWithEmbedding(ctx context.Context, content string, metadata map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.embeddingEngine == nil {
		// Fallback to keyword-based storage (backward compatible)
		return s.storeVectorKeywordOnly(content, metadata)
	}

	// Generate embedding
	embeddingVec, err := s.embeddingEngine.Embed(ctx, content)
	if err != nil {
		return fmt.Errorf("failed to generate embedding: %w", err)
	}

	// Serialize embedding as JSON
	embeddingJSON, err := json.Marshal(embeddingVec)
	if err != nil {
		return fmt.Errorf("failed to serialize embedding: %w", err)
	}

	metaJSON, _ := json.Marshal(metadata)

	_, err = s.db.Exec(
		"INSERT OR REPLACE INTO vectors (content, embedding, metadata) VALUES (?, ?, ?)",
		content, string(embeddingJSON), string(metaJSON),
	)
	return err
}

// storeVectorKeywordOnly stores content without embeddings (fallback).
func (s *LocalStore) storeVectorKeywordOnly(content string, metadata map[string]interface{}) error {
	metaJSON, _ := json.Marshal(metadata)

	_, err := s.db.Exec(
		"INSERT OR REPLACE INTO vectors (content, metadata) VALUES (?, ?)",
		content, string(metaJSON),
	)
	return err
}

// VectorRecallSemantic performs true semantic search using cosine similarity.
// This is the new method that replaces VectorRecall for semantic search.
func (s *LocalStore) VectorRecallSemantic(ctx context.Context, query string, limit int) ([]VectorEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 10
	}

	if s.embeddingEngine == nil {
		// Fallback to keyword search
		return s.vectorRecallKeyword(query, limit)
	}

	// Generate query embedding
	queryEmbedding, err := s.embeddingEngine.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	// Fetch all vectors from database
	rows, err := s.db.Query(
		"SELECT id, content, embedding, metadata, created_at FROM vectors WHERE embedding IS NOT NULL",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type candidate struct {
		entry      VectorEntry
		similarity float64
	}

	var candidates []candidate

	for rows.Next() {
		var entry VectorEntry
		var embeddingJSON, metaJSON string

		if err := rows.Scan(&entry.ID, &entry.Content, &embeddingJSON, &metaJSON, &entry.CreatedAt); err != nil {
			continue
		}

		// Deserialize embedding
		var embeddingVec []float32
		if err := json.Unmarshal([]byte(embeddingJSON), &embeddingVec); err != nil {
			continue
		}

		// Calculate cosine similarity
		similarity, err := embedding.CosineSimilarity(queryEmbedding, embeddingVec)
		if err != nil {
			continue
		}

		// Deserialize metadata
		if metaJSON != "" {
			json.Unmarshal([]byte(metaJSON), &entry.Metadata)
		}

		candidates = append(candidates, candidate{
			entry:      entry,
			similarity: similarity,
		})
	}

	// Sort by similarity descending
	for i := 0; i < len(candidates); i++ {
		for j := i + 1; j < len(candidates); j++ {
			if candidates[j].similarity > candidates[i].similarity {
				candidates[i], candidates[j] = candidates[j], candidates[i]
			}
		}
	}

	// Return top K
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}

	results := make([]VectorEntry, len(candidates))
	for i, c := range candidates {
		results[i] = c.entry
		// Optionally store similarity in metadata
		if results[i].Metadata == nil {
			results[i].Metadata = make(map[string]interface{})
		}
		results[i].Metadata["similarity"] = c.similarity
	}

	return results, nil
}

// vectorRecallKeyword is the fallback keyword-based search.
func (s *LocalStore) vectorRecallKeyword(query string, limit int) ([]VectorEntry, error) {
	// This is the old implementation from local.go VectorRecall
	// Kept for backward compatibility when no embedding engine is set
	return s.VectorRecall(query, limit)
}

// GetVectorStats returns statistics about stored vectors.
func (s *LocalStore) GetVectorStats() (map[string]interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := make(map[string]interface{})

	var totalVectors int64
	s.db.QueryRow("SELECT COUNT(*) FROM vectors").Scan(&totalVectors)
	stats["total_vectors"] = totalVectors

	var withEmbeddings int64
	s.db.QueryRow("SELECT COUNT(*) FROM vectors WHERE embedding IS NOT NULL").Scan(&withEmbeddings)
	stats["with_embeddings"] = withEmbeddings

	var withoutEmbeddings int64
	withoutEmbeddings = totalVectors - withEmbeddings
	stats["without_embeddings"] = withoutEmbeddings

	if s.embeddingEngine != nil {
		stats["embedding_engine"] = s.embeddingEngine.Name()
		stats["embedding_dimensions"] = s.embeddingEngine.Dimensions()
	} else {
		stats["embedding_engine"] = "none (keyword search)"
	}

	return stats, nil
}

// ReembedAllVectors regenerates embeddings for all vectors that don't have them.
// Useful for migrating from keyword-only to embedding-based search.
func (s *LocalStore) ReembedAllVectors(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.embeddingEngine == nil {
		return fmt.Errorf("no embedding engine configured")
	}

	// Fetch all vectors without embeddings
	rows, err := s.db.Query("SELECT id, content FROM vectors WHERE embedding IS NULL")
	if err != nil {
		return err
	}
	defer rows.Close()

	type vectorToEmbed struct {
		id      int64
		content string
	}

	var vectors []vectorToEmbed
	for rows.Next() {
		var v vectorToEmbed
		if err := rows.Scan(&v.id, &v.content); err != nil {
			continue
		}
		vectors = append(vectors, v)
	}

	if len(vectors) == 0 {
		return nil // Nothing to do
	}

	// Generate embeddings in batches
	batchSize := 32
	for i := 0; i < len(vectors); i += batchSize {
		end := int(math.Min(float64(i+batchSize), float64(len(vectors))))
		batch := vectors[i:end]

		// Collect texts
		texts := make([]string, len(batch))
		for j, v := range batch {
			texts[j] = v.content
		}

		// Generate embeddings
		embeddings, err := s.embeddingEngine.EmbedBatch(ctx, texts)
		if err != nil {
			return fmt.Errorf("failed to generate batch embeddings: %w", err)
		}

		// Update database
		for j, v := range batch {
			embeddingJSON, _ := json.Marshal(embeddings[j])
			_, err := s.db.Exec(
				"UPDATE vectors SET embedding = ? WHERE id = ?",
				string(embeddingJSON), v.id,
			)
			if err != nil {
				return fmt.Errorf("failed to update vector %d: %w", v.id, err)
			}
		}
	}

	return nil
}
