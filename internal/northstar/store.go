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
	db       *sql.DB
	dbPath   string
	mu       sync.RWMutex
}

// NewStore creates or opens a Northstar knowledge store.
func NewStore(nerdDir string) (*Store, error) {
	dbPath := filepath.Join(nerdDir, "northstar_knowledge.db")

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	store := &Store{
		db:     db,
		dbPath: dbPath,
	}

	if err := store.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return store, nil
}

// Close closes the database connection.
func (s *Store) Close() error {
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
	s.mu.Lock()
	defer s.mu.Unlock()

	personasJSON, _ := json.Marshal(v.Personas)
	capsJSON, _ := json.Marshal(v.Capabilities)
	risksJSON, _ := json.Marshal(v.Risks)
	reqsJSON, _ := json.Marshal(v.Requirements)
	constraintsJSON, _ := json.Marshal(v.Constraints)

	now := time.Now()
	if v.CreatedAt.IsZero() {
		v.CreatedAt = now
	}
	v.UpdatedAt = now

	_, err := s.db.Exec(`
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
	_, err = s.db.Exec(`UPDATE guardian_state SET vision_defined = 1 WHERE id = 1`)
	return err
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

	if personasJSON.Valid {
		json.Unmarshal([]byte(personasJSON.String), &v.Personas)
	}
	if capsJSON.Valid {
		json.Unmarshal([]byte(capsJSON.String), &v.Capabilities)
	}
	if risksJSON.Valid {
		json.Unmarshal([]byte(risksJSON.String), &v.Risks)
	}
	if reqsJSON.Valid {
		json.Unmarshal([]byte(reqsJSON.String), &v.Requirements)
	}
	if constraintsJSON.Valid {
		json.Unmarshal([]byte(constraintsJSON.String), &v.Constraints)
	}

	return &v, nil
}

// HasVision returns true if a vision is defined.
func (s *Store) HasVision() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var count int
	s.db.QueryRow(`SELECT COUNT(*) FROM vision WHERE id = 1`).Scan(&count)
	return count > 0
}

// =============================================================================
// OBSERVATION OPERATIONS
// =============================================================================

// RecordObservation stores a new observation.
func (s *Store) RecordObservation(obs *Observation) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if obs.ID == "" {
		obs.ID = fmt.Sprintf("obs-%d", time.Now().UnixNano())
	}
	if obs.Timestamp.IsZero() {
		obs.Timestamp = time.Now()
	}

	tagsJSON, _ := json.Marshal(obs.Tags)
	metaJSON, _ := json.Marshal(obs.Metadata)

	_, err := s.db.Exec(`
		INSERT INTO observations (id, session_id, timestamp, type, subject, content,
			relevance, tags_json, metadata_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, obs.ID, obs.SessionID, obs.Timestamp, obs.Type, obs.Subject, obs.Content,
		obs.Relevance, tagsJSON, metaJSON)

	if err != nil {
		return fmt.Errorf("failed to record observation: %w", err)
	}

	// Update session observation count
	_, err = s.db.Exec(`UPDATE guardian_state SET session_observations = session_observations + 1 WHERE id = 1`)
	return err
}

// GetRecentObservations retrieves recent observations.
func (s *Store) GetRecentObservations(limit int) ([]Observation, error) {
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
			continue
		}
		if tagsJSON.Valid {
			json.Unmarshal([]byte(tagsJSON.String), &obs.Tags)
		}
		if metaJSON.Valid {
			json.Unmarshal([]byte(metaJSON.String), &obs.Metadata)
		}
		observations = append(observations, obs)
	}
	return observations, nil
}

// =============================================================================
// ALIGNMENT CHECK OPERATIONS
// =============================================================================

// RecordAlignmentCheck stores an alignment check result.
func (s *Store) RecordAlignmentCheck(check *AlignmentCheck) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if check.ID == "" {
		check.ID = fmt.Sprintf("check-%d", time.Now().UnixNano())
	}
	if check.Timestamp.IsZero() {
		check.Timestamp = time.Now()
	}

	suggestionsJSON, _ := json.Marshal(check.Suggestions)

	_, err := s.db.Exec(`
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
	_, err = s.db.Exec(`
		UPDATE guardian_state SET
			last_check = ?,
			tasks_since_check = 0,
			overall_alignment = (overall_alignment * 0.8 + ? * 0.2)
		WHERE id = 1
	`, check.Timestamp, check.Score)

	return err
}

// GetAlignmentHistory retrieves alignment check history.
func (s *Store) GetAlignmentHistory(limit int) ([]AlignmentCheck, error) {
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
			continue
		}
		check.Context = context.String
		check.Explanation = explanation.String
		check.Duration = time.Duration(durationMs) * time.Millisecond
		if suggestionsJSON.Valid {
			json.Unmarshal([]byte(suggestionsJSON.String), &check.Suggestions)
		}
		checks = append(checks, check)
	}
	return checks, nil
}

// =============================================================================
// DRIFT EVENT OPERATIONS
// =============================================================================

// RecordDriftEvent stores a drift event.
func (s *Store) RecordDriftEvent(drift *DriftEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if drift.ID == "" {
		drift.ID = fmt.Sprintf("drift-%d", time.Now().UnixNano())
	}
	if drift.Timestamp.IsZero() {
		drift.Timestamp = time.Now()
	}

	evidenceJSON, _ := json.Marshal(drift.Evidence)

	_, err := s.db.Exec(`
		INSERT INTO drift_events (id, timestamp, severity, category, description,
			evidence_json, related_check, resolved)
		VALUES (?, ?, ?, ?, ?, ?, ?, 0)
	`, drift.ID, drift.Timestamp, drift.Severity, drift.Category, drift.Description,
		evidenceJSON, drift.RelatedCheck)

	if err != nil {
		return fmt.Errorf("failed to record drift event: %w", err)
	}

	// Update active drift count
	_, err = s.db.Exec(`UPDATE guardian_state SET active_drift_count = active_drift_count + 1 WHERE id = 1`)
	return err
}

// ResolveDriftEvent marks a drift event as resolved.
func (s *Store) ResolveDriftEvent(id string, resolution string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	result, err := s.db.Exec(`
		UPDATE drift_events SET resolved = 1, resolved_at = ?, resolution = ?
		WHERE id = ? AND resolved = 0
	`, now, resolution, id)
	if err != nil {
		return err
	}

	affected, _ := result.RowsAffected()
	if affected > 0 {
		s.db.Exec(`UPDATE guardian_state SET active_drift_count = MAX(0, active_drift_count - 1) WHERE id = 1`)
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
			continue
		}
		event.RelatedCheck = relatedCheck.String
		if evidenceJSON.Valid {
			json.Unmarshal([]byte(evidenceJSON.String), &event.Evidence)
		}
		events = append(events, event)
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
	s.db.QueryRow(`SELECT tasks_since_check FROM guardian_state WHERE id = 1`).Scan(&count)
	return count, nil
}

// ResetSessionObservations resets the session observation counter.
func (s *Store) ResetSessionObservations() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`UPDATE guardian_state SET session_observations = 0 WHERE id = 1`)
	return err
}
