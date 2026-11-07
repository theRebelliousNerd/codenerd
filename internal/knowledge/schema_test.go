package knowledge

import (
	"context"
	"testing"
	"time"
)

// An outcomes table an earlier build created, without the later columns, is
// read as it is and gains them on the next fill.
func TestAnEarlierAcquisitionsTableIsReadAndUpgraded(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db := newKB(t)
	if _, err := db.GetDB().Exec(`CREATE TABLE knowledge_acquisitions (
		source TEXT NOT NULL, subject TEXT NOT NULL, status TEXT NOT NULL,
		atoms INTEGER NOT NULL, detail TEXT NOT NULL, attempted_at TEXT NOT NULL,
		PRIMARY KEY (source, subject))`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.GetDB().Exec(`INSERT INTO knowledge_acquisitions VALUES ('web', 'go', 'failed', 0, 'HTTP 503', '2026-10-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	got, err := Outcomes(ctx, db.GetDB())
	if err != nil || len(got) != 1 || got[0].Status != StatusFailed {
		t.Fatalf("Outcomes over the earlier table = %+v, %v", got, err)
	}
	if err := ensureAcquisitionsTable(ctx, db.GetDB()); err != nil {
		t.Fatalf("ensure over the earlier table: %v", err)
	}
	if err := ensureAcquisitionsTable(ctx, db.GetDB()); err != nil {
		t.Fatalf("repeated ensure must be idempotent: %v", err)
	}
	cols, err := tableColumnSet(ctx, db.GetDB(), "knowledge_acquisitions")
	if err != nil || !cols["concepts"] || !cols["version"] {
		t.Fatalf("upgraded columns = %+v, %v", cols, err)
	}
	got, err = Outcomes(ctx, db.GetDB())
	if err != nil || len(got) != 1 || got[0].Status != StatusFailed {
		t.Fatalf("Outcomes after idempotent upgrade = %+v, %v", got, err)
	}
	var calls int
	acq := Acquirer{Sources: []Source{countingSource{name: "web", calls: &calls}}}
	if _, _, err := Fill(ctx, db, acq, []Subject{{Topic: "go"}}); err != nil {
		t.Fatalf("Fill over the earlier table: %v", err)
	}
	got, err = Outcomes(ctx, db.GetDB())
	if err != nil || len(got) != 1 || got[0].Status != StatusAcquired || len(got[0].Concepts) != 1 {
		t.Fatalf("after the fill = %+v, %v", got, err)
	}
	if err := ensureAcquisitionsTable(ctx, db.GetDB()); err != nil {
		t.Fatalf("ensure after fill: %v", err)
	}
	if err := ensureAcquisitionsTable(ctx, db.GetDB()); err != nil {
		t.Fatalf("repeated ensure after fill must be idempotent: %v", err)
	}
	got, err = Outcomes(ctx, db.GetDB())
	if err != nil || len(got) != 1 || got[0].Status != StatusAcquired {
		t.Fatalf("Outcomes after repeated ensure = %+v, %v", got, err)
	}
}
