package store

import (
	"fmt"

	"codenerd/internal/logging"
)

// =============================================================================
// REVIEW FINDINGS STORAGE
// =============================================================================

// StoredReviewFinding represents a review finding to be persisted.
// Defined here to avoid circular dependency with reviewer package.
type StoredReviewFinding struct {
	FilePath    string
	Line        int
	Severity    string
	Category    string
	RuleID      string
	Message     string
	ProjectRoot string
}

// StoreReviewFinding persists a review finding to the database.
func (s *LocalStore) StoreReviewFinding(f StoredReviewFinding) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("local store not initialized")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	logging.StoreDebug("Storing review finding: file=%s line=%d severity=%s category=%s rule=%s",
		f.FilePath, f.Line, f.Severity, f.Category, f.RuleID)

	_, err := s.db.Exec(
		`INSERT INTO review_findings (file_path, line, severity, category, rule_id, message, project_root)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		f.FilePath, f.Line, f.Severity, f.Category, f.RuleID, f.Message, f.ProjectRoot,
	)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to store review finding for %s:%d: %v", f.FilePath, f.Line, err)
		return err
	}

	logging.StoreDebug("Review finding stored: %s:%d [%s]", f.FilePath, f.Line, f.Severity)
	return nil
}

// ListReviewFindings returns stored review findings, newest first, bounded by
// limit (non-positive means all). An empty projectRoot matches every project;
// otherwise only findings recorded for that root are returned.
//
// The table used to be write-only: the reviewer persisted findings nothing
// ever read back. This reader wires the history into the system so past
// findings can actually inform later reviews.
func (s *LocalStore) ListReviewFindings(projectRoot string, limit int) ([]StoredReviewFinding, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("local store not initialized")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `SELECT file_path, line, severity, category, rule_id, message, project_root
		FROM review_findings`
	var args []any
	if projectRoot != "" {
		query += " WHERE project_root = ?"
		args = append(args, projectRoot)
	}
	query += " ORDER BY id DESC"
	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list review findings: %w", err)
	}
	defer rows.Close()

	var out []StoredReviewFinding
	for rows.Next() {
		var f StoredReviewFinding
		if err := rows.Scan(&f.FilePath, &f.Line, &f.Severity, &f.Category, &f.RuleID, &f.Message, &f.ProjectRoot); err != nil {
			logging.Get(logging.CategoryStore).Warn("Skipping malformed review finding row: %v", err)
			continue
		}
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to list review findings: %w", err)
	}
	return out, nil
}
