package chat

import (
	"context"
	"errors"
	"strings"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/perception"
)

// failedIntent is what the perception layer returns for a classification no
// model completed: the zero-confidence /explain compatibility projection with
// a nil error, carrying the typed cause.
func failedIntent(err error) perception.Intent {
	return perception.Intent{
		Category: "/query", Verb: "/explain", Confidence: 0,
		Response: "I had trouble understanding that: the model call failed",
		Outcome:  perception.FailedOutcome(err, perception.MethodGeneric, 1),
	}
}

func classifierUnauthorized() error { return errors.New("API request failed with status 401: bad key") }
func classifierOutage() error       { return errors.New("transient server error (503): overloaded") }

// A fresh normal turn whose classification was rejected ends with the kernel's
// notice for the cause. It does not route as an ordinary /explain answer: the
// articulation model is never asked, and no task runs.
func TestClassificationFailure_EndsTheTurnWithTheCauseSpecificNotice(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		err    error
		notice string
	}{
		{"authentication", classifierUnauthorized(), "rejected the configured credentials"},
		{"outage", classifierOutage(), "couldn't reach the language model"},
		{"unreadable answer", &perception.ClassificationError{Stage: perception.StageDecode, Code: "bare_field_type:action_type"}, "could not be understood"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, exec := journeyModel(t, config.WorkModeYolo, failedIntent(tc.err))
			mock := m.client.(*MockLLMClient)
			m, msg, _ := say(t, m, "Investigate why the parser ignores this file")
			reply, ok := msg.(responseMsg)
			if !ok {
				t.Fatalf("a failed classification answered %T, want the kernel's notice", msg)
			}
			if !strings.Contains(string(reply), tc.notice) {
				t.Fatalf("notice = %q, want it to contain %q", string(reply), tc.notice)
			}
			if mock.callCount != 0 || exec.callCount() != 0 {
				t.Fatalf("the failed classification continued: articulation calls=%d, tasks=%d", mock.callCount, exec.callCount())
			}
			if rows, _ := m.kernel.Query("classification_outcome"); len(rows) != 1 {
				t.Fatalf("classification_outcome rows = %v, want the turn's single row", rows)
			}
		})
	}
}

// The controls: a read-and-uncertain turn, and a turn no model classified,
// are not stopped.
func TestClassificationFailure_DoesNotStopAdmittedOrPresetTurns(t *testing.T) {
	t.Parallel()
	uncertain := perception.Intent{
		Category: "/query", Verb: "/converse", Target: "none", Confidence: 0.2,
		Response: "Hello!",
		Outcome:  perception.ClassificationOutcome{Disposition: perception.ClassificationAdmitted, Method: perception.MethodGeneric, Attempts: 1},
	}
	m, _ := journeyModel(t, config.WorkModeYolo, uncertain)
	if _, stopped := m.classificationStop(uncertain, nil); stopped {
		t.Fatal("an admitted low-confidence reading was stopped as a classification failure")
	}
	if _, stopped := m.classificationStop(perception.Intent{Verb: "/fix"}, nil); stopped {
		t.Fatal("a turn no model classified was stopped")
	}
}

// Held work's go-ahead keeps its precedence over a failed classification,
// from Jev's independent judgment alone. The failed classification is not
// relabeled successful, and the work runs under its own held intent.
func TestClassificationFailure_HeldWorkGoAheadStillApproves(t *testing.T) {
	t.Parallel()
	fix := perception.Intent{
		Category: "/mutation", Verb: "/fix", Target: "README.md", Confidence: 0.95,
		Response: "I can fix the typo in README.md.",
		Brief:    &perception.Brief{Goal: "fix the typo in README.md", Ready: true},
	}
	m, _ := journeyModel(t, config.WorkModePropose, fix, failedIntent(classifierOutage()))
	jev := &fakeJevServer{probs: map[string]float64{"approves_held_work": 0.97, "adds_to_held_work": 0.03}}
	m = withJev(t, m, jev)

	m, msg, _ := say(t, m, "can you fix the typo in README.md")
	if _, ok := msg.(holdMsg); !ok {
		t.Fatalf("propose mode answered %T, want the work held", msg)
	}
	m, msg, _ = say(t, m, "yes, ship it")
	if _, ok := msg.(approveWorkMsg); !ok {
		t.Fatalf("a Jev-read go-ahead during a failed classification answered %T, want the approve lane", msg)
	}
}

