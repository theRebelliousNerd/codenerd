package store

import (
	"cmp"
	"codenerd/internal/embedding"
	"codenerd/internal/logging"
	"database/sql"
	"encoding/json"
	"fmt"
	"slices"
)

// vectorRecallBruteForce is the fallback cosine similarity search.
func (s *LocalStore) vectorRecallBruteForce(queryText string, queryEmbedding []float32, limit int) ([]VectorEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(
		"SELECT id, content, embedding, metadata, created_at FROM vectors WHERE embedding IS NOT NULL",
	)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to query vectors: %v", err)
		return nil, err
	}
	defer rows.Close()

	type candidate struct {
		entry      VectorEntry
		similarity float64
	}

	var candidates []candidate

	var embeddingVec []float32
	for rows.Next() {
		var entry VectorEntry
		var embeddingJSON, metaJSON []byte

		if err := rows.Scan(&entry.ID, &entry.Content, &embeddingJSON, &metaJSON, &entry.CreatedAt); err != nil {
			continue
		}

		var parseErr error

		embeddingVec, parseErr = fastParseVectorJSON(embeddingJSON, embeddingVec)

		if parseErr != nil {
			continue
		}

		similarity, err := embedding.CosineSimilarity(queryEmbedding, embeddingVec)
		if err != nil {
			continue
		}

		if len(metaJSON) > 0 {
			json.Unmarshal(metaJSON, &entry.Metadata)
		}

		candidates = append(candidates, candidate{
			entry:      entry,
			similarity: similarity,
		})
	}

	// Sort by similarity descending
	slices.SortFunc(candidates, func(a, b candidate) int {
		return cmp.Compare(b.similarity, a.similarity)
	})

	if len(candidates) > limit {
		candidates = candidates[:limit]
	}

	results := make([]VectorEntry, len(candidates))
	for i, c := range candidates {
		results[i] = c.entry
		if results[i].Metadata == nil {
			results[i].Metadata = make(map[string]any)
		}
		results[i].Metadata["similarity"] = c.similarity
	}

	if len(results) > 0 {
		logging.Store("VECTOR QUERY [brute-force] -> %d results (top match dist: %.4f | '%.30s...')", len(results), 1-results[0].Metadata["similarity"].(float64), results[0].Content)
	} else {
		logging.Store("VECTOR QUERY [brute-force] -> 0 results returned")
	}
	return results, nil
}

// vectorRecallBruteForceByPaths is the fallback cosine similarity search with path filtering.
func (s *LocalStore) vectorRecallBruteForceByPaths(queryText string, queryEmbedding []float32, limit int, allowedPaths []string) ([]VectorEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	queryStr, args := buildPathFilteredQuery(allowedPaths)
	rows, err := s.db.Query(queryStr, args...)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to query path-filtered vectors: %v", err)
		return nil, err
	}
	defer rows.Close()

	type candidate struct {
		entry      VectorEntry
		similarity float64
	}

	candidates := make([]candidate, 0, limit*2)

	var embeddingVec []float32
	for rows.Next() {
		var entry VectorEntry
		var embeddingJSON, metaJSON []byte

		if err := rows.Scan(&entry.ID, &entry.Content, &embeddingJSON, &metaJSON, &entry.CreatedAt); err != nil {
			continue
		}

		if len(metaJSON) > 0 {
			json.Unmarshal(metaJSON, &entry.Metadata)
		}

		var parseErr error
		embeddingVec, parseErr = fastParseVectorJSON(embeddingJSON, embeddingVec)
		if parseErr != nil {
			continue
		}

		similarity, err := embedding.CosineSimilarity(queryEmbedding, embeddingVec)
		if err != nil {
			continue
		}

		candidates = append(candidates, candidate{
			entry:      entry,
			similarity: similarity,
		})
	}

	slices.SortFunc(candidates, func(a, b candidate) int {
		return cmp.Compare(b.similarity, a.similarity)
	})

	if len(candidates) > limit {
		candidates = candidates[:limit]
	}

	results := make([]VectorEntry, len(candidates))
	for i, c := range candidates {
		results[i] = c.entry
		if results[i].Metadata == nil {
			results[i].Metadata = make(map[string]any)
		}
		results[i].Metadata["similarity"] = c.similarity
	}

	return results, nil
}

// vectorRecallBruteForceFiltered is the fallback cosine similarity search with metadata filtering.
func (s *LocalStore) vectorRecallBruteForceFiltered(queryText string, queryEmbedding []float32, limit int, metaKey string, metaValue any) ([]VectorEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	queryStr := "SELECT id, content, embedding, metadata, created_at FROM vectors WHERE embedding IS NOT NULL"
	var rows *sql.Rows
	var err error

	if metaKey != "" && metaValue != nil {
		pattern := fmt.Sprintf("%%\"%s\":\"%v\"%%", metaKey, metaValue)
		rows, err = s.db.Query(queryStr+" AND metadata LIKE ?", pattern)
	} else {
		rows, err = s.db.Query(queryStr)
	}
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to query vectors: %v", err)
		return nil, err
	}
	defer rows.Close()

	type candidate struct {
		entry      VectorEntry
		similarity float64
	}

	var candidates []candidate

	var embeddingVec []float32
	for rows.Next() {
		var entry VectorEntry
		var embeddingJSON, metaJSON []byte

		if err := rows.Scan(&entry.ID, &entry.Content, &embeddingJSON, &metaJSON, &entry.CreatedAt); err != nil {
			continue
		}

		if len(metaJSON) > 0 {
			json.Unmarshal(metaJSON, &entry.Metadata)
		}
		if !matchesMetadata(entry.Metadata, metaKey, metaValue) {
			continue
		}

		var parseErr error

		embeddingVec, parseErr = fastParseVectorJSON(embeddingJSON, embeddingVec)

		if parseErr != nil {
			continue
		}

		similarity, err := embedding.CosineSimilarity(queryEmbedding, embeddingVec)
		if err != nil {
			continue
		}

		candidates = append(candidates, candidate{
			entry:      entry,
			similarity: similarity,
		})
	}

	// Sort by similarity descending
	slices.SortFunc(candidates, func(a, b candidate) int {
		return cmp.Compare(b.similarity, a.similarity)
	})

	if len(candidates) > limit {
		candidates = candidates[:limit]
	}

	results := make([]VectorEntry, len(candidates))
	for i, c := range candidates {
		results[i] = c.entry
		if results[i].Metadata == nil {
			results[i].Metadata = make(map[string]any)
		}
		results[i].Metadata["similarity"] = c.similarity
	}

	return results, nil
}

// filterByContentType filters vector entries by content_type metadata field.
func filterByContentType(entries []VectorEntry, contentType string) []VectorEntry {
	out := make([]VectorEntry, 0, len(entries))
	for _, e := range entries {
		if e.Metadata != nil {
			if ct, ok := e.Metadata["content_type"].(string); ok && ct == contentType {
				out = append(out, e)
			}
		}
	}
	return out
}
