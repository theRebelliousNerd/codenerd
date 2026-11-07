package knowledge

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"codenerd/internal/types"
)

// versionedSource answers a different body each call, like a page that
// changed between two acquisitions.
type versionedSource struct {
	name    string
	version *int
}

func (v versionedSource) Name() string { return v.name }

func (v versionedSource) Acquire(ctx context.Context, s Subject) ([]Atom, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	*v.version++
	atoms := []Atom{{Concept: v.name + "/" + s.Key() + "/section_0", Content: fmt.Sprintf("Answer revision %d about %s", *v.version, s.Topic), Confidence: 0.8, Source: v.name}}
	if *v.version == 1 {
		atoms = append(atoms, Atom{Concept: v.name + "/" + s.Key() + "/section_1", Content: "A section only the first answer had about " + s.Topic, Confidence: 0.8, Source: v.name})
	}
	return atoms, nil
}

// Refresh asks again whatever the KB holds, and a source that acquires the
// subject again replaces every row its last acquisition wrote, the rewritten
// concept included; a source that fails leaves its rows in place.
func TestRefreshReplacesWhatTheLastAcquisitionWrote(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db := newKB(t)
	// Fixed clock: Fill/Refresh stamp outcomes deterministically; the fakes
	// below never touch the network.
	fixed := time.Date(2026, time.October, 29, 12, 0, 0, 0, time.UTC)
	version := 0
	acq := Acquirer{Sources: []Source{versionedSource{name: "web", version: &version}}, Now: func() time.Time { return fixed }}
	subjects := []Subject{{Agent: "goexpert", Topic: "go modules"}}
	if _, _, err := Fill(ctx, db, acq, subjects); err != nil {
		t.Fatal(err)
	}
	if _, again, err := Fill(ctx, db, acq, subjects); err != nil || len(again) != 0 || version != 1 {
		t.Fatalf("Fill asked an acquired source again: outcomes %v, calls %d, err %v", again, version, err)
	}
	if _, _, err := Refresh(ctx, db, acq, subjects); err != nil {
		t.Fatal(err)
	}
	atoms, err := db.GetAllKnowledgeAtoms()
	if err != nil {
		t.Fatal(err)
	}
	if len(atoms) != 1 || !strings.Contains(atoms[0].Content, "revision 2") {
		t.Fatalf("after the refresh the KB holds %+v, want only revision 2", atoms)
	}

	var calls int
	failing := Acquirer{
		Sources: []Source{countingSource{name: "web", fail: errors.New("HTTP 429"), calls: &calls}},
		Now:     func() time.Time { return fixed.Add(time.Hour) },
	}
	if _, _, err := Refresh(ctx, db, failing, subjects); err != nil {
		t.Fatal(err)
	}
	atoms, err = db.GetAllKnowledgeAtoms()
	if err != nil {
		t.Fatal(err)
	}
	if len(atoms) != 1 || !strings.Contains(atoms[0].Content, "revision 2") {
		t.Fatalf("a failed refresh removed knowledge: %+v", atoms)
	}
	recorded, err := Outcomes(ctx, db.GetDB())
	if err != nil {
		t.Fatal(err)
	}
	if len(recorded) != 1 || recorded[0].Status != StatusFailed || len(recorded[0].Concepts) != 1 {
		t.Fatalf("the failed attempt lost track of the rows it left: %+v", recorded)
	}
	// Pinned: fill honors cancellation instead of touching the KB.
	cancelled, stop := context.WithCancel(context.Background())
	stop()
	if _, _, err := Fill(cancelled, db, acq, subjects); err == nil {
		t.Fatal("Fill with a cancelled context succeeded, want cancellation")
	}
	if _, _, err := Refresh(cancelled, db, acq, subjects); err == nil {
		t.Fatal("Refresh with a cancelled context succeeded, want cancellation")
	}
	if _, _, err := fill(cancelled, db, acq, subjects, false); err == nil {
		t.Fatal("fill with a cancelled context succeeded, want cancellation")
	}
}

