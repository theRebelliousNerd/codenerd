package chat

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/problemlog"
	"codenerd/internal/session"
	"codenerd/internal/turnjudge"
)

// Self-repair: the autonomous stage of debug-self (policy/self_repair.mg).
//
// While logging.debug_self is on, the chat asks the kernel at every idle
// moment -- when a turn or a background job settles, and on a timer while
// nothing happens -- whether a self-repair is due. The kernel decides: which
// hot problems (self_problem_hot) are due, or, with none left, which recurring
// operation is the costliest (self_timing), and by which persona. The chat
// only executes: it starts the self-diagnosis chain (self_diagnosis.go) for
// the due targets as a background job -- never while a turn of the user's
// runs, is queued or awaits approval -- records each attempt's lifecycle in
// the ledger under .nerd/self_repair (problemlog.RepairLedger), and adopts the
// kernel's verdicts once the process runs the fix.
//
// The chain: a read-only diagnosis in the background; its follow-up is the
// fix turn, under the self_repair_fixing() marker that makes the safety gate
// and the permission logic project_forbidden_path; a verified turn that
// changed source rebuilds the binary, self-checks it and installs it
// (self_restart.go), the attempts wait for the restart; the restart resumes
// the session and its work; the new process takes the attempts up for
// verification, and the kernel says whether the problem stayed away.

// selfRepairState is the self-repair bookkeeping, shared across Model copies.
type selfRepairState struct {
	mu      sync.Mutex
	ledger  *problemlog.RepairLedger
	loadErr error
	timing  *problemlog.TimingWatcher
	// fixing mirrors the kernel's self_repair_fixing() marker.
	fixing bool
	// fixTurn is the execution running the latest fix turn (runChainStep).
	fixTurn uint64
	// journal records every write of the open fix chain, of journalJob;
	// settleFixTurn undoes it unless the fix was installed.
	journal    *session.WriteJournal
	journalJob string
	// overBudgetJob is the fix chain the kernel derived as over budget and
	// the chat stopped (self_repair_budget.go); settleFixTurn records it so.
	overBudgetJob string
	// overBudgetKinds says which budgets that chain ran past ("time",
	// "tokens"), as the kernel derived them.
	overBudgetKinds []string
	// fixStartTokens is the session's token total when the open fix chain
	// began; what the chain has spent is the total now less this.
	fixStartTokens int64
	// gen numbers the timer loop; a tick of an older loop stops.
	gen uint64
	// planning is set while a look's plan is made off the Update thread
	// (planSelfRepair) and cleared when it lands: never two at once.
	planning bool
	// now is the clock, audit names this process's audit log (the timings'
	// source) and problems its problems log; tests pin all three.
	now      func() time.Time
	audit    func() string
	problems func() string
}

func newSelfRepairState() *selfRepairState {
	return &selfRepairState{timing: problemlog.NewTimingWatcher(), now: time.Now, audit: logging.AuditPath, problems: currentProblemsLog}
}

func currentProblemsLog() string {
	path, _, _ := logging.ProblemsSnapshot()
	return path
}

func (s *selfRepairState) auditLog() string {
	if s.audit == nil {
		return logging.AuditPath()
	}
	return s.audit()
}

func (s *selfRepairState) problemsLog() string {
	if s.problems == nil {
		return currentProblemsLog()
	}
	return s.problems()
}

func (s *selfRepairState) clock() time.Time {
	if s.now == nil {
		return time.Now()
	}
	return s.now()
}

// ledgerFor loads the workspace's ledger once per process. Loading it is
// what takes a fix the previous process installed up for verification here.
func (s *selfRepairState) ledgerFor(workspace string) (*problemlog.RepairLedger, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ledger == nil && s.loadErr == nil {
		s.ledger, s.loadErr = problemlog.LoadRepairLedger(problemlog.RepairLedgerPath(workspace), s.clock())
	}
	return s.ledger, s.loadErr
}

// selfRepairTickMsg is the idle timer; gen names its loop.
type selfRepairTickMsg struct{ gen uint64 }

// selfRepairCheckMsg asks once, at a turn boundary.
type selfRepairCheckMsg struct{}

// startSelfRepairTimer starts a new timer loop (an older one stops at its
// next tick). Nil when debug-self is off.
func (m Model) startSelfRepairTimer() tea.Cmd {
	if m.selfRepair == nil || !m.debugSelfEnabled() {
		return nil
	}
	m.selfRepair.mu.Lock()
	m.selfRepair.gen++
	gen := m.selfRepair.gen
	m.selfRepair.mu.Unlock()
	return m.selfRepairTick(gen)
}

