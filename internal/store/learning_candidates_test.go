package store

import (
	"path/filepath"
	"testing"
)

func TestLearningCandidateStoreRecordAndConfirm(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "knowledge.db")
	store, err := NewLocalStore(dbPath)
	if err != nil {
		t.Fatalf("NewLocalStore() error = %v", err)
	}

	count, err := store.RecordLearningCandidate("deploy service", "/deploy", "service", "/unknown_verb")
	if err != nil {
		t.Fatalf("RecordLearningCandidate error = %v", err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1", count)
	}

	count, err = store.RecordLearningCandidate("deploy service", "/deploy", "service", "/unknown_verb")
	if err != nil {
		t.Fatalf("RecordLearningCandidate error = %v", err)
	}
	if count != 2 {
		t.Fatalf("count = %d, want 2", count)
	}

	pending, err := store.ListLearningCandidates("pending", 10)
	if err != nil {
		t.Fatalf("ListLearningCandidates error = %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("pending candidates = %d, want 1", len(pending))
	}
	if pending[0].Count != 2 {
		t.Fatalf("pending count = %d, want 2", pending[0].Count)
	}

	if err := store.ConfirmLearningCandidate(pending[0].ID); err != nil {
		t.Fatalf("ConfirmLearningCandidate error = %v", err)
	}

	confirmed, err := store.ListLearningCandidates("confirmed", 10)
	if err != nil {
		t.Fatalf("ListLearningCandidates error = %v", err)
	}
	if len(confirmed) != 1 {
		t.Fatalf("confirmed candidates = %d, want 1", len(confirmed))
	}
	if confirmed[0].ID != pending[0].ID {
		t.Fatalf("confirmed candidate ID = %d, want %d", confirmed[0].ID, pending[0].ID)
	}
}
