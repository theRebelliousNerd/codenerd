package session

import (
	"testing"
	"time"
)

// The repair episode's wall clock is the model's budget; the time the harness
// spends re-running the failed gates after each attempt is added from what
// those gates were measured to take. Observed 2026-09-18: a repair in a package
// whose recheck costs about a minute ran out of a flat five minutes after two
// of its three attempts.
func TestRepairClock_AddsTheMeasuredGateTimePerAttempt(t *testing.T) {
	budget := RepairBudget{MaxAttempts: 3, WallClock: 5 * time.Minute}

	slow := &ExecutionResult{
		BuildCheck: BuildVerification{Duration: 20 * time.Second},
		TestCheck:  TestVerification{Duration: 40 * time.Second},
	}
	if got, want := repairClockWithGateTime(budget, slow), 8*time.Minute; got != want {
		t.Errorf("clock = %s, want %s (5m for the model + 3 rechecks of a 1m gate)", got, want)
	}

	if got := repairClockWithGateTime(budget, &ExecutionResult{}); got != budget.WallClock {
		t.Errorf("clock = %s with no measured gate time, want the budget's own %s", got, budget.WallClock)
	}
	if got := repairClockWithGateTime(budget, nil); got != budget.WallClock {
		t.Errorf("clock = %s with no result, want %s", got, budget.WallClock)
	}
}
