// Package store - Vector embedding support for LocalStore
// This file extends LocalStore with real vector embeddings using the embedding engine.
package store

import (
	"bytes"
	"codenerd/internal/embedding"
	"codenerd/internal/logging"
	"codenerd/internal/types"
	"context"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// =============================================================================
// TYPES
// =============================================================================

// VECTOR STORE WITH REAL EMBEDDINGS
// =============================================================================

// SetEmbeddingEngine configures the embedding engine for this LocalStore.
// Must be called before StoreVectorWithEmbedding.
func (s *LocalStore) SetEmbeddingEngine(engine embedding.EmbeddingEngine) {
	if s == nil {
		return
	}

	var startReflection bool
	var stopReflection bool

	s.mu.Lock()
	if engine != nil {
		logging.Store("Setting embedding engine: %s (dimensions=%d)", engine.Name(), engine.Dimensions())
		s.initVecIndex(engine.Dimensions())
		// Run backfill in background to avoid blocking startup: the
		// sqlite-vec INSERT can be very slow (minutes). The backfill takes
		// no store lock — it runs in short per-batch transactions, and
		// holding s.mu for the whole run blocked every reader for minutes.
		// Ordering with force re-embed is explicit instead: the re-embed
		// loops wait for the pending backfill (see waitForVecBackfill),
		// otherwise the two writers race on the same vec_index rows.
		dim := engine.Dimensions()
		// The index holds the attached engine's space and nothing else, so
		// the backfill is told which space that is (see vector_identity.go).
		space := embeddingSpace(engine)
		// Snapshot the vec flag with this generation: the background job must
		// never read s.vectorExt itself (that unlocked read raced the test
		// and init writers). The done channel below is the generation token.
		vecExt := s.vectorExt.Load()
		done := make(chan struct{})
		s.backfillMu.Lock()
		s.backfillDone = done
		s.backfillMu.Unlock()
		logging.Store("Spawning background goroutine for vector index backfill")
		go func() {
			defer close(done)
			if hold := backfillStartHold; hold != nil {
				<-hold
			}
			logging.Store("Background vector index backfill starting (dim=%d)", dim)
			s.backfillVecIndex(dim, space, vecExt, done)
			logging.Store("Background vector index backfill completed")
			s.vecIndexRev.Add(1)
			s.backfillMu.Lock()
			if s.backfillDone == done {
				s.backfillDone = nil
			}
			s.backfillMu.Unlock()
		}()
		if s.reflectionCfg != nil && s.reflectionCfg.Enabled {
			startReflection = true
		}
	} else {
		logging.StoreDebug("Embedding engine set to nil (keyword-only mode)")
		stopReflection = true
	}
	s.embeddingEngine = engine
	s.mu.Unlock()

	if startReflection {
		s.startReflectionWorker()
	} else if stopReflection {
		s.stopReflectionWorker()
	}
}

// backfillStartHold, when set, holds each backfill goroutine before it starts,
// so a test can inspect a generation that is still pending: on an empty store
// the backfill otherwise finishes, and clears its token, before the caller
// looks. Nil in production.
var backfillStartHold chan struct{}

// waitForVecBackfill blocks until no background vec backfill is pending. The
// wait loops because a second SetEmbeddingEngine can start a new backfill
// while the caller waits on the previous one's channel. Force re-embed loops
// must call this before rewriting vec_index rows; ordinary stores address
// fresh rowids and need no ordering. Call without holding s.mu.
func (s *LocalStore) waitForVecBackfill(ctx context.Context) error {
	for {
		s.backfillMu.Lock()
		done := s.backfillDone
		s.backfillMu.Unlock()
		if done == nil {
			return nil
		}
		select {
		case <-done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// isCurrentBackfill reports whether done is still the pending backfill
// generation. A second SetEmbeddingEngine drops and recreates vec_index; a
// superseded backfill must stop inserting instead of writing into the new
// table (duplicates) or failing noisily batch after batch.
func (s *LocalStore) isCurrentBackfill(done chan struct{}) bool {
	s.backfillMu.Lock()
	defer s.backfillMu.Unlock()
	return s.backfillDone != nil && s.backfillDone == done
}

// StoreVectorWithEmbedding stores content with a real vector embedding.
// This is the new method that replaces StoreVector for semantic search.
func (s *LocalStore) StoreVectorWithEmbedding(ctx context.Context, content string, metadata map[string]any) error {
	timer := logging.StartTimer(logging.CategoryStore, "StoreVectorWithEmbedding")
	defer timer.Stop()

	s.mu.RLock()
	engine := s.embeddingEngine
	s.mu.RUnlock()

	if engine == nil {
		logging.StoreDebug("No embedding engine, falling back to keyword-only storage")
		s.mu.Lock()
		defer s.mu.Unlock()
		return s.storeVectorKeywordOnly(content, metadata)
	}

	logging.StoreDebug("Generating embedding for content (length=%d bytes)", len(content))

	// Generate embedding outside the write lock: it is a network call, and
	// holding mu across it serialized every reader behind every store.
	taskType := embedding.GetOptimalTaskType(content, metadata, false)
	var embeddingVec []float32
	var err error
	if taskAware, ok := engine.(embedding.TaskTypeAwareEngine); ok && taskType != "" {
		embeddingVec, err = taskAware.EmbedWithTask(ctx, content, taskType)
	} else {
		embeddingVec, err = engine.Embed(ctx, content)
	}
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to generate embedding: %v", err)
		return fmt.Errorf("failed to generate embedding: %w", err)
	}

	logging.StoreDebug("Embedding generated: %d dimensions", len(embeddingVec))

	// The stamp names the engine that produced these bytes: the one captured
	// above, not whichever engine is attached when the lock is finally taken.
	space, err := requireEmbeddingSpace(engine)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Serialize embedding as JSON
	embeddingJSON, err := json.Marshal(embeddingVec)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to serialize embedding: %v", err)
		return fmt.Errorf("failed to serialize embedding: %w", err)
	}

	metaJSON, err := encodeRowMetadata(metadata)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Refusing to store vector: %v", err)
		return err
	}

	res, err := s.db.Exec(
		"INSERT OR REPLACE INTO vectors (content, embedding, embedding_model, metadata) VALUES (?, ?, ?, ?)",
		content, string(embeddingJSON), space, metaJSON,
	)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to store vector in SQLite: %v", err)
		return err
	}

	// If sqlite-vec is available, store in vec_index for fast ANN.
	// Errors here MUST be checked: silently dropping the vec_index insert
	// produces the "ANN drift" pathology — `vectors` has the row but
	// vec_index doesn't, so ANN search returns nothing while brute-force
	// returns the result. The earlier `_, _ = ...` swallowed both
	// LastInsertId failures and vec_index Exec failures.
	//
	// Only a vector in the attached engine's space enters the index: an engine
	// replaced during the provider call leaves its row stored and stamped, but
	// the index belongs to the new space.
	if s.vectorExt.Load() && s.currentSpaceLocked() != space {
		logging.Get(logging.CategoryStore).Warn("embedding engine changed while embedding: vector stored in space %q and not indexed (current space %q)", space, s.currentSpaceLocked())
	} else if s.vectorExt.Load() {
		id, lidErr := res.LastInsertId()
		if lidErr != nil {
			logging.Get(logging.CategoryStore).Warn("vec_index skipped: LastInsertId failed: %v (vectors row %s persists; ANN index drift)", lidErr, content)
		} else if vecErr := s.insertVecIndexRow(id, embeddingVec, content, string(metaJSON)); vecErr != nil {
			logging.Get(logging.CategoryStore).Warn("vec_index insert failed for rowid=%d: %v (ANN drift)", id, vecErr)
		} else {
			logging.StoreDebug("Vector also indexed in sqlite-vec for ANN search")
		}
	}

	logging.StoreDebug("Vector stored successfully with embedding")
	return nil
}

