package perception

import (
	"bytes"
	"codenerd/internal/logging"
	"codenerd/internal/types"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// postGenerateContent POSTs one :generateContent request with retry for rate
// limits and transient server errors. Both tool-bearing paths share it; the
// chat and schema paths keep their own loops for prompt caching and
// schema-fallback handling. Callers map the parsed response.
func (c *GeminiClient) postGenerateContent(ctx context.Context, reqBody GeminiRequest) (*GeminiResponse, error) {
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", c.baseURL, c.model, c.apiKey)

	maxRetries := 3
	var lastErr error
	for i := 0; i <= maxRetries; i++ {
		if i > 0 {
			// Context-aware backoff: a cancelled turn must exit during
			// the sleep, not after it (matches ExecuteOpenAIRequest).
			backoff := time.Duration(1<<uint(i-1)) * time.Second
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("request cancelled during retry backoff: %w", ctx.Err())
			case <-time.After(backoff):
			}
		}

		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
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

		if isTransientHTTPStatus(resp.StatusCode) {
			// Wrap the sentinel like the chat paths so the perception
			// firewall reports /llm_unavailable rather than laundering a
			// transient into a clarification.
			lastErr = fmt.Errorf("transient server error (%d): %s: %w", resp.StatusCode, strings.TrimSpace(string(body)), ErrLLMUnavailable)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
		}

		var geminiResp GeminiResponse
		if err := json.Unmarshal(body, &geminiResp); err != nil {
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}
		if geminiResp.Error != nil {
			return nil, fmt.Errorf("API error: %s", geminiResp.Error.Message)
		}
		return &geminiResp, nil
	}
	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}

// mapToolsResponse records continuity state and maps one :generateContent
// reply onto the tool-response contract. Shared by the single-shot,
// native multi-turn, and ToolResultsProvider-adapted paths so a length
// stop, a usage row, or the thought-signature capture cannot drift between
// them. method names the public entry point for truncation diagnostics.
func (c *GeminiClient) mapToolsResponse(ctx context.Context, geminiResp *GeminiResponse, method string, toolsLen int) (*LLMToolResponse, error) {
	// Capture thought signatures/tool calls for Gemini 3 multi-turn continuity.
	c.captureThoughtSignature(geminiResp)
	c.lastToolCalls = c.extractToolCalls(geminiResp)
	c.lastThoughtSummary = geminiResp.ThoughtSummary
	c.lastThinkingTokens = geminiResp.UsageMetadata.ThoughtsTokenCount

	result := &LLMToolResponse{}
	if geminiResp.ThoughtSummary != "" {
		result.ThoughtSummary = geminiResp.ThoughtSummary
	}
	if c.lastThoughtSignature != "" {
		result.ThoughtSignature = c.lastThoughtSignature
	}

	result.Usage = types.UsageMetadata{
		InputTokens:         geminiResp.UsageMetadata.PromptTokenCount,
		OutputTokens:        geminiResp.UsageMetadata.CandidatesTokenCount,
		TotalTokens:         geminiResp.UsageMetadata.TotalTokenCount,
		ThinkingTokens:      geminiResp.UsageMetadata.ThoughtsTokenCount,
		CachedContentTokens: geminiResp.UsageMetadata.CachedContentTokenCount,
	}

	trackUsage(ctx, c.model, ProviderGemini,
		geminiResp.UsageMetadata.PromptTokenCount,
		geminiOutputTokens(geminiResp.UsageMetadata.CandidatesTokenCount, geminiResp.UsageMetadata.ThoughtsTokenCount),
		usageOpFor(toolsLen))

	if len(geminiResp.Candidates) > 0 {
		result.StopReason = geminiResp.Candidates[0].FinishReason
		if types.LengthStop(result.StopReason) {
			// A function call cut mid-arguments must not reach the executor.
			return nil, outputTruncated(ProviderGemini, c.model, method, result.StopReason, "",
				c.maxOutputTokens, geminiResp.UsageMetadata.CandidatesTokenCount)
		}
		// applyGeminiBlocks reads the parts in order, keeping per-part thought
		// signatures, and fills Text and ToolCalls as the flat projection of
		// them. Text is the answer's TEXT parts only: thought-part text is the
		// model's private reasoning and must not be prepended to the answer.
		// Tool-call ids are minted positionally ("call_N"), as before.
		applyGeminiBlocks(result, geminiResp)

		if gm := geminiResp.Candidates[0].GroundingMetadata; gm != nil {
			for _, chunk := range gm.GroundingChunks {
				if chunk.Web != nil && chunk.Web.URI != "" {
					result.GroundingSources = append(result.GroundingSources, chunk.Web.URI)
				}
			}
			if len(gm.GroundingChunks) > 0 {
				logging.PerceptionDebug("[Gemini] %s: grounding sources=%d queries=%v",
					method, len(gm.GroundingChunks), gm.WebSearchQueries)
			}
		}
	}
	return result, nil
}

