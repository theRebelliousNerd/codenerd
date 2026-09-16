package perception

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// compile-time assertion that OpenAICompatClient implements GroundedWebSearcher.
// GroundedWebSearch exists on OpenAICompatClient for all vendors but fails closed unless vendor is Meta.
var _ types.GroundedWebSearcher = (*OpenAICompatClient)(nil)

// SupportsGroundedWebSearch reports whether this client can perform Meta grounded search.
// It is deterministic: true only when the receiver is non-nil and its vendor is Meta.
func (c *OpenAICompatClient) SupportsGroundedWebSearch() bool {
	if c == nil {
		return false
	}
	return c.vendor == ProviderMeta
}

// metaGroundedRequest is the wire payload for POST /responses.
type metaGroundedRequest struct {
	Model     string                  `json:"model"`
	Input     []metaGroundedInputItem `json:"input"`
	Tools     []metaGroundedTool      `json:"tools"`
	Reasoning *metaGroundedReasoning  `json:"reasoning,omitempty"`
	Stream    bool                    `json:"stream"`
}

type metaGroundedInputItem struct {
	Role    string                     `json:"role"`
	Content []metaGroundedInputContent `json:"content"`
}

type metaGroundedInputContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type metaGroundedTool struct {
	Type string `json:"type"`
}

type metaGroundedReasoning struct {
	Effort string `json:"effort"`
}

// metaGroundedResponse is the minimal shape we need to parse.
type metaGroundedResponse struct {
	Output []metaGroundedOutputItem `json:"output"`
	Usage  *metaGroundedUsage       `json:"usage,omitempty"`
	Error  *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

type metaGroundedOutputItem struct {
	Type    string                      `json:"type"`
	Role    string                      `json:"role,omitempty"`
	Content []metaGroundedOutputContent `json:"content,omitempty"`
}

type metaGroundedOutputContent struct {
	Type        string                   `json:"type"`
	Text        string                   `json:"text,omitempty"`
	Annotations []metaGroundedAnnotation `json:"annotations,omitempty"`
}

type metaGroundedAnnotation struct {
	Type       string `json:"type"`
	URL        string `json:"url"`
	Title      string `json:"title,omitempty"`
	StartIndex int    `json:"start_index"`
	EndIndex   int    `json:"end_index"`
}

type metaGroundedUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
	// Also accept alternative naming that some vendors use; not marshalled, only for unmarshal fallback.
	PromptTokens     int `json:"prompt_tokens,omitempty"`
	CompletionTokens int `json:"completion_tokens,omitempty"`
}

const (
	maxGroundedQueryChars    = 10000
	maxGroundedRequestBytes  = 1 << 20          // 1 MiB
	maxGroundedResponseBytes = 10 * 1024 * 1024 // 10 MiB
	maxGroundedErrorCodeLen  = 128
	maxGroundedErrorTypeLen  = 128
	// Bounded default timeout for grounded search when no deadline is set.
	defaultGroundedTimeout = 30 * time.Second
	maxGroundedTimeout     = 5 * time.Minute
)

// sanitizeGroundedErrorToken trims, bounds, allows only conventional identifier
// characters ([A-Za-z0-9_.-]), and drops any token containing the configured API
// key. It returns "" for anything that does not look like a safe code/type
// identifier or that could exfiltrate the key via a malicious structured error.
func sanitizeGroundedErrorToken(s, apiKey string, maxLen int) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if apiKey != "" && strings.Contains(s, apiKey) {
		return ""
	}
	if len(s) > maxLen {
		s = s[:maxLen]
		s = strings.TrimSpace(s)
		if s == "" {
			return ""
		}
		if apiKey != "" && strings.Contains(s, apiKey) {
			return ""
		}
	}
	for _, r := range s {
		if r >= 'a' && r <= 'z' {
			continue
		}
		if r >= 'A' && r <= 'Z' {
			continue
		}
		if r >= '0' && r <= '9' {
			continue
		}
		if r == '_' || r == '-' || r == '.' {
			continue
		}
		return ""
	}
	if apiKey != "" && strings.Contains(s, apiKey) {
		return ""
	}
	return s
}

