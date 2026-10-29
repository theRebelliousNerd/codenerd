package chat

import (
	"strings"
	"testing"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/perception"
)

// The truth table for one judgment has a row for the middle band: between the
// no and yes thresholds it derives jev_unsettled and nothing else. Perception's
// reading routes the turn, so a judgment at 0.5 never disputes a mutation,
// approves a go-ahead, stops a job or adds a restriction.
func TestJevJourney_UnsettledJudgmentsAreNamedAndActOnNothing(t *testing.T) {
	m, exec := journeyModel(t, config.WorkModeYolo, fixReadmeTurn("Fixing it.", nil))
	unsettled := map[string]float64{
		"changes_files": 0.5, "names_target": 0.5, "hypothetical": 0.5, "rules_out_changes": 0.5, "about_codenerd": 0.5,
	}
	m = withJev(t, m, &fakeJevServer{probs: unsettled})
	intent := fixReadmeTurn("", nil)
	assertRouteIntent(t, m, intent)
	m.recordTurnJudgments(m.shutdownCtx, m.startTurnJudgments(m.shutdownCtx, "make the readme better", nil))

	rows, err := m.kernel.Query("jev_unsettled")
	if err != nil || len(rows) != len(unsettled) {
		t.Fatalf("jev_unsettled = %d rows (%v), want one per unsettled judgment (%d)", len(rows), err, len(unsettled))
	}
	// The act choice, split evenly over its options, settles no act.
	if acts, err := m.kernel.Query("jev_choice_unsettled"); err != nil || len(acts) != 1 {
		t.Fatalf("jev_choice_unsettled = %v (%v), want the conversation act", acts, err)
	}
	if note := m.unsettledNote(); !strings.Contains(note, "conversation_act") {
		t.Fatalf("the log line does not name the unsettled judgments and the unsettled act: %q", note)
	}
	if route := m.decideRoute("make the readme better", intent, "coder"); route.Kind == RouteClarify {
		t.Fatalf("an unsettled judgment disputed the mutation: route %s", route.Kind)
	}
	if r := m.jevRestriction(); !r.IsZero() {
		t.Fatalf("unsettled judgments restricted the turn: %v", r.Kinds())
	}
	if exec.callCount() != 0 {
		t.Fatal("routing ran work")
	}
}

// With no work running there is nothing to stop, so the stop judgment is not
// asked; with a job running it is, and the state names the job.
func TestJevJourney_StopIsAskedOnlyWhileWorkRuns(t *testing.T) {
	chat := perception.Intent{Category: "/query", Verb: "/converse", Target: "none", Confidence: 0.9}
	m, _ := journeyModel(t, config.WorkModePropose, chat)
	if got := m.runningWork(); len(got) != 0 {
		t.Fatalf("running work = %v on an idle chat", got)
	}
	block := make(chan struct{})
	m.taskExecutor = &blockingExecutor{release: block}
	job := backgroundJob{ID: "bg-1", Shard: "researcher", Task: "survey the retry code", Started: time.Now()}
	run := m.startBackground(job, job.Task)
	done := make(chan struct{})
	go func() { run(); close(done) }()
	defer func() { close(block); <-done }()
	deadline := time.Now().Add(5 * time.Second)
	for len(m.runningWork()) == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	got := m.runningWork()
	if len(got) != 1 || got[0] != "researcher: survey the retry code" {
		t.Fatalf("running work = %v, want the job as a person reads it", got)
	}
}
