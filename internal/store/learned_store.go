// Package store - Learned corpus store for dynamic intent patterns.
// This file implements a writable store for learned patterns that are
// discovered during runtime through user interactions and feedback.
package store

import (
	"bytes"
	"codenerd/internal/embedding"
	"codenerd/internal/logging"
	"codenerd/internal/sqlpragmas"
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
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
	return NewLearnedCorpusStoreWithContext(context.Background(), dbPath, engine)
}

// NewLearnedCorpusStoreWithContext admits and initializes the SQL backend with
// the caller's context. Failed admission drains SQL and closes owned handles.
func NewLearnedCorpusStoreWithContext(ctx context.Context, dbPath string, engine embedding.EmbeddingEngine) (*LearnedCorpusStore, error) {
	return newLearnedCorpusStoreWithDriver(ctx, dbPath, engine, nil)
}

func newLearnedCorpusStoreWithDriver(ctx context.Context, dbPath string, engine embedding.EmbeddingEngine, backendDriver driver.Driver) (corpus *LearnedCorpusStore, outcome error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	timer := logging.StartTimer(logging.CategoryStore, "NewLearnedCorpusStore")
	defer timer.Stop()
	var database *sql.DB
	defer func() {
		outcome = errors.Join(outcome, ctx.Err())
		if outcome != nil {
			if database != nil {
				outcome = errors.Join(outcome, database.Close())
			}
			outcome = errors.Join(outcome, ctx.Err())
			corpus = nil
		}
	}()

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

	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if backendDriver == nil {
		probe, err := sql.Open("sqlite3", dbPath)
		if err != nil {
			return nil, fmt.Errorf("failed to open database driver: %w", err)
		}
		backendDriver = probe.Driver()
		if err := probe.Close(); err != nil {
			return nil, fmt.Errorf("failed to close database driver probe: %w", err)
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	connector, err := sqlpragmas.NewConnector(learnedAdmissionDriver{Driver: backendDriver}, dbPath, ProfileHot)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connector: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	database = sql.OpenDB(learnedAdmissionConnector{Connector: connector})

	// Verify connection
	if err := database.PingContext(ctx); err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to ping learned corpus database: %v", err)
		return nil, fmt.Errorf("failed to verify database connection: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	store := &LearnedCorpusStore{
		db:          database,
		embedEngine: engine,
		dbPath:      dbPath,
	}

	// Initialize schema
	if err := store.initializeSchemaWithContext(ctx); err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to initialize learned corpus schema: %v", err)
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}
	logging.Store("Learned corpus store initialized successfully")
	return store, nil
}

type learnedAdmissionConnector struct {
	driver.Connector
}

type learnedAdmissionDriver struct {
	driver.Driver
}

func (backend learnedAdmissionDriver) OpenConnector(path string) (driver.Connector, error) {
	var connector driver.Connector
	if contextual, ok := backend.Driver.(driver.DriverContext); ok {
		var err error
		connector, err = contextual.OpenConnector(path)
		if err != nil {
			return nil, err
		}
	} else {
		connector = learnedDSNConnector{backend: backend.Driver, path: path}
	}
	return learnedAdmissionConnector{Connector: connector}, nil
}

type learnedDSNConnector struct {
	backend driver.Driver
	path    string
}

func (connector learnedDSNConnector) Driver() driver.Driver { return connector.backend }

func (connector learnedDSNConnector) Connect(ctx context.Context) (driver.Conn, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return connector.backend.Open(connector.path)
}

func (connector learnedAdmissionConnector) Connect(ctx context.Context) (driver.Conn, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	connection, err := connector.Connector.Connect(ctx)
	if admissionErr := errors.Join(err, ctx.Err()); admissionErr != nil {
		if connection != nil {
			admissionErr = errors.Join(admissionErr, connection.Close())
		}
		return nil, errors.Join(admissionErr, ctx.Err())
	}
	return connection, nil
}

func (s *LearnedCorpusStore) initializeSchemaWithContext(ctx context.Context) (outcome error) {
	timer := logging.StartTimer(logging.CategoryStore, "LearnedCorpusStore.initializeSchema")
	defer timer.Stop()
	if err := ctx.Err(); err != nil {
		return err
	}
	connection, err := s.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer func() { outcome = errors.Join(outcome, connection.Close()) }()
	if _, err := connection.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return errors.Join(err, ctx.Err())
	}
	transactionOpen := true
	defer func() {
		if transactionOpen {
			// Rollback stays on the constructor's call stack and must complete
			// even after the caller cancels the schema operation.
			_, rollbackErr := connection.ExecContext(context.WithoutCancel(ctx), "ROLLBACK")
			outcome = errors.Join(outcome, rollbackErr)
		}
	}()

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

	if _, err := connection.ExecContext(ctx, patternsTable); err != nil {
		return fmt.Errorf("failed to create patterns table: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	// Create sqlite-vec virtual table for ANN search only if engine is provided.
	// Otherwise, it will be deferred to SetEmbeddingEngine.
	if s.embedEngine != nil {
		dims := s.embedEngine.Dimensions()
		logging.StoreDebug("Initializing learned store vec_learned with vector dimensions: %d", dims)

		// Drop first to ensure dimension enforcement if user changed models
		_, dropErr := connection.ExecContext(ctx, "DROP TABLE IF EXISTS vec_learned")
		if err := ctx.Err(); err != nil {
			return errors.Join(err, dropErr)
		}

		vecTable := fmt.Sprintf(`
		CREATE VIRTUAL TABLE vec_learned USING vec0(
			embedding float[%d],
			pattern TEXT,
			verb TEXT
		);
		`, dims)

		if _, err := connection.ExecContext(ctx, vecTable); err != nil {
			if ctx.Err() != nil {
				return errors.Join(err, ctx.Err())
			}
			// Log warning but don't fail - vec extension might not be available
			logging.Get(logging.CategoryStore).Warn("Failed to create vec_learned table (sqlite-vec may not be available): %v", err)
		} else {
			logging.StoreDebug("sqlite-vec table created with %d dimensions", dims)
			// The drop above wiped the ANN index; restore it from the durable
			// table or every previously learned pattern is unsearchable
			// until re-added. See backfillVecLearnedWithContext.
			if err := s.backfillVecLearnedWithContext(ctx, connection); err != nil {
				return err
			}
		}
	} else {
		logging.StoreDebug("Skipping vec_learned creation during init (no embedding engine provided, deferred to SetEmbeddingEngine)")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := connection.ExecContext(ctx, "COMMIT"); err != nil {
		return errors.Join(err, ctx.Err())
	}
	transactionOpen = false

	logging.StoreDebug("Learned corpus schema initialized")
	return nil
}

// backfillVecLearnedWithContext restores the ANN index from learned_patterns after a
// drop-and-recreate. Rows are inserted one at a time: a pattern whose
// embedding no longer matches the recreated dimensions (model switch) fails
// its own insert and is skipped, not the whole backfill.
func (s *LearnedCorpusStore) backfillVecLearnedWithContext(ctx context.Context, connection *sql.Conn) error {
	rows, err := connection.QueryContext(ctx, "SELECT embedding, pattern, verb FROM learned_patterns")
	if err != nil {
		if ctx.Err() != nil {
			return errors.Join(err, ctx.Err())
		}
		logging.Get(logging.CategoryStore).Warn("vec_learned backfill query failed: %v", err)
		return nil
	}
	type learnedVectorRow struct {
		blob    []byte
		pattern string
		verb    string
	}
	var entries []learnedVectorRow
	backfilled, skipped := 0, 0
	for rows.Next() {
		var blob []byte
		var pattern, verb string
		if err := rows.Scan(&blob, &pattern, &verb); err != nil {
			skipped++
			continue
		}
		entries = append(entries, learnedVectorRow{blob: blob, pattern: pattern, verb: verb})
	}
	iterationErr := rows.Err()
	closeErr := rows.Close()
	if err := errors.Join(ctx.Err(), closeErr); err != nil {
		return errors.Join(err, iterationErr)
	}
	if iterationErr != nil {
		logging.Get(logging.CategoryStore).Warn("vec_learned backfill iteration failed: %v", iterationErr)
	}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		if _, err := connection.ExecContext(ctx, "INSERT INTO vec_learned (embedding, pattern, verb) VALUES (?, ?, ?)", entry.blob, entry.pattern, entry.verb); err != nil {
			if ctx.Err() != nil {
				return errors.Join(err, ctx.Err())
			}
			skipped++
			continue
		}
		backfilled++
	}
	if backfilled > 0 || skipped > 0 {
		logging.Store("vec_learned backfilled: %d patterns restored, %d skipped", backfilled, skipped)
	}
	return ctx.Err()
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

	// Snapshot the engine under a read lock, then embed WITHOUT holding the
	// write lock: embedding is a network call, and holding mu across it
	// serialized every reader behind every AddPattern.
	s.mu.RLock()
	engine := s.embedEngine
	s.mu.RUnlock()
	if engine == nil {
		return fmt.Errorf("embedding engine not configured")
	}

	// Generate embedding for the pattern
	taskType := embedding.SelectTaskType(embedding.ContentTypeKnowledgeAtom, false)
	var embeddingVec []float32
	var err error
	if taskAware, ok := engine.(embedding.TaskTypeAwareEngine); ok && taskType != "" {
		embeddingVec, err = taskAware.EmbedWithTask(ctx, pattern, taskType)
	} else {
		embeddingVec, err = engine.Embed(ctx, pattern)
	}
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to generate embedding for pattern: %v", err)
		return fmt.Errorf("failed to generate embedding: %w", err)
	}

	logging.StoreDebug("Generated embedding: %d dimensions", len(embeddingVec))

	s.mu.Lock()
	defer s.mu.Unlock()

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
		// No ANN index (extension missing, or creation deferred for lack of
		// an engine) falls back to brute force over the durable table —
		// the learned corpus is small, and erroring every search when the
		// index is absent made the whole classifier fail over nothing.
		// A failed query against an EXISTING index is a real error.
		if tableExists(s.db, "vec_learned") {
			logging.Get(logging.CategoryStore).Error("vec search failed: %v", err)
			return nil, fmt.Errorf("ANN search failed: %w", err)
		}
		logging.StoreDebug("vec_learned absent, brute-forcing learned search over %d dims", len(queryEmbedding))
		return s.searchBruteForce(queryEmbedding, topK)
	}

	logging.StoreDebug("Learned corpus search returned %d matches", len(matches))
	return matches, nil
}