// groundedStatusError builds the sanitized error for a non-200 response.
// Never returns raw body or error message bodies: only the status plus
// sanitized structured code/type tokens.
func groundedStatusError(status int, body []byte, apiKey string) error {
	var errPayload struct {
		Error *struct {
			Code    string `json:"code"`
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	code := ""
	typ := ""
	if jsonErr := json.Unmarshal(body, &errPayload); jsonErr == nil && errPayload.Error != nil {
		code = sanitizeGroundedErrorToken(errPayload.Error.Code, apiKey, maxGroundedErrorCodeLen)
		typ = sanitizeGroundedErrorToken(errPayload.Error.Type, apiKey, maxGroundedErrorTypeLen)
	} else {
		// Also try flat code/type if not nested.
		var flat struct {
			Code string `json:"code"`
			Type string `json:"type"`
		}
		if jsonErr2 := json.Unmarshal(body, &flat); jsonErr2 == nil {
			code = sanitizeGroundedErrorToken(flat.Code, apiKey, maxGroundedErrorCodeLen)
			typ = sanitizeGroundedErrorToken(flat.Type, apiKey, maxGroundedErrorTypeLen)
		}
	}
	switch {
	case code != "" && typ != "":
		return fmt.Errorf("meta grounded search: request failed with status %d: code=%s type=%s", status, code, typ)
	case code != "":
		return fmt.Errorf("meta grounded search: request failed with status %d: code=%s", status, code)
	case typ != "":
		return fmt.Errorf("meta grounded search: request failed with status %d: type=%s", status, typ)
	default:
		return fmt.Errorf("meta grounded search: request failed with status %d", status)
	}
}

// groundedContextErr maps a transport failure to the context-cancellation
// error when the context (or the failure itself) carries cancellation,
// preserving the identity callers use to distinguish timeouts. It returns nil
// when the failure is a plain transport error that the retry loop should
// absorb rather than return.
func groundedContextErr(ctx context.Context, err error) error {
	if errors.Is(err, context.Canceled) {
		return fmt.Errorf("meta grounded search: request canceled: %w", context.Canceled)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("meta grounded search: request deadline exceeded: %w", context.DeadlineExceeded)
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		if errors.Is(ctxErr, context.Canceled) {
			return fmt.Errorf("meta grounded search: request canceled: %w", context.Canceled)
		}
		if errors.Is(ctxErr, context.DeadlineExceeded) {
			return fmt.Errorf("meta grounded search: request deadline exceeded: %w", context.DeadlineExceeded)
		}
	}
	return nil
}

// groundedSleepErr maps a sleepCtx interruption (the context died mid-backoff)
// to the same cancellation errors the transport path returns.
func groundedSleepErr(err error) error {
	if errors.Is(err, context.Canceled) {
		return fmt.Errorf("meta grounded search: request canceled: %w", context.Canceled)
	}
	return fmt.Errorf("meta grounded search: request deadline exceeded: %w", context.DeadlineExceeded)
}

// GroundedWebSearch performs a Meta-native grounded web search via POST /responses.
// It is only supported when the client vendor is Meta; non-Meta calls fail closed
// before any HTTP is attempted.
func (c *OpenAICompatClient) GroundedWebSearch(ctx context.Context, query string) (*types.GroundedWebSearchResult, error) {
	if c == nil {
		return nil, fmt.Errorf("meta grounded search: client is nil")
	}
	if c.vendor != ProviderMeta {
		return nil, fmt.Errorf("grounded web search is only supported for provider %q (got %q)", ProviderMeta, c.vendor)
	}
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return nil, fmt.Errorf("grounded web search query must not be blank")
	}
	if len(trimmed) > maxGroundedQueryChars {
		return nil, fmt.Errorf("grounded web search query exceeds %d characters", maxGroundedQueryChars)
	}
	effectiveTimeout := defaultGroundedTimeout
	if c.httpClient != nil && c.httpClient.Timeout > 0 {
		effectiveTimeout = c.httpClient.Timeout
	}
	if effectiveTimeout > maxGroundedTimeout {
		effectiveTimeout = maxGroundedTimeout
	}
	if effectiveTimeout <= 0 {
		effectiveTimeout = defaultGroundedTimeout
	}
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, effectiveTimeout)
		defer cancel()
	}

	// Determine reasoning effort: explicit override > per-shard capability > default xhigh.
	effort := strings.TrimSpace(c.reasoningEffortOverride)
	if effort == "" {
		effort = strings.TrimSpace(c.reasoningEffortForContext(ctx))
	}
	if effort == "" {
		effort = "xhigh"
	}
	// "none" is rejected by Muse Spark (see newResponsesRequest), so it is
	// never forwarded; omit the field rather than fail the search.
	if effort == "none" {
		effort = ""
	}

	model := c.ModelForContext(ctx)

	reqBody := metaGroundedRequest{
		Model: model,
		Input: []metaGroundedInputItem{
			{
				Role: "user",
				Content: []metaGroundedInputContent{
					{Type: "input_text", Text: trimmed},
				},
			},
		},
		Tools: []metaGroundedTool{
			{Type: "web_search"},
		},
	}
	if effort != "" {
		reqBody.Reasoning = &metaGroundedReasoning{Effort: effort}
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("meta grounded search: failed to marshal request: %w", err)
	}
	if len(jsonData) > maxGroundedRequestBytes {
		return nil, fmt.Errorf("meta grounded search: serialized request exceeds %d bytes", maxGroundedRequestBytes)
	}

	c.throttle()

	endpoint := strings.TrimSuffix(c.baseURL, "/") + metaResponsesPath

	httpClient := c.httpClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: effectiveTimeout, Transport: sharedTransport}
	}

	// Bounded retry, mirroring executeResponses. A grounded search used to be
	// a single POST, so one transient 503 killed the whole search while the
	// sibling tool-loop path retried the same failure. The retry decision
	// uses only the status code — never the body — so the sanitized-error
	// posture below is unchanged: vendor text still never reaches an error.
	const maxRetries = 3
	var (
		body    []byte
		lastErr error
	)
	for attempt := 0; attempt <= maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(jsonData))
		if err != nil {
			return nil, fmt.Errorf("meta grounded search: failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+c.apiKey)

		resp, err := httpClient.Do(req)
		if err != nil {
			// Never expose API key or arbitrary provider text from a custom
			// RoundTripper. Preserve context cancellation/deadline identity so
			// callers can distinguish timeouts, but otherwise return a generic
			// transport failure without wrapping the raw error string.
			if ctxErr := groundedContextErr(ctx, err); ctxErr != nil {
				return nil, ctxErr
			}
			lastErr = fmt.Errorf("meta grounded search: transport error")
			logging.PerceptionWarn("[%s] grounded search attempt %d/%d transport failure; retrying",
				c.vendor, attempt+1, maxRetries+1)
			if sleepErr := sleepCtx(ctx, retryDelay(nil, attempt)); sleepErr != nil {
				return nil, groundedSleepErr(sleepErr)
			}
			continue
		}

		// Bound response body.
		body, err = io.ReadAll(io.LimitReader(resp.Body, maxGroundedResponseBytes+1))
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("meta grounded search: failed to read response: %w", err)
		}
		if int64(len(body)) > maxGroundedResponseBytes {
			return nil, fmt.Errorf("meta grounded search: response too large (%d bytes)", len(body))
		}

		if resp.StatusCode == http.StatusOK {
			lastErr = nil
			break
		}

		if resp.StatusCode == http.StatusTooManyRequests || isTransientHTTPStatus(resp.StatusCode) {
			wait := retryDelay(resp, attempt)
			logging.PerceptionWarn("[%s] grounded search attempt %d/%d got HTTP %d; retrying in %v",
				c.vendor, attempt+1, maxRetries+1, resp.StatusCode, wait)
			lastErr = groundedStatusError(resp.StatusCode, body, c.apiKey)
			if sleepErr := sleepCtx(ctx, wait); sleepErr != nil {
				return nil, groundedSleepErr(sleepErr)
			}
			continue
		}
		return nil, groundedStatusError(resp.StatusCode, body, c.apiKey)
	}
	if lastErr != nil {
		return nil, fmt.Errorf("%w (after %d attempts)", lastErr, maxRetries+1)
	}

	var parsed metaGroundedResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("meta grounded search: failed to parse response: %w", err)
	}
	if parsed.Error != nil {
		code := sanitizeGroundedErrorToken(parsed.Error.Code, c.apiKey, maxGroundedErrorCodeLen)
		typ := sanitizeGroundedErrorToken(parsed.Error.Type, c.apiKey, maxGroundedErrorTypeLen)
		// Any non-nil error object must fail, even without message. Never expose message.
		switch {
		case code != "" && typ != "":
			return nil, fmt.Errorf("meta grounded search: api error: code=%s type=%s", code, typ)
		case code != "":
			return nil, fmt.Errorf("meta grounded search: api error: code=%s", code)
		case typ != "":
			return nil, fmt.Errorf("meta grounded search: api error: type=%s", typ)
		default:
			return nil, fmt.Errorf("meta grounded search: api error")
		}
	}

	var textBuilder strings.Builder
	var citations []types.GroundedCitation
	for _, item := range parsed.Output {
		// Ignore reasoning and web_search_call items entirely; never expose reasoning trace.
		if item.Type == "reasoning" || item.Type == "web_search_call" {
			continue
		}
		if item.Type != "message" {
			continue
		}
		if item.Role != "" && item.Role != "assistant" {
			continue
		}
		for _, content := range item.Content {
			if content.Type != "output_text" {
				continue
			}
			textBuilder.WriteString(content.Text)
			for _, ann := range content.Annotations {
				if ann.Type != "url_citation" {
					continue
				}
				u := strings.TrimSpace(ann.URL)
				if u == "" {
					continue
				}
				parsedURL, uerr := url.Parse(u)
				if uerr != nil {
					continue
				}
				if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
					continue
				}
				if parsedURL.Host == "" {
					continue
				}
				citations = append(citations, types.GroundedCitation{
					URL:        u,
					Title:      ann.Title,
					StartIndex: ann.StartIndex,
					EndIndex:   ann.EndIndex,
				})
			}
		}
	}

	text := strings.TrimSpace(textBuilder.String())
	if text == "" {
		return nil, fmt.Errorf("meta grounded search: no output_text in response")
	}

	usage := types.GroundedUsage{}
	if parsed.Usage != nil {
		// Prefer native input/output naming; fallback to prompt/completion if the primary
		// input/output counts are zero but alternative counts are present.
		if parsed.Usage.InputTokens != 0 || parsed.Usage.OutputTokens != 0 {
			usage.InputTokens = parsed.Usage.InputTokens
			usage.OutputTokens = parsed.Usage.OutputTokens
			usage.TotalTokens = parsed.Usage.TotalTokens
		} else if parsed.Usage.PromptTokens != 0 || parsed.Usage.CompletionTokens != 0 {
			usage.InputTokens = parsed.Usage.PromptTokens
			usage.OutputTokens = parsed.Usage.CompletionTokens
			usage.TotalTokens = parsed.Usage.TotalTokens
			if usage.TotalTokens == 0 {
				usage.TotalTokens = usage.InputTokens + usage.OutputTokens
			}
		} else {
			usage.TotalTokens = parsed.Usage.TotalTokens
		}
	}
	// A grounded search is a billed model turn like any other; it just does not
	// go through executeChat, so it needs its own Track.
	trackUsage(ctx, model, c.vendor, usage.InputTokens, usage.OutputTokens, usageOpSearch)

	// Ensure citations is non-nil vs nil consistency for tests; return empty slice vs nil both acceptable,
	// but normalize to empty slice.
	if citations == nil {
		citations = []types.GroundedCitation{}
	}

	result := &types.GroundedWebSearchResult{
		Text:      text,
		Citations: citations,
		Usage:     usage,
	}
	return result, nil
}
