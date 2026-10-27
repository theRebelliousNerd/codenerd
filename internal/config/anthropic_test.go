package config

import (
	"encoding/json"
	"testing"
)

func TestAnthropicConfigValidation(t *testing.T) {
	for _, tc := range []struct {
		name  string
		cfg   AnthropicProviderConfig
		valid bool
	}{
		{"default", AnthropicProviderConfig{}, true},
		{"adaptive", AnthropicProviderConfig{Thinking: "adaptive", ThinkingDisplay: "omitted"}, true},
		{"legacy", AnthropicProviderConfig{Thinking: "enabled", ThinkingBudgetTokens: 1024}, true},
		{"between tools", AnthropicProviderConfig{Thinking: "between_tools"}, true},
		{"bad mode", AnthropicProviderConfig{Thinking: "turbo"}, false},
		{"bad budget", AnthropicProviderConfig{Thinking: "enabled", ThinkingBudgetTokens: 100}, false},
		{"unused budget", AnthropicProviderConfig{ThinkingBudgetTokens: 1024}, false},
		{"display without mode", AnthropicProviderConfig{ThinkingDisplay: "summarized"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.cfg.Validate(); (err == nil) != tc.valid {
				t.Fatalf("Validate=%v", err)
			}
		})
	}
	for _, level := range []string{"", "low", "medium", "high", "xhigh", "max"} {
		if err := ValidateAnthropicEffort(level); err != nil {
			t.Fatal(err)
		}
	}
	if err := ValidateAnthropicEffort("minimal"); err == nil {
		t.Fatal("Meta-only effort accepted for Anthropic")
	}
}

func TestAnthropicConfigCheckAndRoundTrip(t *testing.T) {
	raw := []byte(`{"provider":"anthropic","model":"claude-opus-5-5","anthropic_api_key":"k","reasoning_effort":"max","anthropic":{"workspace_id":"wrkspc_main","thinking":"adaptive"},"worker":{"provider":"anthropic","model":"claude-haiku-5-5","reasoning_effort":"low","anthropic":{"workspace_id":"wrkspc_worker"}}}`)
	var cfg UserConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Anthropic == nil || cfg.Worker.Anthropic == nil || cfg.Worker.Anthropic.WorkspaceID != "wrkspc_worker" {
		t.Fatal("Anthropic settings dropped")
	}
	for _, problem := range cfg.Check(raw) {
		if problem.Severity == SeverityError {
			t.Fatal(problem)
		}
	}
	cfg.ReasoningEffort = "minimal"
	cfg.Worker.ReasoningEffort = "turbo"
	var main, worker bool
	for _, problem := range cfg.Check(raw) {
		if problem.Severity == SeverityError {
			main = main || problem.Path == "reasoning_effort"
			worker = worker || problem.Path == "worker.reasoning_effort"
		}
	}
	if !main || !worker {
		t.Fatalf("invalid effort not surfaced: main=%v worker=%v", main, worker)
	}
}
