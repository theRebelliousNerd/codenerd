package chat

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"codenerd/internal/config"
	"codenerd/internal/perception"
	"codenerd/internal/problemlog"
	"codenerd/internal/projectdoc"
	"codenerd/internal/selfrestart"
)

// selfRepairRig is a yolo chat on a real kernel with debug-self on, a pinned
// self-repair clock, and the given problems-log lines as this run's log.
type selfRepairRig struct {
	m    Model
	exec *processControlTaskExecutor
	now  *time.Time
	mu   *sync.Mutex
}

func newSelfRepairRig(t *testing.T, workspace, logName, log string, intents ...perception.Intent) selfRepairRig {
	t.Helper()
	m, exec := journeyModel(t, config.WorkModeYolo, intents...)
	if workspace != "" {
		m.workspace = workspace
	}
	m.Config.Logging = &config.LoggingConfig{DebugSelf: true, SelfRepairPollSec: 1}
	now := time.Unix(1_800_000_000, 0)
	mu := &sync.Mutex{}
	m.selfRepair = newSelfRepairState()
	m.selfRepair.now = func() time.Time { mu.Lock(); defer mu.Unlock(); return now }
	dir := filepath.Join(m.workspace, ".nerd", "logs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// This process's audit log is the rig's, never another test's.
	audit := filepath.Join(dir, strings.TrimSuffix(logName, "_problems.log")+"_audit.log")
	m.selfRepair.audit = func() string { return audit }
	path := filepath.Join(dir, logName)
	if err := os.WriteFile(path, []byte(log), 0o644); err != nil {
		t.Fatal(err)
	}
	m.selfDebug = newSelfDebugWatcher()
	m.selfDebug.prime(path)
	m.selfRepair.problems = func() string { return path }
	return selfRepairRig{m: m, exec: exec, now: &now, mu: mu}
}

func (r selfRepairRig) advance(d time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	*r.now = r.now.Add(d)
}

func (r selfRepairRig) ledger(t *testing.T) *problemlog.RepairLedger {
	t.Helper()
	l, err := r.m.selfRepair.ledgerFor(r.m.workspace)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

const nilSessionError = "2026/10/28 06:19:54.100 [ERROR] [session] nil session in executor dispatch: panic recovered\n"

// The idle timer, with no one typing, starts the self-diagnosis chain for a
// hot problem of codeNERD's own log: a background job the kernel chose the
// persona of, recorded in the ledger as being fixed.
func TestSelfRepair_TheIdleTimerStartsTheChainUnasked(t *testing.T) {
	t.Parallel()
	r := newSelfRepairRig(t, "", "20261028_061954_problems.log", nilSessionError)
	m, cmd := r.m.handleSelfRepairTick(selfRepairTickMsg{gen: 0})
	if cmd == nil {
		t.Fatal("the tick neither started a job nor kept the timer going")
	}
	jobs := m.background.list()
	if len(jobs) != 1 || jobs[0].SelfRepair != problemlog.LaneRepair || jobs[0].Shard != "reviewer" {
		t.Fatalf("background jobs = %+v, want one reviewer self-repair job", jobs)
	}
	if !strings.Contains(jobs[0].Task, "nil session in executor dispatch") || !strings.Contains(jobs[0].FollowUp, "never change its safety gate") {
		t.Fatalf("the job lacks the evidence or the guarded fix step:\n%s\n%s", jobs[0].Task, jobs[0].FollowUp)
	}
	attempts := r.ledger(t).Attempts()
	if len(attempts) != 1 || attempts[0].Status != problemlog.RepairBeingFixed || attempts[0].Phase != problemlog.PhaseDiagnosing || attempts[0].Job != jobs[0].ID {
		t.Fatalf("ledger = %+v", attempts)
	}
	if _, err := os.Stat(problemlog.RepairLedgerPath(m.workspace)); err != nil {
		t.Fatalf("the ledger is not persisted under .nerd: %v", err)
	}
	if !strings.Contains(lastAssistant(m), "Self-repair (debug-self)") {
		t.Fatalf("the user is not told: %q", lastAssistant(m))
	}
	// While it runs, nothing else starts (logging.self_repair_max_jobs 1).
	again, _ := m.selfRepairStep()
	if n := len(again.background.list()); n != 1 {
		t.Fatalf("%d jobs after a second look, want the one", n)
	}
	// A tick of a superseded timer loop stops.
	m.selfRepair.gen = 5
	if _, cmd := m.handleSelfRepairTick(selfRepairTickMsg{gen: 4}); cmd != nil {
		t.Fatal("a stale timer loop kept ticking")
	}
}

// A session booted with debug-self already on, in which nobody has typed,
// follows its problems log from the timer alone: the problem is found and the
// lines stay pending for the first turn's digest.
func TestSelfRepair_ASessionNobodyTypedInStillFindsItsProblems(t *testing.T) {
	t.Parallel()
	r := newSelfRepairRig(t, "", "20261028_061954_problems.log", nilSessionError)
	r.m.selfDebug = newSelfDebugWatcher() // never primed: no turn, no toggle
	m, _ := r.m.selfRepairStep()
	if n := len(m.background.list()); n != 1 {
		t.Fatalf("%d self-repair jobs, want the one for the logged error", n)
	}
	if p := m.selfDebug.collect(m.selfRepair.problemsLog()); p.TotalErrors != 1 {
		t.Fatalf("the first turn's digest lost the error the timer read: %+v", p)
	}
}

// The user's own work always comes first: nothing starts beside a turn, a
// queued message, held work, or a pending restart; and nothing with debug-self
// off or self-repair disabled.
func TestSelfRepair_NeverStartsBesideTheUsersWork(t *testing.T) {
	t.Parallel()
	for name, mutate := range map[string]func(*Model){
		"turn running":   func(m *Model) { m.isLoading = true },
		"message queued": func(m *Model) { m.turn.enqueue(queuedMessage{input: "and the README"}) },
		"work held":      func(m *Model) { m.pendingWork = &pendingWork{Input: "fix the README", At: time.Now()} },
		"booting":        func(m *Model) { m.isBooting = true },
		"debug-self off": func(m *Model) { m.Config.Logging.DebugSelf = false },
		"disabled":       func(m *Model) { m.Config.Logging.SelfRepairDisabled = true },
		"restored work":  func(m *Model) { m.resumeWork = []selfrestart.ResumeWork{{Kind: selfrestart.ResumeTurn, Input: "x"}} },
	} {
		t.Run(name, func(t *testing.T) {
			r := newSelfRepairRig(t, "", "20261028_061954_problems.log", nilSessionError)
			mutate(&r.m)
			m, _ := r.m.selfRepairStep()
			if n := len(m.background.list()); n != 0 {
				t.Fatalf("a self-repair started beside %s", name)
			}
		})
	}
}

// fakeRebuild is an executable inside the workspace, a restarter watching it,
// and build/check hooks that record their calls.
type fakeRebuild struct {
	exe     string
	built   int
	checked [][]string
	failure error
}

func (f *fakeRebuild) install(t *testing.T, m *Model, exec selfrestart.ExecFunc) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(m.workspace, "cmd", "nerd"), 0o755); err != nil {
		t.Fatal(err)
	}
	f.exe = filepath.Join(m.workspace, "nerd")
	if err := os.WriteFile(f.exe, []byte("old build"), 0o755); err != nil {
		t.Fatal(err)
	}
	id, err := selfrestart.IdentityOf(f.exe)
	if err != nil {
		t.Fatal(err)
	}
	m.restarter = selfrestart.NewRestarter(id, filepath.Join(m.workspace, ".nerd", "restart"), exec)
	m.rebuildHooks = &selfRebuildHooks{
		Build: func(_ context.Context, _, out string) error {
			f.built++
			return os.WriteFile(out, []byte("new build, longer than the old one"), 0o755)
		},
		Check: func(_ context.Context, _, _ string, changed []string) error {
			f.checked = append(f.checked, changed)
			return f.failure
		},
	}
}

