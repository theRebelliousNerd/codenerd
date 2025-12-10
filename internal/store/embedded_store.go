// Package store - Embedded corpus store for baked-in intent classification.
// This file implements read-only access to the pre-built intent corpus
// that is embedded into the binary at compile time.
package store

import (
	"bytes"
	"codenerd/internal/core/defaults"
	"codenerd/internal/logging"
	"database/sql"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"sync"
)

// SemanticMatch represents a match from semantic search.
type SemanticMatch struct {
	TextContent string  // Original text that was matched
	Predicate   string  // Mangle predicate (e.g., "user_intent")
	Verb        string  // Intent verb (e.g., "create", "fix", "explain")
	Target      string  // Intent target (e.g., "function", "test", "file")
	Category    string  // Intent category (e.g., "code", "test", "review")
	Similarity  float64 // Cosine similarity score (0.0-1.0)
	Rank        int     // Result rank (1-based)
}

// EmbeddedCorpusStore provides read-only access to the baked-in intent corpus.
// The corpus is extracted from go:embed to a temp file on first use since
// SQLite requires a file path, not an in-memory buffer.
type EmbeddedCorpusStore struct {
	db       *sql.DB
	tempPath string // Path to extracted temp DB file
	mu       sync.RWMutex
}

// NewEmbeddedCorpusStore initializes the embedded corpus.
// It extracts the embedded DB to a temp file and opens it read-only.
// Returns an error if the corpus is not available (e.g., development mode).
func NewEmbeddedCorpusStore() (*EmbeddedCorpusStore, error) {
	timer := logging.StartTimer(logging.CategoryStore, "NewEmbeddedCorpusStore")
	defer timer.Stop()

	logging.Store("Initializing embedded intent corpus store")

	// Check if corpus is available
	if !defaults.IntentCorpusAvailable() {
		logging.Get(logging.CategoryStore).Warn("Embedded intent corpus not available (development mode?)")
		return nil, fmt.Errorf("embedded intent corpus not available: run build pipeline to generate intent_corpus.db")
	}

	// Read embedded corpus data
	data, err := defaults.IntentCorpusDB.ReadFile("intent_corpus.db")
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to read embedded corpus: %v", err)
		return nil, fmt.Errorf("failed to read embedded corpus: %w", err)
	}

	logging.StoreDebug("Embedded corpus loaded: %d bytes", len(data))

	// Create temp file for SQLite
	tempFile, err := os.CreateTemp("", "codenerd-intent-corpus-*.db")
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to create temp file for corpus: %v", err)
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	tempPath := tempFile.Name()

	// Write embedded data to temp file
	if _, err := io.Copy(tempFile, bytes.NewReader(data)); err != nil {
		tempFile.Close()
		os.Remove(tempPath)
		logging.Get(logging.CategoryStore).Error("Failed to write corpus to temp file: %v", err)
		return nil, fmt.Errorf("failed to write corpus to temp file: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		os.Remove(tempPath)
		logging.Get(logging.CategoryStore).Error("Failed to close temp file: %v", err)
		return nil, fmt.Errorf("failed to close temp file: %w", err)
	}

	logging.StoreDebug("Corpus extracted to temp file: %s", tempPath)

	// Open database in read-only mode
	dsn := fmt.Sprintf("%s?mode=ro", tempPath)
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		os.Remove(tempPath)
		logging.Get(logging.CategoryStore).Error("Failed to open corpus database: %v", err)
		return nil, fmt.Errorf("failed to open corpus database: %w", err)
	}

	// Verify database is accessible
	if err := db.Ping(); err != nil {
		db.Close()
		os.Remove(tempPath)
		logging.Get(logging.CategoryStore).Error("Failed to ping corpus database: %v", err)
		return nil, fmt.Errorf("failed to verify corpus database: %w", err)
	}

	logging.Store("Embedded corpus store initialized successfully")
	return &EmbeddedCorpusStore{
		db:       db,
		tempPath: tempPath,
	}, nil
}

