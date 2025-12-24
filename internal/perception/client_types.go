package perception

import (
	"context"
	"time"
)

const defaultSystemPrompt = "You are codeNERD. Respond in English. Be concise. When summarizing code, ground answers only in provided text. Do not claim to browse the filesystem or network; only use supplied content."

// LLMClient defines the interface for LLM providers.
type LLMClient interface {
	Complete(ctx context.Context, prompt string) (string, error)
	CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error)
}

// Provider represents an LLM provider.
type Provider string

const (
	ProviderZAI        Provider = "zai"
	ProviderAnthropic  Provider = "anthropic"
	ProviderOpenAI     Provider = "openai"
	ProviderGemini     Provider = "gemini"
	ProviderXAI        Provider = "xai"
	ProviderOpenRouter Provider = "openrouter"
)

// ZAIConfig holds configuration for ZAI client.
type ZAIConfig struct {
	APIKey           string
	BaseURL          string
	Model            string
	Timeout          time.Duration
	SystemPrompt     string
	DisableSemaphore bool // Set true when using external APIScheduler for concurrency control
}

// AnthropicConfig holds configuration for Anthropic client.
type AnthropicConfig struct {
	APIKey  string
	BaseURL string
	Model   string
	Timeout time.Duration
}

// OpenAIConfig holds configuration for OpenAI client.
type OpenAIConfig struct {
	APIKey  string
	BaseURL string
	Model   string
	Timeout time.Duration
}

// GeminiConfig holds configuration for Gemini client.
type GeminiConfig struct {
	APIKey  string
	BaseURL string
	Model   string
	Timeout time.Duration
}

// XAIConfig holds configuration for xAI client.
type XAIConfig struct {
	APIKey  string
	BaseURL string
	Model   string
	Timeout time.Duration
}

// OpenRouterConfig holds configuration for OpenRouter client.
type OpenRouterConfig struct {
	APIKey   string
	BaseURL  string
	Model    string
	Timeout  time.Duration
	SiteURL  string // Optional
	SiteName string // Optional
}

// ZAIStreamOptions configures streaming behavior.
type ZAIStreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

// ZAIResponseFormat enforces structured output (JSON schema).
type ZAIResponseFormat struct {
	Type       string         `json:"type"` // "json_schema"
	JSONSchema *ZAIJSONSchema `json:"json_schema,omitempty"`
}

// ZAIJSONSchema defines the structured output schema.
type ZAIJSONSchema struct {
	Name   string                 `json:"name"`
	Strict bool                   `json:"strict"`
	Schema map[string]interface{} `json:"schema"`
}

// ZAIThinking enables extended reasoning mode.
type ZAIThinking struct {
	Type         string `json:"type"`                    // "enabled"
	BudgetTokens int    `json:"budget_tokens,omitempty"` // Optional token budget
}

// ZAIMessage represents a message in the conversation.
type ZAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ZAIRequest represents the API request structure (Enhanced for v1.2.0).
type ZAIRequest struct {
	Model          string             `json:"model"`
	Messages       []ZAIMessage       `json:"messages"`
	MaxTokens      int                `json:"max_tokens,omitempty"`
	Temperature    float64            `json:"temperature,omitempty"`
	TopP           float64            `json:"top_p,omitempty"`
	Stream         bool               `json:"stream,omitempty"`
	StreamOptions  *ZAIStreamOptions  `json:"stream_options,omitempty"`
	ResponseFormat *ZAIResponseFormat `json:"response_format,omitempty"` // Structured output
	Thinking       *ZAIThinking       `json:"thinking,omitempty"`        // Extended reasoning
}

// ZAIResponse represents the API response structure (Enhanced for v1.2.0).
type ZAIResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		Delta *struct { // For streaming
			Role    string `json:"role,omitempty"`
			Content string `json:"content,omitempty"`
		} `json:"delta,omitempty"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
		// Thinking mode tokens
		ThinkingTokens int `json:"thinking_tokens,omitempty"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

// AnthropicMessage represents a message.
type AnthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// AnthropicRequest represents the Anthropic API request.
type AnthropicRequest struct {
	Model       string             `json:"model"`
	MaxTokens   int                `json:"max_tokens"`
	System      string             `json:"system,omitempty"`
	Messages    []AnthropicMessage `json:"messages"`
	Temperature float64            `json:"temperature,omitempty"`
	Stream      bool               `json:"stream,omitempty"`
}

