package core

import (
	"slices"
	"testing"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/problemlog"
)

// An unattended fix chain is over budget once its window has been open longer
// than logging.self_repair_fix_budget_sec; a chain inside its budget, and one
// whose window has closed, are not.
func TestSelfRepair_AnOpenFixChainPastItsBudgetIsOverBudget(t *testing.T) {
	start := time.Unix(1_800_000_000, 0)
	cfg := &config.LoggingConfig{SelfRepairFixBudgetSec: 600}

	open := emptyLedger(t)
	if err := open.OpenFixWindow("job-open", start); err != nil {
		t.Fatal(err)
	}
	closed := emptyLedger(t)
	if err := closed.OpenFixWindow("job-closed", start); err != nil {
		t.Fatal(err)
	}
	if err := closed.CloseFixWindow("job-closed", start.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}

	for _, c := range []struct {
		name   string
		ledger *problemlog.RepairLedger
		after  time.Duration
		want   []string
	}{
		{"inside the budget", open, 9 * time.Minute, nil},
		{"exactly at the budget", open, 10 * time.Minute, nil},
		{"past the budget", open, 10*time.Minute + time.Second, []string{"job-open"}},
		{"a closed window, long after", closed, time.Hour, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			k := selfRepairKernel(t, cfg, c.ledger, nil, start.Add(c.after))
			got, err := problemlog.OverBudgetJobs(k)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(got, c.want) {
				t.Fatalf("over budget = %v, want %v", got, c.want)
			}
		})
	}
}

// The default budget applies when the config sets none.
func TestSelfRepair_TheDefaultFixBudgetApplies(t *testing.T) {
	start := time.Unix(1_800_000_000, 0)
	ledger := emptyLedger(t)
	if err := ledger.OpenFixWindow("job", start); err != nil {
		t.Fatal(err)
	}
	budget := (*config.LoggingConfig)(nil).SelfRepairFixBudget()
	k := selfRepairKernel(t, &config.LoggingConfig{}, ledger, nil, start.Add(budget+time.Second))
	got, err := problemlog.OverBudgetJobs(k)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []string{"job"}) {
		t.Fatalf("over budget = %v, want the open job past the default %s", got, budget)
	}
}
