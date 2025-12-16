package store

import (
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
