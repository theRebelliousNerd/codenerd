package perception

import (
	"bytes"
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ErrLLMUnavailable is a sentinel marking a transient model-side failure
// (the 5xx family, most commonly 503 UNAVAILABLE "high demand") that survived
// all retries. It is wrapped into the returned error via %w at the transient
// retry sites so callers up the chain can distinguish a temporarily-unreachable
// model from a genuine parse/ambiguity failure using errors.Is. This is the
// linchpin of the "transient failure must be distinguishable from user
// ambiguity" guarantee: without it, a 503 is laundered into "I don't understand
// you" at the perception firewall.
var ErrLLMUnavailable = errors.New("llm temporarily unavailable")

// GeminiClient implements LLMClient for Google Gemini API.
type GeminiClient struct {
	apiKey                string
	baseURL               string
	model                 string
	maxOutputTokens       int
	maxOutputTokensConfig bool
	httpClient            *http.Client
	mu                    sync.Mutex
	lastRequest           time.Time

	// Thinking mode settings
	enableThinking bool
	thinkingLevel  string // Gemini 3: minimal/low/medium/high (lowercase)

	// Built-in tools
	enableGoogleSearch bool
	enableURLContext   bool
	urlContextURLs     []string

	// Multi-turn function calling: thought signature from last response
	lastThoughtSignature string
	lastToolCalls        []geminiToolCall

	// Grounding sources from last response (for transparency)
	lastGroundingSources []string

	// Thinking metadata from last response (for SPL learning)
	lastThoughtSummary string
	lastThinkingTokens int

	// Context Caching
	cachedContentName string // Resource name of active cached context
}

// Compile-time interface assertions for GeminiClient.
// These ensure GeminiClient properly implements all required interfaces.
var (
	_ LLMClient = (*GeminiClient)(nil) // Base LLM client interface

	// Gemini advanced features interfaces (from types package)
	_ interface{ GetLastThoughtSummary() string }     = (*GeminiClient)(nil) // ThinkingProvider
	_ interface{ GetLastThinkingTokens() int }        = (*GeminiClient)(nil) // ThinkingProvider
	_ interface{ IsThinkingEnabled() bool }           = (*GeminiClient)(nil) // ThinkingProvider
	_ interface{ GetThinkingLevel() string }          = (*GeminiClient)(nil) // ThinkingProvider
	_ interface{ GetLastThoughtSignature() string }   = (*GeminiClient)(nil) // ThoughtSignatureProvider
	_ interface{ GetLastGroundingSources() []string } = (*GeminiClient)(nil) // GroundingProvider
	_ interface{ IsGoogleSearchEnabled() bool }       = (*GeminiClient)(nil) // GroundingProvider
	_ interface{ IsURLContextEnabled() bool }         = (*GeminiClient)(nil) // GroundingProvider
	_ interface{ SetEnableGoogleSearch(bool) }        = (*GeminiClient)(nil) // GroundingController
	_ interface{ SetEnableURLContext(bool) }          = (*GeminiClient)(nil) // GroundingController
	_ interface{ SetURLContextURLs([]string) }        = (*GeminiClient)(nil) // GroundingController
	_ interface{ ShouldUsePiggybackTools() bool }     = (*GeminiClient)(nil) // PiggybackToolProvider
)

type geminiToolCall struct {
	id        string
	name      string
	signature string
}

// DefaultGeminiConfig returns sensible defaults.
//
// Model choice — gemini-3.5-flash:
//
//	codeNERD uses structured output (responseJsonSchema) together with
//	grounding tools (Google Search + URL Context) for chat articulation.
//	Per the Gemini docs, that combination is currently a Preview feature
//	supported ONLY on `gemini-3.1-pro-preview` and `gemini-3.5-flash`. On
//	models outside that list (notably `gemini-3.1-flash-lite`), the API
//	exhibits undefined behavior — most commonly mid-stream truncation of
//	the JSON envelope, which surfaces to users as raw control_packet
//	dumps. Flash is the right default: structured-output capable and fast.
//
// Thinking level — "medium":
//
//	Per the Gemini 3 docs, `medium` is the API default and yields "very
//	good results across a wide range of tasks while being faster and more
//	cost-efficient." Set explicitly so we don't surprise users who omit
//	the field. Set to "high" only when a task genuinely benefits from
//	maximum reasoning depth.
//
// Output tokens — 65536:
//
//	Gemini 3 cap. thoughtsTokenCount and candidatesTokenCount are tracked
//	separately by the API (UsageMetadata), so high thinking does NOT eat
//	the visible-output budget.
func DefaultGeminiConfig(apiKey string) GeminiConfig {
	return GeminiConfig{
		APIKey:          apiKey,
		BaseURL:         "https://generativelanguage.googleapis.com/v1beta",
		Model:           "gemini-3.5-flash",
		Timeout:         10 * time.Minute, // Large context models need extended timeout
		MaxOutputTokens: 65536,
		EnableThinking:  true,
		ThinkingLevel:   "medium",
	}
}

// NewGeminiClient creates a new Gemini client.
func NewGeminiClient(apiKey string) *GeminiClient {
	config := DefaultGeminiConfig(apiKey)
	return NewGeminiClientWithConfig(config)
}

// NewGeminiClientWithConfig creates a new Gemini client with custom config.
func NewGeminiClientWithConfig(config GeminiConfig) *GeminiClient {
	model := strings.TrimSpace(config.Model)
	if model == "" {
		// Match DefaultGeminiConfig() so the runtime fallback and the
		// constructor agree on the canonical default.
		model = "gemini-3.5-flash"
	}

	maxOutputTokens := config.MaxOutputTokens
	maxOutputTokensConfigured := config.MaxOutputTokens > 0
	if !maxOutputTokensConfigured {
		maxOutputTokens = defaultMaxOutputTokensForModel(model)
	}

	thinkingLevel := strings.ToLower(strings.TrimSpace(config.ThinkingLevel))
	if config.EnableThinking {
		if thinkingLevel == "" {
			// Per Gemini 3 docs, "medium" is the API default and recommended
			// for most tasks. The previous "high" auto-default forced max
			// reasoning depth on every call without the user opting in.
			thinkingLevel = "medium"
		}
	}

	return &GeminiClient{
		apiKey:                config.APIKey,
		baseURL:               config.BaseURL,
		model:                 model,
		maxOutputTokens:       maxOutputTokens,
		maxOutputTokensConfig: maxOutputTokensConfigured,
		httpClient:            NewSharedHTTPClient(config.Timeout),
		// Thinking mode
		enableThinking: config.EnableThinking || isGemini3Model(model),
		thinkingLevel:  thinkingLevel,
		// Built-in tools
		enableGoogleSearch: config.EnableGoogleSearch,
		enableURLContext:   config.EnableURLContext,
		urlContextURLs:     config.URLContextURLs,
	}
}

func isGemini3Model(model string) bool {
	return strings.Contains(strings.ToLower(model), "gemini-3")
}

func defaultMaxOutputTokensForModel(model string) int {
	if isGemini3Model(model) {
		return 65536
	}
	return 65536
}

// buildThinkingConfig creates a GeminiThinkingConfig if thinking is enabled.
func (c *GeminiClient) buildThinkingConfig() *GeminiThinkingConfig {
	if !c.enableThinking {
		return nil
	}

	cfg := &GeminiThinkingConfig{
		IncludeThoughts: true,
	}

	level := c.thinkingLevel
	if level == "" {
		// Match the documented API default rather than forcing "high".
		level = "medium"
	}
	cfg.ThinkingLevel = level

	return cfg
}

func (c *GeminiClient) captureThoughtSignature(resp *GeminiResponse) {
	if resp == nil {
		return
	}
	if resp.ThoughtSignature != "" {
		c.lastThoughtSignature = resp.ThoughtSignature
	}
	if len(resp.Candidates) == 0 {
		return
	}
	for _, part := range resp.Candidates[0].Content.Parts {
		if part.FunctionCall != nil && part.FunctionCall.ThoughtSignature != "" {
			c.lastThoughtSignature = part.FunctionCall.ThoughtSignature
			return
		}
		if part.ThoughtSignature != "" {
			c.lastThoughtSignature = part.ThoughtSignature
			return
		}
	}
}

func (c *GeminiClient) extractToolCalls(resp *GeminiResponse) []geminiToolCall {
	if resp == nil || len(resp.Candidates) == 0 {
		return nil
	}
	responseSignature := resp.ThoughtSignature
	calls := make([]geminiToolCall, 0, len(resp.Candidates[0].Content.Parts))
	for _, part := range resp.Candidates[0].Content.Parts {
		if part.FunctionCall == nil {
			continue
		}
		signature := part.FunctionCall.ThoughtSignature
		if signature == "" {
			signature = part.ThoughtSignature
		}
		if signature == "" {
			signature = responseSignature
		}
		calls = append(calls, geminiToolCall{
			id:        fmt.Sprintf("call_%d", len(calls)),
			name:      part.FunctionCall.Name,
			signature: signature,
		})
	}
	return calls
}

// buildBuiltInTools creates GeminiTool entries for enabled built-in tools.
func (c *GeminiClient) buildBuiltInTools() []GeminiTool {
	var tools []GeminiTool

	if c.enableGoogleSearch {
		tools = append(tools, GeminiTool{
			GoogleSearch: &GeminiGoogleSearch{},
		})
	}

	if c.enableURLContext && len(c.urlContextURLs) > 0 {
		// Limit to max 20 URLs per API spec
		urls := c.urlContextURLs
		if len(urls) > 20 {
			urls = urls[:20]
		}
		tools = append(tools, GeminiTool{
			URLContext: &GeminiURLContext{URLs: urls},
		})
	}

	return tools
}

// SetURLContextURLs sets URLs for the URL context tool.
func (c *GeminiClient) SetURLContextURLs(urls []string) {
	c.urlContextURLs = urls
}

// SetEnableGoogleSearch enables or disables Google Search grounding at runtime.
func (c *GeminiClient) SetEnableGoogleSearch(enable bool) {
	c.enableGoogleSearch = enable
}

// SetEnableURLContext enables or disables URL Context tool at runtime.
func (c *GeminiClient) SetEnableURLContext(enable bool) {
	c.enableURLContext = enable
}

// IsGoogleSearchEnabled returns whether Google Search grounding is enabled.
func (c *GeminiClient) IsGoogleSearchEnabled() bool {
	return c.enableGoogleSearch
}

// IsURLContextEnabled returns whether URL Context tool is enabled.
func (c *GeminiClient) IsURLContextEnabled() bool {
	return c.enableURLContext
}

// GetLastGroundingSources returns the grounding sources from the last response.
// These are URLs that Gemini used to ground its response via Google Search or URL Context.
func (c *GeminiClient) GetLastGroundingSources() []string {
	return c.lastGroundingSources
}

// ShouldUsePiggybackTools returns true if this client should use Piggyback Protocol
// for tool invocation instead of native function calling.
// For Gemini, this is true when grounding tools (Google Search, URL Context) are enabled,
// because Gemini API cannot combine built-in tools with function declarations.
func (c *GeminiClient) ShouldUsePiggybackTools() bool {
	// Use Piggyback for tools when grounding is enabled
	// This allows tool_requests via structured output while keeping grounding active
	return c.enableGoogleSearch || c.enableURLContext
}

// GetLastThoughtSignature returns the thought signature from the last response.
// This should be passed back in multi-turn function calling scenarios.
func (c *GeminiClient) GetLastThoughtSignature() string {
	return c.lastThoughtSignature
}

// =============================================================================
// ThinkingProvider Interface Implementation
// =============================================================================

// GetLastThoughtSummary returns the model's reasoning process from the last call.
// Used by SPL to understand WHY the model made certain decisions.
func (c *GeminiClient) GetLastThoughtSummary() string {
	return c.lastThoughtSummary
}

// GetLastThinkingTokens returns the number of tokens used for reasoning.
// Used by SPL for budget monitoring and reasoning quality assessment.
func (c *GeminiClient) GetLastThinkingTokens() int {
	return c.lastThinkingTokens
}

// IsThinkingEnabled returns whether thinking mode is currently enabled.
func (c *GeminiClient) IsThinkingEnabled() bool {
	return c.enableThinking
}

// GetThinkingLevel returns the current thinking level (e.g., "minimal", "low", "medium", "high").
func (c *GeminiClient) GetThinkingLevel() string {
	return c.thinkingLevel
}

// SetCachedContent sets the active cached context for subsequent requests.
// pass empty string to disable.
func (c *GeminiClient) SetCachedContent(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cachedContentName = name
}

// =============================================================================
// SHARED REQUEST HELPERS
// =============================================================================

// isTransientGeminiStatus reports whether an HTTP status from the Gemini API
// is a transient, retryable server-side condition. 429 (rate limit) is handled
// separately with its own message; this covers the 5xx family — most importantly
// 503 UNAVAILABLE ("model is currently experiencing high demand"), which is the
// single most common transient Gemini failure. These are safe to retry with
// backoff rather than surfacing as a hard error that collapses the turn.
func isTransientGeminiStatus(code int) bool {
	switch code {
	case http.StatusInternalServerError, // 500
		http.StatusBadGateway,         // 502
		http.StatusServiceUnavailable, // 503
		http.StatusGatewayTimeout:     // 504
		return true
	default:
		return false
	}
}

// rateLimit enforces minimum inter-request spacing to avoid 429 responses.
// Must be called before each API request.
func (c *GeminiClient) rateLimit() {
	c.mu.Lock()
	elapsed := time.Since(c.lastRequest)
	if elapsed < 100*time.Millisecond {
		time.Sleep(100*time.Millisecond - elapsed)
	}
	c.lastRequest = time.Now()
	c.mu.Unlock()
}

// extractResponseText separates thought parts from response parts in a Gemini response.
// Returns the thought content and the response content as separate strings.
func extractResponseText(parts []GeminiResponsePart) (thoughts string, response string) {
	var thoughtBuilder strings.Builder
	var responseBuilder strings.Builder
	for _, part := range parts {
		if part.Thought {
			thoughtBuilder.WriteString(part.Text)
		} else if part.Text != "" {
			responseBuilder.WriteString(part.Text)
		}
	}
	return strings.TrimSpace(thoughtBuilder.String()), strings.TrimSpace(responseBuilder.String())
}

// captureGroundingSources extracts web grounding sources from a Gemini response
// and stores them in the client's lastGroundingSources field.
func (c *GeminiClient) captureGroundingSources(resp *GeminiResponse) {
	c.lastGroundingSources = nil
	if len(resp.Candidates) > 0 && resp.Candidates[0].GroundingMetadata != nil {
		gm := resp.Candidates[0].GroundingMetadata
		for _, chunk := range gm.GroundingChunks {
			if chunk.Web != nil && chunk.Web.URI != "" {
				c.lastGroundingSources = append(c.lastGroundingSources, chunk.Web.URI)
			}
		}
	}
}

// Complete sends a prompt and returns the completion.
func (c *GeminiClient) Complete(ctx context.Context, prompt string) (string, error) {
	return c.CompleteWithSystem(ctx, "", prompt)
}

// CompleteWithSystem sends a prompt with a system message.
func (c *GeminiClient) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	// Auto-apply timeout if context has no deadline (centralized timeout handling)
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.httpClient.Timeout)
		defer cancel()
	}

	startTime := time.Now()
	logging.PerceptionDebug("[Gemini] CompleteWithSystem: model=%s system_len=%d user_len=%d", c.model, len(systemPrompt), len(userPrompt))

	if c.apiKey == "" {
		logging.PerceptionError("[Gemini] CompleteWithSystem: API key not configured")
		return "", fmt.Errorf("API key not configured")
	}

	// Check if using cached content
	c.mu.Lock()
	cachedContent := c.cachedContentName
	c.mu.Unlock()

	if strings.TrimSpace(systemPrompt) == "" {
		systemPrompt = defaultSystemPrompt
	}

	// If using cached content, we generally shouldn't send system instructions again
	// if they were part of the cache. For now, we'll leave it as is,
	// assuming the API handles overrides or the user knows what they are doing.

	isPiggyback := strings.Contains(systemPrompt, "control_packet") ||
		strings.Contains(userPrompt, "PiggybackEnvelope") ||
		strings.Contains(userPrompt, "control_packet")

	// Rate limiting
	c.rateLimit()

	reqBody := GeminiRequest{
		Contents: []GeminiContent{
			{
				Role:  "user",
				Parts: []GeminiPart{{Text: userPrompt}},
			},
		},
		SystemInstruction: &GeminiContent{
			Parts: []GeminiPart{{Text: systemPrompt}},
		},
		GenerationConfig: GeminiGenerationConfig{
			Temperature:     1.0,
			MaxOutputTokens: c.maxOutputTokens,
			ThinkingConfig:  c.buildThinkingConfig(),
		},
		Tools: c.buildBuiltInTools(),
	}
	if isPiggyback {
		reqBody.GenerationConfig.ResponseMimeType = "application/json"
		reqBody.GenerationConfig.ResponseSchema = BuildGeminiPiggybackEnvelopeSchema()
	} else if requiresJSONOutput(systemPrompt, userPrompt) {
		reqBody.GenerationConfig.ResponseMimeType = "application/json"
	}

	// Apply cached content if set
	if cachedContent != "" {
		reqBody.CachedContent = cachedContent
	}

	// Construct URL with API key
	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", c.baseURL, c.model, c.apiKey)

	logging.LogLLMRequest("GeminiClient", systemPrompt, userPrompt, nil, c.model, reqBody.GenerationConfig.Temperature)

	// Retry loop for rate limits
	maxRetries := 3
	var lastErr error

	for i := 0; i <= maxRetries; i++ {
		if i > 0 {
			time.Sleep(time.Duration(1<<uint(i-1)) * time.Second)
		}

		jsonData, err := json.Marshal(reqBody)
		if err != nil {
			return "", fmt.Errorf("failed to marshal request: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
		if err != nil {
			return "", fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("request failed: %w", err)
			continue
		}

		body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("failed to read response: %w", err)
			continue
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			lastErr = fmt.Errorf("rate limit exceeded (429)")
			continue
		}

		if isTransientGeminiStatus(resp.StatusCode) {
			// Wrap the sentinel so errors.Is(err, ErrLLMUnavailable) holds up the
			// whole chain (through the post-loop "max retries exceeded: %w"). This
			// lets the perception firewall report /llm_unavailable rather than
			// laundering a transient 503 into a "you were unclear" clarification.
			lastErr = fmt.Errorf("transient server error (%d): %s: %w", resp.StatusCode, strings.TrimSpace(string(body)), ErrLLMUnavailable)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			// Some models may reject responseJsonSchema; retry once without it.
			if isPiggyback && reqBody.GenerationConfig.ResponseSchema != nil && resp.StatusCode == http.StatusBadRequest {
				bodyStr := string(body)
				if strings.Contains(bodyStr, "responseJsonSchema") || strings.Contains(bodyStr, "responseMimeType") ||
					strings.Contains(bodyStr, "response_schema") || strings.Contains(bodyStr, "response_mime_type") {
					reqBody.GenerationConfig.ResponseSchema = nil
					reqBody.GenerationConfig.ResponseMimeType = ""
					lastErr = fmt.Errorf("request rejected structured output, retrying without responseJsonSchema: %s", bodyStr)
					continue
				}
			}
			return "", fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
		}

		var geminiResp GeminiResponse
		if err := json.Unmarshal(body, &geminiResp); err != nil {
			return "", fmt.Errorf("failed to parse response: %w", err)
		}

		if geminiResp.Error != nil {
			return "", fmt.Errorf("API error: %s", geminiResp.Error.Message)
		}

		c.captureThoughtSignature(&geminiResp)
		c.lastThoughtSummary = geminiResp.ThoughtSummary
		c.lastThinkingTokens = geminiResp.UsageMetadata.ThoughtsTokenCount

		if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
			return "", fmt.Errorf("no completion returned")
		}

		// Separate thought parts from response parts for clear presentation
		thinkingText, responseText := extractResponseText(geminiResp.Candidates[0].Content.Parts)
		logging.PerceptionDebug("[Gemini] CompleteWithSystem: thinking_len=%d thoughtSummary=%d chars",
			len(thinkingText), len(geminiResp.ThoughtSummary))

		// Build final response with thinking section if present
		// Use part-level thoughts if available, otherwise fall back to response-level thoughtSummary
		var result strings.Builder
		if thinkingText == "" && geminiResp.ThoughtSummary != "" {
			thinkingText = strings.TrimSpace(geminiResp.ThoughtSummary)
		}
		// The thinking text is stored in c.lastThoughtSummary (below) and shouldn't be prepended to the returned string.
		result.WriteString(responseText)
		response := result.String()

		// Store thought summary for API access
		if thinkingText != "" && c.lastThoughtSummary == "" {
			c.lastThoughtSummary = thinkingText
		}

		// Extract and store grounding sources for transparency
		c.captureGroundingSources(&geminiResp)

		// Log thinking tokens if used
		cachedTokens := geminiResp.UsageMetadata.CachedContentTokenCount
		if geminiResp.UsageMetadata.ThoughtsTokenCount > 0 {
			logging.Perception("[Gemini] CompleteWithSystem: completed in %v response_len=%d thinking_tokens=%d thinking_chars=%d grounding_sources=%d cached_tokens=%d",
				time.Since(startTime), len(response), geminiResp.UsageMetadata.ThoughtsTokenCount, len(thinkingText), len(c.lastGroundingSources), cachedTokens)
		} else {
			logging.Perception("[Gemini] CompleteWithSystem: completed in %v response_len=%d grounding_sources=%d cached_tokens=%d",
				time.Since(startTime), len(response), len(c.lastGroundingSources), cachedTokens)
		}

		logging.LogLLMResponse("gemini", response, time.Since(startTime), len(response)/4)
		return response, nil
	}

	logging.PerceptionError("[Gemini] CompleteWithSystem: max retries exceeded after %v: %v", time.Since(startTime), lastErr)
	logging.LogLLMError("gemini", lastErr, time.Since(startTime))
	return "", fmt.Errorf("max retries exceeded: %w", lastErr)
}

