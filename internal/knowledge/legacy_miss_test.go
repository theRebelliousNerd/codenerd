package knowledge

import (
	"path/filepath"
	"testing"
	"time"

	"codenerd/internal/store"
	"codenerd/internal/types"
)

func missQueries(t *testing.T, facts []types.Fact) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	for _, f := range facts {
		if f.Predicate == "knowledge_miss" {
			out[types.ExtractString(f.Args[1])] = true
		}
	}
	return out
}

// Miss rows written before searches recorded their outcome could stand for a
// row the budget left out or a failed search; they were measured as gaps all
// the same. They are now no gap until settled from what a search shows: a
// question the KB returns rows for becomes history, one it still finds
// nothing for becomes a current miss, and a later genuine miss is current.
func TestLegacyMisses_AreHistoryUnlessASearchShowsTheGap(t *testing.T) {
	db, err := store.NewLocalStore(filepath.Join(t.TempDir(), "kb.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.StoreKnowledgeAtom("web/widget calibration", "Calibrate a widget against the reference table.", 0.8, "web/probe"); err != nil {
		t.Fatal(err)
	}
	// The table as the old recorder made it.
	if _, err := db.GetDB().Exec(`CREATE TABLE knowledge_misses (query TEXT PRIMARY KEY, misses INTEGER NOT NULL, last_missed_at TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC().Format(time.RFC3339)
	for _, q := range []string{"widget calibration reference table", "zephyrquark xyzzy plover"} {
		if _, err := db.GetDB().Exec(`INSERT INTO knowledge_misses (query, misses, last_missed_at) VALUES (?, 3, ?)`, q, at); err != nil {
			t.Fatal(err)
		}
	}
	ctx := t.Context()
	agent := types.MangleAtom("/widgetexpert")

	facts, err := Measure(ctx, db.GetDB(), agent, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if got := missQueries(t, facts); len(got) != 0 {
		t.Fatalf("unsettled legacy misses were measured as gaps: %v", got)
	}

	current, history, err := SettleLegacyMisses(ctx, db.GetDB())
	if err != nil || current != 1 || history != 1 {
		t.Fatalf("settled current=%d history=%d (err %v), want 1 and 1", current, history, err)
	}
	facts, err = Measure(ctx, db.GetDB(), agent, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	got := missQueries(t, facts)
	if len(got) != 1 || !got["zephyrquark xyzzy plover"] {
		t.Fatalf("after settling the gaps are %v, want only the question the KB cannot answer", got)
	}

	if err := RecordMiss(ctx, db.GetDB(), "widget calibration reference table", time.Now()); err != nil {
		t.Fatal(err)
	}
	facts, err = Measure(ctx, db.GetDB(), agent, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !missQueries(t, facts)["widget calibration reference table"] {
		t.Fatal("a genuine miss after settling is not a current gap")
	}
}
