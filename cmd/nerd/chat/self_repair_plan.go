package chat

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"codenerd/internal/logging"
	"codenerd/internal/problemlog"
)

// A self-repair look runs in three phases so the UI never waits on it:
//
//	A  (Update thread, selfRepairStep): the guards, settling the fix turn
//	   that ended, ending attempts whose turn or job is gone.
//	B  (a command, planSelfRepair): following the problems log and the audit
//	   log into the kernel, the kernel's verdicts and due targets, and the job
//	   for them -- the Jev triage included. It reads a snapshot of the Model
//	   and records nothing in the ledger.
//	C  (Update thread, landSelfRepairPlan): if the session is still idle,
//	   adopt the verdicts or start the job; if the user began something
//	   while B ran, the plan is dropped and the next idle look makes it again.
//
// Live 2026-11-12: the synchronous look took 238ms and 523ms on the Update
// thread (performance log, "Slow Update() handler: chat.selfRepairCheckMsg").

// selfRepairPlanMsg is what phase B found, at the clock now it was found at:
// the verdicts on the fixes this process runs, or else (with none to adopt)
// the job for what the kernel says is due, or the reason none was built.
type selfRepairPlanMsg struct {
	now time.Time
	// verified and regressed are the kernel's verdicts on attempts the ledger
	// still has verifying. With any, nothing else was planned: what is due is
	// asked again once they are adopted, as the kernel then sees them.
	verified, regressed []string
	// due counts the kernel's due targets; job is built for them when ok, and
	// reason says why not otherwise.
	due    int
	job    backgroundJob
	ok     bool
	reason string
	// baselines is the optimize lane's settled mean per target, when B built
	// an optimize job.
	baselines map[string]int64
	// finish is a parked candidate's job the kernel derived due to be
	// finished (self_repair_finish_due); with it, nothing else was planned.
	finish string
}

// planInFlight reports a plan being made.
func (s *selfRepairState) planInFlight() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.planning
}

// startPlanning claims the one plan slot; false when a plan is already being
// made.
func (s *selfRepairState) startPlanning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.planning {
		return false
	}
	s.planning = true
	return true
}

func (s *selfRepairState) endPlanning() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.planning = false
}

// planSelfRepairCmd is phase B's command, or nil when a plan is already being
// made. It is cancelled with the session (shutdownCtx).
func (m Model) planSelfRepairCmd(ledger *problemlog.RepairLedger, now time.Time) tea.Cmd {
	if !m.selfRepair.startPlanning() {
		return nil
	}
	ctx := m.shutdownCtx
	if ctx == nil {
		ctx = context.Background()
	}
	planner := m.selfRepairPlanner()
	return func() tea.Msg {
		return planner.planSelfRepair(ctx, ledger, now)
	}
}

// selfRepairPlanner is the Model phase B reads: this one, with its own copy
// of the config sections B reads (logging, typesafe), so a /debug-self or
// /config change on the Update thread while B runs is no data race. The rest
// B touches is safe for it: the kernel, the ledger, the problems watcher, the
// timing watcher and the Jev client lock themselves.
func (m Model) selfRepairPlanner() Model {
	if m.Config == nil {
		return m
	}
	cfg := *m.Config
	if cfg.Logging != nil {
		l := *cfg.Logging
		cfg.Logging = &l
	}
	if cfg.TypeSafe != nil {
		ts := *cfg.TypeSafe
		cfg.TypeSafe = &ts
	}
	m.Config = &cfg
	return m
}

// planSelfRepair is phase B: refresh the evidence, ask the kernel for its
// verdicts and what is due, and build the job. It changes no Model field and
// records nothing in the ledger; the kernel facts it replaces are the
// evidence (problems, timings, the ledger's projection at now) and the Jev
// triage's, which every look replaces again.
func (m Model) planSelfRepair(ctx context.Context, ledger *problemlog.RepairLedger, now time.Time) selfRepairPlanMsg {
	plan := selfRepairPlanMsg{now: now}
	log := logging.Get(logging.CategoryKernel)
	if ctx.Err() != nil {
		return plan
	}
	if err := m.syncSelfRepairEvidence(ledger, now); err != nil {
		log.Warn("%s self-repair evidence: %v", selfDebugMarker, err)
		return plan
	}
	verified, regressed, err := problemlog.Verdicts(m.kernel)
	if err != nil {
		log.Warn("%s self-repair verdicts: %v", selfDebugMarker, err)
	}
	plan.verified, plan.regressed = verifying(ledger, verified), verifying(ledger, regressed)
	if len(plan.verified)+len(plan.regressed) > 0 {
		return plan
	}
	if ctx.Err() != nil {
		return plan
	}
	// A parked candidate is finished before a new target is chosen; the
	// kernel says when (self_repair_finish_due).
	finish, err := problemlog.FinishDue(m.kernel)
	if err != nil {
		log.Warn("%s self-repair: %v", selfDebugMarker, err)
		return plan
	}
	if len(finish) > 0 {
		plan.finish = finish[0]
		return plan
	}
	due, err := problemlog.DueTargets(m.kernel)
	if err != nil {
		log.Warn("%s self-repair: %v", selfDebugMarker, err)
		return plan
	}
	plan.due = len(due)
	if len(due) == 0 {
		return plan
	}
	plan.job, plan.ok, plan.reason = m.selfRepairJob(ctx, due, ledger, now)
	if plan.ok && ctx.Err() != nil {
		plan.ok, plan.reason = false, "the session is shutting down"
	}
	if plan.ok && plan.job.SelfRepair == problemlog.LaneOptimize {
		plan.baselines = map[string]int64{}
		for _, t := range plan.job.selfRepairTargets {
			if tm, ok := m.selfRepair.timing.Lookup(t); ok {
				// The steady state's mean, the one the lane ranked and the
				// verdict compares (self_timing_settled).
				plan.baselines[t] = tm.SettledMeanMs()
			}
		}
	}
	return plan
}

