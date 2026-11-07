package prompt

import (
	"context"
	"database/sql"
	"encoding/binary"
	"math"
	"strings"

	"codenerd/internal/embedding"
	"codenerd/internal/logging"
)

// BackfillPromptLimit bounds one embedding batch. The compiler's background
// backfill (StartPromptVectorBackfill) runs batches until none is missing.
const BackfillPromptLimit = 200

// BackfillMissingPromptVectors embeds prompt rows whose embedding is NULL and
// stamps them with task and model. Rows already carrying a stamped vector are
// left alone. It is bounded by maxAtoms, cancellable via ctx, and logs its
// result. It mirrors backfillKnowledgeVectors: heal, do not invent rows.
func BackfillMissingPromptVectors(ctx context.Context, db *sql.DB, dbPath string, engine embedding.EmbeddingEngine, maxAtoms int) (int, error) {
	if db == nil {
		return 0, nil
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if engine == nil {
		return 0, nil
	}
	if strings.TrimSpace(engine.Name()) == "" {
		return 0, nil
	}
	if maxAtoms <= 0 {
		maxAtoms = BackfillPromptLimit
	}
	model := strings.TrimSpace(engine.Name())
	// Rows with no text to embed are excluded, so they cannot fill every batch
	// and stop a backfill that runs batches until none is left.
	rows, err := db.QueryContext(ctx, `SELECT atom_id, description, content FROM prompt_atoms WHERE (embedding IS NULL OR embedding_task IS NULL OR embedding_model IS NULL OR embedding_model != ?) AND TRIM(COALESCE(NULLIF(TRIM(description), ''), content, '')) != '' LIMIT ?`, model, maxAtoms)
	if err != nil {
		return 0, err
	}
	type pending struct {
		id   string
		text string
	}
	var work []pending
	for rows.Next() {
		var id string
		var desc, content sql.NullString
		if err := rows.Scan(&id, &desc, &content); err != nil {
			rows.Close()
			return 0, err
		}
		text := strings.TrimSpace(desc.String)
		if text == "" {
			text = content.String
		}
		if strings.TrimSpace(text) == "" {
			continue
		}
		work = append(work, pending{id: id, text: text})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()
	if len(work) == 0 {
		return 0, nil
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	taskType := embedding.SelectTaskType(embedding.ContentTypePromptAtom, false)
	texts := make([]string, len(work))
	for i, w := range work {
		texts[i] = w.text
	}
	var vecs [][]float32
	if batchAware, ok := engine.(embedding.TaskTypeBatchAwareEngine); ok && taskType != "" {
		vecs, err = batchAware.EmbedBatchWithTask(ctx, texts, taskType)
		if err != nil {
			return 0, err
		}
	} else if taskAware, ok := engine.(embedding.TaskTypeAwareEngine); ok && taskType != "" {
		vecs = make([][]float32, len(texts))
		for i, text := range texts {
			if err := ctx.Err(); err != nil {
				return 0, err
			}
			vec, embedErr := taskAware.EmbedWithTask(ctx, text, taskType)
			if embedErr != nil {
				logging.Get(logging.CategoryStore).Warn("Prompt backfill: embed %s failed: %v", work[i].id, embedErr)
				continue
			}
			vecs[i] = vec
		}
	} else {
		vecs, err = engine.EmbedBatch(ctx, texts)
		if err != nil {
			return 0, err
		}
	}
	filled := 0
	for i, w := range work {
		if err := ctx.Err(); err != nil {
			return filled, err
		}
		if i >= len(vecs) || len(vecs[i]) == 0 {
			continue
		}
		blob := make([]byte, len(vecs[i])*4)
		for j, v := range vecs[i] {
			binary.LittleEndian.PutUint32(blob[j*4:], math.Float32bits(v))
		}
		if _, err := db.ExecContext(ctx, `UPDATE prompt_atoms SET embedding = ?, embedding_task = ?, embedding_model = ? WHERE atom_id = ?`, blob, nullableBackfillString(taskType), nullableBackfillString(model), w.id); err != nil {
			return filled, err
		}
		filled++
	}
	if filled > 0 {
		label := dbPath
		if strings.TrimSpace(label) == "" {
			label = "project corpus"
		}
		logging.Get(logging.CategoryStore).Info("Prompt backfill: embedded %d atoms in %s", filled, label)
	}
	return filled, nil
}

func nullableBackfillString(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}
