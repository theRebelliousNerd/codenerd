package store

import (
	"codenerd/internal/embedding"
	"codenerd/internal/logging"
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
)

// LocalStore implements Shards B, C, and D using SQLite.
// Shard B: Vector/Associative Memory (semantic search)
// Shard C: Knowledge Graph (relational links)
// Shard D: Cold Storage (persistent facts and preferences)
//
// Storage Tiers:
// - Cold Storage: Active facts with access tracking (last_accessed, access_count)
// - Archival Storage: Old, rarely-accessed facts moved from cold storage
//
// Usage Example:
//
//	// Store facts
//	store.StoreFact("user_preference", []interface{}{"theme", "dark"}, "preference", 10)
//
//	// Load facts (automatically tracks access)
//	facts, _ := store.LoadFacts("user_preference")
//
//	// Periodic maintenance - archive old facts
//	config := MaintenanceConfig{
//	  ArchiveOlderThanDays: 90,        // Archive facts not accessed in 90 days
//	  MaxAccessCount: 5,               // Only if accessed <= 5 times
//	  PurgeArchivedOlderThanDays: 365, // Delete archived facts older than 1 year
//	  CleanActivationLogDays: 30,      // Clean activation logs older than 30 days
//	  VacuumDatabase: true,            // Reclaim disk space
//	}
//	stats, _ := store.MaintenanceCleanup(config)
//
//	// Restore archived fact if needed
//	store.RestoreArchivedFact("user_preference", []interface{}{"theme", "dark"})
type LocalStore struct {
	db              *sql.DB
	mu              sync.RWMutex
	dbPath          string
	embeddingEngine embedding.EmbeddingEngine // Optional embedding engine for semantic search
	vectorExt       bool                      // sqlite-vec available
	requireVec      bool                      // require vec extension or fail fast
	traceStore      *TraceStore               // Dedicated trace store for self-learning
}

