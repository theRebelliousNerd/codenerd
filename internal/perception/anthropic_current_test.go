package perception

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"codenerd/internal/broker"
	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/types"
	"codenerd/internal/usage"
)

const anthropicClosedSchema = `{"type":"object","properties":{"answer":{"type":"string","minLength":1}},"required":["answer"],"additionalProperties":false}`
const anthropicJSONReply = `{"type":"message","content":[{"type":"text","text":"{\"answer\":\"yes\"}"}],"stop_reason":"end_turn","usage":{"input_tokens":4,"output_tokens":5}}`

func anthropicSSE(reason string) string {
	return "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":4,\"cache_read_input_tokens\":10,\"cache_creation_input_tokens\":2}}}\n\n" +
		"event: content_block_start\ndata: {\"type\":\"content_block_start\",\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n" +
		"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"hello\"}}\n\n" +
		fmt.Sprintf("event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":%q},\"usage\":{\"output_tokens\":5}}\n\n", reason) +
		"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"
}

func drainAnthropic(t *testing.T, content <-chan string, errs <-chan error) (string, error) {
	t.Helper()
	var text strings.Builder
	for part := range content {
		text.WriteString(part)
	}
	var result error
	for err := range errs {
		if err != nil {
			result = err
		}
	}
	return text.String(), result
}

func TestAnthropicCurrentAPI_AllInferenceSurfaces(t *testing.T) {
	for _, surface := range []string{"chat", "schema", "tools", "history", "stream"} {
		t.Run(surface, func(t *testing.T) {
			var captured map[string]any
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v1/messages" || r.Method != "POST" {
					t.Errorf("route = %s %s", r.Method, r.URL.Path)
				}
				if r.Header.Get("anthropic-version") != config.AnthropicAPIVersion || r.Header.Get("x-api-key") != "test-key" || r.Header.Get("anthropic-workspace-id") != "wrkspc_test" {
					t.Errorf("request contract headers missing")
				}
				if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
					t.Error(err)
				}
				if surface == "stream" {
					w.Header().Set("Content-Type", "text/event-stream")
					io.WriteString(w, anthropicSSE("end_turn"))
					return
				}
				w.Header().Set("Content-Type", "application/json")
				if surface == "schema" {
					io.WriteString(w, anthropicJSONReply)
				} else {
					io.WriteString(w, anthropicChatOK)
				}
			}))
			defer srv.Close()
			c := NewAnthropicClientWithConfig(AnthropicConfig{APIKey: "test-key", BaseURL: srv.URL + "/v1/", Model: "claude-opus-5-5", ReasoningEffort: "high", Options: config.AnthropicProviderConfig{WorkspaceID: "wrkspc_test", Thinking: "adaptive"}})
			c.EnableSystemCaching()
			var err error
			switch surface {
			case "chat":
				_, err = c.CompleteWithSystem(context.Background(), "sys", "user")
			case "schema":
				_, err = c.CompleteWithSchema(context.Background(), "sys", "user", anthropicClosedSchema)
			case "tools":
				_, err = c.CompleteWithTools(context.Background(), "sys", "user", nil)
			case "history":
				_, err = c.CompleteWithToolResults(context.Background(), "sys", []types.Message{types.NewUserMessage(types.TextBlock("user"))}, nil)
			case "stream":
				ch, errs := c.CompleteWithStreaming(context.Background(), "sys", "user", false)
				_, err = drainAnthropic(t, ch, errs)
			}
			if err != nil {
				t.Fatal(err)
			}
			for _, key := range []string{"temperature", "top_p", "top_k"} {
				if _, exists := captured[key]; exists {
					t.Fatalf("removed parameter %q was sent", key)
				}
			}
			if captured["output_config"].(map[string]any)["effort"] != "high" {
				t.Fatal("effort lost, including cached request")
			}
			if captured["thinking"].(map[string]any)["type"] != "adaptive" {
				t.Fatal("thinking lost")
			}
			if _, ok := captured["system"].([]any); !ok {
				t.Fatal("system cache marker missing")
			}
		})
	}
}

