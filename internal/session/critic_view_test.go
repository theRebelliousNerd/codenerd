package session

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestChangedSpans(t *testing.T) {
	before := "a\nb\nc\nd\ne\n"
	for _, tc := range []struct {
		name  string
		after string
		want  []criticSpan
	}{
		{"unchanged", before, nil},
		{"one line edited", "a\nb\nC\nd\ne\n", []criticSpan{{3, 3}}},
		{"lines inserted", "a\nb\nx\ny\nc\nd\ne\n", []criticSpan{{3, 4}}},
		{"a line deleted is anchored where it was", "a\nb\nd\ne\n", []criticSpan{{3, 3}}},
		{"two separate hunks", "A\nb\nc\nd\nE\n", []criticSpan{{1, 1}, {5, 5}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := changedSpans(before, tc.after)
			if fmt.Sprint(got) != fmt.Sprint(tc.want) {
				t.Errorf("changedSpans = %v, want %v", got, tc.want)
			}
		})
	}
	if got := changedSpans("", "x\ny\n"); fmt.Sprint(got) != fmt.Sprint([]criticSpan{{1, 2}}) {
		t.Errorf("a new file = %v, want all of it", got)
	}
}

// bigGoFile is a Go file well past criticMaxFileBytes whose last function is
// the one a turn edits.
func bigGoFile(tail string) string {
	var b strings.Builder
	b.WriteString("package p\n\n")
	for i := 0; b.Len() < 2*criticMaxFileBytes; i++ {
		fmt.Fprintf(&b, "func filler%d() int {\n\treturn %d\n}\n\n", i, i)
	}
	b.WriteString("func Target(a, b int) int {\n\t" + tail + "\n}\n")
	return b.String()
}

// Until 2026-09-26 the critic read the first criticMaxFileBytes bytes of each
// file. An edit past that point was reviewed by a critic that could not see it.
func TestCriticFileView_WhenALargeFileIsEditedPastTheCap_ShouldShowTheEditedFunction(t *testing.T) {
	before := bigGoFile("return a + b")
	after := bigGoFile("return a - b")
	view, windows := criticFileView("p.go", after, changedSpans(before, after))

	if !strings.Contains(view, "return a - b") || !strings.Contains(view, "func Target(a, b int) int {") {
		t.Fatalf("the view does not show the edited function:\n%s", view[max(0, len(view)-400):])
	}
	if strings.Contains(view, "func filler0()") {
		t.Error("the view shows the head of the file, which the turn did not change")
	}
	if len(view) > criticMaxFileBytes {
		t.Errorf("view is %d bytes, want within %d", len(view), criticMaxFileBytes)
	}
	target := strings.Count(after[:strings.Index(after, "func Target")], "\n") + 1
	if !lineInSpans(target, windows) {
		t.Errorf("windows %v do not cover the edited function at line %d", windows, target)
	}
}

func TestCriticFileView_WhenAFileIsSmall_ShouldShowItWholeAndNumbered(t *testing.T) {
	content := "package p\n\nfunc A() {}\n"
	view, _ := criticFileView("a.go", content, []criticSpan{{3, 3}})
	for _, want := range []string{"    1| package p", "    3| func A() {}"} {
		if !strings.Contains(view, want) {
			t.Errorf("view is missing %q:\n%s", want, view)
		}
	}
}

func TestFindingsOnChange(t *testing.T) {
	windows := map[string][]criticSpan{"internal/p/a.go": {{10, 20}}}
	findings := []CriticFinding{
		{File: "internal/p/a.go", Line: 15, Severity: "high", Claim: "inside"},
		{File: "a.go", Line: 12, Severity: "high", Claim: "inside, path written shorter"},
		{File: "internal/p/a.go", Line: 400, Severity: "high", Claim: "untouched code"},
		{File: "other.go", Line: 1, Severity: "high", Claim: "a file the review cannot place"},
	}
	kept, dropped := findingsOnChange(findings, windows)
	if len(kept) != 3 || len(dropped) != 1 || dropped[0].Claim != "untouched code" {
		t.Errorf("kept %v, dropped %v; want only the finding on untouched code dropped", kept, dropped)
	}
}

