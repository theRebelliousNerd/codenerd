package store

import (
	"codenerd/internal/logging"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// TraceStore provides persistence for reasoning traces with self-learning capabilities.
// This enables shards to learn from their own execution history across sessions.
//
// Architecture:
// - Implements perception.TraceStore for write operations
// - Implements perception.ShardTraceReader for read operations
// - Backed by SQLite for durability
// - Thread-safe with read-write mutex
type TraceStore struct {
	db     *sql.DB
	mu     sync.RWMutex
	dbPath string
}

// ReasoningTrace represents a captured LLM interaction for learning.
// Mirrors perception.ReasoningTrace to avoid import cycles.
type ReasoningTrace struct {
	ID                string    `json:"id"`
	ShardID           string    `json:"shard_id"`
	ShardType         string    `json:"shard_type"`
	ShardCategory     string    `json:"shard_category"` // system, ephemeral, specialist
	SessionID         string    `json:"session_id"`
	TaskContext       string    `json:"task_context"`
	SystemPrompt      string    `json:"system_prompt"`
	UserPrompt        string    `json:"user_prompt"`
	Response          string    `json:"response"`
	Model             string    `json:"model,omitempty"`
	TokensUsed        int       `json:"tokens_used,omitempty"`
	DurationMs        int64     `json:"duration_ms"`
	Success           bool      `json:"success"`
	ErrorMessage      string    `json:"error_message,omitempty"`
	QualityScore      float64   `json:"quality_score,omitempty"`
	LearningNotes     []string  `json:"learning_notes,omitempty"`
	SummaryDescriptor string    `json:"summary_descriptor,omitempty"`
	DescriptorVersion int       `json:"descriptor_version,omitempty"`
	DescriptorHash    string    `json:"descriptor_hash,omitempty"`
	Embedding         []byte    `json:"embedding,omitempty"`
	EmbeddingModelID  string    `json:"embedding_model_id,omitempty"`
	EmbeddingDim      int       `json:"embedding_dim,omitempty"`
	EmbeddingTask     string    `json:"embedding_task,omitempty"`
	CreatedAt         time.Time `json:"timestamp"`
}

// NewTraceStore creates a new TraceStore using an existing database connection.
// The database must already have the reasoning_traces table created.
func NewTraceStore(db *sql.DB, dbPath string) (*TraceStore, error) {
	logging.StoreDebug("Initializing TraceStore at path: %s", dbPath)

	store := &TraceStore{
		db:     db,
		dbPath: dbPath,
	}

	// Ensure the table exists
	if err := store.ensureSchema(); err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to ensure trace schema: %v", err)
		return nil, fmt.Errorf("failed to ensure trace schema: %w", err)
	}

	logging.Store("TraceStore initialized for self-learning persistence")
	return store, nil
}

