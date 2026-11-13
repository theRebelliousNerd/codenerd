package chat

import (
	"context"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"codenerd/internal/config"
	"codenerd/internal/problemlog"
	"codenerd/internal/typesafe"
)

// stepSelfRepair runs one self-repair look as the program does: phase A on
// the Update thread (selfRepairStep), phase B's command, and its plan landed
// back through Update (phase C). It returns what C returned: the job's
// command when one started.
func stepSelfRepair(t *testing.T, m Model) (Model, tea.Cmd) {
	t.Helper()
	return landPlan(m.selfRepairStep())
}

// landPlan runs the plan command in cmd -- a look's, or the first of a
// timer tick's batch -- and feeds the plan through Update. With no plan in
// cmd, the model comes back unchanged with no command.
func landPlan(m Model, cmd tea.Cmd) (Model, tea.Cmd) {
	plan, ok := planIn(cmd)
	if !ok {
		return m, nil
	}
	next, out := m.Update(plan)
	return next.(Model), out
}

func planIn(cmd tea.Cmd) (selfRepairPlanMsg, bool) {
	if cmd == nil {
		return selfRepairPlanMsg{}, false
	}
	switch msg := cmd().(type) {
	case selfRepairPlanMsg:
		return msg, true
	case tea.BatchMsg:
		// The plan comes first in a tick's batch; the timer after it is
		// never run here.
		for _, c := range msg {
			if c == nil {
				continue
			}
			if plan, ok := c().(selfRepairPlanMsg); ok {
				return plan, true
			}
		}
	}
	return selfRepairPlanMsg{}, false
}

// slowJev answers every question after a delay, honoring its context, and
// counts the calls: the triage of a self-repair look is a Jev network call.
type slowJev struct {
	verdictJev
	delay time.Duration
	calls sync.WaitGroup
}

func (s *slowJev) Ask(ctx context.Context, state any, qs map[string]typesafe.Question) (*typesafe.Response, error) {
	select {
	case <-time.After(s.delay):
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return s.verdictJev.Ask(ctx, state, qs)
}

func (s *slowJev) askedCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.asked
}

// withTriage turns the self-repair triage on, through jev.
func withTriage(r *selfRepairRig, jev *slowJev) {
	r.m.jev = jev
	on, off := true, false
	r.m.Config.TypeSafe = &config.TypeSafeConfig{DiagnosisTriage: &on, DiagnosisVerdict: &off}
}

// The check a turn boundary asks returns at once: the triage, a Jev network
// call, runs in the look's command, never on the Update thread. Live
// 2026-11-12 the synchronous look took 523ms there.
func TestSelfRepairPlan_TheTriageNeverRunsOnTheUpdateThread(t *testing.T) {
	t.Parallel()
	r := newSelfRepairRig(t, "", "20261028_061954_problems.log", nilSessionError)
	jev := &slowJev{verdictJev: verdictJev{def: 0.9}, delay: 300 * time.Millisecond}
	withTriage(&r, jev)

	start := time.Now()
	next, cmd := r.m.Update(selfRepairCheckMsg{})
	onThread := time.Since(start)
	if jev.askedCount() != 0 {
		t.Fatalf("Jev was asked %d time(s) on the Update thread", jev.askedCount())
	}
	if cmd == nil {
		t.Fatal("the look returned no plan command")
	}
	start = time.Now()
	plan, ok := planIn(cmd)
	planning := time.Since(start)
	if !ok || !plan.ok || jev.askedCount() == 0 {
		t.Fatalf("the plan command did not triage and build the job: %+v (jev asked %d)", plan, jev.askedCount())
	}
	t.Logf("Update thread: %v; the plan command, Jev included: %v", onThread, planning)
	landed, start2 := next.(Model), time.Now()
	after, out := landed.Update(plan)
	t.Logf("landing the plan on the Update thread: %v", time.Since(start2))
	if out == nil || len(after.(Model).background.list()) != 1 {
		t.Fatalf("the landed plan started no job: %+v", after.(Model).background.list())
	}
}

// A plan made while idle is dropped when the user started something before
// it landed: nothing is recorded, no job starts, and the next idle look makes
// it again.
func TestSelfRepairPlan_DroppedWhenTheUserBecameBusy(t *testing.T) {
	t.Parallel()
	r := newSelfRepairRig(t, "", "20261028_061954_problems.log", nilSessionError)
	m, cmd := r.m.selfRepairStep()
	plan, ok := planIn(cmd)
	if !ok || !plan.ok {
		t.Fatalf("no plan was made: %+v", plan)
	}
	m.isLoading = true // the user's turn began while the plan was made
	next, out := m.Update(plan)
	m = next.(Model)
	if out != nil {
		t.Fatal("a plan that landed beside the user's turn returned a command")
	}
	if n := len(m.background.list()); n != 0 {
		t.Fatalf("%d job(s) started beside the user's turn", n)
	}
	if n := len(r.ledger(t).Attempts()); n != 0 {
		t.Fatalf("the dropped plan recorded %d attempt(s)", n)
	}
	if m.selfRepair.planInFlight() {
		t.Fatal("the dropped plan still holds the plan slot")
	}
	m.isLoading = false
	m, _ = stepSelfRepair(t, m)
	if n := len(m.background.list()); n != 1 {
		t.Fatalf("the next idle look started %d job(s), want the one", n)
	}
}

