package store

import (
	"codenerd/internal/embedding"
	"codenerd/internal/logging"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
)

// Useful for migrating from keyword-only to embedding-based search.
// Returns nil if no vectors need re-embedding.
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
	rows, err := s.db.Query("SELECT id, content, metadata FROM vectors WHERE embedding IS NULL")
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to query vectors for re-embedding: %v", err)
		return err
	}
	defer rows.Close()

	type vectorToEmbed struct {
		id       int64
		content  string
		metadata string
	}

	var vectors []vectorToEmbed
	for rows.Next() {
		var v vectorToEmbed
		if err := rows.Scan(&v.id, &v.content, &v.metadata); err != nil {
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
		tx, err := s.db.Begin()
		if err != nil {
			logging.Get(logging.CategoryStore).Error("Failed to start transaction: %v", err)
			return fmt.Errorf("failed to start transaction: %w", err)
		}

		updateStmt, err := tx.Prepare("UPDATE vectors SET embedding = ? WHERE id = ?")
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to prepare update stmt: %w", err)
		}

		var insertVecStmt *sql.Stmt
		if s.vectorExt {
			insertVecStmt, err = tx.Prepare("INSERT OR REPLACE INTO vec_index (rowid, embedding, content, metadata) VALUES (?, ?, ?, ?)")
			if err != nil {
				updateStmt.Close()
				tx.Rollback()
				return fmt.Errorf("failed to prepare vec_index stmt: %w", err)
			}
		}

		for j, v := range batch {
			embeddingJSON, _ := json.Marshal(embeddings[j])
			_, err := updateStmt.Exec(string(embeddingJSON), v.id)
			if err != nil {
				updateStmt.Close()
				if insertVecStmt != nil {
					insertVecStmt.Close()
				}
				tx.Rollback()
				logging.Get(logging.CategoryStore).Error("Failed to update vector %d: %v", v.id, err)
				return fmt.Errorf("failed to update vector %d: %w", v.id, err)
			}
			// Keep sqlite-vec index in sync when available.
			if s.vectorExt {
				vecBlob := encodeFloat32Slice(embeddings[j])
				_, err = insertVecStmt.Exec(v.id, vecBlob, v.content, v.metadata)
				if err != nil {
					updateStmt.Close()
					insertVecStmt.Close()
					tx.Rollback()
					return fmt.Errorf("failed to update vec_index for vector %d: %w", v.id, err)
				}
			}
			totalEmbedded++
		}

		updateStmt.Close()
		if insertVecStmt != nil {
			insertVecStmt.Close()
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit transaction: %w", err)
		}
	}

	logging.Store("Re-embedding complete: %d vectors processed", totalEmbedded)
	return nil
}

