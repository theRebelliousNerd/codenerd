package northstar

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Store manages the Northstar knowledge database.
type Store struct {
	db     *sql.DB
	dbPath string
	mu     sync.RWMutex
}

// NewStore creates or opens a Northstar knowledge store.
func NewStore(nerdDir string) (*Store, error) {
	dbPath := filepath.Join(nerdDir, "northstar_knowledge.db")

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	store := &Store{
		db:     db,
		dbPath: dbPath,
	}

	if err := store.initSchema(); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			return nil, fmt.Errorf("failed to initialize schema: %w (close error: %v)", err, closeErr)
		}
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return store, nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// Path returns the database file path.
func (s *Store) Path() string {
	return s.dbPath
}

// initSchema creates the database schema.
func (s *Store) initSchema() error {
	schema := `
	-- Vision definition table
	CREATE TABLE IF NOT EXISTS vision (
		id INTEGER PRIMARY KEY CHECK (id = 1),
		mission TEXT NOT NULL,
		problem TEXT NOT NULL,
		vision_statement TEXT NOT NULL,
		personas_json TEXT,
		capabilities_json TEXT,
		risks_json TEXT,
		requirements_json TEXT,
		constraints_json TEXT,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL
	);

	-- Observations table
	CREATE TABLE IF NOT EXISTS observations (
		id TEXT PRIMARY KEY,
		session_id TEXT NOT NULL,
		timestamp DATETIME NOT NULL,
		type TEXT NOT NULL,
		subject TEXT NOT NULL,
		content TEXT NOT NULL,
		relevance REAL NOT NULL,
		tags_json TEXT,
		metadata_json TEXT
	);
	CREATE INDEX IF NOT EXISTS idx_observations_session ON observations(session_id);
	CREATE INDEX IF NOT EXISTS idx_observations_type ON observations(type);
	CREATE INDEX IF NOT EXISTS idx_observations_timestamp ON observations(timestamp);

	-- Alignment checks table
	CREATE TABLE IF NOT EXISTS alignment_checks (
		id TEXT PRIMARY KEY,
		timestamp DATETIME NOT NULL,
		trigger TEXT NOT NULL,
		subject TEXT NOT NULL,
		context TEXT,
		result TEXT NOT NULL,
		score REAL NOT NULL,
		explanation TEXT,
		suggestions_json TEXT,
		duration_ms INTEGER NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_checks_timestamp ON alignment_checks(timestamp);
	CREATE INDEX IF NOT EXISTS idx_checks_result ON alignment_checks(result);

	-- Drift events table
	CREATE TABLE IF NOT EXISTS drift_events (
		id TEXT PRIMARY KEY,
		timestamp DATETIME NOT NULL,
		severity TEXT NOT NULL,
		category TEXT NOT NULL,
		description TEXT NOT NULL,
		evidence_json TEXT,
		related_check TEXT,
		resolved INTEGER NOT NULL DEFAULT 0,
		resolved_at DATETIME,
		resolution TEXT,
		FOREIGN KEY (related_check) REFERENCES alignment_checks(id)
	);
	CREATE INDEX IF NOT EXISTS idx_drift_severity ON drift_events(severity);
	CREATE INDEX IF NOT EXISTS idx_drift_resolved ON drift_events(resolved);

	-- Guardian state table
	CREATE TABLE IF NOT EXISTS guardian_state (
		id INTEGER PRIMARY KEY CHECK (id = 1),
		vision_defined INTEGER NOT NULL DEFAULT 0,
		last_check DATETIME,
		tasks_since_check INTEGER NOT NULL DEFAULT 0,
		active_drift_count INTEGER NOT NULL DEFAULT 0,
		overall_alignment REAL NOT NULL DEFAULT 1.0,
		session_observations INTEGER NOT NULL DEFAULT 0
	);

	-- Insert default state if not exists
	INSERT OR IGNORE INTO guardian_state (id, vision_defined, tasks_since_check, active_drift_count, overall_alignment, session_observations)
	VALUES (1, 0, 0, 0, 1.0, 0);

	-- Document ingestion table (for Northstar-specific docs)
	CREATE TABLE IF NOT EXISTS ingested_docs (
		id TEXT PRIMARY KEY,
		path TEXT NOT NULL,
		title TEXT,
		content TEXT NOT NULL,
		summary TEXT,
		relevance REAL NOT NULL,
		ingested_at DATETIME NOT NULL,
		embedding BLOB
	);
	CREATE INDEX IF NOT EXISTS idx_docs_relevance ON ingested_docs(relevance);
	`

	_, err := s.db.Exec(schema)
	return err
}

// =============================================================================
// VISION OPERATIONS
// =============================================================================