// The whole chain with fakes: the idle trigger starts the diagnosis; its
// result starts the fix turn under the safety marker; the verified fix's
// rebuild is self-checked and installed; the session restarts onto it,
// carrying the running job and the queued turn, which the new process starts
// again; and the new process, whose log stays clean for the verify window,
// verifies the fix.
func TestSelfRepair_WalksTriggerJobFixRebuildRestoreResumeVerify(t *testing.T) {
	t.Parallel()
	fix := perception.Intent{
		Category: "/mutation", Verb: "/fix", Target: "internal/session/executor.go", Confidence: 0.9,
		Response: "Fixing the nil check the diagnosis found.",
		Brief:    &perception.Brief{Goal: "fix the root cause the self-diagnosis found", Ready: true},
	}
	r := newSelfRepairRig(t, "", "20261028_061954_problems.log", nilSessionError, fix)
	r.exec.response = "Root cause: executor.go:120 dereferences a nil session. Fix: guard it."
	var argv []string
	rebuild := &fakeRebuild{}
	rebuild.install(t, &r.m, func(_ string, a []string, _ []string) error { argv = a; return nil })

	// 1. Trigger: the timer, unasked.
	m, cmd := r.m.handleSelfRepairTick(selfRepairTickMsg{gen: 0})
	jobs := m.background.list()
	if len(jobs) != 1 {
		t.Fatalf("no self-repair job started: %+v", jobs)
	}
	sig := r.ledger(t).Attempts()[0].Target

	// 2. The diagnosis runs and comes back; its follow-up is the fix turn.
	done := runBatchFor[backgroundDoneMsg](t, cmd)
	next, followUp := m.Update(done)
	m = next.(Model)
	if followUp == nil || !m.isLoading {
		t.Fatal("the chain stopped before its fix turn")
	}
	if a, _ := r.ledger(t).Get(sig); a.Status != problemlog.RepairBeingFixed || a.Phase != problemlog.PhaseFixing {
		t.Fatalf("after the diagnosis: %+v, want the fixing phase", a)
	}
	// During the fix turn the safety gate is off-limits.
	constitution := filepath.Join(m.workspace, "internal", "core", "defaults", "policy", "constitution.mg")
	if _, forbidden, err := projectdoc.ForbiddenByKernel(m.kernel, constitution); err != nil || !forbidden {
		t.Fatalf("the constitution is writable during a self-repair fix turn: %v %v", forbidden, err)
	}

	// 3. The fix turn verified a source change: rebuild, self-check, install.
	if err := m.rebuildWatchedExecutable(m.shutdownCtx, []string{"internal/session/executor.go"}); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	if rebuild.built != 1 || len(rebuild.checked) != 1 || rebuild.checked[0][0] != "internal/session/executor.go" {
		t.Fatalf("built %d, checked %v", rebuild.built, rebuild.checked)
	}
	if data, _ := os.ReadFile(rebuild.exe); !strings.HasPrefix(string(data), "new build") {
		t.Fatal("the checked build was not installed over the running binary")
	}
	if a, _ := r.ledger(t).Get(sig); a.Status != problemlog.RepairPendingRestart {
		t.Fatalf("after the install: %+v, want pending_restart", a)
	}
	if _, forbidden, _ := projectdoc.ForbiddenByKernel(m.kernel, constitution); forbidden {
		t.Fatal("the safety marker outlived the fix turn")
	}
	m.isLoading = false // the fix turn ends

	// Nothing new starts while the fix waits for its restart.
	m, _ = m.selfRepairStep()
	if a, _ := r.ledger(t).Get(sig); a.Status != problemlog.RepairPendingRestart {
		t.Fatalf("idle after the install: %+v", a)
	}

	// 4. Restart once idle, carrying a running job and a queued turn.
	m.background.add(backgroundJob{ID: "bg-keep", Shard: "researcher", Task: "read the retry policy", FollowUp: "then summarize it", Started: time.Now()}, nil)
	m.turn.enqueue(queuedMessage{input: "then tidy the logs"})
	m.isBooting = false
	m.sessionID = "sess-self-repair"
	if err := m.saveSessionState(); err != nil {
		t.Fatal(err)
	}
	st := m.restartState()
	if len(st.Resume) != 2 || st.Resume[0].ID != "bg-keep" || st.Resume[0].FollowUp != "then summarize it" || st.Resume[1].Input != "then tidy the logs" {
		t.Fatalf("restart state carries %+v, want the job and the queued turn", st.Resume)
	}
	m.restartRequest = &st
	if _, err := m.performRestart(nil); err != nil {
		t.Fatalf("performRestart: %v", err)
	}
	restore, err := readRestore(argv[len(argv)-1])
	if err != nil {
		t.Fatal(err)
	}

	// 5. The new process: its own clock, a fresh problems log, the same
	// workspace and ledger.
	fresh := newSelfRepairRig(t, m.workspace, "20261112_090000_problems.log", "")
	fresh.advance(time.Hour)
	fresh.m.sessionID = restore.SessionID
	fresh.m.restore = restore
	fresh.m = fresh.m.applyRestore()
	if !strings.Contains(lastAssistant(fresh.m), "Resuming once the workspace scan completes") {
		t.Fatalf("the restore notice does not say what resumes:\n%s", lastAssistant(fresh.m))
	}
	fresh.m.isBooting = false
	var resumed tea.Cmd
	fresh.m, resumed = fresh.m.resumeRestoredWork()
	if resumed == nil {
		t.Fatal("the restored background job was not started again")
	}
	running := fresh.m.background.list()
	if len(running) != 1 || running[0].ID != "bg-keep" || running[0].FollowUp != "then summarize it" {
		t.Fatalf("resumed jobs = %+v", running)
	}
	if q := fresh.m.turn.queuedWork(); len(q) != 1 || q[0].input != "then tidy the logs" {
		t.Fatalf("resumed queue = %+v", q)
	}
	_, _ = fresh.m.turn.dequeue() // the queued turn runs and ends

	// 6. Verification: taken up at load; clean through the window; verified.
	fresh.m, _ = fresh.m.selfRepairStep()
	if a, _ := fresh.ledger(t).Get(sig); a.Status != problemlog.RepairVerifying {
		t.Fatalf("the new process did not take the fix up: %+v", a)
	}
	fresh.advance(time.Duration(config.DefaultSelfRepairVerifyWindowSec+1) * time.Second)
	fresh.m, _ = fresh.m.selfRepairStep()
	if a, _ := fresh.ledger(t).Get(sig); a.Status != problemlog.RepairVerified {
		t.Fatalf("after a clean window: %+v, want verified", a)
	}
	if !strings.Contains(lastAssistant(fresh.m), "Self-repair verified after the restart") {
		t.Fatalf("the verdict is not said: %q", lastAssistant(fresh.m))
	}
}

