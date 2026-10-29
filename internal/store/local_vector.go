package store

import (
	"codenerd/internal/logging"
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
	Metadata   map[string]any
	CreatedAt  time.Time
	Similarity float64 // Cosine similarity score from vector search
}

// StoreVector stores content for semantic retrieval.
func (s *LocalStore) StoreVector(content string, metadata map[string]any) error {
	timer := logging.StartTimer(logging.CategoryStore, "StoreVector")
	defer timer.Stop()

	s.mu.Lock()
	defer s.mu.Unlock()

	logging.StoreDebug("Storing vector content (length=%d bytes, metadata keys=%d)", len(content), len(metadata))

	metaJSON, err := encodeRowMetadata(metadata)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Refusing to store vector: %v", err)
		return err
	}

	_, err = s.db.Exec(
		"INSERT OR REPLACE INTO vectors (content, metadata) VALUES (?, ?)",
		content, metaJSON,
	)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to store vector: %v", err)
		return err
	}

	logging.StoreDebug("Vector stored successfully")
	return nil
}

// maxVectorRecallKeywords bounds the keyword fallback's WHERE clause.
// Each keyword becomes one LOWER(content) LIKE ? OR branch, and SQLite
// rejects expression trees deeper than 1000. A multi-paragraph campaign
// goal tokenizes to thousands of keywords, so an unbounded OR chain fails
// every hydrate with "Expression tree is too large". Capping well below
// the limit keeps recall working for a query of any length.
const maxVectorRecallKeywords = 32

// maxVectorRecallKeywordRunes truncates a single absurdly long token so one
// pasted blob cannot turn its LIKE pattern into a full-table scan stall.
const maxVectorRecallKeywordRunes = 64

// VectorRecall performs keyword fallback search when no embedding engine is
// configured. Semantic search lives in VectorRecallSemantic; this stays
// keyword-only for callers without an engine.
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

	// Deduplicate and bound: keep the first distinct keywords so a query of
	// any length produces at most maxVectorRecallKeywords OR branches.
	seen := make(map[string]struct{}, len(keywords))
	bounded := make([]string, 0, min(len(keywords), maxVectorRecallKeywords))
	for _, kw := range keywords {
		if _, dup := seen[kw]; dup {
			continue
		}
		seen[kw] = struct{}{}
		if len([]rune(kw)) > maxVectorRecallKeywordRunes {
			kw = string([]rune(kw)[:maxVectorRecallKeywordRunes])
			if _, dup := seen[kw]; dup {
				continue
			}
			seen[kw] = struct{}{}
		}
		bounded = append(bounded, kw)
		if len(bounded) >= maxVectorRecallKeywords {
			break
		}
	}
	if len(bounded) == 0 {
		logging.StoreDebug("Empty query after dedupe, returning nil")
		return nil, nil
	}
	if len(bounded) < len(seen) {
		logging.StoreDebug("Vector recall truncated %d distinct keywords to %d", len(seen), len(bounded))
	}

	// Build search query with LIKE for each keyword
	var conditions []string
	var args []any
	for _, kw := range bounded {
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
		entry.Metadata = decodeRowMetadata(entry.ID, []byte(metaJSON))
		results = append(results, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	logging.StoreDebug("Vector recall returned %d results", len(results))
	return results, nil
}
