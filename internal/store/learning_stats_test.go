package store

import (
	"strings"
	"testing"
)

// GetStats ran two QueryRow.Scan calls, a Query and a rows.Scan with every
// error dropped. A shard whose learnings table was missing or unreadable
// reported total_learnings 0, avg_confidence 0 and an empty by_predicate map —
// exactly what a healthy shard that has learned nothing reports. "Nothing
// learned yet" and "the learning store is broken" are opposite conclusions.

func TestLearningStore_GetStats_EmptyShardIsNotAnError(t *testing.T) {
	ls, err := NewLearningStore(t.TempDir())
	if err != nil {
		t.Fatalf("new learning store: %v", err)
	}
	defer ls.Close()

	stats, err := ls.GetStats("coder")
	if err != nil {
		t.Fatalf("a shard with no learnings must not be an error: %v", err)
	}
	if stats["total_learnings"].(int64) != 0 {
		t.Errorf("expected zero learnings, got %v", stats["total_learnings"])
	}
	// AVG over an empty table is NULL. Scanning it into a bare float64 fails,
	// and that failure used to be dropped.
	if stats["avg_confidence"].(float64) != 0 {
		t.Errorf("expected zero average confidence, got %v", stats["avg_confidence"])
	}
}

func TestLearningStore_GetStats_ReportsRealCounts(t *testing.T) {
	ls, err := NewLearningStore(t.TempDir())
	if err != nil {
		t.Fatalf("new learning store: %v", err)
	}
	defer ls.Close()

	if err := ls.Save("coder", "approach_learned", []any{"hypothetical", "content"}, "dream_state"); err != nil {
		t.Fatalf("save: %v", err)
	}

	stats, err := ls.GetStats("coder")
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if stats["total_learnings"].(int64) != 1 {
		t.Errorf("expected one learning, got %v", stats["total_learnings"])
	}
	byPred := stats["by_predicate"].(map[string]int64)
	if byPred["approach_learned"] != 1 {
		t.Errorf("expected the predicate counted, got %v", byPred)
	}
}

func TestLearningStore_GetStats_BrokenTableIsReported(t *testing.T) {
	ls, err := NewLearningStore(t.TempDir())
	if err != nil {
		t.Fatalf("new learning store: %v", err)
	}
	defer ls.Close()

	db, err := ls.getDB("coder")
	if err != nil {
		t.Fatalf("getDB: %v", err)
	}
	if _, err := db.Exec("DROP TABLE learnings"); err != nil {
		t.Fatalf("drop table: %v", err)
	}

	stats, err := ls.GetStats("coder")
	if err == nil {
		t.Fatalf("a shard with no learnings table reported clean stats: %v", stats)
	}
	if !strings.Contains(err.Error(), "coder") {
		t.Errorf("error should name the shard, got: %v", err)
	}
}