// A fix whose problem comes back in the process running it regressed: it
// counts as a failure, and the retry is told the earlier fix did not hold.
func TestSelfRepair_ARecurringProblemRegressesAndTheRetryIsToldSo(t *testing.T) {
	t.Parallel()
	r := newSelfRepairRig(t, "", "20261028_061954_problems.log", nilSessionError)
	sigs := []string{problemlog.Signature("nil session in executor dispatch: panic recovered")}
	l := r.ledger(t)
	if err := l.Begin(sigs, problemlog.LaneRepair, "job-1", nil, *r.now); err != nil {
		t.Fatal(err)
	}
	_ = l.Fixing("job-1", *r.now)
	if _, err := l.Installed("bbbb", *r.now); err != nil {
		t.Fatal(err)
	}

	// The new process takes it up before the problem is logged again.
	took := time.Date(2026, 11, 12, 9, 0, 0, 0, time.Local)
	fresh := newSelfRepairRig(t, r.m.workspace, "20261112_090000_problems.log",
		"2026/11/12 09:05:00.000 [ERROR] [session] nil session in executor dispatch: panic recovered\n")
	*fresh.now = took
	fresh.m, _ = fresh.m.selfRepairStep()
	a, _ := fresh.ledger(t).Get(sigs[0])
	if a.Status != problemlog.RepairRegressed || a.Failures != 1 || !strings.Contains(a.Note, "did not hold") {
		t.Fatalf("a recurring problem: %+v, want regressed with a note", a)
	}
	// Past its cooldown it is due again, and the diagnosis says what failed.
	fresh.advance(time.Duration(config.DefaultSelfRepairCooldownSec+1) * time.Second)
	fresh.m, _ = fresh.m.selfRepairStep()
	jobs := fresh.m.background.list()
	if len(jobs) != 1 || !strings.Contains(jobs[0].Task, "Do not repeat what they did") || !strings.Contains(jobs[0].Task, "did not hold") {
		t.Fatalf("the retry is not told the earlier fix failed: %+v", jobs)
	}
}

