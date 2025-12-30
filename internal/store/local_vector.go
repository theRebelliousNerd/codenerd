package store

import (
	"codenerd/internal/logging"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// =============================================================================
// VECTOR STORE (Shard B: Vector/Associative Memory)
// =============================================================================

// VectorEntry represents a vector store entry.
type VectorEntry struct {
	ID         int64
	Content    string
	Metadata   map[string]interface{}
	CreatedAt  time.Time
	Similarity float64 // Cosine similarity score from vector search
}

// StoreVector stores content for semantic retrieval.
func (s *LocalStore) StoreVector(content string, metadata map[string]interface{}) error {
	timer := logging.StartTimer(logging.CategoryStore, "StoreVector")
	defer timer.Stop()

	s.mu.Lock()
	defer s.mu.Unlock()

	logging.StoreDebug("Storing vector content (length=%d bytes, metadata keys=%d)", len(content), len(metadata))

	metaJSON, _ := json.Marshal(metadata)

	_, err := s.db.Exec(
		"INSERT OR REPLACE INTO vectors (content, metadata) VALUES (?, ?)",
		content, string(metaJSON),
	)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to store vector: %v", err)
		return err
	}

	logging.StoreDebug("Vector stored successfully")
	return nil
}

// VectorRecall performs semantic search using keyword matching.
// In production, use actual vector embeddings with sqlite-vec.
func (s *LocalStore) VectorRecall(query string, limit int) ([]VectorEntry, error) {
	timer := logging.StartTimer(logging.CategoryStore, "VectorRecall")
	defer timer.Stop()

	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 10
	}

	logging.StoreDebug("Vector recall query: %q (limit=%d)", query, limit)

	// Simple keyword search (production would use vector similarity)
	keywords := strings.Fields(strings.ToLower(query))
	if len(keywords) == 0 {
		logging.StoreDebug("Empty query, returning nil")
		return nil, nil
	}

	// Build search query with LIKE for each keyword
	var conditions []string
	var args []interface{}
	for _, kw := range keywords {
		conditions = append(conditions, "LOWER(content) LIKE ?")
		args = append(args, "%"+kw+"%")
	}

	sqlQuery := fmt.Sprintf(
		"SELECT id, content, metadata, created_at FROM vectors WHERE %s ORDER BY created_at DESC LIMIT ?",
		strings.Join(conditions, " OR "),
	)
	args = append(args, limit)

	rows, err := s.db.Query(sqlQuery, args...)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Vector recall query failed: %v", err)
		return nil, err
	}
	defer rows.Close()

	var results []VectorEntry
	for rows.Next() {
		var entry VectorEntry
		var metaJSON string
		if err := rows.Scan(&entry.ID, &entry.Content, &metaJSON, &entry.CreatedAt); err != nil {
			continue
		}
		if metaJSON != "" {
			json.Unmarshal([]byte(metaJSON), &entry.Metadata)
		}
		results = append(results, entry)
	}

	logging.StoreDebug("Vector recall returned %d results", len(results))
	return results, nil
}
