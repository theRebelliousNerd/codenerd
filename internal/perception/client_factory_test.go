package perception

import (
	"codenerd/internal/broker"
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

// The factory now returns metered clients. Every concrete-type assertion in
// this file therefore goes through broker.Base, which is the same thing
// production code does (see cmd/nerd/chat/model_session_context.go). An
// assertion that stopped matching here would have stopped matching there too,
// which is exactly why these tests are worth keeping in this shape rather than
// relaxing them to interface checks.

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
	if _, ok := broker.Base(client).(*ClaudeCodeCLIClient); !ok {
		t.Errorf("Expected *ClaudeCodeCLIClient, got %T", broker.Base(client))
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

	// 3. Invalid Engine
	cfg = &ProviderConfig{
		Engine: "invalid-cli",
		Model:  "test-model",
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
		Model:    "test-model",
		APIKey:   "sk-ant-test",
	}
	client, err := NewClientFromConfig(cfg)
	if err != nil {
		t.Fatalf("Failed to create Anthropic client: %v", err)
	}
	if _, ok := broker.Base(client).(*AnthropicClient); !ok {
		t.Errorf("Expected *AnthropicClient, got %T", broker.Base(client))
	}

	// 2. OpenAI
	cfg = &ProviderConfig{
		Provider: ProviderOpenAI,
		Model:    "test-model",
		APIKey:   "sk-openai-test",
	}
	client, err = NewClientFromConfig(cfg)
	if err != nil {
		t.Fatalf("Failed to create OpenAI client: %v", err)
	}
	if _, ok := broker.Base(client).(*OpenAIClient); !ok {
		t.Errorf("Expected *OpenAIClient, got %T", broker.Base(client))
	}

	// 3. Gemini (with config)
	cfg = &ProviderConfig{
		Provider: ProviderGemini,
		Model:    "test-model",
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
	if geminiClient, ok := broker.Base(client).(*GeminiClient); !ok {
		t.Errorf("Expected *GeminiClient, got %T", broker.Base(client))
	} else {
		// Verify config propagated using interface method
		if !geminiClient.IsThinkingEnabled() {
			t.Error("Gemini config EnableThinking not propagated")
		}
	}

	// 4. Unknown Provider
	cfg = &ProviderConfig{
		Provider: Provider("unknown"),
		Model:    "test-model",
		APIKey:   "key",
	}
	_, err = NewClientFromConfig(cfg)
	if err == nil {
		t.Error("Expected error for unknown provider")
	}
}

func TestNewClientFromConfig_MetaReasoningEffort_XHigh(t *testing.T) {
	cfg := &ProviderConfig{Model: metaContributorModel, Provider: ProviderMeta, APIKey: "k", ReasoningEffort: "xhigh"}
	client, err := NewClientFromConfig(cfg)
	if err != nil {
		t.Fatalf("NewClientFromConfig: %v", err)
	}
	compat, ok := broker.Base(client).(*OpenAICompatClient)
	if !ok {
		t.Fatalf("got %T, want *OpenAICompatClient", client)
	}
	req := compat.buildRequest(context.Background(), nil, true)
	if req.ReasoningEffort != "xhigh" {
		t.Fatalf("main request reasoning_effort = %q, want xhigh", req.ReasoningEffort)
	}
	// The classifier is sent the config's classification effort, never the
	// main slot's working effort.
	cfg.ClassificationReasoningEffort = "minimal"
	class, err := NewClassificationClientFromConfig(cfg)
	if err != nil {
		t.Fatalf("NewClassificationClientFromConfig: %v", err)
	}
	classCompat, ok := broker.Base(class).(*OpenAICompatClient)
	if !ok {
		t.Fatalf("classification got %T, want *OpenAICompatClient", class)
	}
	creq := classCompat.buildRequest(context.Background(), nil, false)
	if creq.ReasoningEffort != "minimal" {
		t.Fatalf("classification reasoning_effort = %q, want minimal (the classification effort, not the main slot's xhigh)", creq.ReasoningEffort)
	}
}

func TestNewClientFromConfig_InvalidMetaEffortRejects(t *testing.T) {
	cfg := &ProviderConfig{Model: metaContributorModel, Provider: ProviderMeta, APIKey: "k", ReasoningEffort: "nope"}
	if _, err := NewClientFromConfig(cfg); err == nil {
		t.Fatal("expected error for invalid reasoning_effort")
	}
}

func TestNewClientFromConfig_DashScopeNeverEmitsReasoningEffort(t *testing.T) {
	cfg := &ProviderConfig{Model: "test-model", Provider: ProviderDashScope, APIKey: "k", ReasoningEffort: "xhigh"}
	client, err := NewClientFromConfig(cfg)
	if err != nil {
		t.Fatalf("NewClientFromConfig: %v", err)
	}
	compat := broker.Base(client).(*OpenAICompatClient)
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
	if got := broker.Base(dsClient).(*OpenAICompatClient).buildRequest(context.Background(), nil, true).ReasoningEffort; got != "xhigh" {
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
	if got := broker.Base(plannerClient).(*OpenAICompatClient).buildRequest(context.Background(), nil, true).ReasoningEffort; got != "xhigh" {
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
	compat, ok := broker.Base(client).(*OpenAICompatClient)
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
	classCompat, ok := broker.Base(classClient).(*OpenAICompatClient)
	if !ok {
		t.Fatalf("classification client got %T, want *OpenAICompatClient", classClient)
	}
	// classification_reasoning_effort is unset, so the classifier on Meta is
	// sent Meta's lowest effort, with thinking off: the main slot's xhigh does
	// not leak into the per-turn label.
	if pc.ClassificationReasoningEffort != "minimal" {
		t.Fatalf("ClassificationReasoningEffort = %q, want minimal", pc.ClassificationReasoningEffort)
	}
	creq := classCompat.buildRequest(context.Background(), nil, false)
	if creq.ReasoningEffort != "minimal" {
		t.Fatalf("classification request reasoning_effort = %q, want minimal (thinking=false)", creq.ReasoningEffort)
	}
}

// The Meta classifier is sent the classification effort config resolves:
// classification_reasoning_effort when set, else Meta's lowest (minimal). The
// default lives in config, so a ProviderConfig that carries none sends none.
func TestNewClassificationClientFromConfig_MetaSendsTheClassificationEffort(t *testing.T) {
	tests := []struct {
		name string
		user *config.UserConfig
		want string
	}{
		{name: "unset defaults to Meta's lowest", want: "minimal",
			user: &config.UserConfig{Provider: "meta", MetaAPIKey: "k", Model: metaContributorModel, ReasoningEffort: "xhigh"}},
		{name: "an explicit effort is sent", want: "low",
			user: &config.UserConfig{Provider: "meta", MetaAPIKey: "k", Model: metaContributorModel, ClassificationReasoningEffort: "low"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pc, err := ProviderConfigFromUserConfig(tt.user)
			if err != nil {
				t.Fatalf("ProviderConfigFromUserConfig: %v", err)
			}
			class, err := NewClassificationClientFromConfig(pc)
			if err != nil {
				t.Fatalf("NewClassificationClientFromConfig: %v", err)
			}
			compat, ok := broker.Base(class).(*OpenAICompatClient)
			if !ok {
				t.Fatalf("classification got %T, want *OpenAICompatClient", class)
			}
			if got := compat.buildRequest(context.Background(), nil, false).ReasoningEffort; got != tt.want {
				t.Fatalf("classification reasoning_effort = %q, want %q", got, tt.want)
			}
		})
	}
	bare, err := NewClassificationClientFromConfig(&ProviderConfig{Model: metaContributorModel, Provider: ProviderMeta, APIKey: "k"})
	if err != nil {
		t.Fatalf("NewClassificationClientFromConfig: %v", err)
	}
	if got := broker.Base(bare).(*OpenAICompatClient).buildRequest(context.Background(), nil, false).ReasoningEffort; got != "" {
		t.Errorf("a config carrying no classification effort was sent %q; perception invents none", got)
	}
}

func TestNewClassificationClientFromConfig_DashScopeNoReasoningEffort(t *testing.T) {
	cfg := &ProviderConfig{Model: "test-model", Provider: ProviderDashScope, APIKey: "k"}
	class, err := NewClassificationClientFromConfig(cfg)
	if err != nil {
		t.Fatalf("NewClassificationClientFromConfig: %v", err)
	}
	compat, ok := broker.Base(class).(*OpenAICompatClient)
	if !ok {
		t.Fatalf("classification got %T, want *OpenAICompatClient", class)
	}
	creq := compat.buildRequest(context.Background(), nil, false)
	if creq.ReasoningEffort != "" {
		t.Fatalf("dashscope classification reasoning_effort = %q, want empty", creq.ReasoningEffort)
	}
}
