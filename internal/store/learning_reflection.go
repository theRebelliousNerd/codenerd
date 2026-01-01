package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"codenerd/internal/logging"
)

// LearningEmbeddingCandidate represents a learning entry that needs descriptor or embedding updates.
type LearningEmbeddingCandidate struct {
	ID               int64
	ShardType        string
	FactPredicate    string
	FactArgs         []any
	LearnedAt        time.Time
	Confidence       float64
	SemanticHandle   string
	HandleVersion    int
	HandleHash       string
	Embedding        []byte
	EmbeddingModelID string
	EmbeddingDim     int
	EmbeddingTask    string
}

// LearningEmbeddingUpdate holds updated handle + embedding data.
type LearningEmbeddingUpdate struct {
	ID               int64
	SemanticHandle   string
	HandleVersion    int
	HandleHash       string
	Embedding        []byte
	EmbeddingModelID string
	EmbeddingDim     int
	EmbeddingTask    string
}

// ensureLearningIndexes creates indexes that depend on migrated columns.
func (ls *LearningStore) ensureLearningIndexes(db *sql.DB) error {
	if db == nil {
		return nil
	}
	_, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_learnings_handle_hash ON learnings(handle_hash);`)
	return err
}

// ListLearningEmbeddingCandidates returns learning rows missing handles or embeddings.
func (ls *LearningStore) ListLearningEmbeddingCandidates(shardType string, limit int, expectedModel string, expectedDim int, expectedTask string) ([]LearningEmbeddingCandidate, error) {
	timer := logging.StartTimer(logging.CategoryStore, "ListLearningEmbeddingCandidates")
	defer timer.Stop()

	db, err := ls.getDB(shardType)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 50
	}

	query := `
		SELECT id, shard_type, fact_predicate, fact_args, learned_at, confidence,
		       COALESCE(semantic_handle, ''), COALESCE(handle_version, 0), COALESCE(handle_hash, ''),
		       embedding, COALESCE(embedding_model_id, ''), COALESCE(embedding_dim, 0), COALESCE(embedding_task, '')
		FROM learnings
		WHERE (semantic_handle IS NULL OR semantic_handle = '' OR handle_version IS NULL OR handle_version != ? OR handle_hash IS NULL OR handle_hash = '')
		   OR (embedding IS NULL OR length(embedding) = 0 OR embedding_model_id IS NULL OR embedding_model_id = '' OR embedding_dim IS NULL OR embedding_dim = 0 OR embedding_task IS NULL OR embedding_task = '')
	`
	args := []interface{}{learningHandleVersion}
	if expectedModel != "" {
		query += " OR embedding_model_id != ?"
		args = append(args, expectedModel)
	}
	if expectedDim > 0 {
		query += " OR embedding_dim != ?"
		args = append(args, expectedDim)
	}
	if expectedTask != "" {
		query += " OR embedding_task != ?"
		args = append(args, expectedTask)
	}
	query += `
		ORDER BY learned_at DESC
		LIMIT ?`
	args = append(args, limit)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var candidates []LearningEmbeddingCandidate
	for rows.Next() {
		var c LearningEmbeddingCandidate
		var argsJSON string
		var embedding []byte
		if err := rows.Scan(
			&c.ID,
			&c.ShardType,
			&c.FactPredicate,
			&argsJSON,
			&c.LearnedAt,
			&c.Confidence,
			&c.SemanticHandle,
			&c.HandleVersion,
			&c.HandleHash,
			&embedding,
			&c.EmbeddingModelID,
			&c.EmbeddingDim,
			&c.EmbeddingTask,
		); err != nil {
			continue
		}
		if argsJSON != "" {
			_ = json.Unmarshal([]byte(argsJSON), &c.FactArgs)
		}
		c.Embedding = embedding
		candidates = append(candidates, c)
	}
	return candidates, nil
}

// ListAllLearningEmbeddingCandidates returns learning rows regardless of embedding state.
func (ls *LearningStore) ListAllLearningEmbeddingCandidates(shardType string, limit int, offset int) ([]LearningEmbeddingCandidate, error) {
	timer := logging.StartTimer(logging.CategoryStore, "ListAllLearningEmbeddingCandidates")
	defer timer.Stop()

	db, err := ls.getDB(shardType)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 100
	}

	query := `
		SELECT id, shard_type, fact_predicate, fact_args, learned_at, confidence,
		       COALESCE(semantic_handle, ''), COALESCE(handle_version, 0), COALESCE(handle_hash, ''),
		       embedding, COALESCE(embedding_model_id, ''), COALESCE(embedding_dim, 0), COALESCE(embedding_task, '')
		FROM learnings
		ORDER BY learned_at DESC
		LIMIT ? OFFSET ?`

	rows, err := db.Query(query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var candidates []LearningEmbeddingCandidate
	for rows.Next() {
		var c LearningEmbeddingCandidate
		var argsJSON string
		var embedding []byte
		if err := rows.Scan(
			&c.ID,
			&c.ShardType,
			&c.FactPredicate,
			&argsJSON,
			&c.LearnedAt,
			&c.Confidence,
			&c.SemanticHandle,
			&c.HandleVersion,
			&c.HandleHash,
			&embedding,
			&c.EmbeddingModelID,
			&c.EmbeddingDim,
			&c.EmbeddingTask,
		); err != nil {
			continue
		}
		if argsJSON != "" {
			_ = json.Unmarshal([]byte(argsJSON), &c.FactArgs)
		}
		c.Embedding = embedding
		candidates = append(candidates, c)
	}
	return candidates, nil
}

// CountLearningEmbeddingBacklog returns count of learnings missing handles or embeddings.
func (ls *LearningStore) CountLearningEmbeddingBacklog(shardType string, expectedModel string, expectedDim int, expectedTask string) (int, error) {
	db, err := ls.getDB(shardType)
	if err != nil {
		return 0, err
	}
	query := `
		SELECT COUNT(*)
		FROM learnings
		WHERE (semantic_handle IS NULL OR semantic_handle = '' OR handle_version IS NULL OR handle_version != ? OR handle_hash IS NULL OR handle_hash = '')
		   OR (embedding IS NULL OR length(embedding) = 0 OR embedding_model_id IS NULL OR embedding_model_id = '' OR embedding_dim IS NULL OR embedding_dim = 0 OR embedding_task IS NULL OR embedding_task = '')
	`
	args := []interface{}{learningHandleVersion}
	if expectedModel != "" {
		query += " OR embedding_model_id != ?"
		args = append(args, expectedModel)
	}
	if expectedDim > 0 {
		query += " OR embedding_dim != ?"
		args = append(args, expectedDim)
	}
	if expectedTask != "" {
		query += " OR embedding_task != ?"
		args = append(args, expectedTask)
	}
	var count int
	if err := db.QueryRow(query, args...).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// ApplyLearningEmbeddingUpdates writes handle + embedding updates in a batch.
func (ls *LearningStore) ApplyLearningEmbeddingUpdates(shardType string, updates []LearningEmbeddingUpdate) error {
	if len(updates) == 0 {
		return nil
	}

	db, err := ls.getDB(shardType)
	if err != nil {
		return err
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(`
		UPDATE learnings
		SET semantic_handle = ?, handle_version = ?, handle_hash = ?,
		    embedding = ?, embedding_model_id = ?, embedding_dim = ?, embedding_task = ?
		WHERE id = ?
	`)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer stmt.Close()

	for _, u := range updates {
		if _, err := stmt.Exec(
			u.SemanticHandle,
			u.HandleVersion,
			u.HandleHash,
			u.Embedding,
			u.EmbeddingModelID,
			u.EmbeddingDim,
			u.EmbeddingTask,
			u.ID,
		); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func buildLearningHandle(shardType, predicate string, args []any) string {
	predicate = strings.TrimSpace(predicate)
	shardType = strings.TrimSpace(shardType)
	argStr := joinArgs(args)

	var handle string
	switch predicate {
	case "avoid_pattern":
		if len(args) >= 2 {
			handle = fmt.Sprintf("Avoid %v because %v", args[0], args[1])
		} else if len(args) >= 1 {
			handle = fmt.Sprintf("Avoid %v", args[0])
		}
	case "preferred_pattern":
		if len(args) >= 1 {
			handle = fmt.Sprintf("Prefer %v", args[0])
		}
	case "style_preference":
		if len(args) >= 1 {
			handle = fmt.Sprintf("Style preference: %v", args[0])
		}
	case "domain_expertise":
		if len(args) >= 1 {
			handle = fmt.Sprintf("Domain focus: %v", args[0])
		}
	case "tool_preference":
		if len(args) >= 2 {
			handle = fmt.Sprintf("For %v, use %v", args[0], args[1])
		}
	default:
		if predicate != "" && argStr != "" {
			handle = fmt.Sprintf("%s: %s", predicate, argStr)
		} else if predicate != "" {
			handle = predicate
		} else {
			handle = argStr
		}
	}

	if shardType != "" {
		handle = fmt.Sprintf("[%s] %s", shardType, handle)
	}

	return sanitizeDescriptor(handle)
}

func joinArgs(args []any) string {
	if len(args) == 0 {
		return ""
	}
	parts := make([]string, 0, len(args))
	for _, arg := range args {
		parts = append(parts, fmt.Sprintf("%v", arg))
	}
	return strings.Join(parts, ", ")
}
