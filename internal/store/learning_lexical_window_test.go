package store

import (
	"fmt"
	"testing"
	"time"
)

// Lexical recall took each shard's newest limit*3 matching rows and only
// then scored keyword coverage, so an older learning matching every keyword
// lost to fifteen newer ones matching one, and the global sort could not get
// it back (audit learning-lexical-candidate-window). Coverage now ranks
// before the cap, recency breaking ties.
func TestRecallLearningsLexical_FullCoverageIsNotCutByNewerWeakMatches(t *testing.T) {
	ls, err := NewLearningStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer ls.Close()
	const shard = "coder"
	if err := ls.Save(shard, "learned_pattern", []any{"walcheckpoint eviction replay recovery"}, "c"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 15; i++ {
		if err := ls.Save(shard, "learned_pattern", []any{fmt.Sprintf("eviction note %d", i)}, "c"); err != nil {
			t.Fatal(err)
		}
	}
	db, err := ls.getDB(shard)
	if err != nil {
		t.Fatal(err)
	}
	// Strictly ordered: the full match is the oldest.
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	rows, err := db.Query(`SELECT id, fact_args FROM learnings ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	var ids []int64
	for rows.Next() {
		var id int64
		var a string
		if err := rows.Scan(&id, &a); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	rows.Close()
	for i, id := range ids {
		if _, err := db.Exec(`UPDATE learnings SET learned_at = ? WHERE id = ?`, base.Add(time.Duration(i)*time.Minute), id); err != nil {
			t.Fatal(err)
		}
	}

	hits, err := ls.RecallLearningsLexical("walcheckpoint eviction replay recovery", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 || hits[0].LearningID != ids[0] || hits[0].Score != 1 {
		t.Fatalf("top hit = %+v, want the older learning matching all four keywords at score 1", hits)
	}
	// Among the weak matches, the newest come first.
	if len(hits) > 2 && hits[1].LearnedAt.Before(hits[2].LearnedAt) {
		t.Fatalf("equal-score hits are not newest first: %v then %v", hits[1].LearnedAt, hits[2].LearnedAt)
	}
}
