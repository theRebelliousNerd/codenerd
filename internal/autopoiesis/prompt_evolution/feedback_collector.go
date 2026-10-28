package prompt_evolution

import (
	"database/sql"
	"encoding/json"
	"errors"
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
		       provider, model, created_at, verdict_version`

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

	closed bool

	// ledger is the evidence account kept in the same database.
	ledger *EvidenceLedger
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
	ledger, err := newEvidenceLedger(db)
	if err != nil {
		db.Close()
		return nil, err
	}
	fc.ledger = ledger

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

	CREATE TABLE IF NOT EXISTS evolution_stats (
		key TEXT PRIMARY KEY,
		value INTEGER
	);
	`

	if _, err := fc.db.Exec(schema); err != nil {
		return err
	}

	if err := fc.ensureExecutionRecordColumns(); err != nil {
		return err
	}

	// Indexes over migrated columns come after the column migration. This one
	// sat in the schema batch above, so on any evolution.db created before
	// serving provenance existed it failed with "no such column: provider"
	// before ensureExecutionRecordColumns could add the column — and the
	// evolver reported "this session will not learn from its turns" on every
	// boot of an established workspace.
	if _, err := fc.db.Exec(`CREATE INDEX IF NOT EXISTS idx_records_serving ON execution_records(provider, model)`); err != nil {
		return err
	}
	return fc.backfillVerdictVersions()
}

