package perception

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"codenerd/internal/config"
)

func TestNewClaudeCodeCLIClient(t *testing.T) {
	tests := []struct {
		name        string
		cfg         *config.ClaudeCLIConfig
		wantModel   string
		wantTimeout time.Duration
	}{
		{
			name:        "nil config uses defaults",
			cfg:         nil,
			wantModel:   "sonnet",
			wantTimeout: 600 * time.Second,
		},
		{
			name: "custom model",
			cfg: &config.ClaudeCLIConfig{
				Model:   "opus",
				Timeout: 600,
			},
			wantModel:   "opus",
			wantTimeout: 600 * time.Second,
		},
		{
			name: "empty model uses default",
			cfg: &config.ClaudeCLIConfig{
				Model:   "",
				Timeout: 120,
			},
			wantModel:   "sonnet",
			wantTimeout: 120 * time.Second,
		},
		{
			name: "zero timeout uses default",
			cfg: &config.ClaudeCLIConfig{
				Model:   "haiku",
				Timeout: 0,
			},
			wantModel:   "haiku",
			wantTimeout: 600 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClaudeCodeCLIClient(tt.cfg)

			if client.GetModel() != tt.wantModel {
				t.Errorf("GetModel() = %q, want %q", client.GetModel(), tt.wantModel)
			}

			if client.GetTimeout() != tt.wantTimeout {
				t.Errorf("GetTimeout() = %v, want %v", client.GetTimeout(), tt.wantTimeout)
			}
		})
	}
}

func TestClaudeCodeCLIClient_SettersGetters(t *testing.T) {
	client := NewClaudeCodeCLIClient(nil)

	t.Run("SetModel and GetModel", func(t *testing.T) {
		client.SetModel("opus")
		if got := client.GetModel(); got != "opus" {
			t.Errorf("GetModel() after SetModel(opus) = %q, want opus", got)
		}
	})

	t.Run("SetTimeout and GetTimeout", func(t *testing.T) {
		client.SetTimeout(60 * time.Second)
		if got := client.GetTimeout(); got != 60*time.Second {
			t.Errorf("GetTimeout() after SetTimeout(60s) = %v, want 60s", got)
		}
	})
}