// ensureSchema creates the reasoning_traces table if it doesn't exist.
func (ts *TraceStore) ensureSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS reasoning_traces (
		id TEXT PRIMARY KEY,
		shard_id TEXT NOT NULL,
		shard_type TEXT NOT NULL,
		shard_category TEXT NOT NULL,
		session_id TEXT NOT NULL,
		task_context TEXT,
		system_prompt TEXT NOT NULL,
		user_prompt TEXT NOT NULL,
		response TEXT NOT NULL,
		model TEXT,
		tokens_used INTEGER,
		duration_ms INTEGER,
		success BOOLEAN NOT NULL,
		error_message TEXT,
		quality_score REAL,
		learning_notes TEXT,
		summary_descriptor TEXT,
		descriptor_version INTEGER DEFAULT 0,
		descriptor_hash TEXT,
		embedding BLOB,
		embedding_model_id TEXT,
		embedding_dim INTEGER,
		embedding_task TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_traces_shard_type ON reasoning_traces(shard_type);
	CREATE INDEX IF NOT EXISTS idx_traces_session ON reasoning_traces(session_id);
	CREATE INDEX IF NOT EXISTS idx_traces_shard_id ON reasoning_traces(shard_id);
	CREATE INDEX IF NOT EXISTS idx_traces_success ON reasoning_traces(success);
	CREATE INDEX IF NOT EXISTS idx_traces_created ON reasoning_traces(created_at);
	CREATE INDEX IF NOT EXISTS idx_traces_category ON reasoning_traces(shard_category);
	CREATE INDEX IF NOT EXISTS idx_traces_descriptor_hash ON reasoning_traces(descriptor_hash);
	`

	_, err := ts.db.Exec(schema)
	return err
}

// ========== Write Operations (perception.TraceStore interface) ==========

// StoreReasoningTrace persists a reasoning trace for later analysis.
// This is called asynchronously by TracingLLMClient after each LLM interaction.
//
// Accepts *ReasoningTrace directly - the LocalStore wrapper handles reflection
// for perception.ReasoningTrace to avoid import cycles.
func (ts *TraceStore) StoreReasoningTrace(trace *ReasoningTrace) error {
	timer := logging.StartTimer(logging.CategoryStore, "StoreReasoningTrace")
	defer timer.Stop()

	ts.mu.Lock()
	defer ts.mu.Unlock()

	logging.StoreDebug("Storing reasoning trace: id=%s shard=%s type=%s success=%v", trace.ID, trace.ShardID, trace.ShardType, trace.Success)

	notesJSON, _ := json.Marshal(trace.LearningNotes)

	_, err := ts.db.Exec(`
		INSERT OR REPLACE INTO reasoning_traces
		(id, shard_id, shard_type, shard_category, session_id, task_context,
		 system_prompt, user_prompt, response, model, tokens_used, duration_ms,
		 success, error_message, quality_score, learning_notes,
		 summary_descriptor, descriptor_version, descriptor_hash,
		 embedding, embedding_model_id, embedding_dim, embedding_task)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		trace.ID, trace.ShardID, trace.ShardType, trace.ShardCategory,
		trace.SessionID, trace.TaskContext, trace.SystemPrompt, trace.UserPrompt,
		trace.Response, trace.Model, trace.TokensUsed, trace.DurationMs,
		trace.Success, trace.ErrorMessage, trace.QualityScore, string(notesJSON),
		trace.SummaryDescriptor, trace.DescriptorVersion, trace.DescriptorHash,
		trace.Embedding, trace.EmbeddingModelID, trace.EmbeddingDim, trace.EmbeddingTask,
	)

	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to store reasoning trace %s: %v", trace.ID, err)
		return err
	}

	logging.StoreDebug("Reasoning trace stored: %s (duration=%dms, tokens=%d)", trace.ID, trace.DurationMs, trace.TokensUsed)
	return nil
}

