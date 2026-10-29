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
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/types"
)

// A thinking block is valid only under the prefix it was made under. The
// working policy opens raw search mid-loop, which changes the tools list, so
// the first follow-up carried a thinking block the endpoint rejected: every
// reviewer turn on claude-opus-5-5 failed with a 400 until the request told
// the endpoint to drop a block whose prefix moved.
const bindingRejection = "messages.1.content.0: Invalid `signature` in `thinking` block. The block is bound to a different conversation. " +
	"The `tools` list differs from the one this block was created with."

type bindingEndpoint struct {
	mu       sync.Mutex
	requests []map[string]json.RawMessage
	betas    []string
	firstSet string
}

func (e *bindingEndpoint) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	var req map[string]json.RawMessage
	_ = json.Unmarshal(body, &req)
	e.mu.Lock()
	defer e.mu.Unlock()
	e.requests = append(e.requests, req)
	e.betas = append(e.betas, r.Header.Get("anthropic-beta"))
	w.Header().Set("Content-Type", "application/json")
	if len(e.requests) == 1 {
		e.firstSet = string(req["tools"])
		io.WriteString(w, `{"type":"message","content":[`+
			`{"type":"thinking","thinking":"","signature":"sig-1"},`+
			`{"type":"tool_use","id":"toolu_1","name":"read_file","input":{"path":"a.go"}}],`+
			`"stop_reason":"tool_use","usage":{"input_tokens":4,"output_tokens":5}}`)
		return
	}
	var thinking struct {
		BlockBinding *struct {
			PrefixMismatchBehavior string `json:"prefix_mismatch_behavior"`
		} `json:"block_binding"`
	}
	_ = json.Unmarshal(req["thinking"], &thinking)
	drops := thinking.BlockBinding != nil && thinking.BlockBinding.PrefixMismatchBehavior == "drop_block" &&
		strings.Contains(r.Header.Get("anthropic-beta"), config.AnthropicThinkingBindingBeta)
	if string(req["tools"]) != e.firstSet && !drops {
		w.Header().Set("request-id", "req_binding")
		w.WriteHeader(http.StatusBadRequest)
		msg, _ := json.Marshal(bindingRejection)
		io.WriteString(w, `{"type":"error","error":{"type":"invalid_request_error","message":`+string(msg)+`}}`)
		return
	}
	io.WriteString(w, anthropicJSONReply)
}

func bindingClient(t *testing.T, model, url string) *AnthropicClient {
	t.Helper()
	return NewAnthropicClientWithConfig(AnthropicConfig{APIKey: "test-key", BaseURL: url, Model: model, Timeout: 30e9})
}

func bindingToolLoop(t *testing.T, client *AnthropicClient) error {
	t.Helper()
	ctx := context.Background()
	first := []types.ToolDefinition{{Name: "read_file", InputSchema: map[string]any{"type": "object"}}}
	grown := append(append([]types.ToolDefinition(nil), first...), types.ToolDefinition{Name: "grep", InputSchema: map[string]any{"type": "object"}})
	resp, err := client.CompleteWithTools(ctx, "sys", "review", first)
	if err != nil {
		t.Fatalf("first request: %v", err)
	}
	history := []types.Message{
		{Role: "user", Text: "review"},
		types.AssistantMessageFrom(resp),
		{Role: "user", ToolResults: []types.ToolResult{{ToolUseID: "toolu_1", Content: "package a"}}},
	}
	_, err = client.CompleteWithToolResults(ctx, "sys", history, grown)
	return err
}

