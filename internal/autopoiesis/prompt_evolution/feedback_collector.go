package prompt_evolution

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"codenerd/internal/logging"
)

// FeedbackCollector records and manages execution feedback.
// It buffers executions and persists them to SQLite for analysis.
type FeedbackCollector struct {
	mu sync.RWMutex

	// Storage
	db        *sql.DB
	storePath string

	// Buffer for batching
	buffer   []*ExecutionRecord
	capacity int

	// Statistics
	totalRecorded int
	totalFailures int
}

// NewFeedbackCollector creates a new feedback collector.
func NewFeedbackCollector(nerdDir string) (*FeedbackCollector, error) {
	storePath := filepath.Join(nerdDir, "prompts", "evolution.db")

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(storePath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create evolution directory: %w", err)
	}

	db, err := sql.Open("sqlite3", storePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open evolution database: %w", err)
	}

	fc := &FeedbackCollector{
		db:        db,
		storePath: storePath,
		buffer:    make([]*ExecutionRecord, 0, 100),
		capacity:  100,
	}

	if err := fc.ensureSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ensure schema: %w", err)
	}

	// Load stats
	fc.loadStats()

	logging.Autopoiesis("FeedbackCollector initialized: path=%s, recorded=%d, failures=%d",
		storePath, fc.totalRecorded, fc.totalFailures)

	return fc, nil
}

// ensureSchema creates the necessary tables.
func (fc *FeedbackCollector) ensureSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS execution_records (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		task_id TEXT UNIQUE,
		session_id TEXT,
		shard_id TEXT,
		shard_type TEXT,
		task_request TEXT,
		problem_type TEXT,
		actions_json TEXT,
		result_json TEXT,
		duration_ms INTEGER,
		atom_ids_json TEXT,
		verdict_json TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_records_shard ON execution_records(shard_type);
	CREATE INDEX IF NOT EXISTS idx_records_problem ON execution_records(problem_type);
	CREATE INDEX IF NOT EXISTS idx_records_created ON execution_records(created_at);

	CREATE TABLE IF NOT EXISTS evolution_stats (
		key TEXT PRIMARY KEY,
		value INTEGER
	);
	`

	_, err := fc.db.Exec(schema)
	return err
}

// loadStats loads statistics from the database.
func (fc *FeedbackCollector) loadStats() {
	row := fc.db.QueryRow("SELECT COUNT(*) FROM execution_records")
	row.Scan(&fc.totalRecorded)

	row = fc.db.QueryRow("SELECT COUNT(*) FROM execution_records WHERE verdict_json LIKE '%\"verdict\":\"FAIL\"%'")
	row.Scan(&fc.totalFailures)
}

// Record stores an execution record.
func (fc *FeedbackCollector) Record(exec *ExecutionRecord) error {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	logging.AutopoiesisDebug("Recording execution: task=%s, shard=%s, success=%v",
		exec.TaskID, exec.ShardType, exec.ExecutionResult.Success)

	// Serialize JSON fields
	actionsJSON, _ := json.Marshal(exec.AgentActions)
	resultJSON, _ := json.Marshal(exec.ExecutionResult)
	atomIDsJSON, _ := json.Marshal(exec.AtomIDs)

	var verdictJSON []byte
	if exec.Verdict != nil {
		verdictJSON, _ = json.Marshal(exec.Verdict)
	}

	// Insert into database
	_, err := fc.db.Exec(`
		INSERT OR REPLACE INTO execution_records
		(task_id, session_id, shard_id, shard_type, task_request, problem_type,
		 actions_json, result_json, duration_ms, atom_ids_json, verdict_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		exec.TaskID, exec.SessionID, exec.ShardID, exec.ShardType,
		exec.TaskRequest, exec.ProblemType,
		string(actionsJSON), string(resultJSON), exec.Duration.Milliseconds(),
		string(atomIDsJSON), string(verdictJSON), exec.Timestamp,
	)

	if err != nil {
		logging.Get(logging.CategoryAutopoiesis).Error("Failed to record execution: %v", err)
		return err
	}

	fc.totalRecorded++
	if exec.Verdict != nil && exec.Verdict.IsFail() {
		fc.totalFailures++
	}

	// Add to buffer for quick access
	fc.buffer = append(fc.buffer, exec)
	if len(fc.buffer) > fc.capacity {
		fc.buffer = fc.buffer[1:]
	}

	logging.Autopoiesis("Execution recorded: task=%s, total=%d", exec.TaskID, fc.totalRecorded)
	return nil
}