// storeReasoningTraceRaw is the internal method that handles raw parameter storage.
// Used by LocalStore after reflection conversion.
func (ts *TraceStore) storeReasoningTraceRaw(
	id, shardID, shardType, shardCategory, sessionID, taskContext,
	systemPrompt, userPrompt, response, model, errorMessage string,
	tokensUsed int, durationMs int64, success bool,
	qualityScore float64, learningNotes []string,
	summaryDescriptor string, descriptorVersion int, descriptorHash string,
	embedding []byte, embeddingModelID string, embeddingDim int, embeddingTask string,
) error {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	notesJSON, _ := json.Marshal(learningNotes)

	_, err := ts.db.Exec(`
		INSERT OR REPLACE INTO reasoning_traces
		(id, shard_id, shard_type, shard_category, session_id, task_context,
		 system_prompt, user_prompt, response, model, tokens_used, duration_ms,
		 success, error_message, quality_score, learning_notes,
		 summary_descriptor, descriptor_version, descriptor_hash,
		 embedding, embedding_model_id, embedding_dim, embedding_task)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, shardID, shardType, shardCategory, sessionID, taskContext,
		systemPrompt, userPrompt, response, model, tokensUsed, durationMs,
		success, errorMessage, qualityScore, string(notesJSON),
		summaryDescriptor, descriptorVersion, descriptorHash,
		embedding, embeddingModelID, embeddingDim, embeddingTask,
	)

	return err
}

// ========== Read Operations (perception.ShardTraceReader interface) ==========

// GetShardTraces retrieves recent traces for a specific shard type.
// Used by shards to learn from their own past behavior.
func (ts *TraceStore) GetShardTraces(shardType string, limit int) ([]ReasoningTrace, error) {
	timer := logging.StartTimer(logging.CategoryStore, "GetShardTraces")
	defer timer.Stop()

	ts.mu.RLock()
	defer ts.mu.RUnlock()

	if limit <= 0 {
		limit = 50
	}

	logging.StoreDebug("Retrieving traces for shard type=%s (limit=%d)", shardType, limit)

	rows, err := ts.db.Query(`
		SELECT id, shard_id, shard_type, shard_category, session_id, task_context,
		       system_prompt, user_prompt, response, model, tokens_used, duration_ms,
		       success, error_message, quality_score, learning_notes, created_at
		FROM reasoning_traces
		WHERE shard_type = ?
		ORDER BY created_at DESC
		LIMIT ?`, shardType, limit)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to retrieve traces for shard=%s: %v", shardType, err)
		return nil, err
	}
	defer rows.Close()

	traces, err := ts.scanTraces(rows)
	if err == nil {
		logging.StoreDebug("Retrieved %d traces for shard type=%s", len(traces), shardType)
	}
	return traces, err
}

// GetFailedShardTraces retrieves failed traces for learning from errors.
// Critical for self-improvement - shards can analyze what went wrong.
func (ts *TraceStore) GetFailedShardTraces(shardType string, limit int) ([]ReasoningTrace, error) {
	timer := logging.StartTimer(logging.CategoryStore, "GetFailedShardTraces")
	defer timer.Stop()

	ts.mu.RLock()
	defer ts.mu.RUnlock()

	if limit <= 0 {
		limit = 20
	}

	logging.StoreDebug("Retrieving failed traces for shard type=%s (limit=%d)", shardType, limit)

	rows, err := ts.db.Query(`
		SELECT id, shard_id, shard_type, shard_category, session_id, task_context,
		       system_prompt, user_prompt, response, model, tokens_used, duration_ms,
		       success, error_message, quality_score, learning_notes, created_at
		FROM reasoning_traces
		WHERE shard_type = ? AND success = 0
		ORDER BY created_at DESC
		LIMIT ?`, shardType, limit)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to retrieve failed traces for shard=%s: %v", shardType, err)
		return nil, err
	}
	defer rows.Close()

	traces, err := ts.scanTraces(rows)
	if err == nil {
		logging.StoreDebug("Retrieved %d failed traces for shard type=%s", len(traces), shardType)
	}
	return traces, err
}

// GetSimilarTaskTraces finds traces for tasks matching a pattern.
// Enables shards to recall how they handled similar situations before.
func (ts *TraceStore) GetSimilarTaskTraces(shardType, taskPattern string, limit int) ([]ReasoningTrace, error) {
	timer := logging.StartTimer(logging.CategoryStore, "GetSimilarTaskTraces")
	defer timer.Stop()

	ts.mu.RLock()
	defer ts.mu.RUnlock()

	if limit <= 0 {
		limit = 10
	}

	logging.StoreDebug("Finding similar task traces: shard=%s pattern=%q limit=%d", shardType, taskPattern, limit)

	// Simple pattern matching - could be enhanced with FTS or vector similarity
	rows, err := ts.db.Query(`
		SELECT id, shard_id, shard_type, shard_category, session_id, task_context,
		       system_prompt, user_prompt, response, model, tokens_used, duration_ms,
		       success, error_message, quality_score, learning_notes, created_at
		FROM reasoning_traces
		WHERE shard_type = ? AND task_context LIKE ?
		ORDER BY created_at DESC
		LIMIT ?`, shardType, "%"+taskPattern+"%", limit)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to find similar task traces: %v", err)
		return nil, err
	}
	defer rows.Close()

	traces, err := ts.scanTraces(rows)
	if err == nil {
		logging.StoreDebug("Found %d similar task traces for pattern=%q", len(traces), taskPattern)
	}
	return traces, err
}

// GetHighQualityTraces retrieves successful traces with high quality scores.
// Used to identify and learn from best practices and successful patterns.
func (ts *TraceStore) GetHighQualityTraces(shardType string, minScore float64, limit int) ([]ReasoningTrace, error) {
	timer := logging.StartTimer(logging.CategoryStore, "GetHighQualityTraces")
	defer timer.Stop()

	ts.mu.RLock()
	defer ts.mu.RUnlock()

	if limit <= 0 {
		limit = 20
	}

	logging.StoreDebug("Retrieving high-quality traces: shard=%s minScore=%.2f limit=%d", shardType, minScore, limit)

	rows, err := ts.db.Query(`
		SELECT id, shard_id, shard_type, shard_category, session_id, task_context,
		       system_prompt, user_prompt, response, model, tokens_used, duration_ms,
		       success, error_message, quality_score, learning_notes, created_at
		FROM reasoning_traces
		WHERE shard_type = ? AND success = 1 AND quality_score >= ?
		ORDER BY quality_score DESC, created_at DESC
		LIMIT ?`, shardType, minScore, limit)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to retrieve high-quality traces: %v", err)
		return nil, err
	}
	defer rows.Close()

	traces, err := ts.scanTraces(rows)
	if err == nil {
		logging.StoreDebug("Retrieved %d high-quality traces for shard=%s", len(traces), shardType)
	}
	return traces, err
}

// ========== Additional Query Methods ==========

// GetRecentTraces retrieves recent traces across all shards.
// Used by the main agent for system-wide oversight.
func (ts *TraceStore) GetRecentTraces(limit int) ([]ReasoningTrace, error) {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	if limit <= 0 {
		limit = 100
	}

	rows, err := ts.db.Query(`
		SELECT id, shard_id, shard_type, shard_category, session_id, task_context,
		       system_prompt, user_prompt, response, model, tokens_used, duration_ms,
		       success, error_message, quality_score, learning_notes, created_at
		FROM reasoning_traces
		ORDER BY created_at DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return ts.scanTraces(rows)
}

// GetTracesBySession retrieves all traces for a specific session.
// Useful for session replay and debugging.
func (ts *TraceStore) GetTracesBySession(sessionID string) ([]ReasoningTrace, error) {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	rows, err := ts.db.Query(`
		SELECT id, shard_id, shard_type, shard_category, session_id, task_context,
		       system_prompt, user_prompt, response, model, tokens_used, duration_ms,
		       success, error_message, quality_score, learning_notes, created_at
		FROM reasoning_traces
		WHERE session_id = ?
		ORDER BY created_at ASC`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return ts.scanTraces(rows)
}

// GetTracesByCategory retrieves traces by shard category (system, ephemeral, specialist).
// Enables category-specific analysis and learning.
func (ts *TraceStore) GetTracesByCategory(category string, limit int) ([]ReasoningTrace, error) {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	if limit <= 0 {
		limit = 50
	}

	rows, err := ts.db.Query(`
		SELECT id, shard_id, shard_type, shard_category, session_id, task_context,
		       system_prompt, user_prompt, response, model, tokens_used, duration_ms,
		       success, error_message, quality_score, learning_notes, created_at
		FROM reasoning_traces
		WHERE shard_category = ?
		ORDER BY created_at DESC
		LIMIT ?`, category, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return ts.scanTraces(rows)
}

// ========== Quality Tracking ==========

// UpdateTraceQuality updates the quality score and learning notes for a trace.
// Called after post-execution analysis or user feedback.
func (ts *TraceStore) UpdateTraceQuality(traceID string, score float64, notes []string) error {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	logging.StoreDebug("Updating trace quality: id=%s score=%.2f notes=%d", traceID, score, len(notes))

	notesJSON, _ := json.Marshal(notes)

	_, err := ts.db.Exec(`
		UPDATE reasoning_traces
		SET quality_score = ?, learning_notes = ?
		WHERE id = ?`, score, string(notesJSON), traceID)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to update trace quality for %s: %v", traceID, err)
		return err
	}

	logging.StoreDebug("Trace quality updated: %s -> %.2f", traceID, score)
	return nil
}

// GetTraceStats returns comprehensive statistics about reasoning traces.
// Provides insights into agent performance and learning patterns.
func (ts *TraceStore) GetTraceStats() (map[string]interface{}, error) {
	timer := logging.StartTimer(logging.CategoryStore, "GetTraceStats")
	defer timer.Stop()

	ts.mu.RLock()
	defer ts.mu.RUnlock()

	logging.StoreDebug("Computing trace statistics")

	stats := make(map[string]interface{})

	// Total traces
	var totalCount int64
	ts.db.QueryRow("SELECT COUNT(*) FROM reasoning_traces").Scan(&totalCount)
	stats["total_traces"] = totalCount

	// Success rate
	var successCount int64
	ts.db.QueryRow("SELECT COUNT(*) FROM reasoning_traces WHERE success = 1").Scan(&successCount)
	if totalCount > 0 {
		stats["success_rate"] = float64(successCount) / float64(totalCount)
	}

	// Average duration
	var avgDuration float64
	ts.db.QueryRow("SELECT AVG(duration_ms) FROM reasoning_traces").Scan(&avgDuration)
	stats["avg_duration_ms"] = avgDuration

	// Average quality score
	var avgQuality float64
	ts.db.QueryRow("SELECT AVG(quality_score) FROM reasoning_traces WHERE quality_score > 0").Scan(&avgQuality)
	stats["avg_quality_score"] = avgQuality

	// Traces by category
	categoryStats := make(map[string]int64)
	rows, err := ts.db.Query("SELECT shard_category, COUNT(*) FROM reasoning_traces GROUP BY shard_category")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var category string
			var count int64
			if rows.Scan(&category, &count) == nil {
				categoryStats[category] = count
			}
		}
	}
	stats["by_category"] = categoryStats

	// Top shard types by volume
	shardStats := make(map[string]int64)
	rows2, err := ts.db.Query("SELECT shard_type, COUNT(*) FROM reasoning_traces GROUP BY shard_type ORDER BY COUNT(*) DESC LIMIT 10")
	if err == nil {
		defer rows2.Close()
		for rows2.Next() {
			var shardType string
			var count int64
			if rows2.Scan(&shardType, &count) == nil {
				shardStats[shardType] = count
			}
		}
	}
	stats["by_shard_type"] = shardStats

	// Success rate by shard type
	successByType := make(map[string]float64)
	rows3, err := ts.db.Query(`
		SELECT shard_type,
		       CAST(SUM(CASE WHEN success = 1 THEN 1 ELSE 0 END) AS REAL) / COUNT(*) as rate
		FROM reasoning_traces
		GROUP BY shard_type
		HAVING COUNT(*) >= 5
		ORDER BY rate DESC
	`)
	if err == nil {
		defer rows3.Close()
		for rows3.Next() {
			var shardType string
			var rate float64
			if rows3.Scan(&shardType, &rate) == nil {
				successByType[shardType] = rate
			}
		}
	}
	stats["success_rate_by_type"] = successByType

	return stats, nil
}