func (m Model) selfRepairTick(gen uint64) tea.Cmd {
	return tea.Tick(m.Config.Logging.SelfRepairPoll(), func(time.Time) tea.Msg { return selfRepairTickMsg{gen: gen} })
}

// handleSelfRepairTick looks at what is due and keeps the loop going while
// debug-self is on.
func (m Model) handleSelfRepairTick(msg selfRepairTickMsg) (Model, tea.Cmd) {
	if m.selfRepair == nil || !m.debugSelfEnabled() {
		return m, nil
	}
	m.selfRepair.mu.Lock()
	current := msg.gen == m.selfRepair.gen
	m.selfRepair.mu.Unlock()
	if !current {
		return m, nil
	}
	m, cmd := m.selfRepairStep()
	return m, tea.Batch(cmd, m.checkFixBudgetCmd(), m.selfRepairTick(msg.gen))
}

// selfRepairBusy reports anything of the user's, or of a restart, that a
// self-repair must not start beside: a turn running or queued, the boot, an
// approval being decided, held work waiting for go, an offered step, a
// /recurse or campaign run, a restart on its way, or restored work not yet
// resumed. The user's own turn always comes first.
func (m Model) selfRepairBusy() bool {
	return m.isLoading || m.isBooting || m.viewMode == ApprovalView ||
		m.turn.current() != 0 || m.turn.queuedCount() > 0 ||
		m.pendingWork != nil || m.offeredSubtask != nil ||
		m.recurse != nil || m.campaignOrch != nil ||
		m.hasPendingRestart() || len(m.resumeWork) > 0
}

// selfRepairStep is one idle look, phase A of three (self_repair_plan.go has
// B and C): on the Update thread it only settles what ended -- the fix turn
// this process ran, a fixing phase whose turn is gone, a diagnosis whose job
// was lost -- then hands the rest to a command. Refreshing the evidence,
// asking the kernel for verdicts and what is due, and building the job
// (whose triage is a Jev call) run off the Update thread (planSelfRepair);
// what they found lands as a selfRepairPlanMsg (landSelfRepairPlan). A look
// that finds the last one's plan still being made does nothing.
func (m Model) selfRepairStep() (Model, tea.Cmd) {
	if m.selfRepair == nil || m.kernel == nil || !m.debugSelfEnabled() || m.selfRepairBusy() {
		return m, nil
	}
	if m.selfRepair.planInFlight() {
		return m, nil
	}
	log := logging.Get(logging.CategoryKernel)
	ledger, err := m.selfRepair.ledgerFor(m.workspace)
	if err != nil {
		log.Error("%s self-repair off, its ledger is unreadable: %v", selfDebugMarker, err)
		return m, nil
	}
	now := m.selfRepair.clock()

	// Idle means every fix turn has ended. The one this process ran settles
	// with its own outcome and its edits undone unless it was installed
	// (settleFixTurn; normally already settled at its turn boundary). One
	// still in its fixing phase with no fix turn here -- its process exited,
	// or it never started -- did not fix anything.
	m = m.settleFixTurn()
	if n, err := ledger.EndFixing(problemlog.RepairFailed, "the fix turn ended without a verified, rebuilt and self-checked binary", now); err != nil {
		log.Error("%s self-repair ledger: %v", selfDebugMarker, err)
	} else if n > 0 {
		m = m.pushAssistantMsg(fmt.Sprintf("_Self-repair: the fix turn for %d target(s) ended without installing a fix; they are recorded as failed._", n))
	}
	m.syncSelfRepairFixing(ledger)
	// A diagnosis whose job is gone (the process that ran it exited without a
	// restart that carried it) did not finish.
	running := map[string]bool{}
	for _, job := range m.background.list() {
		running[job.ID] = true
	}
	for _, a := range ledger.Attempts() {
		if a.Status == problemlog.RepairBeingFixed && a.Phase == problemlog.PhaseDiagnosing && !running[a.Job] {
			if _, err := ledger.EndJob(a.Job, problemlog.RepairFailed, "its diagnosis job was lost: codeNERD exited while it ran", now); err != nil {
				log.Error("%s self-repair ledger: %v", selfDebugMarker, err)
			}
		}
	}
	return m, m.planSelfRepairCmd(ledger, now)
}

func selfRepairSubject(lane string, targets []string) string {
	if lane == problemlog.LaneOptimize {
		return "codeNERD's costliest recurring operation, " + strings.TrimPrefix(strings.Join(targets, ", "), problemlog.TimingTargetPrefix)
	}
	return fmt.Sprintf("%d of codeNERD's own hot problem(s)", len(targets))
}