// StoreVectorBatchWithEmbedding stores a batch of entries with embeddings.
// Falls back to keyword-only storage when no embedding engine is configured.
func (s *LocalStore) StoreVectorBatchWithEmbedding(ctx context.Context, contents []string, metadata []map[string]any) (int, error) {
	timer := logging.StartTimer(logging.CategoryStore, "StoreVectorBatchWithEmbedding")
	defer timer.Stop()

	if len(contents) == 0 {
		return 0, nil
	}
	if len(contents) != len(metadata) {
		return 0, fmt.Errorf("contents/metadata length mismatch: %d != %d", len(contents), len(metadata))
	}

	s.mu.RLock()
	engine := s.embeddingEngine
	vecEnabled := s.vectorExt.Load()
	s.mu.RUnlock()

	if engine == nil {
		return s.storeVectorBatchKeywordOnly(contents, metadata)
	}

	taskTypes := make([]string, len(contents))
	uniformTask := true
	for i, content := range contents {
		taskTypes[i] = embedding.GetOptimalTaskType(content, metadata[i], false)
		if i > 0 && taskTypes[i] != taskTypes[0] {
			uniformTask = false
		}
	}

	var embeddings [][]float32
	var err error
	if uniformTask && taskTypes[0] != "" {
		if batchAware, ok := engine.(embedding.TaskTypeBatchAwareEngine); ok {
			embeddings, err = batchAware.EmbedBatchWithTask(ctx, contents, taskTypes[0])
		} else if taskAware, ok := engine.(embedding.TaskTypeAwareEngine); ok {
			embeddings = make([][]float32, len(contents))
			for i, content := range contents {
				vec, embedErr := taskAware.EmbedWithTask(ctx, content, taskTypes[0])
				if embedErr != nil {
					logging.Get(logging.CategoryStore).Warn("Failed to embed batch item %d (task_type=%s): %v", i, taskTypes[0], embedErr)
					continue
				}
				embeddings[i] = vec
			}
		} else {
			embeddings, err = engine.EmbedBatch(ctx, contents)
		}
	} else if taskAware, ok := engine.(embedding.TaskTypeAwareEngine); ok {
		embeddings = make([][]float32, len(contents))
		for i, content := range contents {
			vec, embedErr := taskAware.EmbedWithTask(ctx, content, taskTypes[i])
			if embedErr != nil {
				logging.Get(logging.CategoryStore).Warn("Failed to embed batch item %d (task_type=%s): %v", i, taskTypes[i], embedErr)
				continue
			}
			embeddings[i] = vec
		}
	} else {
		embeddings, err = engine.EmbedBatch(ctx, contents)
	}
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Batch embedding failed: %v", err)
		return 0, err
	}
	if len(embeddings) != len(contents) {
		return 0, fmt.Errorf("embedding batch size mismatch: %d != %d", len(embeddings), len(contents))
	}
	space, err := requireEmbeddingSpace(engine)
	if err != nil {
		return 0, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	// The index belongs to the attached engine's space; a batch produced by a
	// since-replaced engine is stored and stamped but not indexed.
	if vecEnabled && s.currentSpaceLocked() != space {
		logging.Get(logging.CategoryStore).Warn("embedding engine changed while embedding a batch: vectors stored in space %q and not indexed (current space %q)", space, s.currentSpaceLocked())
		vecEnabled = false
	}

	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	stmt, err := tx.Prepare("INSERT OR REPLACE INTO vectors (content, embedding, embedding_model, metadata) VALUES (?, ?, ?, ?)")
	if err != nil {
		_ = tx.Rollback()
		return 0, err
	}
	defer stmt.Close()

	var vecStmt, vecDelStmt *sql.Stmt
	if vecEnabled {
		// Delete-then-insert: vec0 errors on INSERT OR REPLACE rowid
		// conflicts instead of replacing. See insertVecIndexRow.
		vecDelStmt, err = tx.Prepare("DELETE FROM vec_index WHERE rowid = ? OR (content = ? AND json_extract(metadata, '$.kind') = 'predicate')")
		if err != nil {
			_ = tx.Rollback()
			return 0, err
		}
		defer vecDelStmt.Close()
		vecStmt, err = tx.Prepare("INSERT INTO vec_index (rowid, embedding, content, metadata) VALUES (?, ?, ?, ?)")
		if err != nil {
			_ = tx.Rollback()
			return 0, err
		}
		defer vecStmt.Close()
	}

	stored := 0
	failed := 0
	var firstErr error
	for i, content := range contents {
		if len(embeddings[i]) == 0 {
			failed++
			if firstErr == nil {
				firstErr = fmt.Errorf("empty embedding for content index %d", i)
			}
			continue
		}
		embeddingJSON, err := json.Marshal(embeddings[i])
		if err != nil {
			failed++
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		metaJSON, err := encodeRowMetadata(metadata[i])
		if err != nil {
			failed++
			if firstErr == nil {
				firstErr = fmt.Errorf("content index %d: %w", i, err)
			}
			continue
		}
		res, err := stmt.Exec(content, string(embeddingJSON), space, metaJSON)
		if err != nil {
			failed++
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if vecEnabled {
			id, lidErr := res.LastInsertId()
			if lidErr != nil {
				logging.Get(logging.CategoryStore).Warn("batch vec_index skipped: LastInsertId failed for row %d: %v (ANN drift)", i, lidErr)
			} else {
				vecBlob := encodeFloat32Slice(embeddings[i])
				if _, delErr := vecDelStmt.Exec(id, content); delErr != nil {
					logging.Get(logging.CategoryStore).Warn("batch vec_index clear failed for rowid=%d: %v (ANN drift)", id, delErr)
				} else if _, vecErr := vecStmt.Exec(id, vecBlob, content, metaJSON); vecErr != nil {
					logging.Get(logging.CategoryStore).Warn("batch vec_index insert failed for rowid=%d: %v (ANN drift)", id, vecErr)
				}
			}
		}
		stored++
	}

	if err := tx.Commit(); err != nil {
		return stored, err
	}
	if failed > 0 {
		logging.Get(logging.CategoryStore).Warn("StoreVectorBatchWithEmbedding: stored %d/%d vectors (%d failed): %v", stored, len(contents), failed, firstErr)
		return stored, fmt.Errorf("stored %d/%d vectors (%d failed): %v", stored, len(contents), failed, firstErr)
	}
	logging.Store("StoreVectorBatchWithEmbedding: stored %d vectors", stored)
	return stored, nil
}

// storeVectorKeywordOnly stores content without embeddings (fallback).
func (s *LocalStore) storeVectorKeywordOnly(content string, metadata map[string]any) error {
	metaJSON, err := encodeRowMetadata(metadata)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(
		"INSERT OR REPLACE INTO vectors (content, metadata) VALUES (?, ?)",
		content, metaJSON,
	)
	return err
}

func (s *LocalStore) storeVectorBatchKeywordOnly(contents []string, metadata []map[string]any) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	stmt, err := tx.Prepare("INSERT OR REPLACE INTO vectors (content, metadata) VALUES (?, ?)")
	if err != nil {
		_ = tx.Rollback()
		return 0, err
	}
	defer stmt.Close()

	stored := 0
	failed := 0
	var firstErr error
	// The sibling embedding path reports partial failure; this one used to
	// swallow it, so a batch could store nothing and still return nil.
	note := func(i int, err error) {
		failed++
		if firstErr == nil {
			firstErr = fmt.Errorf("content index %d: %w", i, err)
		}
	}
	for i, content := range contents {
		metaJSON, err := encodeRowMetadata(metadata[i])
		if err != nil {
			note(i, err)
			continue
		}
		if _, err := stmt.Exec(content, metaJSON); err != nil {
			note(i, err)
			continue
		}
		stored++
	}

	if err := tx.Commit(); err != nil {
		return stored, err
	}
	if failed > 0 {
		logging.Get(logging.CategoryStore).Warn("storeVectorBatchKeywordOnly: stored %d/%d vectors (%d failed): %v", stored, len(contents), failed, firstErr)
		return stored, fmt.Errorf("stored %d/%d vectors (%d failed): %w", stored, len(contents), failed, firstErr)
	}
	return stored, nil
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
	vecEnabled := s.vectorExt.Load()
	s.mu.RUnlock()

	if engine == nil {
		logging.StoreDebug("No embedding engine, falling back to keyword search")
		return s.vectorRecallKeyword(query, limit)
	}

	// Generate query embedding
	queryTaskType := embedding.GetOptimalTaskType(query, nil, true)
	var queryEmbedding []float32
	var err error
	if taskAware, ok := engine.(embedding.TaskTypeAwareEngine); ok && queryTaskType != "" {
		queryEmbedding, err = taskAware.EmbedWithTask(ctx, query, queryTaskType)
	} else {
		queryEmbedding, err = engine.Embed(ctx, query)
	}
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to generate query embedding: %v", err)
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	logging.StoreDebug("Query embedding generated: %d dimensions", len(queryEmbedding))

	sq, err := newSemanticQuery(engine, queryEmbedding)
	if err != nil {
		return nil, err
	}

	// One-time attribution: rows written before the space was recorded are
	// re-embedded with this engine so this and later recalls rank them.
	s.maybeHealUnattributed(ctx, engine, sq.space)

	if vecEnabled {
		logging.StoreDebug("Using sqlite-vec ANN search")
		return s.vectorRecallVec(query, sq, limit, nil, "", nil)
	}

	logging.StoreDebug("Using brute-force cosine similarity search")
	return s.vectorRecallBruteForce(query, sq, limit)
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
	vecEnabled := s.vectorExt.Load()
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

	queryTaskType := embedding.GetOptimalTaskType(query, nil, true)
	var queryEmbedding []float32
	var err error
	if taskAware, ok := engine.(embedding.TaskTypeAwareEngine); ok && queryTaskType != "" {
		queryEmbedding, err = taskAware.EmbedWithTask(ctx, query, queryTaskType)
	} else {
		queryEmbedding, err = engine.Embed(ctx, query)
	}
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to generate embedding for path search: %v", err)
		return nil, fmt.Errorf("failed to generate embedding: %w", err)
	}

	sq, err := newSemanticQuery(engine, queryEmbedding)
	if err != nil {
		return nil, err
	}

	// One-time attribution: rows written before the space was recorded are
	// re-embedded with this engine so this and later recalls rank them.
	s.maybeHealUnattributed(ctx, engine, sq.space)

	if vecEnabled {
		logging.StoreDebug("Using sqlite-vec ANN search with path filter")
		return s.vectorRecallVec(query, sq, limit, allowedPaths, "", nil)
	}

	logging.StoreDebug("Using brute-force cosine similarity search with path filter")
	return s.vectorRecallBruteForceByPaths(query, sq, limit, allowedPaths)
}

