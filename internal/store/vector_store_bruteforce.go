package store

import (
	"cmp"
	"codenerd/internal/embedding"
	"codenerd/internal/logging"
	"database/sql"
	"fmt"
	"slices"
)

// bruteForceRowsToEntries is the shared scan/parse/score/sort/truncate tail
// for the three brute-force fallbacks. Each used to hand-roll this loop,
// which is how the metadata-filtered one grew a lossy LIKE prefilter and
// none of them checked rows.Err.
func bruteForceRowsToEntries(rows *sql.Rows, queryEmbedding []float32, limit int, keep func(VectorEntry) bool) ([]VectorEntry, error) {
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

		entry.Metadata = decodeRowMetadata(entry.ID, metaJSON)
		if keep != nil && !keep(entry) {
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
	if err := rows.Err(); err != nil {
		return nil, err
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

	results, err := bruteForceRowsToEntries(rows, queryEmbedding, limit, nil)
	if err != nil {
		return nil, err
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

	return bruteForceRowsToEntries(rows, queryEmbedding, limit, nil)
}

// vectorRecallBruteForceFiltered is the fallback cosine similarity search with metadata filtering.
func (s *LocalStore) vectorRecallBruteForceFiltered(queryText string, queryEmbedding []float32, limit int, metaKey string, metaValue any) ([]VectorEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	queryStr := "SELECT id, content, embedding, metadata, created_at FROM vectors WHERE embedding IS NOT NULL"
	var rows *sql.Rows
	var err error

	// The prefilter uses json_extract, matching the ANN path and the count
	// helpers. It used to be `metadata LIKE '%"key":"value"%'`, which silently
	// dropped every row whose value is not a JSON string: a numeric {"n": 5}
	// never matches '%"n":"5"%', so filtered brute-force search lost numeric
	// rows the ANN path found.
	if metaKey != "" && metaValue != nil {
		path := fmt.Sprintf(`$."%s"`, metaKey)
		rows, err = s.db.Query(queryStr+" AND json_extract(metadata, ?) = ?", path, metaValue)
	} else {
		rows, err = s.db.Query(queryStr)
	}
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to query vectors: %v", err)
		return nil, err
	}
	defer rows.Close()

	return bruteForceRowsToEntries(rows, queryEmbedding, limit, func(e VectorEntry) bool {
		return matchesMetadata(e.Metadata, metaKey, metaValue)
	})
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
