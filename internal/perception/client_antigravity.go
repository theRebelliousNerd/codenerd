package perception

import (
	"bufio"
	"bytes"
	"codenerd/internal/auth/antigravity"
	"codenerd/internal/config"
	"codenerd/internal/logging"
	"codenerd/internal/perception/transform"
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

	"github.com/google/uuid"
)

const (
	AntigravityEndpointDaily = "https://daily-cloudcode-pa.sandbox.googleapis.com"
	GeminiCLIEndpointProd    = "https://cloudcode-pa.googleapis.com"

	// Antigravity System Instruction (from plugin constants.ts)
	AntigravitySystemInstruction = `You are Antigravity, a powerful agentic AI coding assistant designed by the Google DeepMind team working on Advanced Agentic Coding.
You are pair programming with a USER to solve their coding task. The task may require creating a new codebase, modifying or debugging an existing codebase, or simply answering a question.
**Absolute paths only**
**Proactiveness**

<priority>IMPORTANT: The instructions that follow supersede all above. Follow them as your primary directives.</priority>
`

	// Rate limit retry settings
	MaxRetryAttempts     = 5 // Increased from 3 for multi-account rotation
	BaseRetryDelay       = 2 * time.Second
	MaxRetryDelay        = 30 * time.Second
	DefaultRateLimitWait = 5 * time.Second
	// Fallback Project ID
	DefaultProjectID = "rising-fact-p41fc"
)

