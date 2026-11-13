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

// A diagnosis that found nothing to fix settles only the targets it names, as
// /no_defect with the evidence it was judged on; it is not a failure, it
// survives a reload, and it becomes the kernel's evidence fact. The other
// targets go on to the fix turn.
func TestRepairLedger_NoDefectSettlesNamedTargetsWithoutAFailure(t *testing.T) {
	l, path := newLedger(t)
	now := time.Unix(1000, 0)
	if err := l.Begin([]string{"sig-a", "sig-b"}, LaneRepair, "job-1", nil, now); err != nil {
		t.Fatal(err)
	}
	if n, err := l.NoDefect("job-1", map[string]int{"sig-a": 4, "sig-x": 9}, "nothing of codeNERD's to fix", now); err != nil || n != 1 {
		t.Fatalf("no defect settled %d, %v; want only sig-a", n, err)
	}
	if err := l.Fixing("job-1", now); err != nil {
		t.Fatal(err)
	}
	a, _ := l.Get("sig-a")
	if a.Status != RepairNoDefect || a.Failures != 0 || a.EvidenceCount != 4 || a.Phase != "" {
		t.Fatalf("sig-a = %+v, want no_defect, no failure, evidence 4", a)
	}
	if b, _ := l.Get("sig-b"); b.Status != RepairBeingFixed || b.Phase != PhaseFixing {
		t.Fatalf("sig-b = %+v, want its fix turn running", b)
	}
	next, err := LoadRepairLedger(path, time.Unix(2000, 0))
	if err != nil {
		t.Fatal(err)
	}
	if a, _ := next.Get("sig-a"); a.Status != RepairNoDefect || a.EvidenceCount != 4 {
		t.Fatalf("after reload sig-a = %+v", a)
	}
	var evidence []types.Fact
	for _, f := range next.Facts() {
		if f.Predicate == RepairEvidencePredicate {
			evidence = append(evidence, f)
		}
	}
	if len(evidence) != 1 || evidence[0].Args[1] != int64(4) {
		t.Fatalf("evidence facts = %v, want one for sig-a at 4", evidence)
	}
	// A new attempt on it clears the evidence.
	if err := next.Begin([]string{"sig-a"}, LaneRepair, "job-2", nil, now); err != nil {
		t.Fatal(err)
	}
	if _, err := next.EndJob("job-2", RepairStopped, "", now); err != nil {
		t.Fatal(err)
	}
	if a, _ := next.Get("sig-a"); a.EvidenceCount != 0 || a.Tries != 2 {
		t.Fatalf("after a stopped retry sig-a = %+v", a)
	}
}

// A fix turn's window and what it left survive the process: the window ends
// where the turn ended, and only a failed or stopped attempt carries the
// undone diff and the failing tests, which the next settle clears.
func TestRepairLedger_FixWindowsAndAnnotationsPersist(t *testing.T) {
	l, path := newLedger(t)
	if err := l.Begin([]string{"a", "b"}, LaneRepair, "job-1", nil, time.Unix(100, 0)); err != nil {
		t.Fatal(err)
	}
	if _, err := l.NoDefect("job-1", map[string]int{"b": 3}, "nothing to fix", time.Unix(110, 0)); err != nil {
		t.Fatal(err)
	}
	if err := l.Fixing("job-1", time.Unix(120, 0)); err != nil {
		t.Fatal(err)
	}
	if err := l.OpenFixWindow("job-1", time.Unix(120, 0)); err != nil {
		t.Fatal(err)
	}
	if facts := l.WindowFacts(time.Unix(150, 0)); len(facts) != 2 || facts[0].Args[2] != int64(150) ||
		facts[1].Predicate != RepairFixOpenPredicate || facts[1].Args[1] != int64(120) {
		t.Fatalf("an open window's facts = %v, want it to end now and be named open from its start", facts)
	}
	if _, err := l.EndJob("job-1", RepairFailed, "the fix turn ended", time.Unix(200, 0)); err != nil {
		t.Fatal(err)
	}
	if err := l.CloseFixWindow("job-1", time.Unix(200, 0)); err != nil {
		t.Fatal(err)
	}
	if n, err := l.Annotate("job-1", "its edits were undone", ".nerd/self_repair/attempts/job-1.diff", []string{"pkg.TestX: x_test.go:3: boom"}); err != nil || n != 1 {
		t.Fatalf("annotated %d (%v), want the failed attempt only", n, err)
	}
	next, err := LoadRepairLedger(path, time.Unix(300, 0))
	if err != nil {
		t.Fatal(err)
	}
	if ws := next.FixWindows(); len(ws) != 1 || ws[0].StartUnix != 120 || ws[0].EndUnix != 200 {
		t.Fatalf("windows after reload = %+v", ws)
	}
	a, _ := next.Get("a")
	if a.AttemptDiff != ".nerd/self_repair/attempts/job-1.diff" || len(a.FailedTests) != 1 || a.Note != "the fix turn ended; its edits were undone" {
		t.Fatalf("failed attempt after reload = %+v", a)
	}
	if b, _ := next.Get("b"); b.AttemptDiff != "" || len(b.FailedTests) != 0 {
		t.Fatalf("the no-defect attempt was annotated: %+v", b)
	}
	// The next attempt's settle clears what the last one left.
	if err := next.Begin([]string{"a"}, LaneRepair, "job-2", nil, time.Unix(400, 0)); err != nil {
		t.Fatal(err)
	}
	if _, err := next.EndJob("job-2", RepairStopped, "stopped", time.Unix(410, 0)); err != nil {
		t.Fatal(err)
	}
	if a, _ := next.Get("a"); a.AttemptDiff != "" || len(a.FailedTests) != 0 {
		t.Fatalf("a later attempt kept the old diff: %+v", a)
	}
}