// VectorRecallSemanticFiltered restricts search to entries whose metadata contain a key/value pair.
// This reduces scanning cost when the store contains vectors from many campaigns.
func (s *LocalStore) VectorRecallSemanticFiltered(ctx context.Context, query string, limit int, metaKey string, metaValue any) ([]VectorEntry, error) {
	timer := logging.StartTimer(logging.CategoryStore, "VectorRecallSemanticFiltered")
	defer timer.Stop()

	if limit <= 0 {
		limit = 10
	}

	logging.StoreDebug("Semantic vector recall with filter: query=%q limit=%d filter=%s=%v", query, limit, metaKey, metaValue)

	s.mu.RLock()
	engine := s.embeddingEngine
	vecEnabled := s.vectorExt.Load()
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

	queryTaskType := embedding.GetOptimalTaskType(query, nil, true)
	var queryEmbedding []float32
	var err error
	if taskAware, ok := engine.(embedding.TaskTypeAwareEngine); ok && queryTaskType != "" {
		queryEmbedding, err = taskAware.EmbedWithTask(ctx, query, queryTaskType)
	} else {
		queryEmbedding, err = engine.Embed(ctx, query)
	}
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to generate embedding for filtered search: %v", err)
		return nil, fmt.Errorf("failed to generate embedding: %w", err)
	}

	sq, err := newSemanticQuery(engine, queryEmbedding)
	if err != nil {
		return nil, err
	}

	// One-time attribution: rows written before the space was recorded are
	// re-embedded with this engine so this and later recalls rank them.
	s.maybeHealUnattributed(ctx, engine, sq.space)

	if vecEnabled {
		logging.StoreDebug("Using sqlite-vec ANN search with metadata filter")
		return s.vectorRecallVec(query, sq, limit, nil, metaKey, metaValue)
	}

	logging.StoreDebug("Using brute-force cosine similarity search with metadata filter")
	return s.vectorRecallBruteForceFiltered(query, sq, limit, metaKey, metaValue)
}

