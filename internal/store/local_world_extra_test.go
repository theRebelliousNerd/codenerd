package store

import (
	"testing"
)

func TestLocalStore_WorldFilesAndFacts(t *testing.T) {
	s, err := NewLocalStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer s.Close()

	// Ensure tables exist
	_, err = s.db.Exec(`CREATE TABLE IF NOT EXISTS world_files (
		path TEXT PRIMARY KEY,
		lang TEXT,
		size INTEGER,
		modtime INTEGER,
		hash TEXT,
		fingerprint TEXT,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		t.Fatalf("failed to create world_files table: %v", err)
	}

	_, err = s.db.Exec(`CREATE TABLE IF NOT EXISTS world_facts (
		path TEXT,
		depth TEXT,
		fingerprint TEXT,
		predicate TEXT,
		args TEXT,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (path, depth, predicate, args)
	)`)
	if err != nil {
		t.Fatalf("failed to create world_facts table: %v", err)
	}

	meta := WorldFileMeta{
		Path:        "/test/path.go",
		Lang:        "go",
		Size:        123,
		ModTime:     456,
		Hash:        "hash",
		Fingerprint: "fingerprint",
	}

	// Test UpsertWorldFile
	if err := s.UpsertWorldFile(meta); err != nil {
		t.Errorf("UpsertWorldFile failed: %v", err)
	}

	// Update
	meta.Size = 456
	if err := s.UpsertWorldFile(meta); err != nil {
		t.Errorf("UpsertWorldFile update failed: %v", err)
	}

	// Test ReplaceWorldFactsForFile
	facts := []WorldFactInput{
		{Predicate: "test_pred", Args: []any{"arg1"}},
	}
	if err := s.ReplaceWorldFactsForFile("/test/path.go", "fast", "fingerprint", facts); err != nil {
		t.Errorf("ReplaceWorldFactsForFile failed: %v", err)
	}

	// Test ReplaceWorldFactsForFile empty depth
	if err := s.ReplaceWorldFactsForFile("/test/path2.go", "", "fingerprint", facts); err != nil {
		t.Errorf("ReplaceWorldFactsForFile failed: %v", err)
	}

	// Test LoadWorldFactsForFile
	loaded, fp, err := s.LoadWorldFactsForFile("/test/path.go", "fast")
	if err != nil {
		t.Errorf("LoadWorldFactsForFile failed: %v", err)
	}
	if fp != "fingerprint" {
		t.Errorf("Expected fingerprint 'fingerprint', got %s", fp)
	}
	if len(loaded) != 1 {
		t.Errorf("Expected 1 fact, got %d", len(loaded))
	}

	// Empty depth
	loaded2, _, err := s.LoadWorldFactsForFile("/test/path2.go", "")
	if err != nil {
		t.Errorf("LoadWorldFactsForFile empty depth failed: %v", err)
	}
	if len(loaded2) != 1 {
		t.Errorf("Expected 1 fact, got %d", len(loaded2))
	}

	// Test LoadAllWorldFacts
	allFacts, err := s.LoadAllWorldFacts("fast")
	if err != nil {
		t.Errorf("LoadAllWorldFacts failed: %v", err)
	}
	if len(allFacts) != 2 {
		t.Errorf("Expected 2 facts (from fast depth), got %d", len(allFacts))
	}

	// Test DeleteWorldFile
	if err := s.DeleteWorldFile("/test/path.go"); err != nil {
		t.Errorf("DeleteWorldFile failed: %v", err)
	}

	loaded3, _, _ := s.LoadWorldFactsForFile("/test/path.go", "fast")
	if len(loaded3) != 0 {
		t.Errorf("Expected 0 facts after delete, got %d", len(loaded3))
	}
}