// NewLocalStore initializes the SQLite database at the given path.
func NewLocalStore(path string) (*LocalStore, error) {
	timer := logging.StartTimer(logging.CategoryStore, "NewLocalStore")
	defer timer.Stop()

	logging.Store("Initializing LocalStore at path: %s", path)

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to create directory %s: %v", dir, err)
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}
	logging.StoreDebug("Created directory: %s", dir)

	db, err := sql.Open("sqlite3", path)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to open database at %s: %v", path, err)
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	logging.StoreDebug("Opened SQLite database connection")

	store := &LocalStore{db: db, dbPath: path}
	if err := store.initialize(); err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to initialize schema: %v", err)
		db.Close()
		return nil, err
	}
	logging.StoreDebug("Database schema initialized successfully")

	// Detect sqlite-vec extension availability
	store.detectVecExtension()
	store.requireVec = defaultRequireVec
	if store.requireVec && !store.vectorExt {
		logging.Get(logging.CategoryStore).Error("sqlite-vec extension not available")
		db.Close()
		return nil, fmt.Errorf("sqlite-vec extension not available; rebuild modernc SQLite with vec0 (set SQLITE3_EXT=vec0 or include vec sources) to enable ANN search")
	}
	if store.vectorExt {
		logging.Store("sqlite-vec extension detected and enabled")
	} else {
		logging.Get(logging.CategoryStore).Warn("sqlite-vec extension not available; continuing without ANN search")
	}

	// Initialize trace store for self-learning
	traceStore, err := NewTraceStore(db, path)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to initialize trace store: %v", err)
		db.Close()
		return nil, fmt.Errorf("failed to initialize trace store: %w", err)
	}
	store.traceStore = traceStore
	logging.StoreDebug("TraceStore initialized for self-learning")

	// Backfill content_hash for any existing atoms missing it
	if err := store.ensureContentHashes(); err != nil {
		logging.Get(logging.CategoryStore).Warn("Content hash backfill had issues: %v", err)
		// Don't fail - this is a non-critical maintenance operation
	}

	logging.Store("LocalStore initialization complete (RAM, Vector, Graph, Cold tiers ready)")
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

	// World Model Cache (Fast + Deep AST projections)
	// These tables store per-file world facts so boot and incremental scans
	// can rehydrate without reparsing unchanged files.
	worldFilesTable := `
	CREATE TABLE IF NOT EXISTS world_files (
		path TEXT PRIMARY KEY,
		lang TEXT,
		size INTEGER,
		modtime INTEGER,
		hash TEXT,
		fingerprint TEXT,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_world_files_fingerprint ON world_files(fingerprint);
	CREATE INDEX IF NOT EXISTS idx_world_files_lang ON world_files(lang);
	`

	worldFactsTable := `
	CREATE TABLE IF NOT EXISTS world_facts (
		path TEXT NOT NULL,
		depth TEXT NOT NULL, -- fast | deep
		fingerprint TEXT NOT NULL,
		predicate TEXT NOT NULL,
		args TEXT NOT NULL,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY(path, depth, predicate, args)
	);
	CREATE INDEX IF NOT EXISTS idx_world_facts_predicate ON world_facts(predicate);
	CREATE INDEX IF NOT EXISTS idx_world_facts_depth ON world_facts(depth);
	CREATE INDEX IF NOT EXISTS idx_world_facts_path ON world_facts(path);
	`

	// Shard D: Cold Storage and Archival tier are created below with migration-aware logic

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

	// Compressed context state for infinite-context rehydration
	// Stores semantic compression state per session/turn.
	compressedStateTable := `
	CREATE TABLE IF NOT EXISTS compressed_states (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id TEXT NOT NULL,
		turn_number INTEGER NOT NULL,
		state_json TEXT NOT NULL,
		compression_ratio REAL DEFAULT 1.0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(session_id, turn_number)
	);
	CREATE INDEX IF NOT EXISTS idx_compressed_states_session ON compressed_states(session_id);
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

	// Review Findings (for persistent history and analysis)
	reviewFindingsTable := `
	CREATE TABLE IF NOT EXISTS review_findings (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		reviewed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		file_path TEXT NOT NULL,
		line INTEGER,
		severity TEXT,
		category TEXT,
		rule_id TEXT,
		message TEXT,
		project_root TEXT
	);
	CREATE INDEX IF NOT EXISTS idx_findings_path ON review_findings(file_path);
	CREATE INDEX IF NOT EXISTS idx_findings_severity ON review_findings(severity);
	`

	// Prompt Atoms (for Universal JIT Prompt Compiler)
	// NOTE: Table creation WITHOUT indexes - indexes created after migrations
	promptAtomsTableOnly := `
	CREATE TABLE IF NOT EXISTS prompt_atoms (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		atom_id TEXT NOT NULL UNIQUE,
		version INTEGER DEFAULT 1,
		content TEXT NOT NULL,
		token_count INTEGER NOT NULL,
		content_hash TEXT NOT NULL,

		-- Polymorphism (for different verbosity levels)
		description TEXT,
		content_concise TEXT,
		content_min TEXT,

		-- Classification
		category TEXT NOT NULL,
		subcategory TEXT,

		-- Contextual Selectors (JSON arrays)
		operational_modes TEXT,
		campaign_phases TEXT,
		build_layers TEXT,
		init_phases TEXT,
		northstar_phases TEXT,
		ouroboros_stages TEXT,
		intent_verbs TEXT,
		shard_types TEXT,
		languages TEXT,
		frameworks TEXT,
		world_states TEXT,

		-- Composition
		priority INTEGER DEFAULT 50,
		is_mandatory BOOLEAN DEFAULT FALSE,
		is_exclusive TEXT,
		depends_on TEXT,
		conflicts_with TEXT,

		-- Embeddings
		embedding BLOB,
		embedding_task TEXT DEFAULT 'RETRIEVAL_DOCUMENT',

		-- Metadata
		source_file TEXT,

		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`

	// First, create tables WITHOUT indexes that depend on migrated columns
	// Split cold_storage into table creation and index creation
	coldTableOnly := `
	CREATE TABLE IF NOT EXISTS cold_storage (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		predicate TEXT NOT NULL,
		args TEXT NOT NULL,
		fact_type TEXT DEFAULT 'fact',
		priority INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		last_accessed DATETIME DEFAULT CURRENT_TIMESTAMP,
		access_count INTEGER DEFAULT 0,
		UNIQUE(predicate, args)
	);
	CREATE INDEX IF NOT EXISTS idx_cold_predicate ON cold_storage(predicate);
	CREATE INDEX IF NOT EXISTS idx_cold_type ON cold_storage(fact_type);
	`

	archivedTableOnly := `
	CREATE TABLE IF NOT EXISTS archived_facts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		predicate TEXT NOT NULL,
		args TEXT NOT NULL,
		fact_type TEXT DEFAULT 'fact',
		priority INTEGER DEFAULT 0,
		created_at DATETIME,
		updated_at DATETIME,
		last_accessed DATETIME,
		access_count INTEGER DEFAULT 0,
		archived_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(predicate, args)
	);
	CREATE INDEX IF NOT EXISTS idx_archived_predicate ON archived_facts(predicate);
	CREATE INDEX IF NOT EXISTS idx_archived_type ON archived_facts(fact_type);
	CREATE INDEX IF NOT EXISTS idx_archived_at ON archived_facts(archived_at);
	`

	// Create base tables first (without columns that need migration)
	for _, table := range []string{
		vectorTable,
		graphTable,
		worldFilesTable,
		worldFactsTable,
		coldTableOnly,
		archivedTableOnly,
		activationTable,
		sessionTable,
		compressedStateTable,
		verificationTable,
		reasoningTracesTable,
		reviewFindingsTable,
		promptAtomsTableOnly,
	} {
		if _, err := s.db.Exec(table); err != nil {
			return fmt.Errorf("failed to create table: %w", err)
		}
	}

	// Run schema migrations for existing databases (adds missing columns like last_accessed, description)
	if err := RunMigrations(s.db); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	// Now create indexes that depend on migrated columns
	coldIndexes := `
	CREATE INDEX IF NOT EXISTS idx_cold_last_accessed ON cold_storage(last_accessed);
	CREATE INDEX IF NOT EXISTS idx_cold_access_count ON cold_storage(access_count);
	`
	if _, err := s.db.Exec(coldIndexes); err != nil {
		// Non-fatal: indexes improve performance but aren't required
		logging.Get(logging.CategoryStore).Warn("Failed to create cold storage indexes: %v", err)
	}

	// Create prompt atoms indexes after migrations (description column added by migration)
	promptAtomsIndexes := `
	CREATE INDEX IF NOT EXISTS idx_prompt_atoms_category ON prompt_atoms(category);
	CREATE INDEX IF NOT EXISTS idx_prompt_atoms_hash ON prompt_atoms(content_hash);
	CREATE INDEX IF NOT EXISTS idx_prompt_atoms_mandatory ON prompt_atoms(is_mandatory);
	CREATE INDEX IF NOT EXISTS idx_prompt_atoms_description ON prompt_atoms(description);
	`
	if _, err := s.db.Exec(promptAtomsIndexes); err != nil {
		// Non-fatal: indexes improve performance but aren't required
		logging.Get(logging.CategoryStore).Warn("Failed to create prompt atoms indexes: %v", err)
	}

	return nil
}

// GetTraceStore returns the dedicated trace store for self-learning operations.
// This allows external components to access trace persistence separately.
func (s *LocalStore) GetTraceStore() *TraceStore {
	return s.traceStore
}

// Close closes the database connection.
func (s *LocalStore) Close() error {
	logging.Store("Closing LocalStore database connection")
	return s.db.Close()
}

// GetDB returns the underlying SQL database connection.
func (s *LocalStore) GetDB() *sql.DB {
	return s.db
}

// =============================================================================
// WORLD MODEL CACHE (Fast + Deep AST facts)
// =============================================================================

// WorldFileMeta stores per-file metadata for world cache invalidation.
type WorldFileMeta struct {
	Path        string
	Lang        string
	Size        int64
	ModTime     int64
	Hash        string
	Fingerprint string
}

// WorldFactInput is a lightweight fact carrier for world cache I/O.
// Predicate + Args mirror core/types.Fact without importing core.
type WorldFactInput struct {
	Predicate string
	Args      []interface{}
}

// UpsertWorldFile stores or updates world_files metadata.
func (s *LocalStore) UpsertWorldFile(meta WorldFileMeta) error {
	timer := logging.StartTimer(logging.CategoryStore, "UpsertWorldFile")
	defer timer.Stop()

	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(
		`INSERT INTO world_files (path, lang, size, modtime, hash, fingerprint, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(path) DO UPDATE SET
		   lang = excluded.lang,
		   size = excluded.size,
		   modtime = excluded.modtime,
		   hash = excluded.hash,
		   fingerprint = excluded.fingerprint,
		   updated_at = CURRENT_TIMESTAMP`,
		meta.Path, meta.Lang, meta.Size, meta.ModTime, meta.Hash, meta.Fingerprint,
	)
	return err
}

// DeleteWorldFile removes world_files and all cached facts for a file.
func (s *LocalStore) DeleteWorldFile(path string) error {
	timer := logging.StartTimer(logging.CategoryStore, "DeleteWorldFile")
	defer timer.Stop()

	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM world_facts WHERE path = ?", path); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM world_files WHERE path = ?", path); err != nil {
		return err
	}
	return tx.Commit()
}

// ReplaceWorldFactsForFile replaces cached facts for a file at a given depth.
func (s *LocalStore) ReplaceWorldFactsForFile(path, depth, fingerprint string, facts []WorldFactInput) error {
	timer := logging.StartTimer(logging.CategoryStore, "ReplaceWorldFactsForFile")
	defer timer.Stop()

	if depth == "" {
		depth = "fast"
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM world_facts WHERE path = ? AND depth = ?", path, depth); err != nil {
		return err
	}

	stmt, err := tx.Prepare(
		`INSERT OR REPLACE INTO world_facts (path, depth, fingerprint, predicate, args, updated_at)
		 VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
	)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, f := range facts {
		argsJSON, _ := json.Marshal(f.Args)
		if _, err := stmt.Exec(path, depth, fingerprint, f.Predicate, string(argsJSON)); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// LoadWorldFactsForFile loads cached facts for a file at a given depth.
// Returns the facts and the stored fingerprint (empty if none).
func (s *LocalStore) LoadWorldFactsForFile(path, depth string) ([]WorldFactInput, string, error) {
	timer := logging.StartTimer(logging.CategoryStore, "LoadWorldFactsForFile")
	defer timer.Stop()

	if depth == "" {
		depth = "fast"
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(
		"SELECT predicate, args, fingerprint FROM world_facts WHERE path = ? AND depth = ?",
		path, depth,
	)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var out []WorldFactInput
	var fp string
	for rows.Next() {
		var pred, argsJSON, fingerprint string
		if err := rows.Scan(&pred, &argsJSON, &fingerprint); err != nil {
			continue
		}
		fp = fingerprint
		var args []interface{}
		_ = json.Unmarshal([]byte(argsJSON), &args)
		out = append(out, WorldFactInput{Predicate: pred, Args: args})
	}
	return out, fp, nil
}

// LoadAllWorldFacts loads all cached facts for a given depth.
func (s *LocalStore) LoadAllWorldFacts(depth string) ([]WorldFactInput, error) {
	timer := logging.StartTimer(logging.CategoryStore, "LoadAllWorldFacts")
	defer timer.Stop()

	if depth == "" {
		depth = "fast"
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(
		"SELECT predicate, args FROM world_facts WHERE depth = ? ORDER BY path",
		depth,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]WorldFactInput, 0)
	for rows.Next() {
		var pred, argsJSON string
		if err := rows.Scan(&pred, &argsJSON); err != nil {
			continue
		}
		var args []interface{}
		_ = json.Unmarshal([]byte(argsJSON), &args)
		out = append(out, WorldFactInput{Predicate: pred, Args: args})
	}
	return out, nil
}

// ========== Shard B: Vector/Associative Memory ==========

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
	timer := logging.StartTimer(logging.CategoryStore, "StoreLink")
	defer timer.Stop()

	s.mu.Lock()
	defer s.mu.Unlock()

	logging.StoreDebug("Storing graph link: %s -[%s]-> %s (weight=%.2f)", entityA, relation, entityB, weight)

	metaJSON, _ := json.Marshal(metadata)

	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO knowledge_graph (entity_a, relation, entity_b, weight, metadata)
		 VALUES (?, ?, ?, ?, ?)`,
		entityA, relation, entityB, weight, string(metaJSON),
	)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to store graph link: %v", err)
		return err
	}

	logging.StoreDebug("Graph link stored successfully")
	return nil
}

// QueryLinks retrieves links for an entity.
func (s *LocalStore) QueryLinks(entity string, direction string) ([]KnowledgeLink, error) {
	timer := logging.StartTimer(logging.CategoryStore, "QueryLinks")
	defer timer.Stop()

	s.mu.RLock()
	defer s.mu.RUnlock()

	logging.StoreDebug("Querying graph links for entity=%q direction=%s", entity, direction)

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
		logging.Get(logging.CategoryStore).Error("Graph query failed for entity=%q: %v", entity, err)
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

	logging.StoreDebug("Graph query returned %d links", len(links))
	return links, nil
}

// TraversePath finds a path between two entities using BFS.
func (s *LocalStore) TraversePath(from, to string, maxDepth int) ([]KnowledgeLink, error) {
	timer := logging.StartTimer(logging.CategoryStore, "TraversePath")
	defer timer.Stop()

	s.mu.RLock()
	defer s.mu.RUnlock()

	if maxDepth <= 0 {
		maxDepth = 5
	}

	logging.StoreDebug("Graph traversal: %s -> %s (maxDepth=%d)", from, to, maxDepth)

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
			logging.StoreDebug("Path found with %d hops, visited %d nodes", len(current.path), len(visited))
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

	logging.StoreDebug("No path found from %s to %s (visited %d nodes)", from, to, len(visited))
	return nil, fmt.Errorf("no path found from %s to %s", from, to)
}

// ========== Shard D: Cold Storage ==========

// StoredFact represents a persisted fact.
type StoredFact struct {
	ID           int64
	Predicate    string
	Args         []interface{}
	FactType     string
	Priority     int
	CreatedAt    time.Time
	UpdatedAt    time.Time
	LastAccessed time.Time
	AccessCount  int
}

// ArchivedFact represents a fact moved to archival storage.
type ArchivedFact struct {
	ID           int64
	Predicate    string
	Args         []interface{}
	FactType     string
	Priority     int
	CreatedAt    time.Time
	UpdatedAt    time.Time
	LastAccessed time.Time
	AccessCount  int
	ArchivedAt   time.Time
}

// StoreFact persists a fact to cold storage.
func (s *LocalStore) StoreFact(predicate string, args []interface{}, factType string, priority int) error {
	timer := logging.StartTimer(logging.CategoryStore, "StoreFact")
	defer timer.Stop()

	s.mu.Lock()
	defer s.mu.Unlock()

	logging.StoreDebug("Storing fact to cold storage: %s/%d args (type=%s, priority=%d)", predicate, len(args), factType, priority)

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
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to store fact %s: %v", predicate, err)
		return err
	}

	logging.StoreDebug("Fact stored successfully in cold storage")
	return nil
}

// LoadFacts retrieves facts by predicate and updates access tracking.
func (s *LocalStore) LoadFacts(predicate string) ([]StoredFact, error) {
	timer := logging.StartTimer(logging.CategoryStore, "LoadFacts")
	defer timer.Stop()

	s.mu.Lock()
	defer s.mu.Unlock()

	logging.StoreDebug("Loading facts from cold storage: predicate=%s", predicate)

	rows, err := s.db.Query(
		"SELECT id, predicate, args, fact_type, priority, created_at, updated_at, last_accessed, access_count FROM cold_storage WHERE predicate = ? ORDER BY priority DESC",
		predicate,
	)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to load facts for predicate=%s: %v", predicate, err)
		return nil, err
	}
	defer rows.Close()

	var facts []StoredFact
	var factIDs []int64
	for rows.Next() {
		var fact StoredFact
		var argsJSON string
		if err := rows.Scan(&fact.ID, &fact.Predicate, &argsJSON, &fact.FactType, &fact.Priority, &fact.CreatedAt, &fact.UpdatedAt, &fact.LastAccessed, &fact.AccessCount); err != nil {
			continue
		}
		json.Unmarshal([]byte(argsJSON), &fact.Args)
		facts = append(facts, fact)
		factIDs = append(factIDs, fact.ID)
	}

	// Update access tracking for retrieved facts
	for _, id := range factIDs {
		s.db.Exec(
			"UPDATE cold_storage SET last_accessed = CURRENT_TIMESTAMP, access_count = access_count + 1 WHERE id = ?",
			id,
		)
	}

	logging.StoreDebug("Loaded %d facts for predicate=%s (access tracking updated)", len(facts), predicate)
	return facts, nil
}

// LoadAllFacts retrieves all facts, optionally filtered by type.
// Does not update access tracking (use LoadFacts for that).
func (s *LocalStore) LoadAllFacts(factType string) ([]StoredFact, error) {
	timer := logging.StartTimer(logging.CategoryStore, "LoadAllFacts")
	defer timer.Stop()

	s.mu.RLock()
	defer s.mu.RUnlock()

	logging.StoreDebug("Loading all facts from cold storage (type filter=%q)", factType)

	var query string
	var args []interface{}

	if factType != "" {
		query = "SELECT id, predicate, args, fact_type, priority, created_at, updated_at, last_accessed, access_count FROM cold_storage WHERE fact_type = ? ORDER BY priority DESC"
		args = []interface{}{factType}
	} else {
		query = "SELECT id, predicate, args, fact_type, priority, created_at, updated_at, last_accessed, access_count FROM cold_storage ORDER BY priority DESC"
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to load all facts: %v", err)
		return nil, err
	}
	defer rows.Close()

	var facts []StoredFact
	for rows.Next() {
		var fact StoredFact
		var argsJSON string
		if err := rows.Scan(&fact.ID, &fact.Predicate, &argsJSON, &fact.FactType, &fact.Priority, &fact.CreatedAt, &fact.UpdatedAt, &fact.LastAccessed, &fact.AccessCount); err != nil {
			continue
		}
		json.Unmarshal([]byte(argsJSON), &fact.Args)
		facts = append(facts, fact)
	}

	logging.StoreDebug("Loaded %d total facts from cold storage", len(facts))
	return facts, nil
}

// DeleteFact removes a fact by predicate and args.
func (s *LocalStore) DeleteFact(predicate string, args []interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	logging.StoreDebug("Deleting fact from cold storage: %s", predicate)

	argsJSON, _ := json.Marshal(args)
	_, err := s.db.Exec("DELETE FROM cold_storage WHERE predicate = ? AND args = ?", predicate, string(argsJSON))
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to delete fact %s: %v", predicate, err)
		return err
	}

	logging.StoreDebug("Fact deleted from cold storage: %s", predicate)
	return nil
}

// ========== Archival Tier Management ==========

// ArchiveOldFacts moves old, rarely-accessed facts to archival storage.
// Facts are archived if they meet ALL criteria:
// - Older than olderThanDays
// - Access count below maxAccessCount
// Returns the number of facts archived.
func (s *LocalStore) ArchiveOldFacts(olderThanDays int, maxAccessCount int) (int, error) {
	timer := logging.StartTimer(logging.CategoryStore, "ArchiveOldFacts")
	defer timer.Stop()

	s.mu.Lock()
	defer s.mu.Unlock()

	if olderThanDays <= 0 {
		olderThanDays = 90 // Default to 90 days
	}
	if maxAccessCount < 0 {
		maxAccessCount = 5 // Default: archive facts accessed 5 times or less
	}

	logging.Store("Archiving facts older than %d days with access count <= %d", olderThanDays, maxAccessCount)

	// Start transaction for atomic move
	tx, err := s.db.Begin()
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to start archive transaction: %v", err)
		return 0, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	// Find facts to archive
	rows, err := tx.Query(
		`SELECT id, predicate, args, fact_type, priority, created_at, updated_at, last_accessed, access_count
		 FROM cold_storage
		 WHERE datetime(last_accessed) < datetime('now', '-' || ? || ' days')
		 AND access_count <= ?`,
		olderThanDays, maxAccessCount,
	)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to query old facts for archival: %v", err)
		return 0, fmt.Errorf("failed to query old facts: %w", err)
	}
	defer rows.Close()

	var archivedCount int
	var idsToDelete []int64

	// Insert into archived_facts
	for rows.Next() {
		var id int64
		var predicate, argsJSON, factType string
		var priority, accessCount int
		var createdAt, updatedAt, lastAccessed time.Time

		if err := rows.Scan(&id, &predicate, &argsJSON, &factType, &priority, &createdAt, &updatedAt, &lastAccessed, &accessCount); err != nil {
			continue
		}

		// Insert into archive
		_, err := tx.Exec(
			`INSERT OR REPLACE INTO archived_facts (predicate, args, fact_type, priority, created_at, updated_at, last_accessed, access_count)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			predicate, argsJSON, factType, priority, createdAt, updatedAt, lastAccessed, accessCount,
		)
		if err != nil {
			logging.Get(logging.CategoryStore).Warn("Failed to archive fact %s: %v", predicate, err)
			continue
		}

		idsToDelete = append(idsToDelete, id)
		archivedCount++
	}

	// Delete from cold_storage
	for _, id := range idsToDelete {
		_, err := tx.Exec("DELETE FROM cold_storage WHERE id = ?", id)
		if err != nil {
			logging.Get(logging.CategoryStore).Error("Failed to delete archived fact id=%d: %v", id, err)
			return 0, fmt.Errorf("failed to delete archived fact: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to commit archive transaction: %v", err)
		return 0, fmt.Errorf("failed to commit archive transaction: %w", err)
	}

	logging.Store("Archived %d facts from cold storage to archival tier", archivedCount)
	return archivedCount, nil
}

