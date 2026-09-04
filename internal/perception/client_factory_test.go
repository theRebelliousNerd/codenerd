package perception

import (
	"context"
	"strings"
	"testing"

	"codenerd/internal/config"
)

// TODO: TEST_GAP: [Null/Undefined/Empty] Missing tests for `providerKeyFieldName` when passing an empty string or undefined string.
// TODO: TEST_GAP: [Type Coercion] Testing behavior when string inputs simulate invalid or partially formed integer configurations implicitly mapped by upstream config loaders.
// TODO: TEST_GAP: [User Request Extremes] Testing massive strings for models or keys in `NewClientFromConfig` and massive strings in `providerKeyFieldName`.
// TODO: TEST_GAP: [State Conflicts] Testing concurrency behavior of reading env variables while they are modified by other tests/goroutines (though Go limits os.Setenv concurrency safety natively, the package logic still lacks assertions for TOC/TOU or env reset behavior).
// TODO: TEST_GAP: [Null/Undefined/Empty] Verify fallback behavior when APIKey is an empty string for providers that require authentication.
// TODO: TEST_GAP: [Type Coercion] Verify handling of unexpected casing (e.g., "AnThroPic") or padding in the Provider string to ensure correct provider resolution.
// TODO: TEST_GAP: [State Conflicts] Verify thread safety of NewClientFromConfig if invoked concurrently with shared ProviderConfig pointers that might be mutated elsewhere.

func TestNewClientFromConfig_NilConfig(t *testing.T) {
	client, err := NewClientFromConfig(nil)
	if err == nil {
		t.Fatal("NewClientFromConfig(nil) error = nil; want explicit configuration error")
	}
	if client != nil {
		t.Fatalf("NewClientFromConfig(nil) client = %T; want nil", client)
	}
	if !strings.Contains(err.Error(), "provider config is nil") {
		t.Fatalf("NewClientFromConfig(nil) error = %q; want actionable nil-config message", err)
	}
}

func TestNewClientFromConfig_Engines(t *testing.T) {
	// 1. Claude CLI
	cfg := &ProviderConfig{
		Engine: "claude-cli",
		ClaudeCLI: &config.ClaudeCLIConfig{
			Model: "sonnet",
		},
	}
	client, err := NewClientFromConfig(cfg)
	if err != nil {
		t.Fatalf("Failed to create claude-cli client: %v", err)
	}
	if _, ok := client.(*ClaudeCodeCLIClient); !ok {
		t.Errorf("Expected *ClaudeCodeCLIClient, got %T", client)
	}

	// 2. Codex CLI
	cfg = &ProviderConfig{
		Engine: "codex-cli",
		CodexCLI: &config.CodexCLIConfig{
			Model: "gpt-5",
		},
	}
	client, err = NewClientFromConfig(cfg)
	if err != nil {
		t.Fatalf("Failed to create codex-cli client: %v", err)
	}
	if client == nil {
		t.Fatal("Expected non-nil codex-cli client")
	}

	// 3. SuperGrok OAuth
	importGrok := false
	cfg = &ProviderConfig{
		Engine: "xai-oauth",
		XAIOAuth: &config.XAIOAuthConfig{
			Model:          "grok-4.5",
			ImportGrokAuth: &importGrok,
		},
	}
	client, err = NewClientFromConfig(cfg)
	if err != nil {
		t.Fatalf("Failed to create xai-oauth client: %v", err)
	}
	if client == nil {
		t.Fatal("Expected non-nil xai-oauth client")
	}

	// 4. Invalid Engine
	cfg = &ProviderConfig{
		Engine: "invalid-cli",
	}
	_, err = NewClientFromConfig(cfg)
	if err == nil {
		t.Error("Expected error for invalid engine")
	}
}

