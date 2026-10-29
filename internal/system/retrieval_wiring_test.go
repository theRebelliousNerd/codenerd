package system

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/retrieval"
	"codenerd/internal/types"
)

// Every boot hands the session executor the retrieval pass, and the chat
// shares the same retriever through Cortex.Retriever. Until 2026-09-25 only
// the chat TUI built a retriever, so `nerd fix` never retrieved.
func TestBoot_WiresIssueRetrieverIntoSessionExecutor(t *testing.T) {
	cortex := bootSessionWiringCortex(t, "")
	if cortex.Retriever == nil {
		t.Fatal("Cortex.Retriever is nil: the chat and the executor cannot share a keyword cache")
	}
	if !cortex.SessionExecutor.HasIssueRetriever() {
		t.Fatal("the session executor was booted without the retrieval pass")
	}
}

// The retrieval decisions derive in the production kernel -- the domain-
// sharded Cortex -- and not only in a single-store RealKernel: user_intent is
// shared, the verb table is a program fact, tiered_context_file lands in the
// catch-all shard and config_param is replicated. A split join here would
// derive nothing and every brief would be empty.
func TestRetrievalDecisions_DeriveInTheDomainCortex(t *testing.T) {
	ws := t.TempDir()
	for name, body := range map[string]string{
		"payment_processor.go": "package pay\n\n// ProcessRefund returns a refund.\nfunc ProcessRefund(amount int) int { return amount }\n",
		"refund_ledger.go":     "package pay\n\n// RefundLedger records every ProcessRefund call.\ntype RefundLedger struct{}\n",
	} {
		if err := os.WriteFile(filepath.Join(ws, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	cortex, err := NewDomainCortex(ws)
	if err != nil {
		t.Fatalf("NewDomainCortex: %v", err)
	}
	if err := cortex.Assert(types.Fact{Predicate: "user_intent", Args: []any{
		types.MangleAtom("/task_intent_cortex"), types.MangleAtom("/mutation"), types.MangleAtom("/fix"), "payment_processor.go", "",
	}}); err != nil {
		t.Fatal(err)
	}

	tr := retrieval.NewTaskRetriever(cortex, retrieval.TaskRetrieverConfig{
		WorkDir: ws,
		Params:  config.RetrievalConfig{BriefMinRelevance: 1}.Params(),
	})
	brief, release := tr.Retrieve(context.Background(), "/task_intent_cortex",
		"Fix the rounding bug in payment_processor.go: ProcessRefund returns the wrong amount")
	if !strings.Contains(brief, "payment_processor.go (tier1,") {
		t.Fatalf("the sharded kernel derived no brief for the named file:\n%s", brief)
	}
	if !strings.Contains(brief, "refund_ledger.go") {
		t.Fatalf("the sharded kernel dropped the keyword match above the floor:\n%s", brief)
	}
	release()
	if rows, _ := cortex.Query("tiered_context_file"); len(rows) != 0 {
		t.Fatalf("release left %d tiered_context_file rows in the sharded kernel", len(rows))
	}
}
