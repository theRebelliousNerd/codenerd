package system

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"codenerd/internal/types"
)

// explosiveLLMClient fails any test that lets the planner reach it: goal
// decomposition must never run for a lifecycle label.
type explosiveLLMClient struct {
	calls atomic.Int64
}

func (m *explosiveLLMClient) note() {
	m.calls.Add(1)
}

func (m *explosiveLLMClient) Complete(context.Context, string) (string, error) {
	m.note()
	return "", nil
}

func (m *explosiveLLMClient) CompleteWithSystem(context.Context, string, string) (string, error) {
	m.note()
	return "", nil
}

func (m *explosiveLLMClient) CompleteWithStreaming(context.Context, string, string, bool) (<-chan string, <-chan error) {
	m.note()
	out := make(chan string)
	errch := make(chan error, 1)
	close(out)
	errch <- nil
	return out, errch
}

func (m *explosiveLLMClient) CompleteWithTools(context.Context, string, string, []types.ToolDefinition) (*types.LLMToolResponse, error) {
	m.note()
	return &types.LLMToolResponse{}, nil
}

// TestExecuteActivationLabelSkipsDecomposition proves the planner treats the
// on-demand activation label as a startup signal, not a goal: Execute must
// enter its event loop without a single LLM call and stay there until
// cancelled. Decomposing the label would burn a reasoning-tier call on a
// nonsense agenda every activation.
func TestExecuteActivationLabelSkipsDecomposition(t *testing.T) {
	shard := NewSessionPlannerShard()
	llm := &explosiveLLMClient{}
	shard.SetLLMClient(llm)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := shard.Execute(ctx, "on_demand_activation")
		done <- err
	}()

	// Give a wrongful decomposition ample time to fire an LLM call.
	time.Sleep(500 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Execute did not return after context cancellation")
	}
	if got := llm.calls.Load(); got != 0 {
		t.Fatalf("activation label triggered %d LLM call(s); want 0", got)
	}
}
