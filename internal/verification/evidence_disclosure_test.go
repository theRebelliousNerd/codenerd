package verification

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/observation"
)

// V2: an expansion was the first judge_evidence_bytes of the file every time,
// so a decisive function past that prefix was unreachable and a repeated
// request re-sent the same prefix. Each request now discloses the next range.
func TestVerifyWithRetry_RepeatedRequestsReachEvidencePastTheFirstRange(t *testing.T) {
	dir := t.TempDir()
	var b strings.Builder
	b.WriteString("package g\n\n")
	for i := 0; i < 600; i++ {
		fmt.Fprintf(&b, "// filler line %04d keeps the decisive function far from the top\n", i)
	}
	b.WriteString("func Decisive() int { return 42 }\n")
	write := changedFile(t, dir, "g.go", b.String())
	ask := `{"contract":"judge/v2","status":"insufficient_evidence","confidence":0.5,"explanation":"not reached yet","requested_evidence":["E2"]}`
	var users []string
	client := &stubLLMClient{completeWithSystem: func(_ context.Context, _, user string) (string, error) {
		users = append(users, user)
		if strings.Contains(user, "func Decisive() int { return 42 }") {
			return judgeSays, nil
		}
		return ask, nil
	}}
	exec := &stubTaskExecutor{result: "implemented Decisive", outcome: "/done", writes: []observation.FileWrite{write}, changed: []string{"g.go"}, workspace: dir}
	v := newDelegationVerifier(t, client, nil, exec)
	d := delegation("implement Decisive", 1)
	budget := 16000 // the file is ~41 KB: three ranges reach its end
	rounds := 5
	d.Config.JudgeEvidenceBytes = &budget
	d.Config.JudgeEvidenceRounds = &rounds

	_, verdict, err := v.VerifyWithRetry(context.Background(), d)
	if err != nil || !verdict.Accepted {
		t.Fatalf("VerifyWithRetry = %v, want acceptance once the decisive range is disclosed", err)
	}
	if len(users) < 3 {
		t.Fatalf("%d judge rounds; the decisive function lies past the first range", len(users))
	}
	last := users[len(users)-1]
	// Admission and disclosures walk the same diff, including its prefix.
	for _, line := range []string{"\n+// filler line 0000 ", "\n+// filler line 0300 ", "\n+// filler line 0599 "} {
		if n := strings.Count(last, line); n != 1 {
			t.Fatalf("%q is disclosed %d times in the last request; no range may be disclosed twice", line, n)
		}
	}
	if !strings.Contains(users[1], "request E2 again for the next range") {
		t.Fatalf("the first disclosure does not say more remains:\n%s", users[1][len(users[1])-400:])
	}
}

// A request for an item already shown in full is said to be unsuppliable, not
// answered with a repeat; with nothing else to supply the attempt is
// unverifiable and the reason names why.
func TestVerifyWithRetry_ARequestThatCannotProgressIsNamed(t *testing.T) {
	dir := t.TempDir()
	write := changedFile(t, dir, "h.go", "package h\n\nfunc H() {}\n")
	client, _ := judgeClient(`{"contract":"judge/v2","status":"insufficient_evidence","confidence":0.5,"explanation":"need more","requested_evidence":["E2","E99"]}`)
	exec := &stubTaskExecutor{result: "done", outcome: "/done", writes: []observation.FileWrite{write}, workspace: dir}
	v := newDelegationVerifier(t, client, nil, exec)

	_, verdict, err := v.VerifyWithRetry(context.Background(), delegation("implement H", 1))
	if !errors.Is(err, ErrVerificationUnavailable) {
		t.Fatalf("VerifyWithRetry = %v, want unverifiable", err)
	}
	for _, want := range []string{"E2: the packet already shows all of it", "E99: not in the packet"} {
		if !strings.Contains(verdict.Reason, want) {
			t.Errorf("reason %q lacks %q", verdict.Reason, want)
		}
	}
}

