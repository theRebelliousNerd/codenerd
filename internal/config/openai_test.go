package config

import (
	"strings"
	"testing"
)

func TestOpenAIModelVersion(t *testing.T) {
	cases := map[string][3]int{ // major, minor, ok
		"gpt-6.1-sol":             {6, 1, 1},
		"gpt-6-astra":             {6, 0, 1},
		"gpt-5.5":                 {5, 5, 1},
		"gpt-5.4-mini-2026-03-17": {5, 4, 1},
		"gpt-4o":                  {4, 0, 1},
		"o4-mini":                 {0, 0, 0},
		"claude-opus-5-5":         {0, 0, 0},
	}
	for model, want := range cases {
		major, minor, ok := OpenAIModelVersion(model)
		if major != want[0] || minor != want[1] || ok != (want[2] == 1) {
			t.Errorf("%s = %d.%d %v, want %v", model, major, minor, ok, want)
		}
	}
}

func TestOpenAIEffortsFollowTheModel(t *testing.T) {
	for _, tc := range []struct {
		model, effort string
		ok            bool
	}{
		{"gpt-6.1-sol", "high", true},
		{"gpt-6.1-sol", "max", true},
		{"gpt-6.1-sol", "none", false},
		{"gpt-6.1-sol", "minimal", false},
		{"gpt-6-astra", "none", false},
		{"gpt-6.1-terra", "none", true},
		{"gpt-6-luna", "none", true},
		{"gpt-5.4", "minimal", true}, // no record: vocabulary only
		{"gpt-6.1-sol", "turbo", false},
		{"", "xhigh", true},
	} {
		err := ValidateReasoningEffort("openai", tc.model, tc.effort)
		if (err == nil) != tc.ok {
			t.Errorf("%s/%s: err=%v, want ok=%v", tc.model, tc.effort, err, tc.ok)
		}
	}
}

func TestOpenAICapabilities(t *testing.T) {
	if !OpenAIUsesCacheOptions("gpt-6.1-sol") || !OpenAIUsesCacheOptions("gpt-5.6-terra") || OpenAIUsesCacheOptions("gpt-5.5") {
		t.Error("cache-options generation misdetected")
	}
	if !OpenAIIsReasoningModel("gpt-6-luna") || !OpenAIIsReasoningModel("o3") || OpenAIIsReasoningModel("gpt-4.1") {
		t.Error("reasoning-model detection wrong")
	}
	for model, want := range map[string]int{"gpt-6.1-sol": 128000, "gpt-4.1-mini": 32768, "gpt-4o": 16384, "mystery": 0} {
		if got := OpenAIMaxOutputTokens(model); got != want {
			t.Errorf("OpenAIMaxOutputTokens(%s)=%d, want %d", model, got, want)
		}
	}
}

func TestOpenAIProviderConfigCheck(t *testing.T) {
	ok := &OpenAIProviderConfig{ServiceTier: "ultrafast", Verbosity: "high", ReasoningMode: "pro", ReasoningContext: "all_turns", ReasoningSummary: "auto", PromptCacheTTL: "12h", SearchContextSize: "low"}
	if p := ok.Check("openai"); len(p) != 0 {
		t.Errorf("valid block refused: %v", p)
	}
	bad := &OpenAIProviderConfig{ServiceTier: "turbo", PromptCacheTTL: "24h"}
	p := bad.Check("openai")
	if len(p) != 2 || !strings.Contains(p[0].Path+p[1].Path, "openai.service_tier") || !strings.Contains(p[0].Path+p[1].Path, "openai.prompt_cache_ttl") {
		t.Errorf("problems=%v", p)
	}
	var none *OpenAIProviderConfig
	if none.WebSearchEnabled() || none.Check("openai") != nil {
		t.Error("nil block is not the default")
	}
}

func TestUserConfigCheckRefusesABadOpenAIBlock(t *testing.T) {
	raw := []byte(`{"provider":"openai","model":"gpt-6.1-sol","openai":{"verbosity":"loud"}}`)
	cfg := &UserConfig{Provider: "openai", Model: "gpt-6.1-sol", OpenAI: &OpenAIProviderConfig{Verbosity: "loud"}}
	found := false
	for _, p := range cfg.Check(raw) {
		if p.Path == "openai.verbosity" && p.Severity == SeverityError {
			found = true
		}
	}
	if !found {
		t.Error("UserConfig.Check did not refuse openai.verbosity loud")
	}
}
