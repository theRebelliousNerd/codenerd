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

// CompleteWithToolResults continues a multi-turn function calling conversation.
// This is used after the model returns tool calls - we execute the tools and
// pass the results back along with the thought signature for reasoning continuity.
func (c *GeminiClient) CompleteWithToolResults(ctx context.Context, systemPrompt string, contents []GeminiContent, toolResults []ToolResult, tools []ToolDefinition) (*LLMToolResponse, error) {
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

	// Append the tool results as a function role message
	allContents := append(contents, GeminiContent{
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
