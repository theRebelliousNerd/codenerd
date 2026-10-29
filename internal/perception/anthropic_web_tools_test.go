package perception

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"codenerd/internal/broker"
	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/types"
)

// webToolServer is an Anthropic endpoint that answers inference requests from
// a script of replies, in order, and records every request body.
type webToolServer struct {
	mu       sync.Mutex
	replies  []string
	requests []map[string]any
	server   *httptest.Server
}

func newWebToolServer(t *testing.T, replies ...string) *webToolServer {
	t.Helper()
	ws := &webToolServer{replies: replies}
	ws.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/count_tokens") {
			io.WriteString(w, `{"input_tokens":4}`)
			return
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
		}
		ws.mu.Lock()
		ws.requests = append(ws.requests, body)
		n := len(ws.requests)
		reply := ws.replies[len(ws.replies)-1]
		if n <= len(ws.replies) {
			reply = ws.replies[n-1]
		}
		ws.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, reply)
	}))
	t.Cleanup(ws.server.Close)
	return ws
}

func (ws *webToolServer) request(t *testing.T, i int) map[string]any {
	t.Helper()
	ws.mu.Lock()
	defer ws.mu.Unlock()
	if i >= len(ws.requests) {
		t.Fatalf("request %d was never sent (%d sent)", i, len(ws.requests))
	}
	return ws.requests[i]
}

// serverToolsIn are the server tools a request body declares, as type ->
// max_uses.
func serverToolsIn(body map[string]any) map[string]float64 {
	out := map[string]float64{}
	tools, _ := body["tools"].([]any)
	for _, raw := range tools {
		tool, _ := raw.(map[string]any)
		if kind, _ := tool["type"].(string); kind != "" {
			uses, _ := tool["max_uses"].(float64)
			out[kind] = uses
		}
	}
	return out
}

func newWebToolClient(t *testing.T, base, model string, options config.AnthropicProviderConfig) *AnthropicClient {
	t.Helper()
	c := NewAnthropicClientWithConfig(AnthropicConfig{APIKey: "test-key", BaseURL: base, Model: model, Options: options})
	if c.configErr != nil {
		t.Fatalf("client config: %v", c.configErr)
	}
	return c
}

// Off by default: a request declares no server tool. On, it declares each
// opted-in tool in the version its model takes, with the per-request cap.
func TestAnthropicWebTools_DeclaredOnlyWhenOptedIn(t *testing.T) {
	tests := []struct {
		name    string
		model   string
		options config.AnthropicProviderConfig
		want    map[string]float64
	}{
		{name: "off by default", model: "claude-opus-5-5", want: map[string]float64{}},
		{name: "search on a dynamic model", model: "claude-opus-5-5",
			options: config.AnthropicProviderConfig{WebSearch: true},
			want:    map[string]float64{"web_search_20260209": 5}},
		{name: "both on a dynamic Sonnet with caps", model: "claude-sonnet-4-6",
			options: config.AnthropicProviderConfig{WebSearch: true, WebFetch: true, WebSearchMaxUses: 3, WebFetchMaxUses: 2},
			want:    map[string]float64{"web_search_20260209": 3, "web_fetch_20260209": 2}},
		{name: "Haiku 5.5 takes the basic versions", model: "claude-haiku-5-5",
			options: config.AnthropicProviderConfig{WebSearch: true, WebFetch: true},
			want:    map[string]float64{"web_search_20250305": 5, "web_fetch_20250910": 5}},
		{name: "Sonnet 4.5 takes the basic versions", model: "claude-sonnet-4-5",
			options: config.AnthropicProviderConfig{WebFetch: true},
			want:    map[string]float64{"web_fetch_20250910": 5}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ws := newWebToolServer(t, anthropicChatOK)
			c := newWebToolClient(t, ws.server.URL, tt.model, tt.options)
			if _, err := c.CompleteWithSystem(context.Background(), "sys", "what changed in Go 1.27?"); err != nil {
				t.Fatalf("CompleteWithSystem: %v", err)
			}
			got := serverToolsIn(ws.request(t, 0))
			if len(got) != len(tt.want) {
				t.Fatalf("server tools = %v, want %v", got, tt.want)
			}
			for kind, uses := range tt.want {
				if got[kind] != uses {
					t.Errorf("server tool %s max_uses = %v, want %v (all: %v)", kind, got[kind], uses, got)
				}
			}
		})
	}
}

