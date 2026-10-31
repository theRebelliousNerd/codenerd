package mangle

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

type readinessBarrierContext struct {
	context.Context
	cancel  context.CancelFunc
	checks  int
	ready   chan struct{}
	release chan struct{}
}

func (c *readinessBarrierContext) Err() error {
	c.checks++
	if c.checks == 3 {
		close(c.ready)
		<-c.release
		c.cancel()
	}
	return c.Context.Err()
}

func TestEngineReadiness_ConcurrentReadersObserveFailedWriter(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoEval = true
	e := readinessEngine(t, cfg)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	check := &readinessBarrierContext{Context: ctx, cancel: cancel, ready: make(chan struct{}), release: make(chan struct{})}
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(check.release) }) }
	writeDone := make(chan error, 1)
	var workers sync.WaitGroup
	batch := []Fact{{Predicate: "excluded", Args: []any{"work.go"}}}
	for i := range 256 {
		batch = append(batch, Fact{Predicate: "tag", Args: []any{fmt.Sprint(i)}})
	}
	workers.Go(func() { writeDone <- e.AddFactsContext(check, batch) })
	t.Cleanup(func() { release(); cancel(); workers.Wait() })
	select {
	case <-check.ready:
	case <-time.After(5 * time.Second):
		t.Fatal("writer never reached its accepted-prefix barrier")
	}
	type readResult struct {
		err   error
		empty bool
	}
	results := make(chan readResult, 3)
	started := make(chan struct{}, 3)
	for _, read := range []func() readResult{
		func() readResult { rows, err := e.GetFacts("eligible"); return readResult{err, len(rows) == 0} },
		func() readResult { rows, err := e.AtomStrings("eligible"); return readResult{err, len(rows) == 0} },
		func() readResult {
			rows, err := e.Query(t.Context(), "eligible(X)")
			return readResult{err, rows == nil}
		},
	} {
		workers.Go(func() { started <- struct{}{}; results <- read() })
	}
	for range 3 {
		<-started
	}
	release()
	if err := <-writeDone; !errors.Is(err, context.Canceled) {
		t.Fatalf("write did not retain a canceled prefix: %v", err)
	}
	for range 3 {
		result := <-results
		if !result.empty || !errors.Is(result.err, ErrDerivedStateNotReady) || !errors.Is(result.err, context.Canceled) {
			t.Errorf("reader bypassed readiness after writer release: %+v", result)
		}
	}
	if rows, err := e.Query(t.Context(), "excluded(X)"); err != nil || len(rows.Bindings) != 1 {
		t.Fatalf("accepted base prefix not queryable: %v / %v", rows, err)
	}
}

func TestEngineReadiness_ClearAndResetDiscardFailure(t *testing.T) {
	for _, reset := range []bool{false, true} {
		t.Run(fmt.Sprint(reset), func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.AutoEval = true
			e := readinessEngine(t, cfg)
			if err := e.AddFacts([]Fact{{Predicate: "excluded", Args: []any{"work.go"}}, {Predicate: "unknown"}}); err == nil {
				t.Fatal("failure fixture succeeded")
			}
			if reset {
				e.Reset()
				if err := e.LoadSchemaString(readinessProgram); err != nil {
					t.Fatal(err)
				}
			} else {
				e.Clear()
			}
			if rows, err := e.GetFacts("eligible"); err != nil || len(rows) != 0 {
				t.Fatalf("explicit reset did not discard failure state: %v / %v", rows, err)
			}
			if err := e.AddFact("pending", "new.go"); err != nil {
				t.Fatal(err)
			}
			if rows, err := e.GetFacts("eligible"); err != nil || len(rows) != 1 || rows[0].Args[0] != "new.go" {
				t.Fatalf("new state retained old conclusions: %v / %v", rows, err)
			}
		})
	}
}

func TestEngineReadiness_SuccessfulManualBatchRemainsDeferred(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoEval = true
	e := readinessEngine(t, cfg)
	e.ToggleAutoEval(false)
	if err := e.AddFact("excluded", "work.go"); err != nil {
		t.Fatal(err)
	}
	if rows, err := e.GetFacts("eligible"); err != nil || len(rows) != 1 {
		t.Fatalf("successful deferred mode changed before explicit evaluation: %v / %v", rows, err)
	}
	if err := e.RecomputeRules(); err != nil {
		t.Fatal(err)
	}
	if rows, err := e.GetFacts("eligible"); err != nil || len(rows) != 0 {
		t.Fatalf("explicit recomputation did not apply the accepted fact: %v / %v", rows, err)
	}
}

func TestEngineReadiness_RecoveryPreservesAssertedRuleHeadBase(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoEval, cfg.DerivedFactsLimit = true, 2
	e := readinessEngine(t, cfg)
	if err := e.LoadSchemaString("Decl fan_input(N) bound [/number].\nDecl fan(N) bound [/number].\nfan(N) :- fan_input(N).\n"); err != nil {
		t.Fatal(err)
	}
	if err := e.AddFact("eligible", "asserted"); err != nil {
		t.Fatal(err)
	}
	var batch []Fact
	for i := range int64(6) {
		batch = append(batch, Fact{Predicate: "fan_input", Args: []any{i}})
	}
	if err := e.AddFacts(batch); err == nil {
		t.Fatal("gas fixture succeeded")
	}
	if retry := e.Evaluate(); retry == nil {
		t.Fatal("failed recovery incorrectly succeeded under the unchanged gas limit")
	} else {
		assertDerivedUnavailable(t, e, "eligible", retry)
	}
	if err := e.ReplaceControlFacts(nil, "fan_input"); err != nil {
		t.Fatal(err)
	}
	rows, err := e.GetFacts("eligible")
	if err != nil || len(rows) != 2 {
		t.Fatalf("recovery lost an asserted rule-head base: %v / %v", rows, err)
	}
}

func TestEngineReadiness_SuccessfulAutoEvalRecovers(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoEval = true
	e := readinessEngine(t, cfg)
	if err := e.AddFacts([]Fact{{Predicate: "excluded", Args: []any{"work.go"}}, {Predicate: "unknown"}}); err == nil {
		t.Fatal("failure fixture succeeded")
	}
	if err := e.AddFact("tag", "recovery"); err != nil {
		t.Fatal(err)
	}
	if rows, err := e.GetFacts("eligible"); err != nil || len(rows) != 0 {
		t.Fatalf("successful auto-evaluation left stale/unready state: %v / %v", rows, err)
	}
}
