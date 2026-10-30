package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"

	"codenerd/internal/logging"
)

// BackfillKnowledgeAtomVectors embeds every knowledge_atoms row that lacks a
// vectors row with content_type=knowledge_atom. It returns how many vectors
// were added. It is idempotent: atoms that already have a vector are skipped.
// An atom with no text has nothing to embed and is skipped too: it is kept
// (an empty paragraph keeps a split body joinable) but never sent to the
// embedder, which refuses it, so it used to be retried and logged as a
// failure on every boot.
func (s *LocalStore) BackfillKnowledgeAtomVectors(ctx context.Context) (int, error) {
	s.mu.RLock()
	hasEngine := s.embeddingEngine != nil
	s.mu.RUnlock()
	if !hasEngine {
		return 0, nil
	}

	atoms, err := s.GetAllKnowledgeAtoms()
	if err != nil {
		return 0, err
	}
	if len(atoms) == 0 {
		return 0, nil
	}

	embedded := make(map[string]bool, len(atoms))
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()
	if db == nil {
		return 0, nil
	}
	rows, err := db.Query(`SELECT content, metadata FROM vectors`)
	if err != nil {
		// No vectors table yet: every atom needs embedding.
		rows = nil
	} else {
		defer rows.Close()
		for rows.Next() {
			var content string
			var meta sql.NullString
			if err := rows.Scan(&content, &meta); err != nil {
				continue
			}
			if !meta.Valid || meta.String == "" {
				continue
			}
			var m map[string]any
			if err := json.Unmarshal([]byte(meta.String), &m); err != nil {
				continue
			}
			if ctype, _ := m["content_type"].(string); ctype != "knowledge_atom" {
				continue
			}
			concept, _ := m["concept"].(string)
			if concept == "" {
				continue
			}
			embedded[concept+"\x00"+content] = true
		}
	}

	added := 0
	for _, a := range atoms {
		if ctx.Err() != nil {
			return added, ctx.Err()
		}
		if embedded[a.Concept+"\x00"+a.Content] || strings.TrimSpace(a.Content) == "" {
			continue
		}
		if err := s.StoreVectorWithEmbedding(ctx, a.Content, map[string]any{
			"content_type": "knowledge_atom",
			"concept":      a.Concept,
			"confidence":   a.Confidence,
		}); err != nil {
			logging.Get(logging.CategoryStore).Warn("Backfill vector for %s failed: %v", a.Concept, err)
			continue
		}
		embedded[a.Concept+"\x00"+a.Content] = true
		added++
	}
	return added, nil
}
