package session

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"codenerd/internal/jit/config"
	"codenerd/internal/perception"
	"codenerd/internal/tools"
	"codenerd/internal/types"
)

// The defect this guards (F-TIMEOUT-1, observed live on
// `nerd analyze internal/projectdoc`):
//
//	Error: shard execution failed: execution failed: LLM generation failed:
//	tool-result follow-up failed: context deadline exceeded
//
// That names neither the budget that expired, nor how much work had already
// succeeded, nor the flag that controls it. It is indistinguishable from a
// broken shard, and it sent this session chasing a non-defect — the run was
// progressing normally and simply needed longer, exactly as `nerd security`
// had minutes earlier.
//
// This is the third command where a timeout looked like a failure:
// `nerd tool generate` (fixed by describeStageTimeout) and `nerd dream` (fixed
// by dreamSummary) came first. The recurrence is why this is worth a helper
// rather than a one-off string.

func TestDescribeToolLoopFailure_NamesBudgetAndProgress(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()

	err := describeToolLoopFailure(ctx, 3, 5, context.DeadlineExceeded)
	if err == nil {
		t.Fatal("expected an error")
	}
	msg := err.Error()

	if !strings.Contains(msg, "4 tool iteration") {
		t.Errorf("does not report how many iterations completed: %s", msg)
	}
	if !strings.Contains(msg, "5 tool call") {
		t.Errorf("does not report the work done in the final round: %s", msg)
	}
	if !strings.Contains(msg, "--timeout") {
		t.Errorf("does not name the flag that fixes it: %s", msg)
	}
	if !strings.Contains(msg, "not stuck") {
		t.Errorf("does not distinguish a slow run from a broken one: %s", msg)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Error("wrapping lost the sentinel; callers can no longer test for a timeout")
	}
}

// Non-timeout failures must pass through with their original framing. Dressing
// an unrelated error as a budget problem sends the reader the wrong way — the
// exact mistake this helper exists to stop.
func TestDescribeToolLoopFailure_PassesThroughOtherErrors(t *testing.T) {
	orig := errors.New("provider returned malformed tool_use block")

	got := describeToolLoopFailure(context.Background(), 1, 1, orig)
	if got == nil {
		t.Fatal("expected an error")
	}
	if !errors.Is(got, orig) {
		t.Errorf("original error was not wrapped: %v", got)
	}
	if strings.Contains(got.Error(), "--timeout") {
		t.Errorf("a non-timeout failure was blamed on the budget: %v", got)
	}
}

func TestDescribeToolLoopFailure_NilStaysNil(t *testing.T) {
	if err := describeToolLoopFailure(context.Background(), 0, 0, nil); err != nil {
		t.Errorf("nil became %v", err)
	}
}

// A context with no deadline must still produce a usable message rather than
// claiming a budget it cannot name.
func TestDescribeToolLoopFailure_NoDeadlineStillExplains(t *testing.T) {
	err := describeToolLoopFailure(context.Background(), 0, 2, context.DeadlineExceeded)
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "--timeout") {
		t.Errorf("lost the actionable hint: %v", err)
	}
}

func TestToolExplorationCutoff_NoDeadline(t *testing.T) {
	if cutoff, reserve, ok := toolExplorationCutoff(context.Background(), time.Minute); ok {
		t.Fatalf("unexpected cutoff %v with reserve %v for a context without a deadline", cutoff, reserve)
	}
}

func TestToolExplorationCutoff_ReservesConfiguredTail(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	cutoff, reserve, ok := toolExplorationCutoff(ctx, 4*time.Minute)
	if !ok {
		t.Fatal("expected a cutoff")
	}
	deadline, _ := ctx.Deadline()
	if reserve != 4*time.Minute {
		t.Fatalf("reserve = %v, want 4m", reserve)
	}
	if got := deadline.Sub(cutoff); got != reserve {
		t.Fatalf("deadline - cutoff = %v, want %v", got, reserve)
	}
}

func TestToolExplorationCutoff_ShortTurnKeepsHalfForFinal(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	_, reserve, ok := toolExplorationCutoff(ctx, 5*time.Minute)
	if !ok {
		t.Fatal("expected a cutoff")
	}
	if reserve < 90*time.Millisecond || reserve > 100*time.Millisecond {
		t.Fatalf("reserve = %v, want approximately half of the short turn", reserve)
	}
}

