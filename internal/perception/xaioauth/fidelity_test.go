package xaioauth_test

import (
	"strings"
	"testing"

	"codenerd/internal/perception"
	"codenerd/internal/perception/xaioauth"
	"codenerd/internal/types"
)

// This mapper is a hand-copied Chat Completions clone, and hand-copied clones
// drift. internal/perception's own fidelity test cannot reach it — the mapper
// is unexported and lives in a package internal/perception imports — so
// SuperGrok's OAuth surface was named in the BlockFidelity table as a
// SurfaceOpenAIChatCompletions speaker with nothing checking that it still is.
//
// That is the hole the table exists to close, one package over. The external
// test package is the seam: xaioauth_test may import internal/perception even
// though internal/perception imports xaioauth, because the cycle runs through
// the test package only.
func TestSuperGrokMatchesTheChatCompletionsFidelityItIsDeclaredUnder(t *testing.T) {
	declared, ok := perception.BlockFidelity[perception.SurfaceOpenAIChatCompletions]
	if !ok {
		t.Fatal("SurfaceOpenAIChatCompletions is not in BlockFidelity; this surface is undeclared")
	}

	// The same interleaved turn internal/perception drives: prose, a call,
	// more prose, a second call, behind a signed thinking block. A single text
	// block would prove nothing about order.
	history := []types.Message{
		types.NewUserMessage(types.TextBlock("why does LoadConfig ignore the flag?")),
		types.NewAssistantMessage(
			types.ThinkingBlock("Check the config first, then the caller.", "sig-abc-123"),
			types.TextBlock("I'll check two things."),
			types.ToolUseBlock("toolu_A", "read_file", map[string]any{"path": "config.go"}),
			types.TextBlock("Now the caller."),
			types.ToolUseBlock("toolu_B", "search_code", map[string]any{"query": "LoadConfig"}),
		),
		types.NewUserMessage(
			types.ToolResultBlock("toolu_A", "package config\n...", false),
			types.ToolResultBlock("toolu_B", "no matches", true),
		),
	}

	request, err := xaioauth.MapHistoryForFidelity(history)
	if err != nil {
		t.Fatalf("MapHistoryForFidelity: %v", err)
	}

	if replayed := strings.Contains(request, "sig-abc-123"); replayed != declared.ReplaysReasoning {
		t.Errorf("ReplaysReasoning declared %v, mapper does %v\nrequest: %s",
			declared.ReplaysReasoning, replayed, request)
	}
	if kept := !strings.Contains(request, "I'll check two things.Now the caller."); kept != declared.KeepsOrder {
		t.Errorf("KeepsOrder declared %v, mapper does %v\nrequest: %s",
			declared.KeepsOrder, kept, request)
	}
	if carriesIDs := strings.Contains(request, "toolu_A"); carriesIDs != declared.CarriesToolIDs {
		t.Errorf("CarriesToolIDs declared %v, mapper does %v\nrequest: %s",
			declared.CarriesToolIDs, carriesIDs, request)
	}

	// Chat Completions cannot replay reasoning, so the mapper must DROP the
	// thinking block rather than fold it into the prose. Smuggling reads fine
	// and corrupts every structured-output parse downstream.
	if !declared.ReplaysReasoning && strings.Contains(request, "Check the config first, then the caller.") {
		t.Errorf("reasoning reached the wire as prose on a surface that cannot replay it"+
			"\nrequest: %s", request)
	}

	// And the one thing no surface may lose, whatever else it drops.
	for _, want := range []string{"package config", "no matches"} {
		if !strings.Contains(request, want) {
			t.Errorf("dropped the tool result %q\nrequest: %s", want, request)
		}
	}
}
