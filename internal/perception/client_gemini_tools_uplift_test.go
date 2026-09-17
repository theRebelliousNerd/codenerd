package perception

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"codenerd/internal/types"
)

const geminiToolCallPayload = `{
  "candidates": [{
    "content": {"parts": [{"functionCall": {"name": "get_time", "args": {"zone": "UTC"}}}]},
    "finishReason": "STOP"
  }],
  "usageMetadata": {"promptTokenCount": 10, "candidatesTokenCount": 5, "totalTokens": 15, "totalTokenCount": 15}
}`

const geminiTextPayload = `{
  "candidates": [{
    "content": {"parts": [{"text": "done here"}]},
    "finishReason": "STOP"
  }],
  "usageMetadata": {"promptTokenCount": 4, "candidatesTokenCount": 2, "totalTokens": 6, "totalTokenCount": 6}
}`

func geminiToolsForTest() []ToolDefinition {
	return []ToolDefinition{{
		Name:        "get_time",
		Description: "Get the time",
		InputSchema: map[string]any{"type": "object"},
	}}
}

// The single-shot tools path shares the retry policy with the chat paths:
// a transient 503 is retried rather than failing the turn. The old tools
// loop failed immediately on any 5xx.
func TestGeminiTools_RetriesTransientThenSucceeds(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":{"message":"high demand"}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(geminiToolCallPayload))
	}))
	defer srv.Close()

	c := geminiTestClient(srv.URL)
	resp, err := c.CompleteWithTools(context.Background(), "sys", "time?", geminiToolsForTest())
	if err != nil {
		t.Fatalf("CompleteWithTools: %v", err)
	}
	if got := hits.Load(); got != 2 {
		t.Errorf("server hits = %d, want 2 (one transient retried)", got)
	}
	if len(resp.ToolCalls) != 1 || resp.ToolCalls[0].Name != "get_time" {
		t.Fatalf("ToolCalls = %+v, want one get_time call", resp.ToolCalls)
	}
	if zone, _ := resp.ToolCalls[0].Input["zone"].(string); zone != "UTC" {
		t.Errorf("tool input zone = %q, want UTC", zone)
	}
	if resp.Usage.TotalTokens != 15 {
		t.Errorf("Usage.TotalTokens = %d, want 15", resp.Usage.TotalTokens)
	}
}

// Full native sequence on one client: Tools mints the call, the executor
// runs it, ToolResults pairs the result by ID so the wire carries the real
// function name (not the synthetic call ID) — and the transient on the
// second leg is retried. Also pins that mapping the history cannot write
// into the caller's backing array.
func TestGeminiToolResults_FullSequenceWithPairing(t *testing.T) {
	var hits atomic.Int32
	captured := make(chan string, 2)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		n := hits.Add(1)
		if strings.Contains(string(body), "functionResponse") {
			captured <- string(body)
			if n == 2 {
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write([]byte(`{"error":{"message":"high demand"}}`))
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(string(body), "functionResponse") {
			_, _ = w.Write([]byte(geminiTextPayload))
		} else {
			_, _ = w.Write([]byte(geminiToolCallPayload))
		}
	}))
	defer srv.Close()

	c := geminiTestClient(srv.URL)
	ctx := context.Background()
	resp1, err := c.CompleteWithTools(ctx, "sys", "time?", geminiToolsForTest())
	if err != nil {
		t.Fatalf("CompleteWithTools: %v", err)
	}
	if len(resp1.ToolCalls) != 1 {
		t.Fatalf("ToolCalls = %+v, want one call", resp1.ToolCalls)
	}

	backing := make([]types.Message, 3, 6)
	backing[0] = types.Message{Role: "user", Text: "time?"}
	backing[1] = types.AssistantMessageFrom(resp1)
	backing[2] = types.Message{Role: "user", ToolResults: []types.ToolResult{
		{ToolUseID: resp1.ToolCalls[0].ID, Content: "noon"},
	}}
	history := backing[:3]
	resp2, err := c.CompleteWithToolResults(ctx, "sys", history, geminiToolsForTest())
	if err != nil {
		t.Fatalf("CompleteWithToolResults: %v", err)
	}
	if resp2.Text != "done here" {
		t.Errorf("response = %q, want done here", resp2.Text)
	}
	if got := hits.Load(); got != 3 {
		t.Errorf("server hits = %d, want 3 (tools + failed + retried results)", got)
	}
	if full := backing[:cap(backing)]; full[3].Role != "" {
		t.Errorf("caller backing array mutated: backing[3] = %+v, want zero value", full[3])
	}

	var wire struct {
		Contents []struct {
			Role  string `json:"role"`
			Parts []struct {
				FunctionResponse *struct {
					Name     string         `json:"name"`
					Response map[string]any `json:"response"`
				} `json:"functionResponse"`
			} `json:"parts"`
		} `json:"contents"`
	}
	// The retry re-sends; check the last captured body.
	var lastBody string
	for len(captured) > 0 {
		lastBody = <-captured
	}
	if err := json.Unmarshal([]byte(lastBody), &wire); err != nil {
		t.Fatalf("wire request is not JSON: %v", err)
	}
	if len(wire.Contents) != 3 || wire.Contents[2].Role != "function" {
		t.Fatalf("wire contents roles = %v, want [user model function]", wire.Contents)
	}
	fr := wire.Contents[2].Parts[0].FunctionResponse
	if fr == nil || fr.Name != "get_time" {
		t.Fatalf("functionResponse = %+v, want real function name get_time", fr)
	}
	if fr.Response["content"] != "noon" {
		t.Errorf("functionResponse content = %v, want noon", fr.Response["content"])
	}
}

