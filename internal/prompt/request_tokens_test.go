package prompt

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"codenerd/internal/types"
)

// EstimateRequestTokens is the system prompt, the transcript and the catalog,
// each counted by EstimateTokens over its JSON form; an empty catalog costs
// nothing, and a request that cannot be encoded is an error, not a zero.
func TestEstimateRequestTokens(t *testing.T) {
	history := []types.Message{
		{Role: "user", Text: strings.Repeat("u", 400)},
		{Role: "assistant", ToolCalls: []types.ToolCall{{ID: "c1", Name: "read_file", Input: map[string]any{"path": "a.go"}}}},
		{Role: "user", ToolResults: []types.ToolResult{{ToolUseID: "c1", Content: strings.Repeat("r", 4000)}}},
	}
	catalog := []types.ToolDefinition{{Name: "read_file", Description: "read", InputSchema: map[string]any{"type": "object"}}}
	jsonTokens := func(t *testing.T, v any) int {
		t.Helper()
		encoded, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		return EstimateTokens(string(encoded))
	}

	tests := []struct {
		name     string
		system   string
		messages []types.Message
		tools    []types.ToolDefinition
		want     func(t *testing.T) int
		wantErr  bool
	}{
		{
			name:     "system, transcript and catalog",
			system:   strings.Repeat("s", 800),
			messages: history,
			tools:    catalog,
			want: func(t *testing.T) int {
				return EstimateTokens(strings.Repeat("s", 800)) + jsonTokens(t, history) + jsonTokens(t, catalog)
			},
		},
		{
			name:     "an empty catalog costs nothing",
			system:   "sys",
			messages: history,
			want: func(t *testing.T) int {
				return EstimateTokens("sys") + jsonTokens(t, history)
			},
		},
		{
			name:   "the transcript is always counted",
			system: "sys",
			want: func(t *testing.T) int {
				return EstimateTokens("sys") + EstimateTokens("null")
			},
		},
		{
			name:     "an unencodable catalog is an error",
			messages: history,
			tools:    []types.ToolDefinition{{Name: "bad", InputSchema: map[string]any{"x": math.Inf(1)}}},
			wantErr:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := EstimateRequestTokens(tt.system, tt.messages, tt.tools)
			if (err != nil) != tt.wantErr {
				t.Fatalf("EstimateRequestTokens() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if want := tt.want(t); got != want {
				t.Errorf("EstimateRequestTokens() = %d, want %d", got, want)
			}
		})
	}
}

func serverFetchBlock(t *testing.T, page string) types.ContentBlock {
	t.Helper()
	raw, err := json.Marshal(map[string]any{
		"type": "web_fetch_tool_result", "tool_use_id": "srvtoolu_1",
		"content": map[string]any{"type": "web_fetch_result", "url": "https://example.com",
			"source": map[string]any{"type": "text", "media_type": "text/plain", "data": page}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return types.ServerToolBlock(raw)
}

// Two native histories with the same roles, visible text and tools, differing
// only in the page inside one web_fetch_tool_result, replay different amounts
// to the provider. The estimate must differ with them: it used to marshal the
// public projection, which has no place for a server block.
func TestEstimateRequestTokens_CountsReplayedServerContent(t *testing.T) {
	history := func(page string) []types.Message {
		return []types.Message{
			{Role: "user", Text: "read it"},
			types.NewAssistantMessage(serverFetchBlock(t, page), types.TextBlock("done")),
		}
	}
	small, err := EstimateRequestTokens("sys", history("tiny"), nil)
	if err != nil {
		t.Fatal(err)
	}
	large, err := EstimateRequestTokens("sys", history(strings.Repeat("page ", 4000)), nil)
	if err != nil {
		t.Fatal(err)
	}
	if large-small < 4900 {
		t.Fatalf("a 20000-char fetched page added %d tokens, want about 5000; the server block is sent and was not counted", large-small)
	}
}

// Opaque encoded payloads are charged at what they decode to, not as the
// base64 they travel as.
func TestServerContentTokens_DoesNotChargeBase64AsProse(t *testing.T) {
	encoded := strings.Repeat("A", 8000)
	text, _ := json.Marshal(map[string]any{"type": "web_fetch_tool_result", "content": map[string]any{"source": map[string]any{"type": "text", "data": encoded}}})
	pdf, _ := json.Marshal(map[string]any{"type": "web_fetch_tool_result", "content": map[string]any{"source": map[string]any{"type": "base64", "data": encoded}}})
	if got, prose := ServerContentTokens(pdf), ServerContentTokens(text); got >= prose || prose-got < 450 {
		t.Fatalf("a base64 body must cost about three quarters of the same text: base64=%d text=%d", got, prose)
	}
}

// Signed reasoning is replayed and billed too; a redacted block is charged at
// the tokens the provider counted for it, not by its ciphertext length.
func TestEstimateRequestTokens_CountsReplayedReasoning(t *testing.T) {
	plain := []types.Message{types.NewAssistantMessage(types.TextBlock("done"))}
	think := []types.Message{types.NewAssistantMessage(types.ThinkingBlock(strings.Repeat("r", 4000), "sig"), types.TextBlock("done"))}
	redacted := types.RedactedThinkingBlock(strings.Repeat("c", 300000))
	redacted.Tokens = 22853
	hidden := []types.Message{types.NewAssistantMessage(redacted, types.TextBlock("done"))}
	unsigned := []types.Message{types.NewAssistantMessage(types.ThinkingBlock(strings.Repeat("r", 4000), ""), types.TextBlock("done"))}

	base, _ := EstimateRequestTokens("", plain, nil)
	if got, _ := EstimateRequestTokens("", think, nil); got-base < 1000 {
		t.Errorf("signed reasoning added %d tokens, want about 1000", got-base)
	}
	if got, _ := EstimateRequestTokens("", hidden, nil); got-base != 22853 {
		t.Errorf("redacted reasoning added %d tokens, want the provider's 22853", got-base)
	}
	if got, _ := EstimateRequestTokens("", unsigned, nil); got != base {
		t.Errorf("unsigned reasoning is not replayed and must cost nothing, added %d", got-base)
	}
}
