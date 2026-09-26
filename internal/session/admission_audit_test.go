package session

import (
	"context"
	"strings"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/types"
)

// N41: R5-3 closed /done one paragraph after "I cannot execute the remaining
// 177 deletions ... Requesting re-invocation". Every gate was green because the
// deletions it did make were right. A report that admits unfinished work now
// withholds /done and says why.
func TestTurnWhoseReportAdmitsUnfinishedWork_IsNotDone(t *testing.T) {
	e := newObligationExec(t)
	result := writeTurnResult()
	result.VetCheck = BuildVerification{Ran: true, OK: true, Outcome: VerifyPassed}
	result.SelfReportedIncomplete = true
	e.assertTurnEvidence(testTurn, "/fix", result)

	if got := queryCount(t, e, "turn_verified"); got != 0 {
		t.Errorf("turn_verified = %d on a turn whose own report says the work is unfinished", got)
	}
	e.captureTurnOutcome(testTurn, result, nil)
	if result.TurnOutcome == types.MangleAtom("/done") {
		t.Errorf("TurnOutcome = /done after the report admitted unfinished work")
	}
	found := false
	for _, m := range result.MissingEvidence {
		found = found || m == "/self_reported_incomplete"
	}
	if !found {
		t.Errorf("MissingEvidence = %v, want it to name /self_reported_incomplete", result.MissingEvidence)
	}
}

// The fact is the host's reading of the report. A model that could assert or
// retract it would be deciding its own verdict.
func TestTurnSelfReportedIncomplete_IsNotWritableByTheModel(t *testing.T) {
	permissive := core.MangleUpdatePolicy{AllowedPrefixes: []string{""}}
	kept, _ := core.FilterMangleUpdates(nil, []string{"turn_self_reported_incomplete(/fix)."}, permissive)
	if len(kept) != 0 {
		t.Errorf("a model update asserting turn_self_reported_incomplete was accepted: %v", kept)
	}
}

type auditClient struct {
	MockLLMClient
	answer string
	calls  int
	prompt string
}

func (c *auditClient) CompleteWithSystem(_ context.Context, _, prompt string) (string, error) {
	c.calls++
	c.prompt = prompt
	return c.answer, nil
}

func TestAuditFinalReport(t *testing.T) {
	report := "Deleted 11 of 189 files. I cannot execute the remaining 177 deletions; requesting re-invocation."
	for _, tc := range []struct {
		name      string
		answer    string
		writes    int
		wantCall  bool
		wantFlags bool
	}{
		{"an admission withholds", "INCOMPLETE", 1, true, true},
		{"a clean report does not", "COMPLETE", 1, true, false},
		{"an unparseable answer asserts nothing", "maybe?", 1, true, false},
		{"a read-only turn is not read", "INCOMPLETE", 0, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &auditClient{answer: tc.answer}
			e := NewExecutor(nil, nil, client, nil, nil, nil)
			e.config = DefaultExecutorConfig()
			result := &ExecutionResult{Response: report, SuccessfulWriteTools: tc.writes}
			e.auditFinalReport(context.Background(), result)
			if (client.calls == 1) != tc.wantCall {
				t.Errorf("classifier calls = %d, want call=%v", client.calls, tc.wantCall)
			}
			if result.SelfReportedIncomplete != tc.wantFlags {
				t.Errorf("SelfReportedIncomplete = %v, want %v", result.SelfReportedIncomplete, tc.wantFlags)
			}
			if tc.wantCall && !strings.Contains(client.prompt, "remaining 177 deletions") {
				t.Errorf("the classifier was not shown the report:\n%s", client.prompt)
			}
		})
	}
}
