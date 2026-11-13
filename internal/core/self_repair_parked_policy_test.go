package core

import (
	"testing"
	"time"

	"codenerd/internal/problemlog"
)

// A parked candidate is finished before any new target is chosen: while one
// is parked and the slot is open, it is due (self_repair_finish_due) and no
// hot problem is; once it is resumed or dropped, the hot problems are due
// again.
func TestSelfRepair_AParkedCandidateIsFinishedBeforeANewTarget(t *testing.T) {
	ledger := emptyLedger(t)
	start := repairNow.Add(-2 * time.Hour)
	if err := ledger.Begin([]string{"earlier-target"}, problemlog.LaneRepair, "job-parked", nil, start); err != nil {
		t.Fatal(err)
	}
	if err := ledger.Fixing("job-parked", start); err != nil {
		t.Fatal(err)
	}
	if n, err := ledger.Park("job-parked", ".nerd/self_repair/parked/job-parked.json", "parked: package tests green, self-rebuild unfinished", start); err != nil || n != 1 {
		t.Fatalf("Park = %d, %v", n, err)
	}
	k := selfRepairKernel(t, nil, ledger, nil, repairNow)
	if len(hotSignatures(t, k)) == 0 {
		t.Fatal("the fixture has no hot problem")
	}
	finish, err := problemlog.FinishDue(k)
	if err != nil || len(finish) != 1 || finish[0] != "job-parked" {
		t.Fatalf("finish due = %v, %v; want job-parked", finish, err)
	}
	if due := dueByLane(t, k); len(due) != 0 {
		t.Fatalf("a new target is due while a candidate is parked: %v", due)
	}

	// Resumed, it is a job in flight; dropped, the hot problems are due.
	if _, err := ledger.DropParked("job-parked", problemlog.RepairFailed, "the parked candidate is stale", repairNow); err != nil {
		t.Fatal(err)
	}
	k = selfRepairKernel(t, nil, ledger, nil, repairNow)
	if finish, _ := problemlog.FinishDue(k); len(finish) != 0 {
		t.Fatalf("a dropped candidate is still due: %v", finish)
	}
	if due := dueByLane(t, k); len(due[problemlog.LaneRepair]) == 0 {
		t.Fatalf("no hot problem is due once the candidate is dropped: %v", due)
	}
}
