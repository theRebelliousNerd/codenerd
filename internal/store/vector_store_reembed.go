package store

import (
	"codenerd/internal/embedding"
	"codenerd/internal/logging"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

// Useful for migrating from keyword-only to embedding-based search.
// Returns nil if no vectors need re-embedding.
// encodeEmbedding serializes an embedding for the vectors.embedding column,
// and refuses rather than returning the empty string.
//
// The failure it prevents is destructive, which is why it is worth a function.
// Both re-embed loops used to write string(mustIgnore(json.Marshal(vec))) into
// an UPDATE. json.Marshal of a []float32 fails on exactly one thing -- a NaN or
// an Inf, which is what a degenerate input or a mangled model response
// produces -- and a failed marshal yields nil, whose string form is "".
//
// "" is not NULL, so the row still matches `WHERE embedding IS NOT NULL` in
// every recall query, and fastParseVectorJSON then rejects it and the loop
// skips the row. The transaction commits, the function reports success, and a
// row that was searchable a moment ago is quietly unsearchable forever. Same
// for a nil embedding, which marshals to the literal `null` and parses no
// better.
//
// Refusing here costs one row out of the batch instead of that row's index
// entry.
func encodeEmbedding(vec []float32) (string, error) {
	if len(vec) == 0 {
		return "", fmt.Errorf("empty embedding")
	}
	b, err := json.Marshal(vec)
	if err != nil {
		return "", fmt.Errorf("embedding is not JSON-serializable: %w", err)
	}
	return string(b), nil
}

func (s *LocalStore) ReembedAllVectors(ctx context.Context) error {
	timer := logging.StartTimer(logging.CategoryStore, "ReembedAllVectors")
	defer timer.Stop()

	s.mu.RLock()
	engine := s.embeddingEngine
	s.mu.RUnlock()
	if engine == nil {
		logging.Get(logging.CategoryStore).Error("Cannot re-embed: no embedding engine configured")
		return fmt.Errorf("no embedding engine configured")
	}
	// A pending backfill writes the same vec_index rows; wait for it rather
	// than racing it. Must run without holding s.mu.
	if err := s.waitForVecBackfill(ctx); err != nil {
		return err
	}

	logging.Store("Starting re-embedding of all vectors without embeddings")

	// Fetch all vectors without embeddings
	vectors, err := listVectorsForReembed(s.db, true)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to query vectors for re-embedding: %v", err)
		return err
	}

	if len(vectors) == 0 {
		logging.StoreDebug("No vectors need re-embedding")
		return nil
	}

	logging.Store("Found %d vectors to re-embed", len(vectors))

	// Generate embeddings in batches. Embedding is a network call and runs
	// without the store lock; only the write transaction takes it. Holding
	// s.mu for the whole re-embed used to freeze every reader for minutes.
	batchSize := 32
	totalEmbedded := 0
	skipped := 0
	for i := 0; i < len(vectors); i += batchSize {
		end := min(i+batchSize, len(vectors))
		batch := vectors[i:end]

		logging.StoreDebug("Processing batch %d-%d of %d", i, end, len(vectors))

		// Collect texts
		texts := make([]string, len(batch))
		for j, v := range batch {
			texts[j] = v.content
		}

		// Generate embeddings
		embeddings, err := engine.EmbedBatch(ctx, texts)
		if err != nil {
			logging.Get(logging.CategoryStore).Error("Failed to generate batch embeddings: %v", err)
			return fmt.Errorf("failed to generate batch embeddings: %w", err)
		}

		embedded, batchSkipped, err := s.applyVectorEmbeddings(batch, embeddings)
		if err != nil {
			return err
		}
		totalEmbedded += embedded
		skipped += batchSkipped
	}

	if skipped > 0 {
		logging.Get(logging.CategoryStore).Warn(
			"Re-embedding complete: %d vectors processed, %d left at their previous embedding", totalEmbedded, skipped)
	} else {
		logging.Store("Re-embedding complete: %d vectors processed", totalEmbedded)
	}
	return nil
}

// vectorToEmbed is one row awaiting an embedding write.
type vectorToEmbed struct {
	id       int64
	content  string
	metadata string
}