func TestClaudeCodeCLIClient_parseResponse(t *testing.T) {
	client := NewClaudeCodeCLIClient(nil)

	tests := []struct {
		name          string
		data          []byte
		want          string
		wantErr       bool
		wantRateLimit bool
	}{
		{
			name: "valid response with text content",
			data: []byte(`{
				"result": {
					"content": [
						{"type": "text", "text": "Hello, world!"}
					]
				}
			}`),
			want:    "Hello, world!",
			wantErr: false,
		},
		{
			name: "valid response with multiple text blocks",
			data: []byte(`{
				"result": {
					"content": [
						{"type": "text", "text": "First part. "},
						{"type": "text", "text": "Second part."}
					]
				}
			}`),
			want:    "First part. Second part.",
			wantErr: false,
		},
		{
			name: "response with mixed content types",
			data: []byte(`{
				"result": {
					"content": [
						{"type": "text", "text": "Important message"},
						{"type": "tool_use", "text": "ignored"}
					]
				}
			}`),
			want:    "Important message",
			wantErr: false,
		},
		{
			name:    "empty response",
			data:    []byte{},
			want:    "",
			wantErr: true,
		},
		{
			name:    "invalid JSON",
			data:    []byte(`{not valid json}`),
			want:    "",
			wantErr: true,
		},
		{
			name: "response with error",
			data: []byte(`{
				"error": {
					"type": "invalid_request",
					"message": "Something went wrong"
				}
			}`),
			want:    "",
			wantErr: true,
		},
		{
			name: "rate limit error in response",
			data: []byte(`{
				"error": {
					"type": "rate_limit_error",
					"message": "Rate limit exceeded"
				}
			}`),
			want:          "",
			wantErr:       true,
			wantRateLimit: true,
		},
		{
			name: "rate limited flag",
			data: []byte(`{
				"is_rate_limited": true,
				"result": {"content": []}
			}`),
			want:          "",
			wantErr:       true,
			wantRateLimit: true,
		},
		{
			name: "empty content array",
			data: []byte(`{
				"result": {
					"content": []
				}
			}`),
			want:    "",
			wantErr: true,
		},
		{
			name: "whitespace only text",
			data: []byte(`{
				"result": {
					"content": [
						{"type": "text", "text": "   \n\t  "}
					]
				}
			}`),
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := client.parseResponse(context.Background(), tt.data)

			if (err != nil) != tt.wantErr {
				t.Errorf("parseResponse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantRateLimit {
				var rateLimitErr *RateLimitError
				if !errors.As(err, &rateLimitErr) {
					t.Errorf("parseResponse() error = %v, want RateLimitError", err)
				}
			}

			if got != tt.want {
				t.Errorf("parseResponse() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRateLimitError(t *testing.T) {
	tests := []struct {
		name    string
		err     *RateLimitError
		wantMsg string
	}{
		{
			name: "with retry after",
			err: &RateLimitError{
				Provider:   "claude-cli",
				RetryAfter: 30 * time.Second,
			},
			wantMsg: "claude-cli rate limit exceeded, retry after 30s",
		},
		{
			name: "without retry after",
			err: &RateLimitError{
				Provider: "claude-cli",
			},
			wantMsg: "claude-cli rate limit exceeded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if got != tt.wantMsg {
				t.Errorf("Error() = %q, want %q", got, tt.wantMsg)
			}
		})
	}
}

func TestIsRateLimitError(t *testing.T) {
	tests := []struct {
		name   string
		errMsg string
		want   bool
	}{
		{
			name:   "rate limit message",
			errMsg: "Error: Rate limit exceeded",
			want:   true,
		},
		{
			name:   "rate_limit underscore format",
			errMsg: "rate_limit_error occurred",
			want:   true,
		},
		{
			name:   "too many requests",
			errMsg: "Too many requests, please slow down",
			want:   true,
		},
		{
			name:   "429 status code",
			errMsg: "HTTP 429: Request throttled",
			want:   true,
		},
		{
			name:   "case insensitive",
			errMsg: "RATE LIMIT ERROR",
			want:   true,
		},
		{
			name:   "unrelated error",
			errMsg: "Connection refused",
			want:   false,
		},
		{
			name:   "empty string",
			errMsg: "",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isRateLimitError(tt.errMsg)
			if got != tt.want {
				t.Errorf("isRateLimitError(%q) = %v, want %v", tt.errMsg, got, tt.want)
			}
		})
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		name   string
		s      string
		maxLen int
		want   string
	}{
		{
			name:   "short string unchanged",
			s:      "hello",
			maxLen: 10,
			want:   "hello",
		},
		{
			name:   "exact length unchanged",
			s:      "hello",
			maxLen: 5,
			want:   "hello",
		},
		{
			name:   "truncated with ellipsis",
			s:      "hello world",
			maxLen: 8,
			want:   "hello...",
		},
		{
			name:   "empty string",
			s:      "",
			maxLen: 10,
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateString(tt.s, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncateString(%q, %d) = %q, want %q", tt.s, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestClaudeCodeCLIClient_parseResponse_Usage(t *testing.T) {
	client := NewClaudeCodeCLIClient(nil)
	ctx := context.Background()
	// Realistic payload as returned by `claude -p --output-format json`.
	// Includes usage accounting plus extra fields the CLI emits (total_cost_usd, modelUsage)
	// that the decoder should ignore while still parsing usage.
	payload := []byte(`{
		"type": "result",
		"subtype": "success",
		"is_error": false,
		"result": "ok",
		"usage": {
			"input_tokens": 2,
			"output_tokens": 5,
			"cache_creation_input_tokens": 67857,
			"cache_read_input_tokens": 0
		},
		"total_cost_usd": 0.678705,
		"modelUsage": {
			"claude-sonnet-4-5-20250929": {
				"inputTokens": 2,
				"outputTokens": 5,
				"cacheCreationInputTokens": 67857,
				"cacheReadInputTokens": 0,
				"costUSD": 0.678705
			}
		}
	}`)
	// parseResponse should succeed and return the text result, while internally
	// decoding usage. It must not error on unknown fields like total_cost_usd.
	text, err := client.parseResponse(ctx, payload)
	if err != nil {
		t.Fatalf("parseResponse with usage payload failed: %v", err)
	}
	if text != "ok" {
		t.Errorf("parseResponse text = %q, want %q", text, "ok")
	}
	// Verify the Usage struct itself is decoded correctly (direct unmarshal).
	var resp claudeCLIResponse
	if err := json.Unmarshal(payload, &resp); err != nil {
		t.Fatalf("json.Unmarshal claudeCLIResponse failed: %v", err)
	}
	if resp.Usage == nil {
		t.Fatal("Usage is nil after unmarshal, want non-nil")
	}
	if resp.Usage.InputTokens != 2 {
		t.Errorf("Usage.InputTokens = %d, want 2", resp.Usage.InputTokens)
	}
	if resp.Usage.OutputTokens != 5 {
		t.Errorf("Usage.OutputTokens = %d, want 5", resp.Usage.OutputTokens)
	}
	if resp.Usage.CacheCreationInputTokens != 67857 {
		t.Errorf("Usage.CacheCreationInputTokens = %d, want 67857", resp.Usage.CacheCreationInputTokens)
	}
	if resp.Usage.CacheReadInputTokens != 0 {
		t.Errorf("Usage.CacheReadInputTokens = %d, want 0", resp.Usage.CacheReadInputTokens)
	}
	// Object-result format should also carry usage.
	payload2 := []byte(`{
		"type": "result",
		"subtype": "success",
		"is_error": false,
		"result": {"content": [{"type": "text", "text": "Hello from object"}]},
		"usage": {"input_tokens": 10, "output_tokens": 20, "cache_creation_input_tokens": 100, "cache_read_input_tokens": 5}
	}`)
	text2, err := client.parseResponse(ctx, payload2)
	if err != nil {
		t.Fatalf("parseResponse object result with usage failed: %v", err)
	}
	if text2 != "Hello from object" {
		t.Errorf("parseResponse text2 = %q, want %q", text2, "Hello from object")
	}
	var resp2 claudeCLIResponse
	if err := json.Unmarshal(payload2, &resp2); err != nil {
		t.Fatalf("json.Unmarshal second payload failed: %v", err)
	}
	if resp2.Usage == nil || resp2.Usage.InputTokens != 10 || resp2.Usage.OutputTokens != 20 {
		t.Errorf("second payload Usage = %+v, want InputTokens=10 OutputTokens=20", resp2.Usage)
	}
	// Payload without usage should still parse (Usage nil allowed).
	payload3 := []byte(`{"type":"result","subtype":"success","is_error":false,"result":"no usage here"}`)
	text3, err := client.parseResponse(ctx, payload3)
	if err != nil {
		t.Fatalf("parseResponse without usage failed: %v", err)
	}
	if text3 != "no usage here" {
		t.Errorf("parseResponse without usage text = %q, want %q", text3, "no usage here")
	}
	var resp3 claudeCLIResponse
	if err := json.Unmarshal(payload3, &resp3); err != nil {
		t.Fatalf("json.Unmarshal third payload failed: %v", err)
	}
	if resp3.Usage != nil {
		t.Errorf("Usage should be nil when absent, got %+v", resp3.Usage)
	}
}


// TestClaudeCodeCLIClient_LLMClientInterface verifies the client implements LLMClient.
func TestClaudeCodeCLIClient_LLMClientInterface(t *testing.T) {
	var _ LLMClient = (*ClaudeCodeCLIClient)(nil)
}
