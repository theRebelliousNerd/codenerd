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
	"codenerd/internal/prompt"
	"codenerd/internal/sqlpragmas"
)

// executionRecordColumns is the column list every ExecutionRecord query
// selects, in the exact order scanRecords expects. Kept in one place so that
// adding a column cannot leave one of the four call sites behind -- a mismatch
// here is a runtime scan error, not a compile error.
const executionRecordColumns = `task_id, session_id, shard_id, shard_type, task_request, problem_type,
		       actions_json, result_json, duration_ms, prompt_manifest_json, atom_ids_json,
		       thought_summary, thinking_tokens, grounding_sources_json, verdict_json,
		       provider, model, created_at`

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
	sqlpragmas.ApplyDefaultPragmas(db, sqlpragmas.ProfileHot)

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
		prompt_manifest_json TEXT,
		atom_ids_json TEXT,
		thought_summary TEXT,
		thinking_tokens INTEGER DEFAULT 0,
		grounding_sources_json TEXT,
		verdict_json TEXT,
		provider TEXT,
		model TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_records_shard ON execution_records(shard_type);
	CREATE INDEX IF NOT EXISTS idx_records_problem ON execution_records(problem_type);
	CREATE INDEX IF NOT EXISTS idx_records_created ON execution_records(created_at);
	CREATE INDEX IF NOT EXISTS idx_records_serving ON execution_records(provider, model);

	CREATE TABLE IF NOT EXISTS evolution_stats (
		key TEXT PRIMARY KEY,
		value INTEGER
	);
	`

	if _, err := fc.db.Exec(schema); err != nil {
		return err
	}

	return fc.ensureExecutionRecordColumns()
}

func (fc *FeedbackCollector) ensureExecutionRecordColumns() error {
	type columnSpec struct {
		name       string
		definition string
	}

	required := []columnSpec{
		{name: "prompt_manifest_json", definition: "TEXT"},
		{name: "thought_summary", definition: "TEXT"},
		{name: "thinking_tokens", definition: "INTEGER DEFAULT 0"},
		{name: "grounding_sources_json", definition: "TEXT"},
		// Serving provenance. Rows written before pinning existed keep NULL
		// here, which groups them under the empty pin and yields unpinned
		// atoms -- the pre-pinning behavior, applied only to pre-pinning data.
		{name: "provider", definition: "TEXT"},
		{name: "model", definition: "TEXT"},
	}

	existing := make(map[string]struct{})
	rows, err := fc.db.Query(`PRAGMA table_info(execution_records)`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid       int
			name      string
			ctype     string
			notNull   int
			defaultV  sql.NullString
			primaryPK int
		)
		if err := rows.Scan(&cid, &name, &ctype, &notNull, &defaultV, &primaryPK); err != nil {
			return err
		}
		existing[name] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, col := range required {
		if _, ok := existing[col.name]; ok {
			continue
		}
		stmt := fmt.Sprintf("ALTER TABLE execution_records ADD COLUMN %s %s", col.name, col.definition)
		if _, err := fc.db.Exec(stmt); err != nil {
			return fmt.Errorf("failed to add column %s: %w", col.name, err)
		}
	}

	return nil
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
	if exec == nil {
		return fmt.Errorf("execution record is nil")
	}

	logging.AutopoiesisDebug("Recording execution: task=%s, shard=%s, success=%v",
		exec.TaskID, exec.ShardType, exec.ExecutionResult.Success)

	// Serialize JSON fields
	actionsJSON, _ := json.Marshal(exec.AgentActions)
	resultJSON, _ := json.Marshal(exec.ExecutionResult)
	manifestJSON, _ := json.Marshal(exec.PromptManifest)
	atomIDsJSON, _ := json.Marshal(exec.AtomIDs)
	groundingJSON, _ := json.Marshal(exec.GroundingSources)

	var verdictJSON []byte
	if exec.Verdict != nil {
		verdictJSON, _ = json.Marshal(exec.Verdict)
	}

	// Insert into database
	_, err := fc.db.Exec(`
		INSERT OR REPLACE INTO execution_records
		(task_id, session_id, shard_id, shard_type, task_request, problem_type,
		 actions_json, result_json, duration_ms, prompt_manifest_json, atom_ids_json,
		 thought_summary, thinking_tokens, grounding_sources_json, verdict_json,
		 provider, model, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		exec.TaskID, exec.SessionID, exec.ShardID, exec.ShardType,
		exec.TaskRequest, exec.ProblemType,
		string(actionsJSON), string(resultJSON), exec.Duration.Milliseconds(),
		string(manifestJSON), string(atomIDsJSON),
		exec.ThoughtSummary, exec.ThinkingTokens, string(groundingJSON),
		string(verdictJSON), exec.Provider, exec.Model, exec.Timestamp,
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
		SELECT `+executionRecordColumns+`
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
		SELECT `+executionRecordColumns+`
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
		SELECT `+executionRecordColumns+`
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

// FailureGroupKey identifies a set of failures that may be generalized into a
// single atom. Comparable, so it is used directly as a map key rather than a
// formatted string that has to be parsed back apart.
//
// Provider and Model hold canonical tokens (empty when the scope does not
// discriminate on them or the records carry no provenance), not raw vendor
// spellings, so that two spellings of one model group together.
type FailureGroupKey struct {
	ProblemType string
	ShardType   string
	Provider    string
	Model       string
}

// String renders the key for logging.
func (k FailureGroupKey) String() string {
	serving := k.Provider
	if k.Model != "" {
		if serving != "" {
			serving += "/"
		}
		serving += k.Model
	}
	if serving == "" {
		serving = "unpinned"
	}
	return fmt.Sprintf("%s:%s@%s", k.ProblemType, k.ShardType, serving)
}

// GetFailuresByProblemType returns failures grouped by problem type, shard, and
// serving LLM.
//
// scope controls how finely the serving dimension partitions the groups, and it
// must be the same scope the AtomGenerator will pin with. That coupling is the
// point: an atom generalizes exactly one group, so if groups were coarser than
// pins, one atom would be generated from failures spanning two different models
// and then pinned to whichever happened to come first. Grouping and pinning
// share prompt.NormalizeProviderToken / NormalizeModelToken / ModelFamilyToken
// so the group's token and the generated atom's selector are equal by
// construction.
//
// Records predating the provenance columns carry no provider/model and collect
// under the empty pin, producing unpinned atoms as before.
func (fc *FeedbackCollector) GetFailuresByProblemType(minCount int, scope PinScope) (map[FailureGroupKey][]*ExecutionRecord, error) {
	fc.mu.RLock()
	defer fc.mu.RUnlock()

	if !scope.Valid() {
		return nil, fmt.Errorf("invalid pin scope %q", scope)
	}

	rows, err := fc.db.Query(`
		SELECT ` + executionRecordColumns + `
		FROM execution_records
		WHERE verdict_json LIKE '%"verdict":"FAIL"%'
		ORDER BY problem_type, shard_type, provider, model, created_at DESC`)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records, err := fc.scanRecords(rows)
	if err != nil {
		return nil, err
	}

	grouped := make(map[FailureGroupKey][]*ExecutionRecord)
	for _, rec := range records {
		grouped[GroupKeyFor(rec, scope)] = append(grouped[GroupKeyFor(rec, scope)], rec)
	}

	// Filter by minimum count
	result := make(map[FailureGroupKey][]*ExecutionRecord)
	for key, recs := range grouped {
		if len(recs) >= minCount {
			result[key] = recs
		}
	}

	return result, nil
}

// GroupKeyFor derives the failure group a record belongs to under scope.
func GroupKeyFor(rec *ExecutionRecord, scope PinScope) FailureGroupKey {
	key := FailureGroupKey{
		ProblemType: rec.ProblemType,
		ShardType:   rec.ShardType,
	}

	if scope == PinScopeNone {
		return key
	}

	key.Provider = prompt.NormalizeProviderToken(rec.Provider)

	if scope != PinScopeProvider {
		if exact := prompt.NormalizeModelToken(rec.Model); exact != "" {
			key.Model = exact
			if scope == PinScopeModelFamily {
				if family := prompt.ModelFamilyToken(exact); family != "" {
					key.Model = family
				}
			}
		}
	}

	return key
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
		var actionsJSON, resultJSON, manifestJSON, atomIDsJSON, groundingJSON string
		var verdictJSON sql.NullString
		var durationMs int64
		var thoughtSummary sql.NullString
		var thinkingTokens sql.NullInt64
		var provider, model sql.NullString
		var createdAt time.Time

		err := rows.Scan(
			&rec.TaskID, &rec.SessionID, &rec.ShardID, &rec.ShardType,
			&rec.TaskRequest, &rec.ProblemType,
			&actionsJSON, &resultJSON, &durationMs, &manifestJSON, &atomIDsJSON,
			&thoughtSummary, &thinkingTokens, &groundingJSON, &verdictJSON,
			&provider, &model, &createdAt,
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
		if manifestJSON != "" && manifestJSON != "null" {
			rec.PromptManifest = &prompt.PromptManifest{}
			if err := json.Unmarshal([]byte(manifestJSON), rec.PromptManifest); err != nil {
				rec.PromptManifest = nil
			}
		}
		if atomIDsJSON != "" {
			json.Unmarshal([]byte(atomIDsJSON), &rec.AtomIDs)
		}
		if thoughtSummary.Valid {
			rec.ThoughtSummary = thoughtSummary.String
		}
		if thinkingTokens.Valid {
			rec.ThinkingTokens = int(thinkingTokens.Int64)
		}
		if provider.Valid {
			rec.Provider = provider.String
		}
		if model.Valid {
			rec.Model = model.String
		}
		if groundingJSON != "" && groundingJSON != "null" {
			json.Unmarshal([]byte(groundingJSON), &rec.GroundingSources)
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
