package perception

import (
	"bytes"
	"codenerd/internal/logging"
	"context"
	"encoding/json"
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
func (c *GeminiClient) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (<-chan string, <-chan error) {
	contentChan := make(chan string, 100)
	errorChan := make(chan error, 1)

	// Use the unified streaming engine. Passing nil for the thoughtsChan
	// causes thinking parts to be discarded — preserving the existing
	// 2-channel contract for the 40+ callers/mocks that depend on it.
	go c.runStreamingRequest(ctx, systemPrompt, userPrompt, enableThinking, contentChan, nil, errorChan)
	return contentChan, errorChan
}

// CompleteWithStreamingAndThoughts is the opt-in 3-channel variant of
// CompleteWithStreaming. The model's thinking trace streams on the
// thoughts channel as it arrives, and visible response text streams on
// the content channel — same semantics as the 2-channel version. Both
// channels are closed when the stream ends.
//
// Use this when you want to render the model's reasoning live (TUI
// "thought animation"), or when you need the thought stream for
// observability. Most callers (shards, transducers, verifiers) should
// stick with the 2-channel CompleteWithStreaming since they don't care
// about thinking content.
func (c *GeminiClient) CompleteWithStreamingAndThoughts(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (<-chan string, <-chan string, <-chan error) {
	contentChan := make(chan string, 100)
	thoughtsChan := make(chan string, 100)
	errorChan := make(chan error, 1)

	go c.runStreamingRequest(ctx, systemPrompt, userPrompt, enableThinking, contentChan, thoughtsChan, errorChan)
	return contentChan, thoughtsChan, errorChan
}

// runStreamingRequest is the unified streaming engine for Gemini. It is
// called by both CompleteWithStreaming (2-channel) and
// CompleteWithStreamingAndThoughts (3-channel). When thoughtsChan is
// non-nil, thinking parts are routed to it; when nil, they are dropped.
// Both content and thoughts channels (and the error channel) are closed
// when the stream terminates.
//
// This factoring keeps the streaming retry / scanner / finish-reason
// logging in one place — adding a new public method just means choosing
// what to do with the thinking parts.
func (c *GeminiClient) runStreamingRequest(ctx context.Context, systemPrompt, userPrompt string, _ bool, contentChan, thoughtsChan chan<- string, errorChan chan<- error) {
	defer close(contentChan)
	if thoughtsChan != nil {
		defer close(thoughtsChan)
	}
	defer close(errorChan)

	logging.PerceptionDebug("[Gemini] streaming start: model=%s thoughts_enabled=%v", c.model, thoughtsChan != nil)

	// Auto-apply timeout if context has no deadline (centralized timeout handling)
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.httpClient.Timeout)
		defer cancel()
	}

	startTime := time.Now()

	if c.apiKey == "" {
		logging.PerceptionError("[Gemini] CompleteWithStreaming: API key not configured")
		errorChan <- fmt.Errorf("API key not configured")
		return
	}

	if strings.TrimSpace(systemPrompt) == "" {
		systemPrompt = defaultSystemPrompt
	}

	isPiggyback := strings.Contains(systemPrompt, "control_packet") ||
		strings.Contains(systemPrompt, "surface_response") ||
		strings.Contains(userPrompt, "PiggybackEnvelope") ||
		strings.Contains(userPrompt, "control_packet")

	// Rate limiting
	c.mu.Lock()
	elapsed := time.Since(c.lastRequest)
	if elapsed < 100*time.Millisecond {
		time.Sleep(100*time.Millisecond - elapsed)
	}
	c.lastRequest = time.Now()
	c.mu.Unlock()

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
	}

	url := fmt.Sprintf("%s/models/%s:streamGenerateContent?alt=sse&key=%s", c.baseURL, c.model, c.apiKey)

	maxRetries := 3
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(1<<uint(attempt-1)) * time.Second)
		}

		jsonData, err := json.Marshal(reqBody)
		if err != nil {
			errorChan <- fmt.Errorf("failed to marshal request: %w", err)
			return
		}

		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
		if err != nil {
			errorChan <- fmt.Errorf("failed to create request: %w", err)
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

		if isTransientGeminiStatus(resp.StatusCode) {
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

			errorChan <- fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
			return
		}

		scanner, releaseScanner := newPooledScanner(resp.Body, 1024*1024)
		defer releaseScanner()

		scanDone := make(chan struct{})
		scanErrChan := make(chan error, 1)

		// Track the last observed finish_reason and usage metadata so
		// we can log them after the stream ends. Previously these were
		// silently dropped, which made truncation impossible to
		// diagnose — the user saw a half-finished envelope and we
		// had no signal explaining why generation stopped.
		var (
			lastFinishReason string
			lastUsage        struct {
				promptTokens   int
				outputTokens   int
				thoughtsTokens int
				totalTokens    int
				cachedTokens   int
			}
			outputChars int
		)

		go func() {
			defer close(scanDone)
			for scanner.Scan() {
				line := scanner.Text()
				if !strings.HasPrefix(line, "data:") {
					continue
				}
				data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
				if data == "" {
					continue
				}
				if data == "[DONE]" {
					return
				}

				var chunk GeminiResponse
				if err := json.Unmarshal([]byte(data), &chunk); err != nil {
					continue
				}
				if chunk.Error != nil {
					scanErrChan <- fmt.Errorf("API error: %s", chunk.Error.Message)
					return
				}
				if chunk.ThoughtSignature != "" {
					c.lastThoughtSignature = chunk.ThoughtSignature
				}
				// Pick up usage metadata whenever the chunk carries it
				// (typically the final chunk).
				if chunk.UsageMetadata.TotalTokenCount > 0 ||
					chunk.UsageMetadata.CandidatesTokenCount > 0 ||
					chunk.UsageMetadata.ThoughtsTokenCount > 0 {
					lastUsage.promptTokens = chunk.UsageMetadata.PromptTokenCount
					lastUsage.outputTokens = chunk.UsageMetadata.CandidatesTokenCount
					lastUsage.thoughtsTokens = chunk.UsageMetadata.ThoughtsTokenCount
					lastUsage.totalTokens = chunk.UsageMetadata.TotalTokenCount
					lastUsage.cachedTokens = chunk.UsageMetadata.CachedContentTokenCount
				}
				if len(chunk.Candidates) == 0 {
					continue
				}
				if chunk.Candidates[0].FinishReason != "" {
					lastFinishReason = chunk.Candidates[0].FinishReason
				}
				for _, part := range chunk.Candidates[0].Content.Parts {
					if part.FunctionCall != nil && part.FunctionCall.ThoughtSignature != "" {
						c.lastThoughtSignature = part.FunctionCall.ThoughtSignature
					}
					if part.ThoughtSignature != "" {
						c.lastThoughtSignature = part.ThoughtSignature
					}
					if part.Thought {
						// Thinking parts MUST NOT go to contentChan —
						// mixing them with the visible-output stream
						// corrupts Piggyback JSON parsing on the
						// downstream StreamParser (which only knows how
						// to extract surface_response from clean JSON).
						//
						// When the caller wants thoughts, we route them
						// to a dedicated thoughtsChan; otherwise we
						// drop them, preserving the old 2-channel
						// contract.
						if thoughtsChan != nil && part.Text != "" {
							select {
							case thoughtsChan <- part.Text:
							case <-ctx.Done():
								return
							}
						}
						continue
					}
					if part.Text == "" {
						continue
					}
					outputChars += len(part.Text)
					select {
					case contentChan <- part.Text:
					case <-ctx.Done():
						return
					}
				}
			}
			if err := scanner.Err(); err != nil {
				scanErrChan <- err
			}
		}()

		select {
		case <-scanDone:
			resp.Body.Close()
			select {
			case err := <-scanErrChan:
				logging.PerceptionError("[Gemini] CompleteWithStreaming: stream error after %v: %v", time.Since(startTime), err)
				errorChan <- fmt.Errorf("stream error: %w", err)
			default:
				// Surface finish_reason and usage so truncation is
				// immediately diagnosable. A MAX_TOKENS finish on a
				// short response is the smoking gun for a thinking
				// budget that ate the output.
				if lastFinishReason != "" && lastFinishReason != "STOP" {
					logging.Get(logging.CategoryPerception).Warn(
						"[Gemini] CompleteWithStreaming: completed in %v finish=%s output_chars=%d output_tokens=%d thoughts_tokens=%d total_tokens=%d prompt_tokens=%d cached=%d max_output_tokens=%d (response likely truncated)",
						time.Since(startTime),
						lastFinishReason,
						outputChars,
						lastUsage.outputTokens,
						lastUsage.thoughtsTokens,
						lastUsage.totalTokens,
						lastUsage.promptTokens,
						lastUsage.cachedTokens,
						c.maxOutputTokens,
					)
				} else {
					logging.Perception("[Gemini] CompleteWithStreaming: completed in %v finish=%s output_chars=%d output_tokens=%d thoughts_tokens=%d total_tokens=%d cached=%d",
						time.Since(startTime),
						lastFinishReason,
						outputChars,
						lastUsage.outputTokens,
						lastUsage.thoughtsTokens,
						lastUsage.totalTokens,
						lastUsage.cachedTokens,
					)
				}
			}
		case <-ctx.Done():
			resp.Body.Close()
			<-scanDone
			logging.PerceptionWarn("[Gemini] CompleteWithStreaming: cancelled after %v finish=%s output_chars=%d", time.Since(startTime), lastFinishReason, outputChars)
			errorChan <- ctx.Err()
		}
		return
	}

	logging.PerceptionError("[Gemini] CompleteWithStreaming: max retries exceeded after %v: %v", time.Since(startTime), lastErr)
	errorChan <- fmt.Errorf("max retries exceeded: %w", lastErr)
}

// SetModel changes the model used for completions.
