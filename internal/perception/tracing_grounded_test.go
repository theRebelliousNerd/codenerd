package perception

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"codenerd/internal/types"
)

// mock grounded searcher for wrapper tests

type groundedMock struct {
	supports bool
	handler  func(ctx context.Context, query string) (*types.GroundedWebSearchResult, error)
	lastQ    string
	calls    int
}

func (m *groundedMock) SupportsGroundedWebSearch() bool { return m.supports }
func (m *groundedMock) Complete(ctx context.Context, prompt string) (string, error) {
	return "ok", nil
}
func (m *groundedMock) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return "ok", nil
}
func (m *groundedMock) CompleteWithStreaming(_ context.Context, _, _ string, _ bool) (<-chan string, <-chan error) {
	ch := make(chan string)
	close(ch)
	errCh := make(chan error)
	close(errCh)
	return ch, errCh
}
func (m *groundedMock) CompleteWithTools(_ context.Context, _, _ string, _ []types.ToolDefinition) (*types.LLMToolResponse, error) {
	return &types.LLMToolResponse{Text: "ok"}, nil
}
func (m *groundedMock) GroundedWebSearch(ctx context.Context, query string) (*types.GroundedWebSearchResult, error) {
	m.lastQ = query
	m.calls++
	if m.handler != nil {
		return m.handler(ctx, query)
	}
	return &types.GroundedWebSearchResult{Text: "answer", Citations: []types.GroundedCitation{{URL: "https://example.com", Title: "t", StartIndex: 0, EndIndex: 2}}, Usage: types.GroundedUsage{InputTokens: 1, OutputTokens: 2, TotalTokens: 3}}, nil
}

// compile-time interface checks
var _ types.GroundedWebSearcher = (*groundedMock)(nil)

func TestTracingLLMClient_GroundedWebSearch_Forwards(t *testing.T) {
	t.Parallel()
	m := &groundedMock{supports: true, handler: func(_ context.Context, q string) (*types.GroundedWebSearchResult, error) {
		if q != "exact query 42" {
			t.Errorf("query = %q, want exact forwarding", q)
		}
		return &types.GroundedWebSearchResult{Text: "hello", Citations: []types.GroundedCitation{{URL: "https://a.com"}}, Usage: types.GroundedUsage{InputTokens: 10, OutputTokens: 5, TotalTokens: 15}}, nil
	}}
	store := &mockTraceStore{}
	tc := NewTracingLLMClient(m, store)
	// Also check Supports delegation
	if !tc.SupportsGroundedWebSearch() {
		t.Fatal("tracing should delegate supports=true")
	}
	res, err := tc.GroundedWebSearch(context.Background(), "exact query 42")
	if err != nil {
		t.Fatalf("GroundedWebSearch: %v", err)
	}
	if res.Text != "hello" {
		t.Errorf("text = %q", res.Text)
	}
	if m.lastQ != "exact query 42" {
		t.Errorf("underlying query not exact: %q", m.lastQ)
	}
	// Wait for trace store
	time.Sleep(50 * time.Millisecond)
	traces := store.getTraces()
	if len(traces) != 1 {
		t.Fatalf("traces = %d, want 1", len(traces))
	}
	if traces[0].Response != "hello" {
		t.Errorf("trace response = %q", traces[0].Response)
	}
	if traces[0].UserPrompt != "exact query 42" {
		t.Errorf("trace userprompt = %q", traces[0].UserPrompt)
	}
	// Tracing must not fabricate reasoning content into the result
	if strings.Contains(res.Text, "reasoning") {
		t.Error("result leaked reasoning")
	}
}

func TestTracingLLMClient_GroundedWebSearch_ImplementsInterface(t *testing.T) {
	var tc any = &TracingLLMClient{}
	if _, ok := tc.(types.GroundedWebSearcher); !ok {
		t.Error("TracingLLMClient must implement GroundedWebSearcher")
	}
}

