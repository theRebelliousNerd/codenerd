package context

import (
	"fmt"
	"testing"

	"codenerd/internal/core"
)

// countingReader answers the view questions and counts the row reads.
type countingReader struct {
	views []core.Fact
	rows  map[string][]core.Fact
	reads int
}

func (r *countingReader) Query(pred string) ([]core.Fact, error) {
	switch pred {
	case "context_view":
		return r.views, nil
	case "context_focal":
		return nil, nil
	}
	r.reads++
	return r.rows[pred], nil
}

// A wide neighbourhood selects hundreds of entities; the views are read only
// until the rows fill the atom budget, since selection could take none past it.
func TestViewFacts_StopsReadingAtTheBudget(t *testing.T) {
	comp := newKernelBackedCompressor(t)
	reader := &countingReader{
		views: []core.Fact{{Predicate: "context_view", Args: []any{core.MangleAtom("/ev_note"), int64(2), int64(0), int64(1), core.MangleAtom("/any")}}},
		rows:  map[string][]core.Fact{},
	}
	var entities []prioritized
	for i := range 400 {
		e := fmt.Sprintf("pkg/file%03d.go", i)
		entities = append(entities, prioritized{factStr: e, priority: 80})
		reader.rows["ev_note"] = append(reader.rows["ev_note"], fact("ev_note", e, "a note long enough to cost some tokens of the budget"))
	}
	comp.SetKernelReader(reader)

	comp.mu.Lock()
	defer comp.mu.Unlock()
	all := comp.viewFacts(entities, 1<<30)
	if len(all) != len(entities) || reader.reads != len(entities) {
		t.Fatalf("unbounded: %d facts from %d reads, want one of each for %d entities", len(all), reader.reads, len(entities))
	}
	reader.reads = 0
	some := comp.viewFacts(entities, 200)
	if len(some) == 0 || len(some) >= len(entities) {
		t.Fatalf("a 200-token budget returned %d of %d facts", len(some), len(entities))
	}
	if reader.reads != len(some) {
		t.Errorf("%d reads for %d facts: reading went on past the budget", reader.reads, len(some))
	}
	for i, sf := range some {
		if sf.Fact.Args[0] != entities[i].factStr {
			t.Fatalf("fact %d is of %v, want the entities in priority order (%s)", i, sf.Fact.Args[0], entities[i].factStr)
		}
	}
}

// A malformed view row and a view of a predicate the kernel cannot answer are
// skipped; the other views still apply.
func TestViewFacts_SkipsWhatItCannotRead(t *testing.T) {
	comp := newKernelBackedCompressor(t)
	atom := func(s string) core.MangleAtom { return core.MangleAtom(s) }
	reader := &countingReader{
		views: []core.Fact{
			{Predicate: "context_view", Args: []any{atom("/ev_note"), int64(2), int64(5), int64(1), atom("/any")}}, // position outside the arity
			{Predicate: "context_view", Args: []any{atom("/ev_note"), int64(2), int64(0), int64(2), atom("/any")}},
			{Predicate: "context_view", Args: []any{atom("/ev_focal_only"), int64(1), int64(0), int64(1), atom("/focal")}},
		},
		rows: map[string][]core.Fact{
			"ev_note":       {fact("ev_note", "a.go", "kept"), fact("ev_note", "b.go", "of another entity")},
			"ev_focal_only": {fact("ev_focal_only", "a.go")},
		},
	}
	comp.SetKernelReader(reader)
	comp.mu.Lock()
	defer comp.mu.Unlock()
	got := comp.viewFacts([]prioritized{{factStr: "a.go", priority: 90}}, 1<<30)
	if len(got) != 1 || got[0].Fact.Predicate != "ev_note" || got[0].Fact.Args[1] != "kept" || got[0].Score != 90 {
		t.Fatalf("got %v, want only a.go's ev_note row at its entity's priority (a.go is not focal)", got)
	}
}
