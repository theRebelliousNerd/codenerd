package system

import (
	"context"
	"testing"

	"codenerd/internal/types"
	"codenerd/internal/usage"
)

// fakeMeteredLLMClient is a perception.LLMClient stub whose Complete records
// usage against whatever tracker the incoming ctx carries, mirroring what the
// real provider clients do via usage.TrackFromContext.
type fakeMeteredLLMClient struct{}

func (f *fakeMeteredLLMClient) Complete(ctx context.Context, prompt string) (string, error) {
	usage.TrackFromContext(ctx, "m", "meta", 10, 5, "chat")
	return "ok", nil
}

func (f *fakeMeteredLLMClient) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	usage.TrackFromContext(ctx, "m", "meta", 10, 5, "chat")
	return "ok", nil
}

func (f *fakeMeteredLLMClient) CompleteWithTools(ctx context.Context, systemPrompt, userPrompt string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	usage.TrackFromContext(ctx, "m", "meta", 10, 5, "chat")
	return &types.LLMToolResponse{}, nil
}

func (f *fakeMeteredLLMClient) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (<-chan string, <-chan error) {
	usage.TrackFromContext(ctx, "m", "meta", 10, 5, "chat")
	contentChan := make(chan string, 1)
	errorChan := make(chan error, 1)
	contentChan <- "ok"
	close(contentChan)
	close(errorChan)
	return contentChan, errorChan
}

func TestSessionLLMAdapterMetersBareContext(t *testing.T) {
	tr, err := usage.NewTracker(t.TempDir())
	if err != nil {
		t.Fatalf("NewTracker: %v", err)
	}
	t.Cleanup(func() { _ = tr.Close() })

	a := &sessionLLMAdapter{client: &fakeMeteredLLMClient{}, tracker: tr}
	ctx := usage.WithSessionID(context.Background(), "s1")
	if got, err := a.Complete(ctx, "hi"); err != nil || got != "ok" {
		t.Fatalf("Complete = %q, %v; want %q, nil", got, err, "ok")
	}
	counts := tr.SessionTokens("s1")
	if counts.Input != 10 || counts.Output != 5 {
		t.Fatalf("SessionTokens(s1) = input %d output %d; want input 10 output 5", counts.Input, counts.Output)
	}
}

func TestSessionLLMAdapterKeepsExistingTracker(t *testing.T) {
	tr, err := usage.NewTracker(t.TempDir())
	if err != nil {
		t.Fatalf("NewTracker: %v", err)
	}
	t.Cleanup(func() { _ = tr.Close() })
	other, err := usage.NewTracker(t.TempDir())
	if err != nil {
		t.Fatalf("NewTracker(other): %v", err)
	}
	t.Cleanup(func() { _ = other.Close() })

	a := &sessionLLMAdapter{client: &fakeMeteredLLMClient{}, tracker: tr}
	ctx := usage.NewContext(usage.WithSessionID(context.Background(), "s2"), other)
	if _, err := a.Complete(ctx, "hi"); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if counts := tr.SessionTokens("s2"); counts.Input != 0 || counts.Output != 0 {
		t.Fatalf("adapter tracker recorded %v; want zero (existing tracker must win)", counts)
	}
	if counts := other.SessionTokens("s2"); counts.Input != 10 || counts.Output != 5 {
		t.Fatalf("other SessionTokens(s2) = input %d output %d; want input 10 output 5", counts.Input, counts.Output)
	}
}

func TestSessionLLMAdapterNilTrackerIsNoOp(t *testing.T) {
	a := &sessionLLMAdapter{client: &fakeMeteredLLMClient{}}
	ctx := usage.WithSessionID(context.Background(), "s3")
	if got, err := a.Complete(ctx, "hi"); err != nil || got != "ok" {
		t.Fatalf("Complete = %q, %v; want %q, nil", got, err, "ok")
	}
}
