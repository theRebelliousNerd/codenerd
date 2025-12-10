// Package store - Learned corpus store for dynamic intent patterns.
// This file implements a writable store for learned patterns that are
// discovered during runtime through user interactions and feedback.
package store

import (
	"bytes"
	"codenerd/internal/embedding"
	"codenerd/internal/logging"
	"context"
	"database/sql"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// LearnedPattern represents a dynamically learned intent pattern.
type LearnedPattern struct {
	ID         int64     // Database ID
	Pattern    string    // The natural language pattern text
	Verb       string    // Intent verb (e.g., "create", "fix")
	Target     string    // Intent target (e.g., "function", "test")
	Constraint string    // Optional constraint text
	Confidence float64   // Confidence score (0.0-1.0), can decay over time
	CreatedAt  time.Time // When the pattern was first learned
}

// LearnedCorpusStore manages dynamically learned patterns with embeddings.
// Patterns are stored in a user-local SQLite database and can be updated
// as the system learns from user interactions.
type LearnedCorpusStore struct {
	db          *sql.DB
	embedEngine embedding.EmbeddingEngine
	dbPath      string
	mu          sync.RWMutex
}

// NewLearnedCorpusStore creates or opens the learned corpus store.
// Creates the database and schema if it doesn't exist.
//
// Parameters:
//   - dbPath: Path to the SQLite database file (e.g., ".nerd/learned_corpus.db")
//   - engine: Embedding engine for generating pattern embeddings
func NewLearnedCorpusStore(dbPath string, engine embedding.EmbeddingEngine) (*LearnedCorpusStore, error) {
	timer := logging.StartTimer(logging.CategoryStore, "NewLearnedCorpusStore")
	defer timer.Stop()

	if dbPath == "" {
		return nil, fmt.Errorf("database path required")
	}

	logging.Store("Initializing learned corpus store at: %s", dbPath)

	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to create directory %s: %v", dir, err)
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	// Open database
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to open learned corpus database: %v", err)
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Verify connection
	if err := db.Ping(); err != nil {
		db.Close()
		logging.Get(logging.CategoryStore).Error("Failed to ping learned corpus database: %v", err)
		return nil, fmt.Errorf("failed to verify database connection: %w", err)
	}

	store := &LearnedCorpusStore{
		db:          db,
		embedEngine: engine,
		dbPath:      dbPath,
	}

	// Initialize schema
	if err := store.initializeSchema(); err != nil {
		db.Close()
		logging.Get(logging.CategoryStore).Error("Failed to initialize learned corpus schema: %v", err)
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	logging.Store("Learned corpus store initialized successfully")
	return store, nil
}

// initializeSchema creates the required tables for learned patterns.
func (s *LearnedCorpusStore) initializeSchema() error {
	timer := logging.StartTimer(logging.CategoryStore, "LearnedCorpusStore.initializeSchema")
	defer timer.Stop()

	logging.StoreDebug("Initializing learned corpus schema")

	// Main patterns table
	patternsTable := `
	CREATE TABLE IF NOT EXISTS learned_patterns (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		pattern TEXT NOT NULL UNIQUE,
		verb TEXT NOT NULL,
		target TEXT,
		constraint_text TEXT,
		confidence REAL DEFAULT 1.0,
		embedding BLOB NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_learned_verb ON learned_patterns(verb);
	CREATE INDEX IF NOT EXISTS idx_learned_confidence ON learned_patterns(confidence);
	CREATE INDEX IF NOT EXISTS idx_learned_created ON learned_patterns(created_at);
	`

	if _, err := s.db.Exec(patternsTable); err != nil {
		return fmt.Errorf("failed to create patterns table: %w", err)
	}

	// Create sqlite-vec virtual table for ANN search
	// We need to determine embedding dimensions from the engine
	dims := 3072 // Default to Gemini embedding size (updated from 768)
	if s.embedEngine != nil {
		dims = s.embedEngine.Dimensions()
	}

	vecTable := fmt.Sprintf(`
	CREATE VIRTUAL TABLE IF NOT EXISTS vec_learned USING vec0(
		embedding float[%d],
		pattern TEXT,
		verb TEXT
	);
	`, dims)

	if _, err := s.db.Exec(vecTable); err != nil {
		// Log warning but don't fail - vec extension might not be available
		logging.Get(logging.CategoryStore).Warn("Failed to create vec_learned table (sqlite-vec may not be available): %v", err)
	} else {
		logging.StoreDebug("sqlite-vec table created with %d dimensions", dims)
	}

	logging.StoreDebug("Learned corpus schema initialized")
	return nil
}

// AddPattern adds a learned pattern with its embedding.
// Generates embedding automatically using the configured engine.
func (s *LearnedCorpusStore) AddPattern(ctx context.Context, pattern, verb, target, constraint string, confidence float64) error {
	timer := logging.StartTimer(logging.CategoryStore, "LearnedCorpusStore.AddPattern")
	defer timer.Stop()

	if pattern == "" {
		return fmt.Errorf("pattern text required")
	}
	if verb == "" {
		return fmt.Errorf("verb required")
	}

	logging.StoreDebug("Adding learned pattern: verb=%s target=%s confidence=%.2f", verb, target, confidence)

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.embedEngine == nil {
		return fmt.Errorf("embedding engine not configured")
	}

	// Generate embedding for the pattern
	embeddingVec, err := s.embedEngine.Embed(ctx, pattern)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to generate embedding for pattern: %v", err)
		return fmt.Errorf("failed to generate embedding: %w", err)
	}

	logging.StoreDebug("Generated embedding: %d dimensions", len(embeddingVec))

	// Encode embedding as binary blob
	embeddingBlob := encodeFloat32SliceToBlob(embeddingVec)

	// Insert or update pattern
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO learned_patterns (pattern, verb, target, constraint_text, confidence, embedding, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(pattern) DO UPDATE SET
			verb = excluded.verb,
			target = excluded.target,
			constraint_text = excluded.constraint_text,
			confidence = MIN(1.0, confidence + 0.1),
			embedding = excluded.embedding,
			updated_at = CURRENT_TIMESTAMP
	`, pattern, verb, target, constraint, confidence, embeddingBlob)

	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to insert learned pattern: %v", err)
		return fmt.Errorf("failed to insert pattern: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	logging.StoreDebug("Pattern stored (rows affected: %d)", rowsAffected)

	// Also insert into vec table for ANN search
	if _, err := s.db.ExecContext(ctx, `
		INSERT OR REPLACE INTO vec_learned (embedding, pattern, verb)
		VALUES (?, ?, ?)
	`, embeddingBlob, pattern, verb); err != nil {
		// Non-fatal: vec table might not exist
		logging.Get(logging.CategoryStore).Warn("Failed to insert into vec_learned (ANN may be unavailable): %v", err)
	}

	logging.Store("Learned pattern added: verb=%s target=%s", verb, target)
	return nil
}

// Search performs ANN search against learned patterns.
func (s *LearnedCorpusStore) Search(queryEmbedding []float32, topK int) ([]SemanticMatch, error) {
	timer := logging.StartTimer(logging.CategoryStore, "LearnedCorpusStore.Search")
	defer timer.Stop()

	if topK <= 0 {
		topK = 5
	}

	logging.StoreDebug("Searching learned corpus: topK=%d, embedding_dims=%d", topK, len(queryEmbedding))

	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.db == nil {
		return nil, fmt.Errorf("learned corpus store not initialized")
	}

	queryBlob := encodeFloat32SliceToBlob(queryEmbedding)

	// Try vec table first (fast ANN search)
	matches, err := s.searchVec(queryBlob, topK)
	if err != nil {
		// Fall back to brute-force search if vec table not available
		logging.StoreDebug("Falling back to brute-force search: %v", err)
		return s.searchBruteForce(queryEmbedding, topK)
	}

	logging.StoreDebug("Learned corpus search returned %d matches", len(matches))
	return matches, nil
}

// searchVec performs ANN search using sqlite-vec.
func (s *LearnedCorpusStore) searchVec(queryBlob []byte, topK int) ([]SemanticMatch, error) {
	// Join vec_learned with learned_patterns to get full metadata
	query := `
		SELECT
			lp.pattern,
			lp.verb,
			lp.target,
			lp.constraint_text,
			lp.confidence,
			vec_distance_cosine(vl.embedding, ?) AS distance
		FROM vec_learned vl
		JOIN learned_patterns lp ON vl.pattern = lp.pattern
		WHERE lp.confidence > 0.3
		ORDER BY distance ASC
		LIMIT ?
	`

	rows, err := s.db.Query(query, queryBlob, topK)
	if err != nil {
		return nil, fmt.Errorf("vec search failed: %w", err)
	}
	defer rows.Close()

	var matches []SemanticMatch
	rank := 1
	for rows.Next() {
		var match SemanticMatch
		var distance, confidence float64
		var constraintText sql.NullString

		if err := rows.Scan(
			&match.TextContent,
			&match.Verb,
			&match.Target,
			&constraintText,
			&confidence,
			&distance,
		); err != nil {
			logging.Get(logging.CategoryStore).Warn("Failed to scan learned pattern row: %v", err)
			continue
		}

		match.Predicate = "learned_intent"
		match.Category = "learned"
		match.Similarity = 1.0 - distance
		match.Rank = rank
		rank++

		matches = append(matches, match)
	}

	return matches, rows.Err()
}

// searchBruteForce performs brute-force cosine similarity search.
// Used as fallback when sqlite-vec is not available.
func (s *LearnedCorpusStore) searchBruteForce(queryEmbedding []float32, topK int) ([]SemanticMatch, error) {
	rows, err := s.db.Query(`
		SELECT id, pattern, verb, target, constraint_text, confidence, embedding
		FROM learned_patterns
		WHERE confidence > 0.3
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query patterns: %w", err)
	}
	defer rows.Close()

	type candidate struct {
		match      SemanticMatch
		similarity float64
	}

	var candidates []candidate

	for rows.Next() {
		var id int64
		var pattern, verb string
		var target, constraintText sql.NullString
		var confidence float64
		var embeddingBlob []byte

		if err := rows.Scan(&id, &pattern, &verb, &target, &constraintText, &confidence, &embeddingBlob); err != nil {
			continue
		}

		// Decode embedding from blob
		patternEmbedding := decodeFloat32SliceFromBlob(embeddingBlob)
		if len(patternEmbedding) == 0 {
			continue
		}

		// Calculate cosine similarity
		similarity, err := embedding.CosineSimilarity(queryEmbedding, patternEmbedding)
		if err != nil {
			continue
		}

		match := SemanticMatch{
			TextContent: pattern,
			Predicate:   "learned_intent",
			Verb:        verb,
			Target:      target.String,
			Category:    "learned",
			Similarity:  similarity,
		}

		candidates = append(candidates, candidate{match: match, similarity: similarity})
	}

	// Sort by similarity descending
	for i := 0; i < len(candidates); i++ {
		for j := i + 1; j < len(candidates); j++ {
			if candidates[j].similarity > candidates[i].similarity {
				candidates[i], candidates[j] = candidates[j], candidates[i]
			}
		}
	}

	// Return top K
	if len(candidates) > topK {
		candidates = candidates[:topK]
	}

	matches := make([]SemanticMatch, len(candidates))
	for i, c := range candidates {
		c.match.Rank = i + 1
		matches[i] = c.match
	}

	return matches, nil
}

// GetAllPatterns returns all stored learned patterns (for persistence/export).
func (s *LearnedCorpusStore) GetAllPatterns() ([]LearnedPattern, error) {
	timer := logging.StartTimer(logging.CategoryStore, "LearnedCorpusStore.GetAllPatterns")
	defer timer.Stop()

	s.mu.RLock()
	defer s.mu.RUnlock()

	logging.StoreDebug("Retrieving all learned patterns")

	rows, err := s.db.Query(`
		SELECT id, pattern, verb, target, constraint_text, confidence, created_at
		FROM learned_patterns
		ORDER BY confidence DESC, created_at DESC
	`)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to query learned patterns: %v", err)
		return nil, fmt.Errorf("failed to query patterns: %w", err)
	}
	defer rows.Close()

	var patterns []LearnedPattern
	for rows.Next() {
		var p LearnedPattern
		var target, constraint sql.NullString

		if err := rows.Scan(&p.ID, &p.Pattern, &p.Verb, &target, &constraint, &p.Confidence, &p.CreatedAt); err != nil {
			logging.Get(logging.CategoryStore).Warn("Failed to scan pattern row: %v", err)
			continue
		}

		p.Target = target.String
		p.Constraint = constraint.String
		patterns = append(patterns, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating patterns: %w", err)
	}

	logging.StoreDebug("Retrieved %d learned patterns", len(patterns))
	return patterns, nil
}

// GetPatternsByVerb retrieves patterns filtered by verb.
func (s *LearnedCorpusStore) GetPatternsByVerb(verb string) ([]LearnedPattern, error) {
	timer := logging.StartTimer(logging.CategoryStore, "LearnedCorpusStore.GetPatternsByVerb")
	defer timer.Stop()

	s.mu.RLock()
	defer s.mu.RUnlock()

	logging.StoreDebug("Retrieving learned patterns by verb: %s", verb)

	rows, err := s.db.Query(`
		SELECT id, pattern, verb, target, constraint_text, confidence, created_at
		FROM learned_patterns
		WHERE verb = ? AND confidence > 0.3
		ORDER BY confidence DESC
	`, verb)
	if err != nil {
		return nil, fmt.Errorf("failed to query patterns by verb: %w", err)
	}
	defer rows.Close()

	var patterns []LearnedPattern
	for rows.Next() {
		var p LearnedPattern
		var target, constraint sql.NullString

		if err := rows.Scan(&p.ID, &p.Pattern, &p.Verb, &target, &constraint, &p.Confidence, &p.CreatedAt); err != nil {
			continue
		}

		p.Target = target.String
		p.Constraint = constraint.String
		patterns = append(patterns, p)
	}

	logging.StoreDebug("Retrieved %d patterns for verb=%s", len(patterns), verb)
	return patterns, rows.Err()
}

// DecayConfidence reduces confidence of old patterns.
// Implements "forgetting" - patterns not reinforced will fade.
func (s *LearnedCorpusStore) DecayConfidence(decayFactor float64, olderThanDays int) (int, error) {
	timer := logging.StartTimer(logging.CategoryStore, "LearnedCorpusStore.DecayConfidence")
	defer timer.Stop()

	if decayFactor <= 0 || decayFactor >= 1 {
		decayFactor = 0.9 // Default 10% decay
	}
	if olderThanDays <= 0 {
		olderThanDays = 7
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	logging.Store("Decaying learned pattern confidence (factor=%.2f, older than %d days)", decayFactor, olderThanDays)

	// Decay old patterns
	result, err := s.db.Exec(`
		UPDATE learned_patterns
		SET confidence = confidence * ?,
		    updated_at = CURRENT_TIMESTAMP
		WHERE datetime(updated_at) < datetime('now', '-' || ? || ' days')
	`, decayFactor, olderThanDays)

	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to decay pattern confidence: %v", err)
		return 0, fmt.Errorf("failed to decay confidence: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	logging.StoreDebug("Decayed confidence on %d patterns", rowsAffected)

	// Delete patterns with very low confidence
	pruneResult, err := s.db.Exec(`DELETE FROM learned_patterns WHERE confidence < 0.1`)
	if err != nil {
		logging.Get(logging.CategoryStore).Warn("Failed to prune low-confidence patterns: %v", err)
	} else {
		pruned, _ := pruneResult.RowsAffected()
		if pruned > 0 {
			logging.Store("Pruned %d forgotten patterns (confidence < 0.1)", pruned)
		}
	}

	return int(rowsAffected), nil
}

// DeletePattern removes a specific pattern.
func (s *LearnedCorpusStore) DeletePattern(pattern string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	logging.StoreDebug("Deleting learned pattern: %s", pattern)

	// Delete from main table
	if _, err := s.db.Exec("DELETE FROM learned_patterns WHERE pattern = ?", pattern); err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to delete pattern: %v", err)
		return fmt.Errorf("failed to delete pattern: %w", err)
	}

	// Delete from vec table
	if _, err := s.db.Exec("DELETE FROM vec_learned WHERE pattern = ?", pattern); err != nil {
		logging.Get(logging.CategoryStore).Warn("Failed to delete from vec_learned: %v", err)
	}

	logging.StoreDebug("Pattern deleted: %s", pattern)
	return nil
}

// GetStats returns statistics about the learned corpus.
func (s *LearnedCorpusStore) GetStats() (map[string]interface{}, error) {
	timer := logging.StartTimer(logging.CategoryStore, "LearnedCorpusStore.GetStats")
	defer timer.Stop()

	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := make(map[string]interface{})

	// Total patterns
	var total int64
	if err := s.db.QueryRow("SELECT COUNT(*) FROM learned_patterns").Scan(&total); err == nil {
		stats["total_patterns"] = total
	}

	// Active patterns (confidence > 0.3)
	var active int64
	if err := s.db.QueryRow("SELECT COUNT(*) FROM learned_patterns WHERE confidence > 0.3").Scan(&active); err == nil {
		stats["active_patterns"] = active
	}

	// Average confidence
	var avgConfidence float64
	if err := s.db.QueryRow("SELECT COALESCE(AVG(confidence), 0) FROM learned_patterns").Scan(&avgConfidence); err == nil {
		stats["avg_confidence"] = avgConfidence
	}

	// Patterns by verb
	verbRows, err := s.db.Query("SELECT verb, COUNT(*) FROM learned_patterns GROUP BY verb ORDER BY COUNT(*) DESC")
	if err == nil {
		verbs := make(map[string]int64)
		for verbRows.Next() {
			var verb string
			var count int64
			if err := verbRows.Scan(&verb, &count); err == nil {
				verbs[verb] = count
			}
		}
		verbRows.Close()
		stats["by_verb"] = verbs
	}

	// Embedding engine info
	if s.embedEngine != nil {
		stats["embedding_engine"] = s.embedEngine.Name()
		stats["embedding_dimensions"] = s.embedEngine.Dimensions()
	} else {
		stats["embedding_engine"] = "none"
	}

	stats["source"] = "learned"
	stats["writable"] = true
	stats["db_path"] = s.dbPath

	logging.StoreDebug("Learned corpus stats: total=%d, active=%d, avg_confidence=%.2f", total, active, avgConfidence)
	return stats, nil
}

// SetEmbeddingEngine configures or updates the embedding engine.
// Can be called after store creation if engine wasn't available initially.
func (s *LearnedCorpusStore) SetEmbeddingEngine(engine embedding.EmbeddingEngine) {
	s.mu.Lock()
	defer s.mu.Unlock()

	logging.Store("Setting embedding engine for learned corpus: %s", engine.Name())
	s.embedEngine = engine

	// Re-initialize vec table with correct dimensions if needed
	dims := engine.Dimensions()
	vecTable := fmt.Sprintf(`
		CREATE VIRTUAL TABLE IF NOT EXISTS vec_learned USING vec0(
			embedding float[%d],
			pattern TEXT,
			verb TEXT
		);
	`, dims)

	if _, err := s.db.Exec(vecTable); err != nil {
		logging.Get(logging.CategoryStore).Warn("Failed to create/update vec_learned table: %v", err)
	}
}

// Close closes the database connection.
func (s *LearnedCorpusStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	logging.Store("Closing learned corpus store")

	if s.db != nil {
		if err := s.db.Close(); err != nil {
			logging.Get(logging.CategoryStore).Error("Failed to close learned corpus database: %v", err)
			return err
		}
		s.db = nil
	}

	logging.Store("Learned corpus store closed")
	return nil
}

// decodeFloat32SliceFromBlob decodes a binary blob back to float32 slice.
func decodeFloat32SliceFromBlob(blob []byte) []float32 {
	if len(blob) == 0 || len(blob)%4 != 0 {
		return nil
	}

	vec := make([]float32, len(blob)/4)
	reader := bytes.NewReader(blob)
	if err := binary.Read(reader, binary.LittleEndian, &vec); err != nil {
		return nil
	}
	return vec
}
