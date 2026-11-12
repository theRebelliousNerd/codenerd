package core

import (
	"sort"
	"testing"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/problemlog"
	"codenerd/internal/types"
)

// The verdict predicates (turnjudge's VerdictPredicate and friends; that
// package imports this one, so the names are spelled here).
const (
	verdictPred   = "self_diagnosis_verdict"
	finishedPred  = "self_diagnosis_finished"
	fixDuePred    = "self_diagnosis_fix_due"
	fixTargetPred = "self_diagnosis_fix_target"
	noDefectPred  = "self_diagnosis_no_defect"
	jobKey        = "*"
)

type fixDecision struct {
	Due           bool
	Fix, NoDefect []string
}

// verdictKernel is the production policy with the Jev thresholds asserted and
// the given jobs finished, each with its verdicts (target -> percent).
func verdictKernel(t *testing.T, finished []string, verdicts map[string]map[string]float64) *RealKernel {
	t.Helper()
	k, err := NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	if err := config.EnsureParams(k, (*config.TypeSafeConfig)(nil).Params()); err != nil {
		t.Fatal(err)
	}
	var facts []types.Fact
	for _, job := range finished {
		facts = append(facts, types.Fact{Predicate: finishedPred, Args: []any{types.MangleString(job)}})
	}
	for job, byTarget := range verdicts {
		for target, p := range byTarget {
			facts = append(facts, types.Fact{Predicate: verdictPred, Args: []any{
				types.MangleString(job), types.MangleString(target), types.PercentFromRatio(p)}})
		}
	}
	if len(facts) > 0 {
		if err := k.AssertBatch(facts); err != nil {
			t.Fatal(err)
		}
	}
	return k
}

func decide(t *testing.T, k *RealKernel, job string) fixDecision {
	t.Helper()
	var d fixDecision
	rows, err := k.Query(fixDuePred)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if types.ExtractString(r.Args[0]) == job {
			d.Due = true
		}
	}
	d.Fix = jobColumn(t, k, fixTargetPred, job)
	d.NoDefect = jobColumn(t, k, noDefectPred, job)
	sort.Strings(d.Fix)
	sort.Strings(d.NoDefect)
	return d
}

// The verdict's truth table, per target of a finished job: at or below no is
// nothing to fix; unsettled and yes are fix targets; a job is due when any
// target is a fix target.
func TestSelfDiagnosisVerdict_TruthTable(t *testing.T) {
	k := verdictKernel(t, []string{"job-mixed", "job-clean"}, map[string]map[string]float64{
		"job-mixed": {"sig-no": 0.05, "sig-maybe": 0.5, "sig-yes": 0.9},
		"job-clean": {"sig-a": 0.01, "sig-b": 0.30},
	})
	mixed := decide(t, k, "job-mixed")
	if !mixed.Due || len(mixed.Fix) != 2 || mixed.Fix[0] != "sig-maybe" || mixed.Fix[1] != "sig-yes" ||
		len(mixed.NoDefect) != 1 || mixed.NoDefect[0] != "sig-no" {
		t.Fatalf("mixed job = %+v; want due, fix [sig-maybe sig-yes], no defect [sig-no]", mixed)
	}
	// Every target at or below no (30 is no's own threshold): no fix turn.
	clean := decide(t, k, "job-clean")
	if clean.Due || len(clean.Fix) != 0 || len(clean.NoDefect) != 2 {
		t.Fatalf("clean job = %+v; want not due, both targets no defect", clean)
	}
}

// Jev off, or its call failed: the finished job has no verdict, and the fix
// turn runs as before the verdict. A job that has not finished derives
// nothing, verdict or not.
func TestSelfDiagnosisVerdict_UnjudgedRunsAsBeforeAndUnfinishedDerivesNothing(t *testing.T) {
	k := verdictKernel(t, []string{"job-unjudged"}, map[string]map[string]float64{
		"job-running": {"sig-a": 0.01},
	})
	if d := decide(t, k, "job-unjudged"); !d.Due || len(d.Fix) != 0 || len(d.NoDefect) != 0 {
		t.Fatalf("unjudged job = %+v; want due with no per-target split", d)
	}
	if d := decide(t, k, "job-running"); d.Due || len(d.NoDefect) != 0 {
		t.Fatalf("unfinished job = %+v; want nothing derived", d)
	}
}