func TestExpand_DisclosesEachRangeOnceAndThenStops(t *testing.T) {
	dir := t.TempDir()
	content := strings.Repeat("line of the post-change file\n", 400) // ~11.6 KB
	path := filepath.Join(dir, "i.go")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	ret := observation.Return{Outcome: "/done", Writes: []observation.FileWrite{{Path: path, Before: observation.FileState{Known: true}, After: observation.FileState{Known: true, Exists: true, Content: content}}}}
	p, err := buildEvidencePacket(context.Background(), 1, "/implementation", ret, 4096)
	if err != nil {
		t.Fatal(err)
	}
	shown := p.Items[1].Body
	for i := 0; p.expandable("E2"); i++ {
		if i > 10 {
			t.Fatal("the item never stopped being expandable")
		}
		got := p.expand([]string{"E2", "E2"})
		if len(got) > p.Budget {
			t.Fatalf("disclosure %d is %d bytes, over the %d budget", i, len(got), p.Budget)
		}
		parts := strings.SplitN(got, "\n```\n", 3)
		if len(parts) != 3 {
			t.Fatalf("disclosure has no framed range: %q", got)
		}
		shown += parts[1]
	}
	if want := renderDiff(path, "", content); shown != want {
		t.Fatalf("admission plus disclosures do not reconstruct the retained diff: got %d bytes, want %d", len(shown), len(want))
	}
	if why := p.unexpandable("E2"); why != "every byte of it has been shown" {
		t.Fatalf("unexpandable = %q", why)
	}
}

// V3: the budget bounded the bodies only; check details, failing-test lists,
// findings, notes and every label were added outside it.
func TestEvidencePacket_TheWholeRenderedPacketFitsTheBudget(t *testing.T) {
	var findings []observation.Finding
	for i := 0; i < 40; i++ {
		findings = append(findings, observation.Finding{File: "x.go", Line: i, Severity: "warn", Message: strings.Repeat("a long finding message ", 30)})
	}
	var failing []string
	for i := 0; i < 300; i++ {
		failing = append(failing, fmt.Sprintf("TestSomethingLong%03d", i))
	}
	ret := observation.Return{
		Outcome: "/done", Output: strings.Repeat("narrative ", 3000), Stage: "checks_passed",
		Acceptance: &observation.Acceptance{Status: "verified", Contract: "c1", Summary: strings.Repeat("summary ", 2000)},
		Build:      &observation.Verification{Kind: "build", Source: observation.SourceObserved, Ran: true, OK: true, Detail: strings.Repeat("build output\n", 2000)},
		Tests:      &observation.Verification{Kind: "tests", Source: observation.SourceObserved, Ran: true, Failed: 300, FailedNames: failing},
		Findings:   findings,
		Untested:   []string{"a.go", "b.go"},
		Notes:      []string{strings.Repeat("note ", 2000)},
	}
	const task = "the task"
	p, err := buildEvidencePacket(context.Background(), 1, "/implementation", ret, 8192)
	if err != nil {
		t.Fatal(err)
	}
	rendered := p.render(task)
	if got := len(rendered) - len("## Task\n"+task); got > p.Budget {
		t.Fatalf("the rendered packet is %d bytes, over its %d budget", got, p.Budget)
	}
	for _, it := range p.Items {
		if it.Omitted > 0 && !p.expandable(it.ID) {
			t.Errorf("%s (%s) omits %d bytes the judge cannot request", it.ID, it.Kind, it.Omitted)
		}
	}

	t.Run("labels that cannot fit are refused, not cut", func(t *testing.T) {
		if _, err := buildEvidencePacket(context.Background(), 1, "/implementation", ret, 2048); err == nil || !strings.Contains(err.Error(), "no label is cut") {
			t.Fatalf("buildEvidencePacket = %v, want refused", err)
		}
	})
}
