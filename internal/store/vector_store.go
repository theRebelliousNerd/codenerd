// Package store - Vector embedding support for LocalStore
// This file extends LocalStore with real vector embeddings using the embedding engine.
package store

import (
	"bytes"
	"codenerd/internal/embedding"
	"codenerd/internal/logging"
	"context"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"
)

// =============================================================================
// VECTOR STORE WITH REAL EMBEDDINGS
// =============================================================================

// SetEmbeddingEngine configures the embedding engine for this LocalStore.
// Must be called before StoreVectorWithEmbedding.
func (s *LocalStore) SetEmbeddingEngine(engine embedding.EmbeddingEngine) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if engine != nil {
		logging.Store("Setting embedding engine: %s (dimensions=%d)", engine.Name(), engine.Dimensions())
		s.initVecIndex(engine.Dimensions())
		s.backfillVecIndex(engine.Dimensions())
	} else {
		logging.StoreDebug("Embedding engine set to nil (keyword-only mode)")
	}
	s.embeddingEngine = engine
}

// StoreVectorWithEmbedding stores content with a real vector embedding.
// This is the new method that replaces StoreVector for semantic search.
func (s *LocalStore) StoreVectorWithEmbedding(ctx context.Context, content string, metadata map[string]interface{}) error {
	timer := logging.StartTimer(logging.CategoryStore, "StoreVectorWithEmbedding")
	defer timer.Stop()

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.embeddingEngine == nil {
		logging.StoreDebug("No embedding engine, falling back to keyword-only storage")
		return s.storeVectorKeywordOnly(content, metadata)
	}

	logging.StoreDebug("Generating embedding for content (length=%d bytes)", len(content))

	// Generate embedding
	embeddingVec, err := s.embeddingEngine.Embed(ctx, content)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to generate embedding: %v", err)
		return fmt.Errorf("failed to generate embedding: %w", err)
	}

	logging.StoreDebug("Embedding generated: %d dimensions", len(embeddingVec))

	// Serialize embedding as JSON
	embeddingJSON, err := json.Marshal(embeddingVec)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to serialize embedding: %v", err)
		return fmt.Errorf("failed to serialize embedding: %w", err)
	}

	metaJSON, _ := json.Marshal(metadata)

	_, err = s.db.Exec(
		"INSERT OR REPLACE INTO vectors (content, embedding, metadata) VALUES (?, ?, ?)",
		content, string(embeddingJSON), string(metaJSON),
	)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to store vector in SQLite: %v", err)
		return err
	}

	// If sqlite-vec is available, store in vec_index for fast ANN.
	if s.vectorExt {
		vecBlob := encodeFloat32Slice(embeddingVec)
		_, _ = s.db.Exec(
			"INSERT OR REPLACE INTO vec_index (embedding, content, metadata) VALUES (?, ?, ?)",
			vecBlob, content, string(metaJSON),
		)
		logging.StoreDebug("Vector also indexed in sqlite-vec for ANN search")
	}

	logging.StoreDebug("Vector stored successfully with embedding")
	return nil
}

// storeVectorKeywordOnly stores content without embeddings (fallback).
func (s *LocalStore) storeVectorKeywordOnly(content string, metadata map[string]interface{}) error {
	metaJSON, _ := json.Marshal(metadata)

	_, err := s.db.Exec(
		"INSERT OR REPLACE INTO vectors (content, metadata) VALUES (?, ?)",
		content, string(metaJSON),
	)
	return err
}