// A searched turn: two searches (one result list, one error object), a fetch,
// and an answer citing a page. The sources are the results and the citations,
// once each; the error is skipped, not fatal; and no server block becomes a
// tool call the executor would run.
const anthropicSearchedReply = `{"type":"message","role":"assistant","model":"claude-opus-5-5","stop_reason":"end_turn",
"usage":{"input_tokens":10,"output_tokens":20,"server_tool_use":{"web_search_requests":2,"web_fetch_requests":1}},
"content":[
 {"type":"server_tool_use","id":"srvtoolu_1","name":"web_search","input":{"query":"go 1.27"}},
 {"type":"web_search_tool_result","tool_use_id":"srvtoolu_1","content":[
   {"type":"web_search_result","url":"https://go.dev/doc/go1.27","title":"Go 1.27","encrypted_content":"abc"},
   {"type":"web_search_result","url":"https://go.dev/blog/go1.27","title":"Blog","encrypted_content":"def"}]},
 {"type":"server_tool_use","id":"srvtoolu_2","name":"web_search","input":{"query":"go 1.27 again"}},
 {"type":"web_search_tool_result","tool_use_id":"srvtoolu_2","content":{"type":"web_search_tool_result_error","error_code":"max_uses_exceeded"}},
 {"type":"server_tool_use","id":"srvtoolu_3","name":"web_fetch","input":{"url":"https://go.dev/doc/go1.27"}},
 {"type":"web_fetch_tool_result","tool_use_id":"srvtoolu_3","content":{"type":"web_fetch_result","url":"https://go.dev/doc/go1.27","content":{"type":"document","source":{"type":"text","media_type":"text/plain","data":"..."}}}},
 {"type":"text","text":"Go 1.27 ships range-over-func.","citations":[{"type":"web_search_result_location","url":"https://go.dev/blog/go1.27","title":"Blog","encrypted_index":"x","cited_text":"..."},{"type":"web_search_result_location","url":"https://pkg.go.dev/iter","title":"iter","encrypted_index":"y","cited_text":"..."}]}
]}`

func TestAnthropicWebTools_GroundingFromASearchedTurn(t *testing.T) {
	ws := newWebToolServer(t, anthropicSearchedReply)
	c := newWebToolClient(t, ws.server.URL, "claude-opus-5-5", config.AnthropicProviderConfig{WebSearch: true, WebFetch: true})
	resp, err := c.CompleteWithToolResults(context.Background(), "sys", []types.Message{{Role: "user", Text: "what changed in Go 1.27?"}}, nil)
	if err != nil {
		t.Fatalf("CompleteWithToolResults: %v", err)
	}
	want := []string{"https://go.dev/doc/go1.27", "https://go.dev/blog/go1.27", "https://pkg.go.dev/iter"}
	if got := c.GetLastGroundingSources(); strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("GetLastGroundingSources = %v, want %v", got, want)
	}
	if len(resp.ToolCalls) != 0 {
		t.Fatalf("server tool use became %d tool call(s) for the executor: %+v", len(resp.ToolCalls), resp.ToolCalls)
	}
	if resp.Text != "Go 1.27 ships range-over-func." {
		t.Errorf("Text = %q", resp.Text)
	}

	// The assistant turn the executor keeps replays every server block byte
	// for byte, in position.
	assistant := types.AssistantMessageFrom(resp)
	if len(assistant.ToolCalls) != 0 {
		t.Fatalf("the replayed turn carries tool calls: %+v", assistant.ToolCalls)
	}
	replayed, err := buildAnthropicMessagesFromHistory([]types.Message{{Role: "user", Text: "q"}, assistant})
	if err != nil {
		t.Fatalf("buildAnthropicMessagesFromHistory: %v", err)
	}
	encoded, err := json.Marshal(replayed[1].Content)
	if err != nil {
		t.Fatal(err)
	}
	var original struct {
		Content []json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal([]byte(anthropicSearchedReply), &original); err != nil {
		t.Fatal(err)
	}
	// Verbatim up to insignificant whitespace: encoding/json compacts what a
	// Marshaler returns, deterministically, so every replay is the same bytes.
	for i := 0; i < 6; i++ {
		var block bytes.Buffer
		if err := json.Compact(&block, original.Content[i]); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(encoded), block.String()) {
			t.Errorf("server block %d was not replayed verbatim:\n%s", i, block.String())
		}
	}

	// Grounding clears at the next exchange: a later turn never reports a page
	// it did not use.
	ws.mu.Lock()
	ws.replies = []string{anthropicSearchedReply, anthropicChatOK}
	ws.mu.Unlock()
	if _, err := c.CompleteWithSystem(context.Background(), "sys", "thanks"); err != nil {
		t.Fatalf("CompleteWithSystem: %v", err)
	}
	if got := c.GetLastGroundingSources(); len(got) != 0 {
		t.Errorf("sources after an ungrounded turn = %v, want none", got)
	}
}

