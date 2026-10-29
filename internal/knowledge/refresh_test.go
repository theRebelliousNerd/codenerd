package knowledge

import (
	"context"
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

func (v versionedSource) Acquire(_ context.Context, s Subject) ([]Atom, error) {
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
	ctx := context.Background()
	db := newKB(t)
	version := 0
	acq := Acquirer{Sources: []Source{versionedSource{name: "web", version: &version}}}
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
	atoms, _ := db.GetAllKnowledgeAtoms()
	if len(atoms) != 1 || !strings.Contains(atoms[0].Content, "revision 2") {
		t.Fatalf("after the refresh the KB holds %+v, want only revision 2", atoms)
	}

	var calls int
	failing := Acquirer{Sources: []Source{countingSource{name: "web", fail: errors.New("HTTP 429"), calls: &calls}}}
	if _, _, err := Refresh(ctx, db, failing, subjects); err != nil {
		t.Fatal(err)
	}
	atoms, _ = db.GetAllKnowledgeAtoms()
	if len(atoms) != 1 || !strings.Contains(atoms[0].Content, "revision 2") {
		t.Fatalf("a failed refresh removed knowledge: %+v", atoms)
	}
	recorded, _ := Outcomes(ctx, db.GetDB())
	if len(recorded) != 1 || recorded[0].Status != StatusFailed || len(recorded[0].Concepts) != 1 {
		t.Fatalf("the failed attempt lost track of the rows it left: %+v", recorded)
	}
}

// Measure turns a KB into the facts the refresh policy reads: outcomes with
// their age, acquired rows by producer kind (placeholders excluded), what
// consultations retrieved, and what they missed.
func TestMeasureReadsOutcomesUseAndMisses(t *testing.T) {
	ctx := context.Background()
	db := newKB(t)
	now := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	var calls int
	acq := Acquirer{Sources: []Source{countingSource{name: "web", calls: &calls}}, Now: func() time.Time { return now.AddDate(0, 0, -40) }}
	if _, _, err := Fill(ctx, db, acq, []Subject{{Topic: "go modules"}}); err != nil {
		t.Fatal(err)
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
