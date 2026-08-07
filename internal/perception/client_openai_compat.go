package perception

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// OpenAICompatClient implements LLMClient for vendors that speak the OpenAI
// Chat Completions wire format at their own base URL.
//
// Three vendors share this implementation:
//
//	dashscope — Alibaba Model Studio (Qwen), https://dashscope-intl.aliyuncs.com/compatible-mode/v1
//	meta      — Meta Model API (Muse Spark), https://api.meta.ai/v1
//	moonshot  — Moonshot AI (Kimi),          https://api.moonshot.ai/v1
//
// They are NOT interchangeable at the field level, which is why this client
// keeps a vendor discriminator rather than pretending one payload fits all:
//
//   - DashScope gates reasoning behind enable_thinking / thinking_budget and
//     returns the trace in a separate reasoning_content field.
//   - Meta gates reasoning behind reasoning_effort, rejects "none" with HTTP 400,
//     deprecates max_tokens in favour of max_completion_tokens, and is tuned to
//     run at default sampling — so temperature and top_p are left unset.
//
// Adding a fourth OpenAI-compatible vendor needs only a vendorDefaults entry
// unless it has its own field quirks.
type OpenAICompatClient struct {
	vendor     Provider
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client

	maxOutputTokens int
	temperature     float64
	topP            float64

	// enableThinking is the vendor-neutral "reason before answering" switch.
	// It maps to enable_thinking on DashScope and to reasoning_effort on Meta.
	enableThinking bool
	thinkingBudget int

	// reasoningEffort* map codeNERD's per-shard capability tiers onto the
	// vendor's effort scale. Consulted only when the request context carries a
	// types.CtxKeyModelCapability hint, so the vendor default stands otherwise.
	reasoningEffortHighReasoning string
	reasoningEffortBalanced      string
	reasoningEffortHighSpeed     string

	mu          sync.Mutex
	lastRequest time.Time
}

// OpenAICompatConfig configures an OpenAI-compatible client.
type OpenAICompatConfig struct {
	Vendor          Provider
	APIKey          string
	BaseURL         string
	Model           string
	Timeout         time.Duration
	MaxOutputTokens int
	Temperature     float64
	TopP            float64
	EnableThinking  bool
	ThinkingBudget  int
}

// vendorDefault describes a vendor's endpoint and preferred model.
type vendorDefault struct {
	baseURL string
	model   string
}

var openAICompatVendorDefaults = map[Provider]vendorDefault{
	ProviderDashScope: {
		baseURL: "https://dashscope-intl.aliyuncs.com/compatible-mode/v1",
		model:   "qwen3.8-max",
	},
	ProviderMeta: {
		baseURL: "https://api.meta.ai/v1",
		model:   "muse-spark-1.2",
	},
	ProviderMoonshot: {
		baseURL: "https://api.moonshot.ai/v1",
		model:   "kimi-k3",
	},
}

// IsOpenAICompatProvider reports whether a provider is served by this client.
func IsOpenAICompatProvider(p Provider) bool {
	_, ok := openAICompatVendorDefaults[p]
	return ok
}

// minCompletionTokensFor returns the smallest completion budget a vendor needs
// to produce visible output. Reasoning models bill thinking against the same
// budget, so anything below this returns empty content rather than short
// content. Zero means no floor.
func minCompletionTokensFor(vendor Provider) int {
	switch vendor {
	case ProviderMeta:
		// Empirically: 256 -> empty response, 4096 -> normal output.
		return 4096
	default:
		return 0
	}
}

// DefaultOpenAICompatConfig returns defaults for a vendor. An unknown vendor
// yields an empty base URL, which NewOpenAICompatClient rejects — callers must
// supply BaseURL explicitly for vendors codeNERD does not know by name.
func DefaultOpenAICompatConfig(vendor Provider, apiKey string) OpenAICompatConfig {
	d := openAICompatVendorDefaults[vendor]
	cfg := OpenAICompatConfig{
		Vendor:  vendor,
		APIKey:  apiKey,
		BaseURL: d.baseURL,
		Model:   d.model,
		// Million-token-context models routinely exceed a short client timeout.
		Timeout:         10 * time.Minute,
		MaxOutputTokens: 16384,
	}

	switch vendor {
	case ProviderDashScope:
		// Qwen reasoning mode is the reason this tier was chosen; Alibaba
		// recommends these sampling values whenever thinking is enabled.
		cfg.EnableThinking = true
		cfg.Temperature = 0.6
		cfg.TopP = 0.95
	case ProviderMeta:
		// Muse Spark is explicitly tuned for default sampling and warns against
		// setting temperature and top_p together, so both stay unset (0 =>
		// omitted from the payload).
		cfg.EnableThinking = true
	}

	return cfg
}

