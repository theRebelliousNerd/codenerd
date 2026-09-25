package retrieval

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/types"
)

func newDecisionKernel(t *testing.T) *core.RealKernel {
	t.Helper()
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	return k
}

func assertIntent(t *testing.T, k *core.RealKernel, id, category, verb, target string) {
	t.Helper()
	if err := k.Assert(types.Fact{Predicate: "user_intent", Args: []any{
		types.MangleAtom(id), types.MangleAtom(category), types.MangleAtom(verb), target, "",
	}}); err != nil {
		t.Fatalf("assert user_intent: %v", err)
	}
}

// writeRefundWorkspace is a workspace where the issue names one file and the
// keyword search finds another.
func writeRefundWorkspace(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"payment_processor.go": "package pay\n\n// ProcessRefund returns a refund.\nfunc ProcessRefund(amount int) int { return amount }\n",
		"refund_ledger.go":     "package pay\n\n// RefundLedger records every ProcessRefund call.\ntype RefundLedger struct{}\n\nfunc (RefundLedger) Record() { _ = ProcessRefund(1) }\n",
		"unrelated.go":         "package pay\n\nfunc Greeting() string { return \"hello\" }\n",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

const refundIssue = "Fix the rounding bug in payment_processor.go: ProcessRefund returns the wrong amount"

// The kernel, not a Go verb switch, decides whether a turn retrieves.
func TestWanted_IsDerivedFromTheIntentVerb(t *testing.T) {
	k := newDecisionKernel(t)
	assertIntent(t, k, "/task_intent_fix", "/mutation", "/fix", "payment_processor.go")
	assertIntent(t, k, "/task_intent_explain", "/query", "/explain", "payment_processor.go")

	if ok, err := Wanted(k, "/task_intent_fix"); err != nil || !ok {
		t.Fatalf("Wanted(/fix intent) = %v, %v; want true", ok, err)
	}
	if ok, err := Wanted(k, "/task_intent_explain"); err != nil || ok {
		t.Fatalf("Wanted(/explain intent) = %v, %v; want false", ok, err)
	}
	if ok, _ := Wanted(k, "/no_such_intent"); ok {
		t.Fatal("an intent the kernel never saw was wanted")
	}
}

// The brief is what retrieval_brief_file derives: the named file always, a
// searched file only when its relevance clears the configured floor. Moving
// the floor moves the brief, which is how the test tells a kernel decision
// from a Go one.
func TestTaskRetriever_BriefFollowsTheKernelsFloor(t *testing.T) {
	dir := writeRefundWorkspace(t)
	for _, tc := range []struct {
		name       string
		floor      int
		wantLedger bool
	}{
		{name: "floor above every search score hands over only the named file", floor: 101, wantLedger: false},
		{name: "floor at 1 hands over the keyword match too", floor: 1, wantLedger: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			k := newDecisionKernel(t)
			assertIntent(t, k, "/task_intent_1", "/mutation", "/fix", "payment_processor.go")
			tr := NewTaskRetriever(k, TaskRetrieverConfig{
				WorkDir: dir,
				Params:  config.RetrievalConfig{BriefMinRelevance: tc.floor}.Params(),
			})
			brief, release := tr.Retrieve(context.Background(), "/task_intent_1", refundIssue)
			defer release()

			if !strings.Contains(brief, "payment_processor.go (tier1,") {
				t.Fatalf("the file the issue named is not in the brief:\n%s", brief)
			}
			if got := strings.Contains(brief, "refund_ledger.go"); got != tc.wantLedger {
				t.Fatalf("refund_ledger.go in brief = %v, want %v at floor %d:\n%s", got, tc.wantLedger, tc.floor, brief)
			}
			if strings.Contains(brief, "unrelated.go") {
				t.Fatalf("a file with no keyword hit reached the brief:\n%s", brief)
			}
		})
	}
}

// A turn the kernel does not want retrieved runs no pass and asserts nothing.
func TestTaskRetriever_UnwantedIntentRunsNoPass(t *testing.T) {
	k := newDecisionKernel(t)
	assertIntent(t, k, "/task_intent_2", "/query", "/explain", "payment_processor.go")
	tr := NewTaskRetriever(k, TaskRetrieverConfig{WorkDir: writeRefundWorkspace(t), Params: config.DefaultRetrievalConfig().Params()})

	brief, release := tr.Retrieve(context.Background(), "/task_intent_2", refundIssue)
	release()
	if brief != "" {
		t.Fatalf("brief for an unwanted intent: %q", brief)
	}
	if rows, _ := k.Query("issue_text"); len(rows) != 0 {
		t.Fatalf("an unwanted intent seeded %d issue_text rows", len(rows))
	}
}

// A task's pass owns its facts: it asserts only the issue-keyed half, and the
// release takes back exactly those, leaving another issue's evidence alone.
func TestTaskRetriever_ReleaseRetractsOnlyItsOwnIssue(t *testing.T) {
	dir := writeRefundWorkspace(t)
	k := newDecisionKernel(t)

	// Another live issue, seeded the way the chat seeds.
	if _, err := SeedIssueFacts(context.Background(), k, SeedRequest{IssueID: "/chat_issue", IssueText: refundIssue, WorkDir: dir}); err != nil {
		t.Fatal(err)
	}
	candidatesBefore, _ := k.Query("candidate_file")
	if len(candidatesBefore) == 0 {
		t.Fatal("fixture: the chat pass found no candidates")
	}

	assertIntent(t, k, "/task_intent_3", "/mutation", "/fix", "payment_processor.go")
	tr := NewTaskRetriever(k, TaskRetrieverConfig{WorkDir: dir, Params: config.DefaultRetrievalConfig().Params()})
	brief, release := tr.Retrieve(context.Background(), "/task_intent_3", refundIssue)
	if brief == "" {
		t.Fatal("no brief for a /fix task naming a file")
	}
	if rows, _ := k.Query("candidate_file"); len(rows) != len(candidatesBefore) {
		t.Fatalf("a scoped pass asserted unscoped facts: candidate_file %d -> %d", len(candidatesBefore), len(rows))
	}
	release()

	issues := map[string]bool{}
	rows, _ := k.Query("tiered_context_file")
	for _, row := range rows {
		issues[types.ExtractString(row.Args[0])] = true
	}
	if len(issues) != 1 || !issues["/chat_issue"] {
		t.Fatalf("after release the tiered files belong to %v, want only /chat_issue", issues)
	}
	if rows, _ := k.Query("candidate_file"); len(rows) != len(candidatesBefore) {
		t.Fatalf("release took another issue's candidates: %d -> %d", len(candidatesBefore), len(rows))
	}
}

// The chat has one live issue: seeding it again replaces the earlier pass,
// unscoped half included, instead of piling a second turn's candidates on the
// first's.
func TestSupersedeIssue_LeavesOnlyTheCurrentPass(t *testing.T) {
	dir := writeRefundWorkspace(t)
	k := newDecisionKernel(t)
	if _, err := SeedIssueFacts(context.Background(), k, SeedRequest{IssueID: "/chat_issue", IssueText: refundIssue, WorkDir: dir}); err != nil {
		t.Fatal(err)
	}
	if err := SupersedeIssue(k, "/chat_issue"); err != nil {
		t.Fatal(err)
	}
	for _, predicate := range []string{"issue_text", "issue_keyword", "tiered_context_file", "issue_context", "file_mentioned", "candidate_file", "keyword_hit", "context_tier", "keyword_weight"} {
		if rows, _ := k.Query(predicate); len(rows) != 0 {
			t.Errorf("%s: %d rows survived SupersedeIssue", predicate, len(rows))
		}
	}
}