// With no go-ahead evidence, pending work does not let a failed classification
// through.
func TestClassificationFailure_PendingWorkAloneDoesNotClearIt(t *testing.T) {
	t.Parallel()
	fix := perception.Intent{
		Category: "/mutation", Verb: "/fix", Target: "README.md", Confidence: 0.95,
		Response: "I can fix the typo in README.md.",
		Brief:    &perception.Brief{Goal: "fix the typo in README.md", Ready: true},
	}
	m, _ := journeyModel(t, config.WorkModePropose, fix, failedIntent(classifierUnauthorized()))
	m, msg, _ := say(t, m, "can you fix the typo in README.md")
	if _, ok := msg.(holdMsg); !ok {
		t.Fatalf("propose mode answered %T, want the work held", msg)
	}
	_, msg, _ = say(t, m, "hmm what about the license")
	if _, ok := msg.(responseMsg); !ok {
		t.Fatalf("answered %T, want the failure notice", msg)
	}
}

// ---- side turns ------------------------------------------------------------

// A message no model could read, while work runs, is not dropped: when a retry
// can settle the failure it waits for the running turn, and the reply is the
// kernel's notice for the cause.
func TestSideTurn_RetryableFailureQueuesTheMessage(t *testing.T) {
	t.Parallel()
	m, ctx := busyModel(t, failedIntent(classifierOutage()))
	m, cmd := typeAndEnter(t, m, "also add a CHANGELOG entry")
	m = runSideTurn(t, m, cmd)
	if ctx.Err() != nil {
		t.Fatal("a failed read stopped the running work")
	}
	if m.turn.queuedCount() != 1 {
		t.Fatalf("queued = %d; an unread message must wait for the running turn, not vanish", m.turn.queuedCount())
	}
	reply := lastAssistant(m)
	if !strings.Contains(reply, "couldn't reach the language model") || !strings.Contains(reply, "as soon as the current work finishes") {
		t.Fatalf("reply = %q, want the outage notice and the queue promise", reply)
	}
}

// A blocker queues nothing -- the same message fails the same way -- and says
// what to change.
func TestSideTurn_BlockerIsNotQueued(t *testing.T) {
	t.Parallel()
	m, _ := busyModel(t, failedIntent(classifierUnauthorized()))
	m, cmd := typeAndEnter(t, m, "also add a CHANGELOG entry")
	m = runSideTurn(t, m, cmd)
	if m.turn.queuedCount() != 0 {
		t.Fatalf("queued = %d; a rejected credential would only fail again", m.turn.queuedCount())
	}
	if !strings.Contains(lastAssistant(m), "rejected the configured credentials") {
		t.Fatalf("reply = %q", lastAssistant(m))
	}
}

// Jev's stop is independent evidence: a primary classification failure does not
// stop the user from stopping the work.
func TestSideTurn_StopSurvivesAFailedClassification(t *testing.T) {
	t.Parallel()
	m, ctx := busyModel(t, failedIntent(classifierUnauthorized()))
	m = withJev(t, m, &fakeJevServer{probs: map[string]float64{"wants_to_stop": 0.97, "changes_files": 0.03}})
	m, cmd := typeAndEnter(t, m, "stop")
	m = runSideTurn(t, m, cmd)
	if ctx.Err() == nil {
		t.Fatal("the stop was lost because the classification failed")
	}
	if !strings.Contains(lastAssistant(m), "Stopping the current work") {
		t.Fatalf("reply = %q", lastAssistant(m))
	}
}

