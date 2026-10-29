package init

import (
	"context"
	"database/sql"
	"encoding/json"

	"codenerd/internal/logging"
	"codenerd/internal/store"
)

// backfillKnowledgeVectors ensures every knowledge_atoms row has a matching
// vectors row with content_type=knowledge_atom so semantic search can find it.
// This repairs KBs written through StoreKnowledgeAtom (no embedding), which is
// how bubbleteaexpert ended with 73 atoms and 0 vectors.
func backfillKnowledgeVectors(ctx context.Context, db *store.LocalStore) (int, error) {
	atoms, err := db.GetAllKnowledgeAtoms()
	if err != nil {
		return 0, err
	}
	if len(atoms) == 0 {
		return 0, nil
	}
	embedded := listEmbeddedKnowledgeKeys(db)
	added := 0
	for _, atom := range atoms {
		if ctx.Err() != nil {
			return added, ctx.Err()
		}
		key := atom.Concept + "\x00" + atom.Content
		if embedded[key] {
			continue
		}
		if err := db.StoreVectorWithEmbedding(ctx, atom.Content, map[string]any{
			"content_type": "knowledge_atom",
			"concept":      atom.Concept,
			"confidence":   atom.Confidence,
		}); err != nil {
			logging.Boot("Warning: backfill vector for %s failed: %v", atom.Concept, err)
			continue
		}
		embedded[key] = true
		added++
	}
	return added, nil
}

// listEmbeddedKnowledgeKeys returns the set of knowledge atoms that already
// have a vectors row. It reads the vectors table directly and filters in Go
// so it works regardless of the metadata JSON shape.
func listEmbeddedKnowledgeKeys(db *store.LocalStore) map[string]bool {
	out := make(map[string]bool)
	conn := db.GetDB()
	if conn == nil {
		return out
	}
	rows, err := conn.Query(`SELECT content, metadata FROM vectors`)
	if err != nil {
		return out
	}
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
		ctype, _ := m["content_type"].(string)
		if ctype != "knowledge_atom" {
			continue
		}
		concept, _ := m["concept"].(string)
		if concept == "" {
			continue
		}
		out[concept+"\x00"+content] = true
	}
	return out
}