func TestNewClientFromConfig_Providers(t *testing.T) {
	// 1. Anthropic
	cfg := &ProviderConfig{
		Provider: ProviderAnthropic,
		APIKey:   "sk-ant-test",
	}
	client, err := NewClientFromConfig(cfg)
	if err != nil {
		t.Fatalf("Failed to create Anthropic client: %v", err)
	}
	if _, ok := client.(*AnthropicClient); !ok {
		t.Errorf("Expected *AnthropicClient, got %T", client)
	}

	// 2. OpenAI
	cfg = &ProviderConfig{
		Provider: ProviderOpenAI,
		APIKey:   "sk-openai-test",
	}
	client, err = NewClientFromConfig(cfg)
	if err != nil {
		t.Fatalf("Failed to create OpenAI client: %v", err)
	}
	if _, ok := client.(*OpenAIClient); !ok {
		t.Errorf("Expected *OpenAIClient, got %T", client)
	}

	// 3. Gemini (with config)
	cfg = &ProviderConfig{
		Provider: ProviderGemini,
		APIKey:   "gemini-key",
		Gemini: &config.GeminiProviderConfig{
			EnableThinking: true,
			ThinkingLevel:  "high",
		},
	}
	client, err = NewClientFromConfig(cfg)
	if err != nil {
		t.Fatalf("Failed to create Gemini client: %v", err)
	}
	if geminiClient, ok := client.(*GeminiClient); !ok {
		t.Errorf("Expected *GeminiClient, got %T", client)
	} else {
		// Verify config propagated using interface method
		if !geminiClient.IsThinkingEnabled() {
			t.Error("Gemini config EnableThinking not propagated")
		}
	}

	// 4. Unknown Provider
	cfg = &ProviderConfig{
		Provider: Provider("unknown"),
		APIKey:   "key",
	}
	_, err = NewClientFromConfig(cfg)
	if err == nil {
		t.Error("Expected error for unknown provider")
	}
}

func TestNewClientFromConfig_MetaReasoningEffort_XHigh(t *testing.T) {
	cfg := &ProviderConfig{Provider: ProviderMeta, APIKey: "k", ReasoningEffort: "xhigh"}
	client, err := NewClientFromConfig(cfg)
	if err != nil {
		t.Fatalf("NewClientFromConfig: %v", err)
	}
	compat, ok := client.(*OpenAICompatClient)
	if !ok {
		t.Fatalf("got %T, want *OpenAICompatClient", client)
	}
	req := compat.buildRequest(context.Background(), nil, true)
	if req.ReasoningEffort != "xhigh" {
		t.Fatalf("main request reasoning_effort = %q, want xhigh", req.ReasoningEffort)
	}
	class, err := NewClassificationClientFromConfig(cfg)
	if err != nil {
		t.Fatalf("NewClassificationClientFromConfig: %v", err)
	}
	classCompat, ok := class.(*OpenAICompatClient)
	if !ok {
		t.Fatalf("classification got %T, want *OpenAICompatClient", class)
	}
	creq := classCompat.buildRequest(context.Background(), nil, false)
	if creq.ReasoningEffort != "xhigh" {
		t.Fatalf("classification reasoning_effort = %q, want xhigh", creq.ReasoningEffort)
	}
}

func TestNewClientFromConfig_InvalidMetaEffortRejects(t *testing.T) {
	cfg := &ProviderConfig{Provider: ProviderMeta, APIKey: "k", ReasoningEffort: "nope"}
	if _, err := NewClientFromConfig(cfg); err == nil {
		t.Fatal("expected error for invalid reasoning_effort")
	}
}

func TestNewClientFromConfig_DashScopeNeverEmitsReasoningEffort(t *testing.T) {
	cfg := &ProviderConfig{Provider: ProviderDashScope, APIKey: "k", ReasoningEffort: "xhigh"}
	client, err := NewClientFromConfig(cfg)
	if err != nil {
		t.Fatalf("NewClientFromConfig: %v", err)
	}
	compat := client.(*OpenAICompatClient)
	req := compat.buildRequest(context.Background(), nil, true)
	if req.ReasoningEffort != "" {
		t.Fatalf("dashscope reasoning_effort = %q, want empty", req.ReasoningEffort)
	}
}

func TestSecondarySlotClient_MetaPerSlotXHigh(t *testing.T) {
	dsClient, err := newSecondarySlotClient(&config.UserConfig{
		MetaAPIKey: "mk",
		BaseURL:    "",
	}, "worker", &config.SecondaryLLMConfig{Provider: "meta", Model: "muse-spark-1.3-contributor", ReasoningEffort: "xhigh"})
	if err != nil {
		t.Fatalf("worker newSecondarySlotClient: %v", err)
	}
	if got := dsClient.(*OpenAICompatClient).buildRequest(context.Background(), nil, true).ReasoningEffort; got != "xhigh" {
		t.Fatalf("worker reasoning_effort = %q, want xhigh", got)
	}
	plannerClient, err := newSecondarySlotClient(&config.UserConfig{
		DashScopeAPIKey: "dk",
		MetaAPIKey:      "mk2",
		BaseURL:         "",
	}, "planner", &config.SecondaryLLMConfig{Provider: "meta", Model: "muse-spark-1.3-contributor", ReasoningEffort: "xhigh"})
	if err != nil {
		t.Fatalf("planner newSecondarySlotClient: %v", err)
	}
	if got := plannerClient.(*OpenAICompatClient).buildRequest(context.Background(), nil, true).ReasoningEffort; got != "xhigh" {
		t.Fatalf("planner reasoning_effort = %q, want xhigh", got)
	}
}