// listVectorsForReembed reads the rows a re-embed pass will process: only
// unembedded rows normally, every row for a force pass.
func listVectorsForReembed(db *sql.DB, unembeddedOnly bool) ([]vectorToEmbed, error) {
	query := "SELECT id, content, metadata FROM vectors"
	if unembeddedOnly {
		query += " WHERE embedding IS NULL"
	}
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var vectors []vectorToEmbed
	for rows.Next() {
		var v vectorToEmbed
		if err := rows.Scan(&v.id, &v.content, &v.metadata); err != nil {
			continue
		}
		vectors = append(vectors, v)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return vectors, nil
}

// applyVectorEmbeddings writes one batch of fresh embeddings to vectors and,
// when the ANN index exists, to vec_index by rowid. Short batches (a failed
// per-item embed left a nil slot) and unserializable vectors are skipped, so
// one bad row costs the batch nothing.
func (s *LocalStore) applyVectorEmbeddings(batch []vectorToEmbed, embeddings [][]float32) (embedded, skipped int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to start transaction: %w", err)
	}
	// A failed commit or an early return below must not leave the txn open;
	// a committed txn makes this Rollback a no-op.
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	updateStmt, err := tx.Prepare("UPDATE vectors SET embedding = ? WHERE id = ?")
	if err != nil {
		return 0, 0, fmt.Errorf("failed to prepare update stmt: %w", err)
	}
	defer updateStmt.Close()

	var deleteVecStmt, insertVecStmt *sql.Stmt
	if s.vectorExt.Load() {
		// Delete-then-insert: vec0 errors on INSERT OR REPLACE rowid
		// conflicts instead of replacing. See insertVecIndexRow.
		deleteVecStmt, err = tx.Prepare("DELETE FROM vec_index WHERE rowid = ?")
		if err != nil {
			return 0, 0, fmt.Errorf("failed to prepare vec_index delete stmt: %w", err)
		}
		defer deleteVecStmt.Close()
		insertVecStmt, err = tx.Prepare("INSERT INTO vec_index (rowid, embedding, content, metadata) VALUES (?, ?, ?, ?)")
		if err != nil {
			return 0, 0, fmt.Errorf("failed to prepare vec_index stmt: %w", err)
		}
		defer insertVecStmt.Close()
	}

	for j, v := range batch {
		if j >= len(embeddings) {
			skipped++
			continue
		}
		embeddingJSON, encErr := encodeEmbedding(embeddings[j])
		if encErr != nil {
			logging.Get(logging.CategoryStore).Warn(
				"Leaving vector %d at its previous embedding: %v", v.id, encErr)
			skipped++
			continue
		}
		if _, err := updateStmt.Exec(embeddingJSON, v.id); err != nil {
			logging.Get(logging.CategoryStore).Error("Failed to update vector %d: %v", v.id, err)
			return embedded, skipped, fmt.Errorf("failed to update vector %d: %w", v.id, err)
		}
		// Keep sqlite-vec index in sync when available, keyed by rowid so a
		// re-embed replaces the old entry instead of appending a stale twin.
		if s.vectorExt.Load() {
			if _, err := deleteVecStmt.Exec(v.id); err != nil {
				return embedded, skipped, fmt.Errorf("failed to clear vec_index for vector %d: %w", v.id, err)
			}
			vecBlob := encodeFloat32Slice(embeddings[j])
			if _, err := insertVecStmt.Exec(v.id, vecBlob, v.content, v.metadata); err != nil {
				return embedded, skipped, fmt.Errorf("failed to update vec_index for vector %d: %w", v.id, err)
			}
		}
		embedded++
	}

	if err := tx.Commit(); err != nil {
		return embedded, skipped, fmt.Errorf("failed to commit transaction: %w", err)
	}
	committed = true
	return embedded, skipped, nil
}