// A rebuilt binary that fails its self-check never replaces the running one:
// the fix turn's targets fail, the failure is an ERROR the problems log
// carries, and nothing is pending a restart.
func TestSelfRepair_ABrokenBuildNeverReplacesTheRunningBinary(t *testing.T) {
	t.Parallel()
	r := newSelfRepairRig(t, "", "20261028_061954_problems.log", nilSessionError)
	rebuild := &fakeRebuild{failure: errors.New("`nerd --help` failed: exit status 2")}
	rebuild.install(t, &r.m, nil)
	l := r.ledger(t)
	if err := l.Begin([]string{"sig"}, problemlog.LaneRepair, "job-1", nil, *r.now); err != nil {
		t.Fatal(err)
	}
	_ = l.Fixing("job-1", *r.now)

	err := r.m.rebuildWatchedExecutable(r.m.shutdownCtx, []string{"internal/session/executor.go"})
	if err == nil || !strings.Contains(err.Error(), "self-check") {
		t.Fatalf("rebuild err = %v, want the self-check failure", err)
	}
	if data, _ := os.ReadFile(rebuild.exe); string(data) != "old build" {
		t.Fatalf("a binary that failed its self-check replaced the running one: %q", data)
	}
	if _, pending := r.m.restarter.Pending(); pending {
		t.Fatal("a failed build is pending a restart")
	}
	if a, _ := l.Get("sig"); a.Status != problemlog.RepairFailed || a.Failures != 1 || !strings.Contains(a.Note, "self-check") {
		t.Fatalf("ledger after a failed build: %+v", a)
	}
}