// insertVecIndexRow writes one vec_index entry by delete-then-insert. Two
// facts force this shape: vec0 does not honor INSERT OR REPLACE on rowid
// conflicts (it errors with UNIQUE constraint failed), and a predicate-kind
// vectors row replaced by content gets a fresh rowid, orphaning its old vec
// row. The content-scoped delete only touches predicate-kind rows, whose
// content is unique, so duplicate ordinary content is unaffected.
func (s *LocalStore) insertVecIndexRow(id int64, embeddingVec []float32, content, metaJSON string) error {
	vecBlob := encodeFloat32Slice(embeddingVec)
	// Whatever happens below, the index may have changed.
	defer s.vecIndexRev.Add(1)
	if _, err := s.db.Exec("DELETE FROM vec_index WHERE rowid = ?", id); err != nil {
		return err
	}
	if _, err := s.db.Exec(
		"DELETE FROM vec_index WHERE content = ? AND json_extract(metadata, '$.kind') = 'predicate'",
		content,
	); err != nil {
		return err
	}
	_, err := s.db.Exec(
		"INSERT INTO vec_index (rowid, embedding, content, metadata) VALUES (?, ?, ?, ?)",
		id, vecBlob, content, metaJSON,
	)
	return err
}

// vecIndexMissingWhere selects the embedded vectors rows ANN search cannot
// find: an embedding in the attached engine's space, of the index's dimension,
// with no vec_index row. The space filter is the point: a row embedded by an
// earlier model of the SAME dimension passes every length check, and indexing
// it would put another coordinate system into the index. The dimension filter
// still matters for a stamped row whose JSON is torn.
const vecIndexMissingWhere = vectorEligibleClause + `
	AND json_valid(embedding) AND json_array_length(embedding) = ?
	AND id NOT IN (SELECT rowid FROM vec_index)`

// vecIndexMissingLocked counts the ANN drift: -1 when there is no sqlite-vec
// index to drift from. Call holding s.mu (read or write).
func (s *LocalStore) vecIndexMissingLocked(space string, dim int) (int64, error) {
	if s.db == nil || dim <= 0 || space == "" || !s.vectorExt.Load() || !tableExists(s.db, "vec_index") {
		return -1, nil
	}
	var n int64
	if err := s.db.QueryRow("SELECT COUNT(*) FROM vectors WHERE "+vecIndexMissingWhere, space, dim).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// ReconcileVecIndex indexes every embedded vectors row missing from
// vec_index and returns how many it healed.
//
// A vec_index insert that fails leaves its vectors row behind -- the write
// path logs "ANN drift" and moves on, because failing the store over the
// index would lose the row itself -- and until 2026-09-25 nothing put it
// back until the next SetEmbeddingEngine dropped and rebuilt the whole index.
// In a long session that is hours of ANN search not finding rows brute force
// finds. The maintenance cycle (MaintenanceConfig.ReconcileVecIndex) runs this.
//
// It waits for a pending backfill first: the backfill is already rebuilding
// the index, and two writers on the same rowids race.
func (s *LocalStore) ReconcileVecIndex(ctx context.Context) (int, error) {
	if s == nil {
		return 0, nil
	}
	s.mu.RLock()
	engine := s.embeddingEngine
	s.mu.RUnlock()
	if engine == nil || !s.vectorExt.Load() {
		return 0, nil
	}
	if err := s.waitForVecBackfill(ctx); err != nil {
		return 0, err
	}
	dim := engine.Dimensions()

	s.mu.Lock()
	defer s.mu.Unlock()
	// The space is read under the lock: an engine attached while the backfill
	// was awaited must not be reconciled against the previous engine's rows.
	space := s.currentSpaceLocked()
	if space == "" || !tableExists(s.db, "vec_index") {
		return 0, nil
	}
	rows, err := s.db.QueryContext(ctx, "SELECT id, content, embedding, metadata FROM vectors WHERE "+vecIndexMissingWhere, space, dim)
	if err != nil {
		return 0, fmt.Errorf("select vectors missing from vec_index: %w", err)
	}
	type missingRow struct {
		id       int64
		content  string
		vec      []float32
		metaJSON string
	}
	var missing []missingRow
	for rows.Next() {
		var r missingRow
		var embeddingJSON, metaJSON []byte
		if err := rows.Scan(&r.id, &r.content, &embeddingJSON, &metaJSON); err != nil {
			continue
		}
		vec, perr := fastParseVectorJSON(embeddingJSON, nil)
		if perr != nil || len(vec) != dim {
			continue
		}
		r.vec, r.metaJSON = vec, string(metaJSON)
		missing = append(missing, r)
	}
	rowsErr := rows.Err()
	rows.Close()
	if rowsErr != nil {
		return 0, fmt.Errorf("scan vectors missing from vec_index: %w", rowsErr)
	}

	healed := 0
	var firstErr error
	for _, r := range missing {
		if err := ctx.Err(); err != nil {
			return healed, err
		}
		if err := s.insertVecIndexRow(r.id, r.vec, r.content, r.metaJSON); err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("re-index rowid %d: %w", r.id, err)
			}
			continue
		}
		healed++
	}
	if len(missing) > 0 {
		logging.Store("vec_index reconcile: %d of %d drifted rows re-indexed", healed, len(missing))
	}
	return healed, firstErr
}

