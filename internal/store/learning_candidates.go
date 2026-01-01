package store

import (
	"codenerd/internal/logging"
	"database/sql"
	"fmt"
)

// LearningCandidate represents a staged taxonomy learning candidate.
type LearningCandidate struct {
	ID        int64
	Phrase    string
	Verb      string
	Target    string
	Reason    string
	Count     int
	Status    string
	CreatedAt string
	UpdatedAt string
}

// RecordLearningCandidate increments a candidate count or creates a new one.
func (s *LocalStore) RecordLearningCandidate(phrase, verb, target, reason string) (int, error) {
	if s == nil || s.db == nil {
		return 0, fmt.Errorf("local store not initialized")
	}
	if phrase == "" {
		return 0, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`
		INSERT INTO learning_candidates (phrase, verb, target, reason, count, status)
		VALUES (?, ?, ?, ?, 1, 'pending')
		ON CONFLICT(phrase, verb, target, reason) DO UPDATE SET
			count = count + 1,
			updated_at = CURRENT_TIMESTAMP
	`, phrase, verb, target, reason)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to record learning candidate: %v", err)
		return 0, err
	}

	var count int
	if err := s.db.QueryRow(`
		SELECT count
		FROM learning_candidates
		WHERE phrase = ? AND verb = ? AND target = ? AND reason = ?
	`, phrase, verb, target, reason).Scan(&count); err != nil {
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
		SELECT id, phrase, verb, target, reason, count, status, created_at, updated_at
		FROM learning_candidates
	`
	var args []interface{}
	if status != "" {
		query += " WHERE status = ?"
		args = append(args, status)
	}
	query += " ORDER BY updated_at DESC"
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
		if err := rows.Scan(&cand.ID, &cand.Phrase, &cand.Verb, &cand.Target, &cand.Reason, &cand.Count, &cand.Status, &cand.CreatedAt, &cand.UpdatedAt); err != nil {
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
