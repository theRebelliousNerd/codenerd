package main

import (
	"context"
	"strings"
	"testing"

	"codenerd/internal/types"
)

// The comparison harness caps LLM spend per mode at maxCalls. The bound must
// fail closed: once exhausted, no method may reach the inner client, and the
// streaming variant must surface the budget error on its error channel
// instead of hanging the consumer.

type stubLLMClient struct {
	calls int
}

func (s *stubLLMClient) Complete(ctx context.Context, prompt string) (string, error) {
	s.calls++
	return "ok", nil
}

func (s *stubLLMClient) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	s.calls++
	return "ok", nil
}

func (s *stubLLMClient) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (<-chan string, <-chan error) {
	s.calls++
	chunks, errs := make(chan string), make(chan error, 1)
	close(chunks)
	close(errs)
	return chunks, errs
}

func (s *stubLLMClient) CompleteWithTools(ctx context.Context, systemPrompt, userPrompt string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	s.calls++
	return &types.LLMToolResponse{}, nil
}

func TestBoundedClient_BudgetExhausted_FailsClosed(t *testing.T) {
	inner := &stubLLMClient{}
	c := &boundedClient{LLMClient: inner}
	ctx := context.Background()

	for i := 0; i < maxCalls; i++ {
		if _, err := c.Complete(ctx, "p"); err != nil {
			t.Fatalf("call %d: %v", i+1, err)
		}
	}
	if inner.calls != maxCalls {
		t.Fatalf("inner calls = %d, want %d", inner.calls, maxCalls)
	}
	if _, err := c.Complete(ctx, "p"); err == nil || !strings.Contains(err.Error(), "budget exhausted") {
		t.Errorf("call %d err = %v, want budget exhaustion", maxCalls+1, err)
	}
	if _, err := c.CompleteWithSystem(ctx, "s", "p"); err == nil {
		t.Error("CompleteWithSystem after exhaustion must fail")
	}
	if _, err := c.CompleteWithTools(ctx, "s", "p", nil); err == nil {
		t.Error("CompleteWithTools after exhaustion must fail")
	}
	if inner.calls != maxCalls {
		t.Errorf("inner calls = %d after exhaustion, want %d (no passthrough)", inner.calls, maxCalls)
	}
}

func TestBoundedClient_StreamingExhausted_SurfacesError(t *testing.T) {
	inner := &stubLLMClient{}
	c := &boundedClient{LLMClient: inner}
	ctx := context.Background()
	for i := 0; i < maxCalls; i++ {
		chunks, errs := c.CompleteWithStreaming(ctx, "s", "p", false)
		for range chunks {
		}
		for err := range errs {
			t.Fatalf("call %d: %v", i+1, err)
		}
	}
	chunks, errs := c.CompleteWithStreaming(ctx, "s", "p", false)
	for range chunks {
		t.Error("exhausted stream must yield no chunks")
	}
	sawErr := false
	for err := range errs {
		sawErr = true
		if !strings.Contains(err.Error(), "budget exhausted") {
			t.Errorf("stream err = %v, want budget exhaustion", err)
		}
	}
	if !sawErr {
		t.Error("exhausted stream must deliver an error, not hang callers on a silent channel")
	}
}

func TestBoundedClient_Unwrap_ReachesInner(t *testing.T) {
	inner := &stubLLMClient{}
	c := &boundedClient{LLMClient: inner}
	if c.Unwrap() != types.LLMClient(inner) {
		t.Error("Unwrap must expose the wrapped client for broker chain walks")
	}
}
