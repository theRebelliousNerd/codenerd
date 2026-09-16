package store

import (
	"context"
	"fmt"
	"strings"

	"codenerd/internal/embedding"
	"codenerd/internal/logging"
)

// ReembedAllPromptAtomsForce regenerates embeddings for ALL prompt_atoms rows, overwriting existing ones.
// This is required when switching embedding providers/models.
// Returns the number of atoms re-embedded.
func (s *LocalStore) ReembedAllPromptAtomsForce(ctx context.Context) (int, error) {
	timer := logging.StartTimer(logging.CategoryStore, "ReembedAllPromptAtomsForce")
	defer timer.Stop()

	s.mu.RLock()
	engine := s.embeddingEngine
	s.mu.RUnlock()
	if engine == nil {
		logging.Get(logging.CategoryStore).Error("Cannot force re-embed prompt atoms: no embedding engine configured")
		return 0, fmt.Errorf("no embedding engine configured")
	}

	logging.Store("Starting force re-embedding prompt atoms in DB: %s", s.dbPath)

	rows, err := s.db.Query("SELECT atom_id, COALESCE(description, ''), content FROM prompt_atoms")
	if err != nil {
		// Some DBs may not have prompt_atoms (older or non-store DBs).
		logging.Get(logging.CategoryStore).Debug("Skipping prompt_atoms re-embed (query failed): %v", err)
		return 0, nil
	}

	var atoms []atomToEmbed
	for rows.Next() {
		var atomID, description, content string
		if err := rows.Scan(&atomID, &description, &content); err != nil {
			continue
		}
		text := textForAtomEmbedding(description, content)
		if text == "" {
			continue
		}
		atoms = append(atoms, atomToEmbed{atomID: atomID, text: text})
	}
	listErr := rows.Err()
	rows.Close()
	if listErr != nil {
		return 0, fmt.Errorf("list prompt atoms for re-embed: %w", listErr)
	}

	if len(atoms) == 0 {
		return 0, nil
	}

	logging.Store("Force re-embedding %d prompt atoms in DB: %s", len(atoms), s.dbPath)

	taskTypeAware, hasTaskAware := engine.(embedding.TaskTypeAwareEngine)
	taskTypeBatchAware, hasTaskBatchAware := engine.(embedding.TaskTypeBatchAwareEngine)
	expectedTask := embedding.SelectTaskType(embedding.ContentTypePromptAtom, false)

	// Embedding runs without the store lock; only the write transaction takes
	// it. Holding s.mu across the network calls froze every reader.
	batchSize := 32
	totalBatches := (len(atoms) + batchSize - 1) / batchSize
	totalEmbedded := 0
	for i := 0; i < len(atoms); i += batchSize {
		end := min(i+batchSize, len(atoms))
		batch := atoms[i:end]
		batchNum := (i / batchSize) + 1
		logging.Store("ReembedAllPromptAtomsForce [%s]: batch %d/%d (%d atoms)",
			s.dbPath, batchNum, totalBatches, len(batch))

		var embeddings [][]float32

		// If task-batch-aware, embed in batch with task type (FASTEST).
		if hasTaskBatchAware {
			texts := make([]string, len(batch))
			for j, a := range batch {
				texts[j] = a.text
			}
			vecs, err := taskTypeBatchAware.EmbedBatchWithTask(ctx, texts, expectedTask)
			if err != nil {
				return totalEmbedded, fmt.Errorf("failed to batch embed prompt atoms: %w", err)
			}
			embeddings = vecs
		} else if hasTaskAware {
			// If task-aware but not batch-aware, embed individually (SLOW).
			embeddings = make([][]float32, len(batch))
			for j, a := range batch {
				vec, err := taskTypeAware.EmbedWithTask(ctx, a.text, expectedTask)
				if err != nil {
					return totalEmbedded, fmt.Errorf("failed to embed prompt atom %s: %w", a.atomID, err)
				}
				embeddings[j] = vec
			}
		} else {
			texts := make([]string, len(batch))
			for j, a := range batch {
				texts[j] = a.text
			}
			vecs, err := engine.EmbedBatch(ctx, texts)
			if err != nil {
				logging.Get(logging.CategoryStore).Warn("Prompt atom batch embeddings failed for %s (batch %d/%d): %v; falling back to per-item embedding",
					s.dbPath, batchNum, totalBatches, err)
				vecs = make([][]float32, len(batch))
				for j, a := range batch {
					vec, embedErr := engine.Embed(ctx, a.text)
					if embedErr != nil {
						logging.Get(logging.CategoryStore).Warn("Failed to embed prompt atom %s in %s: %v", a.atomID, s.dbPath, embedErr)
						continue
					}
					vecs[j] = vec
				}
			}
			embeddings = vecs
		}

		embedded, err := s.applyPromptAtomEmbeddings(batch, embeddings, expectedTask)
		if err != nil {
			return totalEmbedded, err
		}
		totalEmbedded += embedded
	}

	logging.Store("Force re-embedding prompt atoms complete: %d atoms processed", totalEmbedded)
	return totalEmbedded, nil
}

// atomToEmbed is one prompt atom awaiting an embedding write.
type atomToEmbed struct {
	atomID string
	text   string
}

// applyPromptAtomEmbeddings writes one batch of fresh atom embeddings under
// the write lock. Empty slots (a failed per-item embed) are skipped.
func (s *LocalStore) applyPromptAtomEmbeddings(batch []atomToEmbed, embeddings [][]float32, expectedTask string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Optimization: Use transaction for batch update
	tx, err := s.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	stmt, err := tx.Prepare("UPDATE prompt_atoms SET embedding = ?, embedding_task = ? WHERE atom_id = ?")
	if err != nil {
		return 0, fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	embedded := 0
	for j, a := range batch {
		if j >= len(embeddings) || len(embeddings[j]) == 0 {
			continue
		}
		blob := encodeFloat32Slice(embeddings[j])
		if _, err := stmt.Exec(blob, expectedTask, a.atomID); err != nil {
			return embedded, fmt.Errorf("failed to update prompt atom %s: %w", a.atomID, err)
		}
		embedded++
	}

	if err := tx.Commit(); err != nil {
		return embedded, fmt.Errorf("failed to commit transaction: %w", err)
	}
	committed = true
	return embedded, nil
}

func textForAtomEmbedding(description, content string) string {
	desc := strings.TrimSpace(description)
	if desc != "" {
		return desc
	}
	c := strings.TrimSpace(content)
	// Rune-safe truncation: byte slicing can split a multi-byte rune and
	// hand the embedder invalid UTF-8.
	if r := []rune(c); len(r) > 500 {
		return string(r[:500])
	}
	return c
}
