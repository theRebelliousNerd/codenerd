package perception

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"codenerd/internal/types"
)

// Live integration tests against the real vendor endpoints.
//
// These are skipped unless the corresponding API key is exported, so `go test
// ./...` stays hermetic on a machine without credentials:
//
//	$env:DASHSCOPE_API_KEY = "..."; go test ./internal/perception/ -run Live -v
//	$env:META_API_KEY      = "..."; go test ./internal/perception/ -run Live -v
//
// They exist because the vendor quirks this client encodes (DashScope's
// enable_thinking/reasoning_content split, Meta's max_completion_tokens and
// reasoning_effort) are only verifiable against the real API — a mock would just
// re-assert our own assumptions.
func liveClient(t *testing.T, vendor Provider, envVars ...string) *OpenAICompatClient {
	t.Helper()

	var key string
	for _, v := range envVars {
		if key = strings.TrimSpace(os.Getenv(v)); key != "" {
			break
		}
	}
	if key == "" {
		t.Skipf("set %s to run live %s tests", strings.Join(envVars, " or "), vendor)
	}

	cfg := DefaultOpenAICompatConfig(vendor, key)
	cfg.Timeout = 3 * time.Minute
	cfg.MaxOutputTokens = 512
	if m := strings.TrimSpace(os.Getenv(strings.ToUpper(string(vendor)) + "_MODEL")); m != "" {
		cfg.Model = m
	}

	c, err := NewOpenAICompatClient(cfg)
	if err != nil {
		t.Fatalf("NewOpenAICompatClient(%s): %v", vendor, err)
	}
	return c
}

func TestLive_DashScope_Complete(t *testing.T) {
	c := liveClient(t, ProviderDashScope, "DASHSCOPE_API_KEY")

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	got, err := c.CompleteWithSystem(ctx,
		"You are a terse assistant. Answer with a single word.",
		"What is the capital of France?")
	if err != nil {
		t.Fatalf("CompleteWithSystem: %v", err)
	}
	t.Logf("model=%s response=%q", c.GetModel(), got)

	if strings.TrimSpace(got) == "" {
		t.Fatal("empty response")
	}
	if !strings.Contains(strings.ToLower(got), "paris") {
		t.Errorf("unexpected answer %q — check whether reasoning_content leaked into content", got)
	}
}

func TestLive_DashScope_ToolCalling(t *testing.T) {
	c := liveClient(t, ProviderDashScope, "DASHSCOPE_API_KEY")

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	resp, err := c.CompleteWithTools(ctx,
		"You are a coding assistant. Use the provided tool when asked to read a file.",
		"Read the file internal/core/kernel.go",
		[]ToolDefinition{{
			Name:        "read_file",
			Description: "Read the contents of a file from the repository",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{"type": "string", "description": "Repo-relative path"},
				},
				"required": []string{"path"},
			},
		}})
	if err != nil {
		t.Fatalf("CompleteWithTools: %v", err)
	}
	t.Logf("stop_reason=%s tool_calls=%d text=%q", resp.StopReason, len(resp.ToolCalls), resp.Text)

	// Tool use is the agentic path the session executor depends on. A model that
	// answers in prose here would silently degrade every shard to chat-only.
	if len(resp.ToolCalls) == 0 {
		t.Error("expected a tool call; shards depend on this path working")
	}
	if resp.Usage.TotalTokens == 0 {
		t.Error("usage not reported — token accounting will be wrong")
	}
}

func TestLive_Meta_Complete(t *testing.T) {
	c := liveClient(t, ProviderMeta, "META_API_KEY", "MODEL_API_KEY")

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	got, err := c.CompleteWithSystem(ctx,
		"You are a terse assistant. Answer with a single word.",
		"What is the capital of France?")
	if err != nil {
		t.Fatalf("CompleteWithSystem: %v", err)
	}
	t.Logf("model=%s response=%q", c.GetModel(), got)

	if !strings.Contains(strings.ToLower(got), "paris") {
		t.Errorf("unexpected answer %q", got)
	}
}

// Muse Spark rejects reasoning_effort:"none" with HTTP 400, and every capability
// tier must map to a value it accepts.
func TestLive_Meta_ReasoningEffortTiers(t *testing.T) {
	c := liveClient(t, ProviderMeta, "META_API_KEY", "MODEL_API_KEY")

	for _, capability := range []types.ModelCapability{
		types.CapabilityHighReasoning,
		types.CapabilityBalanced,
		types.CapabilityHighSpeed,
	} {
		t.Run(string(capability), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
			defer cancel()

			ctx = context.WithValue(ctx, types.CtxKeyModelCapability, capability)
			got, err := c.CompleteWithSystem(ctx, "Answer with one word.", "What is 2+2?")
			if err != nil {
				t.Fatalf("capability %s rejected by vendor: %v", capability, err)
			}
			t.Logf("capability=%s response=%q", capability, strings.TrimSpace(got))
		})
	}
}

func TestLive_Meta_ToolCalling(t *testing.T) {
	c := liveClient(t, ProviderMeta, "META_API_KEY", "MODEL_API_KEY")

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	resp, err := c.CompleteWithTools(ctx,
		"You are a coding assistant. Use the provided tool when asked to read a file.",
		"Read the file internal/core/kernel.go",
		[]ToolDefinition{{
			Name:        "read_file",
			Description: "Read the contents of a file from the repository",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{"type": "string", "description": "Repo-relative path"},
				},
				"required": []string{"path"},
			},
		}})
	if err != nil {
		t.Fatalf("CompleteWithTools: %v", err)
	}
	t.Logf("stop_reason=%s tool_calls=%d text=%q", resp.StopReason, len(resp.ToolCalls), resp.Text)

	if len(resp.ToolCalls) == 0 {
		t.Error("expected a tool call; shards depend on this path working")
	}
}

// Streaming feeds the chat surface. A vendor that streams reasoning into the
// content channel would render the thinking trace to the user.
func TestLive_Streaming(t *testing.T) {
	for _, tc := range []struct {
		vendor Provider
		envs   []string
	}{
		{ProviderDashScope, []string{"DASHSCOPE_API_KEY"}},
		{ProviderMeta, []string{"META_API_KEY", "MODEL_API_KEY"}},
	} {
		t.Run(string(tc.vendor), func(t *testing.T) {
			c := liveClient(t, tc.vendor, tc.envs...)

			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
			defer cancel()

			contentCh, errCh := c.CompleteWithStreaming(ctx,
				"You are terse.", "Count from 1 to 5, separated by spaces.", true)

			var sb strings.Builder
			chunks := 0
			for contentCh != nil || errCh != nil {
				select {
				case chunk, ok := <-contentCh:
					if !ok {
						contentCh = nil
						continue
					}
					chunks++
					sb.WriteString(chunk)
				case err, ok := <-errCh:
					if !ok {
						errCh = nil
						continue
					}
					if err != nil {
						t.Fatalf("stream error: %v", err)
					}
				case <-ctx.Done():
					t.Fatal("stream timed out")
				}
			}

			t.Logf("chunks=%d text=%q", chunks, sb.String())
			if sb.Len() == 0 {
				t.Fatal("stream produced no content")
			}
		})
	}
}
