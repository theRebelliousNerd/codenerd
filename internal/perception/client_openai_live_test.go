package perception

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"codenerd/internal/broker"
	"codenerd/internal/config"
	"codenerd/internal/types"
)

// Live tests against api.openai.com. Skipped unless OPENAI_API_KEY is set:
//
//	OPENAI_API_KEY=... go test ./internal/perception/ -run TestLive_OpenAI -v
//
// OPENAI_MODEL picks the model (gpt-6.1-sol when unset). Effort stays low
// except where a test needs reasoning items to exist.
func liveOpenAI(t *testing.T, effort string) *OpenAIClient {
	t.Helper()
	key := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	if key == "" {
		t.Skip("set OPENAI_API_KEY to run live OpenAI tests")
	}
	model := strings.TrimSpace(os.Getenv("OPENAI_MODEL"))
	if model == "" {
		model = "gpt-6.1-sol"
	}
	cfg := DefaultOpenAIConfig(key)
	cfg.Model = model
	cfg.ReasoningEffort = effort
	cfg.MaxOutputTokens = 8000
	cfg.Timeout = 3 * time.Minute
	c, err := NewOpenAIClient(cfg)
	if err != nil {
		t.Fatalf("NewOpenAIClient: %v", err)
	}
	return c
}

func TestLive_OpenAI_CompleteAndStream(t *testing.T) {
	c := liveOpenAI(t, "low")
	ctx := context.Background()
	out, err := c.CompleteWithSystem(ctx, "Answer with one word.", "What color is a clear daytime sky?")
	if err != nil || !strings.Contains(strings.ToLower(out), "blue") {
		t.Fatalf("CompleteWithSystem = %q, %v", out, err)
	}
	content, done := c.CompleteWithStreaming(ctx, "", "List the numbers one to five as digits separated by commas.", false)
	text, res := types.CollectStream(ctx, content, done, nil)
	if res.Err != nil || !strings.Contains(text, "3") || !res.Usage.Reported() {
		t.Fatalf("stream = %q, %+v", text, res)
	}
}

func TestLive_OpenAI_ToolLoopReplaysReasoning(t *testing.T) {
	c := liveOpenAI(t, "medium")
	ctx := context.Background()
	tools := []ToolDefinition{{
		Name:        "read_file",
		Description: "Read a file from the repository",
		InputSchema: map[string]any{"type": "object", "properties": map[string]any{"path": map[string]any{"type": "string"}}, "required": []any{"path"}},
	}}
	sys := "You are a coding agent. Use tools to answer; never guess file contents."
	first, err := c.CompleteWithTools(ctx, sys, "What Go version does go.mod declare? Read it.", tools)
	if err != nil || len(first.ToolCalls) == 0 {
		t.Fatalf("first turn = %+v, %v", first, err)
	}
	call := first.ToolCalls[0]
	history := []types.Message{
		types.NewUserMessage(types.TextBlock("What Go version does go.mod declare? Read it.")),
		types.NewAssistantMessage(first.Blocks...),
		types.NewUserMessage(types.ToolResultBlock(call.ID, "module codenerd\n\ngo 1.26.3\n", false)),
	}
	second, err := c.CompleteWithToolResults(ctx, sys, history, tools)
	if err != nil || !strings.Contains(second.Text, "1.26") {
		t.Fatalf("second turn = %+v, %v", second, err)
	}
}

func TestLive_OpenAI_StructuredPiggyback(t *testing.T) {
	c := liveOpenAI(t, "low")
	out, err := c.CompleteWithSystem(context.Background(),
		"Reply as a PiggybackEnvelope with a surface_response and a control_packet.",
		"Say hello to the user.")
	if err != nil || !strings.Contains(out, "surface_response") {
		t.Fatalf("piggyback = %q, %v", out, err)
	}
}

func TestLive_OpenAI_GroundedWebSearch(t *testing.T) {
	c := liveOpenAI(t, "low")
	res, err := c.GroundedWebSearch(context.Background(), "What is the latest stable Go release? Cite the source.")
	if err != nil || res.Text == "" || len(c.GetLastGroundingSources()) == 0 {
		t.Fatalf("GroundedWebSearch = %+v, %v (sources %v)", res, err, c.GetLastGroundingSources())
	}
}

// TestLive_OpenAI_WorkspaceCoderProfile builds the coder profile's route from
// this repository's own .nerd/config.json, the path a routed shard call takes,
// and sends one request through it.
func TestLive_OpenAI_WorkspaceCoderProfile(t *testing.T) {
	if os.Getenv("OPENAI_API_KEY") == "" {
		t.Skip("set OPENAI_API_KEY to run live OpenAI tests")
	}
	_, file, _, _ := runtime.Caller(0)
	path := filepath.Join(filepath.Dir(file), "..", "..", ".nerd", "config.json")
	cfg, err := config.LoadUserConfig(path)
	if err != nil {
		t.Skipf("no workspace config: %v", err)
	}
	profile := cfg.GetShardProfile("coder")
	if profile.Provider != "openai" {
		t.Skipf("coder profile is on %q, not openai", profile.Provider)
	}
	client, err := NewRoutedClientFromUserConfig(cfg, cfg.GetWorkerLLMConfig(), profile.Provider, profile.Model)
	if err != nil {
		t.Fatalf("route coder profile: %v", err)
	}
	base, ok := broker.Base(client).(*OpenAIClient)
	if !ok {
		t.Fatalf("routed client is %T, want *OpenAIClient", broker.Base(client))
	}
	if _, model := base.ModelIdentity(); model != profile.Model {
		t.Errorf("routed model = %s, want %s", model, profile.Model)
	}
	ctx := config.ShardProfileContext(context.Background(), profile)
	out, err := client.CompleteWithSystem(ctx, "Answer with one word.", "Opposite of hot?")
	if err != nil || out == "" {
		t.Fatalf("coder profile call = %q, %v", out, err)
	}
}
