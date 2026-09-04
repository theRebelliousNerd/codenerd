package config

import (
	"testing"
)

func TestDefaultBuildConfig(t *testing.T) {
	c := DefaultBuildConfig()
	if c.EnvVars == nil {
		t.Error("EnvVars should be non-nil so callers can assign without a nil-map panic")
	}
	if c.GoFlags == nil || c.CGOPackages == nil {
		t.Error("GoFlags and CGOPackages should be non-nil slices")
	}
}

func TestValidateCoreLimits(t *testing.T) {
	valid := CoreLimits{
		MaxTotalMemoryMB:      1024,
		MaxConcurrentShards:   4,
		MaxConcurrentAPICalls: 2,
		MaxSessionDurationMin: 30,
		MaxFactsInKernel:      10000,
		MaxDerivedFactsLimit:  10000,
	}
	if err := (&valid).ValidateCoreLimits(); err != nil {
		t.Fatalf("valid limits rejected: %v", err)
	}

	bad := []struct {
		name  string
		mutch func(c *CoreLimits)
	}{
		{"low memory", func(c *CoreLimits) { c.MaxTotalMemoryMB = 256 }},
		{"zero shards", func(c *CoreLimits) { c.MaxConcurrentShards = 0 }},
		{"low facts", func(c *CoreLimits) { c.MaxFactsInKernel = 10 }},
		{"low derived", func(c *CoreLimits) { c.MaxDerivedFactsLimit = 10 }},
	}
	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) {
			cl := valid
			tc.mutch(&cl)
			if err := (&cl).ValidateCoreLimits(); err == nil {
				t.Errorf("expected validation error for %s", tc.name)
			}
		})
	}
}

func TestContextWindowBudget(t *testing.T) {
	// Explicit reserves are summed verbatim.
	explicit := ContextWindowConfig{MaxTokens: 1000, OutputReserve: 200, ThinkingReserve: 50, ToolUseBuffer: 100}
	if got := explicit.TotalContextWindow(); got != 1350 {
		t.Errorf("TotalContextWindow explicit=%d, want 1350", got)
	}
	// Zero reserves fall back to defaults (8000 output + 4000 tool buffer).
	defaults := ContextWindowConfig{MaxTokens: 1000}
	if got := defaults.TotalContextWindow(); got != 13000 {
		t.Errorf("TotalContextWindow defaults=%d, want 13000", got)
	}
	if got := explicit.EffectiveInputBudget(); got != 1000 {
		t.Errorf("EffectiveInputBudget=%d, want 1000", got)
	}
}

func TestLLMTimeoutPresets(t *testing.T) {
	fast := FastLLMTimeouts()
	aggressive := AggressiveLLMTimeouts()
	def := DefaultLLMTimeouts()
	// Aggressive should retry no more than fast, which should retry no more than default.
	if aggressive.MaxRetries > fast.MaxRetries {
		t.Errorf("aggressive retries (%d) should be <= fast (%d)", aggressive.MaxRetries, fast.MaxRetries)
	}
	if fast.MaxRetries > def.MaxRetries {
		t.Errorf("fast retries (%d) should be <= default (%d)", fast.MaxRetries, def.MaxRetries)
	}
	if fast.PerCallTimeout <= 0 || aggressive.PerCallTimeout <= 0 {
		t.Error("per-call timeouts must be positive")
	}
}

func TestGlobalLLMTimeouts(t *testing.T) {
	orig := GetLLMTimeouts()
	t.Cleanup(func() { SetLLMTimeouts(orig) })

	custom := AggressiveLLMTimeouts()
	SetLLMTimeouts(custom)
	if GetLLMTimeouts().MaxRetries != custom.MaxRetries {
		t.Error("SetLLMTimeouts/GetLLMTimeouts did not round-trip")
	}
}

func TestDefaultTimeout(t *testing.T) {
	cases := map[string]string{
		"scraper": "120s",
		"browser": "60s",
		"unknown": "30s",
		"":        "30s",
	}
	for id, want := range cases {
		if got := DefaultTimeout(id); got != want {
			t.Errorf("DefaultTimeout(%q)=%q, want %q", id, got, want)
		}
	}
}

func TestToMCPServerConfigs(t *testing.T) {
	ic := &IntegrationsConfig{Servers: map[string]MCPServerIntegration{
		"browser":  {Enabled: true, BaseURL: "http://localhost:1"},   // protocol+timeout default
		"scraper":  {Enabled: true, Protocol: "sse", Timeout: "90s"}, // explicit values preserved
		"disabled": {Enabled: false},                                 // skipped
	}}
	configs := ic.ToMCPServerConfigs()
	if _, ok := configs["disabled"]; ok {
		t.Error("disabled server should not be converted")
	}
	if configs["browser"].Protocol != "http" {
		t.Errorf("browser protocol=%q, want http (default)", configs["browser"].Protocol)
	}
	if configs["browser"].Timeout != "60s" {
		t.Errorf("browser timeout=%q, want 60s (default by id)", configs["browser"].Timeout)
	}
	if configs["scraper"].Protocol != "sse" || configs["scraper"].Timeout != "90s" {
		t.Errorf("scraper config not preserved: %+v", configs["scraper"])
	}

	// Nil servers map yields an empty (non-nil) result.
	if got := (&IntegrationsConfig{}).ToMCPServerConfigs(); got == nil || len(got) != 0 {
		t.Errorf("nil servers should produce empty map, got %v", got)
	}
}

func TestLoggingIsCategoryEnabled(t *testing.T) {
	// Debug off -> everything disabled.
	off := &LoggingConfig{DebugMode: false, Categories: map[string]bool{"x": true}}
	if off.IsCategoryEnabled("x") {
		t.Error("category should be disabled when debug_mode is false")
	}
	// Debug on, nil categories -> all enabled.
	onNil := &LoggingConfig{DebugMode: true}
	if !onNil.IsCategoryEnabled("anything") {
		t.Error("category should default to enabled in debug mode with nil categories")
	}
	// Debug on, explicit toggles.
	on := &LoggingConfig{DebugMode: true, Categories: map[string]bool{"on": true, "off": false}}
	if !on.IsCategoryEnabled("on") {
		t.Error("explicitly-enabled category should be enabled")
	}
	if on.IsCategoryEnabled("off") {
		t.Error("explicitly-disabled category should be disabled")
	}
	if !on.IsCategoryEnabled("unspecified") {
		t.Error("unspecified category should default to enabled in debug mode")
	}
}
