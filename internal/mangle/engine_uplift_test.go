package mangle

// Behavioral uplift tests for internal/mangle/engine.go.
//
// Each test pins an observable store behavior, not an implementation detail:
// derivation visibility after control-fact replacement, auto-eval restoration
// after a failed warm start, honest stats timestamps, safe concurrent reads
// across Clear/Reset, and read-path agreement.

import (
	"context"
	"sync"
	"testing"
	"time"
)

const upliftSchema = `
Decl ctl(X).
Decl out(X).
out(X) :- ctl(X).
`

func newUpliftEngine(t *testing.T, persistence Persistence) *Engine {
	t.Helper()
	cfg := DefaultConfig()
	cfg.AutoEval = true
	engine, err := NewEngine(cfg, persistence)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
	if err := engine.LoadSchemaString(upliftSchema); err != nil {
		t.Fatalf("LoadSchemaString() error = %v", err)
	}
	return engine
}

func outArgs(t *testing.T, engine *Engine) []any {
	t.Helper()
	facts, err := engine.GetFacts("out")
	if err != nil {
		t.Fatalf("GetFacts(out) error = %v", err)
	}
	var args []any
	for _, f := range facts {
		if len(f.Args) != 1 {
			t.Fatalf("out fact has %d args, want 1: %v", len(f.Args), f)
		}
		args = append(args, f.Args[0])
	}
	return args
}

func containsArg(args []any, want string) bool {
	for _, a := range args {
		if s, ok := a.(string); ok && (s == want || s == "/"+want) {
			return true
		}
	}
	return false
}

// ReplaceControlFacts promises to re-derive from scratch. With auto-eval off
// it used to wipe every IDB predicate and return without evaluating, leaving
// derived state silently empty.
func TestUpliftReplaceControlFactsDerivesWithAutoEvalOff(t *testing.T) {
	engine := newUpliftEngine(t, nil)
	engine.ToggleAutoEval(false)

	if err := engine.ReplaceControlFacts(
		[]Fact{{Predicate: "ctl", Args: []any{"/a"}}}, "ctl"); err != nil {
		t.Fatalf("ReplaceControlFacts() error = %v", err)
	}
	if got := outArgs(t, engine); !containsArg(got, "a") {
		t.Fatalf("out after replace with auto-eval off = %v, want [/a]", got)
	}

	// Stale derivations from the previous generation must not survive.
	if err := engine.ReplaceControlFacts(
		[]Fact{{Predicate: "ctl", Args: []any{"/b"}}}, "ctl"); err != nil {
		t.Fatalf("ReplaceControlFacts() error = %v", err)
	}
	if got := outArgs(t, engine); !containsArg(got, "b") || containsArg(got, "a") {
		t.Fatalf("out after second replace = %v, want [/b] only", got)
	}
}

// A failed warm start must not leave auto-eval stuck off: the engine the
// caller believes is live would silently stop deriving on every later insert.
func TestUpliftFailedWarmRestoresAutoEval(t *testing.T) {
	bad := &mockPersistence{facts: []Fact{{Predicate: "undeclared_pred", Args: []any{"/x"}}}}
	engine := newUpliftEngine(t, bad)

	if err := engine.WarmFromPersistence(context.Background()); err == nil {
		t.Fatal("WarmFromPersistence(bad facts) = nil, want error")
	}
	// Behavior proves the flag: an insert must derive without any manual
	// RecomputeRules call.
	if err := engine.AddFact("ctl", "/live"); err != nil {
		t.Fatalf("AddFact() error = %v", err)
	}
	if got := outArgs(t, engine); !containsArg(got, "live") {
		t.Fatalf("out after failed warm + insert = %v, want [/live]; auto-eval was not restored", got)
	}
}

// The same restoration must hold when auto-eval was deliberately off: a failed
// warm must not flip it on either.
func TestUpliftFailedWarmPreservesDisabledAutoEval(t *testing.T) {
	bad := &mockPersistence{facts: []Fact{{Predicate: "undeclared_pred", Args: []any{"/x"}}}}
	engine := newUpliftEngine(t, bad)
	engine.ToggleAutoEval(false)

	if err := engine.WarmFromPersistence(context.Background()); err == nil {
		t.Fatal("WarmFromPersistence(bad facts) = nil, want error")
	}
	if err := engine.AddFact("ctl", "/quiet"); err != nil {
		t.Fatalf("AddFact() error = %v", err)
	}
	if got := outArgs(t, engine); len(got) != 0 {
		t.Fatalf("out with auto-eval off = %v, want empty; failed warm flipped auto-eval on", got)
	}
}