// verifying keeps the targets the ledger has verifying: the only ones a
// verdict settles (RepairLedger.Verdict).
func verifying(ledger *problemlog.RepairLedger, targets []string) []string {
	var out []string
	for _, t := range targets {
		if a, ok := ledger.Get(t); ok && a.Status == problemlog.RepairVerifying {
			out = append(out, t)
		}
	}
	return out
}

// syncSelfRepairEvidence puts the run's timings and the ledger into the
// kernel beside the problems log (syncSelfProblemFacts).
func (m Model) syncSelfRepairEvidence(ledger *problemlog.RepairLedger, now time.Time) error {
	// Follow the problems log as a turn would, so a session nobody has typed
	// in yet still sees its own problems. The drained lines stay pending:
	// the next turn's digest still reports them.
	if m.selfDebug != nil {
		m.selfDebug.collect(m.selfRepair.problemsLog())
	}
	m.syncSelfProblemFacts()
	// This process's own audit log only: a fix is judged by the timings of
	// the process running it, never by an earlier run's.
	if err := m.selfRepair.timing.Read(m.selfRepair.auditLog()); err != nil {
		return err
	}
	if m.selfRepair.timing.Changed() {
		if err := problemlog.ReplaceTimings(m.kernel, m.selfRepair.timing.Timings()); err != nil {
			return err
		}
	}
	return problemlog.ReplaceRepair(m.kernel, ledger, now)
}

// selfRepairCheck asks for one more look at once (selfRepairCheckMsg).
func selfRepairCheck() tea.Msg { return selfRepairCheckMsg{} }

// landSelfRepairPlan is phase C. The session must still be idle: the user's
// turn, typed while B ran, comes first, and the plan -- nothing of which was
// recorded -- is dropped. Otherwise it adopts the verdicts, then looks again
// for what is due as the kernel now sees it; or it records and starts the job.
func (m Model) landSelfRepairPlan(plan selfRepairPlanMsg) (Model, tea.Cmd) {
	if m.selfRepair == nil {
		return m, nil
	}
	m.selfRepair.endPlanning()
	log := logging.Get(logging.CategoryKernel)
	if m.kernel == nil || !m.debugSelfEnabled() || m.selfRepairBusy() {
		if plan.ok || len(plan.verified)+len(plan.regressed) > 0 {
			log.Info("%s self-repair plan dropped: the session stopped being idle while it was made; the next idle look makes it again", selfDebugMarker)
		}
		return m, nil
	}
	ledger, err := m.selfRepair.ledgerFor(m.workspace)
	if err != nil {
		log.Error("%s self-repair off, its ledger is unreadable: %v", selfDebugMarker, err)
		return m, nil
	}
	if len(plan.verified)+len(plan.regressed) > 0 {
		m = m.adoptSelfRepairVerdicts(ledger, plan.verified, plan.regressed, plan.now)
		return m, selfRepairCheck
	}
	if plan.finish != "" {
		return m.finishParkedCandidate(ledger, plan.finish, plan.now)
	}
	if plan.due == 0 {
		return m, nil
	}
	if !plan.ok {
		log.Info("%s self-repair due but not started: %s", selfDebugMarker, plan.reason)
		return m, nil
	}
	job := plan.job
	targets := job.selfRepairTargets
	if err := ledger.Begin(targets, job.SelfRepair, job.ID, plan.baselines, plan.now); err != nil {
		log.Error("%s self-repair not started, its ledger cannot record it: %v", selfDebugMarker, err)
		return m, nil
	}
	if err := problemlog.ReplaceRepair(m.kernel, ledger, plan.now); err != nil {
		log.Warn("%s self-repair facts: %v", selfDebugMarker, err)
	}
	m = m.pushAssistantMsg(fmt.Sprintf("_Self-repair (debug-self): the %s shard is diagnosing %s in the background. Its fix follows as a turn of its own; your messages go first._",
		job.Shard, selfRepairSubject(job.SelfRepair, targets)))
	return m, m.startBackground(job, job.Task)
}

// adoptSelfRepairVerdicts records the kernel's verdicts on the fixes this
// process runs, and says them.
func (m Model) adoptSelfRepairVerdicts(ledger *problemlog.RepairLedger, verified, regressed []string, now time.Time) Model {
	changed := false
	for _, t := range verified {
		if ok, err := ledger.Verdict(t, problemlog.RepairVerified, "", now); err == nil && ok {
			changed = true
			m = m.pushAssistantMsg(fmt.Sprintf("_Self-repair verified after the restart: %s._", selfRepairTargetLabel(t)))
		}
	}
	for _, t := range regressed {
		a, _ := ledger.Get(t)
		note := fmt.Sprintf("the fix installed as build %s did not hold: the problem came back in the process running it", a.Build)
		if a.Lane == problemlog.LaneOptimize {
			note = fmt.Sprintf("the optimization installed as build %s did not make it faster than its %dms mean", a.Build, a.BaselineMs)
		}
		if ok, err := ledger.Verdict(t, problemlog.RepairRegressed, note, now); err == nil && ok {
			changed = true
			m = m.pushAssistantMsg(fmt.Sprintf("_Self-repair regressed: %s; %s. A retry is told so and must find another cause._", selfRepairTargetLabel(t), note))
		}
	}
	if changed {
		if err := problemlog.ReplaceRepair(m.kernel, ledger, now); err != nil {
			logging.Get(logging.CategoryKernel).Warn("%s self-repair facts: %v", selfDebugMarker, err)
		}
	}
	return m
}