// GetArchivedFacts retrieves archived facts by predicate.
func (s *LocalStore) GetArchivedFacts(predicate string) ([]ArchivedFact, error) {
	timer := logging.StartTimer(logging.CategoryStore, "GetArchivedFacts")
	defer timer.Stop()

	s.mu.RLock()
	defer s.mu.RUnlock()

	logging.StoreDebug("Retrieving archived facts: predicate=%s", predicate)

	rows, err := s.db.Query(
		`SELECT id, predicate, args, fact_type, priority, created_at, updated_at, last_accessed, access_count, archived_at
		 FROM archived_facts
		 WHERE predicate = ?
		 ORDER BY archived_at DESC`,
		predicate,
	)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to retrieve archived facts for %s: %v", predicate, err)
		return nil, err
	}
	defer rows.Close()

	var facts []ArchivedFact
	for rows.Next() {
		var fact ArchivedFact
		var argsJSON string
		if err := rows.Scan(&fact.ID, &fact.Predicate, &argsJSON, &fact.FactType, &fact.Priority, &fact.CreatedAt, &fact.UpdatedAt, &fact.LastAccessed, &fact.AccessCount, &fact.ArchivedAt); err != nil {
			continue
		}
		json.Unmarshal([]byte(argsJSON), &fact.Args)
		facts = append(facts, fact)
	}

	logging.StoreDebug("Retrieved %d archived facts for predicate=%s", len(facts), predicate)
	return facts, nil
}

