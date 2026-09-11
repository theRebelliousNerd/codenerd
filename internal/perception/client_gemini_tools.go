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
	"time"
)

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
			Temperature:     1.0,
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

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", c.baseURL, c.model, c.apiKey)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		logging.PerceptionError("[Gemini] CompleteWithTools: request failed after %v: %v", time.Since(startTime), err)
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		logging.PerceptionError("[Gemini] CompleteWithTools: API returned status %d: %s", resp.StatusCode, string(body))
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var geminiResp GeminiResponse
	if err := json.Unmarshal(body, &geminiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if geminiResp.Error != nil {
		return nil, fmt.Errorf("API error: %s", geminiResp.Error.Message)
	}

	// Capture thought signatures/tool calls for Gemini 3 multi-turn continuity.
	c.captureThoughtSignature(&geminiResp)
	c.lastToolCalls = c.extractToolCalls(&geminiResp)
	c.lastThoughtSummary = geminiResp.ThoughtSummary
	c.lastThinkingTokens = geminiResp.UsageMetadata.ThoughtsTokenCount

	// Parse response content into text and tool calls
	result := &LLMToolResponse{}

	// Populate thinking metadata for learning and improvement
	if geminiResp.ThoughtSummary != "" {
		result.ThoughtSummary = geminiResp.ThoughtSummary
	}
	if c.lastThoughtSignature != "" {
		result.ThoughtSignature = c.lastThoughtSignature
	}

	// Map usage metadata
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
		usageOpFor(len(tools)))

	if len(geminiResp.Candidates) > 0 {
		result.StopReason = geminiResp.Candidates[0].FinishReason
		// Refuse before building: a function call cut mid-arguments must not
		// reach the executor, and applyGeminiBlocks would happily assemble one.
		if types.LengthStop(result.StopReason) {
			return nil, outputTruncated(ProviderGemini, c.model, "CompleteWithTools", result.StopReason, "",
				c.maxOutputTokens, geminiResp.UsageMetadata.CandidatesTokenCount)
		}
		// Replaces main's manual text/function-call walk: applyGeminiBlocks
		// reads the same parts in order, keeping per-part thought signatures
		// and filling Text and ToolCalls as the flat projection of them.
		applyGeminiBlocks(result, &geminiResp)

		// Extract grounding sources for transparency and learning
		if geminiResp.Candidates[0].GroundingMetadata != nil {
			gm := geminiResp.Candidates[0].GroundingMetadata
			if len(gm.GroundingChunks) > 0 {
				for _, chunk := range gm.GroundingChunks {
					if chunk.Web != nil && chunk.Web.URI != "" {
						result.GroundingSources = append(result.GroundingSources, chunk.Web.URI)
					}
				}
				logging.PerceptionDebug("[Gemini] CompleteWithTools: grounding sources=%d queries=%v",
					len(gm.GroundingChunks), gm.WebSearchQueries)
			}
		}
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
// satisfying types.ToolResultsProvider.
//
// It previously took a pre-built []GeminiContent plus a loose []ToolResult and
// had no caller anywhere in the tree, so Gemini fell through to the
// single-turn path with the conversation flattened into a text transcript.
// Taking the neutral history is what makes the native loop reachable: the
// history's ordered blocks map onto Gemini parts in position, carrying each
// thought signature with the call it belongs to.
//
// This changes routing for one configuration. Gemini's default enables Google
// Search and URL Context, and ShouldUsePiggybackTools is true whenever either
// is on, so the default path is unchanged. A Gemini client with both grounding
// options off now runs the native multi-turn loop instead of a single
// CompleteWithTools call over a rendered transcript.
func (c *GeminiClient) CompleteWithToolResults(ctx context.Context, systemPrompt string, history []types.Message, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
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
	if len(history) == 0 {
		return nil, fmt.Errorf("history must contain at least one message")
	}

	allContents, err := geminiContentsFromHistory(history)
	if err != nil {
		return nil, fmt.Errorf("invalid history: %w", err)
	}
	// A history assembled from legacy flat messages carries no signatures of
	// its own. Fall back to the one the last response left on the client, which
	// is where this path used to get every signature it sent.
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
			Temperature:     1.0,
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

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", c.baseURL, c.model, c.apiKey)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		logging.PerceptionError("[Gemini] CompleteWithToolResults: request failed after %v: %v", time.Since(startTime), err)
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		logging.PerceptionError("[Gemini] CompleteWithToolResults: API returned status %d: %s", resp.StatusCode, string(body))
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var geminiResp GeminiResponse
	if err := json.Unmarshal(body, &geminiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if geminiResp.Error != nil {
		return nil, fmt.Errorf("API error: %s", geminiResp.Error.Message)
	}

	// Update thought signatures/tool calls for next turn
	c.captureThoughtSignature(&geminiResp)
	c.lastToolCalls = c.extractToolCalls(&geminiResp)
	c.lastThoughtSummary = geminiResp.ThoughtSummary
	c.lastThinkingTokens = geminiResp.UsageMetadata.ThoughtsTokenCount

	// Parse response
	result := &LLMToolResponse{}

	// Populate thinking metadata
	if geminiResp.ThoughtSummary != "" {
		result.ThoughtSummary = geminiResp.ThoughtSummary
	}
	if c.lastThoughtSignature != "" {
		result.ThoughtSignature = c.lastThoughtSignature
	}

	// Map usage metadata
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
		usageOpFor(len(tools)))

	if len(geminiResp.Candidates) > 0 {
		result.StopReason = geminiResp.Candidates[0].FinishReason
		// Refuse before building: a function call cut mid-arguments must not
		// reach the executor, and applyGeminiBlocks would happily assemble one.
		if types.LengthStop(result.StopReason) {
			return nil, outputTruncated(ProviderGemini, c.model, "CompleteWithToolResults", result.StopReason, "",
				c.maxOutputTokens, geminiResp.UsageMetadata.CandidatesTokenCount)
		}
		// Replaces main's manual text/function-call walk: applyGeminiBlocks
		// reads the same parts in order, keeping per-part thought signatures
		// and filling Text and ToolCalls as the flat projection of them.
		applyGeminiBlocks(result, &geminiResp)

		// Extract grounding sources
		if geminiResp.Candidates[0].GroundingMetadata != nil {
			gm := geminiResp.Candidates[0].GroundingMetadata
			for _, chunk := range gm.GroundingChunks {
				if chunk.Web != nil && chunk.Web.URI != "" {
					result.GroundingSources = append(result.GroundingSources, chunk.Web.URI)
				}
			}
		}
	}

	logging.Perception("[Gemini] CompleteWithToolResults: completed in %v text_len=%d tool_calls=%d stop_reason=%s",
		time.Since(startTime), len(result.Text), len(result.ToolCalls), result.StopReason)

	return result, nil
}