// The error branch keeps its promise and never shows a provider body.
func TestSideTurn_ErrorBranchQueuesAndStaysSafe(t *testing.T) {
	t.Parallel()
	m, _ := busyModel(t, failedIntent(classifierOutage()))
	slot := m.turn.reserve("also add a CHANGELOG entry", 0)
	m = m.applySideTurn(sideTurnMsg{Input: "also add a CHANGELOG entry", Slot: slot, Err: errors.New("API request failed with status 401: sk-live-secret-body")})
	reply := lastAssistant(m)
	if strings.Contains(reply, "sk-live-secret-body") {
		t.Fatalf("reply leaks the provider body: %q", reply)
	}
	if m.turn.queuedCount() != 1 || !strings.Contains(reply, "queued") {
		t.Fatalf("queued=%d reply=%q; the message claims a queue it must keep", m.turn.queuedCount(), reply)
	}
	m2, _ := busyModel(t, failedIntent(classifierOutage()))
	m2 = m2.applySideTurn(sideTurnMsg{Input: "x", Slot: m2.turn.reserve("x", 0), Err: context.Canceled})
	if m2.turn.queuedCount() != 0 {
		t.Fatal("a canceled read was queued")
	}
}

// Retrying after an outage is a fresh read: the recovered turn inherits no
// failure state, and the failed turn was not mistaken for a clarification the
// retry answered.
func TestClassificationFailure_RetryAfterOutageIsAFreshTurn(t *testing.T) {
	t.Parallel()
	chat := perception.Intent{
		Category: "/query", Verb: "/converse", Target: "none", Confidence: 0.9,
		Response: "It is the second line.",
		Outcome:  perception.ClassificationOutcome{Disposition: perception.ClassificationAdmitted, Method: perception.MethodGeneric, Attempts: 1},
	}
	m, _ := journeyModel(t, config.WorkModeYolo, failedIntent(classifierOutage()), chat)
	m, msg, _ := say(t, m, "where is the typo")
	if _, ok := msg.(responseMsg); !ok || !strings.Contains(string(msg.(responseMsg)), "couldn't reach the language model") {
		t.Fatalf("the outage turn answered %T %v", msg, msg)
	}
	if m.clarificationState != nil {
		t.Fatalf("an outage notice was recorded as a clarification the retry would answer: %+v", m.clarificationState)
	}
	m, msg, _ = say(t, m, "where is the typo")
	if got, ok := msg.(responseMsg); !ok || string(got) != "It is the second line." {
		t.Fatalf("the retried turn answered %T %v, want the perception answer", msg, msg)
	}
	if rows, _ := m.kernel.Query("classification_outcome"); len(rows) != 1 {
		t.Fatalf("classification_outcome rows = %v, want only the recovered turn's", rows)
	}
}

// A stop Jev read while the classification failed was acted on before the
// turn ended; the user is told, not left to find the job gone.
func TestClassificationFailure_NoticeCarriesTheTurnsWarnings(t *testing.T) {
	t.Parallel()
	m, _ := journeyModel(t, config.WorkModeYolo, failedIntent(classifierUnauthorized()))
	failed := failedIntent(classifierUnauthorized())
	if err := perception.PublishClassificationOutcome(m.kernel, "/current_intent", failed.Outcome); err != nil {
		t.Fatal(err)
	}
	reply, stopped := m.classificationStop(failed, []string{"Stopping background reviewer: review the parser"})
	if !stopped {
		t.Fatal("the failed classification did not stop the turn")
	}
	text := string(reply.(responseMsg))
	if !strings.Contains(text, "rejected the configured credentials") || !strings.Contains(text, "Stopping background reviewer") {
		t.Fatalf("notice = %q, want the cause and the stop", text)
	}
}