// GetAllArchivedFacts retrieves all archived facts, optionally filtered by type.
func (s *LocalStore) GetAllArchivedFacts(factType string) ([]ArchivedFact, error) {
	timer := logging.StartTimer(logging.CategoryStore, "GetAllArchivedFacts")
	defer timer.Stop()

	s.mu.RLock()
	defer s.mu.RUnlock()

	logging.StoreDebug("Retrieving all archived facts (type filter=%q)", factType)

	var query string
	var args []interface{}

	if factType != "" {
		query = `SELECT id, predicate, args, fact_type, priority, created_at, updated_at, last_accessed, access_count, archived_at
				 FROM archived_facts WHERE fact_type = ? ORDER BY archived_at DESC`
		args = []interface{}{factType}
	} else {
		query = `SELECT id, predicate, args, fact_type, priority, created_at, updated_at, last_accessed, access_count, archived_at
				 FROM archived_facts ORDER BY archived_at DESC`
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to retrieve all archived facts: %v", err)
		return nil, err
	}
	defer rows.Close()

	var facts []ArchivedFact
	for rows.Next() {
		var fact ArchivedFact
		var argsJSON string
		if err := rows.Scan(&fact.ID, &fact.Predicate, &argsJSON, &fact.FactType, &fact.Priority, &fact.CreatedAt, &fact.UpdatedAt, &fact.LastAccessed, &fact.AccessCount, &fact.ArchivedAt); err != nil {
			continue
		}
		json.Unmarshal([]byte(argsJSON), &fact.Args)
		facts = append(facts, fact)
	}

	logging.StoreDebug("Retrieved %d archived facts", len(facts))
	return facts, nil
}

// RestoreArchivedFact moves a fact from archive back to cold storage.
func (s *LocalStore) RestoreArchivedFact(predicate string, args []interface{}) error {
	timer := logging.StartTimer(logging.CategoryStore, "RestoreArchivedFact")
	defer timer.Stop()

	s.mu.Lock()
	defer s.mu.Unlock()

	logging.Store("Restoring archived fact: %s (promoting to cold storage)", predicate)

	argsJSON, _ := json.Marshal(args)

	// Start transaction
	tx, err := s.db.Begin()
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to start restore transaction: %v", err)
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	// Get archived fact
	var id int64
	var factType string
	var priority, accessCount int
	var createdAt, updatedAt, lastAccessed time.Time

	err = tx.QueryRow(
		"SELECT id, fact_type, priority, created_at, updated_at, last_accessed, access_count FROM archived_facts WHERE predicate = ? AND args = ?",
		predicate, string(argsJSON),
	).Scan(&id, &factType, &priority, &createdAt, &updatedAt, &lastAccessed, &accessCount)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Fact not found in archive: %s: %v", predicate, err)
		return fmt.Errorf("fact not found in archive: %w", err)
	}

	// Insert back into cold_storage
	_, err = tx.Exec(
		`INSERT OR REPLACE INTO cold_storage (predicate, args, fact_type, priority, created_at, updated_at, last_accessed, access_count)
		 VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, ?)`,
		predicate, string(argsJSON), factType, priority, createdAt, updatedAt, accessCount,
	)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to restore fact to cold storage: %v", err)
		return fmt.Errorf("failed to restore fact: %w", err)
	}

	// Delete from archive
	_, err = tx.Exec("DELETE FROM archived_facts WHERE id = ?", id)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to delete from archive after restore: %v", err)
		return fmt.Errorf("failed to delete from archive: %w", err)
	}

	if err := tx.Commit(); err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to commit restore transaction: %v", err)
		return fmt.Errorf("failed to commit restore transaction: %w", err)
	}

	logging.Store("Restored fact %s from archival tier to cold storage", predicate)
	return nil
}

