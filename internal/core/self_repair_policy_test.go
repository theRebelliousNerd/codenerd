package core

import (
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/problemlog"
	"codenerd/internal/projectdoc"
	"codenerd/internal/types"
)

// selfRepairKernel is the production policy over the problems-log fixture,
// debug-self on, the logging knobs asserted as the chat asserts them, and the
// given ledger and timings at now.
func selfRepairKernel(t *testing.T, logging *config.LoggingConfig, ledger *problemlog.RepairLedger, timings []problemlog.Timing, now time.Time) *RealKernel {
	t.Helper()
	k, _ := selfProblemsKernel(t, 0)
	if err := k.Assert(Fact{Predicate: problemlog.DebugSelfActivePredicate}); err != nil {
		t.Fatal(err)
	}
	if err := config.EnsureParams(k, logging.Params()); err != nil {
		t.Fatal(err)
	}
	if ledger == nil {
		ledger = emptyLedger(t)
	}
	if err := problemlog.ReplaceRepair(k, ledger, now); err != nil {
		t.Fatal(err)
	}
	if err := problemlog.ReplaceTimings(k, timings); err != nil {
		t.Fatal(err)
	}
	return k
}

func emptyLedger(t *testing.T) *problemlog.RepairLedger {
	t.Helper()
	l, err := problemlog.LoadRepairLedger(filepath.Join(t.TempDir(), "ledger.json"), time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func dueByLane(t *testing.T, k *RealKernel) map[string][]string {
	t.Helper()
	due, err := problemlog.DueTargets(k)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string][]string{}
	for _, d := range due {
		out[d.Lane] = append(out[d.Lane], d.Target)
	}
	return out
}

func hotSignatures(t *testing.T, k *RealKernel) []string {
	t.Helper()
	rows, err := k.Query("self_problem_hot")
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, f := range rows {
		out = append(out, types.ExtractString(f.Args[0]))
	}
	sort.Strings(out)
	return out
}

var repairNow = time.Unix(1_800_000_000, 0)

// With debug-self on and nothing tried yet, every hot problem is due on the
// repair lane, and the persona is the one the self-diagnosis verb routes to.
func TestSelfRepair_HotProblemsAreDue(t *testing.T) {
	k := selfRepairKernel(t, nil, nil, nil, repairNow)
	hot := hotSignatures(t, k)
	if len(hot) == 0 {
		t.Fatal("the fixture has no hot problem")
	}
	due := dueByLane(t, k)
	if got := strings.Join(due[problemlog.LaneRepair], "|"); got != strings.Join(hot, "|") {
		t.Fatalf("repair due = %v, want every hot problem %v", due[problemlog.LaneRepair], hot)
	}
	if len(due[problemlog.LaneOptimize]) != 0 {
		t.Fatalf("optimize is due while problems are: %v", due)
	}
	shard, err := problemlog.RepairShard(k)
	if err != nil || shard != "reviewer" {
		t.Fatalf("self_repair_shard = %q, %v; want reviewer", shard, err)
	}
}

// Nothing is due while debug-self is off or self-repair is disabled.
func TestSelfRepair_GatedByDebugSelfAndConfig(t *testing.T) {
	k := selfRepairKernel(t, &config.LoggingConfig{SelfRepairDisabled: true}, nil, nil, repairNow)
	if due := dueByLane(t, k); len(due) != 0 {
		t.Fatalf("due with self_repair_disabled: %v", due)
	}
	k = selfRepairKernel(t, nil, nil, nil, repairNow)
	if err := k.Retract(problemlog.DebugSelfActivePredicate); err != nil {
		t.Fatal(err)
	}
	if due := dueByLane(t, k); len(due) != 0 {
		t.Fatalf("due with debug-self off: %v", due)
	}
}

// A job in flight closes the slot (max_jobs 1); a target in cooldown, a
// target that failed max_failures times, and one awaiting its verdict are not
// due; a fix awaiting the restart closes the slot entirely.
func TestSelfRepair_HeldTargetsAndTheSlot(t *testing.T) {
	probe := selfRepairKernel(t, nil, nil, nil, repairNow)
	hot := hotSignatures(t, probe)
	if len(hot) < 3 {
		t.Fatalf("the fixture needs 3 hot problems, has %v", hot)
	}

	// In flight: nothing else starts.
	l := emptyLedger(t)
	if err := l.Begin(hot[:1], problemlog.LaneRepair, "job-1", nil, repairNow); err != nil {
		t.Fatal(err)
	}
	if due := dueByLane(t, selfRepairKernel(t, nil, l, nil, repairNow)); len(due) != 0 {
		t.Fatalf("due while a job runs with max_jobs 1: %v", due)
	}
	// Two jobs allowed: the others are due, the one in flight is not.
	due := dueByLane(t, selfRepairKernel(t, &config.LoggingConfig{SelfRepairMaxJobs: 2}, l, nil, repairNow))
	if strings.Join(due[problemlog.LaneRepair], "|") != strings.Join(hot[1:], "|") {
		t.Fatalf("with max_jobs 2, due = %v, want %v", due, hot[1:])
	}

	// Failed once and cooling down; past the cooldown it is due again.
	if _, err := l.EndJob("job-1", problemlog.RepairFailed, "no change", repairNow); err != nil {
		t.Fatal(err)
	}
	due = dueByLane(t, selfRepairKernel(t, nil, l, nil, repairNow.Add(time.Minute)))
	if hasTarget(due[problemlog.LaneRepair], hot[0]) || len(due[problemlog.LaneRepair]) != len(hot)-1 {
		t.Fatalf("a target in cooldown is due: %v", due)
	}
	later := repairNow.Add(time.Duration(config.DefaultSelfRepairCooldownSec+1) * time.Second)
	if due := dueByLane(t, selfRepairKernel(t, nil, l, nil, later)); !hasTarget(due[problemlog.LaneRepair], hot[0]) {
		t.Fatalf("a target past its cooldown is not due: %v", due)
	}

	// Failed max_failures (2) times: left for good.
	if err := l.Begin(hot[:1], problemlog.LaneRepair, "job-2", nil, later); err != nil {
		t.Fatal(err)
	}
	if _, err := l.EndJob("job-2", problemlog.RepairFailed, "build failed", later); err != nil {
		t.Fatal(err)
	}
	muchLater := later.Add(24 * time.Hour)
	if due := dueByLane(t, selfRepairKernel(t, nil, l, nil, muchLater)); hasTarget(due[problemlog.LaneRepair], hot[0]) {
		t.Fatalf("a target that failed twice is due again: %v", due)
	}

	// A fix awaiting the restart closes the slot.
	if err := l.Begin(hot[1:2], problemlog.LaneRepair, "job-3", nil, muchLater); err != nil {
		t.Fatal(err)
	}
	if err := l.Fixing("job-3", muchLater); err != nil {
		t.Fatal(err)
	}
	if n, err := l.Installed("bbbb", muchLater); err != nil || n != 1 {
		t.Fatalf("installed %d, %v", n, err)
	}
	if due := dueByLane(t, selfRepairKernel(t, nil, l, nil, muchLater)); len(due) != 0 {
		t.Fatalf("due while a fix awaits its restart: %v", due)
	}
}

func hasTarget(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}

// After the restart: a repaired problem the new process logs again regressed;
// one that stays away for the verify window is verified; inside the window
// with no recurrence there is no verdict yet.
func TestSelfRepair_VerdictsAfterTheRestart(t *testing.T) {
	probe := selfRepairKernel(t, nil, nil, nil, repairNow)
	hot := hotSignatures(t, probe)
	path := filepath.Join(t.TempDir(), "ledger.json")
	l, err := problemlog.LoadRepairLedger(path, repairNow)
	if err != nil {
		t.Fatal(err)
	}
	// The fixture's problems are all logged in 2026; a fix taken up for
	// verification before them sees them recur, one taken up after does not.
	if err := l.Begin(hot[:1], problemlog.LaneRepair, "job-1", nil, repairNow); err != nil {
		t.Fatal(err)
	}
	if err := l.Fixing("job-1", repairNow); err != nil {
		t.Fatal(err)
	}
	if _, err := l.Installed("bbbb", repairNow); err != nil {
		t.Fatal(err)
	}

	early := time.Unix(1_000_000_000, 0) // before every fixture line
	adopted, err := problemlog.LoadRepairLedger(path, early)
	if err != nil {
		t.Fatal(err)
	}
	if a, _ := adopted.Get(hot[0]); a.Status != problemlog.RepairVerifying || a.UpdatedUnix != early.Unix() {
		t.Fatalf("the new process did not take the fix up for verification: %+v", a)
	}
	k := selfRepairKernel(t, nil, adopted, nil, early)
	verified, regressed, err := problemlog.Verdicts(k)
	if err != nil {
		t.Fatal(err)
	}
	if len(verified) != 0 || len(regressed) != 1 || regressed[0] != hot[0] {
		t.Fatalf("recurring problem: verified %v regressed %v, want %q regressed", verified, regressed, hot[0])
	}

	// Taken up after the fixture's last line: no recurrence.
	if _, err := adopted.Verdict(hot[0], problemlog.RepairRegressed, "", early); err != nil {
		t.Fatal(err)
	}
	l2, err := problemlog.LoadRepairLedger(filepath.Join(t.TempDir(), "ledger.json"), repairNow)
	if err != nil {
		t.Fatal(err)
	}
	if err := l2.Begin(hot[:1], problemlog.LaneRepair, "job-2", nil, repairNow); err != nil {
		t.Fatal(err)
	}
	_ = l2.Fixing("job-2", repairNow)
	_, _ = l2.Installed("cccc", repairNow)
	reloaded := reloadLedger(t, l2, repairNow)
	inside := repairNow.Add(time.Minute)
	if v, r, _ := problemlog.Verdicts(selfRepairKernel(t, nil, reloaded, nil, inside)); len(v)+len(r) != 0 {
		t.Fatalf("a verdict inside the verify window: verified %v regressed %v", v, r)
	}
	past := repairNow.Add(time.Duration(config.DefaultSelfRepairVerifyWindowSec+1) * time.Second)
	if v, r, _ := problemlog.Verdicts(selfRepairKernel(t, nil, reloaded, nil, past)); len(v) != 1 || len(r) != 0 {
		t.Fatalf("past the window with no recurrence: verified %v regressed %v, want verified", v, r)
	}
}

func reloadLedger(t *testing.T, l *problemlog.RepairLedger, now time.Time) *problemlog.RepairLedger {
	t.Helper()
	path := ledgerPathOf(t, l)
	out, err := problemlog.LoadRepairLedger(path, now)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func ledgerPathOf(t *testing.T, l *problemlog.RepairLedger) string {
	t.Helper()
	p := l.Path()
	if p == "" {
		t.Fatal("ledger has no path")
	}
	return p
}

// The optimize lane: once nothing is left to repair, the recurring operation
// with the largest total time since boot settled is due; one timed too rarely
// after boot, one whose cost is the boot's, or one that waits on a model, is
// not. Its fix is judged against the baseline mean, on the new process's
// timings after its own boot.
func TestSelfRepair_OptimizeTheCostliestRecurringOperation(t *testing.T) {
	timings := []problemlog.Timing{
		{System: "kernel", Operation: "Evaluate", Count: 40, TotalMs: 8000, SettledCount: 40, SettledTotalMs: 8000},
		{System: "store", Operation: "Query", Count: 100, TotalMs: 3000, SettledCount: 100, SettledTotalMs: 3000},
		{System: "api", Operation: "Complete", Count: 50, TotalMs: 90000, SettledCount: 50, SettledTotalMs: 90000},
		{System: "world", Operation: "Scan", Count: 3, TotalMs: 20000, SettledCount: 3, SettledTotalMs: 20000},
		// Costliest over the run, but 22 of its 24 runs were the boot's
		// (live 2026-11-12): not the steady state's cost to cut.
		{System: "kernel", Operation: "Boot", Count: 24, TotalMs: 60000, SettledCount: 2, SettledTotalMs: 40},
	}
	k := selfRepairKernel(t, nil, nil, timings, repairNow)
	if due := dueByLane(t, k); len(due[problemlog.LaneOptimize]) != 0 {
		t.Fatalf("optimize due while problems are hot: %v", due)
	}
	// Every hot problem left (failed twice): optimize.
	l := emptyLedger(t)
	hot := hotSignatures(t, k)
	for i := 0; i < config.DefaultSelfRepairMaxFailures; i++ {
		if err := l.Begin(hot, problemlog.LaneRepair, "job", nil, repairNow); err != nil {
			t.Fatal(err)
		}
		if _, err := l.EndJob("job", problemlog.RepairFailed, "", repairNow); err != nil {
			t.Fatal(err)
		}
	}
	due := dueByLane(t, selfRepairKernel(t, nil, l, timings, repairNow))
	want := problemlog.TimingTarget("kernel", "Evaluate")
	if len(due[problemlog.LaneRepair]) != 0 || strings.Join(due[problemlog.LaneOptimize], "|") != want {
		t.Fatalf("due = %v, want only %s on the optimize lane", due, want)
	}

	// Its fix ran; the new process times it faster: verified. Slower: regressed.
	if err := l.Begin([]string{want}, problemlog.LaneOptimize, "opt", map[string]int64{want: 200}, repairNow); err != nil {
		t.Fatal(err)
	}
	_ = l.Fixing("opt", repairNow)
	_, _ = l.Installed("dddd", repairNow)
	adopted := reloadLedger(t, l, repairNow)
	// The new process's boot is slow again; its steady state is what counts.
	faster := []problemlog.Timing{{System: "kernel", Operation: "Evaluate", Count: 47, TotalMs: 50000, SettledCount: 25, SettledTotalMs: 2500}}
	if v, r, _ := problemlog.Verdicts(selfRepairKernel(t, nil, adopted, faster, repairNow)); len(v) != 1 || v[0] != want || len(r) != 0 {
		t.Fatalf("faster: verified %v regressed %v", v, r)
	}
	slower := []problemlog.Timing{{System: "kernel", Operation: "Evaluate", Count: 25, TotalMs: 7500, SettledCount: 25, SettledTotalMs: 7500}}
	if v, r, _ := problemlog.Verdicts(selfRepairKernel(t, nil, adopted, slower, repairNow)); len(v) != 0 || len(r) != 1 {
		t.Fatalf("slower: verified %v regressed %v", v, r)
	}
	tooFew := []problemlog.Timing{{System: "kernel", Operation: "Evaluate", Count: 30, TotalMs: 10, SettledCount: 2, SettledTotalMs: 10}}
	if v, r, _ := problemlog.Verdicts(selfRepairKernel(t, nil, adopted, tooFew, repairNow)); len(v)+len(r) != 0 {
		t.Fatalf("judged on too few samples: verified %v regressed %v", v, r)
	}
}

// While a self-repair fix turn runs, the safety gate and the permission logic
// are project_forbidden_path: the executor's write gate refuses them and the
// constitution's coder_edit_barred bars them. Outside a fix turn they are not.
func TestSelfRepair_FixTurnNeverTouchesTheSafetyGate(t *testing.T) {
	k := selfRepairKernel(t, nil, nil, nil, repairNow)
	target := "/work/codenerd/internal/core/defaults/policy/constitution.mg"
	if _, forbidden, err := projectdoc.ForbiddenByKernel(k, target); err != nil || forbidden {
		t.Fatalf("forbidden outside a fix turn: %v, %v", forbidden, err)
	}
	if err := k.Assert(Fact{Predicate: problemlog.RepairFixingPredicate}); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{
		target,
		"internal/session/write_guards.go",
		"cmd/nerd/chat/yolo.go",
		"internal/core/defaults/policy/self_repair.mg",
		`cmd\nerd\chat\self_repair.go`,
	} {
		reason, forbidden, err := projectdoc.ForbiddenByKernel(k, p)
		if err != nil || !forbidden || !strings.Contains(reason, "self-repair") {
			t.Errorf("%s during a fix turn: forbidden=%v reason=%q err=%v", p, forbidden, reason, err)
		}
	}
	if _, forbidden, _ := projectdoc.ForbiddenByKernel(k, "internal/problemlog/report.go"); forbidden {
		t.Error("an ordinary source file is forbidden during a fix turn")
	}
	// The constitutional path: an edit of the constitution is barred.
	if err := k.Assert(Fact{Predicate: "pending_action", Args: []any{
		types.MangleString("act-1"), types.MangleAtom("/edit_file"), types.MangleString(target), types.MangleString("{}"), int64(0),
	}}); err != nil {
		t.Fatal(err)
	}
	rows, err := k.Query("coder_edit_barred")
	if err != nil {
		t.Fatal(err)
	}
	barred := false
	for _, f := range rows {
		if types.ExtractString(f.Args[1]) == target {
			barred = true
		}
	}
	if !barred {
		t.Fatalf("coder_edit_barred does not bar the constitution during a fix turn: %v", rows)
	}
}
