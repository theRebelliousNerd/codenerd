package perception

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"codenerd/internal/types"
)

// fallbackFakeClient is a scriptable LLMClient for failover tests. String
// methods return resp/err; the stream emits streamChunks then streamErr,
// mirroring the producer convention (both channels closed).
type fallbackFakeClient struct {
	resp         string
	err          error
	toolResp     *types.LLMToolResponse
	streamChunks []string
	streamErr    error
	nilStream    bool

	completeCalls int
	systemCalls   int
	toolsCalls    int
	streamCalls   int
}

func (f *fallbackFakeClient) Complete(ctx context.Context, prompt string) (string, error) {
	f.completeCalls++
	return f.resp, f.err
}

func (f *fallbackFakeClient) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	f.systemCalls++
	return f.resp, f.err
}

func (f *fallbackFakeClient) CompleteWithTools(ctx context.Context, systemPrompt, userPrompt string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	f.toolsCalls++
	return f.toolResp, f.err
}

func (f *fallbackFakeClient) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (<-chan string, <-chan error) {
	f.streamCalls++
	if f.nilStream {
		return nil, nil
	}
	out := make(chan string, 100)
	errc := make(chan error, 1)
	go func() {
		defer close(out)
		defer close(errc)
		for _, c := range f.streamChunks {
			out <- c
		}
		if f.streamErr != nil {
			errc <- f.streamErr
		}
	}()
	return out, errc
}

// collectStream drains a stream like production consumers do: range content to
// close, then read the terminal error. It fails the test on hang.
func collectStream(t *testing.T, chunks <-chan string, errs <-chan error) (string, error) {
	t.Helper()
	type result struct {
		text string
		err  error
	}
	done := make(chan result, 1)
	go func() {
		var sb strings.Builder
		for c := range chunks {
			sb.WriteString(c)
		}
		var err error
		select {
		case err = <-errs:
		default:
		}
		done <- result{sb.String(), err}
	}()
	select {
	case r := <-done:
		return r.text, r.err
	case <-time.After(10 * time.Second):
		t.Fatal("stream collection hung")
		return "", nil
	}
}

func TestFallbackClient_PrimarySuccessNoSecondaryCall(t *testing.T) {
	primary := &fallbackFakeClient{resp: "primary-label"}
	secondary := &fallbackFakeClient{resp: "secondary-label"}
	c := NewFallbackClient("test", primary, secondary)

	resp, err := c.Complete(context.Background(), "prompt")
	if err != nil || resp != "primary-label" {
		t.Fatalf("Complete = %q, %v; want primary-label, nil", resp, err)
	}
	if secondary.completeCalls != 0 {
		t.Fatalf("secondary called %d times on primary success; want 0", secondary.completeCalls)
	}
}

func TestFallbackClient_PrimaryErrorUsesSecondary(t *testing.T) {
	primary := &fallbackFakeClient{err: errors.New("primary 403")}
	secondary := &fallbackFakeClient{resp: "secondary-label"}
	c := NewFallbackClient("test", primary, secondary)

	resp, err := c.CompleteWithSystem(context.Background(), "sys", "user")
	if err != nil || resp != "secondary-label" {
		t.Fatalf("CompleteWithSystem = %q, %v; want secondary-label, nil", resp, err)
	}
	if primary.systemCalls != 1 || secondary.systemCalls != 1 {
		t.Fatalf("calls primary=%d secondary=%d; want 1/1", primary.systemCalls, secondary.systemCalls)
	}
}

func TestFallbackClient_BothFailSurfacesSecondaryError(t *testing.T) {
	primary := &fallbackFakeClient{err: errors.New("primary boom")}
	secondary := &fallbackFakeClient{err: errors.New("secondary boom")}
	c := NewFallbackClient("test", primary, secondary)

	_, err := c.Complete(context.Background(), "prompt")
	if err == nil || !strings.Contains(err.Error(), "secondary boom") {
		t.Fatalf("Complete err = %v; want secondary boom", err)
	}
}

func TestFallbackClient_NilSecondarySurfacesPrimaryError(t *testing.T) {
	primary := &fallbackFakeClient{err: errors.New("primary boom")}
	c := NewFallbackClient("test", primary, nil)

	_, err := c.Complete(context.Background(), "prompt")
	if err == nil || !strings.Contains(err.Error(), "primary boom") {
		t.Fatalf("Complete err = %v; want primary boom", err)
	}
}

func TestFallbackClient_NilPrimaryUsesSecondary(t *testing.T) {
	secondary := &fallbackFakeClient{resp: "secondary-label"}
	c := NewFallbackClient("test", nil, secondary)

	resp, err := c.Complete(context.Background(), "prompt")
	if err != nil || resp != "secondary-label" {
		t.Fatalf("Complete = %q, %v; want secondary-label, nil", resp, err)
	}
}

func TestFallbackClient_NeitherClientFailsLoudly(t *testing.T) {
	c := NewFallbackClient("test", nil, nil)
	if _, err := c.Complete(context.Background(), "prompt"); !errors.Is(err, errFallbackNoClient) {
		t.Fatalf("Complete err = %v; want errFallbackNoClient", err)
	}
}

func TestFallbackClient_ToolsFailover(t *testing.T) {
	primary := &fallbackFakeClient{err: errors.New("primary 403")}
	secondary := &fallbackFakeClient{toolResp: &types.LLMToolResponse{Text: "tool-ok"}}
	c := NewFallbackClient("test", primary, secondary)

	resp, err := c.CompleteWithTools(context.Background(), "sys", "user", nil)
	if err != nil || resp == nil || resp.Text != "tool-ok" {
		t.Fatalf("CompleteWithTools = %+v, %v; want tool-ok, nil", resp, err)
	}
}

