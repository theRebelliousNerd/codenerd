package knowledge

import (
	"context"
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
	fixedNow := time.Date(2026, 2, 10, 12, 0, 0, 0, time.UTC)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
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
	// legacySettleRows bounds each settling search: only the top rows decide.
	if legacySettleRows != 5 {
		t.Fatalf("legacySettleRows = %d, want 5", legacySettleRows)
	}
	at := fixedNow.UTC().Format(time.RFC3339)
	// Insert in reverse sorted order to prove SettleLegacyMisses settles in
	// deterministic query order (ORDER BY query), not insertion order.
	for _, q := range []string{"zephyrquark xyzzy plover", "widget calibration reference table"} {
		if _, err := db.GetDB().Exec(`INSERT INTO knowledge_misses (query, misses, last_missed_at) VALUES (?, 3, ?)`, q, at); err != nil {
			t.Fatal(err)
		}
	}
	agent := types.MangleAtom("/widgetexpert")

	facts, err := Measure(ctx, db.GetDB(), agent, fixedNow)
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
	facts, err = Measure(ctx, db.GetDB(), agent, fixedNow)
	if err != nil {
		t.Fatal(err)
	}
	got := missQueries(t, facts)
	if len(got) != 1 || !got["zephyrquark xyzzy plover"] {
		t.Fatalf("after settling the gaps are %v, want only the question the KB cannot answer", got)
	}

	// RecordRetrieval is clock-injected, deduplicated and sorted: the same
	// consultation stored in a different order settles the same rows.
	if err := RecordRetrieval(ctx, db.GetDB(), []string{" b ", "a", "a", "", "b"}, fixedNow); err != nil {
		t.Fatal(err)
	}
	var retrieved, usageRows int
	var lastRetrieved string
	rows, err := db.GetDB().Query(`SELECT concept, retrieved, last_retrieved_at FROM knowledge_usage ORDER BY concept`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var concept, lastAt string
		var n int
		if err := rows.Scan(&concept, &n, &lastAt); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		usageRows++
		retrieved += n
		lastRetrieved = lastAt
		if concept != "a" && concept != "b" {
			rows.Close()
			t.Fatalf("unexpected usage concept %q", concept)
		}
	}
	rows.Close()
	if rows.Err() != nil {
		t.Fatal(rows.Err())
	}
	if usageRows != 2 || retrieved != 2 || lastRetrieved != at {
		t.Fatalf("retrieval settled rows=%d total=%d at=%q, want 2 2 %q", usageRows, retrieved, lastRetrieved, at)
	}
	// RecordRecovery marks the current miss recovered once a compile delivers
	// rows for it; the recovered fact holds while no miss is newer.
	recoveredAt := fixedNow.Add(time.Hour).UTC().Format(time.RFC3339)
	if err := RecordRecovery(ctx, db.GetDB(), "zephyrquark xyzzy plover", fixedNow.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	facts, err = Measure(ctx, db.GetDB(), agent, fixedNow.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	hasRecovered := false
	for _, f := range facts {
		if f.Predicate == "knowledge_miss_recovered" && types.ExtractString(f.Args[1]) == "zephyrquark xyzzy plover" {
			hasRecovered = true
		}
	}
	if !hasRecovered {
		t.Fatalf("recovered miss missing after RecordRecovery (recovered_at %q)", recoveredAt)
	}
	if err := RecordMiss(ctx, db.GetDB(), "widget calibration reference table", fixedNow); err != nil {
		t.Fatal(err)
	}
	facts, err = Measure(ctx, db.GetDB(), agent, fixedNow)
	if err != nil {
		t.Fatal(err)
	}
	if !missQueries(t, facts)["widget calibration reference table"] {
		t.Fatal("a genuine miss after settling is not a current gap")
	}
	// Cancellation fails closed with an honest error on every writer and on
	// the settler.
	cancelled, stop := context.WithCancel(t.Context())
	stop()
	if err := RecordRetrieval(cancelled, db.GetDB(), []string{"a"}, fixedNow); err == nil {
		t.Fatal("cancelled RecordRetrieval did not fail")
	}
	if err := RecordMiss(cancelled, db.GetDB(), "q", fixedNow); err == nil {
		t.Fatal("cancelled RecordMiss did not fail")
	}
	if err := RecordRecovery(cancelled, db.GetDB(), "q", fixedNow); err == nil {
		t.Fatal("cancelled RecordRecovery did not fail")
	}
	if _, _, err := SettleLegacyMisses(cancelled, db.GetDB()); err == nil {
		t.Fatal("cancelled SettleLegacyMisses did not fail")
	}
}