// ReembedAllVectorsForce regenerates embeddings for ALL vectors, overwriting existing ones.
// This is required when switching embedding providers/models.
// Returns the number of vectors re-embedded.
func (s *LocalStore) ReembedAllVectorsForce(ctx context.Context) (int, error) {
	timer := logging.StartTimer(logging.CategoryStore, "ReembedAllVectorsForce")
	defer timer.Stop()

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.embeddingEngine == nil {
		logging.Get(logging.CategoryStore).Error("Cannot force re-embed: no embedding engine configured")
		return 0, fmt.Errorf("no embedding engine configured")
	}

	logging.Store("Starting force re-embedding of all vectors in DB: %s", s.dbPath)

	rows, err := s.db.Query("SELECT id, content, metadata FROM vectors")
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to query vectors for force re-embedding: %v", err)
		return 0, err
	}
	defer rows.Close()

	type vectorToEmbed struct {
		id       int64
		content  string
		metadata string
	}

	var vectors []vectorToEmbed
	for rows.Next() {
		var v vectorToEmbed
		if err := rows.Scan(&v.id, &v.content, &v.metadata); err != nil {
			continue
		}
		vectors = append(vectors, v)
	}

	if len(vectors) == 0 {
		logging.StoreDebug("No vectors found for force re-embedding")
		return 0, nil
	}

	logging.Store("Found %d vectors to force re-embed", len(vectors))

	batchSize := 32
	totalBatches := (len(vectors) + batchSize - 1) / batchSize
	totalEmbedded := 0
	var lastFallbackErr error
	for i := 0; i < len(vectors); i += batchSize {
		end := int(math.Min(float64(i+batchSize), float64(len(vectors))))
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
			if batchAware, ok := s.embeddingEngine.(embedding.TaskTypeBatchAwareEngine); ok {
				embeddings, err = batchAware.EmbedBatchWithTask(ctx, texts, taskTypes[0])
			} else if taskAware, ok := s.embeddingEngine.(embedding.TaskTypeAwareEngine); ok {
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
				embeddings, err = s.embeddingEngine.EmbedBatch(ctx, texts)
			}
		} else if taskAware, ok := s.embeddingEngine.(embedding.TaskTypeAwareEngine); ok {
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
			embeddings, err = s.embeddingEngine.EmbedBatch(ctx, texts)
		}

		if err != nil {
			logging.Get(logging.CategoryStore).Warn("Force batch embeddings failed for %s (batch %d/%d): %v; falling back to per-item embedding",
				s.dbPath, batchNum, totalBatches, err)
			embeddings = make([][]float32, len(batch))
			for j, v := range batch {
				var vec []float32
				var embedErr error
				if taskAware, ok := s.embeddingEngine.(embedding.TaskTypeAwareEngine); ok {
					vec, embedErr = taskAware.EmbedWithTask(ctx, v.content, taskTypes[j])
				} else {
					vec, embedErr = s.embeddingEngine.Embed(ctx, v.content)
				}
				if embedErr != nil {
					logging.Get(logging.CategoryStore).Warn("Failed to embed vector %d in %s: %v", v.id, s.dbPath, embedErr)
					lastFallbackErr = embedErr
					continue
				}
				embeddings[j] = vec
			}
		}

		tx, err := s.db.Begin()
		if err != nil {
			logging.Get(logging.CategoryStore).Error("Failed to start transaction: %v", err)
			return totalEmbedded, fmt.Errorf("failed to start transaction: %w", err)
		}

		updateStmt, err := tx.Prepare("UPDATE vectors SET embedding = ? WHERE id = ?")
		if err != nil {
			tx.Rollback()
			return totalEmbedded, fmt.Errorf("failed to prepare update stmt: %w", err)
		}

		var insertVecStmt *sql.Stmt
		if s.vectorExt {
			insertVecStmt, err = tx.Prepare("INSERT OR REPLACE INTO vec_index (embedding, content, metadata) VALUES (?, ?, ?)")
			if err != nil {
				updateStmt.Close()
				tx.Rollback()
				return totalEmbedded, fmt.Errorf("failed to prepare vec_index stmt: %w", err)
			}
		}

		for j, v := range batch {
			if j >= len(embeddings) || embeddings[j] == nil || len(embeddings[j]) == 0 {
				continue
			}
			embeddingJSON, _ := json.Marshal(embeddings[j])
			_, err := updateStmt.Exec(string(embeddingJSON), v.id)
			if err != nil {
				updateStmt.Close()
				if insertVecStmt != nil {
					insertVecStmt.Close()
				}
				tx.Rollback()
				logging.Get(logging.CategoryStore).Error("Failed to update vector %d: %v", v.id, err)
				return totalEmbedded, fmt.Errorf("failed to update vector %d: %w", v.id, err)
			}
			if s.vectorExt {
				vecBlob := encodeFloat32Slice(embeddings[j])
				_, err = insertVecStmt.Exec(vecBlob, v.content, v.metadata)
				if err != nil {
					updateStmt.Close()
					insertVecStmt.Close()
					tx.Rollback()
					return totalEmbedded, fmt.Errorf("failed to update vec_index for vector %d: %w", v.id, err)
				}
			}
			totalEmbedded++
		}

		updateStmt.Close()
		if insertVecStmt != nil {
			insertVecStmt.Close()
		}

		if err := tx.Commit(); err != nil {
			return totalEmbedded, fmt.Errorf("failed to commit transaction: %w", err)
		}
	}

	if totalEmbedded == 0 && lastFallbackErr != nil {
		return 0, lastFallbackErr
	}

	logging.Store("Force re-embedding complete: %d vectors processed", totalEmbedded)
	return totalEmbedded, nil
}

// =============================================================================
// TASK-TYPE AWARE VECTOR SEARCH
// =============================================================================

// VectorRecallSemanticWithTask performs vector search with explicit query task type.
// This allows using RETRIEVAL_QUERY for queries while documents use RETRIEVAL_DOCUMENT.
