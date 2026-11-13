package chat

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/problemlog"
	"codenerd/internal/session"
)

// parkedRig is a fix chain on job that wrote rel (from before to after) with
// its own package tests green, stopped by the chat for running past its
// budget.
func parkedRig(t *testing.T, job, rel, before, after string) (selfRepairRig, *fakeRebuild) {
	t.Helper()
	r := newSelfRepairRig(t, "", "20261028_061954_problems.log", nilSessionError)
	rebuild := &fakeRebuild{}
	rebuild.install(t, &r.m, func(string, []string, []string) error { return nil })
	ws := r.m.workspace
	path := filepath.Join(ws, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(after), 0o644); err != nil {
		t.Fatal(err)
	}
	now := r.m.selfRepair.clock()
	ledger := r.ledger(t)
	if err := ledger.Begin([]string{"sig-" + job}, problemlog.LaneRepair, job, nil, now); err != nil {
		t.Fatal(err)
	}
	if err := ledger.Fixing(job, now); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(after))
	j := session.NewWriteJournal()
	j.Record(ws, []string{rel}, map[string]session.PreImage{rel: {Existed: true, Content: before, LastWriteHash: hex.EncodeToString(sum[:])}})
	j.NoteTestsPassed()
	r.m.selfRepair.mu.Lock()
	r.m.selfRepair.journal, r.m.selfRepair.journalJob, r.m.selfRepair.overBudgetJob = j, job, job
	r.m.selfRepair.mu.Unlock()
	return r, rebuild
}

func readRel(t *testing.T, ws, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(ws, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// A fix chain that ran out of budget with its own package tests green is not
// thrown away: its change is undone and parked with that evidence, the kernel
// makes finishing it due before any new target, and finishing re-applies it
// to the tree it was parked on and runs only the gate it had not reached --
// the self-rebuild -- which installs it.
func TestSelfRepair_ABudgetStoppedGreenChangeIsParkedThenFinished(t *testing.T) {
	t.Parallel()
	const rel = "internal/session/executor.go"
	const before, after = "package session\n\n// before\n", "package session\n\n// the green fix\n"
	r, rebuild := parkedRig(t, "job-green", rel, before, after)
	m := r.m.settleFixTurn()
	ws := m.workspace

	if got := readRel(t, ws, rel); got != before {
		t.Fatalf("the parked change is still in the tree: %q", got)
	}
	a, _ := r.ledger(t).Get("sig-job-green")
	wantManifest := ".nerd/self_repair/parked/job-green.json"
	if a.Status != problemlog.RepairParked || a.Parked != wantManifest || a.Failures != 0 {
		t.Fatalf("ledger after the budget stop = %+v, want parked on %s, no failure", a, wantManifest)
	}
	c, err := problemlog.LoadParkedCandidate(ws, wantManifest)
	if err != nil || len(c.GatesPassed) != 1 || c.GatesPassed[0] != problemlog.GatePackageTests || c.GatesPending[0] != problemlog.GateSelfRebuild {
		t.Fatalf("candidate = %+v, %v: want package tests passed, self-rebuild pending", c, err)
	}
	if !strings.Contains(lastAssistant(m), "parked") {
		t.Fatalf("the user is not told the change was parked: %q", lastAssistant(m))
	}

	// The kernel makes finishing it due before any new target.
	plan := m.planSelfRepair(t.Context(), r.ledger(t), m.selfRepair.clock())
	if plan.finish != "job-green" || plan.ok {
		t.Fatalf("plan = finish %q, ok %v: want the parked job finished first", plan.finish, plan.ok)
	}
	m, cmd := m.landSelfRepairPlan(plan)
	if got := readRel(t, ws, rel); got != after {
		t.Fatalf("finishing did not re-apply the change: %q", got)
	}
	if a, _ := r.ledger(t).Get("sig-job-green"); a.Status != problemlog.RepairBeingFixed || a.Phase != problemlog.PhaseFixing {
		t.Fatalf("ledger while finishing = %+v, want being fixed in its fix turn", a)
	}
	if cmd == nil {
		t.Fatal("finishing ran no gate")
	}
	cmd()
	if rebuild.built != 1 || len(rebuild.checked) != 1 {
		t.Fatalf("the self-rebuild ran %d build(s) and %d check(s), want one of each", rebuild.built, len(rebuild.checked))
	}
	if a, _ := r.ledger(t).Get("sig-job-green"); a.Status != problemlog.RepairPendingRestart {
		t.Fatalf("ledger after the finished gate = %+v, want installed (pending restart)", a)
	}
}

// A parked change whose tree moved is stale: finishing drops it, says so, and
// writes nothing.
func TestSelfRepair_AStaleParkedChangeIsDropped(t *testing.T) {
	t.Parallel()
	const rel = "internal/session/executor.go"
	const before, after = "package session\n\n// before\n", "package session\n\n// the green fix\n"
	r, rebuild := parkedRig(t, "job-stale", rel, before, after)
	m := r.m.settleFixTurn()
	ws := m.workspace
	const moved = "package session\n\n// someone else's later edit\n"
	if err := os.WriteFile(filepath.Join(ws, filepath.FromSlash(rel)), []byte(moved), 0o644); err != nil {
		t.Fatal(err)
	}
	m, _ = m.finishParkedCandidate(r.ledger(t), "job-stale", m.selfRepair.clock())
	if got := readRel(t, ws, rel); got != moved {
		t.Fatalf("a stale candidate was applied over a later edit: %q", got)
	}
	a, _ := r.ledger(t).Get("sig-job-stale")
	if a.Status != problemlog.RepairFailed || !strings.Contains(a.Note, "stale") || !strings.Contains(a.Note, rel) {
		t.Fatalf("ledger after the stale finish = %+v, want failed, naming the moved path", a)
	}
	if !strings.Contains(lastAssistant(m), "stale") {
		t.Fatalf("the user is not told the candidate was stale: %q", lastAssistant(m))
	}
	if rebuild.built != 0 {
		t.Fatal("a stale candidate ran the self-rebuild")
	}
}

// A chain stopped by its budget whose own tests failed is not parked: it ends
// over budget, as before.
func TestSelfRepair_ABudgetStoppedRedChangeIsNotParked(t *testing.T) {
	t.Parallel()
	const rel = "internal/session/executor.go"
	r, _ := parkedRig(t, "job-red", rel, "package session\n\n// before\n", "package session\n\n// a red fix\n")
	r.m.selfRepair.journal.NoteTestFailures([]string{"codenerd/internal/session.TestX: x_test.go:3: failed"})
	r.m.settleFixTurn()
	if a, _ := r.ledger(t).Get("sig-job-red"); a.Status != problemlog.RepairOverBudget || a.Parked != "" {
		t.Fatalf("ledger = %+v, want over budget and nothing parked", a)
	}
}
