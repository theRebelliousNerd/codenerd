package store

import (
	"testing"
)

func TestLocalStore_ReviewFinding_Extra(t *testing.T) {
	s, err := NewLocalStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer s.Close()

	finding := StoredReviewFinding{
		FilePath:    "test.go",
		Line:        10,
		Severity:    "ERROR",
		Category:    "style",
		RuleID:      "rule1",
		Message:     "bad style",
		ProjectRoot: "/",
	}

	err = s.StoreReviewFinding(finding)
	if err != nil {
		t.Errorf("StoreReviewFinding failed: %v", err)
	}

	// Verify manually
	var count int
	err = s.db.QueryRow("SELECT COUNT(*) FROM review_findings WHERE file_path = 'test.go'").Scan(&count)
	if err != nil {
		t.Errorf("Failed to query db manually: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 review finding, got %d", count)
	}
}