// vectorRecallKeyword is the fallback keyword-based search.
func (s *LocalStore) vectorRecallKeyword(query string, limit int) ([]VectorEntry, error) {
	// This is the old implementation from local.go VectorRecall
	// Kept for backward compatibility when no embedding engine is set
	return s.VectorRecall(query, limit)
}

// encodeRowMetadata turns a caller's metadata map into the blob written to the
// vectors table, and is the write-side half of decodeRowMetadata.
//
// The reason it returns an error rather than a best-effort string is a trap
// that took a while to see. Every insert here used to write
// string(mustIgnore(json.Marshal(meta))), so a map holding anything the encoder
// refuses -- a NaN, an Inf, a func, a cycle -- silently became the empty
// string. And the empty string is the ONE value the vectors table will not
// accept: idx_vectors_predicate_content_unique is a partial index over
// json_extract(metadata, '$.kind'), and SQLite evaluates that expression on
// every insert, so an unparseable blob aborts the write.
//
// Nobody wrote that index as a validator, but it is one, and its diagnostics
// are what you would expect from an accident: the insert fails with the bare
// string "malformed JSON", naming no table, no column, and nothing about the
// value that could not be encoded. In the keyword-only batch path the row was
// then skipped without even a count, so the visible symptom was content that
// simply never became searchable.
//
// Failing here, with the marshal error attached, turns that into one sentence
// that says which row and why.
func encodeRowMetadata(meta map[string]any) (string, error) {
	b, err := json.Marshal(meta)
	if err != nil {
		return "", fmt.Errorf("metadata is not JSON-serializable: %w", err)
	}
	return string(b), nil
}

// decodeRowMetadata turns the metadata blob of one vectors row into the map
// callers see, and is the single place that decides what a corrupt blob means.
//
// It exists because the four scan loops that read this table had drifted apart.
// The sqlite-vec path logged the decode failure and marked the row; the three
// brute-force fallback paths dropped the error on the floor. That is backwards
// from where the diagnostics are needed: brute force is what runs when the
// sqlite-vec extension is absent, which is the common case in development.
//
// The metadata-filtered fallback was the sharp one. A row whose blob fails to
// decode has no keys, so matchesMetadata rejects it and the row vanishes from
// the result set — a silent wrong answer with nothing in the log to explain it.
// "My search stopped finding that file" is a very hard question to answer from
// nothing; it is an easy one to answer from a warning naming the row id.
//
// The returned map is never nil, so callers can write to it without a guard,
// and whatever partially decoded before the failure is kept: half a map plus
// the marker is strictly more evidence than an empty one.
func decodeRowMetadata(id int64, metaJSON []byte) map[string]any {
	meta := make(map[string]any)
	if len(metaJSON) == 0 {
		return meta
	}
	err := json.Unmarshal(metaJSON, &meta)
	// The nil check has to happen on the SUCCESS path too, and that is not
	// obvious. json.Unmarshal of the literal `null` into a map sets the map to
	// nil and reports no error -- and `null` is exactly what json.Marshal
	// writes for a row stored with nil metadata, so it is the single most
	// common blob in this table. Re-making the map here is what keeps the
	// "never nil" promise true for the callers that write to it unguarded.
	if meta == nil {
		meta = make(map[string]any)
	}
	if err != nil {
		logging.Get(logging.CategoryStore).Warn(
			"vectors row id=%d: metadata JSON unparseable (%d bytes): %v",
			id, len(metaJSON), err)
		// Marks the row for triage: a caller seeing an unexpectedly empty
		// metadata map can tell "never had any" from "had some, lost it".
		meta["_corrupt_metadata"] = true
	}
	return meta
}

