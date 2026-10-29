package sync

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"codenerd/internal/embedding"
	"codenerd/internal/logging"
	"codenerd/internal/store"
)

type baselineSeedAtom struct {
	concept    string
	content    string
	confidence float64
}

func baselineSeedAtoms(agentID string) []baselineSeedAtom {
	name := strings.TrimSpace(agentID)
	if name == "" {
		name = "specialist"
	}
	return []baselineSeedAtom{
		{concept: "agent_identity", content: fmt.Sprintf("You are %s, a specialist agent in the codeNERD ecosystem.", name), confidence: 0.9},
		{concept: "agent_mission", content: fmt.Sprintf("%s exists to carry out %s tasks in this workspace.", name, strings.ToLower(name)), confidence: 0.85},
		{concept: "methodology/overview", content: fmt.Sprintf("%s methodology: decompose the task, locate the relevant code and docs, apply best practices, verify with the project's build and tests.", name), confidence: 0.8},
	}
}

func agentIDFromDBPath(dbPath string) string {
	base := filepath.Base(dbPath)
	if !strings.HasSuffix(base, "_knowledge.db") {
		return ""
	}
	return strings.TrimSuffix(base, "_knowledge.db")
}

func insertBaselineAtom(ctx context.Context, db *sql.DB, atom baselineSeedAtom) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	hash := store.ComputeContentHash(atom.concept, atom.content)
	_, err := db.Exec(
		`INSERT INTO knowledge_atoms (concept, content, confidence, content_hash) VALUES (?, ?, ?, ?)`,
		atom.concept, atom.content, atom.confidence, hash,
	)
	if err != nil {
		return fmt.Errorf("seed baseline atom %s: %w", atom.concept, err)
	}
	return nil
}

func ensureSeededBaseline(ctx context.Context, db *sql.DB, agentID string) (int, error) {
	if db == nil {
		return 0, nil
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM knowledge_atoms`).Scan(&count); err != nil {
		return 0, nil
	}
	if count > 0 {
		return 0, nil
	}
	name := strings.TrimSpace(agentID)
	if name == "" {
		name = "specialist"
	}
	seeded := 0
	for _, atom := range baselineSeedAtoms(name) {
		if err := insertBaselineAtom(ctx, db, atom); err != nil {
			return seeded, err
		}
		seeded++
	}
	if seeded > 0 {
		logging.Get(logging.CategoryStore).Info("Knowledge seed: added %d baseline atoms for %s", seeded, name)
	}
	return seeded, nil
}

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

// backfillKnowledgeVectors ensures every row in knowledge_atoms has a matching
// vectors row with content_type=knowledge_atom. It also seeds empty KBs with
// baseline atoms so existing agents heal on restart without rerunning init.
// It reuses the caller's open DB instead of opening a second connection.
func backfillKnowledgeVectors(ctx context.Context, db *sql.DB, dbPath string, engine embedding.EmbeddingEngine) error {
	if db == nil {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := ensureSeededBaseline(ctx, db, agentIDFromDBPath(dbPath)); err != nil {
		return err
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