// The effort is the config's: the slot's configured effort, or a shard
// profile's riding the request, is sent to every model that accepts one and to
// no model that does not. A capability hint on the context chooses nothing, an
// effort is never invented, and a profile effort Anthropic cannot take fails
// the request instead of reaching the wire.
func TestAnthropicReasoning_SendsTheConfiguredEffortOnly(t *testing.T) {
	tests := []struct {
		name          string
		model         string
		effort        string
		ctx           func() context.Context
		wantEffort    string // "" means output_config is omitted
		wantThinking  string // "" means no thinking block
		wantErrSubstr string
	}{
		{name: "configured effort on a current model", model: "claude-haiku-5-5", effort: "medium",
			ctx: context.Background, wantEffort: "medium", wantThinking: "adaptive"},
		{name: "a capability hint does not change it", model: "claude-sonnet-4-6", effort: "low",
			ctx: func() context.Context {
				return types.WithModelCapability(context.Background(), types.CapabilityHighReasoning)
			},
			wantEffort: "low", wantThinking: "adaptive"},
		{name: "no effort configured, none invented", model: "claude-sonnet-4-6",
			ctx: func() context.Context {
				return types.WithModelCapability(context.Background(), types.CapabilityHighReasoning)
			}},
		{name: "a shard profile's effort wins", model: "claude-opus-5-5", effort: "medium",
			ctx:        func() context.Context { return types.WithReasoningEffort(context.Background(), "max") },
			wantEffort: "max", wantThinking: "adaptive"},
		{name: "a model that takes no effort is not sent one", model: "claude-haiku-4-5", effort: "low",
			ctx: context.Background},
		{name: "a profile effort Anthropic cannot take fails", model: "claude-opus-5-5", effort: "high",
			ctx:           func() context.Context { return types.WithReasoningEffort(context.Background(), "minimal") },
			wantErrSubstr: "shard profile reasoning_effort"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewAnthropicClientWithConfig(AnthropicConfig{APIKey: "k", Model: tt.model, ReasoningEffort: tt.effort})
			req, err := c.request(tt.ctx(), "sys", nil, nil)
			if tt.wantErrSubstr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErrSubstr) {
					t.Fatalf("request err = %v, want one containing %q", err, tt.wantErrSubstr)
				}
				return
			}
			if err != nil {
				t.Fatalf("request: %v", err)
			}
			gotEffort := ""
			if req.OutputConfig != nil {
				gotEffort = req.OutputConfig.Effort
			}
			if gotEffort != tt.wantEffort {
				t.Errorf("effort = %q, want %q", gotEffort, tt.wantEffort)
			}
			gotThinking := ""
			if req.Thinking != nil {
				gotThinking = req.Thinking.Type
			}
			if gotThinking != tt.wantThinking {
				t.Errorf("thinking = %q, want %q", gotThinking, tt.wantThinking)
			}
		})
	}
}

func TestAnthropicSchema_EnforcesOriginalConstraintsAndRejectsOpenMaps(t *testing.T) {
	for _, tc := range []struct {
		answer  string
		wantErr bool
	}{{"yes", false}, {"", true}} {
		t.Run(fmt.Sprintf("answer=%q", tc.answer), func(t *testing.T) {
			text, _ := json.Marshal(map[string]any{"answer": tc.answer})
			reply, _ := json.Marshal(map[string]any{"type": "message", "content": []any{map[string]any{"type": "text", "text": string(text)}}, "stop_reason": "end_turn"})
			srv, capture := captureAnthropic(t, string(reply))
			c := anthropicTestClient(srv.URL)
			result, err := c.CompleteWithSchema(context.Background(), "sys", "user", anthropicClosedSchema)
			if (err != nil) != tc.wantErr {
				t.Fatalf("result=%q err=%v", result, err)
			}
			schema := capture()["output_config"].(map[string]any)["format"].(map[string]any)["schema"].(map[string]any)
			answer := schema["properties"].(map[string]any)["answer"].(map[string]any)
			if _, ok := answer["minLength"]; ok {
				t.Fatal("unsupported minLength sent to API")
			}
		})
	}
	for _, schema := range []string{`null`, `{}`, `{"type":"object"}`, `{"$ref":"file:///etc/passwd"}`, `{"$ref":"https://example.invalid/schema"}`} {
		c := anthropicTestClient("http://127.0.0.1:1")
		if _, err := c.CompleteWithSchema(context.Background(), "sys", "user", schema); err == nil {
			t.Fatalf("schema accepted: %s", schema)
		}
	}
	var understanding map[string]any
	if err := json.Unmarshal([]byte(understandingSchema), &understanding); err != nil {
		t.Fatal(err)
	}
	if _, _, err := prepareAnthropicSchema(understanding); err != nil {
		t.Fatalf("codeNERD understanding schema: %v", err)
	}
}

