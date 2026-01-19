package perception

import (
	"bytes"
	"codenerd/internal/auth/antigravity"
	"codenerd/internal/config"
	"codenerd/internal/logging"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	AntigravityEndpointDaily = "https://daily-cloudcode-pa.sandbox.googleapis.com"
	GeminiCLIEndpointProd    = "https://cloudcode-pa.googleapis.com"
)

// AntigravityClient implements LLMClient for Google's internal Antigravity service.
type AntigravityClient struct {
	tokenManager   *antigravity.TokenManager
	model          string
	projectID      string
	enableThinking bool
	thinkingLevel  string
	httpClient     *http.Client
	mu             sync.Mutex
	lastRequest    time.Time

	// Grounding state
	enableGoogleSearch bool
	enableURLContext   bool
	urlContext         []string
	lastSources        []string
}

// NewAntigravityClient creates a new Antigravity client.
func NewAntigravityClient(cfg *config.AntigravityProviderConfig, model string) (*AntigravityClient, error) {
	tm, err := antigravity.NewTokenManager()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize token manager: %w", err)
	}

	client := &AntigravityClient{
		tokenManager: tm,
		model:        model,
		httpClient:   &http.Client{Timeout: 90 * time.Second},
	}

	if client.model == "" {
		client.model = "gemini-3.0-pro-exp"
	}

	if cfg != nil {
		client.enableThinking = cfg.EnableThinking
		client.thinkingLevel = cfg.ThinkingLevel
		if cfg.ProjectID != "" {
			client.projectID = cfg.ProjectID
		}
	}

	// Default thinking level
	if client.enableThinking && client.thinkingLevel == "" {
		client.thinkingLevel = "high"
	}

	return client, nil
}

// EnsureAuthenticated ensures we have a valid token and project ID.
func (c *AntigravityClient) EnsureAuthenticated(ctx context.Context) (string, error) {
	// 1. Get Token
	token, err := c.tokenManager.GetToken(ctx)
	if err != nil {
		// If authentication is required, we need to trigger the flow.
		// Since we are in a CLI/TUI environment, we should try to open the browser.
		logging.PerceptionWarn("[Antigravity] Authentication required. Opening browser...")

		result, err := antigravity.StartAuth()
		if err != nil {
			return "", fmt.Errorf("failed to start auth: %w", err)
		}

		fmt.Printf("\n[Antigravity] Opening auth URL: %s\n", result.AuthURL)
		if err := openBrowser(result.AuthURL); err != nil {
			fmt.Printf("[Antigravity] Failed to open browser: %v. Please copy/paste the URL above.\n", err)
		}

		code, err := antigravity.WaitForCallback(ctx, result.State)
		if err != nil {
			return "", fmt.Errorf("auth failed: %w", err)
		}

		token, err = c.tokenManager.ExchangeCode(ctx, code, result.Verifier)
		if err != nil {
			return "", fmt.Errorf("token exchange failed: %w", err)
		}
	}

	// 2. Resolve Project ID (if not set manually)
	if c.projectID == "" {
		if token.ProjectID != "" {
			c.projectID = token.ProjectID
		} else {
			resolver := antigravity.NewProjectResolver(token.AccessToken)
			pid, err := resolver.ResolveProjectID()
			if err != nil {
				logging.PerceptionWarn("[Antigravity] Failed to resolve project ID: %v", err)
			} else {
				c.projectID = pid
				// Update token with project ID and save
				token.ProjectID = pid
				c.tokenManager.SaveToken()
			}
		}
	}

	return token.AccessToken, nil
}

// Complete sends a prompt and returns the completion.
func (c *AntigravityClient) Complete(ctx context.Context, prompt string) (string, error) {
	return c.CompleteWithSystem(ctx, "", prompt)
}