// searchBruteForce scores every confident pattern in Go when the ANN index
// is absent. Same contract as searchVec: confidence > 0.3, cosine order,
// topK cap, 1-based ranks.
func (s *LearnedCorpusStore) searchBruteForce(query []float32, topK int) ([]SemanticMatch, error) {
	rows, err := s.db.Query(`SELECT pattern, verb, target, embedding FROM learned_patterns WHERE confidence > 0.3`)
	if err != nil {
		return nil, fmt.Errorf("brute-force learned search failed: %w", err)
	}
	defer rows.Close()

	var matches []SemanticMatch
	for rows.Next() {
		var pattern, verb string
		var target sql.NullString
		var blob []byte
		if err := rows.Scan(&pattern, &verb, &target, &blob); err != nil {
			continue
		}
		vec := decodeFloat32Blob(blob)
		if len(vec) == 0 || len(vec) != len(query) {
			continue
		}
		matches = append(matches, SemanticMatch{
			TextContent: pattern,
			Predicate:   "learned_intent",
			Verb:        verb,
			Target:      target.String,
			Category:    "learned",
			Similarity:  float32Cosine(query, vec),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("brute-force learned search failed: %w", err)
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].Similarity > matches[j].Similarity })
	if len(matches) > topK {
		matches = matches[:topK]
	}
	for i := range matches {
		matches[i].Rank = i + 1
	}
	return matches, nil
}

// decodeFloat32Blob reverses encodeFloat32SliceToBlob. Malformed blobs decode
// to nil so the caller skips the row instead of scoring garbage.
func decodeFloat32Blob(blob []byte) []float32 {
	if len(blob) == 0 || len(blob)%4 != 0 {
		return nil
	}
	out := make([]float32, 0, len(blob)/4)
	for i := 0; i+4 <= len(blob); i += 4 {
		out = append(out, math.Float32frombits(binary.LittleEndian.Uint32(blob[i:i+4])))
	}
	return out
}

// float32Cosine is cosine similarity over float32 vectors. Mismatched or
// zero vectors score 0 rather than NaN.
func float32Cosine(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
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
