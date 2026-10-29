package context

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

// RefreshBudget used to drop c.mu before calling recalcBudget, on the claim
// that holding it risked deadlock. recalcBudget takes no compressor lock
// (ProcessTurn calls it while holding c.mu), so the only thing that comment
// bought was an unsynchronized read of recentTurns/rollingSummary and an
// unsynchronized write of budget.used against a live turn. Run the whole
// rehydrate/observe/process surface concurrently under -race.
func TestCompressor_WhenBudgetRefreshedDuringTurns_ShouldNotRace(t *testing.T) {
	comp := newKernelBackedCompressor(t)
	input := strings.Repeat("word ", 50)

	var wg sync.WaitGroup
	const iterations = 25

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 1; i <= iterations; i++ {
			_, _ = comp.ProcessTurn(context.Background(), Turn{
				Number:    i,
				Role:      "user",
				UserInput: input,
				Timestamp: time.Now(),
			})
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for range iterations {
			comp.RefreshBudget()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for range iterations {
			_ = comp.GetState()
			_ = comp.GetMetrics()
			_ = comp.GetSelectionStats()
			_, _ = comp.GetBudgetUsage()
			_ = comp.IsCompressionActive()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for range iterations {
			_, _ = comp.BuildContext(context.Background())
		}
	}()

	wg.Wait()

	if _, total := comp.GetBudgetUsage(); total != comp.config.TotalBudget {
		t.Errorf("budget total drifted: %d != %d", total, comp.config.TotalBudget)
	}
}

func TestCompressor_WhenStateReloadedDuringTurns_ShouldNotRace(t *testing.T) {
	comp := newKernelBackedCompressor(t)
	if _, err := comp.ProcessTurn(context.Background(), Turn{Number: 1, Role: "user", UserInput: "seed"}); err != nil {
		t.Fatalf("seed turn: %v", err)
	}
	snapshot := comp.GetState()

	var wg sync.WaitGroup
	const iterations = 20

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 2; i <= iterations; i++ {
			_, _ = comp.ProcessTurn(context.Background(), Turn{Number: i, Role: "user", UserInput: "turn"})
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for range iterations {
			// LoadState now recalculates the budget internally, so this
			// exercises the restore path and the budget path together.
			_ = comp.LoadState(snapshot)
		}
	}()

	wg.Wait()
}