func selfRepairTargetLabel(t string) string {
	return "`" + truncateSelfSignature(t) + "`"
}

// selfRepairOff is /debug-self off: the timer stops at its next tick, and a
// fix turn that is no longer running is settled as stopped so its marker does
// not outlive it. One still running keeps its marker until it ends.
func (m Model) selfRepairOff() {
	if m.selfRepair == nil {
		return
	}
	m.selfRepair.mu.Lock()
	m.selfRepair.gen++
	m.selfRepair.mu.Unlock()
	if m.selfRepairBusy() {
		return
	}
	ledger, err := m.selfRepair.ledgerFor(m.workspace)
	if err != nil {
		return
	}
	if _, err := ledger.EndFixing(problemlog.RepairStopped, "debug-self was turned off", m.selfRepair.clock()); err != nil {
		logging.Get(logging.CategoryKernel).Error("%s self-repair ledger: %v", selfDebugMarker, err)
	}
	m.syncSelfRepairFixing(ledger)
}

// syncSelfRepairFixing keeps the kernel's self_repair_fixing() marker equal
// to the ledger: asserted while a fix turn runs.
func (m Model) syncSelfRepairFixing(ledger *problemlog.RepairLedger) {
	if m.selfRepair == nil || m.kernel == nil || ledger == nil {
		return
	}
	want := len(ledger.InFixing()) > 0
	m.selfRepair.mu.Lock()
	have := m.selfRepair.fixing
	m.selfRepair.fixing = want
	m.selfRepair.mu.Unlock()
	if want == have {
		return
	}
	log := logging.Get(logging.CategoryKernel)
	if want {
		if err := m.kernel.Assert(core.Fact{Predicate: problemlog.RepairFixingPredicate}); err != nil {
			log.Error("%s assert %s: %v", selfDebugMarker, problemlog.RepairFixingPredicate, err)
		}
		return
	}
	if err := m.kernel.Retract(problemlog.RepairFixingPredicate); err != nil {
		log.Error("%s retract %s: %v", selfDebugMarker, problemlog.RepairFixingPredicate, err)
	}
}

// selfRepairSettled records how a self-repair job ended (finishBackground):
// a fix turn that starts takes its targets to the fixing phase, under the
// marker; anything else ends the attempt.
func (m Model) selfRepairSettled(job backgroundJob, err error, stopped, followUpStarts bool) {
	if job.SelfRepair == "" || m.selfRepair == nil {
		return
	}
	ledger, lerr := m.selfRepair.ledgerFor(m.workspace)
	if lerr != nil {
		return
	}
	now := m.selfRepair.clock()
	var werr error
	switch {
	case stopped:
		_, werr = ledger.EndJob(job.ID, problemlog.RepairStopped, "stopped by the user", now)
	case err != nil:
		_, werr = ledger.EndJob(job.ID, problemlog.RepairFailed, "its diagnosis failed: "+oneLine(err.Error()), now)
	case followUpStarts:
		werr = ledger.Fixing(job.ID, now)
	default:
		_, werr = ledger.EndJob(job.ID, problemlog.RepairStopped, "its fix was not started: its session was not open when the diagnosis finished", now)
	}
	if werr != nil {
		logging.Get(logging.CategoryKernel).Error("%s self-repair ledger: %v", selfDebugMarker, werr)
	}
	m.syncSelfRepairFixing(ledger)
	if m.kernel != nil {
		if err := problemlog.ReplaceRepair(m.kernel, ledger, now); err != nil {
			logging.Get(logging.CategoryKernel).Warn("%s self-repair facts: %v", selfDebugMarker, err)
		}
	}
}

// selfRepairRebuilt records a rebuild that ran while a fix turn ran: an
// installed binary sends the turn's targets to wait for the restart; a build
// or self-check failure fails them. Called from the command goroutine that
// rebuilt (rebuildWatchedExecutable); the ledger and kernel are safe for it.
func (m Model) selfRepairRebuilt(build string, buildErr error) {
	if m.selfRepair == nil {
		return
	}
	ledger, err := m.selfRepair.ledgerFor(m.workspace)
	if err != nil || len(ledger.InFixing()) == 0 {
		return
	}
	now := m.selfRepair.clock()
	if buildErr != nil {
		_, err = ledger.EndFixing(problemlog.RepairFailed, "its rebuilt binary did not pass the self-check: "+oneLine(buildErr.Error()), now)
	} else {
		_, err = ledger.Installed(build, now)
	}
	if err != nil {
		logging.Get(logging.CategoryKernel).Error("%s self-repair ledger: %v", selfDebugMarker, err)
	}
	m.syncSelfRepairFixing(ledger)
	if m.kernel != nil {
		if err := problemlog.ReplaceRepair(m.kernel, ledger, now); err != nil {
			logging.Get(logging.CategoryKernel).Warn("%s self-repair facts: %v", selfDebugMarker, err)
		}
	}
}