// NewOpenAICompatClient builds a client for an OpenAI-compatible vendor.
func NewOpenAICompatClient(cfg OpenAICompatConfig) (*OpenAICompatClient, error) {
	if strings.TrimSpace(cfg.BaseURL) == "" {
		return nil, fmt.Errorf("openai-compatible provider %q requires a base_url", cfg.Vendor)
	}
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, fmt.Errorf("openai-compatible provider %q requires an API key", cfg.Vendor)
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 10 * time.Minute
	}
	if cfg.MaxOutputTokens <= 0 {
		cfg.MaxOutputTokens = 16384
	}
	if cfg.Model == "" {
		cfg.Model = openAICompatVendorDefaults[cfg.Vendor].model
	}

	// Reasoning models spend the completion budget on thinking before emitting
	// any visible content, so a small ceiling yields an EMPTY response rather
	// than a truncated one. Verified against Muse Spark: max_completion_tokens
	// 256 returns a single frame with an empty delta and — misleadingly —
	// finish_reason "stop" rather than "length"; 4096 returns normally.
	//
	// Clamp instead of failing: a caller asking for a small budget wants a short
	// answer, not no answer.
	if minTokens := minCompletionTokensFor(cfg.Vendor); cfg.MaxOutputTokens < minTokens {
		logging.PerceptionWarn("[%s] max_output_tokens %d is below the %d floor this reasoning model needs to emit visible content; clamping",
			cfg.Vendor, cfg.MaxOutputTokens, minTokens)
		cfg.MaxOutputTokens = minTokens
	}

	c := &OpenAICompatClient{
		vendor:          cfg.Vendor,
		apiKey:          cfg.APIKey,
		baseURL:         strings.TrimRight(cfg.BaseURL, "/"),
		model:           cfg.Model,
		httpClient:      NewSharedHTTPClient(cfg.Timeout),
		maxOutputTokens: cfg.MaxOutputTokens,
		temperature:     cfg.Temperature,
		topP:            cfg.TopP,
		enableThinking:  cfg.EnableThinking,
		thinkingBudget:  cfg.ThinkingBudget,
		// Never "none": Muse Spark rejects it with HTTP 400.
		reasoningEffortHighReasoning: "high",
		reasoningEffortBalanced:      "medium",
		reasoningEffortHighSpeed:     "low",
	}
	return c, nil
}

// SetModel changes the model used for completions.
func (c *OpenAICompatClient) SetModel(model string) { c.model = model }

// GetModel returns the current model.
func (c *OpenAICompatClient) GetModel() string { return c.model }

// ModelForContext resolves the effective model, preferring a per-shard override
// carried in the context over the client's configured default.
func (c *OpenAICompatClient) ModelForContext(ctx context.Context) string {
	if ctx != nil {
		if v := ctx.Value(types.CtxKeyModelName); v != nil {
			if model, ok := v.(string); ok && strings.TrimSpace(model) != "" {
				return strings.TrimSpace(model)
			}
		}
	}
	return c.model
}

// reasoningEffortForContext maps a per-shard capability tier onto the vendor's
// effort scale. Returns "" when the context carries no hint, leaving the
// vendor's own default in force.
func (c *OpenAICompatClient) reasoningEffortForContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	var capHint types.ModelCapability
	if v := ctx.Value(types.CtxKeyModelCapability); v != nil {
		switch vv := v.(type) {
		case types.ModelCapability:
			capHint = vv
		case string:
			capHint = types.ModelCapability(strings.TrimSpace(vv))
		}
	}

	switch capHint {
	case types.CapabilityHighReasoning:
		return c.reasoningEffortHighReasoning
	case types.CapabilityBalanced:
		return c.reasoningEffortBalanced
	case types.CapabilityHighSpeed:
		return c.reasoningEffortHighSpeed
	}
	return ""
}