// AnthropicResponse represents the API response.
type AnthropicResponse struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Role    string `json:"role"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Model        string `json:"model"`
	StopReason   string `json:"stop_reason"`
	StopSequence string `json:"stop_sequence"`
	Usage        struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// OpenAIStreamOptions configures streaming behavior.
type OpenAIStreamOptions struct {
	IncludeUsage bool `json:"include_usage,omitempty"`
}

// OpenAIMessage represents a message.
type OpenAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OpenAIRequest represents the OpenAI API request.
type OpenAIRequest struct {
	Model          string               `json:"model"`
	Messages       []OpenAIMessage      `json:"messages"`
	MaxTokens      int                  `json:"max_tokens,omitempty"`
	Temperature    float64              `json:"temperature,omitempty"`
	Stream         bool                 `json:"stream,omitempty"`
	StreamOptions  *OpenAIStreamOptions `json:"stream_options,omitempty"`
	ResponseFormat *ZAIResponseFormat   `json:"response_format,omitempty"`
}

// OpenAIResponse represents the API response.
type OpenAIResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		Delta *struct { // For streaming
			Role    string `json:"role,omitempty"`
			Content string `json:"content,omitempty"`
		} `json:"delta,omitempty"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

// GeminiContent represents content in the request.
type GeminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []GeminiPart `json:"parts"`
}

// GeminiPart represents a part of the content.
type GeminiPart struct {
	Text string `json:"text"`
}

// GeminiGenerationConfig represents generation parameters.
// Note: Gemini REST API uses snake_case for these fields.
type GeminiGenerationConfig struct {
	Temperature      float64                `json:"temperature,omitempty"`
	MaxOutputTokens  int                    `json:"maxOutputTokens,omitempty"`
	ResponseMimeType string                 `json:"response_mime_type,omitempty"`
	ResponseSchema   map[string]interface{} `json:"response_schema,omitempty"`
}

// GeminiRequest represents the Gemini API request.
type GeminiRequest struct {
	Contents          []GeminiContent        `json:"contents"`
	SystemInstruction *GeminiContent         `json:"systemInstruction,omitempty"`
	GenerationConfig  GeminiGenerationConfig `json:"generationConfig,omitempty"`
}

// GeminiResponse represents the API response.
type GeminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
			Role string `json:"role"`
		} `json:"content"`
		FinishReason string `json:"finishReason"`
	} `json:"candidates"`
	UsageMetadata struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
		TotalTokenCount      int `json:"totalTokenCount"`
	} `json:"usageMetadata"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error,omitempty"`
}

// XAI uses OpenAI-compatible API format
type XAIRequest = OpenAIRequest
type XAIMessage = OpenAIMessage
type XAIResponse = OpenAIResponse

// OpenRouter uses OpenAI-compatible request/response format
type OpenRouterRequest = OpenAIRequest
type OpenRouterMessage = OpenAIMessage
type OpenRouterResponse = OpenAIResponse

// Popular OpenRouter models (use provider/model format)
// Full list at: https://openrouter.ai/models
var OpenRouterModels = []string{
	// Anthropic
	"anthropic/claude-3.5-sonnet",
	"anthropic/claude-3.5-haiku",
	"anthropic/claude-3-opus",
	// OpenAI
	"openai/gpt-4o",
	"openai/gpt-4o-mini",
	"openai/o1-preview",
	"openai/o1-mini",
	// Google
	"google/gemini-3-flash-preview",
	"google/gemini-2.0-flash-exp:free",
	"google/gemini-pro-1.5",
	// Meta
	"meta-llama/llama-3.1-405b-instruct",
	"meta-llama/llama-3.1-70b-instruct",
	// Mistral
	"mistralai/mistral-large",
	"mistralai/codestral-latest",
	// DeepSeek
	"deepseek/deepseek-chat",
	"deepseek/deepseek-coder",
	// Qwen
	"qwen/qwen-2.5-72b-instruct",
	"qwen/qwen-2.5-coder-32b-instruct",
}