// Search performs ANN search against the corpus.
// Returns top K matches with similarity scores.
func (s *EmbeddedCorpusStore) Search(queryEmbedding []float32, topK int) ([]SemanticMatch, error) {
	timer := logging.StartTimer(logging.CategoryStore, "EmbeddedCorpusStore.Search")
	defer timer.Stop()

	if topK <= 0 {
		topK = 5
	}

	logging.StoreDebug("Searching embedded corpus: topK=%d, embedding_dims=%d", topK, len(queryEmbedding))

	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.db == nil {
		return nil, fmt.Errorf("embedded corpus store not initialized")
	}

	// Encode query embedding as binary blob for sqlite-vec
	queryBlob := encodeFloat32SliceToBlob(queryEmbedding)

	// Query using sqlite-vec's vec_distance_cosine function
	// The corpus table schema:
	//   vec_corpus(embedding, text_content, predicate, verb, target, category)
	query := `
		SELECT
			text_content,
			predicate,
			verb,
			target,
			category,
			vec_distance_cosine(embedding, ?) AS distance
		FROM vec_corpus
		ORDER BY distance ASC
		LIMIT ?
	`

	rows, err := s.db.Query(query, queryBlob, topK)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Embedded corpus search failed: %v", err)
		return nil, fmt.Errorf("corpus search failed: %w", err)
	}
	defer rows.Close()

	var matches []SemanticMatch
	rank := 1
	for rows.Next() {
		var match SemanticMatch
		var distance float64

		if err := rows.Scan(
			&match.TextContent,
			&match.Predicate,
			&match.Verb,
			&match.Target,
			&match.Category,
			&distance,
		); err != nil {
			logging.Get(logging.CategoryStore).Warn("Failed to scan corpus row: %v", err)
			continue
		}

		// Convert distance to similarity (cosine distance is 1 - similarity)
		match.Similarity = 1.0 - distance
		match.Rank = rank
		rank++

		matches = append(matches, match)
	}

	if err := rows.Err(); err != nil {
		logging.Get(logging.CategoryStore).Error("Error iterating corpus results: %v", err)
		return nil, fmt.Errorf("error iterating corpus results: %w", err)
	}

	logging.StoreDebug("Embedded corpus search returned %d matches", len(matches))
	return matches, nil
}

// SearchByPredicate searches only facts of a specific predicate type.
func (s *EmbeddedCorpusStore) SearchByPredicate(queryEmbedding []float32, predicate string, topK int) ([]SemanticMatch, error) {
	timer := logging.StartTimer(logging.CategoryStore, "EmbeddedCorpusStore.SearchByPredicate")
	defer timer.Stop()

	if topK <= 0 {
		topK = 5
	}

	logging.StoreDebug("Searching embedded corpus by predicate: predicate=%s, topK=%d", predicate, topK)

	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.db == nil {
		return nil, fmt.Errorf("embedded corpus store not initialized")
	}

	queryBlob := encodeFloat32SliceToBlob(queryEmbedding)

	// Query with predicate filter
	query := `
		SELECT
			text_content,
			predicate,
			verb,
			target,
			category,
			vec_distance_cosine(embedding, ?) AS distance
		FROM vec_corpus
		WHERE predicate = ?
		ORDER BY distance ASC
		LIMIT ?
	`

	rows, err := s.db.Query(query, queryBlob, predicate, topK)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Embedded corpus predicate search failed: %v", err)
		return nil, fmt.Errorf("corpus predicate search failed: %w", err)
	}
	defer rows.Close()

	var matches []SemanticMatch
	rank := 1
	for rows.Next() {
		var match SemanticMatch
		var distance float64

		if err := rows.Scan(
			&match.TextContent,
			&match.Predicate,
			&match.Verb,
			&match.Target,
			&match.Category,
			&distance,
		); err != nil {
			logging.Get(logging.CategoryStore).Warn("Failed to scan corpus row: %v", err)
			continue
		}

		match.Similarity = 1.0 - distance
		match.Rank = rank
		rank++

		matches = append(matches, match)
	}

	if err := rows.Err(); err != nil {
		logging.Get(logging.CategoryStore).Error("Error iterating corpus predicate results: %v", err)
		return nil, fmt.Errorf("error iterating corpus predicate results: %w", err)
	}

	logging.StoreDebug("Embedded corpus predicate search returned %d matches", len(matches))
	return matches, nil
}