// ReembedAllVectorsForce regenerates embeddings for ALL vectors, overwriting existing ones.
// This is required when switching embedding providers/models.
// Returns the number of vectors re-embedded.
func (s *LocalStore) ReembedAllVectorsForce(ctx context.Context) (int, error) {
	timer := logging.StartTimer(logging.CategoryStore, "ReembedAllVectorsForce")
	defer timer.Stop()

	s.mu.RLock()
	engine := s.embeddingEngine
	s.mu.RUnlock()
	if engine == nil {
		logging.Get(logging.CategoryStore).Error("Cannot force re-embed: no embedding engine configured")
		return 0, fmt.Errorf("no embedding engine configured")
	}
	// A pending backfill writes the same vec_index rows; wait for it rather
	// than racing it. Must run without holding s.mu.
	if err := s.waitForVecBackfill(ctx); err != nil {
		return 0, err
	}

	logging.Store("Starting force re-embedding of all vectors in DB: %s", s.dbPath)

	vectors, err := listVectorsForReembed(s.db, false)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to query vectors for force re-embedding: %v", err)
		return 0, err
	}

	if len(vectors) == 0 {
		logging.StoreDebug("No vectors found for force re-embedding")
		return 0, nil
	}

	logging.Store("Found %d vectors to force re-embed", len(vectors))

	// A force pass rewrites every row, so clear the ANN index once up front.
	// This also heals stale twins appended by the old rowid-less force loop,
	// which per-row deletes cannot reach (they live under other rowids).
	if s.vectorExt.Load() {
		if _, err := s.db.Exec("DELETE FROM vec_index"); err != nil {
			logging.Get(logging.CategoryStore).Warn("Force re-embed vec_index clear failed: %v", err)
		}
	}

	batchSize := 32
	totalBatches := (len(vectors) + batchSize - 1) / batchSize
	totalEmbedded := 0
	skipped := 0
	var lastFallbackErr error
	for i := 0; i < len(vectors); i += batchSize {
		end := min(i+batchSize, len(vectors))
		batch := vectors[i:end]
		batchNum := (i / batchSize) + 1
		logging.Store("ReembedAllVectorsForce [%s]: batch %d/%d (%d vectors)",
			s.dbPath, batchNum, totalBatches, len(batch))

		texts := make([]string, len(batch))
		taskTypes := make([]string, len(batch))
		uniformTask := true
		for j, v := range batch {
			texts[j] = v.content
			var meta map[string]any
			if v.metadata != "" {
				_ = json.Unmarshal([]byte(v.metadata), &meta)
			}
			taskTypes[j] = embedding.GetOptimalTaskType(v.content, meta, false)
			if j > 0 && taskTypes[j] != taskTypes[0] {
				uniformTask = false
			}
		}

		var embeddings [][]float32
		var err error
		if uniformTask && taskTypes[0] != "" {
			if batchAware, ok := engine.(embedding.TaskTypeBatchAwareEngine); ok {
				embeddings, err = batchAware.EmbedBatchWithTask(ctx, texts, taskTypes[0])
			} else if taskAware, ok := engine.(embedding.TaskTypeAwareEngine); ok {
				embeddings = make([][]float32, len(batch))
				for j, v := range batch {
					vec, embedErr := taskAware.EmbedWithTask(ctx, v.content, taskTypes[0])
					if embedErr != nil {
						logging.Get(logging.CategoryStore).Warn("Failed to embed vector %d in %s (task_type=%s): %v", v.id, s.dbPath, taskTypes[0], embedErr)
						lastFallbackErr = embedErr
						continue
					}
					embeddings[j] = vec
				}
			} else {
				embeddings, err = engine.EmbedBatch(ctx, texts)
			}
		} else if taskAware, ok := engine.(embedding.TaskTypeAwareEngine); ok {
			embeddings = make([][]float32, len(batch))
			for j, v := range batch {
				vec, embedErr := taskAware.EmbedWithTask(ctx, v.content, taskTypes[j])
				if embedErr != nil {
					logging.Get(logging.CategoryStore).Warn("Failed to embed vector %d in %s (task_type=%s): %v", v.id, s.dbPath, taskTypes[j], embedErr)
					lastFallbackErr = embedErr
					continue
				}
				embeddings[j] = vec
			}
		} else {
			embeddings, err = engine.EmbedBatch(ctx, texts)
		}

		if err != nil {
			logging.Get(logging.CategoryStore).Warn("Force batch embeddings failed for %s (batch %d/%d): %v; falling back to per-item embedding",
				s.dbPath, batchNum, totalBatches, err)
			embeddings = make([][]float32, len(batch))
			for j, v := range batch {
				var vec []float32
				var embedErr error
				if taskAware, ok := engine.(embedding.TaskTypeAwareEngine); ok {
					vec, embedErr = taskAware.EmbedWithTask(ctx, v.content, taskTypes[j])
				} else {
					vec, embedErr = engine.Embed(ctx, v.content)
				}
				if embedErr != nil {
					logging.Get(logging.CategoryStore).Warn("Failed to embed vector %d in %s: %v", v.id, s.dbPath, embedErr)
					lastFallbackErr = embedErr
					continue
				}
				embeddings[j] = vec
			}
		}

		// The shared writer keys vec_index by rowid: the old force loop
		// omitted rowid, so every force re-embed APPENDED stale twins of
		// each row instead of replacing them.
		embedded, batchSkipped, err := s.applyVectorEmbeddings(batch, embeddings)
		if err != nil {
			return totalEmbedded, err
		}
		totalEmbedded += embedded
		skipped += batchSkipped
	}

	if totalEmbedded == 0 && lastFallbackErr != nil {
		return 0, lastFallbackErr
	}

	if skipped > 0 {
		logging.Get(logging.CategoryStore).Warn(
			"Force re-embedding complete: %d vectors processed, %d left at their previous embedding", totalEmbedded, skipped)
	} else {
		logging.Store("Force re-embedding complete: %d vectors processed", totalEmbedded)
	}
	return totalEmbedded, nil
}

// =============================================================================
// TASK-TYPE AWARE VECTOR SEARCH
// =============================================================================

// VectorRecallSemanticWithTask performs vector search with explicit query task type.
// This allows using RETRIEVAL_QUERY for queries while documents use RETRIEVAL_DOCUMENT.
