package perception

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"codenerd/internal/types"
)

func newTestCompatClient(t *testing.T, vendor Provider, baseURL string) *OpenAICompatClient {
	t.Helper()
	cfg := DefaultOpenAICompatConfig(vendor, "test-key")
	cfg.BaseURL = baseURL
	cfg.Timeout = 5 * time.Second
	c, err := NewOpenAICompatClient(cfg)
	if err != nil {
		t.Fatalf("NewOpenAICompatClient(%s): %v", vendor, err)
	}
	return c
}

func TestNewOpenAICompatClient_RequiresKeyAndBaseURL(t *testing.T) {
	if _, err := NewOpenAICompatClient(OpenAICompatConfig{Vendor: ProviderMeta, BaseURL: "https://x/v1"}); err == nil {
		t.Fatal("expected error when API key is empty")
	}
	if _, err := NewOpenAICompatClient(OpenAICompatConfig{Vendor: "unknown-vendor", APIKey: "k"}); err == nil {
		t.Fatal("expected error when base URL cannot be defaulted")
	}
}

func TestDefaultOpenAICompatConfig_VendorEndpoints(t *testing.T) {
	cases := map[Provider]struct{ base, model string }{
		ProviderDashScope: {"https://dashscope-intl.aliyuncs.com/compatible-mode/v1", "qwen3.8-max"},
		ProviderMeta:      {"https://api.meta.ai/v1", "muse-spark-1.2-contributor"},
		ProviderMoonshot:  {"https://api.moonshot.ai/v1", "kimi-k3"},
	}
	for vendor, want := range cases {
		got := DefaultOpenAICompatConfig(vendor, "k")
		if got.BaseURL != want.base {
			t.Errorf("%s base = %q, want %q", vendor, got.BaseURL, want.base)
		}
		if got.Model != want.model {
			t.Errorf("%s model = %q, want %q", vendor, got.Model, want.model)
		}
		if !IsOpenAICompatProvider(vendor) {
			t.Errorf("IsOpenAICompatProvider(%s) = false", vendor)
		}
	}
	if IsOpenAICompatProvider(ProviderGemini) {
		t.Error("gemini must not route through the OpenAI-compatible client")
	}
}

// Meta deprecates max_tokens and rejects reasoning_effort:"none" with HTTP 400,
// and is tuned to run at default sampling.
func TestBuildRequest_MetaFieldShape(t *testing.T) {
	c := newTestCompatClient(t, ProviderMeta, "https://api.meta.ai/v1")

	ctx := context.WithValue(context.Background(), types.CtxKeyModelCapability, types.CapabilityHighReasoning)
	req := c.buildRequest(ctx, []OpenAIMessage{{Role: "user", Content: "hi"}}, true)

	if req.MaxTokens != 0 {
		t.Errorf("MaxTokens = %d, want 0 (Meta uses max_completion_tokens)", req.MaxTokens)
	}
	if req.MaxCompletionTokens == 0 {
		t.Error("MaxCompletionTokens must be set for Meta")
	}
	if req.ReasoningEffort != "high" {
		t.Errorf("ReasoningEffort = %q, want high for CapabilityHighReasoning", req.ReasoningEffort)
	}
	if req.EnableThinking != nil {
		t.Error("enable_thinking is a DashScope field and must not be sent to Meta")
	}
	if req.Temperature != 0 || req.TopP != 0 {
		t.Errorf("Meta should use default sampling, got temp=%v top_p=%v", req.Temperature, req.TopP)
	}
}

func TestBuildRequest_MetaNeverSendsEffortNone(t *testing.T) {
	c := newTestCompatClient(t, ProviderMeta, "https://api.meta.ai/v1")

	for _, capability := range []types.ModelCapability{
		types.CapabilityHighReasoning,
		types.CapabilityBalanced,
		types.CapabilityHighSpeed,
	} {
		ctx := context.WithValue(context.Background(), types.CtxKeyModelCapability, capability)
		req := c.buildRequest(ctx, nil, true)
		if req.ReasoningEffort == "none" {
			t.Fatalf("capability %s produced reasoning_effort=none, which Muse Spark rejects with HTTP 400", capability)
		}
		if req.ReasoningEffort == "" {
			t.Errorf("capability %s produced no reasoning_effort", capability)
		}
	}

	// With no capability hint the vendor default must stand.
	req := c.buildRequest(context.Background(), nil, true)
	if req.ReasoningEffort != "" {
		t.Errorf("unhinted request set reasoning_effort=%q, want vendor default", req.ReasoningEffort)
	}
}

func TestBuildRequest_DashScopeThinking(t *testing.T) {
	c := newTestCompatClient(t, ProviderDashScope, "https://dashscope-intl.aliyuncs.com/compatible-mode/v1")

	req := c.buildRequest(context.Background(), nil, true)
	if req.EnableThinking == nil || !*req.EnableThinking {
		t.Fatal("DashScope request must set enable_thinking=true")
	}
	if req.Temperature != 0.6 || req.TopP != 0.95 {
		t.Errorf("thinking-mode sampling = temp %v / top_p %v, want 0.6 / 0.95", req.Temperature, req.TopP)
	}
	if req.MaxTokens == 0 {
		t.Error("DashScope uses max_tokens")
	}
	if req.ReasoningEffort != "" {
		t.Error("reasoning_effort is a Meta field and must not be sent to DashScope")
	}

	// enable_thinking:false must be transmitted, not omitted — "off" and "unset"
	// are different requests.
	off := c.buildRequest(context.Background(), nil, false)
	if off.EnableThinking == nil {
		t.Fatal("enable_thinking must be present even when false")
	}
	if *off.EnableThinking {
		t.Error("enable_thinking should be false when thinking is disabled")
	}
	encoded, err := json.Marshal(off)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(encoded), `"enable_thinking":false`) {
		t.Errorf("enable_thinking:false was dropped from payload: %s", encoded)
	}
}