// Stats.LastUpdate used to be time.Now() at call time — a lie. It must be
// zero before any mutation and track the real last store change.
func TestUpliftStatsLastUpdateTracksMutations(t *testing.T) {
	engine := newUpliftEngine(t, nil)
	if got := engine.GetStats().LastUpdate; !got.IsZero() {
		t.Fatalf("LastUpdate before any mutation = %v, want zero", got)
	}

	before := time.Now()
	if err := engine.AddFact("ctl", "/t"); err != nil {
		t.Fatalf("AddFact() error = %v", err)
	}
	stamp := engine.GetStats().LastUpdate
	if stamp.IsZero() || stamp.Before(before) || stamp.After(time.Now()) {
		t.Fatalf("LastUpdate after insert = %v, want within [%v, now]", stamp, before)
	}

	engine.Clear()
	if got := engine.GetStats().LastUpdate; got.Before(stamp) {
		t.Fatalf("LastUpdate after Clear = %v, went backwards from %v", got, stamp)
	}
}

// GetFacts and GetFactsSeq used to read the store field after releasing the
// lock while Clear/Reset swap it — a data race. Hammer reads against swaps;
// run the package with -race to enforce. Every read must either succeed or
// fail cleanly, never panic or race.
func TestUpliftConcurrentReadsAcrossClearAndReset(t *testing.T) {
	engine := newUpliftEngine(t, nil)
	if err := engine.AddFact("ctl", "/r"); err != nil {
		t.Fatalf("AddFact() error = %v", err)
	}

	var wg sync.WaitGroup
	stop := make(chan struct{})
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				switch w % 4 {
				case 0:
					_, _ = engine.GetFacts("ctl")
				case 1:
					for range engine.GetFactsSeq("ctl") {
					}
				case 2:
					_ = engine.QueryFacts("ctl")
				case 3:
					_ = engine.GetStats()
				}
			}
		}(w)
	}
	for i := 0; i < 25; i++ {
		engine.Clear()
		_ = engine.AddFact("ctl", "/r")
	}
	close(stop)
	wg.Wait()

	// Reset path swaps the store AND drops the schema; reads during it must
	// also be race-clean (GetFacts may legitimately error once the schema
	// is gone — the requirement is no race and no panic).
	stop = make(chan struct{})
	for w := 0; w < 2; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				_, _ = engine.GetFacts("ctl")
			}
		}()
	}
	engine.Reset()
	close(stop)
	wg.Wait()
}

// QueryFacts on an undeclared predicate is defined to yield nil (browser
// compat), and must not panic; the miss is only debug-logged.
func TestUpliftQueryFactsUnknownPredicateYieldsNil(t *testing.T) {
	engine := newUpliftEngine(t, nil)
	if got := engine.QueryFacts("no_such_predicate"); got != nil {
		t.Fatalf("QueryFacts(unknown) = %v, want nil", got)
	}
	if err := engine.AddFact("ctl", "/f"); err != nil {
		t.Fatalf("AddFact() error = %v", err)
	}
	if got := engine.QueryFacts("ctl", "f"); len(got) != 1 {
		t.Fatalf("QueryFacts(ctl, f) yielded %d facts, want 1", len(got))
	}
}

// GetFactsSeq must agree with GetFacts on content, and neither fabricates
// timestamps: the EDB stores none.
func TestUpliftFactsSeqAgreesWithGetFacts(t *testing.T) {
	engine := newUpliftEngine(t, nil)
	if err := engine.AddFact("ctl", "/s1"); err != nil {
		t.Fatalf("AddFact() error = %v", err)
	}
	if err := engine.AddFact("ctl", "/s2"); err != nil {
		t.Fatalf("AddFact() error = %v", err)
	}

	direct, err := engine.GetFacts("ctl")
	if err != nil {
		t.Fatalf("GetFacts() error = %v", err)
	}
	var seq []Fact
	for f := range engine.GetFactsSeq("ctl") {
		seq = append(seq, f)
	}
	if len(seq) != len(direct) {
		t.Fatalf("GetFactsSeq yielded %d facts, GetFacts returned %d", len(seq), len(direct))
	}
	for _, f := range seq {
		if !f.Timestamp.IsZero() {
			t.Fatalf("GetFactsSeq fabricated timestamp %v; EDB stores none", f.Timestamp)
		}
	}
	for _, f := range direct {
		if !f.Timestamp.IsZero() {
			t.Fatalf("GetFacts fabricated timestamp %v; EDB stores none", f.Timestamp)
		}
	}
}
