package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// LocalStore implements Shards B, C, and D using SQLite.
// Shard B: Vector/Associative Memory (semantic search)
// Shard C: Knowledge Graph (relational links)
// Shard D: Cold Storage (persistent facts and preferences)
type LocalStore struct {
	db     *sql.DB
	mu     sync.RWMutex
	dbPath string
}

// NewLocalStore initializes the SQLite database at the given path.
func NewLocalStore(path string) (*LocalStore, error) {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	store := &LocalStore{db: db, dbPath: path}
	if err := store.initialize(); err != nil {
		db.Close()
		return nil, err
	}

	return store, nil
}

// initialize creates the required tables.
func (s *LocalStore) initialize() error {
	// Shard B: Vector Store (simplified - using keyword search without external vector lib)
	// In production, use sqlite-vec extension
	vectorTable := `
	CREATE TABLE IF NOT EXISTS vectors (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		content TEXT NOT NULL,
		embedding TEXT,
		metadata TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_vectors_content ON vectors(content);
	`

	// Shard C: Knowledge Graph
	graphTable := `
	CREATE TABLE IF NOT EXISTS knowledge_graph (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		entity_a TEXT NOT NULL,
		relation TEXT NOT NULL,
		entity_b TEXT NOT NULL,
		weight REAL DEFAULT 1.0,
		metadata TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(entity_a, relation, entity_b)
	);
	CREATE INDEX IF NOT EXISTS idx_kg_entity_a ON knowledge_graph(entity_a);
	CREATE INDEX IF NOT EXISTS idx_kg_entity_b ON knowledge_graph(entity_b);
	CREATE INDEX IF NOT EXISTS idx_kg_relation ON knowledge_graph(relation);
	`

	// Shard D: Cold Storage (Facts and Preferences)
	coldTable := `
	CREATE TABLE IF NOT EXISTS cold_storage (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		predicate TEXT NOT NULL,
		args TEXT NOT NULL,
		fact_type TEXT DEFAULT 'fact',
		priority INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(predicate, args)
	);
	CREATE INDEX IF NOT EXISTS idx_cold_predicate ON cold_storage(predicate);
	CREATE INDEX IF NOT EXISTS idx_cold_type ON cold_storage(fact_type);
	`

	// Activation log for spreading activation
	activationTable := `
	CREATE TABLE IF NOT EXISTS activation_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		fact_id TEXT NOT NULL,
		activation_score REAL NOT NULL,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_activation_fact ON activation_log(fact_id);
	`

	// Session history for context
	sessionTable := `
	CREATE TABLE IF NOT EXISTS session_history (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id TEXT NOT NULL,
		turn_number INTEGER NOT NULL,
		user_input TEXT,
		intent_json TEXT,
		response TEXT,
		atoms_json TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_session ON session_history(session_id);
	`

	for _, table := range []string{vectorTable, graphTable, coldTable, activationTable, sessionTable} {
		if _, err := s.db.Exec(table); err != nil {
			return fmt.Errorf("failed to create table: %w", err)
		}
	}

	return nil
}

// Close closes the database connection.
func (s *LocalStore) Close() error {
	return s.db.Close()
}

// ========== Shard B: Vector/Associative Memory ==========

// VectorEntry represents a vector store entry.
type VectorEntry struct {
	ID        int64
	Content   string
	Metadata  map[string]interface{}
	CreatedAt time.Time
}

// StoreVector stores content for semantic retrieval.
func (s *LocalStore) StoreVector(content string, metadata map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	metaJSON, _ := json.Marshal(metadata)

	_, err := s.db.Exec(
		"INSERT OR REPLACE INTO vectors (content, metadata) VALUES (?, ?)",
		content, string(metaJSON),
	)
	return err
}

// VectorRecall performs semantic search using keyword matching.
// In production, use actual vector embeddings with sqlite-vec.
func (s *LocalStore) VectorRecall(query string, limit int) ([]VectorEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 10
	}

	// Simple keyword search (production would use vector similarity)
	keywords := strings.Fields(strings.ToLower(query))
	if len(keywords) == 0 {
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

	return results, nil
}

// ========== Shard C: Knowledge Graph ==========

// KnowledgeLink represents a graph edge.
type KnowledgeLink struct {
	EntityA  string
	Relation string
	EntityB  string
	Weight   float64
	Metadata map[string]interface{}
}