// A per-shard model override in the context must win over the client default.
func TestModelForContext_OverridesDefault(t *testing.T) {
	c := newTestCompatClient(t, ProviderMeta, "https://api.meta.ai/v1")

	// The default is the contributor tier — see normalizeMetaModel. This
	// asserted the plain "muse-spark-1.2", which is the wrong commercial tier
	// and is what let 482 completions in one day run on it.
	if got := c.ModelForContext(context.Background()); got != "muse-spark-1.2-contributor" {
		t.Errorf("default model = %q, want muse-spark-1.2-contributor", got)
	}
	ctx := context.WithValue(context.Background(), types.CtxKeyModelName, "muse-spark-1.2-contributor")
	if got := c.ModelForContext(ctx); got != "muse-spark-1.2-contributor" {
		t.Errorf("context model = %q, want muse-spark-1.2-contributor", got)
	}
	req := c.buildRequest(ctx, nil, false)
	if req.Model != "muse-spark-1.2-contributor" {
		t.Errorf("request model = %q, per-shard override not applied", req.Model)
	}
}

// reasoning_content must never be folded into content: callers parse content as
// JSON for structured output and a prose preamble breaks that parse.
func TestCompleteWithSystem_ExcludesReasoningContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("Authorization = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"choices":[{"index":0,"finish_reason":"stop","message":{
				"role":"assistant",
				"content":"{\"answer\":42}",
				"reasoning_content":"Let me think step by step about this..."
			}}],
			"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}
		}`))
	}))
	defer srv.Close()

	c := newTestCompatClient(t, ProviderDashScope, srv.URL)
	got, err := c.CompleteWithSystem(context.Background(), "sys", "user")
	if err != nil {
		t.Fatalf("CompleteWithSystem: %v", err)
	}
	if got != `{"answer":42}` {
		t.Errorf("content = %q, want the JSON payload alone", got)
	}
	if strings.Contains(got, "step by step") {
		t.Error("reasoning_content leaked into the returned content")
	}
}

func TestExecuteChat_SurfacesAPIErrorEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"bad model","type":"invalid_request_error","code":"model_not_found"}}`))
	}))
	defer srv.Close()

	c := newTestCompatClient(t, ProviderMeta, srv.URL)
	_, err := c.CompleteWithSystem(context.Background(), "sys", "user")
	if err == nil {
		t.Fatal("expected an error for HTTP 400")
	}
	if !strings.Contains(err.Error(), "bad model") {
		t.Errorf("error did not surface the vendor message: %v", err)
	}
}

// Meta's contributor tier is capped at 60 RPM and sends Retry-After; honouring
// it beats guessing with a fixed backoff.
func TestRetryDelay_HonorsRetryAfter(t *testing.T) {
	resp := &http.Response{Header: http.Header{}}
	resp.Header.Set("Retry-After", "7")
	if got := retryDelay(resp, 0); got != 7*time.Second {
		t.Errorf("retryDelay = %v, want 7s", got)
	}

	// A hostile or buggy header must not stall a shard indefinitely.
	resp.Header.Set("Retry-After", "99999")
	if got := retryDelay(resp, 0); got != 60*time.Second {
		t.Errorf("retryDelay = %v, want the 60s cap", got)
	}

	// No header falls back to exponential backoff.
	if got := retryDelay(&http.Response{Header: http.Header{}}, 2); got != 4*time.Second {
		t.Errorf("backoff = %v, want 4s", got)
	}
	if got := retryDelay(nil, 0); got != time.Second {
		t.Errorf("nil-response backoff = %v, want 1s", got)
	}
}

func TestCompleteWithTools_MapsToolCalls(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req OpenAIRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if len(req.Tools) != 1 || req.Tools[0].Function.Name != "read_file" {
			t.Errorf("tools not forwarded: %+v", req.Tools)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"choices":[{"index":0,"finish_reason":"tool_calls","message":{
				"role":"assistant","content":"",
				"tool_calls":[{"id":"call_1","type":"function","function":{"name":"read_file","arguments":"{\"path\":\"a.go\"}"}}]
			}}],
			"usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}
		}`))
	}))
	defer srv.Close()

	c := newTestCompatClient(t, ProviderMoonshot, srv.URL)
	resp, err := c.CompleteWithTools(context.Background(), "sys", "user", []ToolDefinition{
		{Name: "read_file", Description: "read a file", InputSchema: map[string]any{"type": "object"}},
	})
	if err != nil {
		t.Fatalf("CompleteWithTools: %v", err)
	}
	if resp.StopReason != "tool_use" {
		t.Errorf("StopReason = %q, want tool_use (normalized from tool_calls)", resp.StopReason)
	}
	if len(resp.ToolCalls) != 1 || resp.ToolCalls[0].Name != "read_file" {
		t.Fatalf("tool calls = %+v", resp.ToolCalls)
	}
	if resp.ToolCalls[0].Input["path"] != "a.go" {
		t.Errorf("tool args = %+v", resp.ToolCalls[0].Input)
	}
}

// The client must satisfy both the base interface and the multi-turn tool
// interface, so the session executor runs a real agentic loop rather than
// degrading to single-turn tool use.
func TestOpenAICompatClient_ImplementsInterfaces(t *testing.T) {
	var c any = &OpenAICompatClient{}
	if _, ok := c.(types.LLMClient); !ok {
		t.Error("OpenAICompatClient does not implement types.LLMClient")
	}
	if _, ok := c.(types.ToolResultsProvider); !ok {
		t.Error("OpenAICompatClient does not implement types.ToolResultsProvider")
	}
}