// SaveVision stores or updates the project vision.
func (s *Store) SaveVision(v *Vision) error {
	if v == nil {
		return fmt.Errorf("vision is nil")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	personasJSON, err := marshalJSONString("vision personas", v.Personas)
	if err != nil {
		return err
	}
	capsJSON, err := marshalJSONString("vision capabilities", v.Capabilities)
	if err != nil {
		return err
	}
	risksJSON, err := marshalJSONString("vision risks", v.Risks)
	if err != nil {
		return err
	}
	reqsJSON, err := marshalJSONString("vision requirements", v.Requirements)
	if err != nil {
		return err
	}
	constraintsJSON, err := marshalJSONString("vision constraints", v.Constraints)
	if err != nil {
		return err
	}

	now := time.Now()
	if v.CreatedAt.IsZero() {
		createdAt, err := s.lookupVisionCreatedAt()
		if err != nil {
			return err
		}
		if createdAt.IsZero() {
			createdAt = now
		}
		v.CreatedAt = createdAt
	}
	v.UpdatedAt = now

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin save vision transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.Exec(`
		INSERT INTO vision (id, mission, problem, vision_statement, personas_json,
			capabilities_json, risks_json, requirements_json, constraints_json,
			created_at, updated_at)
		VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			mission = excluded.mission,
			problem = excluded.problem,
			vision_statement = excluded.vision_statement,
			personas_json = excluded.personas_json,
			capabilities_json = excluded.capabilities_json,
			risks_json = excluded.risks_json,
			requirements_json = excluded.requirements_json,
			constraints_json = excluded.constraints_json,
			updated_at = excluded.updated_at
	`, v.Mission, v.Problem, v.VisionStmt, personasJSON, capsJSON, risksJSON,
		reqsJSON, constraintsJSON, v.CreatedAt, v.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to save vision: %w", err)
	}

	// Update guardian state
	if _, err := tx.Exec(`UPDATE guardian_state SET vision_defined = 1 WHERE id = 1`); err != nil {
		return fmt.Errorf("failed to update guardian state for vision: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit save vision transaction: %w", err)
	}

	return nil
}

// LoadVision retrieves the project vision.
func (s *Store) LoadVision() (*Vision, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var v Vision
	var personasJSON, capsJSON, risksJSON, reqsJSON, constraintsJSON sql.NullString

	err := s.db.QueryRow(`
		SELECT mission, problem, vision_statement, personas_json, capabilities_json,
			risks_json, requirements_json, constraints_json, created_at, updated_at
		FROM vision WHERE id = 1
	`).Scan(&v.Mission, &v.Problem, &v.VisionStmt, &personasJSON, &capsJSON,
		&risksJSON, &reqsJSON, &constraintsJSON, &v.CreatedAt, &v.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil // No vision defined
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load vision: %w", err)
	}

	if err := unmarshalJSONField("vision personas", personasJSON, &v.Personas); err != nil {
		return nil, err
	}
	if err := unmarshalJSONField("vision capabilities", capsJSON, &v.Capabilities); err != nil {
		return nil, err
	}
	if err := unmarshalJSONField("vision risks", risksJSON, &v.Risks); err != nil {
		return nil, err
	}
	if err := unmarshalJSONField("vision requirements", reqsJSON, &v.Requirements); err != nil {
		return nil, err
	}
	if err := unmarshalJSONField("vision constraints", constraintsJSON, &v.Constraints); err != nil {
		return nil, err
	}

	return &v, nil
}

// HasVision returns true if a vision is defined.
func (s *Store) HasVision() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM vision WHERE id = 1`).Scan(&count); err != nil {
		return false
	}
	return count > 0
}

// =============================================================================
// OBSERVATION OPERATIONS
// =============================================================================

// RecordObservation stores a new observation.
func (s *Store) RecordObservation(obs *Observation) error {
	if obs == nil {
		return fmt.Errorf("observation is nil")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if obs.ID == "" {
		obs.ID = fmt.Sprintf("obs-%d", time.Now().UnixNano())
	}
	if obs.Timestamp.IsZero() {
		obs.Timestamp = time.Now()
	}

	tagsJSON, err := marshalJSONString("observation tags", obs.Tags)
	if err != nil {
		return err
	}
	metaJSON, err := marshalJSONString("observation metadata", obs.Metadata)
	if err != nil {
		return err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin record observation transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.Exec(`
		INSERT INTO observations (id, session_id, timestamp, type, subject, content,
			relevance, tags_json, metadata_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, obs.ID, obs.SessionID, obs.Timestamp, obs.Type, obs.Subject, obs.Content,
		obs.Relevance, tagsJSON, metaJSON)

	if err != nil {
		return fmt.Errorf("failed to record observation: %w", err)
	}

	// Update session observation count
	if _, err := tx.Exec(`UPDATE guardian_state SET session_observations = session_observations + 1 WHERE id = 1`); err != nil {
		return fmt.Errorf("failed to update observation count: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit record observation transaction: %w", err)
	}

	return nil
}