func TestAnthropicStrictTools_ValidatesOriginalSchemaAndPreservesThinking(t *testing.T) {
	defs := []ToolDefinition{{Name: "read", InputSchema: map[string]any{"type": "object", "properties": map[string]any{"path": map[string]any{"type": "string", "minLength": float64(1)}}, "required": []string{"path"}, "additionalProperties": false}}}
	for _, path := range []string{"file.go", ""} {
		t.Run(fmt.Sprintf("path=%q", path), func(t *testing.T) {
			input, _ := json.Marshal(map[string]string{"path": path})
			reply := `{"type":"message","content":[{"type":"thinking","thinking":"","signature":"signed"},{"type":"tool_use","id":"call_1","name":"read","input":` + string(input) + `}],"stop_reason":"tool_use"}`
			srv, capture := captureAnthropic(t, reply)
			c := anthropicTestClient(srv.URL)
			c.options.StrictTools = true
			out, err := c.CompleteWithTools(context.Background(), "sys", "user", defs)
			if path == "" {
				if err == nil {
					t.Fatal("invalid tool arguments reached harness")
				}
				return
			}
			if err != nil || len(out.Blocks) != 2 || out.Blocks[0].Signature != "signed" {
				t.Fatalf("out=%#v err=%v", out, err)
			}
			if capture()["tools"].([]any)[0].(map[string]any)["strict"] != true {
				t.Fatal("strict flag missing")
			}
			if defs[0].InputSchema["properties"].(map[string]any)["path"].(map[string]any)["minLength"] != float64(1) {
				t.Fatal("caller schema mutated")
			}
		})
	}
}

func TestAnthropicStream_RejectsTruncationMalformedDataAndProviderErrors(t *testing.T) {
	cases := []struct {
		name, stream string
		class        ProviderFailureClass
		truncated    bool
	}{
		{"complete", anthropicSSE("end_turn"), "", false},
		{"length", anthropicSSE("max_tokens"), "", true},
		{"eof", strings.ReplaceAll(anthropicSSE("end_turn"), "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n", ""), FailurePermanent, false},
		{"malformed", "data: {bad\n\n", FailurePermanent, false},
		{"overload", "data: {\"type\":\"error\",\"error\":{\"type\":\"overloaded_error\",\"message\":\"secret\"}}\n\n", FailureTransient, false},
		{"refusal", anthropicSSE("refusal"), FailurePermanent, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var hits atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				w.Header().Set("Content-Type", "text/event-stream")
				io.WriteString(w, tc.stream)
			}))
			defer srv.Close()
			rec := &usageRecorder{}
			ctx := usage.WithObserver(context.Background(), rec)
			ch, errs := anthropicTestClient(srv.URL).CompleteWithStreaming(ctx, "sys", "user", false)
			text, err := drainAnthropic(t, ch, errs)
			if tc.name == "complete" {
				if err != nil || text != "hello" {
					t.Fatalf("text=%q err=%v", text, err)
				}
				if rec.input != 16 || rec.output != 5 {
					t.Fatalf("usage=%+v", rec)
				}
				return
			}
			if err == nil {
				t.Fatal("incomplete stream accepted")
			}
			if strings.Contains(err.Error(), "secret") {
				t.Fatal("provider error body leaked")
			}
			if tc.truncated {
				var truncated *types.OutputTruncated
				if !errors.As(err, &truncated) || truncated.Partial != "hello" || truncated.OutputTokens != 5 {
					t.Fatalf("truncation=%v", err)
				}
			} else if ClassifyProviderFailure(err) != tc.class {
				t.Fatalf("class=%s err=%v", ClassifyProviderFailure(err), err)
			}
			if hits.Load() != 1 {
				t.Fatal("stream replayed after start")
			}
		})
	}
}

func TestAnthropicStream_CancelBlockedReader(t *testing.T) {
	started := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		w.(http.Flusher).Flush()
		close(started)
		<-r.Context().Done()
	}))
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch, errs := anthropicTestClient(srv.URL).CompleteWithStreaming(ctx, "sys", "user", false)
	<-started
	cancel()
	_, err := drainAnthropic(t, ch, errs)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
}