// SearchByCategory searches only facts of a specific category.
func (s *EmbeddedCorpusStore) SearchByCategory(queryEmbedding []float32, category string, topK int) ([]SemanticMatch, error) {
	timer := logging.StartTimer(logging.CategoryStore, "EmbeddedCorpusStore.SearchByCategory")
	defer timer.Stop()

	if topK <= 0 {
		topK = 5
	}

	logging.StoreDebug("Searching embedded corpus by category: category=%s, topK=%d", category, topK)

	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.db == nil {
		return nil, fmt.Errorf("embedded corpus store not initialized")
	}

	queryBlob := encodeFloat32SliceToBlob(queryEmbedding)

	query := `
		SELECT
			text_content,
			predicate,
			verb,
			target,
			category,
			vec_distance_cosine(embedding, ?) AS distance
		FROM vec_corpus
		WHERE category = ?
		ORDER BY distance ASC
		LIMIT ?
	`

	rows, err := s.db.Query(query, queryBlob, category, topK)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Embedded corpus category search failed: %v", err)
		return nil, fmt.Errorf("corpus category search failed: %w", err)
	}
	defer rows.Close()

	var matches []SemanticMatch
	rank := 1
	for rows.Next() {
		var match SemanticMatch
		var distance float64

		if err := rows.Scan(
			&match.TextContent,
			&match.Predicate,
			&match.Verb,
			&match.Target,
			&match.Category,
			&distance,
		); err != nil {
			logging.Get(logging.CategoryStore).Warn("Failed to scan corpus row: %v", err)
			continue
		}

		match.Similarity = 1.0 - distance
		match.Rank = rank
		rank++

		matches = append(matches, match)
	}

	if err := rows.Err(); err != nil {
		logging.Get(logging.CategoryStore).Error("Error iterating corpus category results: %v", err)
		return nil, fmt.Errorf("error iterating corpus category results: %w", err)
	}

	logging.StoreDebug("Embedded corpus category search returned %d matches", len(matches))
	return matches, nil
}

// GetStats returns statistics about the embedded corpus.
func (s *EmbeddedCorpusStore) GetStats() (map[string]interface{}, error) {
	timer := logging.StartTimer(logging.CategoryStore, "EmbeddedCorpusStore.GetStats")
	defer timer.Stop()

	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.db == nil {
		return nil, fmt.Errorf("embedded corpus store not initialized")
	}

	stats := make(map[string]interface{})

	// Total entries
	var totalEntries int64
	if err := s.db.QueryRow("SELECT COUNT(*) FROM vec_corpus").Scan(&totalEntries); err != nil {
		logging.Get(logging.CategoryStore).Warn("Failed to count corpus entries: %v", err)
	}
	stats["total_entries"] = totalEntries

	// Entries by category
	categoryRows, err := s.db.Query("SELECT category, COUNT(*) FROM vec_corpus GROUP BY category")
	if err == nil {
		categories := make(map[string]int64)
		for categoryRows.Next() {
			var cat string
			var count int64
			if err := categoryRows.Scan(&cat, &count); err == nil {
				categories[cat] = count
			}
		}
		categoryRows.Close()
		stats["by_category"] = categories
	}

	// Entries by verb
	verbRows, err := s.db.Query("SELECT verb, COUNT(*) FROM vec_corpus GROUP BY verb ORDER BY COUNT(*) DESC LIMIT 20")
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
		stats["top_verbs"] = verbs
	}

	stats["source"] = "embedded"
	stats["readonly"] = true

	logging.StoreDebug("Embedded corpus stats: total=%d", totalEntries)
	return stats, nil
}

// Close cleans up the temp file.
func (s *EmbeddedCorpusStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	logging.Store("Closing embedded corpus store")

	var closeErr error

	if s.db != nil {
		if err := s.db.Close(); err != nil {
			logging.Get(logging.CategoryStore).Error("Failed to close corpus database: %v", err)
			closeErr = err
		}
		s.db = nil
	}

	if s.tempPath != "" {
		if err := os.Remove(s.tempPath); err != nil {
			logging.Get(logging.CategoryStore).Warn("Failed to remove temp corpus file %s: %v", s.tempPath, err)
			if closeErr == nil {
				closeErr = err
			}
		} else {
			logging.StoreDebug("Removed temp corpus file: %s", s.tempPath)
		}
		s.tempPath = ""
	}

	logging.Store("Embedded corpus store closed")
	return closeErr
}

// encodeFloat32SliceToBlob encodes a float32 slice as a binary blob for sqlite-vec.
// Uses little-endian encoding as expected by sqlite-vec.
func encodeFloat32SliceToBlob(vec []float32) []byte {
	buf := &bytes.Buffer{}
	if err := binary.Write(buf, binary.LittleEndian, vec); err != nil {
		// Should never happen with bytes.Buffer
		return nil
	}
	return buf.Bytes()
}