// isSchemaRejection reports whether a 400 body blames the structured-output
// schema, so the caller can retry without response_format instead of failing
// the turn.
//
// Vendors name the offending field inconsistently. OpenAI-style errors mention
// "response_format" or "json_schema", but Meta reports `"param":"schema"` with
// a message about a specific keyword — e.g. `'additionalProperties' is required
// to be supplied and to be false.` — matching neither. That gap turned a
// recoverable schema complaint into a hard turn failure.
func isSchemaRejection(body string) bool {
	for _, marker := range []string{
		"response_format",
		"json_schema",
		`"param":"schema"`,
		`"param": "schema"`,
		"additionalProperties",
	} {
		if strings.Contains(body, marker) {
			return true
		}
	}
	return false
}

// isPiggybackPrompt detects codeNERD's structured-envelope protocol, which wants
// JSON-schema-constrained output when the vendor supports it.
func isPiggybackPrompt(systemPrompt, userPrompt string) bool {
	return strings.Contains(systemPrompt, "control_packet") ||
		strings.Contains(systemPrompt, "surface_response") ||
		strings.Contains(userPrompt, "PiggybackEnvelope") ||
		strings.Contains(userPrompt, "control_packet")
}

// buildRequest assembles a vendor-correct request body.
func (c *OpenAICompatClient) buildRequest(ctx context.Context, messages []OpenAIMessage, thinking bool) OpenAIRequest {
	req := OpenAIRequest{
		Model:    c.ModelForContext(ctx),
		Messages: messages,
	}

	switch c.vendor {
	case ProviderMeta:
		// max_tokens is deprecated on the Meta Model API.
		req.MaxCompletionTokens = c.maxOutputTokens
		if thinking {
			if effort := c.reasoningEffortForContext(ctx); effort != "" {
				req.ReasoningEffort = effort
			}
		}
		// Sampling deliberately left at vendor defaults unless explicitly
		// configured, and never both at once.
		if c.temperature > 0 {
			req.Temperature = c.temperature
		} else if c.topP > 0 {
			req.TopP = c.topP
		}

	case ProviderDashScope:
		req.MaxTokens = c.maxOutputTokens
		enabled := thinking
		req.EnableThinking = &enabled
		if enabled && c.thinkingBudget > 0 {
			req.ThinkingBudget = c.thinkingBudget
		}
		req.Temperature = c.temperature
		req.TopP = c.topP

	default:
		req.MaxTokens = c.maxOutputTokens
		req.Temperature = c.temperature
		req.TopP = c.topP
	}

	return req
}

// throttle enforces a small floor between requests from this client instance.
// The global APIScheduler handles cross-client concurrency; this only prevents
// a single client from bursting.
func (c *OpenAICompatClient) throttle() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if elapsed := time.Since(c.lastRequest); elapsed < 100*time.Millisecond {
		time.Sleep(100*time.Millisecond - elapsed)
	}
	c.lastRequest = time.Now()
}

// retryDelay computes how long to wait before the next attempt, honouring a
// Retry-After header when the vendor sends one. Meta's contributor tier is
// limited to 60 requests/minute and does send it, so obeying beats guessing.
func retryDelay(resp *http.Response, attempt int) time.Duration {
	backoff := time.Duration(1<<uint(attempt)) * time.Second
	if resp == nil {
		return backoff
	}
	ra := strings.TrimSpace(resp.Header.Get("Retry-After"))
	if ra == "" {
		return backoff
	}
	if secs, err := strconv.Atoi(ra); err == nil && secs >= 0 {
		d := time.Duration(secs) * time.Second
		// Cap so a hostile or buggy header cannot stall a shard indefinitely.
		if d > 60*time.Second {
			d = 60 * time.Second
		}
		return d
	}
	if t, err := http.ParseTime(ra); err == nil {
		if d := time.Until(t); d > 0 && d <= 60*time.Second {
			return d
		}
	}
	return backoff
}

