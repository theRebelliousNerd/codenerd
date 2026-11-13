package chat

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/problemlog"
	"codenerd/internal/session"
)

func overBudgetFixture(t *testing.T) (Model, string, *problemlog.RepairLedger) {
	t.Helper()
	m, _ := journeyModel(t, config.WorkModeYolo)
	m.isBooting = false
	m.workspace = t.TempDir()
	m.selfRepair = newSelfRepairState()
	now := time.Unix(1_800_000_000, 0)
	m.selfRepair.now = func() time.Time { return now }
	ledger, err := m.selfRepair.ledgerFor(m.workspace)
	if err != nil {
		t.Fatal(err)
	}
	const job = "self-repair-budget"
	if err := ledger.Begin([]string{"timing:kernel.Query"}, problemlog.LaneOptimize, job, nil, now); err != nil {
		t.Fatal(err)
	}
	if err := ledger.Fixing(job, now); err != nil {
		t.Fatal(err)
	}
	if err := ledger.OpenFixWindow(job, now); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(m.workspace, "kernel.go")
	if err := os.WriteFile(path, []byte("the fix chain's edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	journal := session.NewWriteJournal()
	journal.Record(m.workspace, []string{"kernel.go"}, map[string]session.PreImage{
		"kernel.go": {Existed: true, Content: "before\n", LastWriteHash: hashOf("the fix chain's edit\n")},
	})
	m.selfRepair.mu.Lock()
	m.selfRepair.journal, m.selfRepair.journalJob = journal, job
	m.selfRepair.mu.Unlock()
	return m, path, ledger
}

// A fix chain the kernel names as over budget is stopped, its writes undone
// and its attempt recorded over budget -- a failure the retry is told about,
// not a stop by the user.
func TestSelfRepair_AFixChainOverBudgetIsUndoneAndRecordedSo(t *testing.T) {
	m, path, ledger := overBudgetFixture(t)

	m, _ = m.handleFixOverBudget(selfRepairBudgetMsg{jobs: []string{"self-repair-budget"}})

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "before\n" {
		t.Fatalf("the over-budget chain's edit stayed in the tree: %q", got)
	}
	a, ok := ledger.Get("timing:kernel.Query")
	if !ok || a.Status != problemlog.RepairOverBudget {
		t.Fatalf("attempt = %+v, want over_budget", a)
	}
	if a.Failures != 1 {
		t.Fatalf("over budget must count as a failure, failures = %d", a.Failures)
	}
	if a.AttemptDiff == "" {
		t.Fatal("the undone attempt was not saved for the retry")
	}
	if m.selfRepair.openFixJob() != "" {
		t.Fatal("the chain is still open")
	}
}

// A verdict about another job, or one that arrives after the chain settled,
// stops nothing.
func TestSelfRepair_ABudgetVerdictForAnotherJobStopsNothing(t *testing.T) {
	m, path, ledger := overBudgetFixture(t)

	m, _ = m.handleFixOverBudget(selfRepairBudgetMsg{jobs: []string{"some-other-job"}})

	if got, _ := os.ReadFile(path); string(got) != "the fix chain's edit\n" {
		t.Fatalf("a verdict on another job undid this chain: %q", got)
	}
	if a, _ := ledger.Get("timing:kernel.Query"); a.Status != problemlog.RepairBeingFixed {
		t.Fatalf("attempt = %+v, want still being fixed", a)
	}
	if m.selfRepair.openFixJob() == "" {
		t.Fatal("the chain was closed")
	}
}