func TestFallbackClient_StreamFailoverBeforeFirstChunk(t *testing.T) {
	primary := &fallbackFakeClient{streamErr: errors.New("primary 403")}
	secondary := &fallbackFakeClient{streamChunks: []string{"a", "b"}}
	c := NewFallbackClient("test", primary, secondary)

	chunks, errs := c.CompleteWithStreaming(context.Background(), "sys", "user", false)
	text, err := collectStream(t, chunks, errs)
	if err != nil || text != "ab" {
		t.Fatalf("stream = %q, %v; want ab, nil", text, err)
	}
	if primary.streamCalls != 1 || secondary.streamCalls != 1 {
		t.Fatalf("stream calls primary=%d secondary=%d; want 1/1", primary.streamCalls, secondary.streamCalls)
	}
}

func TestFallbackClient_StreamNoFailoverAfterData(t *testing.T) {
	primary := &fallbackFakeClient{streamChunks: []string{"a"}, streamErr: errors.New("late primary error")}
	secondary := &fallbackFakeClient{streamChunks: []string{"b"}}
	c := NewFallbackClient("test", primary, secondary)

	chunks, errs := c.CompleteWithStreaming(context.Background(), "sys", "user", false)
	text, err := collectStream(t, chunks, errs)
	if text != "a" || err == nil || !strings.Contains(err.Error(), "late primary error") {
		t.Fatalf("stream = %q, %v; want a + late primary error", text, err)
	}
	if secondary.streamCalls != 0 {
		t.Fatalf("secondary stream called %d times after primary data; want 0", secondary.streamCalls)
	}
}

func TestFallbackClient_StreamBothFail(t *testing.T) {
	primary := &fallbackFakeClient{streamErr: errors.New("primary 403")}
	secondary := &fallbackFakeClient{streamErr: errors.New("secondary down")}
	c := NewFallbackClient("test", primary, secondary)

	chunks, errs := c.CompleteWithStreaming(context.Background(), "sys", "user", false)
	text, err := collectStream(t, chunks, errs)
	if text != "" || err == nil || !strings.Contains(err.Error(), "secondary down") {
		t.Fatalf("stream = %q, %v; want empty + secondary down", text, err)
	}
}

// fallbackNamedFakeClient adds the observability methods to the scriptable
// fake. The bare fake deliberately lacks them so fall-through is testable.
type fallbackNamedFakeClient struct {
	*fallbackFakeClient
	model    string
	provider string
}

func (f *fallbackNamedFakeClient) GetModel() string { return f.model }

func (f *fallbackNamedFakeClient) ModelIdentity() (string, string) { return f.provider, f.model }

func TestFallbackClient_ForwardsObservability(t *testing.T) {
	primary := &fallbackNamedFakeClient{fallbackFakeClient: &fallbackFakeClient{}, model: "primary-model", provider: "primary-prov"}
	secondary := &fallbackNamedFakeClient{fallbackFakeClient: &fallbackFakeClient{}, model: "secondary-model", provider: "secondary-prov"}
	c := NewFallbackClient("test", primary, secondary)
	if got := c.GetModel(); got != "primary-model" {
		t.Errorf("GetModel = %q, want primary-model", got)
	}
	if p, m := c.ModelIdentity(); p != "primary-prov" || m != "primary-model" {
		t.Errorf("ModelIdentity = %q/%q, want primary-prov/primary-model", p, m)
	}

	// A primary without observability falls through to the secondary.
	bare := &fallbackFakeClient{}
	c2 := NewFallbackClient("test", bare, secondary)
	if got := c2.GetModel(); got != "secondary-model" {
		t.Errorf("GetModel fall-through = %q, want secondary-model", got)
	}
	if p, m := c2.ModelIdentity(); p != "secondary-prov" || m != "secondary-model" {
		t.Errorf("ModelIdentity fall-through = %q/%q, want secondary-prov/secondary-model", p, m)
	}

	// Neither side observable: empty, not a crash.
	c3 := NewFallbackClient("test", bare, &fallbackFakeClient{})
	if got := c3.GetModel(); got != "" {
		t.Errorf("GetModel with bare clients = %q, want empty", got)
	}
	if p, m := c3.ModelIdentity(); p != "" || m != "" {
		t.Errorf("ModelIdentity with bare clients = %q/%q, want empty/empty", p, m)
	}
}

func TestFallbackClient_NilStreamFailsOver(t *testing.T) {
	primary := &fallbackFakeClient{nilStream: true}
	secondary := &fallbackFakeClient{streamChunks: []string{"ok"}}
	c := NewFallbackClient("test", primary, secondary)

	chunks, errs := c.CompleteWithStreaming(context.Background(), "sys", "user", false)
	text, err := collectStream(t, chunks, errs)
	if err != nil || text != "ok" {
		t.Fatalf("stream = %q, %v; want ok, nil", text, err)
	}
}

// Unwrap exposes the preferred path for broker chain walks: primary when
// set, else secondary, else nil (which ends the walk, mirroring that a
// missing side never serves).
func TestFallbackClient_UnwrapPrefersPrimary(t *testing.T) {
	primary := &fallbackFakeClient{}
	secondary := &fallbackFakeClient{}
	if got := NewFallbackClient("test", primary, secondary).Unwrap(); got != LLMClient(primary) {
		t.Fatalf("Unwrap with both sides = %p, want primary %p", got, primary)
	}
	if got := NewFallbackClient("test", nil, secondary).Unwrap(); got != LLMClient(secondary) {
		t.Fatalf("Unwrap with nil primary = %p, want secondary %p", got, secondary)
	}
	if got := NewFallbackClient("test", nil, nil).Unwrap(); got != nil {
		t.Fatalf("Unwrap with no sides = %v, want nil", got)
	}
}
