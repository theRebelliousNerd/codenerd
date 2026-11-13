package context

import (
	"testing"
	"time"
)

// The chat's footer reads the budget on every Update and View. BuildContext
// holds the compressor's lock for its whole build (2.6s live on 2026-11-12,
// when the chat's memUsageMsg handler waited 1.98s behind it): the budget
// reads must not wait for it.
func TestCompressor_BudgetReadsDoNotWaitForABuild(t *testing.T) {
	t.Parallel()
	comp := newKernelBackedCompressor(t)
	comp.budget.SetUsage(10, 20, 30, 40, 0)

	comp.mu.Lock() // a BuildContext in progress
	defer comp.mu.Unlock()
	type usage struct {
		used, total int
		util        float64
	}
	got := make(chan usage, 1)
	go func() {
		used, total := comp.GetBudgetUsage()
		got <- usage{used, total, comp.GetBudgetUtilization()}
	}()
	select {
	case u := <-got:
		if u.used != 100 || u.total != comp.config.TotalBudget || u.util <= 0 {
			t.Fatalf("budget read %+v, want 100 used of %d", u, comp.config.TotalBudget)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the budget read waited for the compressor's build lock")
	}
}
