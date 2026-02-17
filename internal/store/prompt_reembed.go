package store

import (
	"context"
	"fmt"
	"math"
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

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.embeddingEngine == nil {
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
	defer rows.Close()

	type atomToEmbed struct {
		atomID string
		text   string
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

	if len(atoms) == 0 {
		return 0, nil
	}

	logging.Store("Force re-embedding %d prompt atoms in DB: %s", len(atoms), s.dbPath)

	taskTypeAware, hasTaskAware := s.embeddingEngine.(embedding.TaskTypeAwareEngine)
	expectedTask := embedding.SelectTaskType(embedding.ContentTypePromptAtom, false)

	batchSize := 32
	totalBatches := (len(atoms) + batchSize - 1) / batchSize
	totalEmbedded := 0
	for i := 0; i < len(atoms); i += batchSize {
		end := int(math.Min(float64(i+batchSize), float64(len(atoms))))
		batch := atoms[i:end]
		batchNum := (i / batchSize) + 1
		logging.Store("ReembedAllPromptAtomsForce [%s]: batch %d/%d (%d atoms)",
			s.dbPath, batchNum, totalBatches, len(batch))

		embeddings := make([][]float32, len(batch))

		// If task-aware, embed individually with the prompt atom task type.
		if hasTaskAware {
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
			vecs, err := s.embeddingEngine.EmbedBatch(ctx, texts)
			if err != nil {
				logging.Get(logging.CategoryStore).Warn("Prompt atom batch embeddings failed for %s (batch %d/%d): %v; falling back to per-item embedding",
					s.dbPath, batchNum, totalBatches, err)
				vecs = make([][]float32, len(batch))
				for j, a := range batch {
					vec, embedErr := s.embeddingEngine.Embed(ctx, a.text)
					if embedErr != nil {
						logging.Get(logging.CategoryStore).Warn("Failed to embed prompt atom %s in %s: %v", a.atomID, s.dbPath, embedErr)
						continue
					}
					vecs[j] = vec
				}
			}
			embeddings = vecs
		}

		// Optimization: Use transaction for batch update
		tx, err := s.db.Begin()
		if err != nil {
			return totalEmbedded, fmt.Errorf("failed to begin transaction: %w", err)
		}

		stmt, err := tx.Prepare("UPDATE prompt_atoms SET embedding = ?, embedding_task = ? WHERE atom_id = ?")
		if err != nil {
			tx.Rollback()
			return totalEmbedded, fmt.Errorf("failed to prepare statement: %w", err)
		}
		defer stmt.Close()

		for j, a := range batch {
			if j >= len(embeddings) || embeddings[j] == nil || len(embeddings[j]) == 0 {
				continue
			}
			blob := encodeFloat32Slice(embeddings[j])
			_, err := stmt.Exec(blob, expectedTask, a.atomID)
			if err != nil {
				tx.Rollback()
				return totalEmbedded, fmt.Errorf("failed to update prompt atom %s: %w", a.atomID, err)
			}
			totalEmbedded++
		}

		if err := tx.Commit(); err != nil {
			return totalEmbedded, fmt.Errorf("failed to commit transaction: %w", err)
		}
	}

	logging.Store("Force re-embedding prompt atoms complete: %d atoms processed", totalEmbedded)
	return totalEmbedded, nil
}

func textForAtomEmbedding(description, content string) string {
	desc := strings.TrimSpace(description)
	if desc != "" {
		return desc
	}
	c := strings.TrimSpace(content)
	if len(c) > 500 {
		c = c[:500]
	}
	return c
}