// executeChat performs a non-streaming chat completion with retries.
//
// It is the single HTTP path for every non-streaming method on this client, so
// rate-limit handling, structured-output fallback, and error decoding exist in
// exactly one place.
func (c *OpenAICompatClient) executeChat(ctx context.Context, reqBody OpenAIRequest, allowSchemaFallback bool) (*OpenAIResponse, error) {
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.httpClient.Timeout)
		defer cancel()
	}

	c.throttle()

	const maxRetries = 3
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		jsonData, err := json.Marshal(reqBody)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewReader(jsonData))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+c.apiKey)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("request failed: %w", err)
			if sleepErr := sleepCtx(ctx, retryDelay(nil, attempt)); sleepErr != nil {
				return nil, sleepErr
			}
			continue
		}

		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
		resp.Body.Close()
		if readErr != nil {
			lastErr = fmt.Errorf("failed to read response: %w", readErr)
			if sleepErr := sleepCtx(ctx, retryDelay(resp, attempt)); sleepErr != nil {
				return nil, sleepErr
			}
			continue
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			wait := retryDelay(resp, attempt)
			logging.PerceptionWarn("[%s] rate limited (429), retrying in %v", c.vendor, wait)
			lastErr = fmt.Errorf("rate limit exceeded (429): %s", strings.TrimSpace(string(body)))
			if sleepErr := sleepCtx(ctx, wait); sleepErr != nil {
				return nil, sleepErr
			}
			continue
		}

		if resp.StatusCode != http.StatusOK {
			bodyStr := string(body)
			// Some models reject json_schema output; drop it and retry once.
			if allowSchemaFallback && reqBody.ResponseFormat != nil && resp.StatusCode == http.StatusBadRequest &&
				isSchemaRejection(bodyStr) {
				logging.PerceptionWarn("[%s] structured output rejected, retrying without response_format", c.vendor)
				reqBody.ResponseFormat = nil
				lastErr = fmt.Errorf("structured output rejected: %s", bodyStr)
				continue
			}
			return nil, fmt.Errorf("%s API request failed with status %d: %s", c.vendor, resp.StatusCode, bodyStr)
		}

		var parsed OpenAIResponse
		if err := json.Unmarshal(body, &parsed); err != nil {
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}
		if parsed.Error != nil {
			return nil, fmt.Errorf("%s API error: %s", c.vendor, parsed.Error.Message)
		}
		return &parsed, nil
	}

	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}

// sleepCtx waits for d, aborting early if the context is cancelled.
func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Complete sends a prompt and returns the completion.
func (c *OpenAICompatClient) Complete(ctx context.Context, prompt string) (string, error) {
	return c.CompleteWithSystem(ctx, "", prompt)
}

// CompleteWithSystem sends a prompt with a system message.
func (c *OpenAICompatClient) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	start := time.Now()
	if strings.TrimSpace(systemPrompt) == "" {
		systemPrompt = defaultSystemPrompt
	}

	piggyback := isPiggybackPrompt(systemPrompt, userPrompt)

	reqBody := c.buildRequest(ctx, []OpenAIMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}, c.enableThinking)

	if piggyback {
		reqBody.ResponseFormat = BuildOpenRouterPiggybackEnvelopeSchema()
	}

	resp, err := c.executeChat(ctx, reqBody, piggyback)
	if err != nil {
		logging.PerceptionError("[%s] CompleteWithSystem failed after %v: %v", c.vendor, time.Since(start), err)
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no completion returned")
	}

	msg := resp.Choices[0].Message
	// reasoning_content is deliberately NOT concatenated: callers parse Content
	// as JSON for structured output, and prepending prose would break that.
	if msg.ReasoningContent != "" {
		logging.PerceptionDebug("[%s] reasoning trace: %d chars", c.vendor, len(msg.ReasoningContent))
	}

	out := strings.TrimSpace(msg.Content)

	// An empty completion must fail loudly. Returned as a successful "" it
	// propagates through the whole agent loop as a hollow-but-successful result
	// — the shard_result_empty / generation_degraded class this repo already
	// tracks. The usual cause is a reasoning model exhausting its completion
	// budget on thinking, which the vendor reports as finish_reason "stop"
	// rather than "length", so the status alone does not reveal it.
	if out == "" {
		finish := resp.Choices[0].FinishReason
		return "", fmt.Errorf("%s returned an empty completion (model=%s finish_reason=%q reasoning_chars=%d output_tokens=%d); "+
			"if finish_reason is \"stop\" with 0 content the completion budget was likely consumed by reasoning",
			c.vendor, reqBody.Model, finish, len(msg.ReasoningContent), resp.Usage.CompletionTokens)
	}

	logging.Perception("[%s] CompleteWithSystem: model=%s completed in %v response_len=%d",
		c.vendor, reqBody.Model, time.Since(start), len(out))
	return out, nil
}