// PurgeOldArchivedFacts permanently deletes archived facts older than specified days.
// Use with caution - this is irreversible.
func (s *LocalStore) PurgeOldArchivedFacts(olderThanDays int) (int, error) {
	timer := logging.StartTimer(logging.CategoryStore, "PurgeOldArchivedFacts")
	defer timer.Stop()

	s.mu.Lock()
	defer s.mu.Unlock()

	if olderThanDays <= 0 {
		return 0, fmt.Errorf("olderThanDays must be positive")
	}

	logging.Get(logging.CategoryStore).Warn("Purging archived facts older than %d days (IRREVERSIBLE)", olderThanDays)

	result, err := s.db.Exec(
		`DELETE FROM archived_facts
		 WHERE datetime(archived_at) < datetime('now', '-' || ? || ' days')`,
		olderThanDays,
	)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to purge old archived facts: %v", err)
		return 0, fmt.Errorf("failed to purge old archived facts: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	logging.Store("Purged %d archived facts older than %d days", rowsAffected, olderThanDays)
	return int(rowsAffected), nil
}

// MaintenanceCleanup performs periodic maintenance on the storage tiers.
// Returns statistics about cleanup operations.
func (s *LocalStore) MaintenanceCleanup(config MaintenanceConfig) (MaintenanceStats, error) {
	timer := logging.StartTimer(logging.CategoryStore, "MaintenanceCleanup")
	defer timer.Stop()

	logging.Store("Starting maintenance cleanup cycle")
	stats := MaintenanceStats{}

	// Archive old facts
	if config.ArchiveOlderThanDays > 0 {
		logging.StoreDebug("Archiving facts older than %d days", config.ArchiveOlderThanDays)
		archived, err := s.ArchiveOldFacts(config.ArchiveOlderThanDays, config.MaxAccessCount)
		if err != nil {
			logging.Get(logging.CategoryStore).Error("Archival failed during maintenance: %v", err)
			return stats, fmt.Errorf("archival failed: %w", err)
		}
		stats.FactsArchived = archived
	}

	// Purge very old archived facts
	if config.PurgeArchivedOlderThanDays > 0 {
		logging.StoreDebug("Purging archived facts older than %d days", config.PurgeArchivedOlderThanDays)
		purged, err := s.PurgeOldArchivedFacts(config.PurgeArchivedOlderThanDays)
		if err != nil {
			logging.Get(logging.CategoryStore).Error("Purge failed during maintenance: %v", err)
			return stats, fmt.Errorf("purge failed: %w", err)
		}
		stats.FactsPurged = purged
	}

	// Clean old activation logs
	if config.CleanActivationLogDays > 0 {
		logging.StoreDebug("Cleaning activation logs older than %d days", config.CleanActivationLogDays)
		s.mu.Lock()
		result, err := s.db.Exec(
			`DELETE FROM activation_log
			 WHERE datetime(timestamp) < datetime('now', '-' || ? || ' days')`,
			config.CleanActivationLogDays,
		)
		s.mu.Unlock()
		if err == nil {
			rows, _ := result.RowsAffected()
			stats.ActivationLogsDeleted = int(rows)
		} else {
			logging.Get(logging.CategoryStore).Warn("Failed to clean activation logs: %v", err)
		}
	}

	// Vacuum database to reclaim space
	if config.VacuumDatabase {
		logging.StoreDebug("Running VACUUM to reclaim disk space")
		s.mu.Lock()
		_, err := s.db.Exec("VACUUM")
		s.mu.Unlock()
		if err != nil {
			logging.Get(logging.CategoryStore).Error("VACUUM failed: %v", err)
			return stats, fmt.Errorf("vacuum failed: %w", err)
		}
		stats.DatabaseVacuumed = true
	}

	logging.Store("Maintenance complete: archived=%d, purged=%d, activation_logs_deleted=%d, vacuumed=%v",
		stats.FactsArchived, stats.FactsPurged, stats.ActivationLogsDeleted, stats.DatabaseVacuumed)
	return stats, nil
}

// detectVecExtension attempts to create a vec0 virtual table to see if sqlite-vec is available.
func (s *LocalStore) detectVecExtension() {
	if s.db == nil {
		return
	}
	// First try true sqlite-vec virtual table support.
	if _, err := s.db.Exec("CREATE VIRTUAL TABLE IF NOT EXISTS vec_probe USING vec0(embedding float[4])"); err == nil {
		s.vectorExt = true
		_, _ = s.db.Exec("DROP TABLE IF EXISTS vec_probe")
		return
	}

	s.vectorExt = false
}

// MaintenanceConfig configures maintenance cleanup operations.
type MaintenanceConfig struct {
	ArchiveOlderThanDays       int  // Archive facts not accessed in N days
	MaxAccessCount             int  // Only archive if access count <= this
	PurgeArchivedOlderThanDays int  // Permanently delete archived facts older than N days
	CleanActivationLogDays     int  // Delete activation logs older than N days
	VacuumDatabase             bool // Run VACUUM to reclaim space
}

// MaintenanceStats reports results of maintenance operations.
type MaintenanceStats struct {
	FactsArchived         int
	FactsPurged           int
	ActivationLogsDeleted int
	DatabaseVacuumed      bool
}

// ========== Spreading Activation ==========

// LogActivation records activation scores for facts.
func (s *LocalStore) LogActivation(factID string, score float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	logging.StoreDebug("Logging activation: fact_id=%s score=%.4f", factID, score)

	_, err := s.db.Exec(
		"INSERT INTO activation_log (fact_id, activation_score) VALUES (?, ?)",
		factID, score,
	)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to log activation for %s: %v", factID, err)
		return err
	}

	logging.StoreDebug("Activation logged successfully: fact_id=%s", factID)
	return nil
}

// GetRecentActivations retrieves recent activation scores.
func (s *LocalStore) GetRecentActivations(limit int, minScore float64) (map[string]float64, error) {
	timer := logging.StartTimer(logging.CategoryStore, "GetRecentActivations")
	defer timer.Stop()

	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 100
	}

	logging.StoreDebug("Retrieving recent activations: limit=%d minScore=%.4f", limit, minScore)

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
		logging.Get(logging.CategoryStore).Error("Failed to query recent activations: %v", err)
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

	logging.StoreDebug("Retrieved %d recent activations (minScore=%.4f)", len(activations), minScore)
	return activations, nil
}

// ========== Session History ==========

// StoreSessionTurn records a conversation turn.
// Uses INSERT OR IGNORE for idempotent syncing (duplicate turns are silently skipped).
func (s *LocalStore) StoreSessionTurn(sessionID string, turnNumber int, userInput, intentJSON, response, atomsJSON string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	logging.StoreDebug("Storing session turn: session=%s turn=%d input_len=%d response_len=%d",
		sessionID, turnNumber, len(userInput), len(response))

	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO session_history (session_id, turn_number, user_input, intent_json, response, atoms_json)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		sessionID, turnNumber, userInput, intentJSON, response, atomsJSON,
	)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to store session turn: session=%s turn=%d: %v", sessionID, turnNumber, err)
		return err
	}

	logging.StoreDebug("Session turn stored: session=%s turn=%d", sessionID, turnNumber)
	return nil
}

// GetSessionHistory retrieves session history.
func (s *LocalStore) GetSessionHistory(sessionID string, limit int) ([]map[string]interface{}, error) {
	timer := logging.StartTimer(logging.CategoryStore, "GetSessionHistory")
	defer timer.Stop()

	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 50
	}

	logging.StoreDebug("Retrieving session history: session=%s limit=%d", sessionID, limit)

	rows, err := s.db.Query(
		`SELECT turn_number, user_input, intent_json, response, atoms_json, created_at
		 FROM session_history
		 WHERE session_id = ?
		 ORDER BY turn_number DESC
		 LIMIT ?`,
		sessionID, limit,
	)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to query session history for %s: %v", sessionID, err)
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

	logging.StoreDebug("Retrieved %d session history turns for session=%s", len(history), sessionID)
	return history, nil
}

// ========== Compressed Context State ==========