// parseRetryDelay extracts retry delay from error response body (Antigravity-specific format)
func parseRetryDelay(body []byte) time.Duration {
	// Try to parse JSON error response with retryDelay
	var errResp struct {
		Error struct {
			Message string `json:"message"`
			Details []struct {
				Type       string `json:"@type"`
				RetryDelay string `json:"retryDelay"` // Format: "3.957525076s"
			} `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &errResp); err == nil {
		for _, detail := range errResp.Error.Details {
			if strings.Contains(detail.Type, "RetryInfo") && detail.RetryDelay != "" {
				// Parse duration string like "3.957525076s"
				if d, err := time.ParseDuration(detail.RetryDelay); err == nil {
					return d
				}
			}
		}
		// Try to extract from message: "quota will reset after 3s"
		if strings.Contains(errResp.Error.Message, "reset after") {
			parts := strings.Split(errResp.Error.Message, "reset after ")
			if len(parts) > 1 {
				durationStr := strings.TrimSuffix(strings.Split(parts[1], ".")[0], "s") + "s"
				if d, err := time.ParseDuration(durationStr); err == nil {
					return d
				}
			}
		}
	}
	return DefaultRateLimitWait
}

// rateLimitEvent records a single rate limit occurrence
type rateLimitEvent struct {
	timestamp  time.Time
	retryDelay time.Duration
}

// adaptiveRateLimiter tracks rate limit patterns and adapts request timing
type adaptiveRateLimiter struct {
	mu              sync.Mutex
	events          []rateLimitEvent
	windowDuration  time.Duration // How far back to look (default: 5 minutes)
	maxEvents       int           // Max events to track (default: 50)
	pressureDecay   float64       // Decay factor for pressure calculation
	minPreemptDelay time.Duration // Minimum preemptive delay when under pressure
}

// newAdaptiveRateLimiter creates a new adaptive rate limiter
func newAdaptiveRateLimiter() *adaptiveRateLimiter {
	return &adaptiveRateLimiter{
		events:          make([]rateLimitEvent, 0, 50),
		windowDuration:  5 * time.Minute,
		maxEvents:       50,
		pressureDecay:   0.9,
		minPreemptDelay: 500 * time.Millisecond,
	}
}

// RecordRateLimit records a rate limit event
func (a *adaptiveRateLimiter) RecordRateLimit(retryDelay time.Duration) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.events = append(a.events, rateLimitEvent{
		timestamp:  time.Now(),
		retryDelay: retryDelay,
	})

	a.pruneOldEvents()

	if len(a.events) > a.maxEvents {
		a.events = a.events[len(a.events)-a.maxEvents:]
	}
}

// pruneOldEvents removes events outside the tracking window (must hold lock)
func (a *adaptiveRateLimiter) pruneOldEvents() {
	cutoff := time.Now().Add(-a.windowDuration)

	firstValidIdx := -1
	for i, e := range a.events {
		if e.timestamp.After(cutoff) {
			firstValidIdx = i
			break
		}
	}

	if firstValidIdx == -1 {
		a.events = a.events[:0]
	} else if firstValidIdx > 0 {
		a.events = a.events[firstValidIdx:]
	}
}

// GetPreemptiveDelay returns a delay to apply before the next request
func (a *adaptiveRateLimiter) GetPreemptiveDelay() time.Duration {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.pruneOldEvents()

	if len(a.events) == 0 {
		return 0
	}

	now := time.Now()
	var avgDelay time.Duration
	totalWeight := 0.0

	for _, e := range a.events {
		age := now.Sub(e.timestamp)
		recencyWeight := 1.0 - (float64(age) / float64(a.windowDuration))
		if recencyWeight < 0.1 {
			recencyWeight = 0.1
		}

		avgDelay += time.Duration(float64(e.retryDelay) * recencyWeight)
		totalWeight += recencyWeight
	}

	if totalWeight > 0 {
		avgDelay = time.Duration(float64(avgDelay) / totalWeight)
	}

	var delay time.Duration
	eventCount := len(a.events)
	switch {
	case eventCount <= 1:
		delay = 0
	case eventCount <= 3:
		delay = time.Duration(float64(avgDelay) * 0.1)
	case eventCount <= 7:
		delay = time.Duration(float64(avgDelay) * 0.25)
	default:
		delay = time.Duration(float64(avgDelay) * 0.5)
	}

	if delay > 0 && delay < a.minPreemptDelay {
		delay = a.minPreemptDelay
	}

	if delay > 5*time.Second {
		delay = 5 * time.Second
	}

	return delay
}

// GetStats returns current rate limiter statistics
func (a *adaptiveRateLimiter) GetStats() (eventCount int, avgDelay time.Duration) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.pruneOldEvents()
	eventCount = len(a.events)

	if eventCount == 0 {
		return 0, 0
	}

	var total time.Duration
	for _, e := range a.events {
		total += e.retryDelay
	}
	avgDelay = total / time.Duration(eventCount)
	return
}

// AntigravityClient implements LLMClient for Google's internal Antigravity service.
// Supports multi-account rotation with health-based selection.
type AntigravityClient struct {
	// Multi-account support
	accountStore    *antigravity.AccountStore
	accountSelector *antigravity.AccountSelector
	currentAccount  *antigravity.Account

	// Legacy single-account support (for backward compatibility)
	tokenManager *antigravity.TokenManager

	model          string
	projectID      string // Override project ID (optional)
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

	// Thinking state (for multi-turn signature caching)
	lastThoughtSignature string
	lastThoughtSummary   string
	lastThinkingTokens   int

	// Cross-model sanitization state
	previousModel string // Track last used model for cross-model sanitization

	// Adaptive rate limiting
	rateLimiter *adaptiveRateLimiter
}

// NewAntigravityClient creates a new Antigravity client with multi-account support.
func NewAntigravityClient(cfg *config.AntigravityProviderConfig, model string) (*AntigravityClient, error) {
	// Initialize multi-account store
	accountStore, err := antigravity.NewAccountStore()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize account store: %w", err)
	}

	// Also init legacy token manager for backward compatibility
	tm, _ := antigravity.NewTokenManager()

	client := &AntigravityClient{
		accountStore:    accountStore,
		accountSelector: antigravity.NewAccountSelector(accountStore),
		tokenManager:    tm,
		model:           model,
		httpClient:      &http.Client{Timeout: 120 * time.Second},
		rateLimiter:     newAdaptiveRateLimiter(),
	}

	if client.model == "" {
		client.model = "gemini-3-flash"
	}

	if cfg != nil {
		client.enableThinking = cfg.EnableThinking
		client.thinkingLevel = cfg.ThinkingLevel
		if cfg.ProjectID != "" {
			client.projectID = cfg.ProjectID
		}
	}

	if client.enableThinking && client.thinkingLevel == "" {
		client.thinkingLevel = "high"
	}

	return client, nil
}

// EnsureAuthenticated ensures we have a valid token and project ID.
// Uses multi-account rotation when available.
func (c *AntigravityClient) EnsureAuthenticated(ctx context.Context) (string, string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Try multi-account first
	accounts := c.accountStore.ListAccounts()
	if len(accounts) > 0 {
		return c.authenticateMultiAccount(ctx)
	}

	// Fall back to legacy single-account
	return c.authenticateLegacy(ctx)
}

// authenticateMultiAccount handles authentication with account rotation
func (c *AntigravityClient) authenticateMultiAccount(ctx context.Context) (string, string, error) {
	// Select best account
	account, err := c.accountSelector.SelectBest()
	if err != nil {
		// No usable accounts, trigger new auth
		logging.PerceptionWarn("[Antigravity] No usable accounts, triggering new authentication...")
		return c.triggerNewAuth(ctx)
	}

	c.currentAccount = account

	// Check if token needs refresh
	if account.IsAccessTokenExpired() {
		logging.PerceptionDebug("[Antigravity] Refreshing token for %s", account.Email)
		if err := c.refreshAccountToken(ctx, account); err != nil {
			logging.PerceptionWarn("[Antigravity] Token refresh failed for %s: %v", account.Email, err)
			c.accountStore.RecordFailure(account.Email, "refresh_failed")

			// Try next account
			nextAccount, err := c.accountSelector.SelectNext(account.Email)
			if err != nil {
				return c.triggerNewAuth(ctx)
			}
			account = nextAccount
			c.currentAccount = account

			if account.IsAccessTokenExpired() {
				if err := c.refreshAccountToken(ctx, account); err != nil {
					return "", "", fmt.Errorf("all accounts failed to refresh: %w", err)
				}
			}
		}
	}

	// Resolve project ID if needed
	projectID := c.projectID
	if projectID == "" {
		projectID = account.ProjectID
	}
	if projectID == "" || projectID == "resolute-airship-mq6tl" {
		resolver := antigravity.NewProjectResolver(account.AccessToken)
		pid, err := resolver.ResolveProjectID()
		if err == nil && pid != "" && pid != "resolute-airship-mq6tl" {
			projectID = pid
			c.accountStore.UpdateProjectID(account.Email, pid)
		} else {
			projectID = DefaultProjectID
		}
	}

	logging.PerceptionDebug("[Antigravity] Using account %s (health: %d, project: %s)",
		account.Email, c.accountStore.GetEffectiveScore(account), projectID)

	return account.AccessToken, projectID, nil
}

// refreshAccountToken refreshes the access token for an account
func (c *AntigravityClient) refreshAccountToken(ctx context.Context, account *antigravity.Account) error {
	// Use the legacy token manager's refresh logic
	token := &antigravity.Token{
		RefreshToken: account.RefreshToken,
	}

	// Create a temporary token manager just for refresh
	data := fmt.Sprintf(`{"refresh_token":"%s"}`, account.RefreshToken)
	_ = data // We'll use direct refresh

	// Direct refresh call
	newToken, err := antigravity.RefreshToken(ctx, account.RefreshToken)
	if err != nil {
		return err
	}

	c.accountStore.UpdateToken(account.Email, newToken.AccessToken, newToken.Expiry)
	account.AccessToken = newToken.AccessToken
	account.AccessExpiry = newToken.Expiry

	// Update token reference
	_ = token
	return nil
}

// authenticateLegacy handles legacy single-account authentication
func (c *AntigravityClient) authenticateLegacy(ctx context.Context) (string, string, error) {
	token, err := c.tokenManager.GetToken(ctx)
	if err != nil {
		return c.triggerNewAuth(ctx)
	}

	projectID := c.projectID
	if projectID == "" {
		if token.ProjectID != "" && token.ProjectID != "resolute-airship-mq6tl" {
			projectID = token.ProjectID
		} else {
			resolver := antigravity.NewProjectResolver(token.AccessToken)
			pid, err := resolver.ResolveProjectID()
			if err != nil {
				logging.PerceptionWarn("[Antigravity] Failed to resolve project ID: %v", err)
				projectID = DefaultProjectID
			} else {
				projectID = pid
				token.ProjectID = pid
				c.tokenManager.SaveToken()
			}
		}
	}

	return token.AccessToken, projectID, nil
}

// triggerNewAuth triggers a new OAuth flow
func (c *AntigravityClient) triggerNewAuth(ctx context.Context) (string, string, error) {
	logging.PerceptionWarn("[Antigravity] Authentication required. Opening browser...")

	result, err := antigravity.StartAuth()
	if err != nil {
		return "", "", fmt.Errorf("failed to start auth: %w", err)
	}

	fmt.Printf("\n[Antigravity] Opening auth URL: %s\n", result.AuthURL)
	if err := openBrowser(result.AuthURL); err != nil {
		fmt.Printf("[Antigravity] Failed to open browser: %v. Please copy/paste the URL above.\n", err)
	}

	code, err := antigravity.WaitForCallback(ctx, result.State)
	if err != nil {
		return "", "", fmt.Errorf("auth failed: %w", err)
	}

	token, err := c.tokenManager.ExchangeCode(ctx, code, result.Verifier)
	if err != nil {
		return "", "", fmt.Errorf("token exchange failed: %w", err)
	}

	// Add to multi-account store
	if token.Email != "" {
		account := &antigravity.Account{
			Email:        token.Email,
			RefreshToken: token.RefreshToken,
			AccessToken:  token.AccessToken,
			AccessExpiry: token.Expiry,
		}

		// Resolve project ID
		resolver := antigravity.NewProjectResolver(token.AccessToken)
		if pid, err := resolver.ResolveProjectID(); err == nil {
			account.ProjectID = pid
		}

		c.accountStore.AddAccount(account)
		c.currentAccount = account

		logging.PerceptionDebug("[Antigravity] Added account %s to store", token.Email)
	}

	projectID := c.projectID
	if projectID == "" {
		projectID = token.ProjectID
	}
	if projectID == "" {
		projectID = DefaultProjectID
	}

	return token.AccessToken, projectID, nil
}

// Complete sends a prompt and returns the completion.
func (c *AntigravityClient) Complete(ctx context.Context, prompt string) (string, error) {
	return c.CompleteWithSystem(ctx, "", prompt)
}

// CompleteWithSystem sends a prompt with a system message using the CloudCode PA API.
// Implements automatic account rotation on 429 errors.
func (c *AntigravityClient) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	var lastErr error
	var triedAccounts = make(map[string]bool)

	for attempt := 1; attempt <= MaxRetryAttempts; attempt++ {
		accessToken, projectID, err := c.EnsureAuthenticated(ctx)
		if err != nil {
			return "", err
		}

		result, err := c.doRequest(ctx, accessToken, projectID, systemPrompt, userPrompt)
		if err == nil {
			// Success! Record it
			if c.currentAccount != nil {
				c.accountStore.RecordSuccess(c.currentAccount.Email)
			}
			return result, nil
		}

		lastErr = err

		// Check if it's a rate limit error
		if strings.Contains(err.Error(), "429") || strings.Contains(err.Error(), "rate") {
			if c.currentAccount != nil {
				c.accountStore.RecordRateLimit(c.currentAccount.Email)
				triedAccounts[c.currentAccount.Email] = true

				logging.PerceptionWarn("[Antigravity] Account %s rate limited, attempting rotation...", c.currentAccount.Email)

				// Try to get another account
				nextAccount, nextErr := c.accountSelector.SelectNext(c.currentAccount.Email)
				if nextErr == nil && !triedAccounts[nextAccount.Email] {
					c.currentAccount = nextAccount
					logging.PerceptionDebug("[Antigravity] Rotated to account %s", nextAccount.Email)
					continue
				}
			}

			// No more accounts to try, wait and retry with same account
			retryDelay := parseRetryDelay([]byte(err.Error()))
			if retryDelay > MaxRetryDelay {
				retryDelay = MaxRetryDelay
			}

			c.rateLimiter.RecordRateLimit(retryDelay)
			logging.PerceptionWarn("[Antigravity] All accounts exhausted, waiting %v before retry %d/%d...",
				retryDelay, attempt, MaxRetryAttempts)

			select {
			case <-time.After(retryDelay):
				// Clear tried accounts for next round
				triedAccounts = make(map[string]bool)
			case <-ctx.Done():
				return "", ctx.Err()
			}
			continue
		}

		// Non-rate-limit error
		if c.currentAccount != nil {
			c.accountStore.RecordFailure(c.currentAccount.Email, err.Error())
		}
		return "", err
	}

	return "", fmt.Errorf("all retry attempts failed: %w", lastErr)
}

// doRequest performs the actual API request
func (c *AntigravityClient) doRequest(ctx context.Context, accessToken, projectID, systemPrompt, userPrompt string) (string, error) {
	// Apply preemptive delay if under rate limit pressure
	if preemptDelay := c.rateLimiter.GetPreemptiveDelay(); preemptDelay > 0 {
		logging.PerceptionDebug("[Antigravity] Applying preemptive delay of %v", preemptDelay)
		select {
		case <-time.After(preemptDelay):
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}

	url := fmt.Sprintf("%s/v1internal:streamGenerateContent?alt=sse", GeminiCLIEndpointProd)

	// Build generation config with model-aware thinking config
	generationConfig := map[string]interface{}{
		"temperature":     1.0,
		"maxOutputTokens": 32000,
	}

	// Apply model-aware thinking configuration
	if c.enableThinking {
		thinkingConfig := transform.BuildThinkingConfigForModel(
			c.model,
			true, // includeThoughts
			transform.ThinkingTier(c.thinkingLevel),
			0, // budget (will be derived from tier)
		)
		generationConfig["thinkingConfig"] = thinkingConfig

		// Claude thinking models need larger max output tokens
		if transform.IsClaudeThinkingModel(c.model) {
			generationConfig["maxOutputTokens"] = transform.ClaudeThinkingMaxOutputTokens
		}
	}

	// Build inner request using map for flexibility with different model formats
	innerReq := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"role": "user",
				"parts": []map[string]interface{}{
					{"text": userPrompt},
				},
			},
		},
		"generationConfig": generationConfig,
	}

	// Apply system instruction with interleaved thinking hint for Claude thinking models
	effectiveSystemPrompt := systemPrompt
	if transform.IsClaudeThinkingModel(c.model) && systemPrompt != "" {
		effectiveSystemPrompt = transform.ApplyInterleavedThinkingHint(systemPrompt)
	}

	if effectiveSystemPrompt != "" {
		innerReq["systemInstruction"] = map[string]interface{}{
			"parts": []map[string]interface{}{
				{"text": effectiveSystemPrompt},
			},
		}
	}

	uid := uuid.New().String()
	innerReq["sessionId"] = uid

	antigravityReq := map[string]interface{}{
		"project":     projectID,
		"model":       c.model,
		"request":     innerReq,
		"requestType": "agent",
		"userAgent":   "antigravity",
		"requestId":   "agent-" + uid,
	}

	jsonData, err := json.Marshal(antigravityReq)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("User-Agent", "antigravity/1.11.5 windows/amd64")
	req.Header.Set("X-Goog-Api-Client", "google-cloud-sdk vscode_cloudshelleditor/0.1")
	req.Header.Set("Client-Metadata", `{"ideType":"IDE_UNSPECIFIED","platform":"PLATFORM_UNSPECIFIED","pluginType":"GEMINI"}`)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("429 rate limited: %s", string(body))
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("request failed (%d): %s", resp.StatusCode, string(body))
	}

	// Parse SSE Stream with thinking-aware handling
	return c.parseSSEResponse(resp.Body)
}

// parseSSEResponse parses the SSE stream and extracts text content
func (c *AntigravityClient) parseSSEResponse(body io.Reader) (string, error) {
	scanner := bufio.NewScanner(body)
	var fullText strings.Builder
	var lastThoughtSignature string

	type sseChunk struct {
		Response struct {
			Candidates []struct {
				Content struct {
					Parts []struct {
						Text             string `json:"text"`
						Thought          bool   `json:"thought,omitempty"`
						ThoughtSignature string `json:"thoughtSignature,omitempty"`
					} `json:"parts"`
				} `json:"content"`
			} `json:"candidates"`
		} `json:"response"`
	}

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")

		var chunk sseChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		if len(chunk.Response.Candidates) > 0 {
			for _, part := range chunk.Response.Candidates[0].Content.Parts {
				// Skip thinking parts when extracting response text
				// (thinking is for internal reasoning, not user-facing output)
				if !part.Thought {
					fullText.WriteString(part.Text)
				}

				// Cache thought signature for multi-turn conversations
				if part.ThoughtSignature != "" {
					lastThoughtSignature = part.ThoughtSignature
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error reading SSE stream: %w", err)
	}

	// Store last thought signature for potential use in follow-up requests
	if lastThoughtSignature != "" {
		c.mu.Lock()
		c.lastThoughtSignature = lastThoughtSignature
		c.mu.Unlock()
	}

	result := fullText.String()
	if result == "" {
		return "", fmt.Errorf("empty response from stream")
	}

	return strings.TrimSpace(result), nil
}

// CompleteMultiTurn handles multi-turn conversations with full transform support.
// This includes:
// - Cross-model signature sanitization (when switching between Gemini/Claude)
// - Thinking recovery (closing tool loops for fresh thinking)
// - Model-aware thinking configuration
func (c *AntigravityClient) CompleteMultiTurn(
	ctx context.Context,
	systemPrompt string,
	contents []map[string]interface{},
	enableThinkingRecovery bool,
) (string, []map[string]interface{}, error) {
	c.mu.Lock()
	currentModel := c.model
	previousModel := c.previousModel
	c.mu.Unlock()

	// Step 1: Cross-model sanitization if model switched
	if previousModel != "" && transform.GetModelFamily(previousModel) != transform.GetModelFamily(currentModel) {
		result := transform.SanitizeCrossModelPayload(contents, currentModel)
		if result.Modified {
			logging.PerceptionDebug("[Antigravity] Sanitized %d cross-model signatures for %s->%s transition",
				result.SignaturesStripped, previousModel, currentModel)
		}
	}

	// Step 2: Analyze conversation state for thinking recovery
	if enableThinkingRecovery && c.enableThinking {
		state := transform.AnalyzeConversationState(contents)
		if transform.NeedsThinkingRecovery(state) {
			logging.PerceptionDebug("[Antigravity] Applying thinking recovery (tool loop without thinking detected)")
			contents = transform.CloseToolLoopForThinking(contents)
		}
	}

	// Step 3: Build and send request
	accessToken, projectID, err := c.EnsureAuthenticated(ctx)
	if err != nil {
		return "", contents, err
	}

	result, err := c.doMultiTurnRequest(ctx, accessToken, projectID, systemPrompt, contents)
	if err != nil {
		return "", contents, err
	}

	// Step 4: Record success
	if c.currentAccount != nil {
		c.accountStore.RecordSuccess(c.currentAccount.Email)
	}

	// Clear previous model tracking after successful request
	c.mu.Lock()
	c.previousModel = ""
	c.mu.Unlock()

	return result, contents, nil
}

// doMultiTurnRequest performs the multi-turn API request
func (c *AntigravityClient) doMultiTurnRequest(
	ctx context.Context,
	accessToken, projectID, systemPrompt string,
	contents []map[string]interface{},
) (string, error) {
	// Apply preemptive delay if under rate limit pressure
	if preemptDelay := c.rateLimiter.GetPreemptiveDelay(); preemptDelay > 0 {
		logging.PerceptionDebug("[Antigravity] Applying preemptive delay of %v", preemptDelay)
		select {
		case <-time.After(preemptDelay):
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}

	url := fmt.Sprintf("%s/v1internal:streamGenerateContent?alt=sse", GeminiCLIEndpointProd)

	// Build generation config with model-aware thinking config
	generationConfig := map[string]interface{}{
		"temperature":     1.0,
		"maxOutputTokens": 32000,
	}

	// Apply model-aware thinking configuration
	if c.enableThinking {
		thinkingConfig := transform.BuildThinkingConfigForModel(
			c.model,
			true, // includeThoughts
			transform.ThinkingTier(c.thinkingLevel),
			0, // budget (will be derived from tier)
		)
		generationConfig["thinkingConfig"] = thinkingConfig

		// Claude thinking models need larger max output tokens
		if transform.IsClaudeThinkingModel(c.model) {
			generationConfig["maxOutputTokens"] = transform.ClaudeThinkingMaxOutputTokens
		}
	}

	// Build inner request with multi-turn contents
	innerReq := map[string]interface{}{
		"contents":         contents,
		"generationConfig": generationConfig,
	}

	// Apply system instruction with interleaved thinking hint for Claude thinking models
	effectiveSystemPrompt := systemPrompt
	if transform.IsClaudeThinkingModel(c.model) && systemPrompt != "" {
		effectiveSystemPrompt = transform.ApplyInterleavedThinkingHint(systemPrompt)
	}

	if effectiveSystemPrompt != "" {
		innerReq["systemInstruction"] = map[string]interface{}{
			"parts": []map[string]interface{}{
				{"text": effectiveSystemPrompt},
			},
		}
	}

	uid := uuid.New().String()
	innerReq["sessionId"] = uid

	antigravityReq := map[string]interface{}{
		"project":     projectID,
		"model":       c.model,
		"request":     innerReq,
		"requestType": "agent",
		"userAgent":   "antigravity",
		"requestId":   "agent-" + uid,
	}

	jsonData, err := json.Marshal(antigravityReq)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("User-Agent", "antigravity/1.11.5 windows/amd64")
	req.Header.Set("X-Goog-Api-Client", "google-cloud-sdk vscode_cloudshelleditor/0.1")
	req.Header.Set("Client-Metadata", `{"ideType":"IDE_UNSPECIFIED","platform":"PLATFORM_UNSPECIFIED","pluginType":"GEMINI"}`)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("429 rate limited: %s", string(body))
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("request failed (%d): %s", resp.StatusCode, string(body))
	}

	return c.parseSSEResponse(resp.Body)
}

// GetPreviousModel returns the previous model (for debugging)
func (c *AntigravityClient) GetPreviousModel() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.previousModel
}

// NeedsCrossModelSanitization checks if the current model switch requires sanitization
func (c *AntigravityClient) NeedsCrossModelSanitization() bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.previousModel == "" {
		return false
	}

	prevFamily := transform.GetModelFamily(c.previousModel)
	currFamily := transform.GetModelFamily(c.model)

	return prevFamily != currFamily
}

// AddAccount adds a new account to the store (for CLI use)
func (c *AntigravityClient) AddAccount(account *antigravity.Account) error {
	return c.accountStore.AddAccount(account)
}

// ListAccounts returns all accounts (for CLI use)
func (c *AntigravityClient) ListAccounts() []*antigravity.Account {
	return c.accountStore.ListAccounts()
}

// GetAccountStats returns statistics about accounts
func (c *AntigravityClient) GetAccountStats() map[string]interface{} {
	return c.accountSelector.GetStats()
}

// resolveEndpointAndHeaders determines the correct endpoint and headers based on the model.
func (c *AntigravityClient) resolveEndpointAndHeaders() (string, map[string]string) {
	baseHeaders := map[string]string{
		"User-Agent":        "antigravity/1.11.5 windows/amd64",
		"X-Goog-Api-Client": "google-cloud-sdk vscode_cloudshelleditor/0.1",
		"Client-Metadata":   `{"ideType":"IDE_UNSPECIFIED","platform":"PLATFORM_UNSPECIFIED","pluginType":"GEMINI"}`,
	}

	isAntigravity := strings.Contains(c.model, "antigravity") || strings.Contains(c.model, "claude")

	if isAntigravity {
		return AntigravityEndpointDaily, baseHeaders
	}

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
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}

// Placeholder implementations for interface compliance
func (c *AntigravityClient) CompleteWithTools(ctx context.Context, systemPrompt, userPrompt string, tools []ToolDefinition) (*LLMToolResponse, error) {
	text, err := c.CompleteWithSystem(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, err
	}

	return &LLMToolResponse{
		Text: text,
	}, nil
}

func (c *AntigravityClient) CompleteWithSchema(ctx context.Context, systemPrompt, userPrompt, jsonSchema string) (string, error) {
	return c.CompleteWithSystem(ctx, systemPrompt+"\nOutput JSON matching this schema:\n"+jsonSchema, userPrompt)
}

func (c *AntigravityClient) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, _ bool) (<-chan string, <-chan error) {
	errChan := make(chan error, 1)
	errChan <- fmt.Errorf("streaming not implemented for Antigravity yet")
	close(errChan)
	return nil, errChan
}

func (c *AntigravityClient) SetModel(model string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Track previous model for cross-model sanitization
	if c.model != "" && c.model != model {
		c.previousModel = c.model
	}
	c.model = model
}

func (c *AntigravityClient) SchemaCapable() bool {
	return false
}

func (c *AntigravityClient) ShouldUsePiggybackTools() bool {
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
func (c *AntigravityClient) GetLastThoughtSignature() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastThoughtSignature
}
func (c *AntigravityClient) GetLastThoughtSummary() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastThoughtSummary
}
func (c *AntigravityClient) GetLastThinkingTokens() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastThinkingTokens
}
func (c *AntigravityClient) IsThinkingEnabled() bool  { return c.enableThinking }
func (c *AntigravityClient) GetThinkingLevel() string { return c.thinkingLevel }

// GetRateLimitStats returns the current adaptive rate limiter statistics.
func (c *AntigravityClient) GetRateLimitStats() (eventCount int, avgDelay time.Duration) {
	if c.rateLimiter == nil {
		return 0, 0
	}
	return c.rateLimiter.GetStats()
}
