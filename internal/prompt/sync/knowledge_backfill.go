package sync

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"codenerd/internal/embedding"
	"codenerd/internal/knowledge"
	"codenerd/internal/logging"
)

type knowledgeAtomRow struct {
	concept    string
	content    string
	confidence float64
}

func fetchKnowledgeAtoms(db *sql.DB) ([]knowledgeAtomRow, error) {
	rows, err := db.Query(`SELECT concept, content, confidence FROM knowledge_atoms`)
	if err != nil {
		return nil, nil
	}
	defer rows.Close()
	var atoms []knowledgeAtomRow
	for rows.Next() {
		var a knowledgeAtomRow
		if err := rows.Scan(&a.concept, &a.content, &a.confidence); err != nil {
			continue
		}
		atoms = append(atoms, a)
	}
	if err := rows.Err(); err != nil {
		return atoms, err
	}
	return atoms, nil
}

func listEmbeddedKnowledgeKeys(db *sql.DB) map[string]bool {
	out := make(map[string]bool)
	if db == nil {
		return out
	}
	rows, err := db.Query(`SELECT content, metadata FROM vectors`)
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

func storeVectorForAtom(ctx context.Context, db *sql.DB, engine embedding.EmbeddingEngine, model string, a knowledgeAtomRow) error {
	vec, err := engine.Embed(ctx, a.content)
	if err != nil {
		return err
	}
	embJSON, err := json.Marshal(vec)
	if err != nil {
		return err
	}
	metaJSON, err := json.Marshal(map[string]any{
		"content_type": "knowledge_atom",
		"concept":      a.concept,
		"confidence":   a.confidence,
	})
	if err != nil {
		return err
	}
	_, err = db.Exec(
		`INSERT OR REPLACE INTO vectors (content, embedding, embedding_model, metadata) VALUES (?, ?, ?, ?)`,
		a.content, string(embJSON), model, string(metaJSON),
	)
	return err
}

func backfillMissingVectors(ctx context.Context, db *sql.DB, engine embedding.EmbeddingEngine, atoms []knowledgeAtomRow, embedded map[string]bool) int {
	model := engine.Name()
	done := 0
	for _, a := range atoms {
		if ctx.Err() != nil {
			return done
		}
		if embedded[a.concept+"\x00"+a.content] {
			continue
		}
		if err := storeVectorForAtom(ctx, db, engine, model, a); err != nil {
			logging.Get(logging.CategoryStore).Warn("Backfill vector for %s failed: %v", a.concept, err)
			continue
		}
		embedded[a.concept+"\x00"+a.content] = true
		done++
	}
	return done
}

// backfillKnowledgeVectors heals an agent KB on every boot: it prunes the
// removed seed's placeholder rows (internal/knowledge), which taught nothing
// and made unresearched topics read as covered, and ensures every remaining
// knowledge_atoms row has a matching vectors row with
// content_type=knowledge_atom. It writes no knowledge of its own: an empty KB
// stays empty until a knowledge source teaches it. It reuses the caller's
// open DB instead of opening a second connection.
func backfillKnowledgeVectors(ctx context.Context, db *sql.DB, dbPath string, engine embedding.EmbeddingEngine) error {
	if db == nil {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if n, err := knowledge.PrunePlaceholders(ctx, db); err != nil {
		return fmt.Errorf("prune placeholder knowledge in %s: %w", dbPath, err)
	} else if n > 0 {
		logging.Get(logging.CategoryStore).Info("Knowledge heal: pruned %d placeholder atom(s) from %s", n, dbPath)
	}
	if engine == nil {
		return nil
	}
	atoms, err := fetchKnowledgeAtoms(db)
	if err != nil {
		return err
	}
	if len(atoms) == 0 {
		return nil
	}
	embedded := listEmbeddedKnowledgeKeys(db)
	if n := backfillMissingVectors(ctx, db, engine, atoms, embedded); n > 0 {
		logging.Get(logging.CategoryStore).Info("Knowledge backfill: embedded %d atoms in %s", n, dbPath)
	}
	return nil
}
