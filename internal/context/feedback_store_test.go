package context

import (
	"path/filepath"
	"testing"
)

func TestContextFeedbackStore_PersistsTaskSucceeded(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "feedback.db")

	store, err := NewContextFeedbackStore(dbPath)
	if err != nil {
		t.Fatalf("NewContextFeedbackStore failed: %v", err)
	}
	defer store.Close()

	if err := store.StoreFeedback(1, "hash-1", 0.8, "/fix", false, []string{"file_topology"}, []string{"browser_state"}); err != nil {
		t.Fatalf("StoreFeedback failed: %v", err)
	}

	var taskSucceeded int
	if err := store.db.QueryRow("SELECT task_succeeded FROM context_feedback WHERE turn_id = 1").Scan(&taskSucceeded); err != nil {
		t.Fatalf("failed to query stored feedback: %v", err)
	}
	if taskSucceeded != 0 {
		t.Fatalf("expected task_succeeded=0, got %d", taskSucceeded)
	}
}

func TestContextFeedbackStore_QueryHelpersReturnWithoutRecursiveLocking(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "feedback_helpers.db")

	store, err := NewContextFeedbackStore(dbPath)
	if err != nil {
		t.Fatalf("NewContextFeedbackStore failed: %v", err)
	}
	defer store.Close()

	store.minSamples = 1
	for i := 0; i < 3; i++ {
		if err := store.StoreFeedback(i+1, "", 0.9, "/fix", true, []string{"file_topology"}, []string{"browser_state"}); err != nil {
			t.Fatalf("StoreFeedback failed: %v", err)
		}
	}

	pf, err := store.GetPredicateFeedback("file_topology")
	if err != nil {
		t.Fatalf("GetPredicateFeedback failed: %v", err)
	}
	if pf == nil || pf.Predicate != "file_topology" {
		t.Fatalf("expected predicate feedback for file_topology, got %+v", pf)
	}

	helpful, err := store.GetTopHelpfulPredicates(5)
	if err != nil {
		t.Fatalf("GetTopHelpfulPredicates failed: %v", err)
	}
	if len(helpful) == 0 {
		t.Fatal("expected helpful predicates")
	}

	noise, err := store.GetTopNoisePredicates(5)
	if err != nil {
		t.Fatalf("GetTopNoisePredicates failed: %v", err)
	}
	if len(noise) == 0 {
		t.Fatal("expected noise predicates")
	}
}
