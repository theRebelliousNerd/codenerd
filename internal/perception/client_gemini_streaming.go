package perception

import (
	"bytes"
	"codenerd/internal/logging"
	"codenerd/internal/types"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// CompleteWithStreaming sends a prompt with streaming enabled.
// Returns channels of incremental content deltas. Thinking parts are
// dropped silently — see CompleteWithStreamingAndThoughts when you want
// to surface the model's reasoning trace separately (e.g. for the TUI).
func (c *GeminiClient) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (<-chan string, <-chan types.StreamResult) {
	w, content, done := types.NewStream()
	go c.runStreamingRequest(ctx, systemPrompt, userPrompt, enableThinking, w)
	return content, done
}

// CompleteWithStreamingAndThoughts is the opt-in variant of
// CompleteWithStreaming that also streams the model's thinking trace on a
// thoughts channel as it arrives. Visible response text streams on the
// content channel with the same semantics as the plain version.
//
// Use this when you want to render the model's reasoning live (TUI
// "thought animation"), or when you need the thought stream for
// observability. Most callers (shards, transducers, verifiers) should
// stick with CompleteWithStreaming since they don't care about thinking
// content.
func (c *GeminiClient) CompleteWithStreamingAndThoughts(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (<-chan string, <-chan string, <-chan types.StreamResult) {
	w, content, thoughts, done := types.NewThoughtStream()
	go c.runStreamingRequest(ctx, systemPrompt, userPrompt, enableThinking, w)
	return content, thoughts, done
}

// runStreamingRequest is the unified streaming engine for Gemini, shared by
// both public methods. Thinking parts go to w's thoughts channel when the
// writer has one and are dropped otherwise; they never reach content.
// geminiSystemInstruction is the systemInstruction of a single-turn request.
// The caller owns the system prompt: a blank one sends none (the field is
// omitzero), and no client substitutes text of its own.
func geminiSystemInstruction(systemPrompt string) *GeminiContent {
	if strings.TrimSpace(systemPrompt) == "" {
		return nil
	}
	return &GeminiContent{Parts: []GeminiPart{{Text: systemPrompt}}}
}

func (c *GeminiClient) runStreamingRequest(ctx context.Context, systemPrompt, userPrompt string, _ bool, w *types.StreamWriter) {
	defer w.Close()

	logging.PerceptionDebug("[Gemini] streaming start: model=%s thoughts_enabled=%v", c.model, w.HasThoughts())

	// Auto-apply timeout if context has no deadline (centralized timeout handling)
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.httpClient.Timeout)
		defer cancel()
	}

	startTime := time.Now()

	if c.apiKey == "" {
		logging.PerceptionError("[Gemini] CompleteWithStreaming: API key not configured")
		w.Fail(fmt.Errorf("API key not configured"))
		return
	}

	isPiggyback := strings.Contains(systemPrompt, "control_packet") ||
		strings.Contains(systemPrompt, "surface_response") ||
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
		SystemInstruction: geminiSystemInstruction(systemPrompt),
		GenerationConfig: GeminiGenerationConfig{
			Temperature:     types.TemperatureFor(ctx, 1.0),
			MaxOutputTokens: c.maxOutputTokens,
			ThinkingConfig:  c.buildThinkingConfig(),
		},
		Tools: c.buildBuiltInTools(),
	}
	if isPiggyback {
		reqBody.GenerationConfig.ResponseMimeType = "application/json"
		reqBody.GenerationConfig.ResponseSchema = BuildGeminiPiggybackEnvelopeSchema()
	}

	url := fmt.Sprintf("%s/models/%s:streamGenerateContent?alt=sse&key=%s", c.baseURL, c.model, c.apiKey)

	maxRetries := llmMaxRetries()
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Context-aware backoff: a cancelled turn must exit during
			// the sleep, not after it (matches ExecuteOpenAIRequest).
			backoff := llmRetryBackoff(attempt)
			select {
			case <-ctx.Done():
				w.Fail(fmt.Errorf("request cancelled during retry backoff: %w", ctx.Err()))
				return
			case <-time.After(backoff):
			}
		}

		jsonData, err := json.Marshal(reqBody)
		if err != nil {
			w.Fail(fmt.Errorf("failed to marshal request: %w", err))
			return
		}

		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
		if err != nil {
			w.Fail(fmt.Errorf("failed to create request: %w", err))
			return
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "text/event-stream")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("request failed: %w", err)
			continue
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
			resp.Body.Close()
			lastErr = fmt.Errorf("rate limit exceeded (429): %s", strings.TrimSpace(string(body)))
			continue
		}

		if isTransientHTTPStatus(resp.StatusCode) {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
			resp.Body.Close()
			// Wrap the sentinel so errors.Is(err, ErrLLMUnavailable) holds up the
			// whole chain (through the post-loop "max retries exceeded: %w"). This
			// lets the perception firewall report /llm_unavailable rather than
			// laundering a transient 503 into a "you were unclear" clarification.
			lastErr = fmt.Errorf("transient server error (%d): %s: %w", resp.StatusCode, strings.TrimSpace(string(body)), ErrLLMUnavailable)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
			resp.Body.Close()

			// Some models may reject responseJsonSchema; retry once without it.
			if isPiggyback && reqBody.GenerationConfig.ResponseSchema != nil && resp.StatusCode == http.StatusBadRequest {
				bodyStr := string(body)
				bodyLower := strings.ToLower(bodyStr)
				if strings.Contains(bodyLower, "responsejsonschema") || strings.Contains(bodyLower, "responsemimetype") ||
					strings.Contains(bodyLower, "response_schema") || strings.Contains(bodyLower, "response_mime_type") ||
					strings.Contains(bodyLower, "responseschema") {
					reqBody.GenerationConfig.ResponseSchema = nil
					reqBody.GenerationConfig.ResponseMimeType = ""
					lastErr = fmt.Errorf("request rejected structured output, retrying without responseJsonSchema: %s", bodyStr)
					continue
				}
			}

			w.Fail(fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body)))
			return
		}

		res := c.readGeminiStream(ctx, resp.Body, w)
		resp.Body.Close()
		reportUsage(ctx, c.model, ProviderGemini, usageOpChat, res.Usage)
		logStreamEnd("Gemini", startTime, res)
		w.Finish(res)
		return
	}

	logging.PerceptionError("[Gemini] CompleteWithStreaming: max retries exceeded after %v: %v", time.Since(startTime), lastErr)
	w.Fail(fmt.Errorf("max retries exceeded: %w", lastErr))
}

