package store

import (
	"codenerd/internal/embedding"
	"codenerd/internal/logging"
	"database/sql"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"
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
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if _, err := db.Exec("PRAGMA busy_timeout = 5000"); err != nil {
		logging.StoreDebug("Failed to set sqlite busy_timeout: %v", err)
	}
	if _, err := db.Exec("PRAGMA journal_mode = WAL"); err != nil {
		logging.StoreDebug("Failed to set sqlite journal_mode=WAL: %v", err)
	}
	// PRAGMA synchronous=NORMAL provides 5-10x write speedup with WAL mode
	// (vs FULL which is default). Safe because WAL already provides crash recovery.
	if _, err := db.Exec("PRAGMA synchronous = NORMAL"); err != nil {
		logging.StoreDebug("Failed to set sqlite synchronous=NORMAL: %v", err)
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