func TestAnthropicTransport_RetriesStreamingHandshakeAndHonorsRetryAfter(t *testing.T) {
	prev := config.GetLLMTimeouts()
	fast := prev
	fast.MaxRetries = 1
	fast.RetryBackoffBase = time.Millisecond
	fast.RetryBackoffMax = time.Millisecond
	config.SetLLMTimeouts(fast)
	defer config.SetLLMTimeouts(prev)
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) == 1 {
			w.Header().Set("Retry-After", "0.15")
			w.WriteHeader(429)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, anthropicSSE("end_turn"))
	}))
	defer srv.Close()
	start := time.Now()
	ch, errs := anthropicTestClient(srv.URL).CompleteWithStreaming(context.Background(), "sys", "user", false)
	_, err := drainAnthropic(t, ch, errs)
	if err != nil || hits.Load() != 2 || time.Since(start) < 140*time.Millisecond {
		t.Fatalf("hits=%d elapsed=%s err=%v", hits.Load(), time.Since(start), err)
	}
	for _, value := range []string{"2", "99999999999999999999999999", time.Now().Add(time.Minute).UTC().Format(http.TimeFormat)} {
		if anthropicRetryAfter(value, time.Now()) < time.Second {
			t.Fatalf("Retry-After %q ignored", value)
		}
	}
}

func TestAnthropicTransport_DoesNotForwardCredentialsOnRedirect(t *testing.T) {
	var leaked atomic.Bool
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { leaked.Store(true) }))
	defer target.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer srv.Close()
	_, err := anthropicTestClient(srv.URL).Complete(context.Background(), "user")
	if err == nil || leaked.Load() {
		t.Fatalf("redirect followed=%v err=%v", leaked.Load(), err)
	}
}

func TestAnthropicFactory_PreservesMainAndSecondarySettings(t *testing.T) {
	cfg := &config.UserConfig{Provider: "anthropic", Model: "claude-opus-5-5", AnthropicAPIKey: "k", BaseURL: "https://gateway.invalid/v1", ReasoningEffort: "high", Anthropic: &config.AnthropicProviderConfig{WorkspaceID: "wrkspc_main", Thinking: "adaptive"},
		Worker:  &config.SecondaryLLMConfig{Provider: "anthropic", Model: "claude-haiku-5-5", Endpoint: "https://worker.invalid/v1", ReasoningEffort: "low", Anthropic: &config.AnthropicProviderConfig{WorkspaceID: "wrkspc_worker"}},
		Planner: &config.SecondaryLLMConfig{Provider: "anthropic", Model: "claude-opus-5-5", ReasoningEffort: "max"}}
	pc, err := ProviderConfigFromUserConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	main, err := NewClientFromConfig(pc)
	if err != nil {
		t.Fatal(err)
	}
	worker, err := NewWorkerClientFromUserConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	planner, err := NewPlannerClientFromUserConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		client                 LLMClient
		url, workspace, effort string
	}{{main, cfg.BaseURL, "wrkspc_main", "high"}, {worker, cfg.Worker.Endpoint, "wrkspc_worker", "low"}, {planner, cfg.BaseURL, "wrkspc_main", "max"}} {
		raw, ok := broker.Base(tc.client).(*AnthropicClient)
		if !ok {
			t.Fatalf("type=%T", broker.Base(tc.client))
		}
		if raw.baseURL != tc.url || raw.options.WorkspaceID != tc.workspace || raw.reasoningEffort != tc.effort {
			t.Fatalf("settings lost: %+v", raw)
		}
		if _, ok := core.AsSchemaCapable(tc.client); !ok {
			t.Fatal("broker masked native schema support")
		}
		traced := NewTracingLLMClient(tc.client, nil)
		if _, ok := core.AsSchemaCapable(traced); !ok {
			t.Fatal("tracing masked native schema support")
		}
		scheduled := &core.ScheduledLLMCall{Client: traced}
		if _, ok := core.AsSchemaCapable(scheduled); !ok {
			t.Fatal("scheduler masked native schema support")
		}
	}
	classification, err := newRawClassificationClientFromConfig(pc)
	if err != nil {
		t.Fatal(err)
	}
	c := classification.(*AnthropicClient)
	if c.options.WorkspaceID != "wrkspc_main" || c.baseURL != cfg.BaseURL || c.reasoningEffort != "low" {
		t.Fatalf("classification settings lost: %+v", c)
	}
	if brokerCredsFor(pc).WorkspaceID != "wrkspc_main" {
		t.Fatal("counter workspace lost")
	}
}

