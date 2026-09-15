package mangle

// Regression tests for step-2 hardening: every error-path fix must fail
// closed with an honest error instead of panicking or returning nil.
//
// These tests pin the behavior observed in engine.go:
//   - operations before LoadSchema return errNoSchemas
//   - bad schemas are dropped, not poisoned
//   - cancelled contexts return context errors honestly
//   - empty batches are a no-op
//   - persistence warm-up handles nil / failing stores without panic

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestHardenStep2E_OperationsWithoutSchemaFailClosed(t *testing.T) {
	cases := map[string]func(e *Engine) error{
		"AddFacts": func(e *Engine) error {
			return e.AddFacts([]Fact{{Predicate: "edge", Args: []any{"/a"}}})
		},
		"AddFact": func(e *Engine) error {
			return e.AddFact("edge", "/a")
		},
		"Evaluate": func(e *Engine) error {
			return e.Evaluate()
		},
		"RecomputeRules": func(e *Engine) error {
			return e.RecomputeRules()
		},
		"ReplaceFactsForFile": func(e *Engine) error {
			return e.ReplaceFactsForFile("some/file.go", []Fact{{Predicate: "edge", Args: []any{"/a"}}})
		},
		"ReplaceFactsForFileWithHash": func(e *Engine) error {
			return e.ReplaceFactsForFileWithHash("some/file.go", []Fact{{Predicate: "edge", Args: []any{"/a"}}}, "abc")
		},
	}

	for name, call := range cases {
		t.Run(name, func(t *testing.T) {
			e, err := NewEngine(DefaultConfig(), nil)
			if err != nil {
				t.Fatalf("NewEngine failed: %v", err)
			}
			err = call(e)
			if err == nil {
				t.Fatalf("%s without schema returned nil, want fail-closed error", name)
			}
			if !errors.Is(err, errNoSchemas) && !strings.Contains(strings.ToLower(err.Error()), "no schemas") {
				t.Fatalf("%s without schema returned dishonest error %q, want errNoSchemas", name, err.Error())
			}
		})
	}
}

func TestHardenStep2E_BadSchemaDoesNotPoisonEngine(t *testing.T) {
	e, err := NewEngine(DefaultConfig(), nil)
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	bad := "this is not valid mangle ::: {{{"
	if err := e.LoadSchemaString(bad); err == nil {
		t.Fatalf("LoadSchemaString(bad) returned nil, want parse/analyze error")
	}

	// Engine must still fail closed with errNoSchemas, not panic or succeed.
	if err := e.AddFacts([]Fact{{Predicate: "edge", Args: []any{"/a"}}}); !errors.Is(err, errNoSchemas) {
		t.Fatalf("AddFacts after bad schema = %v, want errNoSchemas (fragment must be dropped)", err)
	}
	if err := e.Evaluate(); !errors.Is(err, errNoSchemas) {
		t.Fatalf("Evaluate after bad schema = %v, want errNoSchemas", err)
	}

	// A second bad load must also fail honestly, not panic on slice bookkeeping.
	if err := e.LoadSchemaString(bad); err == nil {
		t.Fatalf("second LoadSchemaString(bad) returned nil, want error")
	}
	if err := e.AddFacts([]Fact{{Predicate: "edge", Args: []any{"/a"}}}); !errors.Is(err, errNoSchemas) {
		t.Fatalf("AddFacts after second bad schema = %v, want errNoSchemas", err)
	}
}

func TestHardenStep2E_LoadSchemaMissingFileHonestError(t *testing.T) {
	e, err := NewEngine(DefaultConfig(), nil)
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}
	err = e.LoadSchema("/nonexistent/path/does/not/exist.mg")
	if err == nil {
		t.Fatalf("LoadSchema(missing) returned nil, want honest error")
	}
	if !strings.Contains(err.Error(), "does/not/exist") && !strings.Contains(strings.ToLower(err.Error()), "failed to read") {
		t.Fatalf("LoadSchema(missing) error %q does not mention file", err.Error())
	}
}