// StoreCompressedState persists the semantic compression state for a session turn.
// Uses INSERT OR REPLACE so the latest state for a turn is idempotent.
func (s *LocalStore) StoreCompressedState(sessionID string, turnNumber int, stateJSON string, ratio float64) error {
	timer := logging.StartTimer(logging.CategoryStore, "StoreCompressedState")
	defer timer.Stop()

	if sessionID == "" || stateJSON == "" {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO compressed_states (session_id, turn_number, state_json, compression_ratio)
		 VALUES (?, ?, ?, ?)`,
		sessionID, turnNumber, stateJSON, ratio,
	)
	if err != nil {
		logging.Get(logging.CategoryStore).Warn("Failed to store compressed state: session=%s turn=%d: %v", sessionID, turnNumber, err)
		return err
	}

	logging.StoreDebug("Stored compressed state: session=%s turn=%d ratio=%.1f", sessionID, turnNumber, ratio)
	return nil
}

// LoadLatestCompressedState loads the most recent compressed state for a session.
// Returns empty string if no state exists.
func (s *LocalStore) LoadLatestCompressedState(sessionID string) (string, int, float64, error) {
	timer := logging.StartTimer(logging.CategoryStore, "LoadLatestCompressedState")
	defer timer.Stop()

	if sessionID == "" {
		return "", 0, 1.0, nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	var stateJSON string
	var turnNumber int
	var ratio float64
	err := s.db.QueryRow(
		`SELECT state_json, turn_number, compression_ratio
		 FROM compressed_states
		 WHERE session_id = ?
		 ORDER BY turn_number DESC
		 LIMIT 1`,
		sessionID,
	).Scan(&stateJSON, &turnNumber, &ratio)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", 0, 1.0, nil
		}
		return "", 0, 1.0, err
	}

	return stateJSON, turnNumber, ratio, nil
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
	timer := logging.StartTimer(logging.CategoryStore, "StoreVerification")
	defer timer.Stop()

	s.mu.Lock()
	defer s.mu.Unlock()

	logging.StoreDebug("Storing verification: session=%s turn=%d shard=%s attempt=%d success=%v confidence=%.2f",
		sessionID, turnNumber, shardType, attemptNumber, success, confidence)

	_, err := s.db.Exec(
		`INSERT INTO task_verifications
		 (session_id, turn_number, task, shard_type, attempt_number, success, confidence, reason, quality_violations, corrective_action, evidence, result_hash)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		sessionID, turnNumber, task, shardType, attemptNumber, success, confidence, reason, violationsJSON, correctiveJSON, evidenceJSON, resultHash,
	)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to store verification: session=%s turn=%d: %v", sessionID, turnNumber, err)
		return err
	}

	logging.StoreDebug("Verification stored: session=%s turn=%d shard=%s success=%v", sessionID, turnNumber, shardType, success)
	return nil
}

// GetVerificationHistory retrieves verification attempts for a session.
func (s *LocalStore) GetVerificationHistory(sessionID string, limit int) ([]VerificationRecord, error) {
	timer := logging.StartTimer(logging.CategoryStore, "GetVerificationHistory")
	defer timer.Stop()

	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 50
	}

	logging.StoreDebug("Retrieving verification history: session=%s limit=%d", sessionID, limit)

	rows, err := s.db.Query(
		`SELECT id, session_id, turn_number, task, shard_type, attempt_number, success, confidence, reason, quality_violations, corrective_action, evidence, result_hash, created_at
		 FROM task_verifications
		 WHERE session_id = ?
		 ORDER BY created_at DESC
		 LIMIT ?`,
		sessionID, limit,
	)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to query verification history for %s: %v", sessionID, err)
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

	logging.StoreDebug("Retrieved %d verification records for session=%s", len(records), sessionID)
	return records, nil
}