// VectorRecallSemantic performs true semantic search using cosine similarity.
// This is the new method that replaces VectorRecall for semantic search.
func (s *LocalStore) VectorRecallSemantic(ctx context.Context, query string, limit int) ([]VectorEntry, error) {
	timer := logging.StartTimer(logging.CategoryStore, "VectorRecallSemantic")
	defer timer.Stop()

	if limit <= 0 {
		limit = 10
	}

	logging.StoreDebug("Semantic vector recall: query=%q limit=%d", query, limit)

	s.mu.RLock()
	engine := s.embeddingEngine
	vecEnabled := s.vectorExt
	s.mu.RUnlock()

	if engine == nil {
		logging.StoreDebug("No embedding engine, falling back to keyword search")
		return s.vectorRecallKeyword(query, limit)
	}

	// Generate query embedding
	queryEmbedding, err := engine.Embed(ctx, query)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to generate query embedding: %v", err)
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	logging.StoreDebug("Query embedding generated: %d dimensions", len(queryEmbedding))

	if vecEnabled {
		logging.StoreDebug("Using sqlite-vec ANN search")
		return s.vectorRecallVec(queryEmbedding, limit, nil, "", nil)
	}

	logging.StoreDebug("Using brute-force cosine similarity search")

	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(
		"SELECT id, content, embedding, metadata, created_at FROM vectors WHERE embedding IS NOT NULL",
	)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to query vectors: %v", err)
		return nil, err
	}
	defer rows.Close()

	type candidate struct {
		entry      VectorEntry
		similarity float64
	}

	var candidates []candidate

	for rows.Next() {
		var entry VectorEntry
		var embeddingJSON, metaJSON string

		if err := rows.Scan(&entry.ID, &entry.Content, &embeddingJSON, &metaJSON, &entry.CreatedAt); err != nil {
			continue
		}

		// Deserialize embedding
		var embeddingVec []float32
		if err := json.Unmarshal([]byte(embeddingJSON), &embeddingVec); err != nil {
			continue
		}

		// Calculate cosine similarity
		similarity, err := embedding.CosineSimilarity(queryEmbedding, embeddingVec)
		if err != nil {
			continue
		}

		// Deserialize metadata
		if metaJSON != "" {
			json.Unmarshal([]byte(metaJSON), &entry.Metadata)
		}

		candidates = append(candidates, candidate{
			entry:      entry,
			similarity: similarity,
		})
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
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}

	results := make([]VectorEntry, len(candidates))
	for i, c := range candidates {
		results[i] = c.entry
		// Optionally store similarity in metadata
		if results[i].Metadata == nil {
			results[i].Metadata = make(map[string]interface{})
		}
		results[i].Metadata["similarity"] = c.similarity
	}

	logging.StoreDebug("Semantic search returned %d results (searched %d candidates)", len(results), len(candidates))
	return results, nil
}