// geminiStreamChunk is one streamGenerateContent event. promptFeedback is
// where Gemini reports a prompt it blocked before generating anything.
type geminiStreamChunk struct {
	GeminiResponse
	PromptFeedback *struct {
		BlockReason string `json:"blockReason"`
	} `json:"promptFeedback"`
}

// readGeminiStream consumes a streamGenerateContent?alt=sse body, forwarding
// visible text to w and thinking parts to w's thoughts channel, and returns
// the terminal result without publishing it.
//
// The protocol's terminal evidence is the candidate's finishReason. A missing
// one at EOF is an incomplete stream; a non-STOP reason used to reach the
// caller only as a warning in the log while the stream reported success.
// usageMetadata snapshots are cumulative; the last one is the bill.
func (c *GeminiClient) readGeminiStream(ctx context.Context, body io.Reader, w *types.StreamWriter) types.StreamResult {
	r := newSSEReader(body)
	defer r.Close()

	end := streamEnd{provider: ProviderGemini, model: c.model, limit: c.maxOutputTokens}
	var text strings.Builder
	var blocked string
	readErr := func() error {
		for {
			ev, err := r.Next()
			if errors.Is(err, io.EOF) {
				return nil
			}
			if err != nil {
				return transportError(ctx, ProviderGemini, c.model, err)
			}
			data := strings.TrimSpace(ev.Data)
			if data == "" {
				continue
			}
			if data == "[DONE]" {
				// Not a Gemini event, but some proxies append the OpenAI
				// sentinel. It ends the events; it is not terminal evidence.
				return nil
			}
			var chunk geminiStreamChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				return protocolError(ProviderGemini, c.model, "malformed stream event", err)
			}
			if chunk.Error != nil {
				return providerStreamError(ProviderGemini, c.model, chunk.Error.Message)
			}
			if chunk.PromptFeedback != nil && chunk.PromptFeedback.BlockReason != "" {
				blocked = chunk.PromptFeedback.BlockReason
			}
			c.captureStreamSignature(&chunk.GeminiResponse)
			if u := chunk.UsageMetadata; u.TotalTokenCount > 0 || u.CandidatesTokenCount > 0 || u.thoughts() > 0 {
				end.usage = u.metadata()
				end.usageSeen = true
			}
			if len(chunk.Candidates) == 0 {
				continue
			}
			if len(chunk.Candidates) > 1 {
				return protocolError(ProviderGemini, c.model, "multiple candidates in a single-candidate stream", nil)
			}
			cand := chunk.Candidates[0]
			for _, part := range cand.Content.Parts {
				if part.Thought {
					// Thinking parts MUST NOT reach content: mixing them into
					// the visible stream corrupts Piggyback JSON parsing.
					if err := w.Thought(ctx, part.Text); err != nil {
						return err
					}
					continue
				}
				if part.Text == "" {
					continue
				}
				text.WriteString(part.Text)
				if err := w.Content(ctx, part.Text); err != nil {
					return err
				}
			}
			if cand.FinishReason != "" {
				end.finish = cand.FinishReason
			}
		}
	}()
	end.text = text.String()
	if readErr == nil && end.finish == "" && blocked != "" {
		end.finish = blocked
		return end.result(ctx, nil, func(e streamEnd) error {
			return e.failure(types.StreamFiltered, "prompt blocked before generation", nil)
		})
	}
	return end.result(ctx, readErr, func(e streamEnd) error {
		if types.LengthStop(e.finish) {
			return outputTruncated(ProviderGemini, c.model, streamMethod, e.finish, e.text, c.maxOutputTokens, e.usage.OutputTokens)
		}
		switch e.finish {
		case "STOP":
			return e.emptyStream(ctx)
		case "SAFETY", "RECITATION", "BLOCKLIST", "PROHIBITED_CONTENT", "SPII", "IMAGE_SAFETY":
			return e.failure(types.StreamFiltered, "provider safety filter stopped generation", nil)
		default:
			// MALFORMED_FUNCTION_CALL, UNEXPECTED_TOOL_CALL, LANGUAGE, OTHER,
			// FINISH_REASON_UNSPECIFIED, and any reason added later: not a
			// completed text answer, and never success by default.
			return e.failure(types.StreamUnsupported, "finish reason is not a completed text answer", nil)
		}
	})
}

// captureStreamSignature records the latest thought signature a streamed
// chunk carried, for Gemini 3 multi-turn continuity.
func (c *GeminiClient) captureStreamSignature(resp *GeminiResponse) {
	if resp.ThoughtSignature != "" {
		c.lastThoughtSignature = resp.ThoughtSignature
	}
	if len(resp.Candidates) == 0 {
		return
	}
	for _, part := range resp.Candidates[0].Content.Parts {
		if part.FunctionCall != nil && part.FunctionCall.ThoughtSignature != "" {
			c.lastThoughtSignature = part.FunctionCall.ThoughtSignature
		}
		if part.ThoughtSignature != "" {
			c.lastThoughtSignature = part.ThoughtSignature
		}
	}
}
