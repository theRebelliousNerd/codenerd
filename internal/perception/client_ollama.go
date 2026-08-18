package perception

import (
	"context"
	"fmt"
	"strings"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// OllamaClient is a first-class LLM client for local Ollama.
// Uses Ollama's OpenAI-compatible API at {endpoint}/v1/chat/completions
// (no cloud API key required; auth header is a placeholder "ollama").
type OllamaClient struct {
	// openai is the shared OpenAI-compatible transport pointed at Ollama.
	openai *OpenAIClient
	model  string
}

// DefaultOllamaLLMConfig returns sensible defaults for local chat.
func DefaultOllamaLLMConfig() OllamaLLMConfig {
	return OllamaLLMConfig{
		Endpoint: "http://127.0.0.1:11434",
		Model:    "gemma4:12b",
		Timeout:  10 * time.Minute,
	}
}

// OllamaLLMConfig holds chat (not embedding) settings for Ollama.
type OllamaLLMConfig struct {
	Endpoint string
	Model    string
	Timeout  time.Duration
}

// NewOllamaClient creates an Ollama chat client with defaults.
func NewOllamaClient(model string) *OllamaClient {
	cfg := DefaultOllamaLLMConfig()
	if model != "" {
		cfg.Model = model
	}
	return NewOllamaClientWithConfig(cfg)
}

// NewOllamaClientWithConfig creates an Ollama chat client from config.
func NewOllamaClientWithConfig(cfg OllamaLLMConfig) *OllamaClient {
	if cfg.Endpoint == "" {
		cfg.Endpoint = "http://127.0.0.1:11434"
	}
	if cfg.Model == "" {
		cfg.Model = "gemma4:12b"
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 10 * time.Minute
	}
	// Strip trailing slash; OpenAI transport appends /chat/completions to BaseURL/v1.
	endpoint := strings.TrimRight(cfg.Endpoint, "/")
	baseURL := endpoint + "/v1"
	openai := NewOpenAIClientWithConfig(OpenAIConfig{
		APIKey:  "ollama", // Ollama ignores auth; OpenAI client requires non-empty key
		BaseURL: baseURL,
		Model:   cfg.Model,
		Timeout: cfg.Timeout,
	})
	// Ollama borrows the OpenAI transport but is its own provider for billing
	// purposes: local tokens cost nothing and must not inflate the openai row.
	openai.provider = ProviderOllama
	logging.Perception("Ollama chat client: endpoint=%s model=%s", endpoint, cfg.Model)
	return &OllamaClient{openai: openai, model: cfg.Model}
}

// Complete implements LLMClient.
func (c *OllamaClient) Complete(ctx context.Context, prompt string) (string, error) {
	return c.openai.Complete(ctx, prompt)
}

// CompleteWithSystem implements LLMClient.
func (c *OllamaClient) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return c.openai.CompleteWithSystem(ctx, systemPrompt, userPrompt)
}

// CompleteWithTools implements LLMClient.
func (c *OllamaClient) CompleteWithTools(ctx context.Context, systemPrompt, userPrompt string, tools []ToolDefinition) (*LLMToolResponse, error) {
	return c.openai.CompleteWithTools(ctx, systemPrompt, userPrompt, tools)
}

// CompleteWithStreaming implements LLMClient.
func (c *OllamaClient) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (<-chan string, <-chan error) {
	return c.openai.CompleteWithStreaming(ctx, systemPrompt, userPrompt, enableThinking)
}

// CompleteWithToolResults implements types.ToolResultsProvider so multi-turn
// tool loops work for local Ollama the same way as xAI/OpenAI.
func (c *OllamaClient) CompleteWithToolResults(ctx context.Context, systemPrompt string, history []types.Message, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	// OpenAIClient may not expose ToolResultsProvider yet — use shared helpers.
	msgs, err := MapTypesHistoryToOpenAIMessages(systemPrompt, history)
	if err != nil {
		return nil, err
	}
	pTools := make([]ToolDefinition, len(tools))
	for i, t := range tools {
		pTools[i] = ToolDefinition{Name: t.Name, Description: t.Description, InputSchema: t.InputSchema}
	}
	reqBody := OpenAIRequest{
		Model:      c.model,
		Messages:   msgs,
		Tools:      MapToolDefinitionsToOpenAI(pTools),
		ToolChoice: "auto",
		Stream:     false,
	}
	resp, err := ExecuteOpenAIRequest(ctx, c.openai.httpClient, c.openai.baseURL, c.openai.apiKey, reqBody)
	if err != nil {
		return nil, fmt.Errorf("ollama tool-results: %w", err)
	}
	trackUsage(ctx, c.model, ProviderOllama,
		resp.Usage.PromptTokens, resp.Usage.CompletionTokens, usageOpFor(len(tools)))
	return OpenAIToolResponseFromResponse(resp)
}

// SetModel updates the model name.
func (c *OllamaClient) SetModel(model string) {
	if model == "" {
		return
	}
	c.model = model
	c.openai.SetModel(model)
}

// GetModel returns the configured model.
func (c *OllamaClient) GetModel() string {
	return c.model
}

// Name returns a debug label.
func (c *OllamaClient) Name() string {
	return fmt.Sprintf("ollama:%s", c.model)
}

// ModelIdentity reports the provider and model this client serves, satisfying
// types.ModelIdentifier. The model is the resolved one -- the vendor default
// when config supplied no override -- which is what prompt-atom pinning must
// key on.
func (c *OllamaClient) ModelIdentity() (string, string) {
	return string(ProviderOllama), c.model
}
