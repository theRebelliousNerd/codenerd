package perception

import (
	"testing"

	"codenerd/internal/broker"
	"codenerd/internal/config"
)

// geminiCap is the output ceiling the Gemini client under c will send.
func geminiCap(t *testing.T, c LLMClient) int {
	t.Helper()
	g, ok := broker.Base(c).(*GeminiClient)
	if !ok {
		t.Fatalf("client %T is not a Gemini client", broker.Base(c))
	}
	return g.maxOutputTokens
}

// A Gemini worker or planner sends its own slot's max_output_tokens, not the
// workspace-wide gemini.max_output_tokens: a cheap worker below it and a
// planner above it both keep their budgets. An unset slot cap keeps the
// Gemini ceiling, and the main client's documented contract (Gemini ignores
// the generic main max_output_tokens) is unchanged (audit
// gemini-secondary-output-cap).
func TestGeminiSecondarySlots_SendTheirOwnOutputCap(t *testing.T) {
	cfg := &config.UserConfig{
		Provider:        "gemini",
		Model:           "gemini-main",
		GeminiAPIKey:    "test-key",
		MaxOutputTokens: 1234,
		Gemini:          &config.GeminiProviderConfig{MaxOutputTokens: 8192},
		Worker:          &config.SecondaryLLMConfig{Provider: "gemini", Model: "gemini-worker", MaxOutputTokens: 2048},
		Planner:         &config.SecondaryLLMConfig{Provider: "gemini", Model: "gemini-planner", MaxOutputTokens: 32768},
	}
	worker, err := NewWorkerClientFromUserConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if got := geminiCap(t, worker); got != 2048 {
		t.Fatalf("worker sends %d, want its slot's 2048", got)
	}
	planner, err := NewPlannerClientFromUserConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if got := geminiCap(t, planner); got != 32768 {
		t.Fatalf("planner sends %d, want its slot's 32768", got)
	}

	cfg.Worker.MaxOutputTokens = 0
	unset, err := NewWorkerClientFromUserConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if got := geminiCap(t, unset); got != 8192 {
		t.Fatalf("an unset worker cap sends %d, want the Gemini ceiling 8192", got)
	}

	pc, err := ProviderConfigFromUserConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	main, err := NewClientFromConfig(pc)
	if err != nil {
		t.Fatal(err)
	}
	if got := geminiCap(t, main); got != 8192 {
		t.Fatalf("the main client sends %d, want gemini.max_output_tokens 8192 (the generic main field does not apply to Gemini)", got)
	}
}
