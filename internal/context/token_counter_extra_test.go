package context

import (
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	"codenerd/internal/broker"
	"codenerd/internal/core"
	"codenerd/internal/types"
)

// TestCountFactsAndScoredFacts verifies the aggregate token counters sum their
// per-fact estimates and that an empty slice costs nothing.
func TestCountFactsAndScoredFacts(t *testing.T) {
	tc := NewTokenCounter()
	facts := []core.Fact{
		{Predicate: "user_intent", Args: []any{"build a feature"}},
		{Predicate: "focus_resolution", Args: []any{"main.go", 42}},
	}

	want := tc.CountFact(facts[0]) + tc.CountFact(facts[1])
	if got := tc.CountFacts(facts); got != want {
		t.Errorf("CountFacts=%d, want sum of per-fact counts %d", got, want)
	}
	if got := tc.CountFacts(nil); got != 0 {
		t.Errorf("CountFacts(nil)=%d, want 0", got)
	}

	scored := []ScoredFact{{Fact: facts[0], Score: 9}, {Fact: facts[1], Score: 3}}
	if got := tc.CountScoredFacts(scored); got != want {
		t.Errorf("CountScoredFacts=%d, want %d (scores must not affect token count)", got, want)
	}
}

// TestCountCompressedContext sums the string and turn components, treating a
// nil context as zero cost.
func TestCountCompressedContext(t *testing.T) {
	tc := NewTokenCounter()
	if got := tc.CountCompressedContext(nil); got != 0 {
		t.Errorf("CountCompressedContext(nil)=%d, want 0", got)
	}

	ctx := &CompressedContext{
		ContextAtoms:   "context_atom data here",
		CoreFacts:      "core constitutional facts",
		HistorySummary: "a brief summary of prior turns",
		RecentTurns: []CompressedTurn{
			{TurnNumber: 1, Role: "user", IntentAtom: &core.Fact{Predicate: "user_intent", Args: []any{"hi"}}},
		},
	}
	want := tc.CountString(ctx.ContextAtoms) + tc.CountString(ctx.CoreFacts) +
		tc.CountString(ctx.HistorySummary) + tc.CountTurns(ctx.RecentTurns)
	if got := tc.CountCompressedContext(ctx); got != want {
		t.Errorf("CountCompressedContext=%d, want %d", got, want)
	}
}

// TestTruncateFact verifies long arguments are clipped to the display width and
// the predicate/args are rendered in Datalog form.
func TestTruncateFact(t *testing.T) {
	fs := NewFactSerializer()
	long := strings.Repeat("x", 1000)
	out := fs.truncateFact(core.Fact{Predicate: "note", Args: []any{long}})
	if !strings.HasPrefix(out, "note(") || !strings.HasSuffix(out, ").") {
		t.Errorf("truncateFact rendering malformed: %q", out)
	}
	if !types.IsClamped(out) {
		t.Errorf("truncateFact should clip a 1000-char arg and say so: %q", out)
	}

	// A short arg is left intact.
	short := fs.truncateFact(core.Fact{Predicate: "p", Args: []any{"ok"}})
	if types.IsClamped(short) {
		t.Errorf("short arg should not be clipped: %q", short)
	}

	// So is an argument the cut would not actually shorten. The marker costs
	// more characters than a 100-char argument has to give: dropping 55 of
	// them to append 56 leaves the block larger and the fact less complete,
	// which is worse on both counts than carrying the argument whole.
	modest := strings.Repeat("x", 100)
	kept := fs.truncateFact(core.Fact{Predicate: "note", Args: []any{modest}})
	if !strings.Contains(kept, modest) {
		t.Errorf("a 100-char arg should be carried whole rather than traded for a longer marker: %q", kept)
	}
}

