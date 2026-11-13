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

// A chain is also over budget once it has spent more model tokens than
// logging.self_repair_fix_budget_tokens, however little time that took; the
// kernel says which budget it ran past. With no measurement of the spend the
// token budget derives nothing.
func TestSelfRepair_AnOpenFixChainPastItsTokenBudgetIsOverBudget(t *testing.T) {
	start := time.Unix(1_800_000_000, 0)
	cfg := &config.LoggingConfig{SelfRepairFixBudgetSec: 600, SelfRepairFixBudgetTokens: 1000}
	ledger := emptyLedger(t)
	if err := ledger.OpenFixWindow("job", start); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name     string
		after    time.Duration
		measured bool
		spent    int64
		want     []string
	}{
		{"no measurement", time.Minute, false, 0, nil},
		{"inside both budgets", time.Minute, true, 1000, nil},
		{"past the token budget, early", time.Minute, true, 1001, []string{"tokens"}},
		{"past the time budget only", 11 * time.Minute, true, 10, []string{"time"}},
		{"past both", 11 * time.Minute, true, 5000, []string{"time", "tokens"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			k := selfRepairKernel(t, cfg, ledger, nil, start.Add(c.after))
			if c.measured {
				if err := problemlog.ReplaceFixSpent(k, "job", c.spent); err != nil {
					t.Fatal(err)
				}
			}
			kinds, err := problemlog.OverBudgetKinds(k)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(kinds["job"], c.want) {
				t.Fatalf("budgets run past = %v, want %v", kinds["job"], c.want)
			}
			jobs, err := problemlog.OverBudgetJobs(k)
			if err != nil {
				t.Fatal(err)
			}
			if (len(jobs) == 1) != (len(c.want) > 0) {
				t.Fatalf("over budget = %v with budgets run past %v", jobs, c.want)
			}
		})
	}
}