// CompleteWithStreaming sends a prompt with streaming enabled and returns
// channels of incremental visible-content deltas.
//
// Reasoning deltas (DashScope's reasoning_content) are intentionally not
// forwarded: the channel feeds the chat surface and structured-output parsers,
// neither of which should see the thinking trace.
func (c *OpenAICompatClient) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (<-chan string, <-chan error) {
	contentChan := make(chan string, 100)
	errorChan := make(chan error, 1)

	go func() {
		defer close(contentChan)
		defer close(errorChan)

		if _, hasDeadline := ctx.Deadline(); !hasDeadline {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, c.httpClient.Timeout)
			defer cancel()
		}

		start := time.Now()
		if strings.TrimSpace(systemPrompt) == "" {
			systemPrompt = defaultSystemPrompt
		}

		piggyback := isPiggybackPrompt(systemPrompt, userPrompt)

		reqBody := c.buildRequest(ctx, []OpenAIMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		}, enableThinking || c.enableThinking)
		reqBody.Stream = true
		reqBody.StreamOptions = &OpenAIStreamOptions{IncludeUsage: true}
		if piggyback {
			reqBody.ResponseFormat = BuildOpenRouterPiggybackEnvelopeSchema()
		}

		c.throttle()

		const maxRetries = 3
		var lastErr error

		for attempt := 0; attempt <= maxRetries; attempt++ {
			jsonData, err := json.Marshal(reqBody)
			if err != nil {
				errorChan <- fmt.Errorf("failed to marshal request: %w", err)
				return
			}

			req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewReader(jsonData))
			if err != nil {
				errorChan <- fmt.Errorf("failed to create request: %w", err)
				return
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+c.apiKey)
			req.Header.Set("Accept", "text/event-stream")

			resp, err := c.httpClient.Do(req)
			if err != nil {
				lastErr = fmt.Errorf("request failed: %w", err)
				if sleepErr := sleepCtx(ctx, retryDelay(nil, attempt)); sleepErr != nil {
					errorChan <- sleepErr
					return
				}
				continue
			}

			if resp.StatusCode == http.StatusTooManyRequests {
				body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
				wait := retryDelay(resp, attempt)
				resp.Body.Close()
				logging.PerceptionWarn("[%s] stream rate limited (429), retrying in %v", c.vendor, wait)
				lastErr = fmt.Errorf("rate limit exceeded (429): %s", strings.TrimSpace(string(body)))
				if sleepErr := sleepCtx(ctx, wait); sleepErr != nil {
					errorChan <- sleepErr
					return
				}
				continue
			}

			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
				resp.Body.Close()
				bodyStr := string(body)
				if piggyback && reqBody.ResponseFormat != nil && resp.StatusCode == http.StatusBadRequest &&
					isSchemaRejection(bodyStr) {
					reqBody.ResponseFormat = nil
					lastErr = fmt.Errorf("structured output rejected: %s", bodyStr)
					continue
				}
				errorChan <- fmt.Errorf("%s API request failed with status %d: %s", c.vendor, resp.StatusCode, bodyStr)
				return
			}

			c.consumeStream(ctx, resp, contentChan, errorChan, start)
			return
		}

		logging.PerceptionError("[%s] CompleteWithStreaming: max retries exceeded after %v: %v", c.vendor, time.Since(start), lastErr)
		errorChan <- fmt.Errorf("max retries exceeded: %w", lastErr)
	}()

	return contentChan, errorChan
}