// Measure turns a KB into the facts the refresh policy reads: outcomes with
// their age, acquired rows by producer kind (placeholders excluded), what
// consultations retrieved, and what they missed.
func TestMeasureReadsOutcomesUseAndMisses(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db := newKB(t)
	// Fixed clock throughout: Fill stamps outcomes 40 days before now, and
	// retrieval/miss/measurement all read the same instant. The only source
	// is the in-process countingSource fake; no live network runs here.
	now := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	var calls int
	acq := Acquirer{Sources: []Source{countingSource{name: "web", calls: &calls}}, Now: func() time.Time { return now.AddDate(0, 0, -40) }}
	if _, _, err := Fill(ctx, db, acq, []Subject{{Topic: "go modules"}}); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("Fill calls = %d, want 1 from the fake source", calls)
	}
	if err := db.StoreKnowledgeAtom("agent_identity", "You are goexpert, a specialist agent in the codeNERD ecosystem.", 0.9, ""); err != nil {
		t.Fatal(err)
	}
	if err := RecordRetrieval(ctx, db.GetDB(), []string{"web/go modules", "web/go modules"}, now); err != nil {
		t.Fatal(err)
	}
	if err := RecordRetrieval(ctx, db.GetDB(), []string{"web/go modules"}, now); err != nil {
		t.Fatal(err)
	}
	if err := RecordMiss(ctx, db.GetDB(), "how do I vendor a replaced module", now); err != nil {
		t.Fatal(err)
	}
	facts, err := Measure(ctx, db.GetDB(), types.MangleAtom("/goexpert"), now)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string][]any{}
	for _, f := range facts {
		if prior, dup := got[f.Predicate]; dup {
			t.Fatalf("two %s facts: %v and %v", f.Predicate, prior, f.Args)
		}
		got[f.Predicate] = f.Args
	}
	if o := got["knowledge_outcome"]; len(o) != 5 || o[1] != "go modules" || o[2] != "web" || o[3] != types.MangleAtom("/acquired") || o[4] != int64(40) {
		t.Fatalf("knowledge_outcome = %v", o)
	}
	if h := got["knowledge_atom_held"]; len(h) != 4 || h[1] != "web/go modules" || h[2] != "web" {
		t.Fatalf("knowledge_atom_held = %v (placeholders must not count)", h)
	}
	if r := got["knowledge_atom_retrieved"]; len(r) != 3 || r[2] != int64(2) {
		t.Fatalf("knowledge_atom_retrieved = %v, want 2 consultations", r)
	}
	if m := got["knowledge_miss"]; len(m) != 3 || m[1] != "how do I vendor a replaced module" || m[2] != int64(1) {
		t.Fatalf("knowledge_miss = %v", m)
	}
	// Pinned: Measure is deterministic and honors cancellation; its table and
	// row helpers fail closed on bad input instead of returning partial facts.
	again, err := Measure(ctx, db.GetDB(), types.MangleAtom("/goexpert"), now)
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != len(facts) {
		t.Fatalf("Measure is nondeterministic: first %d facts, second %d", len(facts), len(again))
	}
	for i := range facts {
		if facts[i].Predicate != again[i].Predicate || len(facts[i].Args) != len(again[i].Args) {
			t.Fatalf("Measure order changed: %v vs %v", facts, again)
		}
	}
	cancelled, stop := context.WithCancel(context.Background())
	stop()
	if _, err := Measure(cancelled, db.GetDB(), types.MangleAtom("/goexpert"), now); err == nil {
		t.Fatal("Measure with a cancelled context succeeded, want cancellation")
	}
	if _, err := tableSet(cancelled, db.GetDB()); err == nil {
		t.Fatal("tableSet with a cancelled context succeeded, want cancellation")
	}
	if err := scanRows(nil, func(*sql.Rows) error { return nil }); err == nil {
		t.Fatal("scanRows with nil rows succeeded, want an error")
	}
	if tables, err := tableSet(ctx, db.GetDB()); err != nil || !tables["knowledge_acquisitions"] || !tables["knowledge_atoms"] {
		t.Fatalf("tableSet = %v, err %v, want the acquisition and atom tables", tables, err)
	}
	if empty, err := Measure(ctx, nil, types.MangleAtom("/goexpert"), now); err != nil || len(empty) != 0 {
		t.Fatalf("Measure(nil) = %v, err %v, want no facts and no error", empty, err)
	}
}

// Available names the sources that can run here.
func TestAcquirerAvailableNamesWhatCanRun(t *testing.T) {
	acq := NewAcquirer(Config{
		Searchers: []NamedSearcher{{Provider: "openai", Searcher: &fakeSearcher{supports: true}}},
		Workspace: "/ws",
		Run:       func(context.Context, string, string, ...string) ([]byte, error) { return nil, nil },
	})
	if got := strings.Join(acq.Available(), ","); got != "web,godoc" {
		t.Fatalf("Available() = %s, want web,godoc (no Context7 fetch, no world model)", got)
	}
}
