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
