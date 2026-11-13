package session

import (
	"strings"
	"testing"

	"codenerd/internal/types"
)

// The pinning gate's repair episode has its own round cap and a share of the
// turn's time (repair_episode.mg): past either, the policy gives up and the
// reason names the bound, where before only the generic attempt cap (and the
// turn's deadline) stopped it.
func TestPinningEpisodeStopsAtItsCapAndItsShare(t *testing.T) {
	newExec := func() *Executor {
		e := NewExecutor(realKernel(t), nil, nil, nil, nil, nil)
		e.config.PinRepairMaxAttempts = 2
		e.config.PinGateSharePercent = 25
		e.config.RepairMaxAttempts = 5
		return e
	}
	t.Run("round cap", func(t *testing.T) {
		e := newExec()
		ep := newRepairEpisodeAtom()
		e.assertTurnFact(types.Fact{Predicate: "repair_episode_kind", Args: []any{ep, types.MangleAtom("/pinning")}})
		if next := e.nextRepairMove(ep, 1, true, "unpinned: A"); next.gaveUp != "" {
			t.Fatalf("first failed attempt gave up: %q", next.gaveUp)
		}
		next := e.nextRepairMove(ep, 2, true, "unpinned: B")
		if !strings.Contains(next.gaveUp, "pin_repair_max_attempts=2") {
			t.Fatalf("second failed attempt: gaveUp=%q, want the pinning round cap", next.gaveUp)
		}
	})
	t.Run("time share", func(t *testing.T) {
		e := newExec()
		ep := newRepairEpisodeAtom()
		e.assertTurnFact(types.Fact{Predicate: "repair_episode_kind", Args: []any{ep, types.MangleAtom("/pinning")}})
		// 300s spent of the 1000s the turn had left: over the 25% share.
		e.assertTurnFact(types.Fact{Predicate: "repair_episode_elapsed_ms", Args: []any{ep, int64(300_000)}})
		e.assertTurnFact(types.Fact{Predicate: "repair_episode_budget_ms", Args: []any{ep, int64(1_000_000)}})
		next := e.nextRepairMove(ep, 1, true, "unpinned: A")
		if !strings.Contains(next.gaveUp, "pin_gate_share_percent=25") {
			t.Fatalf("gaveUp=%q, want the pinning time share", next.gaveUp)
		}
	})
	t.Run("other gates keep the generic cap", func(t *testing.T) {
		e := newExec()
		ep := newRepairEpisodeAtom()
		e.assertTurnFact(types.Fact{Predicate: "repair_episode_kind", Args: []any{ep, types.MangleAtom("/tests")}})
		e.nextRepairMove(ep, 1, true, "fail A")
		if next := e.nextRepairMove(ep, 2, true, "fail B"); next.gaveUp != "" {
			t.Fatalf("a tests episode stopped at the pinning cap: %q", next.gaveUp)
		}
	})
}