func matchesMetadata(meta map[string]any, key string, value any) bool {
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

func buildPathFilteredQuery(space string, paths []string) (string, []any) {
	if len(paths) == 0 {
		return bruteForceBaseQuery, []any{space}
	}
	var sb strings.Builder
	sb.WriteString(bruteForceBaseQuery)
	sb.WriteString(" AND (")
	args := make([]any, 0, 1+len(paths)*2)
	args = append(args, space)
	for i, p := range paths {
		if i > 0 {
			sb.WriteString(" OR ")
		}
		sb.WriteString("json_extract(metadata, ?) = ?")
		args = append(args, "$.path", p)
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
func (s *LocalStore) vectorRecallVec(queryText string, q semanticQuery, limit int, allowedPaths []string, metaKey string, metaValue any) ([]VectorEntry, error) {
	timer := logging.StartTimer(logging.CategoryStore, "vectorRecallVec")
	defer timer.Stop()

	if !s.vectorExt.Load() {
		logging.Get(logging.CategoryStore).Error("sqlite-vec not enabled for ANN search")
		return nil, fmt.Errorf("sqlite-vec not enabled")
	}
	if limit <= 0 {
		limit = 10
	}

	logging.StoreDebug("sqlite-vec ANN search: limit=%d, paths=%d, metaFilter=%s", limit, len(allowedPaths), metaKey)

	queryBlob := encodeFloat32Slice(q.vec)

	// Only rows embedded in the query's space are ranked. The index is built
	// from such rows alone; the filter keeps a row that predates that rule, or
	// outlived a failed clear, from being ranked against a foreign query.
	where := []string{vecEligibleRowidFilter}
	args := []any{q.space}

	// Path filters
	if len(allowedPaths) > 0 {
		clause := make([]string, 0, len(allowedPaths))
		for _, p := range allowedPaths {
			clause = append(clause, "json_extract(metadata, ?) = ?")
			args = append(args, "$.path", p)
		}
		where = append(where, "("+strings.Join(clause, " OR ")+")")
	}

	if metaKey != "" && metaValue != nil {
		where = append(where, "json_extract(metadata, ?) = ?")
		args = append(args, fmt.Sprintf(`$."%s"`, metaKey), metaValue)
	}

	sqlStr := "SELECT rowid, content, metadata, vec_distance_cosine(embedding, ?) AS dist FROM vec_index"
	args = append([]any{queryBlob}, args...)
	if len(where) > 0 {
		sqlStr += " WHERE " + strings.Join(where, " AND ")
	}
	sqlStr += " ORDER BY dist ASC LIMIT ?"
	args = append(args, limit)

	s.mu.RLock()
	s.noteExcludedLocked(q.space)
	rows, err := s.db.Query(sqlStr, args...)
	s.mu.RUnlock()
	if err != nil {
		logging.Get(logging.CategoryStore).Error("sqlite-vec query failed: %v", err)
		return nil, err
	}
	defer rows.Close()

	// The statement runs as rows are stepped, so a failure the database
	// reports at execution -- a query vector the distance function refuses,
	// a backend limit -- arrives through rows.Err(), after Query returned.
	// It used to be dropped, and a failed search read as a healthy empty one
	// (audit knn-recall-limit-error-contract). A row that cannot be scanned
	// makes the result incomplete, which is a failure too.
	results := make([]VectorEntry, 0, limit)
	for rows.Next() {
		var id int64
		var content string
		var metaJSON []byte
		var nullableDist sql.NullFloat64
		if err := rows.Scan(&id, &content, &metaJSON, &nullableDist); err != nil {
			return nil, fmt.Errorf("sqlite-vec recall: scan row: %w", err)
		}
		if !nullableDist.Valid {
			continue // no distance to rank by (a zero vector): not a candidate
		}
		dist := nullableDist.Float64
		entry := VectorEntry{
			ID:        id,
			Content:   content,
			CreatedAt: time.Now(),
			Metadata:  decodeRowMetadata(id, metaJSON),
		}
		entry.Metadata["similarity"] = 1 - dist
		results = append(results, entry)
	}
	if err := rows.Err(); err != nil {
		logging.Get(logging.CategoryStore).Error("sqlite-vec query failed while stepping: %v", err)
		return nil, fmt.Errorf("sqlite-vec recall: %w", err)
	}

	if len(results) > 0 {
		logging.Store("VECTOR QUERY [vec0 ANN] -> [%q]", queryText)
		logging.Store("SQL Executed: %s", sqlStr)
		for i, res := range results {
			contentStr := types.Ellipsize(res.Content, 100)
			logging.Store("  Result %d: (dist: %.4f) -> %q", i+1, 1-res.Metadata["similarity"].(float64), contentStr)
		}
	} else {
		logging.Store("VECTOR QUERY [vec0 ANN] -> [%q]", queryText)
		logging.Store("SQL Executed: %s", sqlStr)
		logging.Store("  -> 0 results returned")
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
	s.vecIndexRev.Add(1)

	// Drop existing table to ensure clean dimension application if models switched
	_, _ = s.db.Exec("DROP TABLE IF EXISTS vec_index")

	stmt := fmt.Sprintf("CREATE VIRTUAL TABLE vec_index USING vec0(embedding float[%d], content TEXT, metadata TEXT)", dim)
	if _, err := s.db.Exec(stmt); err == nil {
		s.vectorExt.Store(true)
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
// NOTE: This function runs in a background goroutine to avoid blocking startup.
// Uses transaction batching for 30-50x speedup over individual INSERTs.
func (s *LocalStore) backfillVecIndex(dim int, space string, vecExt bool, done chan struct{}) {
	if !vecExt || s.db == nil || dim <= 0 || space == "" {
		return
	}

	logging.StoreDebug("Starting backfill of existing embeddings into sqlite-vec index")

	// Phase 1: Read all rows into memory (quick read, then release rows).
	// rows is closed deterministically once the scan loop exits so the
	// connection returns to the pool before the transaction phase begins.
	rows, err := s.db.Query("SELECT id, content, embedding, metadata FROM vectors WHERE "+vectorEligibleClause, space)
	if err != nil {
		logging.Get(logging.CategoryStore).Warn("Failed to query embeddings for backfill: %v", err)
		return
	}
	defer rows.Close()

	type embeddingRow struct {
		id       int64
		content  string
		vecBlob  []byte
		metaJSON string
	}

	var toInsert []embeddingRow
	skippedCount := 0

	var embeddingVec []float32
	for rows.Next() {
		var id int64
		var content string
		var embeddingJSON, metaJSON []byte
		if err := rows.Scan(&id, &content, &embeddingJSON, &metaJSON); err != nil {
			skippedCount++
			continue
		}
		var parseErr error
		embeddingVec, parseErr = fastParseVectorJSON(embeddingJSON, embeddingVec)
		if parseErr != nil {
			skippedCount++
			continue
		}
		if len(embeddingVec) != dim {
			skippedCount++
			continue
		}
		toInsert = append(toInsert, embeddingRow{
			id:       id,
			content:  content,
			vecBlob:  encodeFloat32Slice(embeddingVec),
			metaJSON: string(metaJSON),
		})
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		logging.Get(logging.CategoryStore).Warn("vec backfill: row iteration error: %v", rowsErr)
	}
	// Release the connection before starting the transaction phase.
	rows.Close()

	if len(toInsert) == 0 {
		logging.StoreDebug("No embeddings to backfill into vec_index")
		return
	}

	logging.Store("Backfilling %d embeddings into sqlite-vec index (batched transaction)", len(toInsert))

	// Phase 2: Batched INSERT within a transaction for 30-50x speedup
	const batchSize = 100
	backfillCount := 0

	for i := 0; i < len(toInsert); i += batchSize {
		if !s.isCurrentBackfill(done) {
			logging.StoreDebug("vec backfill superseded by a newer engine; aborting")
			return
		}
		end := min(i+batchSize, len(toInsert))
		batch := toInsert[i:end]

		// Use a transaction for each batch
		tx, err := s.db.Begin()
		if err != nil {
			logging.Get(logging.CategoryStore).Warn("Failed to begin transaction for backfill batch: %v", err)
			continue
		}

		var queryBuilder strings.Builder
		queryBuilder.WriteString("INSERT OR REPLACE INTO vec_index (rowid, embedding, content, metadata) VALUES ")
		args := make([]any, 0, len(batch)*4)
		for j, row := range batch {
			if j > 0 {
				queryBuilder.WriteString(", ")
			}
			queryBuilder.WriteString("(?, ?, ?, ?)")
			args = append(args, row.id, row.vecBlob, row.content, row.metaJSON)
		}

		stmt, err := tx.Prepare(queryBuilder.String())
		if err != nil {
			tx.Rollback()
			logging.Get(logging.CategoryStore).Warn("Failed to prepare statement for backfill: %v", err)
			continue
		}

		batchSuccess := 0
		if _, err := stmt.Exec(args...); err == nil {
			batchSuccess = len(batch)
		} else {
			logging.Get(logging.CategoryStore).Warn("Failed to execute batch statement for backfill: %v", err)
		}
		stmt.Close()

		if err := tx.Commit(); err != nil {
			logging.Get(logging.CategoryStore).Warn("Failed to commit backfill batch: %v", err)
			continue
		}

		backfillCount += batchSuccess
	}

	logging.Store("Backfill complete: migrated=%d, skipped=%d", backfillCount, skippedCount)
}

// CountVectorsByMetadata returns the number of vectors whose metadata contains
// the key/value pair. With an embedding engine attached it counts only the
// vectors that engine can serve (see vector_identity.go): callers use the count
// to decide whether a corpus is already indexed, and a vector in another
// embedding space is not indexed for this engine, so counting it would stop the
// caller from ever re-embedding what it stored.
func (s *LocalStore) CountVectorsByMetadata(metaKey string, metaValue any) (int, error) {
	timer := logging.StartTimer(logging.CategoryStore, "CountVectorsByMetadata")
	defer timer.Stop()

	if metaKey == "" {
		return 0, fmt.Errorf("metadata key is required")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	path := fmt.Sprintf(`$."%s"`, metaKey)
	query := "SELECT COUNT(*) FROM vectors WHERE json_extract(metadata, ?) = ?"
	args := []any{path, metaValue}
	if space := s.currentSpaceLocked(); space != "" {
		query += " AND " + vectorEligibleClause
		args = append(args, space)
	}
	var count int
	if err := s.db.QueryRow(query, args...).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// VectorContentsByMetadata returns a set of vector contents matching a metadata
// key/value. Like CountVectorsByMetadata it lists only what the attached engine
// can serve, so a caller that skips contents already present re-embeds the ones
// stored in another space.
func (s *LocalStore) VectorContentsByMetadata(metaKey string, metaValue any) (map[string]struct{}, error) {
	if metaKey == "" {
		return nil, fmt.Errorf("metadata key is required")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	path := fmt.Sprintf(`$."%s"`, metaKey)
	query := "SELECT content FROM vectors WHERE json_extract(metadata, ?) = ?"
	args := []any{path, metaValue}
	if space := s.currentSpaceLocked(); space != "" {
		query += " AND " + vectorEligibleClause
		args = append(args, space)
	}
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	contents := make(map[string]struct{})
	for rows.Next() {
		var content string
		if err := rows.Scan(&content); err != nil {
			continue
		}
		contents[content] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return contents, nil
}

// DeleteVectorsByMetadata removes vectors whose metadata contains the key/value pair.
// Returns the number of rows deleted.
func (s *LocalStore) DeleteVectorsByMetadata(metaKey string, metaValue any) (int64, error) {
	timer := logging.StartTimer(logging.CategoryStore, "DeleteVectorsByMetadata")
	defer timer.Stop()

	if metaKey == "" {
		return 0, fmt.Errorf("metadata key is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	path := fmt.Sprintf(`$."%s"`, metaKey)
	result, err := s.db.Exec("DELETE FROM vectors WHERE json_extract(metadata, ?) = ?", path, metaValue)
	if err != nil {
		return 0, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	// ANN reads vec_index, not vectors: rows deleted above would otherwise
	// keep surfacing in sqlite-vec search results as ghosts.
	if s.vectorExt.Load() && tableExists(s.db, "vec_index") {
		if _, err := s.db.Exec("DELETE FROM vec_index WHERE rowid NOT IN (SELECT id FROM vectors)"); err != nil {
			logging.Get(logging.CategoryStore).Warn("vec_index ghost purge after delete failed: %v (ANN drift)", err)
		}
	}
	return rows, nil
}

// VectorSpaceCounts counts the embedded vectors per embedding space they were
// written in ("" for rows written before the space was recorded). It reads
// the canonical table only: no engine, no index work.
func (s *LocalStore) VectorSpaceCounts() (map[string]int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows, err := s.db.Query(`SELECT COALESCE(embedding_model, ''), COUNT(*) FROM vectors WHERE embedding IS NOT NULL GROUP BY COALESCE(embedding_model, '')`)
	if err != nil {
		return nil, fmt.Errorf("count vectors by embedding space: %w", err)
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var space string
		var n int64
		if err := rows.Scan(&space, &n); err != nil {
			return nil, fmt.Errorf("count vectors by embedding space: %w", err)
		}
		out[space] += n
	}
	return out, rows.Err()
}

// VecIndexRows is how many rows the semantic index holds, and false when
// there is no index to count (sqlite-vec unavailable, or never built).
func (s *LocalStore) VecIndexRows() (int64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !tableExists(s.db, "vec_index") {
		return 0, false
	}
	var n int64
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM vec_index`).Scan(&n); err != nil {
		return 0, false
	}
	return n, true
}

// GetVectorStats returns statistics about stored vectors.
func (s *LocalStore) GetVectorStats() (map[string]any, error) {
	timer := logging.StartTimer(logging.CategoryStore, "GetVectorStats")
	defer timer.Stop()

	s.mu.RLock()
	defer s.mu.RUnlock()

	logging.StoreDebug("Computing vector store statistics")

	stats := make(map[string]any)

	// Both scans are checked: on a missing, locked or drifted table the old
	// code reported total_vectors 0 — the exact numbers of a healthy empty
	// store — instead of an error. See LearningStore.GetStats.
	var totalVectors int64
	if err := s.db.QueryRow("SELECT COUNT(*) FROM vectors").Scan(&totalVectors); err != nil {
		return nil, fmt.Errorf("count vectors: %w", err)
	}
	stats["total_vectors"] = totalVectors

	var withEmbeddings int64
	if err := s.db.QueryRow("SELECT COUNT(*) FROM vectors WHERE embedding IS NOT NULL").Scan(&withEmbeddings); err != nil {
		return nil, fmt.Errorf("count embedded vectors: %w", err)
	}
	stats["with_embeddings"] = withEmbeddings

	withoutEmbeddings := totalVectors - withEmbeddings
	stats["without_embeddings"] = withoutEmbeddings

	if s.embeddingEngine != nil {
		stats["embedding_engine"] = s.embeddingEngine.Name()
		stats["embedding_dimensions"] = s.embeddingEngine.Dimensions()
		if space := s.currentSpaceLocked(); space != "" {
			cov, err := s.vectorCoverageLocked(space)
			if err != nil {
				return nil, err
			}
			stats["eligible_vectors"] = cov.Eligible
			stats["unattributed_vectors"] = cov.Unattributed
			stats["foreign_vectors"] = cov.Foreign
		}
	} else {
		stats["embedding_engine"] = "none (keyword search)"
	}

	logging.StoreDebug("Vector stats: total=%d, with_embeddings=%d, engine=%v", totalVectors, withEmbeddings, stats["embedding_engine"])
	return stats, nil
}

// ReembedAllVectors regenerates embeddings for all vectors that don't have them.
func (s *LocalStore) VectorRecallSemanticWithTask(ctx context.Context, query string, limit int, queryTaskType string) ([]VectorEntry, error) {
	timer := logging.StartTimer(logging.CategoryStore, "VectorRecallSemanticWithTask")
	defer timer.Stop()

	if limit <= 0 {
		limit = 10
	}

	logging.StoreDebug("Semantic vector recall with task type: query=%q limit=%d task=%s", query, limit, queryTaskType)

	s.mu.RLock()
	engine := s.embeddingEngine
	vecEnabled := s.vectorExt.Load()
	s.mu.RUnlock()

	if engine == nil {
		logging.StoreDebug("No embedding engine, falling back to keyword search")
		return s.vectorRecallKeyword(query, limit)
	}

	// Try task-type-aware embedding first
	var queryEmbedding []float32
	var err error

	if taskAware, ok := engine.(embedding.TaskTypeAwareEngine); ok && queryTaskType != "" {
		logging.StoreDebug("Using task-aware embedding with task type: %s", queryTaskType)
		queryEmbedding, err = taskAware.EmbedWithTask(ctx, query, queryTaskType)
	} else {
		logging.StoreDebug("Using standard embedding (no task type support)")
		queryEmbedding, err = engine.Embed(ctx, query)
	}

	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to generate query embedding: %v", err)
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	logging.StoreDebug("Query embedding generated: %d dimensions", len(queryEmbedding))

	sq, err := newSemanticQuery(engine, queryEmbedding)
	if err != nil {
		return nil, err
	}

	// One-time attribution: rows written before the space was recorded are
	// re-embedded with this engine so this and later recalls rank them.
	s.maybeHealUnattributed(ctx, engine, sq.space)

	if vecEnabled {
		logging.StoreDebug("Using sqlite-vec ANN search")
		return s.vectorRecallVec(query, sq, limit, nil, "", nil)
	}

	// Fallback to brute-force search
	return s.vectorRecallBruteForce(query, sq, limit)
}

// VectorRecallForPromptAtoms searches for prompt atoms using RETRIEVAL_QUERY task type.
// Filters results by content_type == "prompt_atom" in metadata.
func (s *LocalStore) VectorRecallForPromptAtoms(ctx context.Context, query string, limit int) ([]VectorEntry, error) {
	timer := logging.StartTimer(logging.CategoryStore, "VectorRecallForPromptAtoms")
	defer timer.Stop()

	if limit <= 0 {
		limit = 10
	}

	logging.StoreDebug("Vector recall for prompt atoms: query=%q limit=%d", query, limit)

	s.mu.RLock()
	engine := s.embeddingEngine
	vecEnabled := s.vectorExt.Load()
	s.mu.RUnlock()

	if engine == nil {
		logging.StoreDebug("No embedding engine, falling back to keyword search with filter")
		all, err := s.vectorRecallKeyword(query, limit*5)
		if err != nil {
			return nil, err
		}
		filtered := filterByContentType(all, "prompt_atom")
		if len(filtered) > limit {
			filtered = filtered[:limit]
		}
		logging.StoreDebug("Filtered keyword search returned %d prompt atoms", len(filtered))
		return filtered, nil
	}

	// Use RETRIEVAL_QUERY task type for query embedding
	var queryEmbedding []float32
	var err error

	taskType := embedding.SelectTaskType(embedding.ContentTypeQuery, true)
	if taskAware, ok := engine.(embedding.TaskTypeAwareEngine); ok && taskType != "" {
		logging.StoreDebug("Using %s task type for prompt atom search", taskType)
		queryEmbedding, err = taskAware.EmbedWithTask(ctx, query, taskType)
	} else {
		logging.StoreDebug("Using standard embedding (no task type support)")
		queryEmbedding, err = engine.Embed(ctx, query)
	}

	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to generate query embedding for prompt atoms: %v", err)
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	logging.StoreDebug("Query embedding generated: %d dimensions", len(queryEmbedding))

	sq, err := newSemanticQuery(engine, queryEmbedding)
	if err != nil {
		return nil, err
	}

	// One-time attribution: rows written before the space was recorded are
	// re-embedded with this engine so this and later recalls rank them.
	s.maybeHealUnattributed(ctx, engine, sq.space)

	if vecEnabled {
		logging.StoreDebug("Using sqlite-vec ANN search with prompt_atom filter")
		return s.vectorRecallVec(query, sq, limit, nil, "content_type", "prompt_atom")
	}

	// Fallback to filtered brute-force search
	return s.vectorRecallBruteForceFiltered(query, sq, limit, "content_type", "prompt_atom")
}