// StoreLink stores a knowledge graph edge.
func (s *LocalStore) StoreLink(entityA, relation, entityB string, weight float64, metadata map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	metaJSON, _ := json.Marshal(metadata)

	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO knowledge_graph (entity_a, relation, entity_b, weight, metadata)
		 VALUES (?, ?, ?, ?, ?)`,
		entityA, relation, entityB, weight, string(metaJSON),
	)
	return err
}

// QueryLinks retrieves links for an entity.
func (s *LocalStore) QueryLinks(entity string, direction string) ([]KnowledgeLink, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var query string
	switch direction {
	case "outgoing":
		query = "SELECT entity_a, relation, entity_b, weight, metadata FROM knowledge_graph WHERE entity_a = ?"
	case "incoming":
		query = "SELECT entity_a, relation, entity_b, weight, metadata FROM knowledge_graph WHERE entity_b = ?"
	default: // both
		query = "SELECT entity_a, relation, entity_b, weight, metadata FROM knowledge_graph WHERE entity_a = ? OR entity_b = ?"
	}

	var args []interface{}
	if direction == "both" {
		args = []interface{}{entity, entity}
	} else {
		args = []interface{}{entity}
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var links []KnowledgeLink
	for rows.Next() {
		var link KnowledgeLink
		var metaJSON string
		if err := rows.Scan(&link.EntityA, &link.Relation, &link.EntityB, &link.Weight, &metaJSON); err != nil {
			continue
		}
		if metaJSON != "" {
			json.Unmarshal([]byte(metaJSON), &link.Metadata)
		}
		links = append(links, link)
	}

	return links, nil
}

// TraversePath finds a path between two entities using BFS.
func (s *LocalStore) TraversePath(from, to string, maxDepth int) ([]KnowledgeLink, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if maxDepth <= 0 {
		maxDepth = 5
	}

	// BFS traversal
	type pathNode struct {
		entity string
		path   []KnowledgeLink
	}

	visited := make(map[string]bool)
	queue := []pathNode{{entity: from, path: nil}}

	for len(queue) > 0 && len(queue[0].path) < maxDepth {
		current := queue[0]
		queue = queue[1:]

		if visited[current.entity] {
			continue
		}
		visited[current.entity] = true

		if current.entity == to {
			return current.path, nil
		}

		links, err := s.QueryLinks(current.entity, "outgoing")
		if err != nil {
			continue
		}

		for _, link := range links {
			if !visited[link.EntityB] {
				newPath := make([]KnowledgeLink, len(current.path)+1)
				copy(newPath, current.path)
				newPath[len(current.path)] = link
				queue = append(queue, pathNode{entity: link.EntityB, path: newPath})
			}
		}
	}

	return nil, fmt.Errorf("no path found from %s to %s", from, to)
}

// ========== Shard D: Cold Storage ==========

// StoredFact represents a persisted fact.
type StoredFact struct {
	ID        int64
	Predicate string
	Args      []interface{}
	FactType  string
	Priority  int
	CreatedAt time.Time
	UpdatedAt time.Time
}

// StoreFact persists a fact to cold storage.
func (s *LocalStore) StoreFact(predicate string, args []interface{}, factType string, priority int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	argsJSON, _ := json.Marshal(args)

	_, err := s.db.Exec(
		`INSERT INTO cold_storage (predicate, args, fact_type, priority, updated_at)
		 VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(predicate, args) DO UPDATE SET
		 fact_type = excluded.fact_type,
		 priority = excluded.priority,
		 updated_at = CURRENT_TIMESTAMP`,
		predicate, string(argsJSON), factType, priority,
	)
	return err
}

// LoadFacts retrieves facts by predicate.
func (s *LocalStore) LoadFacts(predicate string) ([]StoredFact, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(
		"SELECT id, predicate, args, fact_type, priority, created_at, updated_at FROM cold_storage WHERE predicate = ? ORDER BY priority DESC",
		predicate,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var facts []StoredFact
	for rows.Next() {
		var fact StoredFact
		var argsJSON string
		if err := rows.Scan(&fact.ID, &fact.Predicate, &argsJSON, &fact.FactType, &fact.Priority, &fact.CreatedAt, &fact.UpdatedAt); err != nil {
			continue
		}
		json.Unmarshal([]byte(argsJSON), &fact.Args)
		facts = append(facts, fact)
	}

	return facts, nil
}

// LoadAllFacts retrieves all facts, optionally filtered by type.
func (s *LocalStore) LoadAllFacts(factType string) ([]StoredFact, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var query string
	var args []interface{}

	if factType != "" {
		query = "SELECT id, predicate, args, fact_type, priority, created_at, updated_at FROM cold_storage WHERE fact_type = ? ORDER BY priority DESC"
		args = []interface{}{factType}
	} else {
		query = "SELECT id, predicate, args, fact_type, priority, created_at, updated_at FROM cold_storage ORDER BY priority DESC"
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var facts []StoredFact
	for rows.Next() {
		var fact StoredFact
		var argsJSON string
		if err := rows.Scan(&fact.ID, &fact.Predicate, &argsJSON, &fact.FactType, &fact.Priority, &fact.CreatedAt, &fact.UpdatedAt); err != nil {
			continue
		}
		json.Unmarshal([]byte(argsJSON), &fact.Args)
		facts = append(facts, fact)
	}

	return facts, nil
}

// DeleteFact removes a fact by predicate and args.
func (s *LocalStore) DeleteFact(predicate string, args []interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	argsJSON, _ := json.Marshal(args)
	_, err := s.db.Exec("DELETE FROM cold_storage WHERE predicate = ? AND args = ?", predicate, string(argsJSON))
	return err
}

// ========== Spreading Activation ==========

// LogActivation records activation scores for facts.
func (s *LocalStore) LogActivation(factID string, score float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(
		"INSERT INTO activation_log (fact_id, activation_score) VALUES (?, ?)",
		factID, score,
	)
	return err
}

// GetRecentActivations retrieves recent activation scores.
func (s *LocalStore) GetRecentActivations(limit int, minScore float64) (map[string]float64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 100
	}

	rows, err := s.db.Query(
		`SELECT fact_id, MAX(activation_score) as max_score
		 FROM activation_log
		 WHERE timestamp > datetime('now', '-1 hour')
		 GROUP BY fact_id
		 HAVING max_score >= ?
		 ORDER BY max_score DESC
		 LIMIT ?`,
		minScore, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	activations := make(map[string]float64)
	for rows.Next() {
		var factID string
		var score float64
		if err := rows.Scan(&factID, &score); err != nil {
			continue
		}
		activations[factID] = score
	}

	return activations, nil
}

// ========== Session History ==========

// StoreSessionTurn records a conversation turn.
func (s *LocalStore) StoreSessionTurn(sessionID string, turnNumber int, userInput, intentJSON, response, atomsJSON string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(
		`INSERT INTO session_history (session_id, turn_number, user_input, intent_json, response, atoms_json)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		sessionID, turnNumber, userInput, intentJSON, response, atomsJSON,
	)
	return err
}