// backfillVerdictVersions versions verdicts stored before verdict_version
// existed, so they can be credited and consumed like any other.
func (fc *FeedbackCollector) backfillVerdictVersions() error {
	rows, err := fc.db.Query(`SELECT task_id, verdict_json FROM execution_records
		WHERE verdict_json IS NOT NULL AND verdict_json <> '' AND (verdict_version IS NULL OR verdict_version = '')`)
	if err != nil {
		return err
	}
	type pending struct{ taskID, version string }
	var updates []pending
	for rows.Next() {
		var taskID, verdictJSON string
		if err := rows.Scan(&taskID, &verdictJSON); err != nil {
			rows.Close()
			return err
		}
		updates = append(updates, pending{taskID, versionOf([]byte(verdictJSON))})
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, u := range updates {
		if _, err := fc.db.Exec(`UPDATE execution_records SET verdict_version = ? WHERE task_id = ?`, u.version, u.taskID); err != nil {
			return err
		}
	}
	return nil
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
		// Event identity: the hash of a record's immutable payload, so a
		// replay is recognized and a conflicting payload under the same task
		// id is refused instead of silently replacing the row.
		{name: "payload_hash", definition: "TEXT"},
		// The version of the stored verdict (sha256 of verdict_json): the
		// unit attribution is keyed by and evidence is consumed at.
		{name: "verdict_version", definition: "TEXT"},
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
//
// Both Scans were unchecked, so a missing or unreadable execution_records table
// left totalRecorded and totalFailures at their zero values — indistinguishable
// from a collector that has genuinely recorded nothing. GetStats then reported
// 0/0 and the prompt-evolution trigger, which fires on accumulated failures,
// simply never fired. loadStats has no error to return (it runs from the
// constructor and after a prune), so a failure is logged and the counters are
// left alone rather than being zeroed by a failed Scan.
func (fc *FeedbackCollector) loadStats() {
	var recorded, failures int
	if err := fc.db.QueryRow("SELECT COUNT(*) FROM execution_records").Scan(&recorded); err != nil {
		logging.Get(logging.CategoryAutopoiesis).Error(
			"Feedback collector could not count execution records; stats are stale, not zero: %v", err)
		return
	}
	if err := fc.db.QueryRow("SELECT COUNT(*) FROM execution_records WHERE verdict_json LIKE '%\"verdict\":\"FAIL\"%'").Scan(&failures); err != nil {
		logging.Get(logging.CategoryAutopoiesis).Error(
			"Feedback collector could not count failed executions; failure-driven prompt evolution will not trigger: %v", err)
		return
	}
	fc.totalRecorded = recorded
	fc.totalFailures = failures
}

// RecordOutcome says what Record did with an event.
type RecordOutcome string

const (
	// RecordInserted is a new event.
	RecordInserted RecordOutcome = "inserted"
	// RecordDuplicate is a replay of an event already stored with the same
	// payload. Nothing changes except, when the replay carries a different
	// verdict, the verdict (UpdateVerdict semantics).
	RecordDuplicate RecordOutcome = "duplicate"
)

// ErrConflictingEvent is returned when an event identity (task id) is
// replayed with a different payload. That is a reconciliation error, not an
// update: two different turns must never share an identity.
var ErrConflictingEvent = errors.New("conflicting payload for an existing execution event")

// eventPayloadHash hashes the immutable part of a record: everything except
// the verdict (versioned separately) and the derived problem type.
func eventPayloadHash(exec *ExecutionRecord) (string, error) {
	payload, err := json.Marshal(struct {
		TaskID, SessionID, ShardID, ShardType, TaskRequest, Provider, Model string
		Timestamp                                                           int64
		Actions                                                             []AgentAction
		Result                                                              ExecutionResult
		Duration                                                            int64
		AtomIDs                                                             []string
	}{
		exec.TaskID, exec.SessionID, exec.ShardID, exec.ShardType, exec.TaskRequest, exec.Provider, exec.Model,
		exec.Timestamp.UnixNano(), exec.AgentActions, exec.ExecutionResult, int64(exec.Duration), exec.AtomIDs,
	})
	if err != nil {
		return "", err
	}
	return versionOf(payload), nil
}

// Record stores an execution event idempotently.
//
// The event identity is the task id; its payload hash decides what a second
// arrival is. Same payload: a duplicate, which changes nothing (a different
// verdict on it is applied as a verdict revision). Different payload: the
// call is refused with ErrConflictingEvent. The counters move only on a real
// insert or a real verdict change, so they cannot drift from the rows.
//
// It used to be INSERT OR REPLACE plus an unconditional totalRecorded++, so a
// replay replaced the row and counted it again, and a different turn reusing
// an id overwrote the first silently.
//
// On success exec.VerdictVersion holds the stored verdict's version.
func (fc *FeedbackCollector) Record(exec *ExecutionRecord) (RecordOutcome, error) {
	if exec == nil {
		return "", fmt.Errorf("execution record is nil")
	}
	payloadHash, err := eventPayloadHash(exec)
	if err != nil {
		return "", fmt.Errorf("hash execution %s: %w", exec.TaskID, err)
	}

	fc.mu.Lock()
	defer fc.mu.Unlock()
	if fc.closed {
		return "", ErrEvolverClosed
	}

	logging.AutopoiesisDebug("Recording execution: task=%s, shard=%s, success=%v",
		exec.TaskID, exec.ShardType, exec.ExecutionResult.Success)

	var storedHash sql.NullString
	err = fc.db.QueryRow(`SELECT payload_hash FROM execution_records WHERE task_id = ?`, exec.TaskID).Scan(&storedHash)
	switch {
	case err == nil:
		if storedHash.Valid && storedHash.String != "" && storedHash.String != payloadHash {
			return "", fmt.Errorf("%w: task %s", ErrConflictingEvent, exec.TaskID)
		}
		if !storedHash.Valid || storedHash.String == "" {
			// A row written before event identity existed adopts this hash.
			if _, err := fc.db.Exec(`UPDATE execution_records SET payload_hash = ? WHERE task_id = ?`, payloadHash, exec.TaskID); err != nil {
				return "", err
			}
		}
		if exec.Verdict != nil {
			version, _, err := fc.updateVerdictLocked(exec.TaskID, exec.Verdict)
			if err != nil {
				return "", err
			}
			exec.VerdictVersion = version
		} else {
			exec.VerdictVersion = fc.storedVerdictVersionLocked(exec.TaskID)
		}
		return RecordDuplicate, nil
	case !errors.Is(err, sql.ErrNoRows):
		return "", fmt.Errorf("look up execution %s: %w", exec.TaskID, err)
	}

	// Serialize JSON fields
	actionsJSON, _ := json.Marshal(exec.AgentActions)
	resultJSON, _ := json.Marshal(exec.ExecutionResult)
	manifestJSON, _ := json.Marshal(exec.PromptManifest)
	atomIDsJSON, _ := json.Marshal(exec.AtomIDs)
	groundingJSON, _ := json.Marshal(exec.GroundingSources)

	var verdictJSON, verdictVersion sql.NullString
	if exec.Verdict != nil {
		encoded, err := json.Marshal(exec.Verdict)
		if err != nil {
			return "", err
		}
		verdictJSON = sql.NullString{String: string(encoded), Valid: true}
		verdictVersion = sql.NullString{String: versionOf(encoded), Valid: true}
	}

	_, err = fc.db.Exec(`
		INSERT INTO execution_records
		(task_id, session_id, shard_id, shard_type, task_request, problem_type,
		 actions_json, result_json, duration_ms, prompt_manifest_json, atom_ids_json,
		 thought_summary, thinking_tokens, grounding_sources_json, verdict_json,
		 provider, model, created_at, payload_hash, verdict_version)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		exec.TaskID, exec.SessionID, exec.ShardID, exec.ShardType,
		exec.TaskRequest, exec.ProblemType,
		string(actionsJSON), string(resultJSON), exec.Duration.Milliseconds(),
		string(manifestJSON), string(atomIDsJSON),
		exec.ThoughtSummary, exec.ThinkingTokens, string(groundingJSON),
		verdictJSON, exec.Provider, exec.Model, exec.Timestamp, payloadHash, verdictVersion,
	)
	if err != nil {
		logging.Get(logging.CategoryAutopoiesis).Error("Failed to record execution: %v", err)
		return "", err
	}
	exec.VerdictVersion = verdictVersion.String

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
	return RecordInserted, nil
}

func (fc *FeedbackCollector) storedVerdictVersionLocked(taskID string) string {
	var version sql.NullString
	_ = fc.db.QueryRow(`SELECT verdict_version FROM execution_records WHERE task_id = ?`, taskID).Scan(&version)
	return version.String
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
//
// Only UNCONSUMED failures are returned: a failure at a verdict version that
// has already bought a generation (consumed_evidence) is not evidence for the
// next one. Without this every cycle re-read every FAIL ever recorded and
// re-purchased generation for groups whose evidence had not changed. A
// revised verdict is a new version and is unconsumed again.
func (fc *FeedbackCollector) GetFailuresByProblemType(minCount int, scope PinScope) (map[FailureGroupKey][]*ExecutionRecord, error) {
	fc.mu.RLock()
	defer fc.mu.RUnlock()

	if !scope.Valid() {
		return nil, fmt.Errorf("invalid pin scope %q", scope)
	}

	rows, err := fc.db.Query(`
		SELECT ` + executionRecordColumns + `
		FROM execution_records r
		WHERE verdict_json LIKE '%"verdict":"FAIL"%'
		  AND NOT EXISTS (SELECT 1 FROM consumed_evidence c
		                  WHERE c.task_id = r.task_id AND c.verdict_version = r.verdict_version)
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

// UpdateVerdict stores a verdict for an execution record and returns its
// version and whether it changed anything. Writing the verdict a record
// already holds is a no-op; a different verdict replaces it and moves the
// failure counter by the difference -- it used to add one for every FAIL
// written, so re-judging a failure counted it twice.
func (fc *FeedbackCollector) UpdateVerdict(taskID string, verdict *JudgeVerdict) (string, bool, error) {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	if fc.closed {
		return "", false, ErrEvolverClosed
	}
	return fc.updateVerdictLocked(taskID, verdict)
}

func (fc *FeedbackCollector) updateVerdictLocked(taskID string, verdict *JudgeVerdict) (string, bool, error) {
	if verdict == nil {
		return "", false, fmt.Errorf("verdict is nil")
	}
	encoded, err := json.Marshal(verdict)
	if err != nil {
		return "", false, err
	}
	version := versionOf(encoded)

	var oldJSON, oldVersion sql.NullString
	err = fc.db.QueryRow(`SELECT verdict_json, verdict_version FROM execution_records WHERE task_id = ?`, taskID).
		Scan(&oldJSON, &oldVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, fmt.Errorf("no execution record %s", taskID)
	}
	if err != nil {
		return "", false, err
	}
	if oldVersion.String == version {
		return version, false, nil
	}

	if _, err := fc.db.Exec(`UPDATE execution_records SET verdict_json = ?, verdict_version = ? WHERE task_id = ?`,
		string(encoded), version, taskID); err != nil {
		return "", false, err
	}

	wasFail := false
	if oldJSON.Valid && oldJSON.String != "" {
		var old JudgeVerdict
		if json.Unmarshal([]byte(oldJSON.String), &old) == nil {
			wasFail = old.IsFail()
		}
	}
	switch {
	case verdict.IsFail() && !wasFail:
		fc.totalFailures++
	case !verdict.IsFail() && wasFail:
		fc.totalFailures--
	}

	for _, rec := range fc.buffer {
		if rec.TaskID == taskID {
			rec.Verdict = verdict
			rec.VerdictVersion = version
			break
		}
	}
	return version, true, nil
}

// logExecutionRecordDecodeFailure names the record and column whose stored JSON
// could not be read back, so a corrupt row is traceable instead of presenting as
// an execution that did nothing.
func logExecutionRecordDecodeFailure(taskID, column string, err error) {
	logging.Get(logging.CategoryAutopoiesis).Warn(
		"Execution record %s: column %s could not be decoded, that field is missing from the record: %v", taskID, column, err)
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
		var provider, model, verdictVersion sql.NullString
		var createdAt time.Time

		err := rows.Scan(
			&rec.TaskID, &rec.SessionID, &rec.ShardID, &rec.ShardType,
			&rec.TaskRequest, &rec.ProblemType,
			&actionsJSON, &resultJSON, &durationMs, &manifestJSON, &atomIDsJSON,
			&thoughtSummary, &thinkingTokens, &groundingJSON, &verdictJSON,
			&provider, &model, &createdAt, &verdictVersion,
		)
		if err != nil {
			logging.Get(logging.CategoryAutopoiesis).Warn("Failed to scan record: %v", err)
			continue
		}

		rec.Duration = time.Duration(durationMs) * time.Millisecond
		rec.VerdictVersion = verdictVersion.String
		rec.Timestamp = createdAt

		// Parse JSON fields.
		//
		// These were bare unmarshals. A corrupt column produced a record with no
		// actions, no result or no atom ids, which is not a parse failure to
		// anything downstream — it is a task that apparently did nothing, and
		// prompt evolution scores atoms on exactly these fields. Each failure is
		// now named with the record it came from; the record is still returned,
		// because a partially-readable history beats a dropped one, but the gap
		// is on the record.
		if actionsJSON != "" {
			if err := json.Unmarshal([]byte(actionsJSON), &rec.AgentActions); err != nil {
				logExecutionRecordDecodeFailure(rec.TaskID, "agent_actions", err)
			}
		}
		if resultJSON != "" {
			if err := json.Unmarshal([]byte(resultJSON), &rec.ExecutionResult); err != nil {
				logExecutionRecordDecodeFailure(rec.TaskID, "execution_result", err)
			}
		}
		if manifestJSON != "" && manifestJSON != "null" {
			rec.PromptManifest = &prompt.PromptManifest{}
			if err := json.Unmarshal([]byte(manifestJSON), rec.PromptManifest); err != nil {
				rec.PromptManifest = nil
				logExecutionRecordDecodeFailure(rec.TaskID, "prompt_manifest", err)
			}
		}
		if atomIDsJSON != "" {
			if err := json.Unmarshal([]byte(atomIDsJSON), &rec.AtomIDs); err != nil {
				logExecutionRecordDecodeFailure(rec.TaskID, "atom_ids", err)
			}
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
			if err := json.Unmarshal([]byte(groundingJSON), &rec.GroundingSources); err != nil {
				logExecutionRecordDecodeFailure(rec.TaskID, "grounding_sources", err)
			}
		}
		if verdictJSON.Valid && verdictJSON.String != "" {
			rec.Verdict = &JudgeVerdict{}
			if err := json.Unmarshal([]byte(verdictJSON.String), rec.Verdict); err != nil {
				// A half-decoded verdict is worse than none: it would count as a
				// judged execution with an empty verdict string, which reads as
				// neither PASS nor FAIL everywhere it is consumed.
				rec.Verdict = nil
				logExecutionRecordDecodeFailure(rec.TaskID, "verdict", err)
			}
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

	if fc.closed {
		return nil
	}
	fc.closed = true
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
