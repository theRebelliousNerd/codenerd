package autopoiesis

import (
	"context"
	"testing"
	"time"

	"codenerd/internal/types"
)

// Production boot coverage lives in internal/system/factory_ouroboros_toolstore_test.go.
// The factory owns the listener; interactive, headless and delegated callers
// consume that shared service. Requiring a chat-local start created a duplicate.
func TestDefaultKernelPollInterval_ShouldMatchInteractiveBootCadence(t *testing.T) {
	if DefaultKernelPollInterval != 2*time.Second {
		t.Errorf("DefaultKernelPollInterval = %v; the interactive boot paths use 2s and the constant documents them",
			DefaultKernelPollInterval)
	}
}

// The listener is the only thing that turns a pending delegation fact into a
// generated tool, so prove it actually polls rather than merely starting.
func TestStartKernelListener_WhenDelegationPending_ShouldProcessIt(t *testing.T) {
	orch, _, _ := createTestOrchestrator(t)
	mock := replaceOuroborosWithMock(orch)

	generated := make(chan string, 1)
	mock.ExecuteFunc = func(ctx context.Context, need *ToolNeed) *LoopResult {
		select {
		case generated <- need.Name:
		default:
		}
		return &LoopResult{
			Success:    true,
			ToolName:   need.Name,
			Stage:      StageComplete,
			ToolHandle: runtimeToolFixture(need.Name),
		}
	}

	kernel := &MockKernelInterface{}
	kernel.QueryPredicateFunc = func(predicate string) ([]types.Fact, error) {
		if predicate != "delegate_task" {
			return nil, nil
		}
		return []types.Fact{{
			Predicate: "delegate_task",
			Args:      []any{"/tool_generator", "csv_summarizer", "/pending"},
		}}, nil
	}
	orch.SetKernel(kernel)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := orch.StartKernelListener(ctx, 10*time.Millisecond)

	select {
	case name := <-generated:
		if name != "csv_summarizer" {
			t.Errorf("listener generated %q, want csv_summarizer", name)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("listener never processed the pending delegation")
	}

	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("listener did not stop after context cancellation")
	}
}