const geminiSchemaDepthLimit = 10 // Gemini 3 can handle deeper nesting

func schemaMaxDepth(value any, depth int) int {
	maxDepth := depth
	switch typed := value.(type) {
	case map[string]any:
		if depth+1 > maxDepth {
			maxDepth = depth + 1
		}
		for _, child := range typed {
			if childDepth := schemaMaxDepth(child, depth+1); childDepth > maxDepth {
				maxDepth = childDepth
			}
		}
	case []any:
		if depth+1 > maxDepth {
			maxDepth = depth + 1
		}
		for _, child := range typed {
			if childDepth := schemaMaxDepth(child, depth+1); childDepth > maxDepth {
				maxDepth = childDepth
			}
		}
	}
	return maxDepth
}

func shallowSchema(schema map[string]any) map[string]any {
	if schema == nil {
		return map[string]any{"type": "object"}
	}
	props := map[string]any{}
	if rawProps, ok := schema["properties"].(map[string]any); ok {
		for key, value := range rawProps {
			props[key] = shallowSchemaProperty(value)
		}
	}
	result := map[string]any{
		"type": "object",
	}
	if len(props) > 0 {
		result["properties"] = props
	}
	if required, ok := schema["required"]; ok {
		result["required"] = required
	}
	return result
}

func shallowSchemaProperty(value any) map[string]any {
	if valueMap, ok := value.(map[string]any); ok {
		if enumVal, ok := valueMap["enum"]; ok {
			return map[string]any{
				"type": "string",
				"enum": enumVal,
			}
		}
		if typeVal, ok := valueMap["type"].(string); ok && typeVal != "" {
			return map[string]any{
				"type": typeVal,
			}
		}
	}
	return map[string]any{
		"type": "string",
	}
}

