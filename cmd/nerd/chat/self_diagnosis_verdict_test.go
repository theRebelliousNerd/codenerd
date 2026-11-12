package chat

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/perception"
	"codenerd/internal/problemlog"
	"codenerd/internal/typesafe"
)

// verdictJev answers every verdict question: p for a question whose problem
// message contains a key of byMessage, else def. fail makes the call fail.
type verdictJev struct {
	mu        sync.Mutex
	def       float64
	byMessage map[string]float64
	fail      bool
	asked     int
	states    []any
}

func (v *verdictJev) Ask(_ context.Context, state any, qs map[string]typesafe.Question) (*typesafe.Response, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.asked++
	v.states = append(v.states, state)
	if v.fail {
		return nil, errors.New("jev is down")
	}
	answers := map[string]any{}
	for name, q := range qs {
		p := v.def
		raw, _ := json.Marshal(q)
		for msg, pm := range v.byMessage {
			if strings.Contains(string(raw), msg) {
				p = pm
			}
		}
		answers[name] = map[string]any{"type": "noul", "noul": p}
	}
	raw, _ := json.Marshal(map[string]any{"model": "jev", "answers": answers, "usage": map[string]any{"input_tokens": 1, "output_tokens": 1}})
	var resp typesafe.Response
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// withVerdictJev turns the diagnosis verdict on with jev as the client.
func withVerdictJev(r *selfRepairRig, jev *verdictJev) {
	r.m.jev = jev
	off := false
	// Only the verdict: the triage would ask the same fake first.
	r.m.Config.TypeSafe = &config.TypeSafeConfig{DiagnosisTriage: &off}
}

// The live sentence a correct "nothing to change" diagnosis ended with.
const liveNoDefect = "Both findings name no defect in codeNERD's own source — handling was correct — so I changed nothing."

const providerOutage = "2026/10/28 06:20:10.000 [ERROR] [api] provider returned 529 overloaded after 3 attempts\n"

// diagnoseOnce runs the rig's self-repair trigger and its diagnosis job, and
// applies the job's result: what finishBackground did with it.
func diagnoseOnce(t *testing.T, r selfRepairRig) (Model, bool) {
	t.Helper()
	m, cmd := r.m.handleSelfRepairTick(selfRepairTickMsg{gen: 0})
	if len(m.background.list()) != 1 {
		t.Fatalf("no self-repair job started: %+v", m.background.list())
	}
	done := runBatchFor[backgroundDoneMsg](t, cmd)
	next, followUp := m.Update(done)
	return next.(Model), followUp != nil
}

// A diagnosis Jev reads as naming nothing of codeNERD's to fix starts no fix
// turn: the user sees one line saying so, the ledger settles the target as
// /no_defect without a failure, and the next look, past the cooldown, does
// not try it again on the same evidence.
func TestSelfRepair_NoDefectVerdictStartsNoFixTurn(t *testing.T) {
	t.Parallel()
	r := newSelfRepairRig(t, "", "20261028_061954_problems.log", providerOutage)
	r.exec.response = liveNoDefect
	jev := &verdictJev{def: 0.01}
	withVerdictJev(&r, jev)

	m, followed := diagnoseOnce(t, r)
	if followed || m.isLoading {
		t.Fatal("a no-defect verdict still started the fix turn")
	}
	if jev.asked != 1 {
		t.Fatalf("jev asked %d times, want once for the verdict", jev.asked)
	}
	if got := lastAssistant(m); !strings.Contains(got, "Self-diagnosis found nothing of codeNERD's to fix") ||
		!strings.Contains(got, "No fix turn was started") || strings.Contains(got, "failed") {
		t.Fatalf("the user is not told plainly:\n%s", got)
	}
	attempts := r.ledger(t).Attempts()
	if len(attempts) != 1 || attempts[0].Status != problemlog.RepairNoDefect || attempts[0].Failures != 0 || attempts[0].EvidenceCount < 1 {
		t.Fatalf("ledger = %+v, want one no_defect attempt, no failure, its evidence count", attempts)
	}
	if len(r.ledger(t).InFixing()) != 0 {
		t.Fatal("a fix turn is recorded as running")
	}

	// Past the cooldown, on the same evidence: not tried again.
	r.advance(time.Duration(config.DefaultSelfRepairCooldownSec+60) * time.Second)
	m, _ = m.selfRepairStep()
	if n := len(m.background.list()); n != 0 {
		t.Fatalf("%d jobs: a no-defect target was retried on the same evidence", n)
	}
	if a, _ := r.ledger(t).Get(attempts[0].Target); a.Tries != 1 || a.Status != problemlog.RepairNoDefect {
		t.Fatalf("after the next look: %+v", a)
	}
	if !strings.Contains(selfRepairStatusText(m), "1 no defect") {
		t.Fatalf("the status line does not count it: %s", selfRepairStatusText(m))
	}
}

// A diagnosis Jev reads as naming a defect runs the fix turn as before, with
// the chain's preset intent: the instruction is the request and the
// diagnosis its evidence, on the files it cites.
func TestSelfRepair_DefectVerdictRunsTheFixTurn(t *testing.T) {
	t.Parallel()
	r := newSelfRepairRig(t, "", "20261028_061954_problems.log", nilSessionError)
	r.exec.response = "Root cause: cmd/nerd/chat/background.go:120 dereferences a nil session. Fix: guard it, with a test."
	withVerdictJev(&r, &verdictJev{def: 0.95})

	m, followed := diagnoseOnce(t, r)
	if !followed || !m.isLoading {
		t.Fatal("a defect verdict did not start the fix turn")
	}
	attempts := r.ledger(t).Attempts()
	if len(attempts) != 1 || attempts[0].Status != problemlog.RepairBeingFixed || attempts[0].Phase != problemlog.PhaseFixing {
		t.Fatalf("ledger = %+v, want the fixing phase", attempts)
	}
	if strings.Contains(lastAssistant(m), "nothing of codeNERD's to fix") {
		t.Fatalf("a defect verdict said nothing to fix:\n%s", lastAssistant(m))
	}
}

// Jev off (the verdict use switched off), or its call failing: no verdict,
// and the fix turn runs as before it. A failed call is never a "no".
func TestSelfRepair_NoVerdictRunsTheFixTurnAsBefore(t *testing.T) {
	t.Parallel()
	for name, set := range map[string]func(*selfRepairRig){
		"jev nil": func(r *selfRepairRig) {},
		"use off": func(r *selfRepairRig) {
			off := false
			r.m.jev = &verdictJev{def: 0.01}
			r.m.Config.TypeSafe = &config.TypeSafeConfig{DiagnosisTriage: &off, DiagnosisVerdict: &off}
		},
		"call failed": func(r *selfRepairRig) { withVerdictJev(r, &verdictJev{def: 0.01, fail: true}) },
	} {
		t.Run(name, func(t *testing.T) {
			r := newSelfRepairRig(t, "", "20261028_061954_problems.log", providerOutage)
			r.exec.response = liveNoDefect
			set(&r)
			m, followed := diagnoseOnce(t, r)
			if !followed || !m.isLoading {
				t.Fatal("with no verdict the fix turn did not run as before")
			}
			if a := r.ledger(t).Attempts(); len(a) != 1 || a[0].Phase != problemlog.PhaseFixing {
				t.Fatalf("ledger = %+v", a)
			}
		})
	}
}

// One job, two targets, two verdicts: the target the diagnosis found nothing
// to fix for is settled as /no_defect, the other goes to the fix turn, which
// is told to leave the first alone.
func TestSelfRepair_PerTargetVerdictSplitsTheJob(t *testing.T) {
	t.Parallel()
	r := newSelfRepairRig(t, "", "20261028_061954_problems.log", nilSessionError+providerOutage)
	r.exec.response = "Problem 1: cmd/nerd/chat/background.go:120 dereferences a nil session; guard it. Problem 2 is the provider's outage, handled correctly."
	cited := filepath.Join(r.m.workspace, "cmd", "nerd", "chat", "background.go")
	if err := os.MkdirAll(filepath.Dir(cited), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cited, []byte("package chat\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	withVerdictJev(&r, &verdictJev{def: 0.5, byMessage: map[string]float64{"nil session": 0.92, "overloaded": 0.03}})

	m, cmd := r.m.handleSelfRepairTick(selfRepairTickMsg{gen: 0})
	done := runBatchFor[backgroundDoneMsg](t, cmd)
	if len(done.Job.diagnosed) != 2 {
		t.Fatalf("the job diagnosed %+v, want both problems", done.Job.diagnosed)
	}
	next, followUp := m.Update(done)
	m = next.(Model)
	if followUp == nil {
		t.Fatal("the defect's fix turn did not run")
	}
	outage := problemlog.Signature("provider returned 529 overloaded after 3 attempts")
	nilSession := problemlog.Signature("nil session in executor dispatch: panic recovered")
	if a, _ := r.ledger(t).Get(outage); a.Status != problemlog.RepairNoDefect {
		t.Fatalf("outage = %+v, want no_defect", a)
	}
	if a, _ := r.ledger(t).Get(nilSession); a.Phase != problemlog.PhaseFixing {
		t.Fatalf("nil session = %+v, want fixing", a)
	}
	if got := lastAssistant(m); !strings.Contains(got, "for part of it") {
		t.Fatalf("the user is not told what was left:\n%s", got)
	}
	step := m.fixStep(done.Job, done.Result, m.decideDiagnosisFix(done.Job, done.Verdict))
	if !strings.Contains(step.Instruction, "leave them") || step.Target != "cmd/nerd/chat/background.go" || step.Verb != "/fix" {
		t.Fatalf("fix step = %+v", step)
	}
}

// The fix step is the chain's: it is one delegated fix of the file the
// diagnosis cites, never a multi-step plan made of the diagnosis's own
// sentences, and its target is known, so it never stops to ask for one.
func TestChainStep_IsOneDelegatedFixNotADecomposedPlan(t *testing.T) {
	t.Parallel()
	m := admissionModel(t, config.WorkModeYolo, nil)
	step := chainStep{
		Instruction: selfDiagnosisFollowUp,
		Evidence:    "Both findings name no defect. First, review the retry loop and then fix the tests; after that, create a test and refactor the parser. " + liveNoDefect,
		Verb:        "/fix",
		Target:      ownSourceTarget,
	}
	intent := step.intent()
	if intent.Target == "" || intent.Category != "/mutation" || !strings.Contains(intent.Constraint, liveNoDefect) {
		t.Fatalf("preset intent = %+v", intent)
	}
	// Read as a request, this input is a plan: the signals fire.
	if sigs := multiStepSignals(step.input(), intent); len(sigs) == 0 {
		t.Fatal("the test input no longer looks like a plan; it proves nothing")
	}
	assertRouteIntent(t, m, intent)
	m.chainStep = &step
	route := m.decideRoute(step.input(), intent, "coder")
	if route.Kind != RouteDelegate || route.Shard != "coder" {
		t.Fatalf("chain step routed %+v, want one delegation to the coder", route)
	}
	// The exemption is the chain's alone: a request that decomposes still
	// does as a user's turn, and does not as a chain's step.
	plan := perception.Intent{Category: "/mutation", Verb: "/create", Target: "auth middleware", Confidence: 0.95}
	assertRouteIntent(t, m, plan)
	m.chainStep = nil
	if route := m.decideRoute("create the auth middleware and write tests for it", plan, "coder"); route.Kind != RouteMultiStep {
		t.Fatalf("a user's multi-step request routed %+v, want the multi-step lane", route)
	}
	m.chainStep = &step
	if route := m.decideRoute("create the auth middleware and write tests for it", plan, "coder"); route.Kind == RouteMultiStep {
		t.Fatalf("a chain step was decomposed: %+v", route)
	}
}

// The user comes first in the queue: a fix step codeNERD queued for itself
// yields to a message of the user's queued after it.
func TestTurnQueue_AnAutonomousStepYieldsToTheUser(t *testing.T) {
	t.Parallel()
	ts := newTurnState()
	ts.enqueue(queuedMessage{input: "fix it", chain: &chainStep{Instruction: "fix it", Autonomous: true}})
	ts.enqueue(queuedMessage{input: "and the README"})
	first, ok := ts.dequeue()
	if !ok || first.input != "and the README" {
		t.Fatalf("first = %+v, want the user's message", first)
	}
	second, ok := ts.dequeue()
	if !ok || second.chain == nil {
		t.Fatalf("second = %+v, want the step", second)
	}
	// A step the user asked for (their own "debug yourself") keeps its place.
	ts.enqueue(queuedMessage{input: "fix it", chain: &chainStep{Instruction: "fix it"}})
	ts.enqueue(queuedMessage{input: "and the README"})
	if first, _ := ts.dequeue(); first.chain == nil {
		t.Fatalf("a user-requested chain step lost its place to a later message: %+v", first)
	}
}

// A queued fix step survives the restart as that step, not as a message to
// perceive.
func TestChainStep_SurvivesTheRestartAsAStep(t *testing.T) {
	t.Parallel()
	r := newSelfRepairRig(t, "", "20261028_061954_problems.log", "")
	step := chainStep{Instruction: "fix", Evidence: "the diagnosis", Verb: "/fix", Target: ownSourceTarget, Autonomous: true}
	r.m.turn.enqueue(queuedMessage{input: step.input(), chain: &step})
	st := r.m.restartState()
	if len(st.Resume) != 1 || st.Resume[0].Step == nil || st.Resume[0].Step.Evidence != "the diagnosis" {
		t.Fatalf("restart state = %+v", st.Resume)
	}
	fresh := newSelfRepairRig(t, "", "20261112_090000_problems.log", "")
	fresh.m.resumeWork = st.Resume
	fresh.m, _ = fresh.m.resumeRestoredWork()
	q := fresh.m.turn.queuedWork()
	if len(q) != 1 || q[0].chain == nil || *q[0].chain != step {
		t.Fatalf("resumed queue = %+v", q)
	}
}

// A self-repair fix turn the user stopped settles as stopped, not failed: it
// does not move the target toward being left.
func TestSelfRepair_AFixTurnTheUserStoppedIsNotAFailure(t *testing.T) {
	t.Parallel()
	r := newSelfRepairRig(t, "", "20261028_061954_problems.log", nilSessionError)
	sig := problemlog.Signature("nil session in executor dispatch: panic recovered")
	l := r.ledger(t)
	if err := l.Begin([]string{sig}, problemlog.LaneRepair, "job-1", nil, *r.now); err != nil {
		t.Fatal(err)
	}
	if err := l.Fixing("job-1", *r.now); err != nil {
		t.Fatal(err)
	}
	id := r.m.turn.accept()
	r.m.turn.begin(id, func() {})
	r.m.selfRepair.fixTurn = id
	if !r.m.turn.stop() {
		t.Fatal("the fix turn could not be stopped")
	}
	r.m.turn.end(id)
	m, _ := r.m.selfRepairStep()
	if a, _ := r.ledger(t).Get(sig); a.Status != problemlog.RepairStopped || a.Failures != 0 {
		t.Fatalf("a stopped fix turn settled %+v, want stopped with no failure", a)
	}
	if !strings.Contains(lastAssistant(m), "recorded as stopped, not failed") {
		t.Fatalf("the user is not told: %q", lastAssistant(m))
	}
}

// The brief is the conversation's, not the chain's: in yolo an unready brief
// holds the user's work but not the chain's fix step; propose still holds it.
func TestChainStep_IsNotHeldOnTheConversationsBrief(t *testing.T) {
	t.Parallel()
	notReady := &perception.Brief{Goal: "talk through the retry policy"}
	step := chainStep{Instruction: selfDiagnosisFollowUp, Evidence: "Root cause at cmd/nerd/chat/background.go:120.", Verb: "/fix", Target: ownSourceTarget}
	for _, tc := range []struct {
		mode config.WorkMode
		want RouteKind
	}{{config.WorkModeYolo, RouteDelegate}, {config.WorkModePropose, RouteHold}} {
		m := admissionModel(t, tc.mode, notReady)
		intent := step.intent()
		assertRouteIntent(t, m, intent)
		m.chainStep = &step
		if route := m.decideRoute(step.input(), intent, "coder"); route.Kind != tc.want {
			t.Fatalf("%s: chain step routed %+v, want %v", tc.mode, route, tc.want)
		}
	}
}