// A job-level verdict ("*", a job that names no targets) decides the same way.
func TestSelfDiagnosisVerdict_JobLevel(t *testing.T) {
	k := verdictKernel(t, []string{"job-1"}, map[string]map[string]float64{"job-1": {jobKey: 0.01}})
	if d := decide(t, k, "job-1"); d.Due || len(d.NoDefect) != 1 || d.NoDefect[0] != jobKey {
		t.Fatalf("job-level no = %+v; want not due", d)
	}
}

// A target whose diagnosis found nothing to fix is held on the same evidence
// however long ago it settled, is not a failure, and is due again once its
// evidence has grown logging.self_repair_no_defect_regrowth times over.
func TestSelfRepair_NoDefectIsHeldUntilTheEvidenceGrows(t *testing.T) {
	probe := selfRepairKernel(t, nil, nil, nil, repairNow)
	hot := hotSignatures(t, probe)
	target := hot[0]
	count := problemCount(t, probe, target)

	l := emptyLedger(t)
	if err := l.Begin([]string{target}, problemlog.LaneRepair, "job-1", nil, repairNow); err != nil {
		t.Fatal(err)
	}
	if n, err := l.NoDefect("job-1", map[string]int{target: count}, "nothing of codeNERD's to fix", repairNow); err != nil || n != 1 {
		t.Fatalf("no defect %d, %v", n, err)
	}
	if a, _ := l.Get(target); a.Failures != 0 {
		t.Fatalf("a no-defect verdict counted as a failure: %+v", a)
	}
	// Long past the cooldown, on the same evidence: held.
	muchLater := repairNow.Add(30 * 24 * time.Hour)
	if due := dueByLane(t, selfRepairKernel(t, nil, l, nil, muchLater)); hasTarget(due[problemlog.LaneRepair], target) {
		t.Fatalf("a no-defect target is due again on the same evidence: %v", due)
	}
	// The other hot problems are not held by it.
	if due := dueByLane(t, selfRepairKernel(t, nil, l, nil, muchLater)); len(due[problemlog.LaneRepair]) != len(hot)-1 {
		t.Fatalf("due = %v, want every other hot problem", due)
	}
	// Grown past the factor (judged on half or less of what is logged now,
	// with the factor at 1 when the fixture logged it once): due again.
	factor := 2
	base := count / factor
	if base < 1 {
		base, factor = 1, 1
	}
	grown := emptyLedger(t)
	if err := grown.Begin([]string{target}, problemlog.LaneRepair, "job-2", nil, repairNow); err != nil {
		t.Fatal(err)
	}
	if _, err := grown.NoDefect("job-2", map[string]int{target: base}, "", repairNow); err != nil {
		t.Fatal(err)
	}
	cfg := &config.LoggingConfig{SelfRepairNoDefectRegrowth: factor}
	if due := dueByLane(t, selfRepairKernel(t, cfg, grown, nil, muchLater)); !hasTarget(due[problemlog.LaneRepair], target) {
		t.Fatalf("a no-defect target whose evidence grew %dx (%d -> %d) is not due: %v", factor, base, count, due)
	}
}

func problemCount(t *testing.T, k *RealKernel, sig string) int {
	t.Helper()
	rows, err := k.Query(problemlog.ProblemPredicate)
	if err != nil {
		t.Fatal(err)
	}
	best := 0
	for _, f := range rows {
		if types.ExtractString(f.Args[0]) != sig {
			continue
		}
		if n, ok := f.Args[3].(int64); ok && int(n) > best {
			best = int(n)
		}
	}
	if best == 0 {
		t.Fatalf("no self_problem count for %q", sig)
	}
	return best
}

func jobColumn(t *testing.T, k *RealKernel, pred, job string) []string {
	t.Helper()
	rows, err := k.Query(pred)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, r := range rows {
		if types.ExtractString(r.Args[0]) == job {
			out = append(out, types.ExtractString(r.Args[1]))
		}
	}
	return out
}