// deadlineToolLoopClient deliberately never concludes while exploration tools
// remain. Once the executor removes them for its reserved final call, it emits
// a verdict immediately. This reproduces the live `nerd review` failure without
// making a test wait 12 or 25 minutes.
type deadlineToolLoopClient struct {
	toolName              string
	ordinaryFollowups     atomic.Int32
	finalFollowups        atomic.Int32
	remainingAtFinalNanos atomic.Int64
}

func (c *deadlineToolLoopClient) Complete(context.Context, string) (string, error) {
	return "", nil
}

func (c *deadlineToolLoopClient) CompleteWithSystem(context.Context, string, string) (string, error) {
	return "", nil
}

func (c *deadlineToolLoopClient) CompleteWithStreaming(
	context.Context, string, string, bool,
) (<-chan string, <-chan error) {
	out := make(chan string)
	errs := make(chan error)
	close(out)
	close(errs)
	return out, errs
}

func (c *deadlineToolLoopClient) CompleteWithTools(
	context.Context, string, string, []types.ToolDefinition,
) (*types.LLMToolResponse, error) {
	return &types.LLMToolResponse{
		ToolCalls:  []types.ToolCall{{ID: "deadline-read-1", Name: c.toolName}},
		StopReason: "tool_use",
	}, nil
}

func (c *deadlineToolLoopClient) CompleteWithToolResults(
	ctx context.Context,
	_ string,
	_ []types.Message,
	availableTools []types.ToolDefinition,
) (*types.LLMToolResponse, error) {
	if len(availableTools) > 0 {
		c.ordinaryFollowups.Add(1)
		<-ctx.Done()
		return nil, ctx.Err()
	}
	c.finalFollowups.Add(1)
	if deadline, ok := ctx.Deadline(); ok {
		c.remainingAtFinalNanos.Store(int64(time.Until(deadline)))
	}
	return &types.LLMToolResponse{Text: "deadline-safe verdict", StopReason: "end_turn"}, nil
}

func TestRunToolLoop_ReservesTimeForFinalVerdict(t *testing.T) {
	const toolName = "deadline_reserve_probe"
	var executions atomic.Int32
	tools.Global().Register(&tools.Tool{
		Name:     toolName,
		Category: tools.CategoryGeneral,
		Execute: func(context.Context, map[string]any) (string, error) {
			executions.Add(1)
			return "probe evidence", nil
		},
	})

	client := &deadlineToolLoopClient{toolName: toolName}
	executor := NewExecutor(
		nil, &MockVirtualStore{}, client, &MockJITCompiler{}, &MockConfigFactory{}, &MockTransducer{})
	executor.config.EnableSafetyGate = false
	executor.config.MaxToolIterations = 24
	executor.config.MaxToolCalls = 10
	executor.config.ToolTimeout = time.Second
	executor.config.FinalAnswerReserve = 120 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 350*time.Millisecond)
	defer cancel()
	result := &ExecutionResult{Intent: perception.Intent{Verb: "/review", Category: "/query"}}
	cfg := &config.EffectiveAgentRuntimeConfig{AllowedTools: []string{toolName}}

	response, _, err := executor.runToolLoop(ctx, "system", "review this", cfg, nil, result)
	if err != nil {
		t.Fatalf("runToolLoop: %v", err)
	}
	if response == nil || response.Text != "deadline-safe verdict" {
		t.Fatalf("response = %#v, want deadline-safe verdict", response)
	}
	if got := executions.Load(); got != 1 {
		t.Fatalf("tool executions = %d, want 1; deadline finalization must not replay side effects", got)
	}
	if got := client.ordinaryFollowups.Load(); got != 1 {
		t.Fatalf("ordinary follow-ups = %d, want 1", got)
	}
	if got := client.finalFollowups.Load(); got != 1 {
		t.Fatalf("final follow-ups = %d, want 1", got)
	}
	if remaining := time.Duration(client.remainingAtFinalNanos.Load()); remaining < 80*time.Millisecond {
		t.Fatalf("final call started with only %v remaining; reserve was consumed by exploration", remaining)
	}
}