// =============================================================================
// THE JOB
// =============================================================================

// selfRepairFollowUpGuard closes every self-repair fix turn: the refusal is
// enforced (self_repair.mg), and saying it spares the turn the refused calls.
const selfRepairFollowUpGuard = "This is codeNERD repairing itself unasked: never change its safety gate, permission or approval logic, or the self-repair trigger; those files are refused."

// selfOptimizeFollowUp opens the optimize lane's fix step.
const selfOptimizeFollowUp = "Make the optimization this analysis of codeNERD's costliest recurring operation found, in codeNERD's own source, with a test or benchmark that proves the behavior is unchanged. If it found the time is spent outside codeNERD's code (a model, the network, the disk), change nothing and say so in a sentence."

// selfRepairJob builds the background job for the kernel's due targets: the
// repair lane's hot problems (or else the optimize lane's operation), each
// told what earlier attempts on it came to.
func (m Model) selfRepairJob(ctx context.Context, due []problemlog.Due, ledger *problemlog.RepairLedger, now time.Time) (backgroundJob, bool, string) {
	shard, err := problemlog.RepairShard(m.kernel)
	if err != nil {
		return backgroundJob{}, false, err.Error()
	}
	if shard == "" {
		return backgroundJob{}, false, "the kernel names no persona for a self-diagnosis (self_repair_shard)"
	}
	lane := due[0].Lane
	var targets []string
	for _, d := range due {
		if d.Lane == lane {
			targets = append(targets, d.Target)
		}
	}
	job := backgroundJob{
		ID:                fmt.Sprintf("self-repair-%d", now.UnixNano()),
		Shard:             shard,
		Started:           now,
		SelfRepair:        lane,
		selfRepairTargets: targets,
	}
	switch lane {
	case problemlog.LaneRepair:
		task, problems, reason := m.selfRepairTask(ctx, targets, ledger)
		if task == "" {
			return backgroundJob{}, false, reason
		}
		job.Task, job.diagnosed = task, diagnosisTargetsOf(problems)
		job.selfRepairTargets = nil
		for _, p := range problems {
			job.selfRepairTargets = append(job.selfRepairTargets, p.Signature)
		}
		sort.Strings(job.selfRepairTargets)
		job.FollowUp = selfDiagnosisFollowUp + " " + selfRepairFollowUpGuard
	case problemlog.LaneOptimize:
		if len(targets) > 1 {
			targets = targets[:1] // tied totals: one operation per job, the first by name
		}
		tm, ok := m.selfRepair.timing.Lookup(targets[0])
		if !ok {
			return backgroundJob{}, false, "the due operation " + targets[0] + " has no timing in this run"
		}
		job.selfRepairTargets = targets
		job.diagnosed = []turnjudge.DiagnosisTarget{{Key: targets[0]}}
		job.Task = selfOptimizeTask(tm, ledger)
		job.FollowUp = selfOptimizeFollowUp + " " + selfRepairFollowUpGuard
	default:
		return backgroundJob{}, false, "unknown self-repair lane " + lane
	}
	return job, true, ""
}

// selfRepairTask is the diagnosis of the due problems, read whole from the
// problems log, defects before failures around codeNERD (triageProblems),
// and the problems it covers in that order.
func (m Model) selfRepairTask(ctx context.Context, targets []string, ledger *problemlog.RepairLedger) (task string, covered []problemlog.Problem, reason string) {
	path := m.latestProblemsLog()
	if path == "" {
		return "", nil, "no problems log to read the due problems from"
	}
	rep, err := problemlog.ReadFile(path, problemlog.Options{Skip: isSelfNote})
	if err != nil {
		return "", nil, err.Error()
	}
	want := map[string]bool{}
	for _, t := range targets {
		want[t] = true
	}
	var problems []problemlog.Problem
	var kept []string
	for _, p := range rep.Problems {
		if want[p.Signature] {
			problems = append(problems, p)
			kept = append(kept, p.Signature)
			delete(want, p.Signature) // one per signature, the most urgent
		}
	}
	if len(problems) == 0 {
		return "", nil, "the due problems are no longer in " + path
	}
	defects, around := m.triageProblems(ctx, problems)
	var b strings.Builder
	fmt.Fprintf(&b, "Diagnose codeNERD itself from its own problems log (%s; %s). Read-only: change nothing. codeNERD started this itself (debug-self self-repair); no one asked.\n", path, rep.Summary())
	b.WriteString("For each problem below: find where codeNERD's source raises it, trace the root cause, and give file:line, the evidence, and the fix you would make. Say when two problems share a cause.\n")
	n := writeDiagnosisProblems(&b, defects, 0)
	if len(around) > 0 {
		b.WriteString("\nThese look like failures around codeNERD (network, providers, configuration, the machine) rather than defects in it. Confirm that briefly; fix codeNERD only where it handles them wrongly:\n")
		writeDiagnosisProblems(&b, around, n)
	}
	writeSelfRepairHistory(&b, kept, ledger)
	return b.String(), append(append([]problemlog.Problem(nil), defects...), around...), ""
}

