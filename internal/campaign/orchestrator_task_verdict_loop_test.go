package campaign

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/observation"
	"codenerd/internal/session"
)

// scriptedExecutor answers each dispatch with the next scripted return and
// records the task text it was sent.
func scriptedExecutor(returns []observation.Return, sent *[]string) *MockTaskExecutor {
	return &MockTaskExecutor{ExecuteObservedFunc: func(_ context.Context, req session.TaskRequest) (observation.Return, error) {
		i := len(*sent)
		*sent = append(*sent, req.Task)
		if i >= len(returns) {
			return observation.Return{}, errors.New("scripted executor: out of returns")
		}
		return returns[i], nil
	}}
}

func campaignTask() Task {
	return Task{ID: "/task_loop", Description: "add the tests", Type: TaskTypeFileModify, WriteSet: []string{"internal/pool/pool.go"}}
}

// A turn the kernel judged /unverified goes out again inside the attempt, with
// what it left missing and the report it ended with, and its writes stay in
// the attempt's record: failing the attempt at the first /unverified turn
// undid its work and started the next attempt over from the failure text.
func TestSpawnTask_AnUnverifiedTurnIsSentBackWithTheGapInsideTheAttempt(t *testing.T) {
	t.Parallel()
	orch, _ := newPolicyFailureOrchestrator(t, nil, campaignTask())
	var sent []string
	orch.taskExecutor = scriptedExecutor([]observation.Return{
		{Output: "Wrote pool.go; TestPool is left.", Outcome: "/unverified", Missing: []string{"/self_reported_incomplete", "/tests_not_written"}, Changed: []string{"pool.go"},
			Writes: []observation.FileWrite{{Path: "/w/pool.go", After: observation.FileState{Known: true, Exists: true, Content: "v1"}}}},
		{Output: "Added TestPool.", Outcome: "/done", Changed: []string{"pool_test.go"},
			Writes: []observation.FileWrite{{Path: "/w/pool_test.go", After: observation.FileState{Known: true, Exists: true, Content: "t"}}}},
	}, &sent)
	task := &orch.campaign.Phases[0].Tasks[0]
	orch.beginAttempt(task)

	out, err := orch.spawnTask(context.Background(), task, "/fix", "modify file:internal/pool/pool.go")
	if err != nil {
		t.Fatalf("spawnTask = %v, want the second dispatch to complete the attempt", err)
	}
	if out != "Added TestPool." {
		t.Fatalf("output = %q, want the verified dispatch's", out)
	}
	if len(sent) != 2 {
		t.Fatalf("dispatched %d time(s), want 2", len(sent))
	}
	for _, want := range []string{"modify file:internal/pool/pool.go", "the turn's own report says part of the requested work was not done", "Wrote pool.go; TestPool is left."} {
		if !strings.Contains(sent[1], want) {
			t.Errorf("the second dispatch does not carry %q:\n%s", want, sent[1])
		}
	}
	if got := len(orch.attemptWrites(task)); got != 2 {
		t.Fatalf("the attempt recorded %d write(s), want both dispatches' (so a failed attempt undoes all of them)", got)
	}
}

// A dispatch that writes nothing and clears nothing ends the attempt, named by
// the gap and by the stop.
func TestSpawnTask_NoProgressFailsTheAttemptNamingTheGap(t *testing.T) {
	t.Parallel()
	orch, _ := newPolicyFailureOrchestrator(t, nil, campaignTask())
	var sent []string
	orch.taskExecutor = scriptedExecutor([]observation.Return{
		{Output: "one", Outcome: "/unverified", Missing: []string{"/tests_not_green"}, Changed: []string{"pool.go"}},
		{Output: "two", Outcome: "/unverified", Missing: []string{"/tests_not_green"}},
		{Output: "three", Outcome: "/done"},
	}, &sent)
	task := &orch.campaign.Phases[0].Tasks[0]

	_, err := orch.spawnTask(context.Background(), task, "/fix", "x")
	if !errors.Is(err, ErrTaskNotDone) {
		t.Fatalf("spawnTask = %v, want ErrTaskNotDone", err)
	}
	if len(sent) != 2 {
		t.Fatalf("dispatched %d time(s), want the stall to end it at 2", len(sent))
	}
	for _, want := range []string{"the tests were not verified green", "wrote nothing and cleared nothing"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not say %q", err, want)
		}
	}
	signals := failureSignals(err)
	for _, want := range []string{"/turn_not_done", "/unverified", "/tests_not_green", "/stalled"} {
		if !slices.Contains(signals, want) {
			t.Errorf("signals %v lack %s", signals, want)
		}
	}
}

