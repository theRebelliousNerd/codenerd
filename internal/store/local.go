package store

import (
	"codenerd/internal/embedding"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
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
	db              *sql.DB
	mu              sync.RWMutex
	dbPath          string
	embeddingEngine embedding.EmbeddingEngine // Optional embedding engine for semantic search
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
	// UNIQUE constraint on (session_id, turn_number) enables idempotent sync
	sessionTable := `
	CREATE TABLE IF NOT EXISTS session_history (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id TEXT NOT NULL,
		turn_number INTEGER NOT NULL,
		user_input TEXT,
		intent_json TEXT,
		response TEXT,
		atoms_json TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(session_id, turn_number)
	);
	CREATE INDEX IF NOT EXISTS idx_session ON session_history(session_id);
	`

	// Task verification history for learning from retry loops
	verificationTable := `
	CREATE TABLE IF NOT EXISTS task_verifications (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id TEXT NOT NULL,
		turn_number INTEGER NOT NULL,
		task TEXT NOT NULL,
		shard_type TEXT NOT NULL,
		attempt_number INTEGER NOT NULL,
		success BOOLEAN NOT NULL,
		confidence REAL,
		reason TEXT,
		quality_violations TEXT,
		corrective_action TEXT,
		evidence TEXT,
		result_hash TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_verifications_session ON task_verifications(session_id);
	CREATE INDEX IF NOT EXISTS idx_verifications_success ON task_verifications(success);
	CREATE INDEX IF NOT EXISTS idx_verifications_shard ON task_verifications(shard_type);
	`

	// Reasoning traces for shard LLM interactions (Task 4)
	reasoningTracesTable := `
	CREATE TABLE IF NOT EXISTS reasoning_traces (
		id TEXT PRIMARY KEY,
		shard_id TEXT NOT NULL,
		shard_type TEXT NOT NULL,
		shard_category TEXT NOT NULL,
		session_id TEXT NOT NULL,
		task_context TEXT,
		system_prompt TEXT NOT NULL,
		user_prompt TEXT NOT NULL,
		response TEXT NOT NULL,
		model TEXT,
		tokens_used INTEGER,
		duration_ms INTEGER,
		success BOOLEAN NOT NULL,
		error_message TEXT,
		quality_score REAL,
		learning_notes TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_traces_shard_type ON reasoning_traces(shard_type);
	CREATE INDEX IF NOT EXISTS idx_traces_session ON reasoning_traces(session_id);
	CREATE INDEX IF NOT EXISTS idx_traces_shard_id ON reasoning_traces(shard_id);
	CREATE INDEX IF NOT EXISTS idx_traces_success ON reasoning_traces(success);
	CREATE INDEX IF NOT EXISTS idx_traces_created ON reasoning_traces(created_at);
	CREATE INDEX IF NOT EXISTS idx_traces_category ON reasoning_traces(shard_category);
	`

	for _, table := range []string{vectorTable, graphTable, coldTable, activationTable, sessionTable, verificationTable, reasoningTracesTable} {
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
// Uses INSERT OR IGNORE for idempotent syncing (duplicate turns are silently skipped).
func (s *LocalStore) StoreSessionTurn(sessionID string, turnNumber int, userInput, intentJSON, response, atomsJSON string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO session_history (session_id, turn_number, user_input, intent_json, response, atoms_json)
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

// ========== Task Verification ==========

// StoreVerification records a verification attempt for learning.
func (s *LocalStore) StoreVerification(
	sessionID string,
	turnNumber int,
	task string,
	shardType string,
	attemptNumber int,
	success bool,
	confidence float64,
	reason string,
	violationsJSON string,
	correctiveJSON string,
	evidenceJSON string,
	resultHash string,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(
		`INSERT INTO task_verifications
		 (session_id, turn_number, task, shard_type, attempt_number, success, confidence, reason, quality_violations, corrective_action, evidence, result_hash)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		sessionID, turnNumber, task, shardType, attemptNumber, success, confidence, reason, violationsJSON, correctiveJSON, evidenceJSON, resultHash,
	)
	return err
}

// GetVerificationHistory retrieves verification attempts for a session.
func (s *LocalStore) GetVerificationHistory(sessionID string, limit int) ([]VerificationRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 50
	}

	rows, err := s.db.Query(
		`SELECT id, session_id, turn_number, task, shard_type, attempt_number, success, confidence, reason, quality_violations, corrective_action, evidence, result_hash, created_at
		 FROM task_verifications
		 WHERE session_id = ?
		 ORDER BY created_at DESC
		 LIMIT ?`,
		sessionID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []VerificationRecord
	for rows.Next() {
		var rec VerificationRecord
		if err := rows.Scan(
			&rec.ID, &rec.SessionID, &rec.TurnNumber, &rec.Task, &rec.ShardType,
			&rec.AttemptNumber, &rec.Success, &rec.Confidence, &rec.Reason,
			&rec.ViolationsJSON, &rec.CorrectiveJSON, &rec.EvidenceJSON, &rec.ResultHash, &rec.CreatedAt,
		); err != nil {
			continue
		}
		records = append(records, rec)
	}

	return records, nil
}

// GetQualityViolationStats retrieves statistics on quality violations for learning.
func (s *LocalStore) GetQualityViolationStats() (map[string]int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(
		`SELECT quality_violations, COUNT(*) as count
		 FROM task_verifications
		 WHERE success = 0 AND quality_violations != '[]'
		 GROUP BY quality_violations
		 ORDER BY count DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := make(map[string]int)
	for rows.Next() {
		var violations string
		var count int
		if err := rows.Scan(&violations, &count); err != nil {
			continue
		}
		stats[violations] = count
	}

	return stats, nil
}

// VerificationRecord represents a stored verification attempt.
type VerificationRecord struct {
	ID             int64
	SessionID      string
	TurnNumber     int
	Task           string
	ShardType      string
	AttemptNumber  int
	Success        bool
	Confidence     float64
	Reason         string
	ViolationsJSON string
	CorrectiveJSON string
	EvidenceJSON   string
	ResultHash     string
	CreatedAt      time.Time
}

// ========== Reasoning Traces ==========

// ReasoningTrace represents a captured LLM interaction for learning.
// This mirrors perception.ReasoningTrace to avoid import cycles.
type ReasoningTrace struct {
	ID            string
	ShardID       string
	ShardType     string
	ShardCategory string
	SessionID     string
	TaskContext   string
	SystemPrompt  string
	UserPrompt    string
	Response      string
	Model         string
	TokensUsed    int
	DurationMs    int64
	Success       bool
	ErrorMessage  string
	QualityScore  float64
	LearningNotes []string
	CreatedAt     time.Time
}

// StoreReasoningTrace persists a reasoning trace.
// Implements perception.TraceStore interface.
// Accepts interface{} to avoid import cycles - uses reflection to extract fields.
func (s *LocalStore) StoreReasoningTrace(trace interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Try direct ReasoningTrace (from store package)
	if rt, ok := trace.(*ReasoningTrace); ok {
		notesJSON, _ := json.Marshal(rt.LearningNotes)
		_, err := s.db.Exec(`
			INSERT OR REPLACE INTO reasoning_traces
			(id, shard_id, shard_type, shard_category, session_id, task_context,
			 system_prompt, user_prompt, response, model, tokens_used, duration_ms,
			 success, error_message, quality_score, learning_notes)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			rt.ID, rt.ShardID, rt.ShardType, rt.ShardCategory, rt.SessionID, rt.TaskContext,
			rt.SystemPrompt, rt.UserPrompt, rt.Response, rt.Model, rt.TokensUsed, rt.DurationMs,
			rt.Success, rt.ErrorMessage, rt.QualityScore, string(notesJSON),
		)
		return err
	}

	// Use reflection to handle perception.ReasoningTrace without import cycle
	// This allows any struct with the same field names to work
	v := reflect.ValueOf(trace)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return fmt.Errorf("invalid trace type: expected struct, got %v", v.Kind())
	}

	// Extract fields by name
	getField := func(name string) interface{} {
		f := v.FieldByName(name)
		if !f.IsValid() {
			return nil
		}
		return f.Interface()
	}

	id, _ := getField("ID").(string)
	shardID, _ := getField("ShardID").(string)
	shardType, _ := getField("ShardType").(string)
	shardCategory, _ := getField("ShardCategory").(string)
	sessionID, _ := getField("SessionID").(string)
	taskContext, _ := getField("TaskContext").(string)
	systemPrompt, _ := getField("SystemPrompt").(string)
	userPrompt, _ := getField("UserPrompt").(string)
	response, _ := getField("Response").(string)
	model, _ := getField("Model").(string)
	tokensUsed, _ := getField("TokensUsed").(int)
	durationMs, _ := getField("DurationMs").(int64)
	success, _ := getField("Success").(bool)
	errorMessage, _ := getField("ErrorMessage").(string)
	qualityScore, _ := getField("QualityScore").(float64)
	learningNotes, _ := getField("LearningNotes").([]string)

	notesJSON, _ := json.Marshal(learningNotes)

	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO reasoning_traces
		(id, shard_id, shard_type, shard_category, session_id, task_context,
		 system_prompt, user_prompt, response, model, tokens_used, duration_ms,
		 success, error_message, quality_score, learning_notes)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, shardID, shardType, shardCategory, sessionID, taskContext,
		systemPrompt, userPrompt, response, model, tokensUsed, durationMs,
		success, errorMessage, qualityScore, string(notesJSON),
	)
	return err
}

// GetShardTraces retrieves traces for a specific shard type.
// Implements perception.ShardTraceReader interface.
func (s *LocalStore) GetShardTraces(shardType string, limit int) ([]ReasoningTrace, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 50
	}

	rows, err := s.db.Query(`
		SELECT id, shard_id, shard_type, shard_category, session_id, task_context,
		       system_prompt, user_prompt, response, model, tokens_used, duration_ms,
		       success, error_message, quality_score, learning_notes, created_at
		FROM reasoning_traces
		WHERE shard_type = ?
		ORDER BY created_at DESC
		LIMIT ?`, shardType, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return s.scanTraces(rows)
}

// GetFailedShardTraces retrieves failed traces for a shard type.
func (s *LocalStore) GetFailedShardTraces(shardType string, limit int) ([]ReasoningTrace, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 20
	}

	rows, err := s.db.Query(`
		SELECT id, shard_id, shard_type, shard_category, session_id, task_context,
		       system_prompt, user_prompt, response, model, tokens_used, duration_ms,
		       success, error_message, quality_score, learning_notes, created_at
		FROM reasoning_traces
		WHERE shard_type = ? AND success = 0
		ORDER BY created_at DESC
		LIMIT ?`, shardType, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return s.scanTraces(rows)
}

// GetSimilarTaskTraces finds traces with similar task context.
func (s *LocalStore) GetSimilarTaskTraces(shardType, taskPattern string, limit int) ([]ReasoningTrace, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 10
	}

	// Simple pattern matching - in production could use FTS or vector similarity
	rows, err := s.db.Query(`
		SELECT id, shard_id, shard_type, shard_category, session_id, task_context,
		       system_prompt, user_prompt, response, model, tokens_used, duration_ms,
		       success, error_message, quality_score, learning_notes, created_at
		FROM reasoning_traces
		WHERE shard_type = ? AND task_context LIKE ?
		ORDER BY created_at DESC
		LIMIT ?`, shardType, "%"+taskPattern+"%", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return s.scanTraces(rows)
}

// GetHighQualityTraces retrieves successful traces with high quality scores.
func (s *LocalStore) GetHighQualityTraces(shardType string, minScore float64, limit int) ([]ReasoningTrace, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 20
	}

	rows, err := s.db.Query(`
		SELECT id, shard_id, shard_type, shard_category, session_id, task_context,
		       system_prompt, user_prompt, response, model, tokens_used, duration_ms,
		       success, error_message, quality_score, learning_notes, created_at
		FROM reasoning_traces
		WHERE shard_type = ? AND success = 1 AND quality_score >= ?
		ORDER BY quality_score DESC, created_at DESC
		LIMIT ?`, shardType, minScore, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return s.scanTraces(rows)
}

// GetRecentTraces retrieves recent traces across all shards.
// Used by main agent for oversight.
func (s *LocalStore) GetRecentTraces(limit int) ([]ReasoningTrace, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 100
	}

	rows, err := s.db.Query(`
		SELECT id, shard_id, shard_type, shard_category, session_id, task_context,
		       system_prompt, user_prompt, response, model, tokens_used, duration_ms,
		       success, error_message, quality_score, learning_notes, created_at
		FROM reasoning_traces
		ORDER BY created_at DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return s.scanTraces(rows)
}

// GetTracesBySession retrieves all traces for a specific session.
func (s *LocalStore) GetTracesBySession(sessionID string) ([]ReasoningTrace, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
		SELECT id, shard_id, shard_type, shard_category, session_id, task_context,
		       system_prompt, user_prompt, response, model, tokens_used, duration_ms,
		       success, error_message, quality_score, learning_notes, created_at
		FROM reasoning_traces
		WHERE session_id = ?
		ORDER BY created_at ASC`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return s.scanTraces(rows)
}

// GetTracesByCategory retrieves traces by shard category.
func (s *LocalStore) GetTracesByCategory(category string, limit int) ([]ReasoningTrace, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 50
	}

	rows, err := s.db.Query(`
		SELECT id, shard_id, shard_type, shard_category, session_id, task_context,
		       system_prompt, user_prompt, response, model, tokens_used, duration_ms,
		       success, error_message, quality_score, learning_notes, created_at
		FROM reasoning_traces
		WHERE shard_category = ?
		ORDER BY created_at DESC
		LIMIT ?`, category, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return s.scanTraces(rows)
}

// UpdateTraceQuality updates the quality score and learning notes for a trace.
func (s *LocalStore) UpdateTraceQuality(traceID string, score float64, notes []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	notesJSON, _ := json.Marshal(notes)

	_, err := s.db.Exec(`
		UPDATE reasoning_traces
		SET quality_score = ?, learning_notes = ?
		WHERE id = ?`, score, string(notesJSON), traceID)
	return err
}

// GetTraceStats returns statistics about reasoning traces.
func (s *LocalStore) GetTraceStats() (map[string]interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := make(map[string]interface{})

	// Total traces
	var totalCount int64
	s.db.QueryRow("SELECT COUNT(*) FROM reasoning_traces").Scan(&totalCount)
	stats["total_traces"] = totalCount

	// Success rate
	var successCount int64
	s.db.QueryRow("SELECT COUNT(*) FROM reasoning_traces WHERE success = 1").Scan(&successCount)
	if totalCount > 0 {
		stats["success_rate"] = float64(successCount) / float64(totalCount)
	}

	// Average duration
	var avgDuration float64
	s.db.QueryRow("SELECT AVG(duration_ms) FROM reasoning_traces").Scan(&avgDuration)
	stats["avg_duration_ms"] = avgDuration

	// Traces by category
	categoryStats := make(map[string]int64)
	rows, err := s.db.Query("SELECT shard_category, COUNT(*) FROM reasoning_traces GROUP BY shard_category")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var category string
			var count int64
			if rows.Scan(&category, &count) == nil {
				categoryStats[category] = count
			}
		}
	}
	stats["by_category"] = categoryStats

	// Traces by shard type
	shardStats := make(map[string]int64)
	rows2, err := s.db.Query("SELECT shard_type, COUNT(*) FROM reasoning_traces GROUP BY shard_type ORDER BY COUNT(*) DESC LIMIT 10")
	if err == nil {
		defer rows2.Close()
		for rows2.Next() {
			var shardType string
			var count int64
			if rows2.Scan(&shardType, &count) == nil {
				shardStats[shardType] = count
			}
		}
	}
	stats["by_shard_type"] = shardStats

	return stats, nil
}

// scanTraces is a helper to scan trace rows into ReasoningTrace structs.
func (s *LocalStore) scanTraces(rows *sql.Rows) ([]ReasoningTrace, error) {
	var traces []ReasoningTrace
	for rows.Next() {
		var t ReasoningTrace
		var notesJSON string
		var model, taskContext, errorMessage sql.NullString
		var tokensUsed sql.NullInt64
		var qualityScore sql.NullFloat64

		err := rows.Scan(
			&t.ID, &t.ShardID, &t.ShardType, &t.ShardCategory, &t.SessionID, &taskContext,
			&t.SystemPrompt, &t.UserPrompt, &t.Response, &model, &tokensUsed, &t.DurationMs,
			&t.Success, &errorMessage, &qualityScore, &notesJSON, &t.CreatedAt,
		)
		if err != nil {
			continue
		}

		if model.Valid {
			t.Model = model.String
		}
		if taskContext.Valid {
			t.TaskContext = taskContext.String
		}
		if errorMessage.Valid {
			t.ErrorMessage = errorMessage.String
		}
		if tokensUsed.Valid {
			t.TokensUsed = int(tokensUsed.Int64)
		}
		if qualityScore.Valid {
			t.QualityScore = qualityScore.Float64
		}
		if notesJSON != "" {
			json.Unmarshal([]byte(notesJSON), &t.LearningNotes)
		}

		traces = append(traces, t)
	}

	return traces, nil
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