// GetRecentFailures returns the most recent failed executions.
func (fc *FeedbackCollector) GetRecentFailures(limit int) ([]*ExecutionRecord, error) {
	fc.mu.RLock()
	defer fc.mu.RUnlock()

	logging.AutopoiesisDebug("Fetching recent failures: limit=%d", limit)

	rows, err := fc.db.Query(`
		SELECT task_id, session_id, shard_id, shard_type, task_request, problem_type,
		       actions_json, result_json, duration_ms, atom_ids_json, verdict_json, created_at
		FROM execution_records
		WHERE verdict_json LIKE '%"verdict":"FAIL"%'
		   OR (verdict_json IS NULL AND result_json LIKE '%"success":false%')
		ORDER BY created_at DESC
		LIMIT ?`, limit)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return fc.scanRecords(rows)
}

// GetRecentByShardType returns recent executions for a specific shard type.
func (fc *FeedbackCollector) GetRecentByShardType(shardType string, limit int) ([]*ExecutionRecord, error) {
	fc.mu.RLock()
	defer fc.mu.RUnlock()

	rows, err := fc.db.Query(`
		SELECT task_id, session_id, shard_id, shard_type, task_request, problem_type,
		       actions_json, result_json, duration_ms, atom_ids_json, verdict_json, created_at
		FROM execution_records
		WHERE shard_type = ?
		ORDER BY created_at DESC
		LIMIT ?`, shardType, limit)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return fc.scanRecords(rows)
}

// GetUnevaluated returns executions that haven't been evaluated yet.
func (fc *FeedbackCollector) GetUnevaluated(limit int) ([]*ExecutionRecord, error) {
	fc.mu.RLock()
	defer fc.mu.RUnlock()

	rows, err := fc.db.Query(`
		SELECT task_id, session_id, shard_id, shard_type, task_request, problem_type,
		       actions_json, result_json, duration_ms, atom_ids_json, verdict_json, created_at
		FROM execution_records
		WHERE verdict_json IS NULL OR verdict_json = ''
		ORDER BY created_at DESC
		LIMIT ?`, limit)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return fc.scanRecords(rows)
}

// GetFailuresByProblemType returns failures grouped by problem type and shard.
func (fc *FeedbackCollector) GetFailuresByProblemType(minCount int) (map[string][]*ExecutionRecord, error) {
	fc.mu.RLock()
	defer fc.mu.RUnlock()

	rows, err := fc.db.Query(`
		SELECT task_id, session_id, shard_id, shard_type, task_request, problem_type,
		       actions_json, result_json, duration_ms, atom_ids_json, verdict_json, created_at
		FROM execution_records
		WHERE verdict_json LIKE '%"verdict":"FAIL"%'
		ORDER BY problem_type, shard_type, created_at DESC`)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records, err := fc.scanRecords(rows)
	if err != nil {
		return nil, err
	}

	// Group by problem_type:shard_type
	grouped := make(map[string][]*ExecutionRecord)
	for _, rec := range records {
		key := fmt.Sprintf("%s:%s", rec.ProblemType, rec.ShardType)
		grouped[key] = append(grouped[key], rec)
	}

	// Filter by minimum count
	result := make(map[string][]*ExecutionRecord)
	for key, recs := range grouped {
		if len(recs) >= minCount {
			result[key] = recs
		}
	}

	return result, nil
}

