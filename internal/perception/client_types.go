package perception

import (
	"time"

	"codenerd/internal/types"
)

const defaultSystemPrompt = "You are codeNERD. Respond in English. Be concise. When summarizing code, ground answers only in provided text. Do not claim to browse the filesystem or network; only use supplied content."

// LLMClient defines the interface for LLM providers.
// This is an alias to types.LLMClient for backward compatibility within the perception package.
type LLMClient = types.LLMClient

// ToolDefinition describes a tool that the LLM can invoke.
// Alias to types.ToolDefinition for package compatibility.
type ToolDefinition = types.ToolDefinition

// ToolCall represents a tool invocation requested by the LLM.
// Alias to types.ToolCall for package compatibility.
type ToolCall = types.ToolCall

// ToolResult represents the result of executing a tool.
type ToolResult struct {
	ToolUseID string `json:"tool_use_id"` // Matches ToolCall.ID
	Content   string `json:"content"`     // Result content
	IsError   bool   `json:"is_error"`    // Whether this is an error result
}

// LLMToolResponse contains both text response and tool calls from the LLM.
// Alias to types.LLMToolResponse for package compatibility.
type LLMToolResponse = types.LLMToolResponse

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
	MaxRetries       int
	RetryBackoffBase time.Duration
	RetryBackoffMax  time.Duration
	RateLimitDelay   time.Duration
	StreamingTimeout time.Duration
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
	APIKey          string
	BaseURL         string
	Model           string
	Timeout         time.Duration
	MaxOutputTokens int // Maximum tokens in response (default 8192)
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

// AnthropicMessage represents a message (supports both text and tool results).
type AnthropicMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"` // Can be string or []AnthropicContentBlock
}

// AnthropicContentBlock represents a content block in a message.
type AnthropicContentBlock struct {
	Type      string                 `json:"type"`                  // "text", "tool_use", "tool_result"
	Text      string                 `json:"text,omitempty"`        // For text blocks
	ID        string                 `json:"id,omitempty"`          // For tool_use blocks
	Name      string                 `json:"name,omitempty"`        // For tool_use blocks
	Input     map[string]interface{} `json:"input,omitempty"`       // For tool_use blocks
	ToolUseID string                 `json:"tool_use_id,omitempty"` // For tool_result blocks
	Content   string                 `json:"content,omitempty"`     // For tool_result blocks (result content)
	IsError   bool                   `json:"is_error,omitempty"`    // For tool_result blocks
}

// AnthropicTool represents a tool definition for Anthropic API.
type AnthropicTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

// AnthropicRequest represents the Anthropic API request.
type AnthropicRequest struct {
	Model       string             `json:"model"`
	MaxTokens   int                `json:"max_tokens"`
	System      string             `json:"system,omitempty"`
	Messages    []AnthropicMessage `json:"messages"`
	Tools       []AnthropicTool    `json:"tools,omitempty"`
	Temperature float64            `json:"temperature,omitempty"`
	Stream      bool               `json:"stream,omitempty"`
}

// AnthropicResponse represents the API response.
type AnthropicResponse struct {
	ID      string                  `json:"id"`
	Type    string                  `json:"type"`
	Role    string                  `json:"role"`
	Content []AnthropicContentBlock `json:"content"`
	Model   string                  `json:"model"`
	// StopReason: "end_turn" for normal completion, "tool_use" when tools are invoked
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
	Text         string                 `json:"text,omitempty"`
	FunctionCall *GeminiFunctionCall    `json:"functionCall,omitempty"`
}

// GeminiFunctionCall represents a function call from the model.
type GeminiFunctionCall struct {
	Name string                 `json:"name"`
	Args map[string]interface{} `json:"args"`
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
	Tools             []GeminiTool           `json:"tools,omitempty"`
}

// GeminiTool represents a tool declaration for function calling.
type GeminiTool struct {
	FunctionDeclarations []GeminiFunctionDeclaration `json:"functionDeclarations"`
}

// GeminiFunctionDeclaration represents a function declaration.
type GeminiFunctionDeclaration struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

// GeminiResponse represents the API response.
type GeminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []GeminiResponsePart `json:"parts"`
			Role  string               `json:"role"`
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

// GeminiResponsePart represents a part of the response content.
type GeminiResponsePart struct {
	Text         string              `json:"text,omitempty"`
	FunctionCall *GeminiFunctionCall `json:"functionCall,omitempty"`
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