// R1-11's review reported 3 of its 4 findings in code the change did not
// touch. A finding outside the changed windows is recorded but does not buy an
// uplift round, and the review is told what the change was for.
func TestVerifyAndUpliftWithCritic_WhenTheFindingIsInUntouchedCode_ShouldNotFireUplift(t *testing.T) {
	ws := t.TempDir()
	var before strings.Builder
	before.WriteString("package p\n")
	for i := 0; i < 60; i++ {
		fmt.Fprintf(&before, "\nfunc F%d() int { return %d }\n", i, i)
	}
	after := strings.Replace(before.String(), "func F59() int { return 59 }", "func F59() int { return -59 }", 1)
	if err := os.WriteFile(filepath.Join(ws, "a.go"), []byte(after), 0o644); err != nil {
		t.Fatal(err)
	}

	critic := &scriptedCriticLLM{review: "FINDING a.go:3 high: F0 has no doc comment"}
	trp := &recordingToolResults{}
	cfg := DefaultExecutorConfig()
	cfg.WorkspaceRoot = ws
	e := &Executor{config: cfg, llmClient: critic}
	result := &ExecutionResult{
		SuccessfulWriteTools: 1, WrittenPaths: []string{"a.go"},
		PreWriteContents: map[string]PreImage{
			canonicalizeWrittenPath("a.go", ws): {Existed: true, Content: before.String()},
		},
	}
	if _, err := e.verifyAndUpliftWithCritic(context.Background(), trp, "sys", nil, nil, nil, result); err != nil {
		t.Fatalf("verifyAndUpliftWithCritic: %v", err)
	}
	if trp.calls != 0 {
		t.Errorf("uplift fired %d time(s) for a finding in untouched code; want 0", trp.calls)
	}
	if len(result.CriticFindings) != 1 {
		t.Errorf("the finding was not recorded: %+v", result.CriticFindings)
	}
}

func TestBuildCriticPrompt_WhenGivenTheRequest_ShouldAskTheReviewToJudgeAgainstIt(t *testing.T) {
	p := buildCriticPrompt(map[string]string{"a.go": "    1| package p\n"}, nil, "", "make Add handle overflow")
	if !strings.Contains(p, "make Add handle overflow") || !strings.Contains(p, "does not do what was asked") {
		t.Errorf("the prompt does not carry the request and the instruction to judge against it:\n%s", p)
	}
	if strings.Contains(buildCriticPrompt(map[string]string{"a.go": "x"}, nil, "", ""), "does not do what was asked") {
		t.Error("with no request, the prompt must not ask to judge against one")
	}
}

// The gate tests every importer of what the turn wrote; a failed run's page of
// "ok" lines is dropped, and the failure is kept whole.
func TestWithoutPassingPackages(t *testing.T) {
	out := "ok  \tcodenerd/a\t0.12s\n--- FAIL: TestX (0.00s)\n    x_test.go:9: want 2, got 1\nFAIL\nFAIL\tcodenerd/b\t0.30s\n?   \tcodenerd/c\t[no test files]\nok  \tcodenerd/d\t(cached)"
	got := withoutPassingPackages(out)
	for _, want := range []string{"--- FAIL: TestX", "want 2, got 1", "FAIL\tcodenerd/b", "(3 other package(s) passed or have no tests)"} {
		if !strings.Contains(got, want) {
			t.Errorf("projection lost %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "codenerd/a") || strings.Contains(got, "codenerd/d") {
		t.Errorf("a passing package's line survived:\n%s", got)
	}
	if names := topLevelFailedTests(got); len(names) != 1 || names[0] != "TestX" {
		t.Errorf("the failing test is no longer parseable from the projection: %v", names)
	}
	if clean := "--- FAIL: TestY\nFAIL"; withoutPassingPackages(clean) != clean {
		t.Error("output with no passing packages must come back unchanged")
	}
}