// The ToolResultsProvider adapter translates neutral history to Gemini
// contents: roles map user->user, assistant->model, results->function, and
// the function response carries the real function name resolved through
// the assistant turn's tool calls.
func TestGeminiToolResultsTRP_TranslatesHistory(t *testing.T) {
	captured := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		captured <- string(body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(geminiTextPayload))
	}))
	defer srv.Close()

	c := geminiTestClient(srv.URL)
	history := []types.Message{
		{Role: "user", Text: "what time is it?"},
		{Role: "assistant", Text: "checking", ToolCalls: []types.ToolCall{
			{ID: "call_0", Name: "get_time", Input: map[string]any{}},
		}},
		{Role: "user", ToolResults: []types.ToolResult{
			{ToolUseID: "call_0", Content: "noon"},
		}},
	}
	resp, err := c.CompleteWithToolResults(context.Background(), "sys", history, nil)
	if err != nil {
		t.Fatalf("CompleteWithToolResults: %v", err)
	}
	if resp.Text != "done here" {
		t.Errorf("response = %q, want done here", resp.Text)
	}

	var wire struct {
		Contents []struct {
			Role  string `json:"role"`
			Parts []struct {
				Text         string `json:"text"`
				FunctionCall *struct {
					Name string `json:"name"`
				} `json:"functionCall"`
				FunctionResponse *struct {
					Name     string         `json:"name"`
					Response map[string]any `json:"response"`
				} `json:"functionResponse"`
			} `json:"parts"`
		} `json:"contents"`
	}
	if err := json.Unmarshal([]byte(<-captured), &wire); err != nil {
		t.Fatalf("wire request is not JSON: %v", err)
	}
	if len(wire.Contents) != 3 {
		t.Fatalf("wire contents = %d, want 3 (user, model, function)", len(wire.Contents))
	}
	roles := []string{wire.Contents[0].Role, wire.Contents[1].Role, wire.Contents[2].Role}
	if roles[0] != "user" || roles[1] != "model" || roles[2] != "function" {
		t.Errorf("wire roles = %v, want [user model function]", roles)
	}
	if fc := wire.Contents[1].Parts[len(wire.Contents[1].Parts)-1].FunctionCall; fc == nil || fc.Name != "get_time" {
		t.Errorf("model functionCall = %+v, want get_time", fc)
	}
	fr := wire.Contents[2].Parts[0].FunctionResponse
	if fr == nil || fr.Name != "get_time" {
		t.Fatalf("functionResponse = %+v, want real function name get_time", fr)
	}
	if fr.Response["content"] != "noon" {
		t.Errorf("functionResponse content = %v, want noon", fr.Response["content"])
	}
	if fr.Response["is_error"] != false {
		t.Errorf("functionResponse is_error = %v, want false", fr.Response["is_error"])
	}
}

// The adapted path shares the retry policy: a transient 503 is retried.
func TestGeminiToolResultsTRP_RetriesTransient(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":{"message":"high demand"}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(geminiTextPayload))
	}))
	defer srv.Close()

	c := geminiTestClient(srv.URL)
	history := []types.Message{
		{Role: "user", Text: "hi"},
		{Role: "assistant", ToolCalls: []types.ToolCall{{ID: "call_0", Name: "ping"}}},
		{Role: "user", ToolResults: []types.ToolResult{{ToolUseID: "call_0", Content: "pong"}}},
	}
	resp, err := c.CompleteWithToolResults(context.Background(), "sys", history, nil)
	if err != nil {
		t.Fatalf("CompleteWithToolResults: %v", err)
	}
	if resp.Text != "done here" {
		t.Errorf("response = %q, want done here", resp.Text)
	}
	if got := hits.Load(); got != 2 {
		t.Errorf("server hits = %d, want 2 (one transient retried)", got)
	}
}