// SchemaCapable reports whether this client supports response schema enforcement.
func (c *GeminiClient) SchemaCapable() bool {
	return true
}

// CompleteWithSchema sends a prompt and enforces a JSON schema in the response.
// Uses Gemini generationConfig.responseJsonSchema with responseMimeType.
func (c *GeminiClient) CompleteWithSchema(ctx context.Context, systemPrompt, userPrompt, jsonSchema string) (string, error) {
	// Auto-apply timeout if context has no deadline (centralized timeout handling)
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.httpClient.Timeout)
		defer cancel()
	}

	startTime := time.Now()
	logging.PerceptionDebug("[Gemini] CompleteWithSchema: model=%s system_len=%d user_len=%d schema_len=%d", c.model, len(systemPrompt), len(userPrompt), len(jsonSchema))

	if c.apiKey == "" {
		logging.PerceptionError("[Gemini] CompleteWithSchema: API key not configured")
		return "", fmt.Errorf("API key not configured")
	}

	schemaText := strings.TrimSpace(jsonSchema)
	if schemaText == "" {
		return "", fmt.Errorf("json schema is empty")
	}

	var schema map[string]any
	if err := json.Unmarshal([]byte(schemaText), &schema); err != nil {
		return "", fmt.Errorf("invalid json schema: %w", err)
	}
	if isGemini3Model(c.model) {
		depth := schemaMaxDepth(schema, 0)
		if depth > geminiSchemaDepthLimit {
			logging.PerceptionWarn("[Gemini] Schema depth %d exceeds limit %d; using shallow schema", depth, geminiSchemaDepthLimit)
			schema = shallowSchema(schema)
		}
	}

	if strings.TrimSpace(systemPrompt) == "" {
		systemPrompt = defaultSystemPrompt
	}

	// Rate limiting
	c.rateLimit()

	reqBody := GeminiRequest{
		Contents: []GeminiContent{
			{
				Role:  "user",
				Parts: []GeminiPart{{Text: userPrompt}},
			},
		},
		SystemInstruction: &GeminiContent{
			Parts: []GeminiPart{{Text: systemPrompt}},
		},
		GenerationConfig: GeminiGenerationConfig{
			Temperature:      1.0,
			MaxOutputTokens:  c.maxOutputTokens,
			ThinkingConfig:   c.buildThinkingConfig(), // Thinking enabled - thought parts will be filtered
			ResponseMimeType: "application/json",
			ResponseSchema:   schema,
		},
		Tools: c.buildBuiltInTools(),
	}

	// Construct URL with API key
	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", c.baseURL, c.model, c.apiKey)

	logging.LogLLMRequest("GeminiClient-Schema", systemPrompt, userPrompt, nil, c.model, reqBody.GenerationConfig.Temperature)

	// Retry loop for rate limits
	maxRetries := 3
	var lastErr error

	for i := 0; i <= maxRetries; i++ {
		if i > 0 {
			time.Sleep(time.Duration(1<<uint(i-1)) * time.Second)
		}

		jsonData, err := json.Marshal(reqBody)
		if err != nil {
			return "", fmt.Errorf("failed to marshal request: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
		if err != nil {
			return "", fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("request failed: %w", err)
			continue
		}

		body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("failed to read response: %w", err)
			continue
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			lastErr = fmt.Errorf("rate limit exceeded (429)")
			continue
		}

		if isTransientGeminiStatus(resp.StatusCode) {
			// Wrap the sentinel so errors.Is(err, ErrLLMUnavailable) holds up the
			// whole chain (through the post-loop "max retries exceeded: %w"). This
			// lets the perception firewall report /llm_unavailable rather than
			// laundering a transient 503 into a "you were unclear" clarification.
			lastErr = fmt.Errorf("transient server error (%d): %s: %w", resp.StatusCode, strings.TrimSpace(string(body)), ErrLLMUnavailable)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			bodyStr := string(body)
			if resp.StatusCode == http.StatusBadRequest {
				bodyLower := strings.ToLower(bodyStr)
				if strings.Contains(bodyLower, "responsejsonschema") ||
					strings.Contains(bodyLower, "responsemimetype") ||
					strings.Contains(bodyLower, "response_schema") ||
					strings.Contains(bodyLower, "response_mime_type") ||
					strings.Contains(bodyLower, "responseschema") ||
					(strings.Contains(bodyLower, "schema") && strings.Contains(bodyLower, "nesting depth")) {
					return "", core.ErrSchemaNotSupported
				}
			}
			return "", fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, bodyStr)
		}

		var geminiResp GeminiResponse
		if err := json.Unmarshal(body, &geminiResp); err != nil {
			return "", fmt.Errorf("failed to parse response: %w", err)
		}

		if geminiResp.Error != nil {
			return "", fmt.Errorf("API error: %s", geminiResp.Error.Message)
		}

		c.captureThoughtSignature(&geminiResp)
		c.lastThoughtSummary = geminiResp.ThoughtSummary
		c.lastThinkingTokens = geminiResp.UsageMetadata.ThoughtsTokenCount

		if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
			return "", fmt.Errorf("no completion returned")
		}

		// Filter out thought parts - only include non-thought content for structured output
		thinkingText, response := extractResponseText(geminiResp.Candidates[0].Content.Parts)

		// Log thought summary if any thinking was done
		if thinkingText != "" {
			logging.PerceptionDebug("[Gemini] CompleteWithSchema: thought summary length=%d", len(thinkingText))
		}

		// Extract and store grounding sources for transparency
		c.captureGroundingSources(&geminiResp)

		// Log thinking tokens if used
		cachedTokens := geminiResp.UsageMetadata.CachedContentTokenCount
		if geminiResp.UsageMetadata.ThoughtsTokenCount > 0 {
			logging.Perception("[Gemini] CompleteWithSchema: completed in %v response_len=%d thinking_tokens=%d grounding_sources=%d cached_tokens=%d",
				time.Since(startTime), len(response), geminiResp.UsageMetadata.ThoughtsTokenCount, len(c.lastGroundingSources), cachedTokens)
		} else {
			logging.Perception("[Gemini] CompleteWithSchema: completed in %v response_len=%d grounding_sources=%d cached_tokens=%d",
				time.Since(startTime), len(response), len(c.lastGroundingSources), cachedTokens)
		}

		logging.LogLLMResponse("gemini-schema", response, time.Since(startTime), len(response)/4)
		return response, nil
	}

	logging.PerceptionError("[Gemini] CompleteWithSchema: max retries exceeded after %v: %v", time.Since(startTime), lastErr)
	logging.LogLLMError("gemini-schema", lastErr, time.Since(startTime))
	return "", fmt.Errorf("max retries exceeded: %w", lastErr)
}