// consumeStream reads a server-sent-event body and forwards content deltas.
func (c *OpenAICompatClient) consumeStream(ctx context.Context, resp *http.Response, contentChan chan<- string, errorChan chan<- error, start time.Time) {
	defer resp.Body.Close()

	scanner, releaseScanner := newPooledScanner(resp.Body, 1024*1024)
	defer releaseScanner()

	scanDone := make(chan struct{})
	scanErrChan := make(chan error, 1)
	// Counted so a clean-but-empty stream can be reported as a failure rather
	// than rendering blank in the chat surface.
	var forwarded atomic.Int64

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

			var chunk OpenAIResponse
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}
			if chunk.Error != nil {
				scanErrChan <- fmt.Errorf("%s API error: %s", c.vendor, chunk.Error.Message)
				return
			}
			if len(chunk.Choices) == 0 || chunk.Choices[0].Delta == nil {
				continue
			}
			if delta := chunk.Choices[0].Delta.Content; delta != "" {
				select {
				case contentChan <- delta:
					forwarded.Add(1)
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
		select {
		case err := <-scanErrChan:
			logging.PerceptionError("[%s] CompleteWithStreaming: stream error after %v: %v", c.vendor, time.Since(start), err)
			errorChan <- fmt.Errorf("stream error: %w", err)
		default:
			// A stream that closes cleanly having emitted nothing is the same
			// hollow-success failure as an empty non-streaming completion; the
			// chat surface would simply render blank. See CompleteWithSystem.
			if forwarded.Load() == 0 {
				logging.PerceptionError("[%s] CompleteWithStreaming: stream closed with no content after %v", c.vendor, time.Since(start))
				errorChan <- fmt.Errorf("%s stream produced no content (model=%s); "+
					"a reasoning model may have consumed the completion budget before emitting output", c.vendor, c.model)
				return
			}
			logging.Perception("[%s] CompleteWithStreaming: completed in %v", c.vendor, time.Since(start))
		}
	case <-ctx.Done():
		resp.Body.Close()
		<-scanDone
		logging.PerceptionWarn("[%s] CompleteWithStreaming: cancelled after %v", c.vendor, time.Since(start))
		errorChan <- ctx.Err()
	}
}

// CompleteWithTools sends a prompt with tool definitions.
func (c *OpenAICompatClient) CompleteWithTools(ctx context.Context, systemPrompt, userPrompt string, tools []ToolDefinition) (*LLMToolResponse, error) {
	if strings.TrimSpace(systemPrompt) == "" {
		systemPrompt = defaultSystemPrompt
	}

	reqBody := c.buildRequest(ctx, []OpenAIMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}, c.enableThinking)
	reqBody.Tools = MapToolDefinitionsToOpenAI(tools)
	if len(reqBody.Tools) > 0 {
		reqBody.ToolChoice = "auto"
	}

	resp, err := c.executeChat(ctx, reqBody, false)
	if err != nil {
		return nil, err
	}
	return c.toToolResponse(resp)
}

// CompleteWithToolResults continues a multi-turn tool-calling conversation,
// satisfying types.ToolResultsProvider so the session executor can run a full
// agentic loop instead of falling back to single-turn tool use.
func (c *OpenAICompatClient) CompleteWithToolResults(ctx context.Context, systemPrompt string, history []types.Message, tools []ToolDefinition) (*LLMToolResponse, error) {
	messages, err := MapTypesHistoryToOpenAIMessages(systemPrompt, history)
	if err != nil {
		return nil, err
	}

	reqBody := c.buildRequest(ctx, messages, c.enableThinking)
	reqBody.Tools = MapToolDefinitionsToOpenAI(tools)
	if len(reqBody.Tools) > 0 {
		reqBody.ToolChoice = "auto"
	}

	resp, err := c.executeChat(ctx, reqBody, false)
	if err != nil {
		return nil, err
	}
	return c.toToolResponse(resp)
}

// toToolResponse converts a raw OpenAI-shaped response into the internal shape.
func (c *OpenAICompatClient) toToolResponse(resp *OpenAIResponse) (*LLMToolResponse, error) {
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}
	choice := resp.Choices[0]

	toolCalls, err := MapOpenAIToolCallsToInternal(choice.Message.ToolCalls)
	if err != nil {
		return nil, err
	}

	stopReason := choice.FinishReason
	if stopReason == "tool_calls" {
		stopReason = "tool_use"
	}

	return &LLMToolResponse{
		Text:       choice.Message.Content,
		ToolCalls:  toolCalls,
		StopReason: stopReason,
		Usage: types.UsageMetadata{
			InputTokens:  resp.Usage.PromptTokens,
			OutputTokens: resp.Usage.CompletionTokens,
			TotalTokens:  resp.Usage.TotalTokens,
		},
	}, nil
}
