package config

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAPIKeyPools_ResolveAndProviderDetection(t *testing.T) {
	t.Setenv("POOL_TEST_A", "secret-a")
	t.Setenv("POOL_TEST_B", "secret-b")
	raw := []byte(`{"provider":"anthropic","model":"claude-opus-5-5","api_key_pools":{"anthropic":[{"name":"a","env":"POOL_TEST_A","workspace_id":"wrkspc_a"},{"name":"b","env":"POOL_TEST_B"}]}}`)
	var cfg UserConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	keys, err := cfg.ResolveAPIKeyPool("anthropic")
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 2 || keys[0].Key != "secret-a" || keys[1].Key != "secret-b" || keys[0].WorkspaceID != "wrkspc_a" {
		t.Fatal("pool did not resolve")
	}
	if provider, key := cfg.GetActiveProvider(); provider != "anthropic" || key != "secret-a" {
		t.Fatal("pool-only configuration did not select provider")
	}
	if !cfg.HasExplicitLLMSelection() {
		t.Fatal("pool not treated as explicit routing")
	}
	if issues := Errors(cfg.Check(raw)); len(issues) > 0 {
		t.Fatalf("valid pool configuration rejected: %+v", issues)
	}
	cfg.AnthropicAPIKey = "ignored-legacy"
	if cfg.APIKeyForProvider("anthropic") != "secret-a" {
		t.Fatal("scalar key overrode explicit pool")
	}
	cfg.Provider = ""
	if provider, _ := cfg.GetActiveProvider(); provider != "anthropic" {
		t.Fatal("pool auto-detection failed")
	}
	t.Setenv("POOL_TEST_A", "")
	if _, err := cfg.ResolveAPIKeyPool("anthropic"); err == nil || strings.Contains(err.Error(), "secret-b") {
		t.Fatal("missing environment key did not fail safely")
	}
	if _, key := cfg.GetActiveProvider(); key != "" {
		t.Fatal("missing pool credential silently fell back to scalar")
	}
}

func TestAPIKeyPools_RejectInvalidConfigurationWithoutExposingKeys(t *testing.T) {
	for _, entries := range [][]ProviderAPIKey{
		{},
		{{Name: "same", APIKey: "secret-value"}, {Name: "same", APIKey: "another"}},
		{{Name: "a", APIKey: "secret-value"}, {Name: "b", APIKey: "secret-value"}},
		{{Name: "bad name", APIKey: "secret-value"}},
		{{Name: "a", APIKey: "secret-value", Env: "POOL_TEST"}},
		{{Name: "a"}},
		{{Name: "a", APIKey: "secret-value\n"}},
	} {
		cfg := &UserConfig{APIKeyPools: map[string][]ProviderAPIKey{"anthropic": entries}}
		_, err := cfg.ResolveAPIKeyPool("anthropic")
		if err == nil || strings.Contains(err.Error(), "secret-value") {
			t.Fatalf("invalid pool failed unsafely: %v", err)
		}
	}
	cfg := &UserConfig{APIKeyPools: map[string][]ProviderAPIKey{"ollama": {{Name: "a", APIKey: "secret"}}}}
	if _, err := cfg.ResolveAPIKeyPool("ollama"); err == nil {
		t.Fatal("keyless provider accepted a key pool")
	}
}