func (c *GeminiClient) SetModel(model string) {
	c.model = model
	if !c.maxOutputTokensConfig {
		c.maxOutputTokens = defaultMaxOutputTokensForModel(model)
	}
}

// GetModel returns the current model.
func (c *GeminiClient) GetModel() string {
	return c.model
}

// CountTokens counts the total number of tokens for the given prompt.
func (c *GeminiClient) CountTokens(ctx context.Context, systemPrompt, userPrompt string) (int, error) {
	reqBody := GeminiRequest{
		Contents: []GeminiContent{
			{
				Role: "user",
				Parts: []GeminiPart{
					{Text: userPrompt},
				},
			},
		},
	}

	if systemPrompt != "" {
		reqBody.SystemInstruction = &GeminiContent{
			Role: "system",
			Parts: []GeminiPart{
				{Text: systemPrompt},
			},
		}
	}

	// Apply cached content if set
	cachedContent := c.cachedContentName
	if cachedContent != "" {
		reqBody.CachedContent = cachedContent
	}

	// For counting tokens, we use the specific API endpoint
	url := fmt.Sprintf("%s/models/%s:countTokens?key=%s", c.baseURL, c.model, c.apiKey)

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return 0, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var countResp struct {
		TotalTokens int `json:"totalTokens"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&countResp); err != nil {
		return 0, err
	}

	return countResp.TotalTokens, nil
}