// CompleteWithTools sends a prompt with tool definitions and returns tool calls.
func (c *GeminiClient) CompleteWithTools(ctx context.Context, systemPrompt, userPrompt string, tools []ToolDefinition) (*LLMToolResponse, error) {
	// Auto-apply timeout if context has no deadline
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.httpClient.Timeout)
		defer cancel()
	}

	startTime := time.Now()
	logging.PerceptionDebug("[Gemini] CompleteWithTools: model=%s tools=%d system_len=%d user_len=%d",
		c.model, len(tools), len(systemPrompt), len(userPrompt))

	if c.apiKey == "" {
		return nil, fmt.Errorf("API key not configured")
	}
	c.lastToolCalls = nil

	// Convert tools to Gemini format
	geminiTools := make([]GeminiFunctionDeclaration, len(tools))
	for i, t := range tools {
		geminiTools[i] = GeminiFunctionDeclaration{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  t.InputSchema,
		}
	}

	// Build request with thinking config
	reqBody := GeminiRequest{
		Contents: []GeminiContent{
			{
				Role:  "user",
				Parts: []GeminiPart{{Text: userPrompt}},
			},
		},
		GenerationConfig: GeminiGenerationConfig{
			Temperature:     types.TemperatureFor(ctx, 1.0),
			MaxOutputTokens: c.maxOutputTokens,
			ThinkingConfig:  c.buildThinkingConfig(),
		},
	}

	if systemPrompt != "" {
		reqBody.SystemInstruction = &GeminiContent{
			Parts: []GeminiPart{{Text: systemPrompt}},
		}
	}

	// CRITICAL: Gemini API cannot combine built-in tools (Google Search, URL Context)
	// with function calling. When we have function declarations, use ONLY those.
	// Built-in tools are available separately via CompleteWithSystem for grounding.
	var allTools []GeminiTool
	if len(geminiTools) > 0 {
		// Function calling mode - NO built-in tools allowed
		allTools = []GeminiTool{{FunctionDeclarations: geminiTools}}
	} else {
		// No function declarations - safe to use built-in tools
		allTools = c.buildBuiltInTools()
	}
	if len(allTools) > 0 {
		reqBody.Tools = allTools
	}

	geminiResp, err := c.postGenerateContent(ctx, reqBody)
	if err != nil {
		return nil, err
	}

	result, err := c.mapToolsResponse(ctx, geminiResp, "CompleteWithTools", len(tools))
	if err != nil {
		return nil, err
	}

	// Log thinking tokens if used
	if geminiResp.UsageMetadata.ThoughtsTokenCount > 0 {
		logging.Perception("[Gemini] CompleteWithTools: completed in %v text_len=%d tool_calls=%d stop_reason=%s thinking_tokens=%d",
			time.Since(startTime), len(result.Text), len(result.ToolCalls), result.StopReason, geminiResp.UsageMetadata.ThoughtsTokenCount)
	} else {
		logging.Perception("[Gemini] CompleteWithTools: completed in %v text_len=%d tool_calls=%d stop_reason=%s",
			time.Since(startTime), len(result.Text), len(result.ToolCalls), result.StopReason)
	}

	return result, nil
}