func TestHardenStep2E_CancelledContextHonestError(t *testing.T) {
	e, err := NewEngine(DefaultConfig(), nil)
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = e.AddFactsContext(ctx, []Fact{{Predicate: "edge", Args: []any{"/a"}}})
	if err == nil {
		t.Fatalf("AddFactsContext(cancelled) returned nil, want context error")
	}
	if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("AddFactsContext(cancelled) = %v, want context.Canceled", err)
	}
}

func TestHardenStep2E_EmptyBatchIsNoOp(t *testing.T) {
	e, err := NewEngine(DefaultConfig(), nil)
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}
	// Empty batch must not require schemas and must not fail.
	if err := e.AddFacts(nil); err != nil {
		t.Fatalf("AddFacts(nil) = %v, want nil (no-op)", err)
	}
	if err := e.AddFacts([]Fact{}); err != nil {
		t.Fatalf("AddFacts(empty) = %v, want nil (no-op)", err)
	}
	if err := e.AddFactsContext(context.Background(), nil); err != nil {
		t.Fatalf("AddFactsContext(nil) = %v, want nil (no-op)", err)
	}
}

type step2EFailingPersistence struct {
	loadErr error
	facts   []Fact
}

func (p *step2EFailingPersistence) ReplaceFactsForFile(_ context.Context, _ string, _ []Fact, _ string) error {
	return errors.New("persist boom")
}

func (p *step2EFailingPersistence) LoadFacts(_ context.Context) ([]Fact, error) {
	if p.loadErr != nil {
		return nil, p.loadErr
	}
	return p.facts, nil
}

func (p *step2EFailingPersistence) GetFileStates(_ context.Context) (map[string]string, error) {
	return map[string]string{}, nil
}

func TestHardenStep2E_WarmNilPersistenceIsNoOp(t *testing.T) {
	e, err := NewEngine(DefaultConfig(), nil)
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}
	if err := e.WarmFromPersistence(context.Background()); err != nil {
		t.Fatalf("WarmFromPersistence(nil store) = %v, want nil", err)
	}
}

func TestHardenStep2E_WarmPropagatesLoadError(t *testing.T) {
	boom := errors.New("disk unreadable")
	e, err := NewEngine(DefaultConfig(), &step2EFailingPersistence{loadErr: boom})
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}
	err = e.WarmFromPersistence(context.Background())
	if err == nil {
		t.Fatalf("WarmFromPersistence(failing store) returned nil, want honest error")
	}
	if !errors.Is(err, boom) && !strings.Contains(err.Error(), "disk unreadable") {
		t.Fatalf("WarmFromPersistence error %q does not wrap store error", err.Error())
	}
}

func TestHardenStep2E_WarmFactsWithoutSchemaFailsClosed(t *testing.T) {
	e, err := NewEngine(DefaultConfig(), &step2EFailingPersistence{
		facts: []Fact{{Predicate: "edge", Args: []any{"/a"}}},
	})
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}
	err = e.WarmFromPersistence(context.Background())
	if err == nil {
		t.Fatalf("WarmFromPersistence(facts, no schema) returned nil, want errNoSchemas")
	}
	if !errors.Is(err, errNoSchemas) {
		t.Fatalf("WarmFromPersistence(facts, no schema) = %v, want errNoSchemas", err)
	}
}

func TestHardenStep2E_StatsClearResetCloseDoNotPanic(t *testing.T) {
	e, err := NewEngine(DefaultConfig(), nil)
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}
	// Any panic here fails the test automatically; these must be safe on an
	// empty engine without schemas.
	_ = e.GetStats()
	e.Clear()
	e.Reset()
	_ = e.Close()
	_ = e.GetDerivedFactCount()
	e.ResetDerivedFactCount()
}
