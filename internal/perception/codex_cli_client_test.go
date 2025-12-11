package perception

import (
	"errors"
	"testing"
	"time"

	"codenerd/internal/config"
)

func TestNewCodexCLIClient(t *testing.T) {
	tests := []struct {
		name        string
		cfg         *config.CodexCLIConfig
		wantModel   string
		wantSandbox string
		wantTimeout time.Duration
	}{
		{
			name:        "nil config uses defaults",
			cfg:         nil,
			wantModel:   "gpt-5.1-codex-max",
			wantSandbox: "read-only",
			wantTimeout: 300 * time.Second,
		},
		{
			name: "custom model",
			cfg: &config.CodexCLIConfig{
				Model:   "o4-mini",
				Sandbox: "read-only",
				Timeout: 600,
			},
			wantModel:   "o4-mini",
			wantSandbox: "read-only",
			wantTimeout: 600 * time.Second,
		},
		{
			name: "custom sandbox",
			cfg: &config.CodexCLIConfig{
				Model:   "gpt-5",
				Sandbox: "workspace-write",
				Timeout: 300,
			},
			wantModel:   "gpt-5",
			wantSandbox: "workspace-write",
			wantTimeout: 300 * time.Second,
		},
		{
			name: "empty model uses default",
			cfg: &config.CodexCLIConfig{
				Model:   "",
				Sandbox: "read-only",
				Timeout: 120,
			},
			wantModel:   "gpt-5.1-codex-max",
			wantSandbox: "read-only",
			wantTimeout: 120 * time.Second,
		},
		{
			name: "zero timeout uses default",
			cfg: &config.CodexCLIConfig{
				Model:   "o3",
				Sandbox: "read-only",
				Timeout: 0,
			},
			wantModel:   "o3",
			wantSandbox: "read-only",
			wantTimeout: 300 * time.Second,
		},
		{
			name: "fallback model configured",
			cfg: &config.CodexCLIConfig{
				Model:         "gpt-5",
				FallbackModel: "o4-mini",
				Timeout:       300,
			},
			wantModel:   "gpt-5",
			wantSandbox: "read-only",
			wantTimeout: 300 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewCodexCLIClient(tt.cfg)

			if client.GetModel() != tt.wantModel {
				t.Errorf("GetModel() = %q, want %q", client.GetModel(), tt.wantModel)
			}

			if client.GetSandbox() != tt.wantSandbox {
				t.Errorf("GetSandbox() = %q, want %q", client.GetSandbox(), tt.wantSandbox)
			}

			if client.GetTimeout() != tt.wantTimeout {
				t.Errorf("GetTimeout() = %v, want %v", client.GetTimeout(), tt.wantTimeout)
			}
		})
	}
}

func TestCodexCLIClient_SettersGetters(t *testing.T) {
	client := NewCodexCLIClient(nil)

	t.Run("SetModel and GetModel", func(t *testing.T) {
		client.SetModel("o4-mini")
		if got := client.GetModel(); got != "o4-mini" {
			t.Errorf("GetModel() after SetModel(o4-mini) = %q, want o4-mini", got)
		}
	})

	t.Run("SetFallbackModel and GetFallbackModel", func(t *testing.T) {
		client.SetFallbackModel("o3-mini")
		if got := client.GetFallbackModel(); got != "o3-mini" {
			t.Errorf("GetFallbackModel() after SetFallbackModel(o3-mini) = %q, want o3-mini", got)
		}
	})

	t.Run("SetSandbox and GetSandbox", func(t *testing.T) {
		client.SetSandbox("workspace-write")
		if got := client.GetSandbox(); got != "workspace-write" {
			t.Errorf("GetSandbox() after SetSandbox(workspace-write) = %q, want workspace-write", got)
		}
	})

	t.Run("SetTimeout and GetTimeout", func(t *testing.T) {
		client.SetTimeout(60 * time.Second)
		if got := client.GetTimeout(); got != 60*time.Second {
			t.Errorf("GetTimeout() after SetTimeout(60s) = %v, want 60s", got)
		}
	})

	t.Run("SetStreaming and GetStreaming", func(t *testing.T) {
		client.SetStreaming(true)
		if got := client.GetStreaming(); got != true {
			t.Errorf("GetStreaming() after SetStreaming(true) = %v, want true", got)
		}
	})
}

