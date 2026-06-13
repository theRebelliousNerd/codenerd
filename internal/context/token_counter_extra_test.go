package context

import (
	"strings"
	"testing"

	"codenerd/internal/core"
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

// TestEstimateCompressionRatio covers the zero-facts identity, the
// incompressible case (compressed >= original), and a genuine reduction.
func TestEstimateCompressionRatio(t *testing.T) {
	if r := EstimateCompressionRatio(1000, 0); r != 1.0 {
		t.Errorf("ratio with 0 facts=%v, want 1.0", r)
	}
	// 5 facts -> ~50 compressed tokens, but original is only 20: not compressible.
	if r := EstimateCompressionRatio(20, 5); r != 1.0 {
		t.Errorf("ratio when compressed>=original=%v, want 1.0", r)
	}
	// 1000 original / (10 facts * 10) = 10x reduction.
	if r := EstimateCompressionRatio(1000, 10); r != 10.0 {
		t.Errorf("ratio=%v, want 10.0", r)
	}
}

// TestTruncateFact verifies long arguments are clipped to the display width and
// the predicate/args are rendered in Datalog form.
func TestTruncateFact(t *testing.T) {
	fs := NewFactSerializer()
	long := strings.Repeat("x", 100)
	out := fs.truncateFact(core.Fact{Predicate: "note", Args: []any{long}})
	if !strings.HasPrefix(out, "note(") || !strings.HasSuffix(out, ").") {
		t.Errorf("truncateFact rendering malformed: %q", out)
	}
	if !strings.Contains(out, "...") {
		t.Errorf("truncateFact should clip a 100-char arg with an ellipsis: %q", out)
	}
	// A short arg is left intact (no ellipsis).
	short := fs.truncateFact(core.Fact{Predicate: "p", Args: []any{"ok"}})
	if strings.Contains(short, "...") {
		t.Errorf("short arg should not be clipped: %q", short)
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