// CompleteWithSystem sends a prompt with a system message.
func (c *AntigravityClient) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	accessToken, err := c.EnsureAuthenticated(ctx)
	if err != nil {
		return "", err
	}

	endpoint, headers := c.resolveEndpointAndHeaders()
	url := fmt.Sprintf("%s/models/%s:generateContent", endpoint, c.model)

	// Piggyback/JSON Mode Detection
	isPiggyback := strings.Contains(systemPrompt, "control_packet") ||
		strings.Contains(systemPrompt, "surface_response") ||
		strings.Contains(userPrompt, "PiggybackEnvelope") ||
		strings.Contains(userPrompt, "control_packet")

	// Grounding Config
	var tools []GeminiTool
	if c.enableGoogleSearch {
		tools = append(tools, GeminiTool{GoogleSearch: &GeminiGoogleSearch{}})
	}

	reqBody := GeminiRequest{
		Contents: []GeminiContent{
			{
				Role:  "user",
				Parts: []GeminiPart{{Text: userPrompt}},
			},
		},
		Tools: tools,
		GenerationConfig: GeminiGenerationConfig{
			Temperature:     1.0,
			MaxOutputTokens: 64000,
		},
	}

	if systemPrompt != "" {
		reqBody.SystemInstruction = &GeminiContent{
			Parts: []GeminiPart{{Text: systemPrompt}},
		}
	}

	// Thinking Config
	if c.enableThinking {
		reqBody.GenerationConfig.ThinkingConfig = &GeminiThinkingConfig{
			IncludeThoughts: true,
			ThinkingLevel:   c.thinkingLevel,
		}
	}

	// Schema / Piggyback Configuration
	if isPiggyback {
		reqBody.GenerationConfig.ResponseMimeType = "application/json"
		reqBody.GenerationConfig.ResponseSchema = BuildGeminiPiggybackEnvelopeSchema()
	} else if requiresJSONOutput(systemPrompt, userPrompt) {
		reqBody.GenerationConfig.ResponseMimeType = "application/json"
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	if c.projectID != "" {
		req.Header.Set("x-goog-user-project", c.projectID)
	}

	// Apply headers
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		bodyStr := string(body)

		// Retry logic for schema rejection (400 Bad Request)
		// Some models might not support the strict schema, retry without it
		if isPiggyback && resp.StatusCode == http.StatusBadRequest {
			lowerBody := strings.ToLower(bodyStr)
			if strings.Contains(lowerBody, "responsejsonschema") || strings.Contains(lowerBody, "responsemimetype") {
				logging.PerceptionWarn("[Antigravity] Schema rejected, retrying without strict schema...")
				reqBody.GenerationConfig.ResponseSchema = nil
				reqBody.GenerationConfig.ResponseMimeType = ""
				// Re-marshal and retry (simple retry for MVP)
				if jsonDataRetry, err := json.Marshal(reqBody); err == nil {
					if reqRetry, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonDataRetry)); err == nil {
						reqRetry.Header = req.Header // Copy headers
						if respRetry, err := c.httpClient.Do(reqRetry); err == nil {
							defer respRetry.Body.Close()
							if respRetry.StatusCode == http.StatusOK {
								resp = respRetry // Success on retry
								// Fall through to success handling
								goto ProcessResponse // Jump to processing
							}
						}
					}
				}
			}
		}

		return "", fmt.Errorf("antigravity request failed (%d): %s", resp.StatusCode, bodyStr)
	}

ProcessResponse:

	var geminiResp GeminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&geminiResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no candidates returned")
	}

	text := ""
	for _, part := range geminiResp.Candidates[0].Content.Parts {
		text += part.Text
	}

	// Capture grounding metadata
	if len(geminiResp.Candidates) > 0 {
		c.lastSources = nil // Reset
		if geminiResp.Candidates[0].GroundingMetadata != nil {
			for _, chunk := range geminiResp.Candidates[0].GroundingMetadata.GroundingChunks {
				if chunk.Web != nil && chunk.Web.URI != "" {
					c.lastSources = append(c.lastSources, chunk.Web.URI)
				}
			}
		}
	}

	return strings.TrimSpace(text), nil
}

