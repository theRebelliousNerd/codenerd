package perception

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"codenerd/internal/types"
	"codenerd/internal/usage"
)

// usageRecorder captures what a client reported to the usage plumbing.
type usageRecorder struct {
	mu            sync.Mutex
	input, output int
}

func (r *usageRecorder) Observed(_, _ string, input, output int, _ string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.input, r.output = input, output
}

// captureAnthropic serves one canned reply and hands back the decoded body of
// the request it received.
func captureAnthropic(t *testing.T, reply string) (*httptest.Server, func() map[string]any) {
	t.Helper()
	var mu sync.Mutex
	var body []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		body = b
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(reply))
	}))
	t.Cleanup(srv.Close)
	return srv, func() map[string]any {
		t.Helper()
		mu.Lock()
		defer mu.Unlock()
		var req map[string]any
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("request body is not JSON: %v\n%s", err, body)
		}
		return req
	}
}

const anthropicCachedReply = `{
  "id": "msg_1", "type": "message", "role": "assistant", "model": "claude-test",
  "content": [{"type": "text", "text": "done"}],
  "stop_reason": "end_turn",
  "usage": {"input_tokens": 40, "output_tokens": 7,
            "cache_creation_input_tokens": 300, "cache_read_input_tokens": 5000}
}`

func hasCacheControl(v any) bool {
	m, ok := v.(map[string]any)
	if !ok {
		return false
	}
	_, ok = m["cache_control"]
	return ok
}

// The tool loop resends tools, system and the growing transcript every round.
// Without breakpoints every round rebills all of it at full price; with one on
// the system block and one on the newest turn's last block, the next round
// reads everything before its own additions from the cache.
func TestAnthropicToolLoop_WhenContinuingAConversation_ShouldMarkSystemAndNewestTurnForCaching(t *testing.T) {
	srv, request := captureAnthropic(t, anthropicCachedReply)
	c := anthropicTestClient(srv.URL)

	history := []types.Message{
		types.NewUserMessage(types.TextBlock("fix the parser")),
		types.NewAssistantMessage(types.ToolUseBlock("call_1", "read_file", map[string]any{"path": "p.go"})),
		types.NewUserMessage(types.ToolResultBlock("call_1", "package p", false)),
	}
	tools := []types.ToolDefinition{{Name: "read_file", Description: "read", InputSchema: map[string]any{"type": "object"}}}
	if _, err := c.CompleteWithToolResults(context.Background(), "system prompt", history, tools); err != nil {
		t.Fatalf("CompleteWithToolResults: %v", err)
	}

	req := request()
	system, ok := req["system"].([]any)
	if !ok || len(system) != 1 || !hasCacheControl(system[0]) {
		t.Fatalf("system = %#v, want one block carrying cache_control", req["system"])
	}
	messages := req["messages"].([]any)
	marked := 0
	for i, m := range messages {
		content, ok := m.(map[string]any)["content"].([]any)
		if !ok {
			continue
		}
		for j, b := range content {
			if !hasCacheControl(b) {
				continue
			}
			marked++
			if i != len(messages)-1 || j != len(content)-1 {
				t.Errorf("breakpoint on message %d block %d, want only the last block of the last message", i, j)
			}
		}
	}
	if marked != 1 {
		t.Errorf("%d message breakpoints, want exactly 1 (on the newest turn)", marked)
	}
}

// Anthropic reports input_tokens as the uncached remainder only. The broker's
// contract is that input includes cache reads, so the client folds the read
// and the write back in, and records each as a sub-count.
func TestAnthropicToolLoop_WhenTheReplyReportsCacheUse_ShouldMeterTheWholePrompt(t *testing.T) {
	srv, _ := captureAnthropic(t, anthropicCachedReply)
	c := anthropicTestClient(srv.URL)
	rec := &usageRecorder{}
	ctx := usage.WithObserver(context.Background(), rec)

	resp, err := c.CompleteWithToolResults(ctx, "sys", []types.Message{types.NewUserMessage(types.TextBlock("hi"))}, nil)
	if err != nil {
		t.Fatalf("CompleteWithToolResults: %v", err)
	}
	const whole = 40 + 300 + 5000
	if rec.input != whole {
		t.Errorf("metered input = %d, want %d (remainder + write + read)", rec.input, whole)
	}
	if resp.Usage.InputTokens != whole || resp.Usage.CachedContentTokens != 5000 || resp.Usage.CacheWriteTokens != 300 {
		t.Errorf("usage = %+v, want input %d, cached 5000, cache write 300", resp.Usage, whole)
	}
}

// A client that was not asked to cache sends exactly what it always sent: a
// bare-string system prompt and no breakpoints.
func TestAnthropicChat_WhenCachingIsNotEnabled_ShouldSendThePlainRequest(t *testing.T) {
	srv, request := captureAnthropic(t, anthropicCachedReply)
	c := anthropicTestClient(srv.URL)
	if _, err := c.CompleteWithSystem(context.Background(), "sys", "hi"); err != nil {
		t.Fatalf("CompleteWithSystem: %v", err)
	}
	if s, ok := request()["system"].(string); !ok || s != "sys" {
		t.Errorf("system = %#v, want the bare string", request()["system"])
	}
}

func TestWithTailBreakpoint_WhenTheLastBlockIsThinking_ShouldMarkTheBlockBeforeItAndLeaveTheInputAlone(t *testing.T) {
	in := []AnthropicMessage{
		{Role: "user", Content: "first"},
		{Role: "assistant", Content: []AnthropicContentBlock{
			{Type: "text", Text: "plan"},
			{Type: "thinking", Thinking: "t", Signature: "sig"},
		}},
	}
	out := withTailBreakpoint(in)

	blocks := out[1].Content.([]AnthropicContentBlock)
	if blocks[0].CacheControl == nil || blocks[1].CacheControl != nil {
		t.Errorf("breakpoints = [%v %v], want only the text block marked", blocks[0].CacheControl, blocks[1].CacheControl)
	}
	if in[1].Content.([]AnthropicContentBlock)[0].CacheControl != nil {
		t.Error("the caller's blocks were modified; a retry would marshal a different request")
	}
	if _, ok := out[0].Content.(string); !ok {
		t.Error("an earlier turn changed shape; only the newest turn may carry the breakpoint")
	}
}

// The CLI reports Anthropic's usage block. Claude Code caches heavily, so the
// uncached remainder alone was a small fraction of the prompt.
func TestClaudeCodeCLIClient_WhenUsageReportsCacheUse_ShouldMeterTheWholePrompt(t *testing.T) {
	client := NewClaudeCodeCLIClient(nil)
	rec := &usageRecorder{}
	ctx := usage.WithObserver(context.Background(), rec)
	payload := []byte(`{"type":"result","subtype":"success","is_error":false,"result":"ok",
		"usage":{"input_tokens":2,"output_tokens":5,"cache_creation_input_tokens":67857,"cache_read_input_tokens":1000}}`)
	if _, err := client.parseResponse(ctx, payload); err != nil {
		t.Fatalf("parseResponse: %v", err)
	}
	if rec.input != 2+67857+1000 || rec.output != 5 {
		t.Errorf("metered = (%d in, %d out), want (%d, 5)", rec.input, rec.output, 2+67857+1000)
	}
}