func TestCodexCLIClient_buildPrompt(t *testing.T) {
	client := NewCodexCLIClient(nil)

	tests := []struct {
		name         string
		systemPrompt string
		userPrompt   string
		want         string
	}{
		{
			name:         "no system prompt",
			systemPrompt: "",
			userPrompt:   "Hello",
			want:         "Hello",
		},
		{
			name:         "whitespace only system prompt",
			systemPrompt: "   ",
			userPrompt:   "Hello",
			want:         "Hello",
		},
		{
			name:         "with system prompt",
			systemPrompt: "You are helpful.",
			userPrompt:   "Hello",
			want:         "<system_instructions>\nYou are helpful.\n</system_instructions>\n\nHello",
		},
		{
			name:         "system prompt with newlines",
			systemPrompt: "Line 1\nLine 2",
			userPrompt:   "Question",
			want:         "<system_instructions>\nLine 1\nLine 2\n</system_instructions>\n\nQuestion",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := client.buildPrompt(tt.systemPrompt, tt.userPrompt)
			if got != tt.want {
				t.Errorf("buildPrompt(%q, %q) = %q, want %q", tt.systemPrompt, tt.userPrompt, got, tt.want)
			}
		})
	}
}

func TestCodexCLIClient_parseNDJSONResponse(t *testing.T) {
	client := NewCodexCLIClient(nil)

	tests := []struct {
		name          string
		data          []byte
		want          string
		wantErr       bool
		wantRateLimit bool
	}{
		{
			name: "valid response with message_stop",
			data: []byte(`{"type":"message_stop","message":{"content":[{"type":"text","text":"Hello, world!"}]}}`),
			want: "Hello, world!",
		},
		{
			name: "response with content_block_delta only",
			data: []byte(`{"type":"content_block_delta","delta":{"type":"text_delta","text":"First"}}
{"type":"content_block_delta","delta":{"type":"text_delta","text":" part"}}
{"type":"message_stop"}`),
			want: "First part",
		},
		{
			name: "response with both delta and final message",
			data: []byte(`{"type":"content_block_delta","delta":{"type":"text_delta","text":"streaming"}}
{"type":"message_stop","message":{"content":[{"type":"text","text":"Final message"}]}}`),
			want: "Final message",
		},
		{
			name: "multiple text blocks in final message",
			data: []byte(`{"type":"message_stop","message":{"content":[{"type":"text","text":"Part 1"},{"type":"text","text":" Part 2"}]}}`),
			want: "Part 1 Part 2",
		},
		{
			name:    "empty response",
			data:    []byte{},
			wantErr: true,
		},
		{
			name: "malformed JSON line skipped",
			data: []byte(`{not valid}
{"type":"content_block_delta","delta":{"type":"text_delta","text":"Valid text"}}
{"type":"message_stop"}`),
			want: "Valid text",
		},
		{
			name: "error event",
			data: []byte(`{"error":{"type":"invalid_request","message":"Bad request"}}`),
			wantErr: true,
		},
		{
			name:          "rate limit error in event",
			data:          []byte(`{"error":{"type":"rate_limit_error","message":"Rate limit exceeded"}}`),
			wantErr:       true,
			wantRateLimit: true,
		},
		{
			name:          "rate limit in message",
			data:          []byte(`{"error":{"type":"error","message":"You have hit the rate limit"}}`),
			wantErr:       true,
			wantRateLimit: true,
		},
		{
			name:    "no text content",
			data:    []byte(`{"type":"message_stop","message":{"content":[]}}`),
			wantErr: true,
		},
		{
			name: "only non-text content",
			data: []byte(`{"type":"message_stop","message":{"content":[{"type":"tool_use","text":"ignored"}]}}`),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := client.parseNDJSONResponse(tt.data)

			if (err != nil) != tt.wantErr {
				t.Errorf("parseNDJSONResponse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantRateLimit {
				var rateLimitErr *RateLimitError
				if !errors.As(err, &rateLimitErr) {
					t.Errorf("parseNDJSONResponse() error = %v, want RateLimitError", err)
				}
			}

			if got != tt.want {
				t.Errorf("parseNDJSONResponse() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCodexCLIClient_FallbackModelConfig(t *testing.T) {
	cfg := &config.CodexCLIConfig{
		Model:         "gpt-5",
		FallbackModel: "o4-mini",
		Sandbox:       "read-only",
		Timeout:       300,
	}

	client := NewCodexCLIClient(cfg)

	if got := client.GetModel(); got != "gpt-5" {
		t.Errorf("GetModel() = %q, want gpt-5", got)
	}

	if got := client.GetFallbackModel(); got != "o4-mini" {
		t.Errorf("GetFallbackModel() = %q, want o4-mini", got)
	}
}

func TestCodexCLIClient_StreamingConfig(t *testing.T) {
	cfg := &config.CodexCLIConfig{
		Model:     "gpt-5",
		Streaming: true,
	}

	client := NewCodexCLIClient(cfg)

	if got := client.GetStreaming(); got != true {
		t.Errorf("GetStreaming() = %v, want true", got)
	}
}

// TestCodexCLIClient_LLMClientInterface verifies the client implements LLMClient.
func TestCodexCLIClient_LLMClientInterface(t *testing.T) {
	var _ LLMClient = (*CodexCLIClient)(nil)
}

// TestCodexExecutionOptions verifies the execution options struct.
func TestCodexExecutionOptions(t *testing.T) {
	opts := &CodexExecutionOptions{
		SystemPrompt: "You are a helpful assistant.",
		Streaming:    true,
	}

	if opts.SystemPrompt != "You are a helpful assistant." {
		t.Errorf("SystemPrompt = %q, want 'You are a helpful assistant.'", opts.SystemPrompt)
	}

	if opts.Streaming != true {
		t.Errorf("Streaming = %v, want true", opts.Streaming)
	}
}

// TestCodexCLIClient_RateLimitDetection tests rate limit error detection in NDJSON.
func TestCodexCLIClient_RateLimitDetection(t *testing.T) {
	client := NewCodexCLIClient(nil)

	tests := []struct {
		name          string
		data          []byte
		wantRateLimit bool
	}{
		{
			name:          "rate_limit_error type",
			data:          []byte(`{"error":{"type":"rate_limit_error","message":"Too many requests"}}`),
			wantRateLimit: true,
		},
		{
			name:          "rate limit in message text",
			data:          []byte(`{"error":{"type":"api_error","message":"Rate limit exceeded, please try again later"}}`),
			wantRateLimit: true,
		},
		{
			name:          "rate_limit underscore in type",
			data:          []byte(`{"error":{"type":"rate_limit","message":"Throttled"}}`),
			wantRateLimit: true,
		},
		{
			name:          "different error type",
			data:          []byte(`{"error":{"type":"invalid_request","message":"Bad parameters"}}`),
			wantRateLimit: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.parseNDJSONResponse(tt.data)

			if err == nil {
				t.Fatal("parseNDJSONResponse() expected error, got nil")
			}

			var rateLimitErr *RateLimitError
			isRateLimit := errors.As(err, &rateLimitErr)

			if isRateLimit != tt.wantRateLimit {
				t.Errorf("isRateLimit = %v, want %v (error: %v)", isRateLimit, tt.wantRateLimit, err)
			}
		})
	}
}

// TestCodexCLIClient_ModelOptions tests various model configurations.
func TestCodexCLIClient_ModelOptions(t *testing.T) {
	models := []string{
		"gpt-5.1-codex-max",  // Recommended
		"gpt-5.1-codex-mini", // Cost-effective
		"gpt-5.1",            // General
		"gpt-5-codex",        // Legacy agentic
		"gpt-5",              // Legacy general
		"o4-mini",            // Legacy fast reasoning
		"o3",                 // Legacy advanced reasoning
		"codex-mini-latest",  // Low-latency
	}

	for _, model := range models {
		t.Run(model, func(t *testing.T) {
			cfg := &config.CodexCLIConfig{
				Model: model,
			}
			client := NewCodexCLIClient(cfg)

			if got := client.GetModel(); got != model {
				t.Errorf("GetModel() = %q, want %q", got, model)
			}
		})
	}
}

// TestCodexCLIClient_SandboxOptions tests sandbox mode configurations.
func TestCodexCLIClient_SandboxOptions(t *testing.T) {
	sandboxModes := []string{"read-only", "workspace-write"}

	for _, sandbox := range sandboxModes {
		t.Run(sandbox, func(t *testing.T) {
			cfg := &config.CodexCLIConfig{
				Sandbox: sandbox,
			}
			client := NewCodexCLIClient(cfg)

			if got := client.GetSandbox(); got != sandbox {
				t.Errorf("GetSandbox() = %q, want %q", got, sandbox)
			}
		})
	}
}
