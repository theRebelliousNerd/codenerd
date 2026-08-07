package session

import (
	"context"
	"sync"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/jit/config"
	"codenerd/internal/perception"
	"codenerd/internal/tools"
	toolscore "codenerd/internal/tools/core"
	"codenerd/internal/types"
)

// recordingLLMClient records every call so a test can prove which slot served
// a turn. It also implements ToolResultsProvider so the executor takes the
// native multi-turn path rather than degrading to a single round.
type recordingLLMClient struct {
	name string

	mu             sync.Mutex
	calls          int
	toolResultRuns int

	// firstTurnToolCalls is returned by CompleteWithTools on the first call so
	// the executor enters the tool-result loop; later calls return a final
	// answer with no tool calls.
	firstTurnToolCalls []types.ToolCall
}

func (c *recordingLLMClient) CompleteWithStreaming(ctx context.Context, sys, user string, enableThinking bool) (<-chan string, <-chan error) {
	c.record()
	out := make(chan string, 1)
	errs := make(chan error, 1)
	out <- c.name
	close(out)
	close(errs)
	return out, errs
}

func (c *recordingLLMClient) record() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	return c.calls
}

func (c *recordingLLMClient) callCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls
}

func (c *recordingLLMClient) Complete(ctx context.Context, prompt string) (string, error) {
	c.record()
	return c.name, nil
}

func (c *recordingLLMClient) CompleteWithSystem(ctx context.Context, sys, user string) (string, error) {
	c.record()
	return c.name, nil
}

func (c *recordingLLMClient) CompleteWithTools(ctx context.Context, sys, user string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	n := c.record()
	if n == 1 && len(c.firstTurnToolCalls) > 0 {
		return &types.LLMToolResponse{Text: c.name, ToolCalls: c.firstTurnToolCalls, StopReason: "tool_use"}, nil
	}
	return &types.LLMToolResponse{Text: c.name, StopReason: "end_turn"}, nil
}

func (c *recordingLLMClient) CompleteWithToolResults(ctx context.Context, sys string, history []types.Message, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	c.record()
	c.mu.Lock()
	c.toolResultRuns++
	c.mu.Unlock()
	return &types.LLMToolResponse{Text: c.name, StopReason: "end_turn"}, nil
}

// newRoutingExecutor builds an executor over the real kernel (so the live
// policy corpus decides routing) with a distinguishable client in each slot.
func newRoutingExecutor(t *testing.T) (*Executor, *recordingLLMClient, *recordingLLMClient) {
	t.Helper()
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	worker := &recordingLLMClient{name: "worker"}
	planner := &recordingLLMClient{name: "planner"}

	e := NewExecutor(k, &MockVirtualStore{}, worker, &MockJITCompiler{}, &MockConfigFactory{}, &MockTransducer{})
	e.SetPlannerClient(planner)
	return e, worker, planner
}

// TestLLMForVerb_RoutesReasoningIntentsToPlanner is the end-to-end proof that
// the two-tier split works: the kernel's intent_requires_reasoning_model/1
// verdict decides which client serves the turn. Without it, pointing the worker
// slot at a cheap bulk model also demotes /review and /audit — the exact turns
// the expensive model exists for.
func TestLLMForVerb_RoutesReasoningIntentsToPlanner(t *testing.T) {
	e, worker, planner := newRoutingExecutor(t)

	for _, verb := range []string{"/review", "/audit", "/analyze", "/campaign", "/generate_tool"} {
		if got := e.llmForVerb(verb); got != types.LLMClient(planner) {
			t.Errorf("llmForVerb(%s) = %v, want the planner client", verb, got)
		}
	}
	for _, verb := range []string{"/explain", "/create", "/fix", "/read", "/general"} {
		if got := e.llmForVerb(verb); got != types.LLMClient(worker) {
			t.Errorf("llmForVerb(%s) = %v, want the worker client", verb, got)
		}
	}
}

// TestLLMForVerb_NoPlannerKeepsEverythingOnDefault pins the opt-in contract:
// with no planner slot configured, routing is a no-op and behaviour is
// byte-identical to the single-tier executor.
func TestLLMForVerb_NoPlannerKeepsEverythingOnDefault(t *testing.T) {
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	worker := &recordingLLMClient{name: "worker"}
	e := NewExecutor(k, &MockVirtualStore{}, worker, &MockJITCompiler{}, &MockConfigFactory{}, &MockTransducer{})

	for _, verb := range []string{"/review", "/audit", "/campaign", "/explain", "/create"} {
		if got := e.llmForVerb(verb); got != types.LLMClient(worker) {
			t.Errorf("llmForVerb(%s) = %v, want the default client when no planner is set", verb, got)
		}
	}
}

// TestSetPlannerClient_IgnoresSameClient guards against a config where both
// slots resolve to the same client: routing would then be pure overhead, and a
// nil planner short-circuits the per-turn kernel query.
func TestSetPlannerClient_IgnoresSameClient(t *testing.T) {
	e, worker, _ := newRoutingExecutor(t)
	e.SetPlannerClient(worker)
	if e.plannerClient != nil {
		t.Fatal("SetPlannerClient(llmClient) should clear the planner slot, not alias it")
	}
	if got := e.llmForVerb("/review"); got != types.LLMClient(worker) {
		t.Errorf("llmForVerb(/review) = %v, want the default client", got)
	}
}

// TestRunToolLoop_UsesOneClientForTheWholeTurn is the correctness constraint
// behind threading the client through generateResponse instead of reading it
// from the struct each call. The initial generation and its tool-result
// follow-ups share one conversation history, so splitting them across two
// vendors would feed one vendor's tool_use IDs to another.
func TestRunToolLoop_UsesOneClientForTheWholeTurn(t *testing.T) {
	// The tool loop only takes the native multi-turn path when it has real tool
	// definitions, which come from the global registry.
	if err := toolscore.RegisterAll(tools.Global()); err != nil {
		t.Fatalf("RegisterAll: %v", err)
	}

	e, worker, planner := newRoutingExecutor(t)
	planner.firstTurnToolCalls = []types.ToolCall{
		{ID: "call_1", Name: "read_file", Input: map[string]any{"path": "go.mod"}},
	}

	result := &ExecutionResult{Intent: perception.Intent{Verb: "/review", Category: "/query"}}
	cfg := &config.EffectiveAgentRuntimeConfig{AllowedTools: []string{"read_file"}}

	_, _, err := e.runToolLoop(context.Background(), "system", "review this", cfg, nil, result)
	if err != nil {
		t.Fatalf("runToolLoop: %v", err)
	}

	if worker.callCount() != 0 {
		t.Errorf("worker client served %d call(s) during a /review turn; the turn must stay on the planner",
			worker.callCount())
	}
	if planner.callCount() < 2 {
		t.Errorf("planner served %d call(s); expected the initial generation plus at least one tool-result follow-up",
			planner.callCount())
	}
	if planner.toolResultRuns == 0 {
		t.Error("tool results were never fed back to the planner client")
	}
}
