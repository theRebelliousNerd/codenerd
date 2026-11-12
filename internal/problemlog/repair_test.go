package problemlog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codenerd/internal/types"
)

func newLedger(t *testing.T) (*RepairLedger, string) {
	t.Helper()
	path := RepairLedgerPath(t.TempDir())
	l, err := LoadRepairLedger(path, time.Unix(100, 0))
	if err != nil {
		t.Fatal(err)
	}
	return l, path
}

// The lifecycle end to end: begun, fixing, installed; a new process takes
// the fix up for verification as of its load; the verdict settles it. Every
// step is on disk.
func TestRepairLedger_LifecycleSurvivesTheRestart(t *testing.T) {
	l, path := newLedger(t)
	now := time.Unix(1000, 0)
	if err := l.Begin([]string{"sig-a", "sig-b"}, LaneRepair, "job-1", nil, now); err != nil {
		t.Fatal(err)
	}
	if got := l.JobTargets("job-1"); len(got) != 2 || got[0].Phase != PhaseDiagnosing || got[0].Tries != 1 {
		t.Fatalf("job targets = %+v", got)
	}
	if err := l.Fixing("job-1", now); err != nil {
		t.Fatal(err)
	}
	if jobs := l.InFixing(); len(jobs) != 1 || jobs[0] != "job-1" {
		t.Fatalf("in fixing = %v", jobs)
	}
	if n, err := l.Installed("bbbb", now); err != nil || n != 2 {
		t.Fatalf("installed %d, %v", n, err)
	}
	if len(l.InFixing()) != 0 {
		t.Fatal("an installed fix is still fixing")
	}

	// The same process does not verify its own install.
	if a, _ := l.Get("sig-a"); a.Status != RepairPendingRestart || a.Build != "bbbb" {
		t.Fatalf("after install: %+v", a)
	}
	// A new process does, as of its load, and says so on disk.
	took := time.Unix(2000, 0)
	next, err := LoadRepairLedger(path, took)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := next.Get("sig-a")
	if a.Status != RepairVerifying || a.UpdatedUnix != took.Unix() || a.Build != "bbbb" {
		t.Fatalf("after the restart: %+v", a)
	}
	again, err := LoadRepairLedger(path, time.Unix(3000, 0))
	if err != nil {
		t.Fatal(err)
	}
	if a, _ := again.Get("sig-a"); a.UpdatedUnix != took.Unix() {
		t.Fatalf("a second load re-took a verifying fix: %+v", a)
	}

	if ok, err := next.Verdict("sig-a", RepairVerified, "", took); err != nil || !ok {
		t.Fatalf("verdict: %v %v", ok, err)
	}
	if ok, _ := next.Verdict("sig-a", RepairRegressed, "", took); ok {
		t.Fatal("a settled target took a second verdict")
	}
	if ok, _ := next.Verdict("sig-b", RepairRegressed, "came back", took); !ok {
		t.Fatal("regressed verdict not adopted")
	}
	if b, _ := next.Get("sig-b"); b.Failures != 1 || b.Note != "came back" {
		t.Fatalf("a regression is not a failure: %+v", b)
	}
}

// Failed and regressed count as failures; a stop does not.
func TestRepairLedger_FailuresCount(t *testing.T) {
	l, _ := newLedger(t)
	now := time.Unix(1000, 0)
	for i, status := range []RepairStatus{RepairFailed, RepairStopped, RepairFailed} {
		job := "job-" + string(rune('a'+i))
		if err := l.Begin([]string{"sig"}, LaneRepair, job, nil, now); err != nil {
			t.Fatal(err)
		}
		if n, err := l.EndJob(job, status, string(status), now); err != nil || n != 1 {
			t.Fatalf("end %s: %d %v", status, n, err)
		}
	}
	a, _ := l.Get("sig")
	if a.Tries != 3 || a.Failures != 2 || a.Status != RepairFailed {
		t.Fatalf("%+v, want 3 tries, 2 failures", a)
	}
	// EndFixing touches only fix turns.
	if n, _ := l.EndFixing(RepairFailed, "", now); n != 0 {
		t.Fatalf("EndFixing settled %d attempts with no fix turn running", n)
	}
}

