package store

import (
	"codenerd/internal/logging"
	"database/sql"
	"fmt"
)

// LearningCandidate represents a staged taxonomy learning candidate.
type LearningCandidate struct {
	ID     int64
	Phrase string
	Verb   string
	Target string
	Reason string
	// Fact is the proposed learned_exemplar, normalized, exactly as it will
	// be persisted: qualifier and confidence included. "" on a row staged
	// before proposals were stored; such a row's qualifier and confidence
	// are unknown, and it is not confirmed by inventing them.
	Fact      string
	Count     int
	Status    string
	CreatedAt string
	UpdatedAt string
}

// learningCandidatesDDL is the candidate table under name. A candidate's
// identity is its mapping, its reason and its whole proposed fact: two
// proposals of one mapping with different qualifiers or confidences are two
// candidates, not one counted twice.
func learningCandidatesDDL(name string) string {
	return `
	CREATE TABLE IF NOT EXISTS ` + name + ` (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		phrase TEXT NOT NULL,
		verb TEXT NOT NULL,
		target TEXT,
		reason TEXT,
		fact TEXT NOT NULL DEFAULT '',
		count INTEGER DEFAULT 1,
		status TEXT DEFAULT 'pending',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(phrase, verb, target, reason, fact)
	);`
}

const learningCandidatesIndexes = `
	CREATE INDEX IF NOT EXISTS idx_learning_candidates_status ON learning_candidates(status);
	CREATE INDEX IF NOT EXISTS idx_learning_candidates_phrase ON learning_candidates(phrase);
	`

// migrateLearningCandidatePayload rebuilds a learning_candidates table from
// before the fact column: its rows keep their ids, counts and statuses with
// fact "" (unknown), and the identity widens to include the fact. Staging
// used to keep only phrase, verb and target, so confirming a candidate by id
// rebuilt the fact with an empty qualifier and confidence 0.90 instead of
// what was proposed (audit manual-learning-evidence-conservation).
func migrateLearningCandidatePayload(db *sql.DB) error {
	if !tableExists(db, "learning_candidates") || columnExists(db, "learning_candidates", "fact") {
		return nil
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	stmts := []string{
		`DROP TABLE IF EXISTS learning_candidates_v2`,
		learningCandidatesDDL("learning_candidates_v2"),
		`INSERT INTO learning_candidates_v2 (id, phrase, verb, target, reason, fact, count, status, created_at, updated_at)
		 SELECT id, phrase, verb, target, reason, '', count, status, created_at, updated_at FROM learning_candidates`,
		`DROP TABLE learning_candidates`,
		`ALTER TABLE learning_candidates_v2 RENAME TO learning_candidates`,
		learningCandidatesIndexes,
	}
	for _, q := range stmts {
		if _, err := tx.Exec(q); err != nil {
			return fmt.Errorf("rebuild learning_candidates: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	logging.Store("Migrated learning_candidates: proposals are now stored whole")
	return nil
}

// RecordLearningCandidate stages a proposed learned fact for confirmation:
// its mapping (phrase, verb, target), why it was proposed, and the fact
// itself, normalized, as it will be persisted. Re-proposing the identical
// fact increments its count; a different fact for the same mapping is a
// different candidate.
func (s *LocalStore) RecordLearningCandidate(phrase, verb, target, reason, fact string) (int, error) {
	if s == nil || s.db == nil {
		return 0, fmt.Errorf("local store not initialized")
	}
	if phrase == "" {
		return 0, nil
	}
	if fact == "" {
		return 0, fmt.Errorf("learning candidate %q has no proposed fact", phrase)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`
		INSERT INTO learning_candidates (phrase, verb, target, reason, fact, count, status)
		VALUES (?, ?, ?, ?, ?, 1, 'pending')
		ON CONFLICT(phrase, verb, target, reason, fact) DO UPDATE SET
			count = count + 1,
			updated_at = CURRENT_TIMESTAMP
	`, phrase, verb, target, reason, fact)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to record learning candidate: %v", err)
		return 0, err
	}

	var count int
	if err := s.db.QueryRow(`
		SELECT count
		FROM learning_candidates
		WHERE phrase = ? AND verb = ? AND target = ? AND reason = ? AND fact = ?
	`, phrase, verb, target, reason, fact).Scan(&count); err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to read learning candidate count: %v", err)
		return 0, err
	}

	return count, nil
}

// ListLearningCandidates returns candidates filtered by status (optional).
func (s *LocalStore) ListLearningCandidates(status string, limit int) ([]LearningCandidate, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("local store not initialized")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
		SELECT id, phrase, verb, target, reason, fact, count, status, created_at, updated_at
		FROM learning_candidates
	`
	var args []any
	if status != "" {
		query += " WHERE status = ?"
		args = append(args, status)
	}
	// updated_at has one-second resolution, so same-second rows need the id
	// tiebreak for a deterministic listing.
	query += " ORDER BY updated_at DESC, id DESC"
	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []LearningCandidate
	for rows.Next() {
		var cand LearningCandidate
		if err := rows.Scan(&cand.ID, &cand.Phrase, &cand.Verb, &cand.Target, &cand.Reason, &cand.Fact, &cand.Count, &cand.Status, &cand.CreatedAt, &cand.UpdatedAt); err != nil {
			// One malformed row never hides the rest, but it must not
			// vanish silently either: name it so the row can be found.
			logging.Get(logging.CategoryStore).Warn("Skipping malformed learning candidate row: %v", err)
			continue
		}
		results = append(results, cand)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

// ConfirmLearningCandidate marks a candidate as confirmed.
func (s *LocalStore) ConfirmLearningCandidate(id int64) error {
	return s.updateLearningCandidateStatus(id, "confirmed")
}

// RejectLearningCandidate marks a candidate as rejected.
func (s *LocalStore) RejectLearningCandidate(id int64) error {
	return s.updateLearningCandidateStatus(id, "rejected")
}

// ConfirmLearningCandidateMatch marks a candidate as confirmed by its fields.
func (s *LocalStore) ConfirmLearningCandidateMatch(phrase, verb, target, reason, fact string) error {
	return s.updateLearningCandidateStatusMatch(phrase, verb, target, reason, fact, "confirmed")
}

// RejectLearningCandidateMatch marks a candidate as rejected by its fields.
func (s *LocalStore) RejectLearningCandidateMatch(phrase, verb, target, reason, fact string) error {
	return s.updateLearningCandidateStatusMatch(phrase, verb, target, reason, fact, "rejected")
}

func (s *LocalStore) updateLearningCandidateStatus(id int64, status string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("local store not initialized")
	}
	if id <= 0 {
		return fmt.Errorf("invalid candidate id")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	result, err := s.db.Exec(`
		UPDATE learning_candidates
		SET status = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, status, id)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *LocalStore) updateLearningCandidateStatusMatch(phrase, verb, target, reason, fact, status string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("local store not initialized")
	}
	if phrase == "" {
		return fmt.Errorf("invalid candidate phrase")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	result, err := s.db.Exec(`
		UPDATE learning_candidates
		SET status = ?, updated_at = CURRENT_TIMESTAMP
		WHERE phrase = ? AND verb = ? AND target = ? AND reason = ? AND fact = ?
	`, status, phrase, verb, target, reason, fact)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}
