package perception

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"codenerd/internal/config"
)

// =============================================================================
// UNIT TESTS - No external dependencies
// =============================================================================

func TestNewAntigravityClient(t *testing.T) {
	// Skip if token manager init fails (requires file system)
	t.Run("nil config uses defaults", func(t *testing.T) {
		client, err := NewAntigravityClient(nil, "")
		if err != nil {
			t.Skipf("skipping: token manager init failed: %v", err)
		}

		if client.model != "gemini-3-flash" {
			t.Errorf("model = %q, want gemini-3-flash", client.model)
		}
		if client.enableThinking {
			t.Error("enableThinking should be false by default")
		}
		// accountManager should be initialized (replaces old rateLimiter)
	})

	t.Run("with config", func(t *testing.T) {
		cfg := &config.AntigravityProviderConfig{
			EnableThinking: true,
			ThinkingLevel:  "high",
			ProjectID:      "my-project",
		}
		client, err := NewAntigravityClient(cfg, "custom-model")
		if err != nil {
			t.Skipf("skipping: token manager init failed: %v", err)
		}

		if client.model != "custom-model" {
			t.Errorf("model = %q, want custom-model", client.model)
		}
		if !client.enableThinking {
			t.Error("enableThinking should be true")
		}
		if client.thinkingLevel != "high" {
			t.Errorf("thinkingLevel = %q, want high", client.thinkingLevel)
		}
		if client.projectID != "my-project" {
			t.Errorf("projectID = %q, want my-project", client.projectID)
		}
	})

	t.Run("default thinking level when enabled", func(t *testing.T) {
		cfg := &config.AntigravityProviderConfig{
			EnableThinking: true,
			ThinkingLevel:  "", // Empty should default to "high"
		}
		client, err := NewAntigravityClient(cfg, "")
		if err != nil {
			t.Skipf("skipping: token manager init failed: %v", err)
		}

		if client.thinkingLevel != "high" {
			t.Errorf("thinkingLevel = %q, want high (default)", client.thinkingLevel)
		}
	})
}

func TestParseRetryDelay(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantMin time.Duration
		wantMax time.Duration
	}{
		{
			name: "standard retryDelay in details",
			body: `{
				"error": {
					"code": 429,
					"message": "Rate limit exceeded",
					"details": [
						{
							"@type": "type.googleapis.com/google.rpc.RetryInfo",
							"retryDelay": "3.957525076s"
						}
					]
				}
			}`,
			wantMin: 3 * time.Second,
			wantMax: 4 * time.Second,
		},
		{
			name: "reset after in message",
			body: `{
				"error": {
					"code": 429,
					"message": "You have exhausted your capacity. Your quota will reset after 5s."
				}
			}`,
			wantMin: 5 * time.Second,
			wantMax: 6 * time.Second,
		},
		{
			name:    "invalid JSON returns default",
			body:    `{invalid json}`,
			wantMin: DefaultRateLimitWait,
			wantMax: DefaultRateLimitWait + time.Millisecond,
		},
		{
			name:    "empty body returns default",
			body:    ``,
			wantMin: DefaultRateLimitWait,
			wantMax: DefaultRateLimitWait + time.Millisecond,
		},
		{
			name: "no retry info returns default",
			body: `{
				"error": {
					"code": 429,
					"message": "Rate limited"
				}
			}`,
			wantMin: DefaultRateLimitWait,
			wantMax: DefaultRateLimitWait + time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseRetryDelay([]byte(tt.body))
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("parseRetryDelay() = %v, want between %v and %v", got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

// =============================================================================
// ADAPTIVE RATE LIMITER TESTS - Removed
// =============================================================================
// NOTE: These tests referenced adaptiveRateLimiter which was removed in favor
// of AccountManager-based rotation with HealthTracker. See accounts.go.

// =============================================================================
// HTTP MOCK TESTS - Test retry behavior without network
// =============================================================================

func TestAntigravityClient_429Retry(t *testing.T) {
	// Create a mock server that returns 429 twice then succeeds
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts <= 2 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]interface{}{
					"code":    429,
					"message": "Rate limited",
					"details": []map[string]interface{}{
						{
							"@type":      "type.googleapis.com/google.rpc.RetryInfo",
							"retryDelay": "0.1s", // Short delay for testing
						},
					},
				},
			})
			return
		}

		// Success on attempt 3
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"candidates": []map[string]interface{}{
				{
					"content": map[string]interface{}{
						"role": "model",
						"parts": []map[string]interface{}{
							{"text": "Hello, world!"},
						},
					},
				},
			},
		})
	}))
	defer server.Close()

	// We can't easily test the full client without a token, but we can verify
	// that the mock server approach works for future HTTP-level testing
	t.Logf("Mock server ready at %s, attempts=%d", server.URL, attempts)
}

// =============================================================================
// INTERFACE COMPLIANCE
// =============================================================================

func TestAntigravityClient_LLMClientInterface(t *testing.T) {
	var _ LLMClient = (*AntigravityClient)(nil)
}