// A task whose last attempt ended at an obstacle its own report named is not
// retried: the next attempt meets the same permission, credential or choice.
// It goes to the replanner once and then fails.
func TestHandleTaskFailure_ABlockedAttemptIsNotRetried(t *testing.T) {
	t.Parallel()
	orch, _ := newPolicyFailureOrchestrator(t, nil, campaignTask())
	blocked := withSignals(errors.New("turn: "+ErrTaskNotDone.Error()), "/turn_not_done", "/unverified", "/self_reported_incomplete", "/blocked")
	live := failTask(orch, "/task_loop", blocked)
	if live.Status != TaskFailed || !live.NextRetryAt.IsZero() {
		t.Fatalf("a blocked attempt left the task %s with retry %v, want failed with none", live.Status, live.NextRetryAt)
	}

	// A stalled attempt is not blocked: a fresh attempt, from a clean tree, may
	// move.
	orch2, _ := newPolicyFailureOrchestrator(t, nil, campaignTask())
	stalled := withSignals(errors.New("turn: "+ErrTaskNotDone.Error()), "/turn_not_done", "/unverified", "/tests_not_green", "/stalled")
	live = failTask(orch2, "/task_loop", stalled)
	if live.Status != TaskPending || live.NextRetryAt.IsZero() {
		t.Fatalf("a stalled attempt left the task %s with retry %v, want a pending retry", live.Status, live.NextRetryAt)
	}
}

// A document task whose shard stopped at an obstacle its report named is not
// answered by generating the document directly: the fallback would deliver
// the deliverable without the permission, input or choice the shard needed.
func TestExecuteWithExplicitShard_ABlockedDocumentTaskGetsNoFallback(t *testing.T) {
	t.Parallel()
	task := Task{ID: "/task_doc", Description: "write the rollout plan", Type: TaskTypeDocument, Shard: "coder", Artifacts: []TaskArtifact{{Type: "/document", Path: "Docs/plan.md"}}}
	orch, kernel := newPolicyFailureOrchestrator(t, nil, task)
	// Jev's thresholds, which the executor asserts when it reads a report.
	if err := config.EnsureParams(kernel, (&config.TypeSafeConfig{}).Params()); err != nil {
		t.Fatal(err)
	}
	var sent []string
	blocked := observation.Return{Output: "I need the deploy key.", Outcome: "/unverified", Missing: []string{"/self_reported_incomplete"}, Report: &observation.ReportReading{Unfinished: 97, Blocked: 98}}
	orch.taskExecutor = scriptedExecutor([]observation.Return{blocked}, &sent)
	generated := false
	orch.llmClient = &MockLLMClient{CompleteFunc: func(context.Context, string) (string, error) {
		generated = true
		return "# plan", nil
	}}

	_, err := orch.executeWithExplicitShard(context.Background(), &orch.campaign.Phases[0].Tasks[0])
	if err == nil || !slices.Contains(failureSignals(err), "/blocked") {
		t.Fatalf("executeWithExplicitShard = %v, want a /blocked failure", err)
	}
	if generated || strings.Contains(err.Error(), "fallback") {
		t.Fatalf("the document was generated directly around the obstacle: %v", err)
	}
	if len(sent) != 1 {
		t.Fatalf("a blocked turn was sent %d times, want once", len(sent))
	}
}