// GetSessionHistory retrieves session history.
func (s *LocalStore) GetSessionHistory(sessionID string, limit int) ([]map[string]interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 50
	}

	rows, err := s.db.Query(
		`SELECT turn_number, user_input, intent_json, response, atoms_json, created_at
		 FROM session_history
		 WHERE session_id = ?
		 ORDER BY turn_number DESC
		 LIMIT ?`,
		sessionID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []map[string]interface{}
	for rows.Next() {
		var turnNumber int
		var userInput, intentJSON, response, atomsJSON string
		var createdAt time.Time
		if err := rows.Scan(&turnNumber, &userInput, &intentJSON, &response, &atomsJSON, &createdAt); err != nil {
			continue
		}
		history = append(history, map[string]interface{}{
			"turn_number": turnNumber,
			"user_input":  userInput,
			"intent":      intentJSON,
			"response":    response,
			"atoms":       atomsJSON,
			"timestamp":   createdAt,
		})
	}

	return history, nil
}

// ========== Utility Functions ==========

// CosineSimilarity computes cosine similarity between two vectors.
func CosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

// GetStats returns database statistics.
func (s *LocalStore) GetStats() (map[string]int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := make(map[string]int64)
	tables := []string{"vectors", "knowledge_graph", "cold_storage", "activation_log", "session_history", "knowledge_atoms"}

	for _, table := range tables {
		var count int64
		err := s.db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", table)).Scan(&count)
		if err != nil {
			continue
		}
		stats[table] = count
	}

	return stats, nil
}

// StoreKnowledgeAtom stores a knowledge atom for agent knowledge bases.
// This is used by Type 3 agents to persist their expertise.
func (s *LocalStore) StoreKnowledgeAtom(concept, content string, confidence float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Ensure knowledge_atoms table exists
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS knowledge_atoms (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			concept TEXT NOT NULL,
			content TEXT NOT NULL,
			confidence REAL DEFAULT 1.0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create knowledge_atoms table: %w", err)
	}

	// Create index if not exists
	_, _ = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_atoms_concept ON knowledge_atoms(concept)`)

	// Insert the knowledge atom
	_, err = s.db.Exec(
		`INSERT INTO knowledge_atoms (concept, content, confidence) VALUES (?, ?, ?)`,
		concept, content, confidence,
	)
	return err
}

// GetKnowledgeAtoms retrieves knowledge atoms by concept.
func (s *LocalStore) GetKnowledgeAtoms(concept string) ([]KnowledgeAtom, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(
		`SELECT id, concept, content, confidence, created_at FROM knowledge_atoms WHERE concept = ?`,
		concept,
	)
	if err != nil {
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

	return atoms, nil
}

// GetAllKnowledgeAtoms retrieves all knowledge atoms.
func (s *LocalStore) GetAllKnowledgeAtoms() ([]KnowledgeAtom, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`SELECT id, concept, content, confidence, created_at FROM knowledge_atoms ORDER BY created_at DESC`)
	if err != nil {
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

	return atoms, nil
}

// KnowledgeAtom represents a piece of knowledge stored for agents.
type KnowledgeAtom struct {
	ID         int64
	Concept    string
	Content    string
	Confidence float64
	CreatedAt  string
}