// selfOptimizeTask is the diagnosis of the costliest recurring operation.
func selfOptimizeTask(tm problemlog.Timing, ledger *problemlog.RepairLedger) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Find why %s.%s is the costliest recurring operation of this codeNERD run once its boot settled: timed %d times since, %dms in all, %dms on average (over the whole run, boot included: %d times, %dms on average, %dms at most; the run's audit log, perf_metric). Read-only: change nothing. codeNERD started this itself (debug-self self-repair); no one asked.\n",
		tm.System, tm.Operation, tm.SettledCount, tm.SettledTotalMs, tm.SettledMeanMs(), tm.Count, tm.MeanMs(), tm.MaxMs)
	fmt.Fprintf(&b, "Find where codeNERD's source times it (logging.StartTimer with the %q category and the %q operation), read what its time goes to, and say whether it is spent in codeNERD's own code -- then give the change that would cut it, with file:line and why behavior stays the same -- or waiting on something outside (a model, the network, the disk) that codeNERD cannot speed up.\n", tm.System, tm.Operation)
	writeSelfRepairHistory(&b, []string{tm.Target()}, ledger)
	return b.String()
}

// writeSelfRepairHistory tells a retry what the earlier attempts on its
// targets came to, so a fix that did not hold is not made the same way again.
func writeSelfRepairHistory(b *strings.Builder, targets []string, ledger *problemlog.RepairLedger) {
	first := true
	for _, t := range targets {
		a, ok := ledger.Get(t)
		if !ok || a.Tries == 0 {
			continue
		}
		if first {
			b.WriteString("\nEarlier self-repair attempts on these targets did not settle them. Do not repeat what they did; find what they missed:\n")
			first = false
		}
		fmt.Fprintf(b, "- %s: %d attempt(s), %d failed; the last was %s", truncateSelfSignature(t), a.Tries, a.Failures, a.Status)
		if a.Note != "" {
			fmt.Fprintf(b, " (%s)", a.Note)
		}
		b.WriteString("\n")
		if a.AttemptDiff != "" {
			fmt.Fprintf(b, "  What it changed was undone and is saved at %s: read it, and do not make the same change again.\n", a.AttemptDiff)
		}
		if len(a.FailedTests) > 0 {
			b.WriteString("  Tests failing when it ended:\n")
			for _, f := range a.FailedTests {
				fmt.Fprintf(b, "    - %s\n", f)
			}
		}
	}
}

// selfRepairStatusText is the /debug-self status line about self-repair.
func selfRepairStatusText(m Model) string {
	if m.Config.Logging != nil && m.Config.Logging.SelfRepairDisabled {
		return "Self-repair: off (logging.self_repair_disabled)."
	}
	if m.selfRepair == nil {
		return "Self-repair: not wired in this session."
	}
	ledger, err := m.selfRepair.ledgerFor(m.workspace)
	if err != nil {
		return fmt.Sprintf("Self-repair: off, its ledger is unreadable: %v.", err)
	}
	counts := map[problemlog.RepairStatus]int{}
	for _, a := range ledger.Attempts() {
		counts[a.Status]++
	}
	return fmt.Sprintf("Self-repair: on (every %s while idle). Ledger %s: %d being fixed, %d awaiting restart, %d verifying, %d verified, %d regressed, %d failed, %d over budget, %d stopped, %d no defect (held until their evidence grows %dx).",
		m.Config.Logging.SelfRepairPoll(), problemlog.RepairLedgerPath(m.workspace),
		counts[problemlog.RepairBeingFixed], counts[problemlog.RepairPendingRestart], counts[problemlog.RepairVerifying],
		counts[problemlog.RepairVerified], counts[problemlog.RepairRegressed], counts[problemlog.RepairFailed], counts[problemlog.RepairOverBudget], counts[problemlog.RepairStopped],
		counts[problemlog.RepairNoDefect], m.Config.Logging.SelfRepairNoDefectRegrowthFactor())
}
