package verification

import (
	"codenerd/internal/config"
	"codenerd/internal/observation"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Sweep finding F5: a delegated mutation took an LLM judge's word as its
// verdict. The verifier spawned through the string-only executor, which
// dropped the turn's kernel verdict, so a turn the kernel ended /unverified
// completed the request when the judge answered success. The kernel's verdict
// decides now (delegation_move, policy/delegation.mg); the judge can only
// withhold.

// judgeClient answers every judgment with answer and records each system and
// user prompt it was given.
func judgeClient(answer string) (*stubLLMClient, *[]string) {
	c, prompts, _ := scriptedJudge(answer)
	return c, prompts
}

// scriptedJudge answers judgment n with answers[n] (the last once they run
// out) and records each system and user prompt.
func scriptedJudge(answers ...string) (*stubLLMClient, *[]string, *[]string) {
	var systems, users []string
	return &stubLLMClient{completeWithSystem: func(_ context.Context, system, user string) (string, error) {
		n := len(systems)
		systems = append(systems, system)
		users = append(users, user)
		return answers[min(n, len(answers)-1)], nil
	}}, &systems, &users
}

const judgeSays = `{"contract":"judge/v2","status":"pass","confidence":0.95,"explanation":"looks complete"}`

func TestVerifyWithRetry_AnUnverifiedTurnIsNotAcceptedOnTheJudgesWord(t *testing.T) {
	client, prompts := judgeClient(judgeSays)
	exec := &stubTaskExecutor{result: "Wrote 1 file(s).", outcome: "/unverified", missing: []string{"/tests_not_written"}}
	v := newDelegationVerifier(t, client, nil, exec)

	_, verdict, err := v.VerifyWithRetry(context.Background(), delegation("create internal/a/a.go", 1))
	if !errors.Is(err, ErrMaxRetriesExceeded) {
		t.Fatalf("VerifyWithRetry = %v, want ErrMaxRetriesExceeded for a turn the kernel ended /unverified", err)
	}
	if verdict == nil || verdict.Accepted {
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
	if verdict == nil || !verdict.Accepted {
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

// A grounded defect: cites the worker's own account (E2), which the packet
// still holds.
const groundedDefect = `{"contract":"judge/v2","status":"defect","confidence":0.3,"explanation":"the parser is a stub","violations":["placeholder"],"citations":["E2"]}`

func TestVerifyWithRetry_TheJudgeWithholdsAtTheConfiguredConfidence(t *testing.T) {
	t.Run("below the threshold the kernel's done stands, and the judgment is kept unedited", func(t *testing.T) {
		client, _ := judgeClient(groundedDefect)
		exec := &stubTaskExecutor{result: "done", outcome: "/done"}
		v := newDelegationVerifier(t, client, nil, exec)
		reject := 50
		d := delegation("fix the parser", 1)
		d.Config.JudgeRejectConfidence = &reject

		_, verdict, err := v.VerifyWithRetry(context.Background(), d)
		if err != nil {
			t.Fatalf("VerifyWithRetry = %v, want acceptance over a 30%% defect under a threshold of 50", err)
		}
		if !verdict.Accepted || !strings.Contains(verdict.Reason, "judge_reject_confidence") {
			t.Fatalf("verdict = %#v, want acceptance naming the threshold the defect fell under", verdict)
		}
		if j := verdict.Judgment; j == nil || j.Status != JudgeDefect || j.Explanation != "the parser is a stub" {
			t.Fatalf("judgment = %+v, want the judge's defect as it gave it, beside the disposition", verdict.Judgment)
		}
	})

	t.Run("at the default any grounded defect withholds", func(t *testing.T) {
		client, _ := judgeClient(groundedDefect)
		exec := &stubTaskExecutor{result: "done", outcome: "/done"}
		v := newDelegationVerifier(t, client, nil, exec)

		_, _, err := v.VerifyWithRetry(context.Background(), delegation("fix the parser", 1))
		if !errors.Is(err, ErrMaxRetriesExceeded) {
			t.Fatalf("VerifyWithRetry = %v, want the judge's defect to withhold", err)
		}
	})
}

// A retry after a grounded defect carries the defect and the packet evidence
// it cites, by ID.
func TestVerifyWithRetry_ARetryCarriesTheGroundedDefect(t *testing.T) {
	client, _, _ := scriptedJudge(groundedDefect, judgeSays)
	exec := &stubTaskExecutor{result: "func Parse() {} // TODO", outcome: "/done"}
	v := newDelegationVerifier(t, client, nil, exec)

	if _, _, err := v.VerifyWithRetry(context.Background(), delegation("fix the parser", 2)); err != nil {
		t.Fatalf("VerifyWithRetry = %v, want the second attempt accepted", err)
	}
	retry := exec.tasks[1]
	for _, want := range []string{"the parser is a stub", "placeholder", "[E2] the worker's own account"} {
		if !strings.Contains(retry, want) {
			t.Errorf("retry task lacks %q:\n%s", want, retry)
		}
	}
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
			if len(*prompts) != 1 || !strings.Contains((*prompts)[0], tc.want) || !strings.Contains((*prompts)[0], JudgeContract) {
				t.Fatalf("the %s was judged with %v, want the rubric containing %q and the reply contract", tc.persona, *prompts, tc.want)
			}
		})
	}
}

// Gap A: a reply that decodes is not a judgment. "{}" used to be a failure at
// confidence 0 -- a worker retry under the default threshold, a pass under
// any other -- and an omitted confidence was a genuine 0. Each is a judge that
// could not be had: fail closed, and never pay for a worker rerun.
func TestVerifyWithRetry_AReplyThatBreaksTheContractFailsClosedWithoutARerun(t *testing.T) {
	for name, reply := range map[string]string{
		"prose":              "the code looks fine to me",
		"empty object":       `{}`,
		"null":               `null`,
		"omitted confidence": `{"contract":"judge/v2","status":"defect","explanation":"stub","citations":["E2"]}`,
		"legacy shape":       `{"success":true,"confidence":0.9,"reason":"ok"}`,
	} {
		t.Run(name, func(t *testing.T) {
			client, _ := judgeClient(reply)
			exec := &stubTaskExecutor{result: "func Add(a, b int) int { return a + b }", outcome: "/done"}
			v := newDelegationVerifier(t, client, nil, exec)

			_, verdict, err := v.VerifyWithRetry(context.Background(), delegation("implement Add", 3))
			if !errors.Is(err, ErrVerificationUnavailable) {
				t.Fatalf("VerifyWithRetry = %v, want ErrVerificationUnavailable", err)
			}
			if verdict == nil || verdict.Accepted || verdict.Judgment != nil {
				t.Fatalf("verdict = %#v, want a failure with no judgment", verdict)
			}
			if exec.calls != 1 {
				t.Fatalf("ran %d attempts; a broken judge is not fixed by re-running the worker", exec.calls)
			}
		})
	}
}

// A defect whose citations resolve to nothing is insufficient evidence, not a
// confirmed defect: with nothing the host can expand, the attempt is
// unverifiable -- not a worker retry.
func TestVerifyWithRetry_AnUngroundedDefectIsUnverifiableNotARetry(t *testing.T) {
	client, _ := judgeClient(`{"contract":"judge/v2","status":"defect","confidence":0.9,"explanation":"hallucinated import","violations":["hallucinated_api"],"citations":["E99"]}`)
	exec := &stubTaskExecutor{result: "done", outcome: "/done"}
	v := newDelegationVerifier(t, client, nil, exec)

	_, verdict, err := v.VerifyWithRetry(context.Background(), delegation("add the importer", 3))
	if !errors.Is(err, ErrVerificationUnavailable) || !strings.Contains(err.Error(), "insufficient evidence") {
		t.Fatalf("VerifyWithRetry = %v, want unverifiable for insufficient evidence", err)
	}
	if exec.calls != 1 {
		t.Fatalf("ran %d attempts for a judge that could not ground its defect", exec.calls)
	}
	if j := verdict.Judgment; j == nil || j.Claimed != JudgeDefect || j.Status != JudgeInsufficientEvidence || !strings.Contains(j.Downgraded, "E99") {
		t.Fatalf("judgment = %+v, want the claimed defect kept and downgraded for its unresolved citation", verdict.Judgment)
	}
}

// changedFile writes content at dir/name and returns the observed write the
// attempt made: absent before, content after.
func changedFile(t *testing.T, dir, name, content string) observation.FileWrite {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return observation.FileWrite{Path: path, Before: observation.FileState{Known: true}, After: observation.FileState{Known: true, Exists: true, Content: content}}
}

// Policy-mediated evidence acquisition: the judge asks for an item the packet
// elided, the host supplies it in a second round over the same packet, and
// the worker runs once.
func TestVerifyWithRetry_TheJudgeMayAskForElidedEvidence(t *testing.T) {
	dir := t.TempDir()
	big := "package a\n\n" + strings.Repeat("// filler line to push the diff past the evidence budget\n", 200) + "func Real() int { return 42 }\n"
	write := changedFile(t, dir, "a.go", big)
	client, _, users := scriptedJudge(
		`{"contract":"judge/v2","status":"insufficient_evidence","confidence":0.5,"explanation":"the diff is cut before the function","requested_evidence":["E2"]}`,
		judgeSays,
	)
	exec := &stubTaskExecutor{result: "implemented Real", outcome: "/done", writes: []observation.FileWrite{write}, changed: []string{"a.go"}}
	v := newDelegationVerifier(t, client, nil, exec)
	d := delegation("implement Real", 2)
	budget := 12000 // the diff (~11.8 KB) overflows its share, the content (~11.4 KB) fits an expansion
	d.Config.JudgeEvidenceBytes = &budget

	_, verdict, err := v.VerifyWithRetry(context.Background(), d)
	if err != nil {
		t.Fatalf("VerifyWithRetry = %v, want acceptance after the requested evidence", err)
	}
	if !verdict.Accepted || exec.calls != 1 || len(*users) != 2 {
		t.Fatalf("accepted=%t worker runs=%d judge rounds=%d, want accepted, 1, 2", verdict.Accepted, exec.calls, len(*users))
	}
	if !strings.Contains((*users)[0], "request E2 for the full post-change content") {
		t.Fatalf("first round did not offer the elided change:\n%s", (*users)[0])
	}
	if second := (*users)[1]; !strings.Contains(second, "## Requested evidence") || !strings.Contains(second, "func Real() int { return 42 }") {
		t.Fatalf("second round lacks the requested content:\n%s", second)
	}

	t.Run("with no evidence rounds the attempt is unverifiable", func(t *testing.T) {
		client, _, users := scriptedJudge(`{"contract":"judge/v2","status":"insufficient_evidence","confidence":0.5,"explanation":"cut","requested_evidence":["E2"]}`)
		exec := &stubTaskExecutor{result: "implemented Real", outcome: "/done", writes: []observation.FileWrite{write}, changed: []string{"a.go"}}
		v := newDelegationVerifier(t, client, nil, exec)
		d := delegation("implement Real", 2)
		d.Config.JudgeEvidenceBytes = &budget
		none := 0
		d.Config.JudgeEvidenceRounds = &none
		if _, _, err := v.VerifyWithRetry(context.Background(), d); !errors.Is(err, ErrVerificationUnavailable) {
			t.Fatalf("VerifyWithRetry = %v, want unverifiable", err)
		}
		if exec.calls != 1 || len(*users) != 1 {
			t.Fatalf("worker runs=%d judge rounds=%d, want 1 and 1", exec.calls, len(*users))
		}
	})
}

// A citation of a file the workspace no longer holds as the attempt left it
// describes a superseded revision: stale, never a confirmed defect.
func TestVerifyWithRetry_AStaleCitationIsNotADefect(t *testing.T) {
	dir := t.TempDir()
	write := changedFile(t, dir, "b.go", "package b\n\nfunc B() {}\n")
	exec := &stubTaskExecutor{result: "done", outcome: "/done", writes: []observation.FileWrite{write}, changed: []string{"b.go"}}
	client := &stubLLMClient{completeWithSystem: func(context.Context, string, string) (string, error) {
		// A concurrent writer replaces the file while the judge reads.
		if err := os.WriteFile(write.Path, []byte("package b\n\nfunc B() { panic(1) }\n"), 0o644); err != nil {
			return "", err
		}
		return `{"contract":"judge/v2","status":"defect","confidence":0.9,"explanation":"B is empty","violations":["empty_function"],"citations":["E2"]}`, nil
	}}
	v := newDelegationVerifier(t, client, nil, exec)

	_, verdict, err := v.VerifyWithRetry(context.Background(), delegation("implement B", 3))
	if !errors.Is(err, ErrVerificationUnavailable) {
		t.Fatalf("VerifyWithRetry = %v, want unverifiable, not a retry over a stale citation", err)
	}
	if j := verdict.Judgment; j == nil || j.Status != JudgeInsufficientEvidence || len(j.Ungrounded) != 1 {
		t.Fatalf("judgment = %+v, want the stale citation ungrounded", verdict.Judgment)
	}
	if exec.calls != 1 {
		t.Fatalf("ran %d attempts", exec.calls)
	}
}

// The judge sees the typed evidence, labeled by provenance, not the prose
// alone.
func TestVerifyWithRetry_TheJudgeReadsTheEvidencePacket(t *testing.T) {
	dir := t.TempDir()
	write := changedFile(t, dir, "c.go", "package c\n\nfunc C() int { return 1 }\n")
	client, _, users := scriptedJudge(judgeSays)
	exec := &stubTaskExecutor{result: "all tests pass", outcome: "/done", writes: []observation.FileWrite{write}, changed: []string{"c.go"}, snapshot: "snap-123"}
	v := newDelegationVerifier(t, client, nil, exec)

	if _, _, err := v.VerifyWithRetry(context.Background(), delegation("implement C", 1)); err != nil {
		t.Fatal(err)
	}
	got := (*users)[0]
	for _, want := range []string{
		"[E1] source=kernel kind=verdict: turn verdict /done",
		"Workspace snapshot: snap-123",
		"source=observed kind=change: changed c.go (before absent, after sha256:",
		"+func C() int { return 1 }",
		"source=reported kind=narrative: the worker's own account",
		"all tests pass",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("judge prompt lacks %q:\n%s", want, got)
		}
	}
}

