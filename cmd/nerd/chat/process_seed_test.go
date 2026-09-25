package chat

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/perception"
	"codenerd/internal/types"
)

// The chat's issue seed is gated by the kernel (issue_retrieval_wanted over
// /current_intent), not by a Go switch on the verb, and it keeps one live chat
// issue: a second /fix turn replaces the first turn's evidence.
func TestSeedIssueFacts_KernelGatesAndOneIssueStaysLive(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{
		"ledger.go":  "package pay\n\nfunc ProcessRefund(amount int) int { return amount }\n",
		"invoice.go": "package pay\n\nfunc IssueInvoice() {}\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	m := NewTestModel()
	m.kernel = k
	m.workspace = dir

	setIntent := func(verb string) {
		t.Helper()
		_ = k.RetractFact(core.Fact{Predicate: "user_intent", Args: []any{"/current_intent"}})
		if err := k.Assert(core.Fact{Predicate: "user_intent", Args: []any{"/current_intent", "/mutation", verb, "", ""}}); err != nil {
			t.Fatal(err)
		}
	}
	issueRows := func() []types.Fact {
		t.Helper()
		rows, err := k.Query("issue_text")
		if err != nil {
			t.Fatal(err)
		}
		return rows
	}

	// /explain is not an issue verb in the kernel's table: no pass.
	setIntent("/explain")
	m.seedIssueFacts(context.Background(), perception.Intent{Verb: "/explain"}, "explain ledger.go ProcessRefund")
	if rows := issueRows(); len(rows) != 0 {
		t.Fatalf("an /explain turn seeded %d issue_text rows", len(rows))
	}

	// The kernel's answer is what gates, not the verb handed in: the intent
	// struct says /explain while the kernel holds /fix.
	setIntent("/fix")
	m.seedIssueFacts(context.Background(), perception.Intent{Verb: "/explain"}, "fix ledger.go ProcessRefund rounding")
	if rows := issueRows(); len(rows) != 1 {
		t.Fatalf("a /fix turn seeded %d issue_text rows, want 1", len(rows))
	}

	m.seedIssueFacts(context.Background(), perception.Intent{Verb: "/fix"}, "fix invoice.go IssueInvoice")
	rows := issueRows()
	if len(rows) != 1 {
		t.Fatalf("after a second /fix turn the kernel holds %d issues, want the one live chat issue", len(rows))
	}
	if got := types.ExtractString(rows[0].Args[1]); got != "fix invoice.go IssueInvoice" {
		t.Fatalf("live chat issue is %q, want the second turn's", got)
	}
	mentions, _ := k.Query("file_mentioned")
	for _, f := range mentions {
		if types.ExtractString(f.Args[0]) == "ledger.go" {
			t.Fatal("the first turn's mention of ledger.go survived the second turn's seed")
		}
	}
}