func TestProviderConfigFromUserConfig_MetaXHigh_FullRootRoute(t *testing.T) {
	userCfg := &config.UserConfig{
		Provider:            "meta",
		MetaAPIKey:          "dummy-meta-key",
		Model:               "muse-spark-1.3-contributor",
		ClassificationModel: "muse-spark-1.3-contributor",
		ReasoningEffort:     "xhigh",
	}
	pc, err := ProviderConfigFromUserConfig(userCfg)
	if err != nil {
		t.Fatalf("ProviderConfigFromUserConfig: %v", err)
	}
	if pc.Provider != ProviderMeta {
		t.Fatalf("Provider = %q, want %q", pc.Provider, ProviderMeta)
	}
	if pc.Model != "muse-spark-1.3-contributor" {
		t.Fatalf("Model = %q, want muse-spark-1.3-contributor", pc.Model)
	}
	if pc.ClassificationModel == "" {
		t.Fatal("ClassificationModel = empty, want non-empty (classification_model set)")
	}
	if pc.ReasoningEffort != "xhigh" {
		t.Fatalf("ReasoningEffort = %q, want xhigh", pc.ReasoningEffort)
	}

	client, err := NewClientFromConfig(pc)
	if err != nil {
		t.Fatalf("NewClientFromConfig: %v", err)
	}
	compat, ok := client.(*OpenAICompatClient)
	if !ok {
		t.Fatalf("main client got %T, want *OpenAICompatClient", client)
	}
	req := compat.buildRequest(context.Background(), nil, true)
	if req.ReasoningEffort != "xhigh" {
		t.Fatalf("main request reasoning_effort = %q, want xhigh", req.ReasoningEffort)
	}

	classClient, err := NewClassificationClientFromConfig(pc)
	if err != nil {
		t.Fatalf("NewClassificationClientFromConfig: %v", err)
	}
	if classClient == nil {
		t.Fatal("classification client = nil, want non-nil for meta with classification_model")
	}
	classCompat, ok := classClient.(*OpenAICompatClient)
	if !ok {
		t.Fatalf("classification client got %T, want *OpenAICompatClient", classClient)
	}
	creq := classCompat.buildRequest(context.Background(), nil, false)
	if creq.ReasoningEffort != "xhigh" {
		t.Fatalf("classification request reasoning_effort = %q, want xhigh (thinking=false)", creq.ReasoningEffort)
	}
}

func TestNewClassificationClientFromConfig_MetaDefaultsMinimal(t *testing.T) {
	cfg := &ProviderConfig{Provider: ProviderMeta, APIKey: "k"}
	class, err := NewClassificationClientFromConfig(cfg)
	if err != nil {
		t.Fatalf("NewClassificationClientFromConfig: %v", err)
	}
	compat, ok := class.(*OpenAICompatClient)
	if !ok {
		t.Fatalf("classification got %T, want *OpenAICompatClient", class)
	}
	creq := compat.buildRequest(context.Background(), nil, false)
	if creq.ReasoningEffort != "minimal" {
		t.Fatalf("classification reasoning_effort = %q, want minimal", creq.ReasoningEffort)
	}
}

func TestNewClassificationClientFromConfig_MetaHonorsExplicitLow(t *testing.T) {
	cfg := &ProviderConfig{Provider: ProviderMeta, APIKey: "k", ReasoningEffort: "low"}
	class, err := NewClassificationClientFromConfig(cfg)
	if err != nil {
		t.Fatalf("NewClassificationClientFromConfig: %v", err)
	}
	compat, ok := class.(*OpenAICompatClient)
	if !ok {
		t.Fatalf("classification got %T, want *OpenAICompatClient", class)
	}
	creq := compat.buildRequest(context.Background(), nil, false)
	if creq.ReasoningEffort != "low" {
		t.Fatalf("classification reasoning_effort = %q, want low", creq.ReasoningEffort)
	}
}

func TestNewClassificationClientFromConfig_DashScopeNoReasoningEffort(t *testing.T) {
	cfg := &ProviderConfig{Provider: ProviderDashScope, APIKey: "k"}
	class, err := NewClassificationClientFromConfig(cfg)
	if err != nil {
		t.Fatalf("NewClassificationClientFromConfig: %v", err)
	}
	compat, ok := class.(*OpenAICompatClient)
	if !ok {
		t.Fatalf("classification got %T, want *OpenAICompatClient", class)
	}
	creq := compat.buildRequest(context.Background(), nil, false)
	if creq.ReasoningEffort != "" {
		t.Fatalf("dashscope classification reasoning_effort = %q, want empty", creq.ReasoningEffort)
	}
}