func TestTracingLLMClient_GroundedWebSearch_SupportsFalseOrNil(t *testing.T) {
	t.Parallel()
	// Nil underlying
	tc := NewTracingLLMClient(nil, nil)
	if tc.SupportsGroundedWebSearch() {
		t.Error("nil underlying should return false")
	}
	_, err := tc.GroundedWebSearch(context.Background(), "q")
	if err == nil {
		t.Error("expected error for nil underlying")
	}
	// Supports false underlying
	m := &groundedMock{supports: false}
	tc2 := NewTracingLLMClient(m, nil)
	if tc2.SupportsGroundedWebSearch() {
		t.Error("should be false when underlying false")
	}
	// Underlying not implementing GroundedWebSearcher
	base := &baseMockLLMClient{}
	tc3 := NewTracingLLMClient(base, nil)
	if tc3.SupportsGroundedWebSearch() {
		t.Error("non-searcher underlying should be false")
	}
	_, err = tc3.GroundedWebSearch(context.Background(), "q")
	if err == nil {
		t.Error("expected error for non-searcher")
	}
	// Nil TracingLLMClient receiver
	var nilTC *TracingLLMClient
	if nilTC.SupportsGroundedWebSearch() {
		t.Error("nil tracing client should be false")
	}
	_, err = nilTC.GroundedWebSearch(context.Background(), "q")
	if err == nil {
		t.Error("nil tracing client call must fail")
	}
}

func TestTracingLLMClient_GroundedWebSearch_PropagatesProviderError(t *testing.T) {
	t.Parallel()
	wantErr := errors.New("meta grounded search: request failed with status 429: code=rate_limited type=throttle")
	m := &groundedMock{supports: true, handler: func(_ context.Context, _ string) (*types.GroundedWebSearchResult, error) {
		return nil, wantErr
	}}
	tc := NewTracingLLMClient(m, nil)
	_, err := tc.GroundedWebSearch(context.Background(), "q")
	if err == nil || !errors.Is(err, wantErr) && err.Error() != wantErr.Error() {
		t.Fatalf("expected provider error, got %v", err)
	}
}

func TestTracingLLMClient_GroundedWebSearch_StructuredUsage(t *testing.T) {
	m := &groundedMock{supports: true, handler: func(_ context.Context, _ string) (*types.GroundedWebSearchResult, error) {
		return &types.GroundedWebSearchResult{
			Text:      "ans",
			Citations: []types.GroundedCitation{{URL: "https://x.com"}},
			Usage:     types.GroundedUsage{InputTokens: 7, OutputTokens: 8, TotalTokens: 15},
		}, nil
	}}
	tc := NewTracingLLMClient(m, nil)
	res, _ := tc.GroundedWebSearch(context.Background(), "q")
	data, _ := json.Marshal(res)
	if !strings.Contains(string(data), `"usage"`) || !strings.Contains(string(data), `"citations"`) {
		t.Errorf("structured result missing fields: %s", data)
	}
}

// Ensure raw Meta transport still sanitizes: hit real OpenAICompatClient via TracingLLMClient
func TestTracingLLMClient_GroundedWebSearch_RawSanitizationViaRealClient(t *testing.T) {
	const secret = "super-secret-reasoning"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		_, _ = w.Write([]byte(`{"error":{"message":"` + secret + `","code":"internal_error","type":"server_error"}}`))
	}))
	defer srv.Close()
	cfg := DefaultOpenAICompatConfig(ProviderMeta, "test-key")
	cfg.BaseURL = srv.URL
	cfg.Timeout = 5 * time.Second
	c, err := NewOpenAICompatClient(cfg)
	if err != nil {
		t.Fatalf("NewOpenAICompatClient: %v", err)
	}
	tc := NewTracingLLMClient(c, nil)
	_, err = tc.GroundedWebSearch(context.Background(), "q")
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), secret) {
		t.Errorf("tracing leaked reasoning via raw client: %q", err.Error())
	}
}