// UpdateVerdict updates the verdict for an execution record.
func (fc *FeedbackCollector) UpdateVerdict(taskID string, verdict *JudgeVerdict) error {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	verdictJSON, err := json.Marshal(verdict)
	if err != nil {
		return err
	}

	_, err = fc.db.Exec(`
		UPDATE execution_records
		SET verdict_json = ?
		WHERE task_id = ?`, string(verdictJSON), taskID)

	if err != nil {
		return err
	}

	if verdict.IsFail() {
		fc.totalFailures++
	}

	// Update buffer
	for _, rec := range fc.buffer {
		if rec.TaskID == taskID {
			rec.Verdict = verdict
			break
		}
	}

	return nil
}

// scanRecords scans rows into ExecutionRecords.
func (fc *FeedbackCollector) scanRecords(rows *sql.Rows) ([]*ExecutionRecord, error) {
	var records []*ExecutionRecord

	for rows.Next() {
		var rec ExecutionRecord
		var actionsJSON, resultJSON, atomIDsJSON string
		var verdictJSON sql.NullString
		var durationMs int64
		var createdAt time.Time

		err := rows.Scan(
			&rec.TaskID, &rec.SessionID, &rec.ShardID, &rec.ShardType,
			&rec.TaskRequest, &rec.ProblemType,
			&actionsJSON, &resultJSON, &durationMs,
			&atomIDsJSON, &verdictJSON, &createdAt,
		)
		if err != nil {
			logging.Get(logging.CategoryAutopoiesis).Warn("Failed to scan record: %v", err)
			continue
		}

		rec.Duration = time.Duration(durationMs) * time.Millisecond
		rec.Timestamp = createdAt

		// Parse JSON fields
		if actionsJSON != "" {
			json.Unmarshal([]byte(actionsJSON), &rec.AgentActions)
		}
		if resultJSON != "" {
			json.Unmarshal([]byte(resultJSON), &rec.ExecutionResult)
		}
		if atomIDsJSON != "" {
			json.Unmarshal([]byte(atomIDsJSON), &rec.AtomIDs)
		}
		if verdictJSON.Valid && verdictJSON.String != "" {
			rec.Verdict = &JudgeVerdict{}
			json.Unmarshal([]byte(verdictJSON.String), rec.Verdict)
		}

		records = append(records, &rec)
	}

	return records, rows.Err()
}

// GetStats returns current statistics.
func (fc *FeedbackCollector) GetStats() (totalRecorded, totalFailures int) {
	fc.mu.RLock()
	defer fc.mu.RUnlock()
	return fc.totalRecorded, fc.totalFailures
}

// GetSuccessRate returns the overall success rate.
func (fc *FeedbackCollector) GetSuccessRate() float64 {
	fc.mu.RLock()
	defer fc.mu.RUnlock()

	if fc.totalRecorded == 0 {
		return 0.5 // Neutral when no data
	}

	successes := fc.totalRecorded - fc.totalFailures
	return float64(successes) / float64(fc.totalRecorded)
}

// Close closes the database connection.
func (fc *FeedbackCollector) Close() error {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	if fc.db != nil {
		return fc.db.Close()
	}
	return nil
}

// PruneOldRecords removes records older than the specified duration.
func (fc *FeedbackCollector) PruneOldRecords(olderThan time.Duration) (int, error) {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	cutoff := time.Now().Add(-olderThan)

	result, err := fc.db.Exec(`
		DELETE FROM execution_records
		WHERE created_at < ?`, cutoff)

	if err != nil {
		return 0, err
	}

	affected, _ := result.RowsAffected()
	if affected > 0 {
		logging.Autopoiesis("Pruned %d old execution records", affected)
		fc.loadStats() // Refresh stats
	}

	return int(affected), nil
}
