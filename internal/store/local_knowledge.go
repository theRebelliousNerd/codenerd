package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"codenerd/internal/logging"
)

// =============================================================================
// KNOWLEDGE ATOMS
// =============================================================================

// KnowledgeAtom represents a piece of knowledge stored for agents.
type KnowledgeAtom struct {
	ID         int64
	Concept    string
	Content    string
	Source     string
	Confidence float64
	Tags       []string
	CreatedAt  time.Time
}

// KnowledgeStore wraps a LocalStore for knowledge-specific operations.
type KnowledgeStore struct {
	*LocalStore
}

// NewKnowledgeStore creates a new knowledge store at the given path.
func NewKnowledgeStore(dbPath string) (*KnowledgeStore, error) {
	ls, err := NewLocalStore(dbPath)
	if err != nil {
		return nil, err
	}
	return &KnowledgeStore{LocalStore: ls}, nil
}

// StoreKnowledgeAtom stores a knowledge atom for agent knowledge bases.
// This is used by Type 3 agents to persist their expertise.
func (s *LocalStore) StoreKnowledgeAtom(concept, content string, confidence float64) error {
	timer := logging.StartTimer(logging.CategoryStore, "StoreKnowledgeAtom")
	defer timer.Stop()

	s.mu.Lock()
	defer s.mu.Unlock()

	logging.StoreDebug("Storing knowledge atom: concept=%s content_len=%d confidence=%.2f", concept, len(content), confidence)

	// Ensure knowledge_atoms table exists
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS knowledge_atoms (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			concept TEXT NOT NULL,
			content TEXT NOT NULL,
			confidence REAL DEFAULT 1.0,
			content_hash TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to create knowledge_atoms table: %v", err)
		return fmt.Errorf("failed to create knowledge_atoms table: %w", err)
	}

	// Create index if not exists
	_, _ = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_atoms_concept ON knowledge_atoms(concept)`)

	// Compute content hash for deduplication
	contentHash := ComputeContentHash(concept, content)

	// Insert the knowledge atom with content_hash
	_, err = s.db.Exec(
		`INSERT INTO knowledge_atoms (concept, content, confidence, content_hash) VALUES (?, ?, ?, ?)`,
		concept, content, confidence, contentHash,
	)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to store knowledge atom %s: %v", concept, err)
		return err
	}

	logging.StoreDebug("Knowledge atom stored: concept=%s", concept)
	return nil
}

// ensureContentHashes backfills content_hash for any existing atoms that are missing it.
// This is called automatically on DB open to handle atoms created before the content_hash column was added.
func (s *LocalStore) ensureContentHashes() error {
	timer := logging.StartTimer(logging.CategoryStore, "ensureContentHashes")
	defer timer.Stop()

	// Check if knowledge_atoms table exists
	var tableExists int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='knowledge_atoms'").Scan(&tableExists); err != nil || tableExists == 0 {
		logging.StoreDebug("knowledge_atoms table does not exist, skipping backfill")
		return nil
	}

	// Check if content_hash column exists
	rows, err := s.db.Query("PRAGMA table_info(knowledge_atoms)")
	if err != nil {
		return fmt.Errorf("failed to get table info: %w", err)
	}
	hasContentHash := false
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dfltValue interface{}
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err != nil {
			continue
		}
		if name == "content_hash" {
			hasContentHash = true
			break
		}
	}
	rows.Close()

	if !hasContentHash {
		logging.StoreDebug("content_hash column does not exist, skipping backfill")
		return nil
	}

	// Count atoms missing content_hash
	var missingCount int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM knowledge_atoms WHERE content_hash IS NULL OR content_hash = ''").Scan(&missingCount); err != nil {
		return fmt.Errorf("failed to count missing hashes: %w", err)
	}

	if missingCount == 0 {
		logging.StoreDebug("All atoms have content_hash, no backfill needed")
		return nil
	}

	logging.Store("Backfilling content_hash for %d atoms", missingCount)

	// Fetch and update atoms missing content_hash
	atomRows, err := s.db.Query("SELECT id, concept, content FROM knowledge_atoms WHERE content_hash IS NULL OR content_hash = ''")
	if err != nil {
		return fmt.Errorf("failed to query atoms for backfill: %w", err)
	}
	defer atomRows.Close()

	type pendingUpdate struct {
		id   int64
		hash string
	}
	var pending []pendingUpdate
	for atomRows.Next() {
		var id int64
		var concept, content string
		if err := atomRows.Scan(&id, &concept, &content); err != nil {
			continue
		}
		pending = append(pending, pendingUpdate{
			id:   id,
			hash: ComputeContentHash(concept, content),
		})
	}
	// Close the read cursor before writing to avoid SQLITE_BUSY/locked errors.
	atomRows.Close()

	updated := 0
	if len(pending) > 0 {
		tx, txErr := s.db.Begin()
		if txErr != nil {
			return fmt.Errorf("failed to begin backfill transaction: %w", txErr)
		}
		stmt, prepErr := tx.Prepare("UPDATE knowledge_atoms SET content_hash = ? WHERE id = ?")
		if prepErr != nil {
			tx.Rollback()
			return fmt.Errorf("failed to prepare backfill update: %w", prepErr)
		}
		for _, u := range pending {
			if _, err := stmt.Exec(u.hash, u.id); err != nil {
				logging.Get(logging.CategoryStore).Warn("Failed to update hash for atom %d: %v", u.id, err)
				continue
			}
			updated++
		}
		stmt.Close()
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit backfill: %w", err)
		}
	}

	logging.Store("Backfilled content_hash for %d/%d atoms", updated, missingCount)
	return nil
}

// GetKnowledgeAtoms retrieves knowledge atoms by concept.
func (s *LocalStore) GetKnowledgeAtoms(concept string) ([]KnowledgeAtom, error) {
	timer := logging.StartTimer(logging.CategoryStore, "GetKnowledgeAtoms")
	defer timer.Stop()

	s.mu.RLock()
	defer s.mu.RUnlock()

	logging.StoreDebug("Retrieving knowledge atoms: concept=%s", concept)

	rows, err := s.db.Query(
		`SELECT id, concept, content, confidence, created_at FROM knowledge_atoms WHERE concept = ?`,
		concept,
	)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to query knowledge atoms for %s: %v", concept, err)
		return nil, err
	}
	defer rows.Close()

	var atoms []KnowledgeAtom
	for rows.Next() {
		var atom KnowledgeAtom
		if err := rows.Scan(&atom.ID, &atom.Concept, &atom.Content, &atom.Confidence, &atom.CreatedAt); err != nil {
			continue
		}
		atoms = append(atoms, atom)
	}

	logging.StoreDebug("Retrieved %d knowledge atoms for concept=%s", len(atoms), concept)
	return atoms, nil
}

// GetAllKnowledgeAtoms retrieves all knowledge atoms.
func (s *LocalStore) GetAllKnowledgeAtoms() ([]KnowledgeAtom, error) {
	timer := logging.StartTimer(logging.CategoryStore, "GetAllKnowledgeAtoms")
	defer timer.Stop()

	s.mu.RLock()
	defer s.mu.RUnlock()

	logging.StoreDebug("Retrieving all knowledge atoms")

	rows, err := s.db.Query(`SELECT id, concept, content, confidence, created_at FROM knowledge_atoms ORDER BY created_at DESC`)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to query all knowledge atoms: %v", err)
		return nil, err
	}
	defer rows.Close()

	var atoms []KnowledgeAtom
	for rows.Next() {
		var atom KnowledgeAtom
		if err := rows.Scan(&atom.ID, &atom.Concept, &atom.Content, &atom.Confidence, &atom.CreatedAt); err != nil {
			continue
		}
		atoms = append(atoms, atom)
	}

	logging.StoreDebug("Retrieved %d total knowledge atoms", len(atoms))
	return atoms, nil
}

// GetKnowledgeAtomsByPrefix retrieves knowledge atoms matching a concept prefix.
// Used for querying strategic knowledge (e.g., "strategic/%").
func (s *LocalStore) GetKnowledgeAtomsByPrefix(conceptPrefix string) ([]KnowledgeAtom, error) {
	timer := logging.StartTimer(logging.CategoryStore, "GetKnowledgeAtomsByPrefix")
	defer timer.Stop()

	s.mu.RLock()
	defer s.mu.RUnlock()

	logging.StoreDebug("Retrieving knowledge atoms with prefix: %s", conceptPrefix)

	// Use LIKE with % wildcard for prefix matching
	pattern := conceptPrefix + "%"
	rows, err := s.db.Query(
		`SELECT id, concept, content, confidence, created_at FROM knowledge_atoms WHERE concept LIKE ? ORDER BY confidence DESC`,
		pattern,
	)
	if err != nil {
		// Table may not exist yet if no atoms have been stored
		if strings.Contains(err.Error(), "no such table") {
			logging.StoreDebug("knowledge_atoms table does not exist yet, returning empty")
			return nil, nil
		}
		logging.Get(logging.CategoryStore).Error("Failed to query knowledge atoms by prefix %s: %v", conceptPrefix, err)
		return nil, err
	}
	defer rows.Close()

	var atoms []KnowledgeAtom
	for rows.Next() {
		var atom KnowledgeAtom
		if err := rows.Scan(&atom.ID, &atom.Concept, &atom.Content, &atom.Confidence, &atom.CreatedAt); err != nil {
			continue
		}
		atoms = append(atoms, atom)
	}

	logging.StoreDebug("Retrieved %d knowledge atoms for prefix=%s", len(atoms), conceptPrefix)
	return atoms, nil
}

// =============================================================================
// SEMANTIC KNOWLEDGE BRIDGE
// =============================================================================
// These methods enable semantic search over knowledge atoms by dual-storing
// them in both the knowledge_atoms table (for prefix queries) and the vectors
// table (for embedding-based semantic search).

// StoreKnowledgeAtomWithEmbedding stores a knowledge atom to BOTH the knowledge_atoms
// table AND the vectors table with embeddings for semantic search.
// This enables knowledge atoms to be:
// 1. Queried by exact concept prefix (via GetKnowledgeAtomsByPrefix)
// 2. Semantically searched (via SearchKnowledgeAtomsSemantic)
func (s *LocalStore) StoreKnowledgeAtomWithEmbedding(ctx context.Context, concept, content string, confidence float64) error {
	timer := logging.StartTimer(logging.CategoryStore, "StoreKnowledgeAtomWithEmbedding")
	defer timer.Stop()

	// 1. Store to knowledge_atoms table (existing behavior)
	if err := s.StoreKnowledgeAtom(concept, content, confidence); err != nil {
		return err
	}

	// 2. Also store to vectors table with embedding for semantic search
	// This makes knowledge atoms discoverable via VectorRecallSemanticFiltered
	if s.embeddingEngine != nil {
		metadata := map[string]interface{}{
			"content_type": "knowledge_atom",
			"concept":      concept,
			"confidence":   confidence,
		}
		if err := s.StoreVectorWithEmbedding(ctx, content, metadata); err != nil {
			// Log but don't fail - the knowledge atom is still stored
			logging.Get(logging.CategoryStore).Warn(
				"Failed to store embedding for knowledge atom %s: %v (atom still stored)",
				concept, err)
		} else {
			logging.StoreDebug("Knowledge atom stored with embedding: concept=%s", concept)
		}
	} else {
		logging.StoreDebug("Knowledge atom stored without embedding (no engine): concept=%s", concept)
	}

	return nil
}

// SearchKnowledgeAtomsSemantic performs semantic search over knowledge atoms.
// It uses the embedding engine to find atoms whose content is semantically
// similar to the query, regardless of exact keyword matches.
// Returns atoms sorted by semantic similarity (highest first).
func (s *LocalStore) SearchKnowledgeAtomsSemantic(ctx context.Context, query string, limit int) ([]KnowledgeAtom, error) {
	timer := logging.StartTimer(logging.CategoryStore, "SearchKnowledgeAtomsSemantic")
	defer timer.Stop()

	if s.embeddingEngine == nil {
		logging.StoreDebug("Semantic search unavailable: no embedding engine")
		return nil, nil
	}

	// Use VectorRecallSemanticFiltered to find knowledge atoms by content_type
	entries, err := s.VectorRecallSemanticFiltered(ctx, query, limit, "content_type", "knowledge_atom")
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Semantic search failed: %v", err)
		return nil, err
	}

	if len(entries) == 0 {
		logging.StoreDebug("Semantic search returned 0 results for query: %s", truncateForLog(query, 50))
		return nil, nil
	}

	// Convert VectorEntry → KnowledgeAtom using concept from metadata
	var atoms []KnowledgeAtom
	for _, entry := range entries {
		atom := KnowledgeAtom{
			Content: entry.Content,
		}

		// Extract concept from metadata if available
		if concept, ok := entry.Metadata["concept"].(string); ok {
			atom.Concept = concept
		}

		// Extract confidence from metadata if available
		if conf, ok := entry.Metadata["confidence"].(float64); ok {
			atom.Confidence = conf
		} else {
			atom.Confidence = 0.8 // Default confidence for semantic results
		}

		// Include similarity score in confidence (blend 70% original, 30% similarity)
		if similarity, ok := entry.Metadata["similarity"].(float64); ok {
			atom.Confidence = atom.Confidence*0.7 + similarity*0.3
		}

		atoms = append(atoms, atom)
	}

	logging.StoreDebug("Semantic search returned %d knowledge atoms for query: %s",
		len(atoms), truncateForLog(query, 50))
	return atoms, nil
}

// truncateForLog truncates a string for logging purposes.
func truncateForLog(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// StoreAtom stores a knowledge atom in the database.
func (ks *KnowledgeStore) StoreAtom(atom KnowledgeAtom) error {
	timer := logging.StartTimer(logging.CategoryStore, "KnowledgeStore.StoreAtom")
	defer timer.Stop()

	ks.mu.Lock()
	defer ks.mu.Unlock()

	logging.StoreDebug("Storing atom: concept=%s source=%s confidence=%.2f tags=%d",
		atom.Concept, atom.Source, atom.Confidence, len(atom.Tags))

	tagsJSON, err := json.Marshal(atom.Tags)
	if err != nil {
		tagsJSON = []byte("[]")
	}

	// Compute content hash for deduplication
	contentHash := ComputeContentHash(atom.Concept, atom.Content)

	_, err = ks.db.Exec(`
		INSERT INTO knowledge_atoms (concept, content, source, confidence, tags, created_at, content_hash)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		atom.Concept, atom.Content, atom.Source, atom.Confidence, string(tagsJSON), atom.CreatedAt.Format(time.RFC3339), contentHash)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to store atom %s: %v", atom.Concept, err)
		return err
	}

	logging.StoreDebug("Atom stored: concept=%s", atom.Concept)
	return nil
}
