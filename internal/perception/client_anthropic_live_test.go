package perception

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/types"
)

// Explicit opt-in keeps credentials alone from triggering billed inference.
//
//	CODENERD_LIVE_ANTHROPIC=1 ANTHROPIC_MODEL=<configured model ID> \
//	  go test ./internal/perception -run '^TestLiveAnthropicDirectAPI$' -v
//
// Also requires ANTHROPIC_API_KEY; ANTHROPIC_WORKSPACE_ID is optional.
func TestLiveAnthropicDirectAPI(t *testing.T) {
	if os.Getenv("CODENERD_LIVE_ANTHROPIC") != "1" {
		t.Skip("set CODENERD_LIVE_ANTHROPIC=1 to run billed direct API checks")
	}
	key, model := os.Getenv("ANTHROPIC_API_KEY"), os.Getenv("ANTHROPIC_MODEL")
	if key == "" || strings.TrimSpace(model) == "" {
		t.Fatal("live check requires ANTHROPIC_API_KEY and an explicit ANTHROPIC_MODEL")
	}
	client, err := NewClientFromConfig(&ProviderConfig{Provider: ProviderAnthropic, APIKey: key, Model: model, MaxOutputTokens: 4096, ReasoningEffort: "low", Anthropic: &config.AnthropicProviderConfig{WorkspaceID: os.Getenv("ANTHROPIC_WORKSPACE_ID"), StrictTools: true}})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if text, err := client.CompleteWithSystem(ctx, "Follow the user's request concisely.", "Reply with just the word hello."); err != nil || !strings.Contains(strings.ToLower(text), "hello") {
		t.Fatalf("text completion: response=%q err=%v", text, err)
	}
	ch, errs := client.CompleteWithStreaming(ctx, "Follow the user's request concisely.", "Reply with just the word hello.", false)
	if text, err := drainAnthropic(t, ch, errs); err != nil || !strings.Contains(strings.ToLower(text), "hello") {
		t.Fatalf("stream: response=%q err=%v", text, err)
	}
	sc, ok := core.AsSchemaCapable(client)
	if !ok {
		t.Fatal("native schema support unavailable")
	}
	if _, err := sc.CompleteWithSchema(ctx, "Return the requested JSON object.", "Set answer to yes.", anthropicClosedSchema); err != nil {
		t.Fatalf("schema: %v", err)
	}
	defs := []types.ToolDefinition{{Name: "echo_probe", Description: "Echo a short input string for a transport test.", InputSchema: map[string]any{"type": "object", "properties": map[string]any{"text": map[string]any{"type": "string"}}, "required": []string{"text"}, "additionalProperties": false}}}
	system := "Use echo_probe exactly once before answering. After receiving its result, reply with that result."
	user := "Call echo_probe with text set to transport-ok."
	out, err := client.CompleteWithTools(ctx, system, user, defs)
	if err != nil {
		t.Fatalf("tool call: %v", err)
	}
	if len(out.ToolCalls) != 1 || out.ToolCalls[0].Name != "echo_probe" {
		t.Fatalf("expected exactly one native echo_probe call, got %+v", out.ToolCalls)
	}
	trp, ok := client.(types.ToolResultsProvider)
	if !ok {
		t.Fatal("native tool continuation interface lost")
	}
	history := []types.Message{types.NewUserMessage(types.TextBlock(user)), types.AssistantMessageFrom(out), types.NewUserMessage(types.ToolResultBlock(out.ToolCalls[0].ID, "transport-ok", false))}
	final, err := trp.CompleteWithToolResults(ctx, system, history, defs)
	if err != nil || len(final.ToolCalls) != 0 || !strings.Contains(final.Text, "transport-ok") {
		t.Fatalf("tool continuation: response=%+v err=%v", final, err)
	}
}