// GetQualityViolationStats retrieves statistics on quality violations for learning.
func (s *LocalStore) GetQualityViolationStats() (map[string]int, error) {
	timer := logging.StartTimer(logging.CategoryStore, "GetQualityViolationStats")
	defer timer.Stop()

	s.mu.RLock()
	defer s.mu.RUnlock()

	logging.StoreDebug("Computing quality violation statistics")

	rows, err := s.db.Query(
		`SELECT quality_violations, COUNT(*) as count
		 FROM task_verifications
		 WHERE success = 0 AND quality_violations != '[]'
		 GROUP BY quality_violations
		 ORDER BY count DESC`,
	)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to query quality violation stats: %v", err)
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

	logging.StoreDebug("Quality violation stats computed: %d unique violation patterns", len(stats))
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
// Note: ReasoningTrace type is defined in trace_store.go to avoid duplication

// StoreReasoningTrace persists a reasoning trace.
// Implements perception.TraceStore interface.
// Accepts interface{} to avoid import cycles - uses reflection to extract fields.
func (s *LocalStore) StoreReasoningTrace(trace interface{}) error {
	// Try direct ReasoningTrace (from store package)
	if rt, ok := trace.(*ReasoningTrace); ok {
		return s.traceStore.StoreReasoningTrace(rt)
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

	// Extract fields by name using reflection
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

	// Delegate to trace store
	return s.traceStore.storeReasoningTraceRaw(
		id, shardID, shardType, shardCategory, sessionID, taskContext,
		systemPrompt, userPrompt, response, model, errorMessage,
		tokensUsed, durationMs, success, qualityScore, learningNotes,
	)
}

// GetShardTraces retrieves traces for a specific shard type.
// Implements perception.ShardTraceReader interface.
func (s *LocalStore) GetShardTraces(shardType string, limit int) ([]ReasoningTrace, error) {
	return s.traceStore.GetShardTraces(shardType, limit)
}

// GetFailedShardTraces retrieves failed traces for a shard type.
func (s *LocalStore) GetFailedShardTraces(shardType string, limit int) ([]ReasoningTrace, error) {
	return s.traceStore.GetFailedShardTraces(shardType, limit)
}

// GetSimilarTaskTraces finds traces with similar task context.
func (s *LocalStore) GetSimilarTaskTraces(shardType, taskPattern string, limit int) ([]ReasoningTrace, error) {
	return s.traceStore.GetSimilarTaskTraces(shardType, taskPattern, limit)
}

// GetHighQualityTraces retrieves successful traces with high quality scores.
func (s *LocalStore) GetHighQualityTraces(shardType string, minScore float64, limit int) ([]ReasoningTrace, error) {
	return s.traceStore.GetHighQualityTraces(shardType, minScore, limit)
}

// GetRecentTraces retrieves recent traces across all shards.
// Used by main agent for oversight.
func (s *LocalStore) GetRecentTraces(limit int) ([]ReasoningTrace, error) {
	return s.traceStore.GetRecentTraces(limit)
}

// GetTracesBySession retrieves all traces for a specific session.
func (s *LocalStore) GetTracesBySession(sessionID string) ([]ReasoningTrace, error) {
	return s.traceStore.GetTracesBySession(sessionID)
}

// GetTracesByCategory retrieves traces by shard category.
func (s *LocalStore) GetTracesByCategory(category string, limit int) ([]ReasoningTrace, error) {
	return s.traceStore.GetTracesByCategory(category, limit)
}

// UpdateTraceQuality updates the quality score and learning notes for a trace.
func (s *LocalStore) UpdateTraceQuality(traceID string, score float64, notes []string) error {
	return s.traceStore.UpdateTraceQuality(traceID, score, notes)
}

// GetTraceStats returns statistics about reasoning traces.
func (s *LocalStore) GetTraceStats() (map[string]interface{}, error) {
	return s.traceStore.GetTraceStats()
}

// scanTraces is deprecated - functionality moved to TraceStore.
// Kept for backward compatibility with any external code that might call it.
func (s *LocalStore) scanTraces(rows *sql.Rows) ([]ReasoningTrace, error) {
	return s.traceStore.scanTraces(rows)
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
	timer := logging.StartTimer(logging.CategoryStore, "GetStats")
	defer timer.Stop()

	s.mu.RLock()
	defer s.mu.RUnlock()

	logging.StoreDebug("Computing database statistics")

	stats := make(map[string]int64)
	tables := []string{"vectors", "knowledge_graph", "cold_storage", "activation_log", "session_history", "compressed_states", "knowledge_atoms", "world_files", "world_facts"}

	for _, table := range tables {
		var count int64
		err := s.db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", table)).Scan(&count)
		if err != nil {
			logging.StoreDebug("Table %s count failed (may not exist): %v", table, err)
			continue
		}
		stats[table] = count
	}

	logging.StoreDebug("Database stats computed: tables=%d", len(stats))
	return stats, nil
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

// HydrateKnowledgeGraph loads all knowledge graph entries and converts them to
// knowledge_link facts for injection into the Mangle kernel.
// This method should be called during kernel initialization or when the knowledge
// graph is updated to ensure facts are available to Mangle rules.
func (s *LocalStore) HydrateKnowledgeGraph(assertFunc func(predicate string, args []interface{}) error) (int, error) {
	timer := logging.StartTimer(logging.CategoryStore, "HydrateKnowledgeGraph")
	defer timer.Stop()

	s.mu.RLock()
	defer s.mu.RUnlock()

	logging.Store("Hydrating knowledge graph into Mangle kernel")

	// Query all knowledge graph entries
	rows, err := s.db.Query(
		`SELECT entity_a, relation, entity_b, weight FROM knowledge_graph ORDER BY weight DESC`,
	)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to query knowledge graph for hydration: %v", err)
		return 0, fmt.Errorf("failed to query knowledge graph: %w", err)
	}
	defer rows.Close()

	count := 0
	skipped := 0
	for rows.Next() {
		var entityA, relation, entityB string
		var weight float64
		if err := rows.Scan(&entityA, &relation, &entityB, &weight); err != nil {
			skipped++
			continue // Skip malformed entries
		}

		// Convert to Mangle fact: knowledge_link(entity_a, relation, entity_b)
		if err := assertFunc("knowledge_link", []interface{}{entityA, relation, entityB}); err == nil {
			count++
		} else {
			skipped++
		}
	}

	logging.Store("Knowledge graph hydration complete: asserted=%d, skipped=%d", count, skipped)
	return count, nil
}

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

// ========== Review Findings Storage ==========

// StoredReviewFinding represents a review finding to be persisted.
// Defined here to avoid circular dependency with reviewer package.
type StoredReviewFinding struct {
	FilePath    string
	Line        int
	Severity    string
	Category    string
	RuleID      string
	Message     string
	ProjectRoot string
}

// StoreReviewFinding persists a review finding to the database.
func (s *LocalStore) StoreReviewFinding(f StoredReviewFinding) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	logging.StoreDebug("Storing review finding: file=%s line=%d severity=%s category=%s rule=%s",
		f.FilePath, f.Line, f.Severity, f.Category, f.RuleID)

	_, err := s.db.Exec(
		`INSERT INTO review_findings (file_path, line, severity, category, rule_id, message, project_root)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		f.FilePath, f.Line, f.Severity, f.Category, f.RuleID, f.Message, f.ProjectRoot,
	)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to store review finding for %s:%d: %v", f.FilePath, f.Line, err)
		return err
	}

	logging.StoreDebug("Review finding stored: %s:%d [%s]", f.FilePath, f.Line, f.Severity)
	return nil
}

// ========== Prompt Atoms Storage (Universal JIT Prompt Compiler) ==========

// PromptAtom represents an atomic unit of prompt content for JIT compilation.
// Atoms are selected based on contextual dimensions and assembled into complete prompts.
type PromptAtom struct {
	ID          int64
	AtomID      string // Unique identifier (e.g., "identity/coder/mission")
	Version     int
	Content     string
	TokenCount  int
	ContentHash string

	// Classification
	Category    string // Primary category (identity, protocol, safety, methodology, etc.)
	Subcategory string // Optional subcategory

	// Contextual Selectors (when this atom applies)
	OperationalModes []string // ["/active", "/debugging", "/dream", etc.]
	CampaignPhases   []string // ["/planning", "/active", "/completed", etc.]
	BuildLayers      []string // ["/scaffold", "/domain_core", "/service", etc.]
	InitPhases       []string // ["/analysis", "/kb_agent", etc.]
	NorthstarPhases  []string // ["/doc_ingestion", "/requirements", etc.]
	OuroborosStages  []string // ["/detection", "/specification", etc.]
	IntentVerbs      []string // ["/fix", "/debug", "/refactor", etc.]
	ShardTypes       []string // ["/coder", "/tester", "/reviewer", etc.]
	Languages        []string // ["/go", "/python", "/typescript", etc.]
	Frameworks       []string // ["/bubbletea", "/gin", "/rod", etc.]
	WorldStates      []string // ["failing_tests", "diagnostics", etc.]

	// Composition rules
	Priority      int    // Higher = more important (0-100)
	IsMandatory   bool   // Must always be included
	IsExclusive   string // Exclusion group (only one from group)
	DependsOn     []string
	ConflictsWith []string

	// Embeddings
	Embedding     []byte
	EmbeddingTask string // Task type used for embedding (RETRIEVAL_DOCUMENT)

	CreatedAt time.Time
}

// StorePromptAtom persists a prompt atom to the database.
func (s *LocalStore) StorePromptAtom(atom *PromptAtom) error {
	timer := logging.StartTimer(logging.CategoryStore, "StorePromptAtom")
	defer timer.Stop()

	s.mu.Lock()
	defer s.mu.Unlock()

	logging.StoreDebug("Storing prompt atom: atom_id=%s category=%s tokens=%d",
		atom.AtomID, atom.Category, atom.TokenCount)

	// Serialize JSON arrays
	operationalModesJSON, _ := json.Marshal(atom.OperationalModes)
	campaignPhasesJSON, _ := json.Marshal(atom.CampaignPhases)
	buildLayersJSON, _ := json.Marshal(atom.BuildLayers)
	initPhasesJSON, _ := json.Marshal(atom.InitPhases)
	northstarPhasesJSON, _ := json.Marshal(atom.NorthstarPhases)
	ouroborosStagesJSON, _ := json.Marshal(atom.OuroborosStages)
	intentVerbsJSON, _ := json.Marshal(atom.IntentVerbs)
	shardTypesJSON, _ := json.Marshal(atom.ShardTypes)
	languagesJSON, _ := json.Marshal(atom.Languages)
	frameworksJSON, _ := json.Marshal(atom.Frameworks)
	worldStatesJSON, _ := json.Marshal(atom.WorldStates)
	dependsOnJSON, _ := json.Marshal(atom.DependsOn)
	conflictsWithJSON, _ := json.Marshal(atom.ConflictsWith)

	_, err := s.db.Exec(`
		INSERT INTO prompt_atoms (
			atom_id, version, content, token_count, content_hash,
			category, subcategory,
			operational_modes, campaign_phases, build_layers, init_phases,
			northstar_phases, ouroboros_stages, intent_verbs, shard_types,
			languages, frameworks, world_states,
			priority, is_mandatory, is_exclusive, depends_on, conflicts_with,
			embedding, embedding_task
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(atom_id) DO UPDATE SET
			version = excluded.version,
			content = excluded.content,
			token_count = excluded.token_count,
			content_hash = excluded.content_hash,
			category = excluded.category,
			subcategory = excluded.subcategory,
			operational_modes = excluded.operational_modes,
			campaign_phases = excluded.campaign_phases,
			build_layers = excluded.build_layers,
			init_phases = excluded.init_phases,
			northstar_phases = excluded.northstar_phases,
			ouroboros_stages = excluded.ouroboros_stages,
			intent_verbs = excluded.intent_verbs,
			shard_types = excluded.shard_types,
			languages = excluded.languages,
			frameworks = excluded.frameworks,
			world_states = excluded.world_states,
			priority = excluded.priority,
			is_mandatory = excluded.is_mandatory,
			is_exclusive = excluded.is_exclusive,
			depends_on = excluded.depends_on,
			conflicts_with = excluded.conflicts_with,
			embedding = excluded.embedding,
			embedding_task = excluded.embedding_task`,
		atom.AtomID, atom.Version, atom.Content, atom.TokenCount, atom.ContentHash,
		atom.Category, atom.Subcategory,
		string(operationalModesJSON), string(campaignPhasesJSON), string(buildLayersJSON), string(initPhasesJSON),
		string(northstarPhasesJSON), string(ouroborosStagesJSON), string(intentVerbsJSON), string(shardTypesJSON),
		string(languagesJSON), string(frameworksJSON), string(worldStatesJSON),
		atom.Priority, atom.IsMandatory, atom.IsExclusive, string(dependsOnJSON), string(conflictsWithJSON),
		atom.Embedding, atom.EmbeddingTask,
	)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to store prompt atom %s: %v", atom.AtomID, err)
		return fmt.Errorf("failed to store prompt atom: %w", err)
	}

	logging.StoreDebug("Prompt atom stored: atom_id=%s", atom.AtomID)
	return nil
}

// LoadPromptAtoms retrieves all prompt atoms from the database.
func (s *LocalStore) LoadPromptAtoms() ([]*PromptAtom, error) {
	timer := logging.StartTimer(logging.CategoryStore, "LoadPromptAtoms")
	defer timer.Stop()

	s.mu.RLock()
	defer s.mu.RUnlock()

	logging.StoreDebug("Loading all prompt atoms")

	rows, err := s.db.Query(`
		SELECT id, atom_id, version, content, token_count, content_hash,
			   category, subcategory,
			   operational_modes, campaign_phases, build_layers, init_phases,
			   northstar_phases, ouroboros_stages, intent_verbs, shard_types,
			   languages, frameworks, world_states,
			   priority, is_mandatory, is_exclusive, depends_on, conflicts_with,
			   embedding, embedding_task, created_at
		FROM prompt_atoms
		ORDER BY priority DESC, category, atom_id`)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to query prompt atoms: %v", err)
		return nil, fmt.Errorf("failed to query prompt atoms: %w", err)
	}
	defer rows.Close()

	atoms, err := s.scanPromptAtoms(rows)
	if err != nil {
		return nil, err
	}

	logging.StoreDebug("Loaded %d prompt atoms", len(atoms))
	return atoms, nil
}

// LoadPromptAtomsByCategory retrieves prompt atoms filtered by category.
func (s *LocalStore) LoadPromptAtomsByCategory(category string) ([]*PromptAtom, error) {
	timer := logging.StartTimer(logging.CategoryStore, "LoadPromptAtomsByCategory")
	defer timer.Stop()

	s.mu.RLock()
	defer s.mu.RUnlock()

	logging.StoreDebug("Loading prompt atoms by category: %s", category)

	rows, err := s.db.Query(`
		SELECT id, atom_id, version, content, token_count, content_hash,
			   category, subcategory,
			   operational_modes, campaign_phases, build_layers, init_phases,
			   northstar_phases, ouroboros_stages, intent_verbs, shard_types,
			   languages, frameworks, world_states,
			   priority, is_mandatory, is_exclusive, depends_on, conflicts_with,
			   embedding, embedding_task, created_at
		FROM prompt_atoms
		WHERE category = ?
		ORDER BY priority DESC, atom_id`, category)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to query prompt atoms by category %s: %v", category, err)
		return nil, fmt.Errorf("failed to query prompt atoms by category: %w", err)
	}
	defer rows.Close()

	atoms, err := s.scanPromptAtoms(rows)
	if err != nil {
		return nil, err
	}

	logging.StoreDebug("Loaded %d prompt atoms for category=%s", len(atoms), category)
	return atoms, nil
}

// GetPromptAtom retrieves a single prompt atom by its atom_id.
func (s *LocalStore) GetPromptAtom(atomID string) (*PromptAtom, error) {
	timer := logging.StartTimer(logging.CategoryStore, "GetPromptAtom")
	defer timer.Stop()

	s.mu.RLock()
	defer s.mu.RUnlock()

	logging.StoreDebug("Getting prompt atom: atom_id=%s", atomID)

	rows, err := s.db.Query(`
		SELECT id, atom_id, version, content, token_count, content_hash,
			   category, subcategory,
			   operational_modes, campaign_phases, build_layers, init_phases,
			   northstar_phases, ouroboros_stages, intent_verbs, shard_types,
			   languages, frameworks, world_states,
			   priority, is_mandatory, is_exclusive, depends_on, conflicts_with,
			   embedding, embedding_task, created_at
		FROM prompt_atoms
		WHERE atom_id = ?`, atomID)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to query prompt atom %s: %v", atomID, err)
		return nil, fmt.Errorf("failed to query prompt atom: %w", err)
	}
	defer rows.Close()

	atoms, err := s.scanPromptAtoms(rows)
	if err != nil {
		return nil, err
	}

	if len(atoms) == 0 {
		logging.StoreDebug("Prompt atom not found: atom_id=%s", atomID)
		return nil, nil
	}

	logging.StoreDebug("Found prompt atom: atom_id=%s", atomID)
	return atoms[0], nil
}

// DeletePromptAtom removes a prompt atom by its atom_id.
func (s *LocalStore) DeletePromptAtom(atomID string) error {
	timer := logging.StartTimer(logging.CategoryStore, "DeletePromptAtom")
	defer timer.Stop()

	s.mu.Lock()
	defer s.mu.Unlock()

	logging.StoreDebug("Deleting prompt atom: atom_id=%s", atomID)

	result, err := s.db.Exec("DELETE FROM prompt_atoms WHERE atom_id = ?", atomID)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to delete prompt atom %s: %v", atomID, err)
		return fmt.Errorf("failed to delete prompt atom: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		logging.StoreDebug("Prompt atom not found for deletion: atom_id=%s", atomID)
		return nil
	}

	logging.StoreDebug("Deleted prompt atom: atom_id=%s", atomID)
	return nil
}

// scanPromptAtoms scans rows into PromptAtom structs.
func (s *LocalStore) scanPromptAtoms(rows *sql.Rows) ([]*PromptAtom, error) {
	var atoms []*PromptAtom

	for rows.Next() {
		atom := &PromptAtom{}
		var operationalModesJSON, campaignPhasesJSON, buildLayersJSON, initPhasesJSON string
		var northstarPhasesJSON, ouroborosStagesJSON, intentVerbsJSON, shardTypesJSON string
		var languagesJSON, frameworksJSON, worldStatesJSON string
		var dependsOnJSON, conflictsWithJSON string
		var subcategory, isExclusive, embeddingTask sql.NullString
		var embedding []byte

		err := rows.Scan(
			&atom.ID, &atom.AtomID, &atom.Version, &atom.Content, &atom.TokenCount, &atom.ContentHash,
			&atom.Category, &subcategory,
			&operationalModesJSON, &campaignPhasesJSON, &buildLayersJSON, &initPhasesJSON,
			&northstarPhasesJSON, &ouroborosStagesJSON, &intentVerbsJSON, &shardTypesJSON,
			&languagesJSON, &frameworksJSON, &worldStatesJSON,
			&atom.Priority, &atom.IsMandatory, &isExclusive, &dependsOnJSON, &conflictsWithJSON,
			&embedding, &embeddingTask, &atom.CreatedAt,
		)
		if err != nil {
			logging.Get(logging.CategoryStore).Warn("Failed to scan prompt atom row: %v", err)
			continue
		}

		// Handle nullable fields
		if subcategory.Valid {
			atom.Subcategory = subcategory.String
		}
		if isExclusive.Valid {
			atom.IsExclusive = isExclusive.String
		}
		if embeddingTask.Valid {
			atom.EmbeddingTask = embeddingTask.String
		}
		atom.Embedding = embedding

		// Deserialize JSON arrays
		json.Unmarshal([]byte(operationalModesJSON), &atom.OperationalModes)
		json.Unmarshal([]byte(campaignPhasesJSON), &atom.CampaignPhases)
		json.Unmarshal([]byte(buildLayersJSON), &atom.BuildLayers)
		json.Unmarshal([]byte(initPhasesJSON), &atom.InitPhases)
		json.Unmarshal([]byte(northstarPhasesJSON), &atom.NorthstarPhases)
		json.Unmarshal([]byte(ouroborosStagesJSON), &atom.OuroborosStages)
		json.Unmarshal([]byte(intentVerbsJSON), &atom.IntentVerbs)
		json.Unmarshal([]byte(shardTypesJSON), &atom.ShardTypes)
		json.Unmarshal([]byte(languagesJSON), &atom.Languages)
		json.Unmarshal([]byte(frameworksJSON), &atom.Frameworks)
		json.Unmarshal([]byte(worldStatesJSON), &atom.WorldStates)
		json.Unmarshal([]byte(dependsOnJSON), &atom.DependsOn)
		json.Unmarshal([]byte(conflictsWithJSON), &atom.ConflictsWith)

		atoms = append(atoms, atom)
	}

	if err := rows.Err(); err != nil {
		logging.Get(logging.CategoryStore).Error("Error iterating prompt atom rows: %v", err)
		return nil, fmt.Errorf("error iterating prompt atom rows: %w", err)
	}

	return atoms, nil
}
