package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"codenerd/internal/logging"
)

// TraceEmbeddingCandidate represents a trace that needs descriptor or embedding updates.
type TraceEmbeddingCandidate struct {
	ID               string
	ShardType        string
	ShardCategory    string
	TaskContext      string
	UserPrompt       string
	ErrorMessage     string
	Success          bool
	LearningNotes    []string
	SummaryDescriptor string
	DescriptorVersion int
	DescriptorHash    string
	Embedding         []byte
	EmbeddingModelID  string
	EmbeddingDim      int
	EmbeddingTask     string
	CreatedAt         time.Time
}

// TraceEmbeddingUpdate holds updated descriptor + embedding data.
type TraceEmbeddingUpdate struct {
	ID                string
	SummaryDescriptor string
	DescriptorVersion int
	DescriptorHash    string
	Embedding         []byte
	EmbeddingModelID  string
	EmbeddingDim      int
	EmbeddingTask     string
}

// ListTraceEmbeddingCandidates returns trace rows missing descriptors or embeddings.
func (ts *TraceStore) ListTraceEmbeddingCandidates(limit int, skipSuccess bool) ([]TraceEmbeddingCandidate, error) {
	timer := logging.StartTimer(logging.CategoryStore, "ListTraceEmbeddingCandidates")
	defer timer.Stop()

	ts.mu.RLock()
	defer ts.mu.RUnlock()

	if limit <= 0 {
		limit = 50
	}

	where := `
		(summary_descriptor IS NULL OR summary_descriptor = '' OR descriptor_version IS NULL OR descriptor_version != ? OR descriptor_hash IS NULL OR descriptor_hash = '')
		OR (embedding IS NULL OR length(embedding) = 0 OR embedding_model_id IS NULL OR embedding_model_id = '' OR embedding_dim IS NULL OR embedding_dim = 0 OR embedding_task IS NULL OR embedding_task = '')
	`
	if skipSuccess {
		where += " AND success = 0"
	}

	query := fmt.Sprintf(`
		SELECT id, shard_type, shard_category, COALESCE(task_context, ''), user_prompt,
		       COALESCE(error_message, ''), success, COALESCE(learning_notes, ''),
		       COALESCE(summary_descriptor, ''), COALESCE(descriptor_version, 0), COALESCE(descriptor_hash, ''),
		       embedding, COALESCE(embedding_model_id, ''), COALESCE(embedding_dim, 0), COALESCE(embedding_task, ''),
		       created_at
		FROM reasoning_traces
		WHERE %s
		ORDER BY created_at DESC
		LIMIT ?`, where)

	rows, err := ts.db.Query(query, traceDescriptorVersion, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var candidates []TraceEmbeddingCandidate
	for rows.Next() {
		var c TraceEmbeddingCandidate
		var notesJSON string
		var embedding []byte
		var createdAt sql.NullTime

		if err := rows.Scan(
			&c.ID,
			&c.ShardType,
			&c.ShardCategory,
			&c.TaskContext,
			&c.UserPrompt,
			&c.ErrorMessage,
			&c.Success,
			&notesJSON,
			&c.SummaryDescriptor,
			&c.DescriptorVersion,
			&c.DescriptorHash,
			&embedding,
			&c.EmbeddingModelID,
			&c.EmbeddingDim,
			&c.EmbeddingTask,
			&createdAt,
		); err != nil {
			continue
		}

		if notesJSON != "" {
			_ = json.Unmarshal([]byte(notesJSON), &c.LearningNotes)
		}
		if createdAt.Valid {
			c.CreatedAt = createdAt.Time
		}
		c.Embedding = embedding
		candidates = append(candidates, c)
	}
	return candidates, nil
}

// CountTraceEmbeddingBacklog returns the number of traces missing descriptors or embeddings.
func (ts *TraceStore) CountTraceEmbeddingBacklog() (int, error) {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	query := `
		SELECT COUNT(*)
		FROM reasoning_traces
		WHERE (summary_descriptor IS NULL OR summary_descriptor = '' OR descriptor_version IS NULL OR descriptor_version != ? OR descriptor_hash IS NULL OR descriptor_hash = '')
		   OR (embedding IS NULL OR length(embedding) = 0 OR embedding_model_id IS NULL OR embedding_model_id = '' OR embedding_dim IS NULL OR embedding_dim = 0 OR embedding_task IS NULL OR embedding_task = '')
	`
	var count int
	if err := ts.db.QueryRow(query, traceDescriptorVersion).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// ApplyTraceEmbeddingUpdates writes descriptor and embedding updates in a batch.
func (ts *TraceStore) ApplyTraceEmbeddingUpdates(updates []TraceEmbeddingUpdate) error {
	if len(updates) == 0 {
		return nil
	}

	ts.mu.Lock()
	defer ts.mu.Unlock()

	tx, err := ts.db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(`
		UPDATE reasoning_traces
		SET summary_descriptor = ?, descriptor_version = ?, descriptor_hash = ?,
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
			u.SummaryDescriptor,
			u.DescriptorVersion,
			u.DescriptorHash,
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

func buildTraceDescriptor(c TraceEmbeddingCandidate) string {
	var parts []string

	intent := strings.TrimSpace(c.TaskContext)
	if intent == "" {
		intent = strings.TrimSpace(c.UserPrompt)
	}
	if intent != "" {
		intent = truncateText(intent, 160)
		parts = append(parts, fmt.Sprintf("Intent: %s", intent))
	}

	shard := strings.TrimSpace(c.ShardType)
	if shard != "" {
		parts = append(parts, fmt.Sprintf("Shard: %s", shard))
	}

	fileHints := extractFileHints(c.TaskContext+" "+c.UserPrompt, 4)
	if len(fileHints) > 0 {
		parts = append(parts, fmt.Sprintf("Files: %s", strings.Join(fileHints, ", ")))
	}

	outcome := "success"
	if !c.Success {
		outcome = "failure"
	}
	parts = append(parts, fmt.Sprintf("Outcome: %s", outcome))

	keyIssue := strings.TrimSpace(c.ErrorMessage)
	if keyIssue == "" && len(c.LearningNotes) > 0 {
		keyIssue = strings.TrimSpace(c.LearningNotes[0])
	}
	if keyIssue != "" {
		keyIssue = truncateText(keyIssue, 160)
		parts = append(parts, fmt.Sprintf("Key Issue: %s", keyIssue))
	}

	desc := strings.Join(parts, " | ")
	return sanitizeDescriptor(desc)
}

func truncateText(text string, max int) string {
	if max <= 0 || len(text) <= max {
		return text
	}
	return text[:max]
}
