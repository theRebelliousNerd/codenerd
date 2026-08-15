package context

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/store"
)

// A long-horizon compression gate.
//
// The backlog item this stands in for ("validate target compression ratio on
// real multi-hour sessions") wants campaign-assault artifacts that the repo
// does not carry — the recorded assault triage JSON holds no turn or window
// data. What IS reproducible here is the property those artifacts would be
// checked for: over hundreds of verbose turns with production-shaped defaults,
// the rolling summary must actually hold the configured ratio, and the window
// must stay bounded. A regression in either shows up here rather than in a
// four-hour session nobody re-runs.
func TestProcessTurn_OverLongSession_ShouldHoldTargetCompressionRatio(t *testing.T) {
	if testing.Short() {
		t.Skip("long-session compression gate")
	}

	kernel, err := core.NewRealKernelWithWorkspace(t.TempDir())
	if err != nil {
		t.Fatalf("kernel: %v", err)
	}
	localStore, err := store.NewLocalStore(":memory:")
	if err != nil {
		t.Fatalf("store: %v", err)
	}

	// Production-shaped config, scaled down so compression triggers within a
	// test-sized run: same reserve percentages, threshold, window and target.
	cfg := NewConfigWithBudget(20000)
	comp := NewCompressor(kernel, localStore, &MockLLMClient{})
	comp.config = cfg
	comp.budget = NewTokenBudget(cfg)
	comp.activation = NewActivationEngine(cfg)

	const turns = 300
	// A verbose turn: the surface text is exactly what compression exists to throw away.
	surface := strings.Repeat("the assistant explained the change in detail. ", 40)

	for i := 1; i <= turns; i++ {
		atoms := []core.Fact{
			{Predicate: "user_intent", Args: []any{fmt.Sprintf("i%d", i), "/code", "/fix", fmt.Sprintf("file%d.go", i%17), "none"}},
			{Predicate: "modified", Args: []any{fmt.Sprintf("file%d.go", i%17)}},
			{Predicate: "diagnostic", Args: []any{fmt.Sprintf("file%d.go", i%17), surface}},
		}
		if _, err := comp.ProcessTurn(context.Background(), Turn{
			Number:          i,
			Role:            "assistant",
			UserInput:       fmt.Sprintf("please fix file%d.go", i%17),
			SurfaceResponse: surface,
			ExtractedAtoms:  atoms,
			Timestamp:       time.Now(),
		}); err != nil {
			t.Fatalf("ProcessTurn %d: %v", i, err)
		}
	}

	metrics := comp.GetMetrics()
	segments := metrics["compressed_segments"].(int)
	if segments == 0 {
		t.Fatalf("no compression across %d verbose turns; the trigger is broken (metrics: %v)", turns, metrics)
	}

	// The window must stay bounded regardless of session length: that is the
	// entire "infinite context" claim.
	if got := len(comp.recentTurns); got > cfg.RecentTurnWindow*2 {
		t.Errorf("recent turn window unbounded: %d turns retained, max %d", got, cfg.RecentTurnWindow*2)
	}

	// The history block must stay inside its reserve; an unbounded rolling
	// summary pushes total usage past the window and BuildContext then refuses
	// to produce any context at all.
	if history := comp.counter.CountString(comp.rollingSummary.Text); history > cfg.HistoryReserve {
		t.Errorf("rolling summary %d tokens exceeds HistoryReserve %d after %d turns",
			history, cfg.HistoryReserve, turns)
	}
	if used := comp.budget.TotalUsed(); used > cfg.TotalBudget {
		t.Errorf("window usage %d exceeds total budget %d after %d turns; BuildContext would refuse",
			used, cfg.TotalBudget, turns)
	}
	if _, err := comp.BuildContext(context.Background()); err != nil {
		t.Errorf("BuildContext must still work after a long session: %v", err)
	}

	ratio := comp.rollingSummary.OverallRatio
	if ratio < cfg.TargetCompressionRatio {
		t.Errorf("rolling compression ratio %.1f:1 below target %.1f:1 after %d turns",
			ratio, cfg.TargetCompressionRatio, turns)
	}
	t.Logf("long session: %d turns, %d segments, rolling ratio %.1f:1, window %d turns, budget %d/%d",
		turns, segments, ratio, len(comp.recentTurns), comp.budget.TotalUsed(), cfg.TotalBudget)
}