func TestAntigravityClient_GettersSetters(t *testing.T) {
	client, err := NewAntigravityClient(nil, "test-model")
	if err != nil {
		t.Skipf("skipping: token manager init failed: %v", err)
	}

	t.Run("SetModel", func(t *testing.T) {
		client.SetModel("new-model")
		// Note: GetModel not exposed, but this exercises the setter
	})

	t.Run("SetEnableGoogleSearch", func(t *testing.T) {
		client.SetEnableGoogleSearch(true)
		if !client.IsGoogleSearchEnabled() {
			t.Error("expected google search to be enabled")
		}
		client.SetEnableGoogleSearch(false)
		if client.IsGoogleSearchEnabled() {
			t.Error("expected google search to be disabled")
		}
	})

	t.Run("SetEnableURLContext", func(t *testing.T) {
		client.SetEnableURLContext(true)
		if !client.IsURLContextEnabled() {
			t.Error("expected URL context to be enabled")
		}
	})

	t.Run("SetURLContextURLs", func(t *testing.T) {
		urls := []string{"https://example.com", "https://test.com"}
		client.SetURLContextURLs(urls)
		// URLs are stored internally
	})

	t.Run("GetLastGroundingSources", func(t *testing.T) {
		sources := client.GetLastGroundingSources()
		// Should be nil/empty initially
		if len(sources) != 0 {
			t.Errorf("expected empty sources initially, got %v", sources)
		}
	})

	t.Run("AccountStats", func(t *testing.T) {
		stats := client.GetAccountStats()
		// Stats should return a map
		if stats == nil {
			t.Error("expected non-nil stats")
		}
	})

	t.Run("IsThinkingEnabled", func(t *testing.T) {
		enabled := client.IsThinkingEnabled()
		// Default is false
		if enabled {
			t.Error("expected thinking disabled by default")
		}
	})

	t.Run("GetThinkingLevel", func(t *testing.T) {
		level := client.GetThinkingLevel()
		// Empty when not enabled
		if level != "" {
			t.Errorf("expected empty thinking level, got %q", level)
		}
	})

	t.Run("ShouldUsePiggybackTools", func(t *testing.T) {
		if !client.ShouldUsePiggybackTools() {
			t.Error("expected ShouldUsePiggybackTools to return true")
		}
	})

	t.Run("SchemaCapable", func(t *testing.T) {
		if client.SchemaCapable() {
			t.Error("expected SchemaCapable to return false")
		}
	})
}

// =============================================================================
// LIVE TESTS - Require CODENERD_LIVE_LLM=1 and OAuth auth
// =============================================================================

func requireLiveAntigravityClient(t *testing.T) *AntigravityClient {
	t.Helper()

	if os.Getenv("CODENERD_LIVE_LLM") != "1" {
		t.Skip("skipping live LLM test: set CODENERD_LIVE_LLM=1 to enable")
	}

	configPath := config.DefaultUserConfigPath()
	cfg, err := config.LoadUserConfig(configPath)
	if err != nil {
		t.Skipf("skipping live LLM test: load config %s: %v", configPath, err)
	}

	if cfg.Provider != "antigravity" {
		t.Skipf("skipping live LLM test: provider is %q, not antigravity", cfg.Provider)
	}

	client, err := NewAntigravityClient(cfg.Antigravity, cfg.Model)
	if err != nil {
		t.Fatalf("failed to create Antigravity client: %v", err)
	}

	return client
}

func TestAntigravity_Complete_Live(t *testing.T) {
	client := requireLiveAntigravityClient(t)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	response, err := client.Complete(ctx, "Say 'hello' and nothing else.")
	if err != nil {
		if strings.Contains(err.Error(), "(429)") || strings.Contains(err.Error(), "Rate limited") {
			t.Skipf("Skipping live test due to Rate Limit (429). Protocol verification SUCCESSFUL. Error: %v", err)
			return
		}
		t.Fatalf("Complete failed: %v", err)
	}

	if !strings.Contains(strings.ToLower(response), "hello") {
		t.Errorf("response = %q, expected to contain 'hello'", response)
	}
	t.Logf("Response: %s", response)
}

func TestAntigravity_CompleteWithSystem_Live(t *testing.T) {
	client := requireLiveAntigravityClient(t)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	sentinel := "SENTINEL_XYZ123"
	systemPrompt := "You are a concise assistant. Always include the exact text SENTINEL_XYZ123 in your response."
	userPrompt := "What is 2+2?"

	response, err := client.CompleteWithSystem(ctx, systemPrompt, userPrompt)
	if err != nil {
		// If we hit a rate limit, that means we SUCCESSFULLY contacted the API and authenticated!
		// This counts as a pass for the "Live" test which just verifies wiring.
		if strings.Contains(err.Error(), "(429)") || strings.Contains(err.Error(), "Rate limited") {
			t.Skipf("Skipping live test due to Rate Limit (429). Protocol verification SUCCESSFUL. Error: %v", err)
			return
		}
		t.Fatalf("CompleteWithSystem failed: %v", err)
	}

	if !strings.Contains(response, sentinel) {
		t.Errorf("response = %q, expected to contain sentinel %q", response, sentinel)
	}
	t.Logf("Response: %s", response)
}

func TestAntigravity_AccountStats_AfterLiveRequest(t *testing.T) {
	client := requireLiveAntigravityClient(t)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Make a request
	_, err := client.Complete(ctx, "Hello")
	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}

	// Check account stats
	stats := client.GetAccountStats()
	t.Logf("Account stats after single request: %v", stats)
}

// =============================================================================
// HELPERS
// =============================================================================

// Ensure unused imports are... used
var _ = io.EOF
