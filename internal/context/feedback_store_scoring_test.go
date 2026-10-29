package context

import (
	"path/filepath"
	"testing"
)

// TestFeedbackUsefulnessScoring exercises the SQLite-backed usefulness scorers
// end to end: a predicate marked helpful across several turns should score
// above one marked as noise, both the global and per-intent variants should be
// cached, and GetOverallStats should reflect the stored feedback.
func TestFeedbackUsefulnessScoring(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "feedback_scoring.db")
	store, err := NewContextFeedbackStore(dbPath)
	if err != nil {
		t.Fatalf("NewContextFeedbackStore: %v", err)
	}
	defer store.Close()

	store.minSamples = 1
	for i := range 4 {
		if err := store.StoreFeedback(i+1, "", 0.8, "/fix", true,
			[]string{"file_topology"}, []string{"browser_state"}); err != nil {
			t.Fatalf("StoreFeedback: %v", err)
		}
	}

	helpful := store.GetPredicateUsefulness("file_topology")
	noise := store.GetPredicateUsefulness("browser_state")
	if helpful <= noise {
		t.Errorf("helpful predicate (%.2f) should outscore noise predicate (%.2f)", helpful, noise)
	}

	// A predicate with no samples scores neutral (0.0).
	if got := store.GetPredicateUsefulness("never_seen"); got != 0.0 {
		t.Errorf("unseen predicate usefulness=%.2f, want 0.0", got)
	}

	// The per-intent variant is computed and cached under a distinct key.
	intentScore := store.GetPredicateUsefulnessForIntent("file_topology", "/fix")
	if intentScore == 0.0 {
		t.Error("per-intent usefulness should be non-zero for a sampled helpful predicate")
	}

	total, avg, err := store.GetOverallStats()
	if err != nil {
		t.Fatalf("GetOverallStats: %v", err)
	}
	if total != 4 {
		t.Errorf("GetOverallStats total=%d, want 4", total)
	}
	if avg <= 0 {
		t.Errorf("GetOverallStats avg=%.2f, want > 0", avg)
	}
}
