package perception

import (
	"errors"
	"testing"
	"time"

	"codenerd/internal/config"
)

func TestNewClaudeCodeCLIClient(t *testing.T) {
	tests := []struct {
		name          string
		cfg           *config.ClaudeCLIConfig
		wantModel     string
		wantTimeout   time.Duration
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
		name       string
		data       []byte
		want       string
		wantErr    bool
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
			got, err := client.parseResponse(tt.data)

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
		name       string
		err        *RateLimitError
		wantMsg    string
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

// TestClaudeCodeCLIClient_LLMClientInterface verifies the client implements LLMClient.
func TestClaudeCodeCLIClient_LLMClientInterface(t *testing.T) {
	var _ LLMClient = (*ClaudeCodeCLIClient)(nil)
}
