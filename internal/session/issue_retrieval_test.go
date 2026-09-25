package session

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/perception"
	"codenerd/internal/retrieval"
	"codenerd/internal/types"
)

// briefCapturingProvider answers every call with a final text and keeps the
// histories it was sent, so a test sees exactly what the model saw.
type briefCapturingProvider struct {
	*MockLLMClient
	histories [][]types.Message
}

func (p *briefCapturingProvider) CompleteWithToolResults(_ context.Context, _ string, history []types.Message, _ []types.ToolDefinition) (*types.LLMToolResponse, error) {
	p.histories = append(p.histories, append([]types.Message(nil), history...))
	return &types.LLMToolResponse{Text: "done"}, nil
}

// firstUserText is the text of the first user turn of the first request.
func (p *briefCapturingProvider) firstUserText(t *testing.T) string {
	t.Helper()
	if len(p.histories) == 0 {
		t.Fatal("the model was never called")
	}
	for _, m := range p.histories[0] {
		if m.Role == "user" {
			return m.Text
		}
	}
	t.Fatal("the first request carried no user turn")
	return ""
}

// retrievalTurn runs one production-shaped ProcessWithIntent turn -- a real
// kernel, a working loop over a real workspace, the production retriever --
// and returns what the model was sent and the kernel afterwards.
func retrievalTurn(t *testing.T, verb, category, task string, withRetriever bool) (*briefCapturingProvider, *core.RealKernel) {
	t.Helper()
	root := t.TempDir()
	for name, body := range map[string]string{
		"payment_processor.go": "package pay\n\n// ProcessRefund returns a refund.\nfunc ProcessRefund(amount int) int { return amount }\n",
		"refund_ledger.go":     "package pay\n\n// RefundLedger records every ProcessRefund call.\ntype RefundLedger struct{}\n",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	client := &briefCapturingProvider{MockLLMClient: &MockLLMClient{}}
	executor := NewExecutor(kernel, &testExecutiveStore{}, client, &MockJITCompiler{}, journeyConfigFactory(), journeyIntent(verb, category))
	cfg := journeyConfig(t)
	cfg.WorkspaceRoot = root
	executor.SetConfig(cfg)
	if withRetriever {
		executor.SetIssueRetriever(retrieval.NewTaskRetriever(kernel, retrieval.TaskRetrieverConfig{
			WorkDir: root,
			Params:  config.DefaultRetrievalConfig().Params(),
		}))
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	// A preset intent is how a delegated task and `nerd fix` reach the
	// executor (task_executor.go); the turn's outcome is not under test.
	if _, err := executor.ProcessWithIntent(ctx, task, &perception.Intent{Verb: verb, Category: category, Target: "payment_processor.go"}); err != nil {
		t.Logf("turn error (not under test): %v", err)
	}
	return client, kernel
}

// A /fix task turn is handed the files the kernel selected from its retrieval
// pass, in the task the model reads first; the pass's facts leave with the
// turn. Before 2026-09-25 no executor turn ran a pass at all.
func TestProcessWithIntent_FixTaskIsHandedTheKernelsRetrievalBrief(t *testing.T) {
	const task = "Fix the rounding bug in payment_processor.go: ProcessRefund returns the wrong amount"
	client, kernel := retrievalTurn(t, "/fix", "/mutation", task, true)

	first := client.firstUserText(t)
	if !strings.HasPrefix(first, task) {
		t.Fatalf("the task no longer opens the anchor:\n%s", first)
	}
	if !strings.Contains(first, "[harness: files the retrieval pass ranked") || !strings.Contains(first, "payment_processor.go (tier1,") {
		t.Fatalf("the first request carries no retrieval brief naming the issue's file:\n%s", first)
	}
	for _, predicate := range []string{"issue_text", "tiered_context_file", "issue_context"} {
		if rows, _ := kernel.Query(predicate); len(rows) != 0 {
			t.Errorf("%s: %d rows outlived the turn that asserted them", predicate, len(rows))
		}
	}
}

// An /explain turn is not one the kernel retrieves for: no pass, no brief.
func TestProcessWithIntent_ExplainTaskGetsNoBrief(t *testing.T) {
	client, _ := retrievalTurn(t, "/explain", "/query", "Explain ProcessRefund in payment_processor.go", true)
	if first := client.firstUserText(t); strings.Contains(first, "[harness: files the retrieval pass ranked") {
		t.Fatalf("an /explain turn was handed a retrieval brief:\n%s", first)
	}
}

// A spawned subagent's executor inherits the retriever, as it inherits the
// file context: the subagent is what does a delegated task's work.
func TestSpawner_ForwardsIssueRetriever(t *testing.T) {
	s := NewSpawner(&MockKernel{}, &MockVirtualStore{}, &MockLLMClient{}, &MockJITCompiler{}, &MockConfigFactory{}, &MockTransducer{}, DefaultSpawnerConfig())
	r := retrieval.NewTaskRetriever(&MockKernel{}, retrieval.TaskRetrieverConfig{WorkDir: t.TempDir()})
	s.SetIssueRetriever(r)
	agent, err := s.Spawn(context.Background(), SpawnRequest{Name: "coder", Task: "fix the thing", Type: SubAgentTypeEphemeral, IntentVerb: "/fix"})
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	if agent.executor.issueRetriever != r {
		t.Fatal("the spawned executor did not inherit the issue retriever")
	}
	if clone := agent.executor.CloneForTask(); clone.issueRetriever != r {
		t.Fatal("CloneForTask dropped the issue retriever")
	}
}