// Never two plans at once: a look, or a tick, that finds one being made does
// nothing -- not even phase A -- until it lands.
func TestSelfRepairPlan_NeverTwoInFlight(t *testing.T) {
	t.Parallel()
	r := newSelfRepairRig(t, "", "20261028_061954_problems.log", nilSessionError)
	m, first := r.m.selfRepairStep()
	if first == nil {
		t.Fatal("the first look made no plan")
	}
	// A fix phase whose turn is gone would be ended by phase A; while the
	// plan is in flight, nothing of A runs.
	l := r.ledger(t)
	if err := l.Begin([]string{"other"}, problemlog.LaneRepair, "job-x", nil, *r.now); err != nil {
		t.Fatal(err)
	}
	if err := l.Fixing("job-x", *r.now); err != nil {
		t.Fatal(err)
	}
	if _, second := m.selfRepairStep(); second != nil {
		t.Fatal("a second look made a plan while the first was in flight")
	}
	next, tick := m.handleSelfRepairTick(selfRepairTickMsg{gen: m.selfRepair.gen})
	if tick == nil {
		t.Fatal("the tick stopped the timer")
	}
	if a, _ := l.Get("other"); a.Status != problemlog.RepairBeingFixed {
		t.Fatalf("phase A ran while a plan was in flight: %+v", a)
	}
	m = next
	// The first plan lands; then a look makes the next.
	if plan, ok := planIn(first); ok {
		landed, _ := m.Update(plan)
		m = landed.(Model)
	}
	if m.selfRepair.planInFlight() {
		t.Fatal("the landed plan still holds the slot")
	}
	if _, third := m.selfRepairStep(); third == nil {
		t.Fatal("no plan after the first landed")
	}
	if a, _ := l.Get("other"); a.Status != problemlog.RepairFailed {
		t.Fatalf("phase A did not run once the slot was free: %+v", a)
	}
}

// Phase B touches nothing the Update thread owns: run under -race, the
// Update thread changes the model's history, flags and config while B runs,
// and a turn syncs the problems log into the kernel beside it. The kernel
// ends with one projection of the log, not two.
func TestSelfRepairPlan_PhaseBSharesNothingWithTheUpdateThread(t *testing.T) {
	t.Parallel()
	r := newSelfRepairRig(t, "", "20261028_061954_problems.log", nilSessionError)
	jev := &slowJev{verdictJev: verdictJev{def: 0.9}, delay: 50 * time.Millisecond}
	withTriage(&r, jev)
	m, cmd := r.m.selfRepairStep()
	if cmd == nil {
		t.Fatal("no plan command")
	}
	plans := make(chan selfRepairPlanMsg, 1)
	go func() {
		plan, _ := planIn(cmd)
		plans <- plan
	}()
	// A turn's goroutine holds its own copy of the model (as processInput's
	// closure does); its config is its own here, so only the shared watcher
	// and kernel are under test.
	turnModel := m.selfRepairPlanner()
	turn := make(chan struct{})
	go func() {
		defer close(turn)
		for i := 0; i < 20; i++ {
			pendingSelfProblemsForTurn(turnModel)
		}
	}()
	// The Update thread carries on meanwhile.
	for i := 0; i < 20; i++ {
		m = m.pushAssistantMsg("still here")
		m.isLoading = i%2 == 0
		m.Config.Logging.SelfRepairPollSec = i + 1
		m.Config.Logging.SelfRepairCooldownSec = i + 1
		m.Config.TypeSafe = &config.TypeSafeConfig{}
	}
	m.isLoading = false
	plan := <-plans
	<-turn
	rows, err := m.kernel.Query(problemlog.ProblemPredicate)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("%d self_problem rows after a look and a turn synced the same one-problem log, want 1", len(rows))
	}
	if !plan.ok {
		t.Fatalf("the plan made beside the Update thread built no job: %s", plan.reason)
	}
}

// Phase B stops with the session: a look whose triage is waiting on Jev when
// the chat shuts down returns at once and starts nothing.
func TestSelfRepairPlan_CancelledWithTheSession(t *testing.T) {
	t.Parallel()
	r := newSelfRepairRig(t, "", "20261028_061954_problems.log", nilSessionError)
	jev := &slowJev{verdictJev: verdictJev{def: 0.9}, delay: time.Minute}
	withTriage(&r, jev)
	m, cmd := r.m.selfRepairStep()
	if cmd == nil {
		t.Fatal("no plan command")
	}
	plans := make(chan selfRepairPlanMsg, 1)
	go func() {
		plan, _ := planIn(cmd)
		plans <- plan
	}()
	time.Sleep(50 * time.Millisecond)
	m.shutdownCancel()
	var plan selfRepairPlanMsg
	select {
	case plan = <-plans:
	case <-time.After(30 * time.Second):
		t.Fatal("the plan did not stop with the session")
	}
	if plan.ok {
		t.Fatal("a cancelled plan built a job")
	}
	next, out := m.Update(plan)
	if out != nil || len(next.(Model).background.list()) != 0 {
		t.Fatal("a cancelled plan started a job")
	}
}