// resolveEndpointAndHeaders determines the correct endpoint and headers based on the model.
func (c *AntigravityClient) resolveEndpointAndHeaders() (string, map[string]string) {
	// Headers
	baseHeaders := map[string]string{
		"User-Agent":        "antigravity/1.11.5 windows/amd64",
		"X-Goog-Api-Client": "google-cloud-sdk vscode_cloudshelleditor/0.1",
		"Client-Metadata":   "ideType=IDE_UNSPECIFIED,platform=PLATFORM_UNSPECIFIED,pluginType=GEMINI",
	}

	// Model-based routing
	// Antigravity models (e.g. claude/gemini-3-pro on antigravity) go to daily sandbox
	// Gemini-CLI models go to prod

	// Check if it's an Antigravity-specific model or Claude
	isAntigravity := strings.Contains(c.model, "antigravity") || strings.Contains(c.model, "claude")

	if isAntigravity {
		return AntigravityEndpointDaily, baseHeaders
	}

	// Fallback to Gemini CLI endpoint (prod)
	// Adjust headers for Gemini CLI simulation if needed
	geminiHeaders := map[string]string{
		"User-Agent":        "google-api-nodejs-client/9.15.1",
		"X-Goog-Api-Client": "gl-node/22.17.0",
		"Client-Metadata":   "ideType=IDE_UNSPECIFIED,platform=PLATFORM_UNSPECIFIED,pluginType=GEMINI",
	}
	return GeminiCLIEndpointProd, geminiHeaders
}

// openBrowser opens the specified URL in the default browser.
func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default: // "linux", "freebsd", "openbsd", "netbsd"
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}

// Placeholder implementations for interface compliance
func (c *AntigravityClient) CompleteWithTools(ctx context.Context, systemPrompt, userPrompt string, tools []ToolDefinition) (*LLMToolResponse, error) {
	// Native tool use is disabled in favor of Piggyback Protocol (prompt-based tool injection).
	// See ShouldUsePiggybackTools().
	// This method serves as a fallback for text-only completion if called directly.

	text, err := c.CompleteWithSystem(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, err
	}

	return &LLMToolResponse{
		Text: text,
	}, nil
}

func (c *AntigravityClient) CompleteWithSchema(ctx context.Context, systemPrompt, userPrompt, jsonSchema string) (string, error) {
	// Schema not fully supported in this MVP, fallback to plain text with prompt instruction
	return c.CompleteWithSystem(ctx, systemPrompt+"\nOutput JSON matching this schema:\n"+jsonSchema, userPrompt)
}

func (c *AntigravityClient) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, _ bool) (<-chan string, <-chan error) {
	// Streaming not implemented in this MVP
	errChan := make(chan error, 1)
	errChan <- fmt.Errorf("streaming not implemented for Antigravity yet")
	close(errChan)
	return nil, errChan
}

func (c *AntigravityClient) SetModel(model string) {
	c.model = model
}

func (c *AntigravityClient) SchemaCapable() bool {
	return false
}

func (c *AntigravityClient) ShouldUsePiggybackTools() bool {
	// "We bring our own tools" -> Use Piggyback Protocol to inject tools via prompt
	// and parse tool calls from JSON output.
	return true
}

func (c *AntigravityClient) IsGoogleSearchEnabled() bool {
	return c.enableGoogleSearch
}

func (c *AntigravityClient) IsURLContextEnabled() bool {
	return c.enableURLContext
}

func (c *AntigravityClient) SetEnableGoogleSearch(enable bool) {
	c.enableGoogleSearch = enable
}

func (c *AntigravityClient) SetEnableURLContext(enable bool) {
	c.enableURLContext = enable
}

func (c *AntigravityClient) SetURLContextURLs(urls []string) {
	c.urlContext = urls
}

func (c *AntigravityClient) GetLastGroundingSources() []string {
	return c.lastSources
}
func (c *AntigravityClient) GetLastThoughtSignature() string { return "" }
func (c *AntigravityClient) GetLastThoughtSummary() string   { return "" }
func (c *AntigravityClient) GetLastThinkingTokens() int      { return 0 }
func (c *AntigravityClient) IsThinkingEnabled() bool         { return c.enableThinking }
func (c *AntigravityClient) GetThinkingLevel() string        { return c.thinkingLevel }
