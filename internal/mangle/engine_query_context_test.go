package mangle

import (
	"context"
	"errors"
	"sync"
	"testing"

	"codeberg.org/TauCeti/mangle-go/ast"
)

// queryScanStore blocks the first row from one selected predicate so a test
// can cancel Query while a direct or nested store scan is in progress.
type queryScanStore struct {
	atomsByPredicate map[string][]ast.Atom
	blockedPredicate string
	entered          chan struct{}
	release          chan struct{}
	finished         chan struct{}
	once             sync.Once
	mu               sync.Mutex
	visited          int
}

func (s *queryScanStore) GetFacts(query ast.Atom, yield func(ast.Atom) error) error {
	predicate := query.Predicate.Symbol
	isBlockedScan := predicate == s.blockedPredicate
	if isBlockedScan {
		defer close(s.finished)
	}
	for i, atom := range s.atomsByPredicate[predicate] {
		if isBlockedScan {
			s.mu.Lock()
			s.visited++
			s.mu.Unlock()
			if i == 0 {
				s.once.Do(func() { close(s.entered) })
				<-s.release
			}
		}
		if err := yield(atom); err != nil {
			return err
		}
	}
	return nil
}

func (s *queryScanStore) Contains(ast.Atom) bool { return false }

func (s *queryScanStore) ListPredicates() []ast.PredicateSym { return nil }

func (s *queryScanStore) EstimateFactCount() int {
	total := 0
	for _, atoms := range s.atomsByPredicate {
		total += len(atoms)
	}
	return total
}

func (s *queryScanStore) visitedRows() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.visited
}

func queryScanAtoms(t *testing.T, predicate, value string, n int) []ast.Atom {
	t.Helper()
	atom, err := ParseAtom(predicate + `("` + value + `")`)
	if err != nil {
		t.Fatal(err)
	}
	atoms := make([]ast.Atom, n)
	for i := range atoms {
		atoms[i] = atom
	}
	return atoms
}

func runCanceledQueryScan(t *testing.T, eng *Engine, store *queryScanStore, query string) {
	t.Helper()
	eng.mu.Lock()
	eng.queryContext.Store = store
	eng.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	queryDone := make(chan error, 1)
	go func() {
		_, err := eng.Query(ctx, query)
		queryDone <- err
	}()

	<-store.entered
	cancel()
	close(store.release)
	if err := <-queryDone; !errors.Is(err, context.Canceled) {
		t.Fatalf("Query error = %v, want context.Canceled", err)
	}
	<-store.finished
	if visited := store.visitedRows(); visited >= 1000 {
		t.Fatalf("canceled query scanned all 1000 rows; want the underlying scan to stop (visited %d)", visited)
	}
}

func TestEngineQuery_CancellationStopsFilteredStoreScan(t *testing.T) {
	eng, err := NewEngine(DefaultConfig(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := eng.LoadSchemaString(`Decl item(Value) bound [/string].`); err != nil {
		t.Fatal(err)
	}
	store := &queryScanStore{
		atomsByPredicate: map[string][]ast.Atom{"item": queryScanAtoms(t, "item", "present", 1000)},
		blockedPredicate: "item",
		entered:          make(chan struct{}),
		release:          make(chan struct{}),
		finished:         make(chan struct{}),
	}
	runCanceledQueryScan(t, eng, store, `item("absent")`)
}

func TestEngineQuery_CancellationStopsNestedPremiseScan(t *testing.T) {
	eng, err := NewEngine(DefaultConfig(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := eng.LoadSchemaString(`
Decl source(Value) bound [/string].
Decl derived(Value) bound [/string].
derived(Value) :- source(Value).
`); err != nil {
		t.Fatal(err)
	}
	store := &queryScanStore{
		atomsByPredicate: map[string][]ast.Atom{"source": queryScanAtoms(t, "source", "present", 1000)},
		blockedPredicate: "source",
		entered:          make(chan struct{}),
		release:          make(chan struct{}),
		finished:         make(chan struct{}),
	}
	runCanceledQueryScan(t, eng, store, `derived("absent")`)
}