// The ledger as facts: one attempt row each, typed as self_repair.mg
// declares, and the optimize lane's baseline.
func TestRepairLedger_Facts(t *testing.T) {
	l, _ := newLedger(t)
	now := time.Unix(1000, 0)
	target := TimingTarget("kernel", "Evaluate")
	if err := l.Begin([]string{target}, LaneOptimize, "job-o", map[string]int64{target: 240}, now); err != nil {
		t.Fatal(err)
	}
	facts := l.Facts()
	if len(facts) != 2 {
		t.Fatalf("facts = %v", facts)
	}
	att := facts[0]
	if att.Predicate != RepairAttemptPredicate || att.Args[0] != types.MangleString(target) ||
		att.Args[1] != types.MangleAtom("/optimize") || att.Args[2] != types.MangleAtom("/being_fixed") ||
		att.Args[3] != types.MangleString("job-o") || att.Args[4] != int64(1) || att.Args[6] != now.Unix() {
		t.Fatalf("attempt fact = %v", att)
	}
	if b := facts[1]; b.Predicate != RepairBaselinePredicate || b.Args[1] != int64(240) {
		t.Fatalf("baseline fact = %v", b)
	}
}

// An unreadable or foreign ledger is an error, never an empty one: self-repair
// does not start over on attempts it cannot read.
func TestRepairLedger_RejectsAnUnreadableFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ledger.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadRepairLedger(path, time.Now()); err == nil {
		t.Fatal("a corrupt ledger loaded")
	}
	if err := os.WriteFile(path, []byte(`{"version": 99, "attempts": []}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadRepairLedger(path, time.Now()); err == nil || !strings.Contains(err.Error(), "version 99") {
		t.Fatalf("a foreign version loaded: %v", err)
	}
}

type fakeKernel struct {
	retracted []string
	asserted  []types.Fact
	rows      map[string][]types.Fact
}

func (k *fakeKernel) Retract(p string) error { k.retracted = append(k.retracted, p); return nil }
func (k *fakeKernel) AssertBatch(f []types.Fact) error {
	k.asserted = append(k.asserted, f...)
	return nil
}
func (k *fakeKernel) Query(p string) ([]types.Fact, error) { return k.rows[p], nil }

func TestReplaceRepair_SwapsTheLedgerAndTheClock(t *testing.T) {
	l, _ := newLedger(t)
	if err := l.Begin([]string{"sig"}, LaneRepair, "j", nil, time.Unix(5, 0)); err != nil {
		t.Fatal(err)
	}
	k := &fakeKernel{}
	if err := ReplaceRepair(k, l, time.Unix(77, 0)); err != nil {
		t.Fatal(err)
	}
	if strings.Join(k.retracted, ",") != strings.Join(RepairPredicates, ",") {
		t.Fatalf("retracted %v", k.retracted)
	}
	last := k.asserted[len(k.asserted)-1]
	if last.Predicate != RepairClockPredicate || last.Args[0] != int64(77) {
		t.Fatalf("clock fact = %v", last)
	}
}

func TestDueTargetsVerdictsAndShard_ReadTheKernel(t *testing.T) {
	k := &fakeKernel{rows: map[string][]types.Fact{
		RepairDuePredicate: {
			{Predicate: RepairDuePredicate, Args: []any{"timing:kernel.Evaluate", "/optimize"}},
			{Predicate: RepairDuePredicate, Args: []any{"sig-b", "/repair"}},
			{Predicate: RepairDuePredicate, Args: []any{"sig-a", "/repair"}},
		},
		RepairVerifiedPredicate: {{Predicate: RepairVerifiedPredicate, Args: []any{"sig-v"}}},
		RepairRegressedPred:     {{Predicate: RepairRegressedPred, Args: []any{types.MangleString("sig-r")}}},
		RepairShardPredicate:    {{Predicate: RepairShardPredicate, Args: []any{types.MangleAtom("/reviewer")}}},
	}}
	due, err := DueTargets(k)
	if err != nil {
		t.Fatal(err)
	}
	if len(due) != 3 || due[0].Lane != LaneOptimize || due[1] != (Due{"sig-a", LaneRepair}) {
		t.Fatalf("due = %+v", due)
	}
	v, r, err := Verdicts(k)
	if err != nil || len(v) != 1 || v[0] != "sig-v" || len(r) != 1 || r[0] != "sig-r" {
		t.Fatalf("verdicts %v %v %v", v, r, err)
	}
	if s, err := RepairShard(k); err != nil || s != "reviewer" {
		t.Fatalf("shard %q %v", s, err)
	}
}
