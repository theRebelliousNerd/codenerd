package world

import (
	"testing"

	"codeberg.org/TauCeti/mangle-go/analysis"
	"codenerd/internal/core"
	"codenerd/internal/types"
)

type mockKernel struct {
	removeFactsCalled   bool
	removedPredicates   map[string]struct{}
	loadFactsCalled     bool
	loadedFacts         []types.Fact
	retractExactCalled  bool
	retractedExactFacts []types.Fact
	retractCalled       bool
	retractedPredicates []string
}

func (m *mockKernel) LoadFacts(facts []types.Fact) error {
	m.loadFactsCalled = true
	m.loadedFacts = append(m.loadedFacts, facts...)
	return nil
}
func (m *mockKernel) Query(predicate string) ([]types.Fact, error) { return nil, nil }
func (m *mockKernel) QueryAll() (map[string][]types.Fact, error)   { return nil, nil }
func (m *mockKernel) Assert(fact types.Fact) error                 { return nil }
func (m *mockKernel) AssertBatch(facts []types.Fact) error         { return nil }
func (m *mockKernel) Retract(predicate string) error {
	m.retractCalled = true
	m.retractedPredicates = append(m.retractedPredicates, predicate)
	return nil
}
func (m *mockKernel) RetractFact(fact types.Fact) error     { return nil }
func (m *mockKernel) UpdateSystemFacts() error              { return nil }
func (m *mockKernel) GetProgramInfo() *analysis.ProgramInfo { return nil }
func (m *mockKernel) Reset()                                {}
func (m *mockKernel) AppendPolicy(policy string)            {}
func (m *mockKernel) RetractExactFactsBatch(facts []types.Fact) error {
	m.retractExactCalled = true
	m.retractedExactFacts = append(m.retractedExactFacts, facts...)
	return nil
}
func (m *mockKernel) RemoveFactsByPredicateSet(predicates map[string]struct{}) error {
	m.removeFactsCalled = true
	m.removedPredicates = predicates
	return nil
}

func TestApplyIncrementalResult(t *testing.T) {
	t.Run("nil result", func(t *testing.T) {
		k := &mockKernel{}
		err := ApplyIncrementalResult(k, nil)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if k.removeFactsCalled || k.retractExactCalled || k.loadFactsCalled {
			t.Errorf("expected no kernel methods to be called")
		}
	})

	t.Run("full res nil new facts", func(t *testing.T) {
		k := &mockKernel{}
		res := &IncrementalResult{
			Full:     true,
			NewFacts: nil,
		}
		err := ApplyIncrementalResult(k, res)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if !k.removeFactsCalled {
			t.Errorf("expected removeFactsCalled to be called")
		}
		// verify WorldPredicateSet was passed
		if len(k.removedPredicates) == 0 {
			t.Errorf("expected removedPredicates to be set")
		}
		if k.loadFactsCalled {
			t.Errorf("expected loadFactsCalled not to be called")
		}
	})

	t.Run("full res with new facts", func(t *testing.T) {
		k := &mockKernel{}
		res := &IncrementalResult{
			Full: true,
			NewFacts: []core.Fact{
				{Predicate: "test", Args: []any{"a"}},
			},
		}
		err := ApplyIncrementalResult(k, res)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if !k.removeFactsCalled {
			t.Errorf("expected removeFactsCalled to be called")
		}
		if !k.loadFactsCalled {
			t.Errorf("expected loadFactsCalled to be called")
		}
		if len(k.loadedFacts) != 1 || k.loadedFacts[0].Predicate != "test" {
			t.Errorf("expected loadedFacts to be correct")
		}
	})

	t.Run("delta res without retract facts", func(t *testing.T) {
		k := &mockKernel{}
		res := &IncrementalResult{
			Full:         false,
			RetractFacts: nil,
			NewFacts: []core.Fact{
				{Predicate: "test2", Args: []any{"a"}},
			},
		}
		err := ApplyIncrementalResult(k, res)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if k.removeFactsCalled {
			t.Errorf("expected removeFactsCalled not to be called")
		}
		if k.retractExactCalled {
			t.Errorf("expected retractExactCalled not to be called")
		}
		if !k.retractCalled || len(k.retractedPredicates) == 0 || k.retractedPredicates[0] != "directory" {
			t.Errorf("expected retractCalled to be called on directory")
		}
		if !k.loadFactsCalled {
			t.Errorf("expected loadFactsCalled to be called")
		}
		if len(k.loadedFacts) != 1 || k.loadedFacts[0].Predicate != "test2" {
			t.Errorf("expected loadedFacts to be correct")
		}
	})

	t.Run("delta res with retract facts", func(t *testing.T) {
		k := &mockKernel{}
		res := &IncrementalResult{
			Full: false,
			RetractFacts: []core.Fact{
				{Predicate: "old", Args: []any{"a"}},
			},
			NewFacts: []core.Fact{
				{Predicate: "test3", Args: []any{"a"}},
			},
		}
		err := ApplyIncrementalResult(k, res)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if k.removeFactsCalled {
			t.Errorf("expected removeFactsCalled not to be called")
		}
		if !k.retractExactCalled {
			t.Errorf("expected retractExactCalled to be called")
		}
		if len(k.retractedExactFacts) != 1 || k.retractedExactFacts[0].Predicate != "old" {
			t.Errorf("expected retractedExactFacts to be correct")
		}
		if !k.retractCalled || len(k.retractedPredicates) == 0 || k.retractedPredicates[0] != "directory" {
			t.Errorf("expected retractCalled to be called on directory")
		}
		if !k.loadFactsCalled {
			t.Errorf("expected loadFactsCalled to be called")
		}
		if len(k.loadedFacts) != 1 || k.loadedFacts[0].Predicate != "test3" {
			t.Errorf("expected loadedFacts to be correct")
		}
	})

	t.Run("delta res nil new facts", func(t *testing.T) {
		k := &mockKernel{}
		res := &IncrementalResult{
			Full:     false,
			NewFacts: nil,
		}
		err := ApplyIncrementalResult(k, res)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if k.removeFactsCalled {
			t.Errorf("expected removeFactsCalled not to be called")
		}
		if !k.retractCalled || len(k.retractedPredicates) == 0 || k.retractedPredicates[0] != "directory" {
			t.Errorf("expected retractCalled to be called on directory")
		}
		if k.loadFactsCalled {
			t.Errorf("expected loadFactsCalled not to be called")
		}
	})
}