// The default self-check runs the rebuilt binary: one that does not start is
// refused before go vet is reached.
func TestSelfCheckBinary_ABinaryThatDoesNotStartIsRefused(t *testing.T) {
	t.Parallel()
	m := NewTestModel()
	m.workspace = t.TempDir()
	bin := filepath.Join(t.TempDir(), "nerd")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\necho broken >&2\nexit 3\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	err := m.selfCheckBinary(context.Background(), m.workspace, bin, nil)
	if err == nil || !strings.Contains(err.Error(), "--help") || !strings.Contains(err.Error(), "broken") {
		t.Fatalf("err = %v, want the failed --help with its output", err)
	}
	ok := filepath.Join(t.TempDir(), "nerd")
	if err := os.WriteFile(ok, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := m.selfCheckBinary(context.Background(), m.workspace, ok, nil); err != nil {
		t.Fatalf("a binary that starts, with nothing else changed, failed: %v", err)
	}
}

func TestChangedSources_SplitsMangleFilesAndGoPackages(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	for _, p := range []string{"internal/a/a.go", "internal/a/b.go", "internal/a/a_test.go", "internal/core/defaults/policy/x.mg", "main.go"} {
		full := filepath.Join(ws, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mg, pkgs := changedSources(ws, []string{
		"internal/a/a.go", filepath.Join(ws, "internal/a/b.go"), "internal/a/a_test.go",
		"internal/core/defaults/policy/x.mg", "internal/gone/gone.go", "main.go", "../outside.go", "",
	})
	if strings.Join(mg, ",") != "internal/core/defaults/policy/x.mg" {
		t.Fatalf("mg = %v", mg)
	}
	if strings.Join(pkgs, ",") != "./.,./internal/a" {
		t.Fatalf("pkgs = %v", pkgs)
	}
}

// A self-repair job's settling moves its targets: a started fix turn to the
// fixing phase (marker on), a stop to stopped, a failure to failed.
func TestSelfRepair_SettlingAJobRecordsItsOutcome(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		err      error
		stopped  bool
		followUp bool
		want     problemlog.RepairStatus
		phase    string
	}{
		{"fix starts", nil, false, true, problemlog.RepairBeingFixed, problemlog.PhaseFixing},
		{"stopped", context.Canceled, true, false, problemlog.RepairStopped, ""},
		{"failed", errors.New("spawn refused"), false, false, problemlog.RepairFailed, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newSelfRepairRig(t, "", "20261028_061954_problems.log", nilSessionError)
			l := r.ledger(t)
			if err := l.Begin([]string{"sig"}, problemlog.LaneRepair, "job-1", nil, *r.now); err != nil {
				t.Fatal(err)
			}
			r.m.selfRepairSettled(backgroundJob{ID: "job-1", SelfRepair: problemlog.LaneRepair}, tc.err, tc.stopped, tc.followUp)
			a, _ := l.Get("sig")
			if a.Status != tc.want || a.Phase != tc.phase {
				t.Fatalf("%+v, want %s/%q", a, tc.want, tc.phase)
			}
			rows, err := r.m.kernel.Query(problemlog.RepairFixingPredicate)
			if err != nil {
				t.Fatal(err)
			}
			if (len(rows) > 0) != tc.followUp {
				t.Fatalf("self_repair_fixing rows = %d with a fix turn starting = %v", len(rows), tc.followUp)
			}
		})
	}
}

// With no hot problem left, the optimize lane takes the run's costliest
// recurring operation from this process's audit log, records its baseline
// mean, and asks for a diagnosis of where its time goes.
func TestSelfRepair_OptimizesTheCostliestOperationOnceNothingIsHot(t *testing.T) {
	t.Parallel()
	r := newSelfRepairRig(t, "", "20261112_090000_problems.log", "")
	settled := r.m.selfRepair.clock().UnixMilli()
	var b strings.Builder
	b.WriteString("# Audit log started\n")
	// The boot: a whole-world evaluation, costlier in all than anything the
	// steady state does, and not the steady state's cost to cut.
	for i := 0; i < config.DefaultSelfOptimizeMinCount+5; i++ {
		fmt.Fprintf(&b, `{"ts":%d,"event":"perf_metric","cat":"world","action":"BootRescan","dur_ms":5000}`+"\n", settled-60_000)
	}
	for i := 0; i < config.DefaultSelfOptimizeMinCount+5; i++ {
		fmt.Fprintf(&b, `{"ts":%d,"event":"perf_metric","cat":"kernel","action":"Evaluate","dur_ms":120}`+"\n", settled+1000)
		fmt.Fprintf(&b, `{"ts":%d,"event":"perf_metric","cat":"store","action":"Get","dur_ms":2}`+"\n", settled+1000)
		fmt.Fprintf(&b, `{"ts":%d,"event":"perf_slow","cat":"api","action":"Complete","dur_ms":9000}`+"\n", settled+1000)
	}
	if err := os.WriteFile(r.m.selfRepair.auditLog(), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	// Before the boot settles, nothing is the steady state's: no optimize job.
	if m, _ := r.m.selfRepairStep(); len(m.background.list()) != 0 {
		t.Fatalf("an optimize job started before the boot settled: %+v", m.background.list())
	}
	// The startup scan completes: the boot has settled.
	r.m.isBooting, r.m.bootStage = true, BootStageScanning
	r.m, _, _ = r.m.handleScanCompleteMsg(scanCompleteMsg{})
	m, cmd := r.m.selfRepairStep()
	jobs := m.background.list()
	if cmd == nil || len(jobs) != 1 || jobs[0].SelfRepair != problemlog.LaneOptimize {
		t.Fatalf("no optimize job: %+v", jobs)
	}
	if !strings.Contains(jobs[0].Task, "kernel.Evaluate") || strings.Contains(jobs[0].Task, "api.Complete") || !strings.Contains(jobs[0].FollowUp, "optimization") {
		t.Fatalf("the optimize job names the wrong operation or lacks its fix step:\n%s", jobs[0].Task)
	}
	a, ok := r.ledger(t).Get(problemlog.TimingTarget("kernel", "Evaluate"))
	if !ok || a.Lane != problemlog.LaneOptimize || a.BaselineMs != 120 {
		t.Fatalf("ledger = %+v, want the optimize attempt with its 120ms baseline", a)
	}
}

// The production chat carries self-repair, and Update routes its timer: a
// chat with debug-self on keeps ticking, one with it off stops.
func TestInitChat_WiresTheSelfRepairTimer(t *testing.T) {
	t.Parallel()
	m := InitChat(Config{})
	if m.selfRepair == nil {
		t.Fatal("InitChat builds no self-repair state")
	}
	m.Config = config.DefaultUserConfig()
	m.workspace = t.TempDir()
	m.selfRepair.audit = func() string { return "" }
	m.selfRepair.problems = func() string { return "" }
	if m.startSelfRepairTimer() != nil {
		t.Fatal("a timer started with debug-self off")
	}
	m.Config.Logging = &config.LoggingConfig{DebugSelf: true}
	if m.startSelfRepairTimer() == nil {
		t.Fatal("no timer with debug-self on")
	}
	next, cmd := m.Update(selfRepairTickMsg{gen: m.selfRepair.gen})
	if cmd == nil {
		t.Fatal("Update did not keep the self-repair timer going")
	}
	off := next.(Model)
	off.Config.Logging.DebugSelf = false
	if _, cmd := off.Update(selfRepairTickMsg{gen: off.selfRepair.gen}); cmd != nil {
		t.Fatal("the timer kept ticking with debug-self off")
	}
}

// /debug-self status names the self-repair ledger.
func TestDebugSelfStatus_NamesSelfRepair(t *testing.T) {
	t.Parallel()
	r := newSelfRepairRig(t, "", "20261028_061954_problems.log", "")
	if s := debugSelfStatusText(r.m); !strings.Contains(s, "Self-repair: on") || !strings.Contains(s, "ledger.json") {
		t.Fatalf("status = %q", s)
	}
	r.m.Config.Logging.SelfRepairDisabled = true
	if s := debugSelfStatusText(r.m); !strings.Contains(s, "Self-repair: off") {
		t.Fatalf("status = %q", s)
	}
}
