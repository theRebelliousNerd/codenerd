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
	// openai is Ollama's Chat Completions transport.
	openai *ollamaTransport
	model  string
}

// DefaultOllamaLLMConfig returns the local endpoint and timeout defaults for
// chat. It names no model: the tag comes from config (ollama.model, or the
// slot's model) or the client is not built.
func DefaultOllamaLLMConfig() OllamaLLMConfig {
	return OllamaLLMConfig{
		Endpoint: "http://127.0.0.1:11434",
		Timeout:  10 * time.Minute,
	}
}

// OllamaLLMConfig holds chat (not embedding) settings for Ollama.
type OllamaLLMConfig struct {
	Endpoint string
	Model    string
	Timeout  time.Duration
}

// NewOllamaClientWithConfig creates an Ollama chat client from config.
func NewOllamaClientWithConfig(cfg OllamaLLMConfig) *OllamaClient {
	if cfg.Endpoint == "" {
		cfg.Endpoint = "http://127.0.0.1:11434"
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 10 * time.Minute
	}
	// Strip trailing slash; OpenAI transport appends /chat/completions to BaseURL/v1.
	endpoint := strings.TrimRight(cfg.Endpoint, "/")
	baseURL := endpoint + "/v1"
	openai := newOllamaTransport(baseURL, cfg.Model, cfg.Timeout, 0)
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
func (c *OllamaClient) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (<-chan string, <-chan types.StreamResult) {
	return c.openai.CompleteWithStreaming(ctx, systemPrompt, userPrompt, enableThinking)
}

// CompleteWithToolResults implements types.ToolResultsProvider so multi-turn
// tool loops work for local Ollama the same way as for the cloud vendors.
func (c *OllamaClient) CompleteWithToolResults(ctx context.Context, systemPrompt string, history []types.Message, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	// The transport's single-turn methods take a prompt, not a history, so
	// the multi-turn request is built here against the same transport.
	msgs, err := MapTypesHistoryToOpenAIMessages(systemPrompt, history)
	if err != nil {
		return nil, err
	}
	pTools := make([]ToolDefinition, len(tools))
	for i, t := range tools {
		pTools[i] = ToolDefinition(t)
	}
	reqBody := OpenAIRequest{
		Model: c.model,
		// The ceiling must ride along: without it max_tokens is omitted
		// (omitzero) and tool loops truncate at Ollama's own default
		// instead of this client's like every sibling path.
		MaxTokens:  c.openai.maxOutputTokens,
		Messages:   msgs,
		Tools:      MapToolDefinitionsToOpenAI(pTools),
		ToolChoice: "auto",
		Stream:     false,
	}
	resp, err := ExecuteOpenAIRequest(ctx, c.openai.httpClient, c.openai.baseURL, c.openai.apiKey, reqBody)
	if err != nil {
		return nil, fmt.Errorf("ollama tool-results: %w", err)
	}
	reportUsage(ctx, c.model, ProviderOllama, usageOpFor(len(tools)), resp.Usage.metadata())
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