func TestRetryTask_WithoutAJudgmentCarriesTheTurnsReason(t *testing.T) {
	got := retryTask("fix the code", &VerificationResult{Reason: "the turn ended /unverified: the tests were not verified green"})
	if !strings.Contains(got, "Previous Attempt Not Accepted") || !strings.Contains(got, "tests were not verified green") {
		t.Fatalf("retry task = %q", got)
	}
}

// The judge's corrective action reaches the retry as advice. Until 2026-09-23
// the verifier ran it on the judge's word, and the judge can only withhold (F5).
func TestRetryTask_TheJudgesCorrectiveActionIsAdviceNotAnAction(t *testing.T) {
	r := &VerificationResult{
		Reason: "the judge grounded a defect",
		Judgment: &Judgment{Status: JudgeDefect, Explanation: "no such API", Violations: []QualityViolation{HallucinatedAPI},
			CorrectiveAction: &CorrectiveAction{Type: CorrectiveTool, Query: "a rod page scraper", Reason: "no tool reads the page"}},
	}
	got := retryTask("scrape the page", r)
	for _, want := range []string{"Suggested Next Step", "tool: a rod page scraper", "no tool reads the page", "hallucinated_api"} {
		if !strings.Contains(got, want) {
			t.Errorf("retry task does not carry %q:\n%s", want, got)
		}
	}
}

func TestStoreVerification_WhenNoLocalDB_ShouldNotPanic(t *testing.T) {
	v := NewTaskVerifier(nil, nil, nil, config.JITConfig{})
	v.storeVerification(delegation("task", 1), &VerificationResult{Accepted: true, Reason: "ok"}, 1, nil)
}
