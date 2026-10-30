package context

import (
	stdctx "context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
)

func saveN(t *testing.T, s *WorkingStore, prefix string, from, to int) {
	t.Helper()
	for i := from; i < to; i++ {
		if err := s.Save(stdctx.Background(), WorkingRecord{ID: fmt.Sprintf("%s%d", prefix, i), Entity: "e", Revision: "v", Kind: "k", Step: int64(i), Body: "b"}); err != nil {
			t.Fatal(err)
		}
	}
}

func handleOf(t *testing.T, s *WorkingStore, id string) string {
	t.Helper()
	h, err := s.Handle(stdctx.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

// Allocating a new record's short handle read the scope's highest numeric
// ordinal through an index that orders handles as text, so it visited every
// handle in the scope: on the linked SQLite it was ~96% of a Save and grew
// linearly with the archive (audit working-handle-allocation). The
// allocation now seeks an index on the numeric expression.
func TestWorkingHandleAllocation_SeeksTheOrdinalIndex(t *testing.T) {
	s, err := OpenWorkingStore(t.TempDir(), "scope-plan")
	if err != nil {
		t.Fatal(err)
	}
	defer s.db.Close()
	rows, err := s.db.Query(`EXPLAIN QUERY PLAN `+nextWorkingHandle, s.scope)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var plan []string
	for rows.Next() {
		var id, parent, unused int
		var detail string
		if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
			t.Fatal(err)
		}
		plan = append(plan, detail)
	}
	if got := strings.Join(plan, "; "); !strings.Contains(got, "working_handle_ordinal") {
		t.Fatalf("the allocation does not read the ordinal index: %s", got)
	}
}

// Handles are numeric ordinals across digit boundaries, a duplicate save
// keeps its handle, and two stores on one scope never commit a duplicate.
func TestWorkingHandleAllocation_Contract(t *testing.T) {
	ws := t.TempDir()
	s, err := OpenWorkingStore(ws, "scope-contract")
	if err != nil {
		t.Fatal(err)
	}
	defer s.db.Close()
	saveN(t, s, "r", 0, 101)
	for i, want := range map[int]string{8: "9", 9: "10", 98: "99", 99: "100", 100: "101"} {
		if got := handleOf(t, s, fmt.Sprintf("r%d", i)); got != want {
			t.Fatalf("record %d has handle %q, want %q", i, got, want)
		}
	}
	saveN(t, s, "r", 50, 51) // a duplicate
	if got := handleOf(t, s, "r50"); got != "51" {
		t.Fatalf("a duplicate save moved the handle to %q", got)
	}
	other, err := OpenWorkingStore(ws, "scope-contract")
	if err != nil {
		t.Fatal(err)
	}
	defer other.db.Close()
	saveN(t, other, "o", 0, 1)
	saveN(t, s, "s", 0, 1)
	if a, b := handleOf(t, s, "o0"), handleOf(t, s, "s0"); a != "102" || b != "103" {
		t.Fatalf("two stores allocated %q and %q, want 102 and 103", a, b)
	}
}

// An archive written before the ordinal index keeps every published handle
// when it is reopened, gains the index, and continues the sequence.
func TestWorkingHandleAllocation_ALegacyArchiveKeepsItsHandles(t *testing.T) {
	ws := t.TempDir()
	s, err := OpenWorkingStore(ws, "scope-legacy")
	if err != nil {
		t.Fatal(err)
	}
	saveN(t, s, "r", 0, 12)
	if _, err := s.db.Exec(`DROP INDEX working_handle_ordinal`); err != nil {
		t.Fatal(err)
	}
	// A row from before handles existed.
	if _, err := s.db.Exec(`INSERT INTO working_records (id,scope,entity,revision,kind,step,body,failed,handle) VALUES ('old','scope-legacy','e','v','k',99,'b',0,'')`); err != nil {
		t.Fatal(err)
	}
	_ = s.db.Close()

	re, err := OpenWorkingStore(ws, "scope-legacy")
	if err != nil {
		t.Fatal(err)
	}
	defer re.db.Close()
	if got := handleOf(t, re, "r10"); got != "11" {
		t.Fatalf("a published handle changed on reopen: %q", got)
	}
	if got := handleOf(t, re, "old"); got != "13" {
		t.Fatalf("the legacy row was backfilled as %q, want 13", got)
	}
	var idx string
	if err := re.db.QueryRow(`SELECT name FROM sqlite_master WHERE type='index' AND name='working_handle_ordinal'`).Scan(&idx); err == sql.ErrNoRows {
		t.Fatal("the reopened archive has no ordinal index")
	}
	saveN(t, re, "n", 0, 1)
	if got := handleOf(t, re, "n0"); got != "14" {
		t.Fatalf("the next handle after reopen is %q, want 14", got)
	}
}