// TestNewConfigWithBudget covers the budget-derived reserve math and the
// non-positive fallback to the 200k default.
func TestNewConfigWithBudget(t *testing.T) {
	cfg := NewConfigWithBudget(100000)
	if cfg.TotalBudget != 100000 {
		t.Errorf("TotalBudget=%d, want 100000", cfg.TotalBudget)
	}
	if cfg.CoreReserve != 5000 || cfg.AtomReserve != 30000 ||
		cfg.HistoryReserve != 15000 || cfg.WorkingReserve != 50000 {
		t.Errorf("reserves miscomputed: core=%d atom=%d hist=%d work=%d",
			cfg.CoreReserve, cfg.AtomReserve, cfg.HistoryReserve, cfg.WorkingReserve)
	}

	def := NewConfigWithBudget(0)
	if def.TotalBudget != 200000 {
		t.Errorf("non-positive budget should fall back to 200000, got %d", def.TotalBudget)
	}
}

// The counter no longer owns a ratio. These tests pin the property that
// replaced the old TokenEstimator seam: counting is delegated to the process
// broker, so it improves as the broker observes real provider responses.
//
// Each test uses a model name unique to itself. The broker's calibrator is
// process-wide by design (one ledger, one ratio), so tests that trained a
// shared key would leak into each other.

func TestTokenCounter_ShouldCountViaBrokerRatio(t *testing.T) {
	counter, model := testCounterForModel("test/context-counter-ratio")

	const text = "the quick brown fox jumps over the lazy dog"
	before := counter.CountString(text)
	if before <= 0 {
		t.Fatalf("CountString returned %d for non-empty text", before)
	}
	if counter.Confidence() != broker.ConfidenceSeeded {
		t.Errorf("a model with no observations should report seeded, got %q", counter.Confidence())
	}

	// Teach the broker that this model packs twice as many characters per
	// token as the seed assumes. The counter must follow without being
	// reconstructed: the compressor holds one counter for a whole session.
	seedRatio := counter.Ratio()
	broker.Default().Calibrator().Observe(broker.Observation{
		Model:             model,
		Chars:             20000,
		ActualInputTokens: int(20000 / (seedRatio * 2)),
	})

	after := counter.CountString(text)
	if after >= before {
		t.Errorf("counter ignored the observed ratio: %d before, %d after (ratio now %.2f)",
			before, after, counter.Ratio())
	}
	if counter.Confidence() != broker.ConfidenceCalibrated {
		t.Errorf("after an observation the counter should report calibrated, got %q", counter.Confidence())
	}
}

func TestTokenCounter_WhenEmpty_ShouldChargeNothing(t *testing.T) {
	counter, _ := testCounterForModel("test/context-counter-empty")
	if got := counter.CountString(""); got != 0 {
		t.Errorf("CountString(\"\") = %d, want 0 — empty input must not be billed", got)
	}
}

func TestTokenCounter_ShouldNeverReturnZeroForNonEmptyText(t *testing.T) {
	// A single character must cost at least one token. Returning zero would let
	// an unbounded number of tiny facts into a budget that believed it was full.
	counter, _ := testCounterForModel("test/context-counter-floor")
	if got := counter.CountString("x"); got < 1 {
		t.Errorf("CountString(\"x\") = %d, want >= 1", got)
	}
}

// testCounterForModel builds a counter bound to an isolated model name so one
// test's calibration cannot move another's learned ratio.
//
// The name is suffixed per call because the calibrator lives on the process
// meter and outlives a single test: a fixed name means the second run of the
// suite in one process finds the ratio the first run taught it, and a test
// asserting "seeded before any observation" fails. That is the same run-once
// defect as the tool-registry one, in a different shared singleton.
//
// It is a test helper rather than a production constructor because nothing in
// the codebase sizes content for a model other than the primary one. Exporting
// a constructor that only tests call is how a package accumulates surface
// nobody maintains, and the repo's dead-code budget is there to catch exactly
// that.
func testCounterForModel(model string) (*TokenCounter, string) {
	unique := fmt.Sprintf("%s#%d", model, testCounterSeq.Add(1))
	return &TokenCounter{estimator: broker.Default().TextCounter(unique)}, unique
}

var testCounterSeq atomic.Int64