// A tool-results call with no results in history fails before any HTTP:
// there is nothing to append and no pairing to perform.
func TestGeminiToolResultsTRP_NoResultsFailsFast(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := geminiTestClient(srv.URL)
	_, err := c.CompleteWithToolResults(context.Background(), "sys",
		[]types.Message{{Role: "user", Text: "hi"}}, nil)
	if err == nil || !strings.Contains(err.Error(), "no tool results") {
		t.Fatalf("err = %v, want the no-results guard", err)
	}
	if got := hits.Load(); got != 0 {
		t.Errorf("server hits = %d, want 0 (no request without results)", got)
	}
}

// Every round keeps its results inline as function contents with real names
// resolved from the tool_use that minted each id, in history order.
func TestGeminiToolResultsTRP_MultiRound(t *testing.T) {
	captured := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		captured <- string(body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(geminiTextPayload))
	}))
	defer srv.Close()

	c := geminiTestClient(srv.URL)
	history := []types.Message{
		{Role: "user", Text: "time and date?"},
		{Role: "assistant", ToolCalls: []types.ToolCall{{ID: "call_0", Name: "get_time"}}},
		{Role: "user", ToolResults: []types.ToolResult{{ToolUseID: "call_0", Content: "noon"}}},
		{Role: "assistant", ToolCalls: []types.ToolCall{{ID: "call_1", Name: "get_date"}}},
		{Role: "user", ToolResults: []types.ToolResult{{ToolUseID: "call_1", Content: "tuesday"}}},
	}
	if _, err := c.CompleteWithToolResults(context.Background(), "sys", history, nil); err != nil {
		t.Fatalf("CompleteWithToolResults: %v", err)
	}

	var wire struct {
		Contents []struct {
			Role  string `json:"role"`
			Parts []struct {
				FunctionResponse *struct {
					Name     string         `json:"name"`
					Response map[string]any `json:"response"`
				} `json:"functionResponse"`
			} `json:"parts"`
		} `json:"contents"`
	}
	if err := json.Unmarshal([]byte(<-captured), &wire); err != nil {
		t.Fatalf("wire request is not JSON: %v", err)
	}
	var roles []string
	for _, c := range wire.Contents {
		roles = append(roles, c.Role)
	}
	want := []string{"user", "model", "function", "model", "function"}
	if strings.Join(roles, ",") != strings.Join(want, ",") {
		t.Fatalf("wire roles = %v, want %v", roles, want)
	}
	if n := wire.Contents[2].Parts[0].FunctionResponse.Name; n != "get_time" {
		t.Errorf("round-1 function name = %q, want get_time", n)
	}
	last := wire.Contents[4].Parts[0].FunctionResponse
	if last.Name != "get_date" || last.Response["content"] != "tuesday" {
		t.Errorf("round-2 function response = %+v, want get_date/tuesday", last)
	}
}

// A length stop names the public entry point it came through, so the
// broker's compression message attributes the truncation correctly.
func TestGeminiTools_TruncationLabels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
		  "candidates": [{"content": {"parts": [{"text": "cut"}]}, "finishReason": "MAX_TOKENS"}],
		  "usageMetadata": {"promptTokenCount": 1, "candidatesTokenCount": 2048, "totalTokenCount": 2049}
		}`))
	}))
	defer srv.Close()

	c := geminiTestClient(srv.URL)
	ctx := context.Background()

	_, err := c.CompleteWithTools(ctx, "sys", "hi", geminiToolsForTest())
	var trunc *types.OutputTruncated
	if !errors.As(err, &trunc) {
		t.Fatalf("tools err = %v (%T), want *types.OutputTruncated", err, err)
	}
	if trunc.Method != "CompleteWithTools" || trunc.LimitTokens != c.maxOutputTokens {
		t.Errorf("tools truncation = method %q limit %d, want CompleteWithTools/%d",
			trunc.Method, trunc.LimitTokens, c.maxOutputTokens)
	}

	history := []types.Message{
		{Role: "user", Text: "hi"},
		{Role: "assistant", ToolCalls: []types.ToolCall{{ID: "call_0", Name: "get_time"}}},
		{Role: "user", ToolResults: []types.ToolResult{{ToolUseID: "call_0", Content: "noon"}}},
	}
	_, err = c.CompleteWithToolResults(ctx, "sys", history, geminiToolsForTest())
	if !errors.As(err, &trunc) {
		t.Fatalf("tool-results err = %v (%T), want *types.OutputTruncated", err, err)
	}
	if trunc.Method != "CompleteWithToolResults" {
		t.Errorf("tool-results truncation method = %q, want CompleteWithToolResults", trunc.Method)
	}
}

// The client must satisfy the executor's multi-turn interface now that the
// adapter exists; without it Gemini turns degrade to one tool batch.
func TestGeminiClient_ImplementsToolResultsProvider(t *testing.T) {
	var _ types.ToolResultsProvider = (*GeminiClient)(nil)
}
