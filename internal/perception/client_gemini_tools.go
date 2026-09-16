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
		var textBuilder strings.Builder
		for _, part := range geminiResp.Candidates[0].Content.Parts {
			if part.Text != "" {
				textBuilder.WriteString(part.Text)
			}
			if part.FunctionCall != nil {
				result.ToolCalls = append(result.ToolCalls, ToolCall{
					ID:    fmt.Sprintf("call_%d", len(result.ToolCalls)),
					Name:  part.FunctionCall.Name,
					Input: part.FunctionCall.Args,
				})
			}
		}
		result.Text = strings.TrimSpace(textBuilder.String())

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

// completeWithToolResultsNative continues a multi-turn function calling
// conversation in Gemini-native terms: typed contents plus thought
// signatures. This is the engine behind the public ToolResultsProvider
// adapter below, which translates provider-neutral history into these
// terms; it is private because no caller speaks GeminiContent directly.
func (c *GeminiClient) completeWithToolResultsNative(ctx context.Context, systemPrompt string, contents []GeminiContent, toolResults []ToolResult, tools []ToolDefinition) (*LLMToolResponse, error) {
	// Auto-apply timeout if context has no deadline
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.httpClient.Timeout)
		defer cancel()
	}

	startTime := time.Now()
	logging.PerceptionDebug("[Gemini] CompleteWithToolResults: model=%s tool_results=%d prev_thought_sig=%t",
		c.model, len(toolResults), c.lastThoughtSignature != "")

	if c.apiKey == "" {
		return nil, fmt.Errorf("API key not configured")
	}

	// Build tool result parts (preserve Gemini 3 thought signature positions)
	resultParts := make([]GeminiPart, 0, len(toolResults))
	if len(c.lastToolCalls) > 0 {
		resultsByID := make(map[string]ToolResult, len(toolResults))
		for _, tr := range toolResults {
			resultsByID[tr.ToolUseID] = tr
		}
		for _, call := range c.lastToolCalls {
			tr, ok := resultsByID[call.id]
			if !ok {
				logging.PerceptionWarn("[Gemini] CompleteWithToolResults: missing tool result for %s", call.id)
				continue
			}
			part := GeminiPart{
				FunctionResponse: &GeminiFunctionResponse{
					Name: call.name,
					Response: map[string]any{
						"content":  tr.Content,
						"is_error": tr.IsError,
					},
				},
			}
			signature := call.signature
			if signature == "" {
				signature = c.lastThoughtSignature
			}
			if signature != "" {
				part.ThoughtSignature = signature
			}
			resultParts = append(resultParts, part)
		}
	} else {
		for _, tr := range toolResults {
			resultParts = append(resultParts, GeminiPart{
				FunctionResponse: &GeminiFunctionResponse{
					Name: tr.ToolUseID,
					Response: map[string]any{
						"content":  tr.Content,
						"is_error": tr.IsError,
					},
				},
			})
		}
	}

	// Append the tool results as a function role message. Copy first: a bare
	// append can write into the caller's backing array when the slice has
	// spare capacity, corrupting history the caller reuses.
	allContents := make([]GeminiContent, 0, len(contents)+1)
	allContents = append(allContents, contents...)
	allContents = append(allContents, GeminiContent{
		Role:  "function",
		Parts: resultParts,
	})

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

// CompleteWithToolResults implements types.ToolResultsProvider: the
// provider-neutral multi-turn tool loop the session executor, broker, and
// scheduled client all dispatch through.
//
// Gemini's wire format carries conversation state as typed contents
// (user/model/function roles) plus thought signatures, so this translates
// the neutral history, replays the last assistant turn's tool calls into
// the pairing state, and delegates to the native engine. Without this
// adapter the executor degrades Gemini turns to one tool batch whose
// results the model never sees.
func (c *GeminiClient) CompleteWithToolResults(ctx context.Context, systemPrompt string, history []types.Message, tools []types.ToolDefinition) (*LLMToolResponse, error) {
	// Index assistant tool calls by ID so every function response carries
	// the real function name even for earlier rounds, and find the latest
	// tool-result turn (the native engine appends exactly that one).
	nameByID := make(map[string]string)
	lastResults := -1
	for i, m := range history {
		for _, call := range m.ToolCalls {
			if call.ID != "" && call.Name != "" {
				nameByID[call.ID] = call.Name
			}
		}
		if m.Role == "user" && len(m.ToolResults) > 0 {
			lastResults = i
		}
	}

	var contents []GeminiContent
	var lastCalls []types.ToolCall
	var results []ToolResult
	for i, m := range history {
		switch {
		case m.Role == "assistant":
			var parts []GeminiPart
			if strings.TrimSpace(m.Text) != "" {
				parts = append(parts, GeminiPart{Text: m.Text})
			}
			for _, call := range m.ToolCalls {
				parts = append(parts, GeminiPart{
					FunctionCall: &GeminiFunctionCall{Name: call.Name, Args: call.Input},
				})
			}
			if len(m.ToolCalls) > 0 {
				lastCalls = m.ToolCalls
			}
			if len(parts) > 0 {
				contents = append(contents, GeminiContent{Role: "model", Parts: parts})
			}
		case m.Role == "user" && len(m.ToolResults) > 0 && i == lastResults:
			for _, tr := range m.ToolResults {
				results = append(results, ToolResult{ToolUseID: tr.ToolUseID, Content: tr.Content, IsError: tr.IsError})
			}
		case m.Role == "user" && len(m.ToolResults) > 0:
			var parts []GeminiPart
			for _, tr := range m.ToolResults {
				name := nameByID[tr.ToolUseID]
				if name == "" {
					name = tr.ToolUseID
				}
				parts = append(parts, GeminiPart{
					FunctionResponse: &GeminiFunctionResponse{
						Name:     name,
						Response: map[string]any{"content": tr.Content, "is_error": tr.IsError},
					},
				})
			}
			contents = append(contents, GeminiContent{Role: "function", Parts: parts})
		default:
			if strings.TrimSpace(m.Text) != "" {
				contents = append(contents, GeminiContent{Role: "user", Parts: []GeminiPart{{Text: m.Text}}})
			}
		}
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("gemini tool-results call with no tool results in history")
	}

	// Replay the pairing state the native engine matches results against.
	c.lastToolCalls = make([]geminiToolCall, 0, len(lastCalls))
	for _, call := range lastCalls {
		c.lastToolCalls = append(c.lastToolCalls, geminiToolCall{id: call.ID, name: call.Name})
	}
	return c.completeWithToolResultsNative(ctx, systemPrompt, contents, results, tools)
}