// GetFailurePatterns analyzes failed traces to identify common failure patterns.
// Returns patterns grouped by error type with frequency counts.
func (ts *TraceStore) GetFailurePatterns(limit int) (map[string]int, error) {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	if limit <= 0 {
		limit = 50
	}

	rows, err := ts.db.Query(`
		SELECT error_message, COUNT(*) as count
		FROM reasoning_traces
		WHERE success = 0 AND error_message != ''
		GROUP BY error_message
		ORDER BY count DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	patterns := make(map[string]int)
	for rows.Next() {
		var errMsg string
		var count int
		if rows.Scan(&errMsg, &count) == nil {
			patterns[errMsg] = count
		}
	}

	return patterns, nil
}

// GetLearningInsights generates actionable learning insights from trace history.
// Analyzes success/failure patterns, performance trends, and quality metrics.
func (ts *TraceStore) GetLearningInsights(shardType string, days int) (map[string]interface{}, error) {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	if days <= 0 {
		days = 7
	}

	insights := make(map[string]interface{})
	cutoff := time.Now().AddDate(0, 0, -days)

	// Recent activity
	var recentCount int64
	ts.db.QueryRow(`
		SELECT COUNT(*) FROM reasoning_traces
		WHERE shard_type = ? AND created_at >= ?`,
		shardType, cutoff).Scan(&recentCount)
	insights["recent_trace_count"] = recentCount

	// Recent success rate
	var recentSuccess int64
	ts.db.QueryRow(`
		SELECT COUNT(*) FROM reasoning_traces
		WHERE shard_type = ? AND created_at >= ? AND success = 1`,
		shardType, cutoff).Scan(&recentSuccess)
	if recentCount > 0 {
		insights["recent_success_rate"] = float64(recentSuccess) / float64(recentCount)
	}

	// Performance trend (comparing first half vs second half of period)
	midpoint := cutoff.Add(time.Duration(days*24/2) * time.Hour)
	var firstHalfSuccess, firstHalfTotal, secondHalfSuccess, secondHalfTotal int64

	ts.db.QueryRow(`
		SELECT COUNT(*), SUM(CASE WHEN success = 1 THEN 1 ELSE 0 END)
		FROM reasoning_traces
		WHERE shard_type = ? AND created_at >= ? AND created_at < ?`,
		shardType, cutoff, midpoint).Scan(&firstHalfTotal, &firstHalfSuccess)

	ts.db.QueryRow(`
		SELECT COUNT(*), SUM(CASE WHEN success = 1 THEN 1 ELSE 0 END)
		FROM reasoning_traces
		WHERE shard_type = ? AND created_at >= ?`,
		shardType, midpoint).Scan(&secondHalfTotal, &secondHalfSuccess)

	if firstHalfTotal > 0 && secondHalfTotal > 0 {
		firstRate := float64(firstHalfSuccess) / float64(firstHalfTotal)
		secondRate := float64(secondHalfSuccess) / float64(secondHalfTotal)
		insights["performance_trend"] = secondRate - firstRate // Positive = improving
	}

	// Common failure reasons
	failurePatterns, _ := ts.GetFailurePatterns(5)
	insights["top_failure_patterns"] = failurePatterns

	// Average response time trend
	var avgDuration float64
	ts.db.QueryRow(`
		SELECT AVG(duration_ms) FROM reasoning_traces
		WHERE shard_type = ? AND created_at >= ?`,
		shardType, cutoff).Scan(&avgDuration)
	insights["avg_duration_ms"] = avgDuration

	return insights, nil
}

// ========== Cleanup Operations ==========

// CleanupOldTraces removes traces older than the specified retention period.
// Returns the number of traces deleted.
func (ts *TraceStore) CleanupOldTraces(retentionDays int) (int64, error) {
	timer := logging.StartTimer(logging.CategoryStore, "CleanupOldTraces")
	defer timer.Stop()

	ts.mu.Lock()
	defer ts.mu.Unlock()

	if retentionDays <= 0 {
		return 0, fmt.Errorf("retention days must be positive")
	}

	logging.Store("Cleaning up traces older than %d days", retentionDays)

	cutoff := time.Now().AddDate(0, 0, -retentionDays)

	result, err := ts.db.Exec(`
		DELETE FROM reasoning_traces
		WHERE created_at < ?`, cutoff)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to cleanup old traces: %v", err)
		return 0, err
	}

	rowsAffected, _ := result.RowsAffected()
	logging.Store("Cleaned up %d old traces (retention=%d days)", rowsAffected, retentionDays)
	return rowsAffected, nil
}

// ========== Helper Methods ==========

// scanTraces is a helper to scan SQL rows into ReasoningTrace structs.
func (ts *TraceStore) scanTraces(rows *sql.Rows) ([]ReasoningTrace, error) {
	var traces []ReasoningTrace
	for rows.Next() {
		var t ReasoningTrace
		var notesJSON string
		var model, taskContext, errorMessage sql.NullString
		var tokensUsed sql.NullInt64
		var qualityScore sql.NullFloat64

		err := rows.Scan(
			&t.ID, &t.ShardID, &t.ShardType, &t.ShardCategory, &t.SessionID, &taskContext,
			&t.SystemPrompt, &t.UserPrompt, &t.Response, &model, &tokensUsed, &t.DurationMs,
			&t.Success, &errorMessage, &qualityScore, &notesJSON, &t.CreatedAt,
		)
		if err != nil {
			continue
		}

		// Handle nullable fields
		if model.Valid {
			t.Model = model.String
		}
		if taskContext.Valid {
			t.TaskContext = taskContext.String
		}
		if errorMessage.Valid {
			t.ErrorMessage = errorMessage.String
		}
		if tokensUsed.Valid {
			t.TokensUsed = int(tokensUsed.Int64)
		}
		if qualityScore.Valid {
			t.QualityScore = qualityScore.Float64
		}
		if notesJSON != "" {
			json.Unmarshal([]byte(notesJSON), &t.LearningNotes)
		}

		traces = append(traces, t)
	}

	return traces, nil
}

// Close is a no-op since TraceStore doesn't own the database connection.
// The parent LocalStore is responsible for closing the connection.
func (ts *TraceStore) Close() error {
	return nil
}