// VectorRecallSemanticByPaths restricts search to a list of allowed paths (matched via metadata).
func (s *LocalStore) VectorRecallSemanticByPaths(ctx context.Context, query string, limit int, allowedPaths []string) ([]VectorEntry, error) {
	timer := logging.StartTimer(logging.CategoryStore, "VectorRecallSemanticByPaths")
	defer timer.Stop()

	if len(allowedPaths) == 0 {
		return s.VectorRecallSemantic(ctx, query, limit)
	}

	logging.StoreDebug("Semantic vector recall by paths: query=%q limit=%d paths=%d", query, limit, len(allowedPaths))

	s.mu.RLock()
	engine := s.embeddingEngine
	vecEnabled := s.vectorExt
	s.mu.RUnlock()

	if limit <= 0 {
		limit = 10
	}

	if engine == nil {
		logging.StoreDebug("No embedding engine, falling back to keyword search with path filter")
		all, err := s.vectorRecallKeyword(query, limit*5)
		if err != nil {
			return nil, err
		}
		filtered := filterByPaths(all, allowedPaths)
		if len(filtered) > limit {
			filtered = filtered[:limit]
		}
		logging.StoreDebug("Path-filtered keyword search returned %d results", len(filtered))
		return filtered, nil
	}

	queryEmbedding, err := engine.Embed(ctx, query)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to generate embedding for path search: %v", err)
		return nil, fmt.Errorf("failed to generate embedding: %w", err)
	}

	if vecEnabled {
		logging.StoreDebug("Using sqlite-vec ANN search with path filter")
		return s.vectorRecallVec(queryEmbedding, limit, allowedPaths, "", nil)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	queryStr, args := buildPathFilteredQuery(allowedPaths)
	rows, err := s.db.Query(queryStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type candidate struct {
		entry      VectorEntry
		similarity float64
	}
	candidates := make([]candidate, 0, limit*2)

	for rows.Next() {
		var entry VectorEntry
		var embeddingJSON, metaJSON string

		if err := rows.Scan(&entry.ID, &entry.Content, &embeddingJSON, &metaJSON, &entry.CreatedAt); err != nil {
			continue
		}

		if metaJSON != "" {
			json.Unmarshal([]byte(metaJSON), &entry.Metadata)
		}

		var embeddingVec []float32
		if err := json.Unmarshal([]byte(embeddingJSON), &embeddingVec); err != nil {
			continue
		}

		similarity, err := embedding.CosineSimilarity(queryEmbedding, embeddingVec)
		if err != nil {
			continue
		}

		candidates = append(candidates, candidate{
			entry:      entry,
			similarity: similarity,
		})
	}

	for i := 0; i < len(candidates); i++ {
		for j := i + 1; j < len(candidates); j++ {
			if candidates[j].similarity > candidates[i].similarity {
				candidates[i], candidates[j] = candidates[j], candidates[i]
			}
		}
	}

	if len(candidates) > limit {
		candidates = candidates[:limit]
	}

	results := make([]VectorEntry, len(candidates))
	for i, c := range candidates {
		results[i] = c.entry
		if results[i].Metadata == nil {
			results[i].Metadata = make(map[string]interface{})
		}
		results[i].Metadata["similarity"] = c.similarity
	}

	return results, nil
}

// VectorRecallSemanticFiltered restricts search to entries whose metadata contain a key/value pair.
// This reduces scanning cost when the store contains vectors from many campaigns.
func (s *LocalStore) VectorRecallSemanticFiltered(ctx context.Context, query string, limit int, metaKey string, metaValue interface{}) ([]VectorEntry, error) {
	timer := logging.StartTimer(logging.CategoryStore, "VectorRecallSemanticFiltered")
	defer timer.Stop()

	if limit <= 0 {
		limit = 10
	}

	logging.StoreDebug("Semantic vector recall with filter: query=%q limit=%d filter=%s=%v", query, limit, metaKey, metaValue)

	s.mu.RLock()
	engine := s.embeddingEngine
	vecEnabled := s.vectorExt
	s.mu.RUnlock()

	if engine == nil {
		logging.StoreDebug("No embedding engine, falling back to keyword search with metadata filter")
		all, err := s.vectorRecallKeyword(query, limit*5)
		if err != nil {
			return nil, err
		}
		filtered := make([]VectorEntry, 0, len(all))
		for _, e := range all {
			if matchesMetadata(e.Metadata, metaKey, metaValue) {
				filtered = append(filtered, e)
			}
		}
		if len(filtered) > limit {
			filtered = filtered[:limit]
		}
		logging.StoreDebug("Metadata-filtered keyword search returned %d results", len(filtered))
		return filtered, nil
	}

	queryEmbedding, err := engine.Embed(ctx, query)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to generate embedding for filtered search: %v", err)
		return nil, fmt.Errorf("failed to generate embedding: %w", err)
	}

	if vecEnabled {
		logging.StoreDebug("Using sqlite-vec ANN search with metadata filter")
		return s.vectorRecallVec(queryEmbedding, limit, nil, metaKey, metaValue)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	queryStr := "SELECT id, content, embedding, metadata, created_at FROM vectors WHERE embedding IS NOT NULL"
	var rows *sql.Rows
	if metaKey != "" && metaValue != nil {
		pattern := fmt.Sprintf("%%\"%s\":\"%v\"%%", metaKey, metaValue)
		rows, err = s.db.Query(queryStr+" AND metadata LIKE ?", pattern)
	} else {
		rows, err = s.db.Query(queryStr)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type candidate struct {
		entry      VectorEntry
		similarity float64
	}

	candidates := make([]candidate, 0, limit*2)

	for rows.Next() {
		var entry VectorEntry
		var embeddingJSON, metaJSON string

		if err := rows.Scan(&entry.ID, &entry.Content, &embeddingJSON, &metaJSON, &entry.CreatedAt); err != nil {
			continue
		}

		if metaJSON != "" {
			json.Unmarshal([]byte(metaJSON), &entry.Metadata)
		}
		if !matchesMetadata(entry.Metadata, metaKey, metaValue) {
			continue
		}

		var embeddingVec []float32
		if err := json.Unmarshal([]byte(embeddingJSON), &embeddingVec); err != nil {
			continue
		}

		similarity, err := embedding.CosineSimilarity(queryEmbedding, embeddingVec)
		if err != nil {
			continue
		}

		candidates = append(candidates, candidate{
			entry:      entry,
			similarity: similarity,
		})
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
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}

	results := make([]VectorEntry, len(candidates))
	for i, c := range candidates {
		results[i] = c.entry
		if results[i].Metadata == nil {
			results[i].Metadata = make(map[string]interface{})
		}
		results[i].Metadata["similarity"] = c.similarity
	}

	return results, nil
}

// vectorRecallKeyword is the fallback keyword-based search.
func (s *LocalStore) vectorRecallKeyword(query string, limit int) ([]VectorEntry, error) {
	// This is the old implementation from local.go VectorRecall
	// Kept for backward compatibility when no embedding engine is set
	return s.VectorRecall(query, limit)
}

func matchesMetadata(meta map[string]interface{}, key string, value interface{}) bool {
	if key == "" {
		return true
	}
	if meta == nil {
		return false
	}
	if v, ok := meta[key]; ok {
		return fmt.Sprintf("%v", v) == fmt.Sprintf("%v", value)
	}
	return false
}

func buildPathFilteredQuery(paths []string) (string, []interface{}) {
	base := "SELECT id, content, embedding, metadata, created_at FROM vectors WHERE embedding IS NOT NULL"
	if len(paths) == 0 {
		return base, nil
	}
	var sb strings.Builder
	sb.WriteString(base)
	sb.WriteString(" AND (")
	args := make([]interface{}, 0, len(paths))
	for i, p := range paths {
		if i > 0 {
			sb.WriteString(" OR ")
		}
		sb.WriteString("metadata LIKE ?")
		args = append(args, fmt.Sprintf("%%\"path\":\"%s\"%%", p))
	}
	sb.WriteString(")")
	return sb.String(), args
}

func filterByPaths(entries []VectorEntry, paths []string) []VectorEntry {
	pathSet := make(map[string]struct{}, len(paths))
	for _, p := range paths {
		pathSet[p] = struct{}{}
	}
	out := make([]VectorEntry, 0, len(entries))
	for _, e := range entries {
		p := ""
		if e.Metadata != nil {
			if v, ok := e.Metadata["path"].(string); ok {
				p = v
			}
		}
		if _, ok := pathSet[p]; ok {
			out = append(out, e)
		}
	}
	return out
}

// vectorRecallVec performs ANN search via sqlite-vec when available.
func (s *LocalStore) vectorRecallVec(queryVec []float32, limit int, allowedPaths []string, metaKey string, metaValue interface{}) ([]VectorEntry, error) {
	timer := logging.StartTimer(logging.CategoryStore, "vectorRecallVec")
	defer timer.Stop()

	if !s.vectorExt {
		logging.Get(logging.CategoryStore).Error("sqlite-vec not enabled for ANN search")
		return nil, fmt.Errorf("sqlite-vec not enabled")
	}
	if limit <= 0 {
		limit = 10
	}

	logging.StoreDebug("sqlite-vec ANN search: limit=%d, paths=%d, metaFilter=%s", limit, len(allowedPaths), metaKey)

	queryBlob := encodeFloat32Slice(queryVec)

	where := make([]string, 0)
	args := make([]interface{}, 0)

	// Path filters
	if len(allowedPaths) > 0 {
		clause := make([]string, 0, len(allowedPaths))
		for _, p := range allowedPaths {
			clause = append(clause, "metadata LIKE ?")
			args = append(args, fmt.Sprintf("%%\"path\":\"%s\"%%", p))
		}
		where = append(where, "("+strings.Join(clause, " OR ")+")")
	}

	if metaKey != "" && metaValue != nil {
		where = append(where, "metadata LIKE ?")
		args = append(args, fmt.Sprintf("%%\"%s\":\"%v\"%%", metaKey, metaValue))
	}

	sqlStr := "SELECT rowid, content, metadata, vec_distance_cosine(embedding, ?) AS dist FROM vec_index"
	args = append([]interface{}{queryBlob}, args...)
	if len(where) > 0 {
		sqlStr += " WHERE " + strings.Join(where, " AND ")
	}
	sqlStr += " ORDER BY dist ASC LIMIT ?"
	args = append(args, limit)

	s.mu.RLock()
	rows, err := s.db.Query(sqlStr, args...)
	s.mu.RUnlock()
	if err != nil {
		logging.Get(logging.CategoryStore).Error("sqlite-vec query failed: %v", err)
		return nil, err
	}
	defer rows.Close()

	results := make([]VectorEntry, 0, limit)
	for rows.Next() {
		var id int64
		var content, metaJSON string
		var dist float64
		if err := rows.Scan(&id, &content, &metaJSON, &dist); err != nil {
			continue
		}
		entry := VectorEntry{
			ID:        id,
			Content:   content,
			CreatedAt: time.Now(),
			Metadata:  make(map[string]interface{}),
		}
		if metaJSON != "" {
			json.Unmarshal([]byte(metaJSON), &entry.Metadata)
		}
		entry.Metadata["similarity"] = 1 - dist
		results = append(results, entry)
	}

	logging.StoreDebug("sqlite-vec ANN search returned %d results", len(results))
	return results, nil
}

// initVecIndex attempts to create a sqlite-vec table; if it succeeds, vectorExt is enabled.
func (s *LocalStore) initVecIndex(dim int) {
	if dim <= 0 || s.db == nil {
		return
	}
	logging.StoreDebug("Initializing sqlite-vec index with %d dimensions", dim)
	stmt := fmt.Sprintf("CREATE VIRTUAL TABLE IF NOT EXISTS vec_index USING vec0(embedding float[%d], content TEXT, metadata TEXT)", dim)
	if _, err := s.db.Exec(stmt); err == nil {
		s.vectorExt = true
		logging.Store("sqlite-vec index initialized successfully (dimensions=%d)", dim)
	} else {
		logging.Get(logging.CategoryStore).Warn("Failed to create sqlite-vec index: %v", err)
	}
}

func encodeFloat32Slice(vec []float32) []byte {
	buf := &bytes.Buffer{}
	_ = binary.Write(buf, binary.LittleEndian, vec)
	return buf.Bytes()
}

// backfillVecIndex migrates existing JSON-stored embeddings into sqlite-vec.
func (s *LocalStore) backfillVecIndex(dim int) {
	if !s.vectorExt || s.db == nil || dim <= 0 {
		return
	}

	logging.StoreDebug("Starting backfill of existing embeddings into sqlite-vec index")

	rows, err := s.db.Query("SELECT content, embedding, metadata FROM vectors WHERE embedding IS NOT NULL")
	if err != nil {
		logging.Get(logging.CategoryStore).Warn("Failed to query embeddings for backfill: %v", err)
		return
	}
	defer rows.Close()

	backfillCount := 0
	skippedCount := 0

	for rows.Next() {
		var content, embeddingJSON, metaJSON string
		if err := rows.Scan(&content, &embeddingJSON, &metaJSON); err != nil {
			skippedCount++
			continue
		}
		var embeddingVec []float32
		if err := json.Unmarshal([]byte(embeddingJSON), &embeddingVec); err != nil {
			skippedCount++
			continue
		}
		if len(embeddingVec) != dim {
			skippedCount++
			continue
		}
		vecBlob := encodeFloat32Slice(embeddingVec)
		_, err := s.db.Exec(
			"INSERT OR REPLACE INTO vec_index (embedding, content, metadata) VALUES (?, ?, ?)",
			vecBlob, content, metaJSON,
		)
		if err == nil {
			backfillCount++
		} else {
			skippedCount++
		}
	}

	if backfillCount > 0 || skippedCount > 0 {
		logging.Store("Backfill complete: migrated=%d, skipped=%d", backfillCount, skippedCount)
	}
}

// GetVectorStats returns statistics about stored vectors.
func (s *LocalStore) GetVectorStats() (map[string]interface{}, error) {
	timer := logging.StartTimer(logging.CategoryStore, "GetVectorStats")
	defer timer.Stop()

	s.mu.RLock()
	defer s.mu.RUnlock()

	logging.StoreDebug("Computing vector store statistics")

	stats := make(map[string]interface{})

	var totalVectors int64
	s.db.QueryRow("SELECT COUNT(*) FROM vectors").Scan(&totalVectors)
	stats["total_vectors"] = totalVectors

	var withEmbeddings int64
	s.db.QueryRow("SELECT COUNT(*) FROM vectors WHERE embedding IS NOT NULL").Scan(&withEmbeddings)
	stats["with_embeddings"] = withEmbeddings

	var withoutEmbeddings int64
	withoutEmbeddings = totalVectors - withEmbeddings
	stats["without_embeddings"] = withoutEmbeddings

	if s.embeddingEngine != nil {
		stats["embedding_engine"] = s.embeddingEngine.Name()
		stats["embedding_dimensions"] = s.embeddingEngine.Dimensions()
	} else {
		stats["embedding_engine"] = "none (keyword search)"
	}

	logging.StoreDebug("Vector stats: total=%d, with_embeddings=%d, engine=%v", totalVectors, withEmbeddings, stats["embedding_engine"])
	return stats, nil
}

// ReembedAllVectors regenerates embeddings for all vectors that don't have them.
// Useful for migrating from keyword-only to embedding-based search.
func (s *LocalStore) ReembedAllVectors(ctx context.Context) error {
	timer := logging.StartTimer(logging.CategoryStore, "ReembedAllVectors")
	defer timer.Stop()

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.embeddingEngine == nil {
		logging.Get(logging.CategoryStore).Error("Cannot re-embed: no embedding engine configured")
		return fmt.Errorf("no embedding engine configured")
	}

	logging.Store("Starting re-embedding of all vectors without embeddings")

	// Fetch all vectors without embeddings
	rows, err := s.db.Query("SELECT id, content FROM vectors WHERE embedding IS NULL")
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to query vectors for re-embedding: %v", err)
		return err
	}
	defer rows.Close()

	type vectorToEmbed struct {
		id      int64
		content string
	}

	var vectors []vectorToEmbed
	for rows.Next() {
		var v vectorToEmbed
		if err := rows.Scan(&v.id, &v.content); err != nil {
			continue
		}
		vectors = append(vectors, v)
	}

	if len(vectors) == 0 {
		logging.StoreDebug("No vectors need re-embedding")
		return nil
	}

	logging.Store("Found %d vectors to re-embed", len(vectors))

	// Generate embeddings in batches
	batchSize := 32
	totalEmbedded := 0
	for i := 0; i < len(vectors); i += batchSize {
		end := int(math.Min(float64(i+batchSize), float64(len(vectors))))
		batch := vectors[i:end]

		logging.StoreDebug("Processing batch %d-%d of %d", i, end, len(vectors))

		// Collect texts
		texts := make([]string, len(batch))
		for j, v := range batch {
			texts[j] = v.content
		}

		// Generate embeddings
		embeddings, err := s.embeddingEngine.EmbedBatch(ctx, texts)
		if err != nil {
			logging.Get(logging.CategoryStore).Error("Failed to generate batch embeddings: %v", err)
			return fmt.Errorf("failed to generate batch embeddings: %w", err)
		}

		// Update database
		for j, v := range batch {
			embeddingJSON, _ := json.Marshal(embeddings[j])
			_, err := s.db.Exec(
				"UPDATE vectors SET embedding = ? WHERE id = ?",
				string(embeddingJSON), v.id,
			)
			if err != nil {
				logging.Get(logging.CategoryStore).Error("Failed to update vector %d: %v", v.id, err)
				return fmt.Errorf("failed to update vector %d: %w", v.id, err)
			}
			totalEmbedded++
		}
	}

	logging.Store("Re-embedding complete: %d vectors processed", totalEmbedded)
	return nil
}
