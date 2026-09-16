package perception

import (
	"context"
	"errors"
	"testing"
	"time"

	"codenerd/internal/types"
)

// waitForTraces polls an async trace store until n traces land or the test
// times out. Traces are stored off the hot path, so a bare read would race.
func waitForTraces(t *testing.T, store *mockTraceStore, n int) []*ReasoningTrace {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if got := store.getTraces(); len(got) >= n {
			return got
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d traces, got %d", n, len(store.getTraces()))
	return nil
}

// Tool-loop turns are the turns most worth learning from, yet they passed
// through this wrapper with no metrics and no trace. Both must land, with the
// last user turn attributed as the driving prompt.
func TestTracingToolResults_RecordsMetricsAndTrace(t *testing.T) {
	store := &mockTraceStore{}
	tc := NewTracingLLMClient(&trpUnderlying{}, store)
	tc.SetShardContext("sh-trp-1", "trp-type-uplift", "trp-cat-uplift", "sess", "task")

	history := []types.Message{
		{Role: "user", Text: "create app"},
		{Role: "assistant", ToolCalls: []types.ToolCall{{ID: "c1", Name: "write_file"}}},
		{Role: "user", Text: "now add tests"},
	}
	resp, err := tc.CompleteWithToolResults(context.Background(), "sys", history, nil)
	if err != nil {
		t.Fatalf("CompleteWithToolResults: %v", err)
	}
	if resp.Text != "traced-tool-results" {
		t.Fatalf("resp = %+v, want the forwarded response", resp)
	}

	traces := waitForTraces(t, store, 1)
	tr := traces[0]
	if !tr.Success || tr.Response != "traced-tool-results" {
		t.Errorf("trace = %+v, want success with the response text", tr)
	}
	if tr.UserPrompt != "now add tests" {
		t.Errorf("trace UserPrompt = %q, want the last user turn", tr.UserPrompt)
	}
	if tr.ShardID != "sh-trp-1" || tr.ShardCategory != "trp-cat-uplift" {
		t.Errorf("trace attribution = %+v, want the shard context", tr)
	}

	got := GetLLMMetrics()["trp-cat-uplift:trp-type-uplift"]
	if got.Calls != 1 {
		t.Errorf("metrics calls = %d, want 1", got.Calls)
	}
	if got.Errors != 0 {
		t.Errorf("metrics errors = %d, want 0", got.Errors)
	}
}

// An underlying client that returns (nil, nil) must not panic the wrapper's
// log line; the empty turn is traced like any other.
func TestTracingTools_NilResponseDoesNotPanic(t *testing.T) {
	store := &mockTraceStore{}
	tc := NewTracingLLMClient(&baseMockLLMClient{}, store)
	tc.SetShardContext("sh-nil-1", "coder", "ephemeral", "sess", "task")

	resp, err := tc.CompleteWithTools(context.Background(), "sys", "user", nil)
	if err != nil {
		t.Fatalf("CompleteWithTools: %v", err)
	}
	if resp != nil {
		t.Fatalf("resp = %+v, want the forwarded nil", resp)
	}
	traces := waitForTraces(t, store, 1)
	if !traces[0].Success || traces[0].Response != "" {
		t.Errorf("trace = %+v, want a successful empty turn", traces[0])
	}
}

// The hand-built tools trace silently dropped Model; the shared constructor
// attributes it like every other path.
func TestTracingTools_TraceCarriesModel(t *testing.T) {
	store := &mockTraceStore{}
	tc := NewTracingLLMClient(&mockModelProvider{model: "uplift-model"}, store)

	if _, err := tc.CompleteWithTools(context.Background(), "sys", "user", nil); err != nil {
		t.Fatalf("CompleteWithTools: %v", err)
	}
	traces := waitForTraces(t, store, 1)
	if traces[0].Model != "uplift-model" {
		t.Errorf("trace Model = %q, want uplift-model", traces[0].Model)
	}
}

// cancelStreamUnderlying is a conforming streaming client: it emits one chunk,
// then closes both channels as soon as the context dies.
type cancelStreamUnderlying struct{ baseMockLLMClient }

func (m *cancelStreamUnderlying) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (<-chan string, <-chan error) {
	content := make(chan string, 1)
	errCh := make(chan error, 1)
	content <- "partial"
	go func() {
		defer close(content)
		defer close(errCh)
		<-ctx.Done()
	}()
	return content, errCh
}

// Cancelling mid-stream must terminate the relay promptly with the
// cancellation error — not hang, not spin, and the partial turn is traced.
func TestTracingStreaming_CancelTerminatesRelay(t *testing.T) {
	store := &mockTraceStore{}
	tc := NewTracingLLMClient(&cancelStreamUnderlying{}, store)
	tc.SetShardContext("sh-stream-1", "coder", "ephemeral", "sess", "task")

	ctx, cancel := context.WithCancel(context.Background())
	outContent, outErr := tc.CompleteWithStreaming(ctx, "sys", "user", false)

	select {
	case chunk := <-outContent:
		if chunk != "partial" {
			t.Fatalf("chunk = %q, want partial", chunk)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the relayed chunk")
	}
	cancel()

	done := make(chan struct{})
	var firstErr error
	go func() {
		defer close(done)
		for range outContent {
		}
		if err, ok := <-outErr; ok {
			firstErr = err
		}
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("relay did not terminate after cancel")
	}
	if !errors.Is(firstErr, context.Canceled) {
		t.Errorf("relay error = %v, want context.Canceled", firstErr)
	}
	traces := waitForTraces(t, store, 1)
	if traces[0].Success || traces[0].Response != "partial" {
		t.Errorf("trace = %+v, want the failed partial turn", traces[0])
	}
}
