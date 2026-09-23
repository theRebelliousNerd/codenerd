package verification

import (
	"codenerd/internal/config"
	"context"
	"errors"
	"strings"
	"testing"
)

// Sweep finding F5: a delegated mutation took an LLM judge's word as its
// verdict. The verifier spawned through the string-only executor, which
// dropped the turn's kernel verdict, so a turn the kernel ended /unverified
// completed the request when the judge answered success. The kernel's verdict
// decides now (delegation_move, policy/delegation.mg); the judge can only
// withhold.

// judgeClient answers every judgment with answer and records each system
// prompt it was given.
func judgeClient(answer string) (*stubLLMClient, *[]string) {
	var prompts []string
	return &stubLLMClient{completeWithSystem: func(_ context.Context, system, _ string) (string, error) {
		prompts = append(prompts, system)
		return answer, nil
	}}, &prompts
}

const judgeSays = `{"success":true,"confidence":0.95,"reason":"looks complete"}`

func TestVerifyWithRetry_AnUnverifiedTurnIsNotAcceptedOnTheJudgesWord(t *testing.T) {
	client, prompts := judgeClient(judgeSays)
	exec := &stubTaskExecutor{result: "Wrote 1 file(s).", outcome: "/unverified", missing: []string{"/tests_not_written"}}
	v := newDelegationVerifier(t, client, nil, exec)

	_, verdict, err := v.VerifyWithRetry(context.Background(), delegation("create internal/a/a.go", 1))
	if !errors.Is(err, ErrMaxRetriesExceeded) {
		t.Fatalf("VerifyWithRetry = %v, want ErrMaxRetriesExceeded for a turn the kernel ended /unverified", err)
	}
	if verdict == nil || verdict.Success {
		t.Fatalf("verdict = %#v, want a failure", verdict)
	}
	if !strings.Contains(verdict.Reason, "/unverified") {
		t.Errorf("verdict reason %q does not name the turn's verdict", verdict.Reason)
	}
	if len(*prompts) != 0 {
		t.Errorf("the judge was asked %d times about a turn the kernel withheld; it can only withhold", len(*prompts))
	}
}

func TestVerifyWithRetry_ARetryCarriesWhyTheAttemptWasNotAccepted(t *testing.T) {
	client, _ := judgeClient(judgeSays)
	exec := &stubTaskExecutor{result: "done", outcomes: []string{"/unverified", "/done"}, missing: []string{"/tests_not_written"}}
	v := newDelegationVerifier(t, client, nil, exec)

	_, verdict, err := v.VerifyWithRetry(context.Background(), delegation("create internal/a/a.go", 2))
	if err != nil {
		t.Fatalf("VerifyWithRetry = %v, want the second, done attempt accepted", err)
	}
	if verdict == nil || !verdict.Success {
		t.Fatalf("verdict = %#v, want success", verdict)
	}
	if exec.calls != 2 {
		t.Fatalf("ran %d attempts, want 2", exec.calls)
	}
	retry := exec.tasks[1]
	if !strings.HasPrefix(retry, "create internal/a/a.go") || !strings.Contains(retry, "/unverified") {
		t.Fatalf("retry task = %q, want the request followed by why the first attempt was not accepted", retry)
	}
}

func TestVerifyWithRetry_TheJudgeWithholdsAtTheConfiguredConfidence(t *testing.T) {
	const unsure = `{"success":false,"confidence":0.3,"reason":"might be incomplete"}`

	t.Run("below the threshold the kernel's done stands", func(t *testing.T) {
		client, _ := judgeClient(unsure)
		exec := &stubTaskExecutor{result: "done", outcome: "/done"}
		v := newDelegationVerifier(t, client, nil, exec)
		reject := 50
		d := delegation("fix the parser", 1)
		d.Params = config.DelegationConfig{JudgeRejectConfidence: &reject}.Params()

		_, verdict, err := v.VerifyWithRetry(context.Background(), d)
		if err != nil {
			t.Fatalf("VerifyWithRetry = %v, want acceptance over a 30%% objection under a threshold of 50", err)
		}
		if !verdict.Success || !strings.Contains(verdict.Reason, "judge_reject_confidence") {
			t.Fatalf("verdict = %#v, want success naming the threshold the objection fell under", verdict)
		}
	})

	t.Run("at the default any failing judgment withholds", func(t *testing.T) {
		client, _ := judgeClient(unsure)
		exec := &stubTaskExecutor{result: "done", outcome: "/done"}
		v := newDelegationVerifier(t, client, nil, exec)

		_, _, err := v.VerifyWithRetry(context.Background(), delegation("fix the parser", 1))
		if !errors.Is(err, ErrMaxRetriesExceeded) {
			t.Fatalf("VerifyWithRetry = %v, want the judge's objection to withhold", err)
		}
	})
}

// The judge's rubric is the kernel's, from the persona: a reviewer's output
// is judged as analysis. It was picked by keywords in the task text.
func TestVerifyWithRetry_TheRubricFollowsThePersona(t *testing.T) {
	for _, tc := range []struct {
		persona string
		want    string
	}{
		{"reviewer", "CODE REVIEW"},
		{"coder", "strict code quality verifier"},
	} {
		t.Run(tc.persona, func(t *testing.T) {
			client, prompts := judgeClient(judgeSays)
			exec := &stubTaskExecutor{result: "done", outcome: "/done"}
			v := newDelegationVerifier(t, client, nil, exec)
			d := delegation("review the parser", 1)
			d.Persona = tc.persona

			if _, _, err := v.VerifyWithRetry(context.Background(), d); err != nil {
				t.Fatal(err)
			}
			if len(*prompts) != 1 || !strings.Contains((*prompts)[0], tc.want) {
				t.Fatalf("the %s was judged with %v, want the rubric containing %q", tc.persona, *prompts, tc.want)
			}
		})
	}
}

// A judgment that does not parse is a judge that could not run: fail closed.
// It used to fall back to a keyword scan for "todo" and "mock", which passed
// anything without those words.
func TestVerifyWithRetry_AMalformedJudgmentFailsClosed(t *testing.T) {
	client, _ := judgeClient("the code looks fine to me")
	exec := &stubTaskExecutor{result: "func Add(a, b int) int { return a + b }", outcome: "/done"}
	v := newDelegationVerifier(t, client, nil, exec)

	_, verdict, err := v.VerifyWithRetry(context.Background(), delegation("implement Add", 3))
	if !errors.Is(err, ErrVerificationUnavailable) {
		t.Fatalf("VerifyWithRetry = %v, want ErrVerificationUnavailable for a malformed judgment", err)
	}
	if verdict == nil || verdict.Success {
		t.Fatalf("verdict = %#v, want a failure", verdict)
	}
	if exec.calls != 1 {
		t.Fatalf("ran %d attempts; a broken judge is not fixed by retrying", exec.calls)
	}
}