// GetRecentObservations retrieves recent observations.
func (s *Store) GetRecentObservations(limit int) ([]Observation, error) {
	if limit <= 0 {
		return []Observation{}, nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
		SELECT id, session_id, timestamp, type, subject, content, relevance, tags_json, metadata_json
		FROM observations
		ORDER BY timestamp DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var observations []Observation
	for rows.Next() {
		var obs Observation
		var tagsJSON, metaJSON sql.NullString
		if err := rows.Scan(&obs.ID, &obs.SessionID, &obs.Timestamp, &obs.Type,
			&obs.Subject, &obs.Content, &obs.Relevance, &tagsJSON, &metaJSON); err != nil {
			return nil, fmt.Errorf("scan observation: %w", err)
		}
		if err := unmarshalJSONField("observation tags", tagsJSON, &obs.Tags); err != nil {
			return nil, err
		}
		if err := unmarshalJSONField("observation metadata", metaJSON, &obs.Metadata); err != nil {
			return nil, err
		}
		observations = append(observations, obs)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate observations: %w", err)
	}
	return observations, nil
}

// =============================================================================
// ALIGNMENT CHECK OPERATIONS
// =============================================================================

// RecordAlignmentCheck stores an alignment check result.
func (s *Store) RecordAlignmentCheck(check *AlignmentCheck) error {
	if check == nil {
		return fmt.Errorf("alignment check is nil")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if check.ID == "" {
		check.ID = fmt.Sprintf("check-%d", time.Now().UnixNano())
	}
	if check.Timestamp.IsZero() {
		check.Timestamp = time.Now()
	}

	suggestionsJSON, err := marshalJSONString("alignment suggestions", check.Suggestions)
	if err != nil {
		return err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin record alignment check transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.Exec(`
		INSERT INTO alignment_checks (id, timestamp, trigger, subject, context,
			result, score, explanation, suggestions_json, duration_ms)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, check.ID, check.Timestamp, check.Trigger, check.Subject, check.Context,
		check.Result, check.Score, check.Explanation, suggestionsJSON,
		check.Duration.Milliseconds())

	if err != nil {
		return fmt.Errorf("failed to record alignment check: %w", err)
	}

	// Update guardian state
	_, err = tx.Exec(`
		UPDATE guardian_state SET
			last_check = ?,
			tasks_since_check = 0,
			overall_alignment = (overall_alignment * 0.8 + ? * 0.2)
		WHERE id = 1
	`, check.Timestamp, check.Score)

	if err != nil {
		return fmt.Errorf("failed to update guardian state for alignment check: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit record alignment check transaction: %w", err)
	}

	return nil
}

// GetAlignmentHistory retrieves alignment check history.
func (s *Store) GetAlignmentHistory(limit int) ([]AlignmentCheck, error) {
	if limit <= 0 {
		return []AlignmentCheck{}, nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
		SELECT id, timestamp, trigger, subject, context, result, score,
			explanation, suggestions_json, duration_ms
		FROM alignment_checks
		ORDER BY timestamp DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var checks []AlignmentCheck
	for rows.Next() {
		var check AlignmentCheck
		var context, explanation sql.NullString
		var suggestionsJSON sql.NullString
		var durationMs int64
		if err := rows.Scan(&check.ID, &check.Timestamp, &check.Trigger, &check.Subject,
			&context, &check.Result, &check.Score, &explanation, &suggestionsJSON, &durationMs); err != nil {
			return nil, fmt.Errorf("scan alignment check: %w", err)
		}
		check.Context = context.String
		check.Explanation = explanation.String
		check.Duration = time.Duration(durationMs) * time.Millisecond
		if err := unmarshalJSONField("alignment suggestions", suggestionsJSON, &check.Suggestions); err != nil {
			return nil, err
		}
		checks = append(checks, check)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate alignment checks: %w", err)
	}
	return checks, nil
}

// =============================================================================
// DRIFT EVENT OPERATIONS
// =============================================================================

// RecordDriftEvent stores a drift event.
func (s *Store) RecordDriftEvent(drift *DriftEvent) error {
	if drift == nil {
		return fmt.Errorf("drift event is nil")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if drift.ID == "" {
		drift.ID = fmt.Sprintf("drift-%d", time.Now().UnixNano())
	}
	if drift.Timestamp.IsZero() {
		drift.Timestamp = time.Now()
	}

	evidenceJSON, err := marshalJSONString("drift evidence", drift.Evidence)
	if err != nil {
		return err
	}
	var relatedCheck interface{}
	if drift.RelatedCheck != "" {
		relatedCheck = drift.RelatedCheck
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin record drift event transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.Exec(`
		INSERT INTO drift_events (id, timestamp, severity, category, description,
			evidence_json, related_check, resolved)
		VALUES (?, ?, ?, ?, ?, ?, ?, 0)
	`, drift.ID, drift.Timestamp, drift.Severity, drift.Category, drift.Description,
		evidenceJSON, relatedCheck)

	if err != nil {
		return fmt.Errorf("failed to record drift event: %w", err)
	}

	// Update active drift count
	if _, err := tx.Exec(`UPDATE guardian_state SET active_drift_count = active_drift_count + 1 WHERE id = 1`); err != nil {
		return fmt.Errorf("failed to update guardian drift count: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit record drift event transaction: %w", err)
	}

	return nil
}

// ResolveDriftEvent marks a drift event as resolved.
func (s *Store) ResolveDriftEvent(id string, resolution string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin resolve drift event transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now()
	result, err := tx.Exec(`
		UPDATE drift_events SET resolved = 1, resolved_at = ?, resolution = ?
		WHERE id = ? AND resolved = 0
	`, now, resolution, id)
	if err != nil {
		return err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read affected drift rows: %w", err)
	}
	if affected > 0 {
		if _, err := tx.Exec(`UPDATE guardian_state SET active_drift_count = MAX(0, active_drift_count - 1) WHERE id = 1`); err != nil {
			return fmt.Errorf("failed to update guardian drift count: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit resolve drift event transaction: %w", err)
	}
	return nil
}

// GetActiveDriftEvents retrieves unresolved drift events.
func (s *Store) GetActiveDriftEvents() ([]DriftEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
		SELECT id, timestamp, severity, category, description, evidence_json, related_check
		FROM drift_events
		WHERE resolved = 0
		ORDER BY timestamp DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []DriftEvent
	for rows.Next() {
		var event DriftEvent
		var evidenceJSON, relatedCheck sql.NullString
		if err := rows.Scan(&event.ID, &event.Timestamp, &event.Severity, &event.Category,
			&event.Description, &evidenceJSON, &relatedCheck); err != nil {
			return nil, fmt.Errorf("scan drift event: %w", err)
		}
		event.RelatedCheck = relatedCheck.String
		if err := unmarshalJSONField("drift evidence", evidenceJSON, &event.Evidence); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate drift events: %w", err)
	}
	return events, nil
}

// =============================================================================
// GUARDIAN STATE OPERATIONS
// =============================================================================

// GetState retrieves the current guardian state.
func (s *Store) GetState() (*GuardianState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var state GuardianState
	var lastCheck sql.NullTime

	err := s.db.QueryRow(`
		SELECT vision_defined, last_check, tasks_since_check, active_drift_count,
			overall_alignment, session_observations
		FROM guardian_state WHERE id = 1
	`).Scan(&state.VisionDefined, &lastCheck, &state.TasksSinceCheck,
		&state.ActiveDriftCount, &state.OverallAlignment, &state.SessionObservations)

	if err != nil {
		return nil, err
	}

	if lastCheck.Valid {
		state.LastCheck = lastCheck.Time
	}

	return &state, nil
}

// IncrementTaskCount increments the task counter since last check.
func (s *Store) IncrementTaskCount() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`UPDATE guardian_state SET tasks_since_check = tasks_since_check + 1 WHERE id = 1`)
	if err != nil {
		return 0, err
	}

	var count int
	if err := s.db.QueryRow(`SELECT tasks_since_check FROM guardian_state WHERE id = 1`).Scan(&count); err != nil {
		return 0, fmt.Errorf("load task count: %w", err)
	}
	return count, nil
}

// ResetSessionObservations resets the session observation counter.
func (s *Store) ResetSessionObservations() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`UPDATE guardian_state SET session_observations = 0 WHERE id = 1`)
	return err
}

func (s *Store) lookupVisionCreatedAt() (time.Time, error) {
	var createdAt time.Time
	err := s.db.QueryRow(`SELECT created_at FROM vision WHERE id = 1`).Scan(&createdAt)
	if err == sql.ErrNoRows {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("load existing vision created_at: %w", err)
	}
	return createdAt, nil
}

func marshalJSONString(field string, value interface{}) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("marshal %s: %w", field, err)
	}
	return string(raw), nil
}

func unmarshalJSONField(field string, raw sql.NullString, dest interface{}) error {
	if !raw.Valid || raw.String == "" {
		return nil
	}
	if err := json.Unmarshal([]byte(raw.String), dest); err != nil {
		return fmt.Errorf("unmarshal %s: %w", field, err)
	}
	return nil
}
