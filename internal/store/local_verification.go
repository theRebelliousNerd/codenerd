package store

import (
	"codenerd/internal/logging"
	"database/sql"
	"fmt"
	"reflect"
	"time"
)

// =============================================================================
// VERIFICATION AND REASONING TRACES
// =============================================================================

// VerificationRecord represents a stored verification attempt.
type VerificationRecord struct {
	ID             int64
	SessionID      string
	TurnNumber     int
	Task           string
	ShardType      string
	AttemptNumber  int
	Success        bool
	Confidence     float64
	Reason         string
	ViolationsJSON string
	CorrectiveJSON string
	EvidenceJSON   string
	ResultHash     string
	CreatedAt      time.Time
}

// StoreVerification records a verification attempt for learning.
func (s *LocalStore) StoreVerification(
	sessionID string,
	turnNumber int,
	task string,
	shardType string,
	attemptNumber int,
	success bool,
	confidence float64,
	reason string,
	violationsJSON string,
	correctiveJSON string,
	evidenceJSON string,
	resultHash string,
) error {
	timer := logging.StartTimer(logging.CategoryStore, "StoreVerification")
	defer timer.Stop()

	s.mu.Lock()
	defer s.mu.Unlock()

	logging.StoreDebug("Storing verification: session=%s turn=%d shard=%s attempt=%d success=%v confidence=%.2f",
		sessionID, turnNumber, shardType, attemptNumber, success, confidence)

	_, err := s.db.Exec(
		`INSERT INTO task_verifications
		 (session_id, turn_number, task, shard_type, attempt_number, success, confidence, reason, quality_violations, corrective_action, evidence, result_hash)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		sessionID, turnNumber, task, shardType, attemptNumber, success, confidence, reason, violationsJSON, correctiveJSON, evidenceJSON, resultHash,
	)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to store verification: session=%s turn=%d: %v", sessionID, turnNumber, err)
		return err
	}

	logging.StoreDebug("Verification stored: session=%s turn=%d shard=%s success=%v", sessionID, turnNumber, shardType, success)
	return nil
}

// GetVerificationHistory retrieves verification attempts for a session.
func (s *LocalStore) GetVerificationHistory(sessionID string, limit int) ([]VerificationRecord, error) {
	timer := logging.StartTimer(logging.CategoryStore, "GetVerificationHistory")
	defer timer.Stop()

	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 50
	}

	logging.StoreDebug("Retrieving verification history: session=%s limit=%d", sessionID, limit)

	rows, err := s.db.Query(
		`SELECT id, session_id, turn_number, task, shard_type, attempt_number, success, confidence, reason, quality_violations, corrective_action, evidence, result_hash, created_at
		 FROM task_verifications
		 WHERE session_id = ?
		 ORDER BY created_at DESC
		 LIMIT ?`,
		sessionID, limit,
	)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to query verification history for %s: %v", sessionID, err)
		return nil, err
	}
	defer rows.Close()

	var records []VerificationRecord
	for rows.Next() {
		var rec VerificationRecord
		if err := rows.Scan(
			&rec.ID, &rec.SessionID, &rec.TurnNumber, &rec.Task, &rec.ShardType,
			&rec.AttemptNumber, &rec.Success, &rec.Confidence, &rec.Reason,
			&rec.ViolationsJSON, &rec.CorrectiveJSON, &rec.EvidenceJSON, &rec.ResultHash, &rec.CreatedAt,
		); err != nil {
			continue
		}
		records = append(records, rec)
	}

	logging.StoreDebug("Retrieved %d verification records for session=%s", len(records), sessionID)
	return records, nil
}

// GetQualityViolationStats retrieves statistics on quality violations for learning.
func (s *LocalStore) GetQualityViolationStats() (map[string]int, error) {
	timer := logging.StartTimer(logging.CategoryStore, "GetQualityViolationStats")
	defer timer.Stop()

	s.mu.RLock()
	defer s.mu.RUnlock()

	logging.StoreDebug("Computing quality violation statistics")

	rows, err := s.db.Query(
		`SELECT quality_violations, COUNT(*) as count
		 FROM task_verifications
		 WHERE success = 0 AND quality_violations != '[]'
		 GROUP BY quality_violations
		 ORDER BY count DESC`,
	)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to query quality violation stats: %v", err)
		return nil, err
	}
	defer rows.Close()

	stats := make(map[string]int)
	for rows.Next() {
		var violations string
		var count int
		if err := rows.Scan(&violations, &count); err != nil {
			continue
		}
		stats[violations] = count
	}

	logging.StoreDebug("Quality violation stats computed: %d unique violation patterns", len(stats))
	return stats, nil
}

// =============================================================================
// REASONING TRACES (delegated to TraceStore)
// =============================================================================

// StoreReasoningTrace persists a reasoning trace.
// Implements perception.TraceStore interface.
// Accepts interface{} to avoid import cycles - uses reflection to extract fields.
func (s *LocalStore) StoreReasoningTrace(trace interface{}) error {
	// Try direct ReasoningTrace (from store package)
	if rt, ok := trace.(*ReasoningTrace); ok {
		return s.traceStore.StoreReasoningTrace(rt)
	}

	// Use reflection to handle perception.ReasoningTrace without import cycle
	// This allows any struct with the same field names to work
	v := reflect.ValueOf(trace)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return fmt.Errorf("invalid trace type: expected struct, got %v", v.Kind())
	}

	// Extract fields by name using reflection
	getField := func(name string) interface{} {
		f := v.FieldByName(name)
		if !f.IsValid() {
			return nil
		}
		return f.Interface()
	}

	id, _ := getField("ID").(string)
	shardID, _ := getField("ShardID").(string)
	shardType, _ := getField("ShardType").(string)
	shardCategory, _ := getField("ShardCategory").(string)
	sessionID, _ := getField("SessionID").(string)
	taskContext, _ := getField("TaskContext").(string)
	systemPrompt, _ := getField("SystemPrompt").(string)
	userPrompt, _ := getField("UserPrompt").(string)
	response, _ := getField("Response").(string)
	model, _ := getField("Model").(string)
	tokensUsed, _ := getField("TokensUsed").(int)
	durationMs, _ := getField("DurationMs").(int64)
	success, _ := getField("Success").(bool)
	errorMessage, _ := getField("ErrorMessage").(string)
	qualityScore, _ := getField("QualityScore").(float64)
	learningNotes, _ := getField("LearningNotes").([]string)
	summaryDescriptor, _ := getField("SummaryDescriptor").(string)
	descriptorVersion, _ := getField("DescriptorVersion").(int)
	descriptorHash, _ := getField("DescriptorHash").(string)
	embedding, _ := getField("Embedding").([]byte)
	embeddingModelID, _ := getField("EmbeddingModelID").(string)
	embeddingDim, _ := getField("EmbeddingDim").(int)
	embeddingTask, _ := getField("EmbeddingTask").(string)

	// Delegate to trace store
	return s.traceStore.storeReasoningTraceRaw(
		id, shardID, shardType, shardCategory, sessionID, taskContext,
		systemPrompt, userPrompt, response, model, errorMessage,
		tokensUsed, durationMs, success, qualityScore, learningNotes,
		summaryDescriptor, descriptorVersion, descriptorHash,
		embedding, embeddingModelID, embeddingDim, embeddingTask,
	)
}

// GetShardTraces retrieves traces for a specific shard type.
// Implements perception.ShardTraceReader interface.
func (s *LocalStore) GetShardTraces(shardType string, limit int) ([]ReasoningTrace, error) {
	return s.traceStore.GetShardTraces(shardType, limit)
}

// GetFailedShardTraces retrieves failed traces for a shard type.
func (s *LocalStore) GetFailedShardTraces(shardType string, limit int) ([]ReasoningTrace, error) {
	return s.traceStore.GetFailedShardTraces(shardType, limit)
}

// GetSimilarTaskTraces finds traces with similar task context.
func (s *LocalStore) GetSimilarTaskTraces(shardType, taskPattern string, limit int) ([]ReasoningTrace, error) {
	return s.traceStore.GetSimilarTaskTraces(shardType, taskPattern, limit)
}

// GetHighQualityTraces retrieves successful traces with high quality scores.
func (s *LocalStore) GetHighQualityTraces(shardType string, minScore float64, limit int) ([]ReasoningTrace, error) {
	return s.traceStore.GetHighQualityTraces(shardType, minScore, limit)
}

// GetRecentTraces retrieves recent traces across all shards.
// Used by main agent for oversight.
func (s *LocalStore) GetRecentTraces(limit int) ([]ReasoningTrace, error) {
	return s.traceStore.GetRecentTraces(limit)
}

// GetTracesBySession retrieves all traces for a specific session.
func (s *LocalStore) GetTracesBySession(sessionID string) ([]ReasoningTrace, error) {
	return s.traceStore.GetTracesBySession(sessionID)
}

// GetTracesByCategory retrieves traces by shard category.
func (s *LocalStore) GetTracesByCategory(category string, limit int) ([]ReasoningTrace, error) {
	return s.traceStore.GetTracesByCategory(category, limit)
}

// UpdateTraceQuality updates the quality score and learning notes for a trace.
func (s *LocalStore) UpdateTraceQuality(traceID string, score float64, notes []string) error {
	return s.traceStore.UpdateTraceQuality(traceID, score, notes)
}

// GetTraceStats returns statistics about reasoning traces.
func (s *LocalStore) GetTraceStats() (map[string]interface{}, error) {
	return s.traceStore.GetTraceStats()
}

// scanTraces is deprecated - functionality moved to TraceStore.
// Kept for backward compatibility with any external code that might call it.
func (s *LocalStore) scanTraces(rows *sql.Rows) ([]ReasoningTrace, error) {
	return s.traceStore.scanTraces(rows)
}
