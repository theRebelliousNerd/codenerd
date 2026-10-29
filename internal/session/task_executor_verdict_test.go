package session

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"codenerd/internal/perception"
	"codenerd/internal/types"
)

// verdictScript is the shipped kernel with the verdict of its first
// `unverifiedRuns` turns taken over: such a turn is not done and the kernel
// names one gap (/tests_not_green); later turns are judged by the kernel
// itself. It stands in for a turn whose gates came back red, which the
// harness cannot cheaply provoke.
type verdictScript struct {
	types.Kernel
	unverifiedRuns int
	gap            string

	mu   sync.Mutex
	turn any
	runs int
}

func (k *verdictScript) note(f types.Fact) {
	if f.Predicate == "turn_evidence" && len(f.Args) > 0 {
		k.mu.Lock()
		k.turn = f.Args[0]
		k.runs++
		k.mu.Unlock()
	}
}

func (k *verdictScript) Assert(f types.Fact) error {
	k.note(f)
	return k.Kernel.Assert(f)
}

func (k *verdictScript) AssertBatch(facts []types.Fact) error {
	for _, f := range facts {
		k.note(f)
	}
	return k.Kernel.AssertBatch(facts)
}

func (k *verdictScript) unverified() (turn any, ok bool) {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.turn, k.runs > 0 && k.runs <= k.unverifiedRuns
}

func (k *verdictScript) Query(predicate string) ([]types.Fact, error) {
	turn, unverified := k.unverified()
	switch {
	case predicate == "turn_done" && unverified:
		return nil, nil
	case predicate == "turn_missing_evidence" && unverified && k.gap != "":
		return []types.Fact{{Predicate: predicate, Args: []any{turn, types.MangleAtom(k.gap)}}}, nil
	}
	return k.Kernel.Query(predicate)
}

func verdictJIT(t *testing.T, kernel types.Kernel, prompts *[]string) *JITExecutor {
	t.Helper()
	llm := &MockLLMClient{
		CompleteWithToolsFunc: func(_ context.Context, _, user string, _ []types.ToolDefinition) (*types.LLMToolResponse, error) {
			*prompts = append(*prompts, user)
			return &types.LLMToolResponse{Text: "Task complete"}, nil
		},
		CompleteWithSystemFunc: func(_ context.Context, _, user string) (string, error) {
			*prompts = append(*prompts, user)
			return "Task complete", nil
		},
	}
	transducer := &MockTransducer{
		ParseIntentWithContextFunc: func(context.Context, string, []perception.ConversationTurn) (perception.Intent, error) {
			return perception.Intent{Verb: "/review", Category: "/query"}, nil
		},
	}
	executor := NewExecutor(kernel, &MockVirtualStore{}, llm, &MockJITCompiler{}, &MockConfigFactory{}, transducer)
	spawner := NewSpawner(realKernel(t), &MockVirtualStore{}, llm, &MockJITCompiler{}, &MockConfigFactory{}, transducer, DefaultSpawnerConfig())
	return NewJITExecutor(executor, spawner, transducer)
}

// The string API returns prose, so it reads the verdict for its caller: a turn
// that did not end /done goes out again with the gap the kernel named, and the
// answer is the verified turn's. Driven through the production executor
// constructor over the shipped kernel.
func TestJITExecutor_Execute_AnUnverifiedTurnIsSentBackWithTheGap(t *testing.T) {
	t.Parallel()
	var prompts []string
	jit := verdictJIT(t, &verdictScript{Kernel: realKernel(t), unverifiedRuns: 1, gap: "/tests_not_green"}, &prompts)

	out, err := jit.Execute(context.Background(), TaskRequest{IntentVerb: "/review", Task: "Review the change"})
	if err != nil {
		t.Fatalf("Execute = %v, want the second dispatch to verify", err)
	}
	if out != "Task complete" {
		t.Fatalf("output = %q", out)
	}
	if len(prompts) != 2 {
		t.Fatalf("the model was asked %d time(s), want one per dispatch (2)", len(prompts))
	}
	if strings.Contains(prompts[0], "NOT VERIFIED") {
		t.Fatalf("the first dispatch carries a gap:\n%s", prompts[0])
	}
	for _, want := range []string{"NOT VERIFIED", "the tests were not verified green"} {
		if !strings.Contains(prompts[1], want) {
			t.Errorf("the second dispatch does not carry %q:\n%s", want, prompts[1])
		}
	}
}

// A turn that is never verified and makes no progress is an error, not an
// answer; the observed API still returns the turn as it ended.
func TestJITExecutor_Execute_ATurnThatStaysUnverifiedIsAnError(t *testing.T) {
	t.Parallel()
	var prompts []string
	jit := verdictJIT(t, &verdictScript{Kernel: realKernel(t), unverifiedRuns: 100, gap: "/tests_not_green"}, &prompts)

	_, err := jit.Execute(context.Background(), TaskRequest{IntentVerb: "/review", Task: "Review the change"})
	var nv *NotVerifiedError
	if !errors.As(err, &nv) || nv.Verdict != "/stalled" {
		t.Fatalf("Execute = %v, want a /stalled NotVerifiedError", err)
	}
	if !strings.Contains(err.Error(), "the tests were not verified green") {
		t.Fatalf("the error does not name the gap: %v", err)
	}
	if len(prompts) != 2 {
		t.Fatalf("dispatched %d time(s), want the stall to end it at 2", len(prompts))
	}

	prompts = nil
	ret, err := jit.ExecuteObserved(context.Background(), TaskRequest{IntentVerb: "/review", Task: "Review the change"})
	if err != nil || ret.Done() || ret.Output != "Task complete" || len(prompts) != 1 {
		t.Fatalf("ExecuteObserved: ret=%+v err=%v dispatched=%d, want the raw unverified turn once", ret, err, len(prompts))
	}
}