func TestAnthropicConcurrentModelAndCachingAccess(t *testing.T) {
	c := anthropicTestClient("http://127.0.0.1:1")
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				c.SetModel("claude-opus-5-5")
				c.EnableSystemCaching()
				_, _ = c.request(context.Background(), "sys", nil, nil)
				_ = c.systemCaching()
				_, _ = c.ModelIdentity()
			}
		}()
	}
	wg.Wait()
}

func TestAnthropicTransport_CancelsRateAndRetryAfterWaits(t *testing.T) {
	c := anthropicTestClient("http://127.0.0.1:1")
	c.lastRequest = time.Now().Add(time.Hour)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := c.Complete(ctx, "user")
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(start) > time.Second {
		t.Fatalf("rate wait: elapsed=%s err=%v", time.Since(start), err)
	}

	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Retry-After", "3600")
		w.WriteHeader(429)
	}))
	defer srv.Close()
	ctx, cancel = context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err = anthropicTestClient(srv.URL).Complete(ctx, "user")
	if !errors.Is(err, context.DeadlineExceeded) || hits.Load() != 1 {
		t.Fatalf("Retry-After wait: hits=%d err=%v", hits.Load(), err)
	}
}

func TestAnthropicAPIError_PreservesDiagnosticsWithoutResponseBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("request-id", "req_test")
		w.WriteHeader(401)
		io.WriteString(w, `{"error":{"type":"authentication_error","message":"secret-value"}}`)
	}))
	defer srv.Close()
	_, err := anthropicTestClient(srv.URL).Complete(context.Background(), "user")
	var apiErr *AnthropicAPIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 401 || apiErr.RequestID != "req_test" || apiErr.Type != "authentication_error" {
		t.Fatalf("diagnostics: %v", err)
	}
	if strings.Contains(err.Error(), "secret-value") || ClassifyProviderFailure(err) != FailureAuth {
		t.Fatalf("unsafe/misclassified error: %v", err)
	}
}

func TestAnthropicSchema_ThroughMeteringAndTracing(t *testing.T) {
	var inference, count atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("anthropic-version") != config.AnthropicAPIVersion || r.Header.Get("anthropic-workspace-id") != "wrkspc_test" {
			t.Error("inference/counting headers diverged")
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/messages/count_tokens":
			count.Add(1)
			io.WriteString(w, `{"input_tokens":4}`)
		case "/messages":
			inference.Add(1)
			io.WriteString(w, anthropicJSONReply)
		default:
			t.Errorf("path=%s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()
	client, err := NewClientFromConfig(&ProviderConfig{Provider: ProviderAnthropic, APIKey: "k", Model: "claude-opus-5-5", BaseURL: srv.URL + "/", Anthropic: &config.AnthropicProviderConfig{WorkspaceID: "wrkspc_test"}})
	if err != nil {
		t.Fatal(err)
	}
	traced := NewTracingLLMClient(client, nil)
	sc, ok := core.AsSchemaCapable(traced)
	if !ok {
		t.Fatal("schema interface lost")
	}
	result, err := sc.CompleteWithSchema(context.Background(), "sys", "user", anthropicClosedSchema)
	if err != nil || result != `{"answer":"yes"}` || inference.Load() != 1 || count.Load() < 1 {
		t.Fatalf("result=%q inference=%d count=%d err=%v", result, inference.Load(), count.Load(), err)
	}
}

func TestAnthropicHistory_ReplaysOmittedThinkingSummaryVerbatim(t *testing.T) {
	history := []types.Message{
		types.NewUserMessage(types.TextBlock("read the file")),
		types.NewAssistantMessage(types.ThinkingBlock("", "opaque-signed-thinking"), types.ToolUseBlock("call_1", "read", map[string]any{})),
		types.NewUserMessage(types.ToolResultBlock("call_1", "done", false)),
	}
	srv, capture := captureAnthropic(t, anthropicChatOK)
	_, err := anthropicTestClient(srv.URL).CompleteWithToolResults(context.Background(), "sys", history, nil)
	if err != nil {
		t.Fatal(err)
	}
	blocks := capture()["messages"].([]any)[1].(map[string]any)["content"].([]any)
	thinking := blocks[0].(map[string]any)
	text, exists := thinking["thinking"]
	if !exists || text != "" || thinking["signature"] != "opaque-signed-thinking" {
		t.Fatalf("signed thinking changed during replay: %#v", thinking)
	}
	if blocks[1].(map[string]any)["id"] != "call_1" {
		t.Fatal("thinking/tool order changed")
	}
}
