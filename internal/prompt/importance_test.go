package prompt

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestImportanceFromScore_NearestWrittenLevel(t *testing.T) {
	for score, want := range map[float64]ImportanceLevel{
		0: ImportanceStyle, 0.49: ImportanceStyle, 0.5: ImportanceConvention,
		2.11: ImportanceEfficiency, 2.6: ImportanceCorrectness, 4: ImportanceCritical,
	} {
		got, err := ImportanceFromScore(score)
		if err != nil || got != want {
			t.Errorf("ImportanceFromScore(%v) = %v, %v; want %v", score, got, err, want)
		}
	}
	for _, bad := range []float64{-0.1, 4.01} {
		if _, err := ImportanceFromScore(bad); err == nil {
			t.Errorf("ImportanceFromScore(%v) accepted a position off the scale", bad)
		}
	}
}

func TestImportanceLevel_ZeroIsUnscoredAndEveryLevelIsWritten(t *testing.T) {
	var zero ImportanceLevel
	if zero.Scored() || zero != ImportanceUnscored {
		t.Fatal("the zero value must be unscored: an atom built in code has no importance")
	}
	if n := len(ImportanceMeanings()); n != int(ImportanceCritical) {
		t.Fatalf("%d written meanings for %d levels", n, ImportanceCritical)
	}
	for l := ImportanceStyle; l <= ImportanceCritical; l++ {
		if l.Meaning() == "" || l.String() == "unscored" {
			t.Fatalf("level %d has no written meaning", l)
		}
	}
}

func TestImportanceLedger_LookupNeedsTheJudgedContent(t *testing.T) {
	ledger, err := ParseImportanceLedger([]byte(`{"scale":"s","model":"jev","atoms":{"a":{"content_hash":"h1","level":4,"score":3.1,"probabilities":[0,0,0.1,0.7,0.2]}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if l, ok := ledger.Lookup("a", "h1"); !ok || l != ImportanceCorrectness {
		t.Fatalf("Lookup(a, h1) = %v, %v", l, ok)
	}
	if _, ok := ledger.Lookup("a", "edited"); ok {
		t.Fatal("a score for other content was applied to an edited atom")
	}
	atoms := []*PromptAtom{{ID: "a", ContentHash: "h1"}, {ID: "b", ContentHash: "x"}}
	if unscored := applyImportance(atoms, ledger); len(unscored) != 1 || unscored[0] != "b" {
		t.Fatalf("unscored = %v, want [b]", unscored)
	}
	if atoms[0].Importance != ImportanceCorrectness || atoms[1].Importance != ImportanceUnscored {
		t.Fatalf("applied levels = %v, %v", atoms[0].Importance, atoms[1].Importance)
	}
	for _, bad := range []string{
		`{"atoms":{"a":{"content_hash":"h","level":9,"probabilities":[0,0,0,0,1]}}}`,
		`{"atoms":{"a":{"content_hash":"","level":2,"probabilities":[0,0,0,0,1]}}}`,
		`{"atoms":{"a":{"content_hash":"h","level":2,"probabilities":[1]}}}`,
	} {
		if _, err := ParseImportanceLedger([]byte(bad)); err == nil {
			t.Errorf("accepted a bad ledger: %s", bad)
		}
	}
}

// TestEmbeddedImportanceLedgerParses: the shipped ledger is well formed. Its
// coverage of the corpus is atomjudge's TestEmbeddedLedgerCoversEveryAtom,
// which knows the question's fingerprint.
func TestEmbeddedImportanceLedgerParses(t *testing.T) {
	if _, err := EmbeddedImportanceLedger(); err != nil {
		t.Fatal(err)
	}
}

// TestImportanceColumn_MigratesPriorityAway: a corpus written with the old
// priority column gains importance and loses priority, and a stored level
// round-trips while NULL reads as unscored.
func TestImportanceColumn_MigratesPriorityAway(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "c.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE prompt_atoms (id INTEGER PRIMARY KEY, atom_id TEXT NOT NULL UNIQUE, priority INTEGER DEFAULT 50)`); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for range 2 { // idempotent
		if err := migrateImportanceColumn(ctx, db); err != nil {
			t.Fatal(err)
		}
	}
	var priorityCols int
	if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('prompt_atoms') WHERE name = 'priority'`).Scan(&priorityCols); err != nil || priorityCols != 0 {
		t.Fatalf("priority column survived the migration (%d, %v)", priorityCols, err)
	}
	if _, err := db.Exec(`INSERT INTO prompt_atoms (atom_id, importance) VALUES ('a', ?), ('b', ?)`, importanceColumn(ImportanceCritical), importanceColumn(ImportanceUnscored)); err != nil {
		t.Fatal(err)
	}
	for id, want := range map[string]ImportanceLevel{"a": ImportanceCritical, "b": ImportanceUnscored} {
		var v sql.NullInt64
		if err := db.QueryRow(`SELECT importance FROM prompt_atoms WHERE atom_id = ?`, id).Scan(&v); err != nil {
			t.Fatal(err)
		}
		if got := importanceFromColumn(v); got != want {
			t.Fatalf("%s read back %v, want %v", id, got, want)
		}
	}
}

func TestImportanceLedger_MarshalRoundTripsOneRecordPerLine(t *testing.T) {
	in := &ImportanceLedger{Scale: "s", Model: "m", Atoms: map[string]ImportanceRecord{
		"b": {ContentHash: "hb", Level: ImportanceStyle, Score: 0.1, Probabilities: []float64{0.9, 0.1, 0, 0, 0}},
		"a": {ContentHash: "ha", Level: ImportanceCritical, Score: 4, Probabilities: []float64{0, 0, 0, 0, 1}},
	}}
	out, err := in.MarshalIndent()
	if err != nil {
		t.Fatal(err)
	}
	if lines := strings.Count(string(out), "\n"); lines != 4 {
		t.Fatalf("%d lines, want a header, one per atom and a close:\n%s", lines, out)
	}
	if strings.Index(string(out), `"a"`) > strings.Index(string(out), `"b"`) {
		t.Fatalf("records not in ID order:\n%s", out)
	}
	back, err := ParseImportanceLedger(out)
	if err != nil || back.Scale != "s" || back.Model != "m" || back.Atoms["a"].Level != ImportanceCritical || len(back.Atoms) != 2 {
		t.Fatalf("round trip = %+v, %v", back, err)
	}
}