// A paused turn is resumed with the paused content sent back as the
// assistant's turn so far; the caller gets the whole turn and the usage of
// both requests. A turn that never stops pausing is refused after the bound.
func TestAnthropicWebTools_PauseTurnIsResumed(t *testing.T) {
	paused := `{"type":"message","role":"assistant","model":"claude-opus-5-5","stop_reason":"pause_turn",
"usage":{"input_tokens":10,"output_tokens":5,"server_tool_use":{"web_search_requests":1}},
"content":[{"type":"server_tool_use","id":"srvtoolu_1","name":"web_search","input":{"query":"q"}},
{"type":"web_search_tool_result","tool_use_id":"srvtoolu_1","content":[{"type":"web_search_result","url":"https://example.com/a","title":"A","encrypted_content":"e"}]}]}`
	done := `{"type":"message","role":"assistant","model":"claude-opus-5-5","stop_reason":"end_turn",
"usage":{"input_tokens":30,"output_tokens":7},"content":[{"type":"text","text":"answer"}]}`

	t.Run("resumed", func(t *testing.T) {
		ws := newWebToolServer(t, paused, done)
		c := newWebToolClient(t, ws.server.URL, "claude-opus-5-5", config.AnthropicProviderConfig{WebSearch: true})
		text, err := c.CompleteWithSystem(context.Background(), "sys", "q")
		if err != nil {
			t.Fatalf("CompleteWithSystem: %v", err)
		}
		if text != "answer" {
			t.Errorf("text = %q, want answer", text)
		}
		second := ws.request(t, 1)
		messages, _ := second["messages"].([]any)
		if len(messages) != 2 {
			t.Fatalf("resumption sent %d messages, want the user turn and the paused assistant turn", len(messages))
		}
		last, _ := messages[1].(map[string]any)
		if last["role"] != "assistant" {
			t.Fatalf("the resumption's last message is %v, want the paused assistant turn", last["role"])
		}
		content, _ := last["content"].([]any)
		if len(content) != 2 {
			t.Fatalf("the paused turn went back with %d blocks, want 2", len(content))
		}
		if got := c.GetLastGroundingSources(); len(got) != 1 || got[0] != "https://example.com/a" {
			t.Errorf("sources = %v, want the paused search's result", got)
		}
	})

	t.Run("bounded", func(t *testing.T) {
		ws := newWebToolServer(t, paused)
		c := newWebToolClient(t, ws.server.URL, "claude-opus-5-5", config.AnthropicProviderConfig{WebSearch: true})
		if _, err := c.CompleteWithSystem(context.Background(), "sys", "q"); err == nil || !strings.Contains(err.Error(), "paused the turn") {
			t.Fatalf("err = %v, want the pause bound", err)
		}
		ws.mu.Lock()
		sent := len(ws.requests)
		ws.mu.Unlock()
		if sent != anthropicMaxPauseResumes+1 {
			t.Errorf("sent %d requests, want %d", sent, anthropicMaxPauseResumes+1)
		}
	})
}

// The sources reach the readers production has -- chat articulation and
// delegation, campaign planning's grounding helper -- through the chain boot
// builds: the scheduler, the tracing client and a credential pool's broker,
// each forwarding the getter, read by the GroundingProvider assertion those
// readers make. The tools follow the config alone: a slot that did not opt in
// declares none and reports none.
func TestAnthropicWebTools_SourcesReachTheProductionChain(t *testing.T) {
	ws := newWebToolServer(t, anthropicSearchedReply)
	cfg := testPoolConfig(t, ws.server.URL)
	cfg.Anthropic = &config.AnthropicProviderConfig{WebSearch: true}
	pooled, err := NewClientFromConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	chain := core.NewScheduledLLMCall("planner", NewTracingLLMClient(pooled, nil))
	if _, ok := broker.Base(chain).(*AnthropicClient); !ok {
		t.Fatalf("the chain does not reach the Anthropic client: %T", broker.Base(chain))
	}
	ctx := context.Background()
	if _, err := chain.Client.CompleteWithSystem(ctx, "sys", "q"); err != nil {
		t.Fatalf("CompleteWithSystem: %v", err)
	}
	if got := serverToolsIn(ws.request(t, 0)); got["web_search_20260209"] != 5 {
		t.Errorf("server tools = %v, want web_search_20260209 with 5 uses", got)
	}
	provider, ok := any(chain).(types.GroundingProvider)
	if !ok {
		t.Fatal("the scheduled chain is not a GroundingProvider")
	}
	if got := provider.GetLastGroundingSources(); len(got) != 3 || !provider.IsWebSearchEnabled() {
		t.Errorf("through the chain: sources=%v web_search=%v, want 3 sources and on", got, provider.IsWebSearchEnabled())
	}

	off := NewAnthropicClientWithConfig(AnthropicConfig{APIKey: "k", Model: "claude-opus-5-5"})
	if off.IsWebSearchEnabled() || off.IsURLContextEnabled() || len(off.serverTools("claude-opus-5-5")) != 0 {
		t.Fatal("a slot that did not opt in declares or reports web tools")
	}
}