// CompleteWithToolResults continues a multi-turn function calling conversation,
// satisfying types.ToolResultsProvider: the provider-neutral multi-turn tool
// loop the session executor, broker, and scheduled client all dispatch through.
// Without it the executor degrades Gemini turns to one tool batch whose results
// the model never sees.
//
// The history's ordered blocks map onto Gemini parts in position
// (geminiContentsFromHistory), carrying each thought signature with the call it
// belongs to; tool results go under the "function" role with the real function
// name resolved from the tool_use block that minted the id. A history built
// from legacy flat messages carries no signatures of its own, so the client's
// memory of the last response fills them in.
//
// This changes routing for one configuration. Gemini's default enables Google
// Search and URL Context, and ShouldUsePiggybackTools is true whenever either
// is on, so the default path is unchanged. A Gemini client with both grounding
// options off runs the native multi-turn loop instead of a single
// CompleteWithTools call over a rendered transcript.
func (c *GeminiClient) CompleteWithToolResults(ctx context.Context, systemPrompt string, history []types.Message, tools []types.ToolDefinition) (*LLMToolResponse, error) {
	// Auto-apply timeout if context has no deadline
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.httpClient.Timeout)
		defer cancel()
	}

	startTime := time.Now()
	logging.PerceptionDebug("[Gemini] CompleteWithToolResults: model=%s history=%d prev_thought_sig=%t",
		c.model, len(history), c.lastThoughtSignature != "")

	if c.apiKey == "" {
		return nil, fmt.Errorf("API key not configured")
	}
	// A tool-results call with no results in history fails before any HTTP:
	// there is nothing to answer and no pairing to perform.
	if !historyHasToolResults(history) {
		return nil, fmt.Errorf("gemini tool-results call with no tool results in history")
	}

	allContents, err := geminiContentsFromHistory(history)
	if err != nil {
		return nil, fmt.Errorf("invalid history: %w", err)
	}
	c.applyFallbackThoughtSignatures(allContents)

	// Convert tools to Gemini format
	geminiTools := make([]GeminiFunctionDeclaration, len(tools))
	for i, t := range tools {
		geminiTools[i] = GeminiFunctionDeclaration{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  t.InputSchema,
		}
	}

	// Build request with thought signature for reasoning continuity
	reqBody := GeminiRequest{
		Contents:         allContents,
		ThoughtSignature: c.lastThoughtSignature, // CRITICAL: Pass signature back for Gemini 3
		GenerationConfig: GeminiGenerationConfig{
			Temperature:     types.TemperatureFor(ctx, 1.0),
			MaxOutputTokens: c.maxOutputTokens,
			ThinkingConfig:  c.buildThinkingConfig(),
		},
	}

	if systemPrompt != "" {
		reqBody.SystemInstruction = &GeminiContent{
			Parts: []GeminiPart{{Text: systemPrompt}},
		}
	}

	// CRITICAL: Gemini API cannot combine built-in tools (Google Search, URL Context)
	// with function calling. When we have function declarations, use ONLY those.
	// Built-in tools are available separately via CompleteWithSystem for grounding.
	var allTools []GeminiTool
	if len(geminiTools) > 0 {
		// Function calling mode - NO built-in tools allowed
		allTools = []GeminiTool{{FunctionDeclarations: geminiTools}}
	} else {
		// No function declarations - safe to use built-in tools
		allTools = c.buildBuiltInTools()
	}
	if len(allTools) > 0 {
		reqBody.Tools = allTools
	}

	geminiResp, err := c.postGenerateContent(ctx, reqBody)
	if err != nil {
		return nil, err
	}

	result, err := c.mapToolsResponse(ctx, geminiResp, "CompleteWithToolResults", len(tools))
	if err != nil {
		return nil, err
	}

	logging.Perception("[Gemini] CompleteWithToolResults: completed in %v text_len=%d tool_calls=%d stop_reason=%s",
		time.Since(startTime), len(result.Text), len(result.ToolCalls), result.StopReason)

	return result, nil
}

// historyHasToolResults reports whether any turn carries a tool_result block.
func historyHasToolResults(history []types.Message) bool {
	for _, m := range history {
		for _, b := range m.Content() {
			if b.Kind == types.BlockToolResult {
				return true
			}
		}
	}
	return false
}