func TestAnthropicRequest_ThinkingBlocksSurviveACatalogChange(t *testing.T) {
	endpoint := &bindingEndpoint{}
	srv := httptest.NewServer(endpoint)
	defer srv.Close()
	if err := bindingToolLoop(t, bindingClient(t, "claude-opus-5-5", srv.URL)); err != nil {
		t.Fatalf("the follow-up under a changed tool catalog failed: %v", err)
	}
	for i, req := range endpoint.requests {
		var thinking AnthropicThinking
		if err := json.Unmarshal(req["thinking"], &thinking); err != nil {
			t.Fatalf("request %d thinking: %v (%s)", i, err, req["thinking"])
		}
		if thinking.Type != "adaptive" || thinking.BlockBinding == nil || thinking.BlockBinding.PrefixMismatchBehavior != "drop_block" {
			t.Fatalf("request %d thinking = %s, want adaptive with block_binding drop_block", i, req["thinking"])
		}
		if endpoint.betas[i] != config.AnthropicThinkingBindingBeta {
			t.Fatalf("request %d anthropic-beta = %q, want %q", i, endpoint.betas[i], config.AnthropicThinkingBindingBeta)
		}
		var maxTokens int
		_ = json.Unmarshal(req["max_tokens"], &maxTokens)
		if maxTokens <= 0 {
			t.Fatalf("request %d max_tokens = %s", i, req["max_tokens"])
		}
	}
}

func TestAnthropicRequest_ModelsWithoutTheCheckAreSentNoBinding(t *testing.T) {
	for _, model := range []string{"claude-opus-4-6", "claude-sonnet-5", "claude-opus-5", "claude-test"} {
		endpoint := &bindingEndpoint{}
		srv := httptest.NewServer(endpoint)
		client := bindingClient(t, model, srv.URL)
		_, _ = client.CompleteWithSystem(context.Background(), "sys", "hi")
		srv.Close()
		if len(endpoint.requests) == 0 {
			t.Fatalf("%s: nothing was sent", model)
		}
		req := endpoint.requests[0]
		if strings.Contains(string(req["thinking"]), "block_binding") || endpoint.betas[0] != "" {
			t.Fatalf("%s sent thinking=%s anthropic-beta=%q; it does not run the check", model, req["thinking"], endpoint.betas[0])
		}
	}
}

// Without the binding control the endpoint's rejection is the failure this
// file exists for; it must reach the operator whole and the user safely.
func TestAnthropicRejection_ReachesTheOperatorThroughThePoolAndTheUserSafely(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("request-id", "req_visible")
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, `{"type":"error","error":{"type":"invalid_request_error","message":"max_tokens: 99999 > 64000, the maximum allowed for claude-opus-5-5"}}`)
	}))
	defer srv.Close()
	client, err := NewClientFromConfig(testPoolConfig(t, srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.CompleteWithSystem(context.Background(), "sys", "hi")
	if err == nil {
		t.Fatal("a 400 produced no error")
	}
	var pooled *pooledCallError
	var api *AnthropicAPIError
	if !errors.As(err, &pooled) || !errors.As(err, &api) {
		t.Fatalf("pool dropped the typed errors: %T %v", err, err)
	}
	if api.Type != "invalid_request_error" || api.RequestID != "req_visible" || !strings.Contains(api.Message, "max_tokens: 99999") {
		t.Fatalf("typed error lost the provider's diagnostic: %+v", api)
	}
	for _, want := range []string{"status 400", "invalid_request_error", "req_visible", "max_tokens: 99999 > 64000"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("operator text %q lacks %q", err.Error(), want)
		}
	}
	if ClassifyProviderFailure(err) != FailureInvalidRequest {
		t.Fatalf("class = %s", ClassifyProviderFailure(err))
	}
	user := UserFailureText(fmt.Errorf("tool-result follow-up failed: %w", err))
	if !strings.HasPrefix(user, "tool-result follow-up failed: ") {
		t.Fatalf("the caller's wrapping was lost: %q", user)
	}
	if strings.Contains(user, "max_tokens") || strings.Contains(user, "req_visible") || !strings.Contains(user, SafeProviderFailureMessage(err)) {
		t.Fatalf("user text is not the safe message: %q", user)
	}
}
