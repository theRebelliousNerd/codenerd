package perception

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// compile-time assertion that OpenAICompatClient implements GroundedWebSearcher.
// GroundedWebSearch exists on OpenAICompatClient for all vendors but fails closed unless vendor is Meta.
var _ types.GroundedWebSearcher = (*OpenAICompatClient)(nil)

// The controller is on the shared type. SupportsGrounding is what says only
// Meta can actually ground; the broker asks that before it trusts the method set.
var (
	_ types.GroundingController = (*OpenAICompatClient)(nil)
	_ types.GroundingCapable    = (*OpenAICompatClient)(nil)
)

// SupportsGroundedWebSearch reports whether this client can perform Meta grounded search.
// It is deterministic: true only when the receiver is non-nil and its vendor is Meta.
func (c *OpenAICompatClient) SupportsGroundedWebSearch() bool {
	if c == nil {
		return false
	}
	return c.vendor == ProviderMeta
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
func groundedStatusError(provider Provider, status int, body []byte, apiKey string) error {
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
		return fmt.Errorf("%s grounded search: request failed with status %d: code=%s type=%s", provider, status, code, typ)
	case code != "":
		return fmt.Errorf("%s grounded search: request failed with status %d: code=%s", provider, status, code)
	case typ != "":
		return fmt.Errorf("%s grounded search: request failed with status %d: type=%s", provider, status, typ)
	default:
		return fmt.Errorf("%s grounded search: request failed with status %d", provider, status)
	}
}

// groundedContextErr maps a transport failure to the context-cancellation
// error when the context (or the failure itself) carries cancellation,
// preserving the identity callers use to distinguish timeouts. It returns nil
// when the failure is a plain transport error that the retry loop should
// absorb rather than return.
func groundedContextErr(provider Provider, ctx context.Context, err error) error {
	if errors.Is(err, context.Canceled) {
		return fmt.Errorf("%s grounded search: request canceled: %w", provider, context.Canceled)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%s grounded search: request deadline exceeded: %w", provider, context.DeadlineExceeded)
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		if errors.Is(ctxErr, context.Canceled) {
			return fmt.Errorf("%s grounded search: request canceled: %w", provider, context.Canceled)
		}
		if errors.Is(ctxErr, context.DeadlineExceeded) {
			return fmt.Errorf("%s grounded search: request deadline exceeded: %w", provider, context.DeadlineExceeded)
		}
	}
	return nil
}

// groundedSleepErr maps a sleepCtx interruption (the context died mid-backoff)
// to the same cancellation errors the transport path returns.
func groundedSleepErr(provider Provider, err error) error {
	if errors.Is(err, context.Canceled) {
		return fmt.Errorf("%s grounded search: request canceled: %w", provider, context.Canceled)
	}
	return fmt.Errorf("%s grounded search: request deadline exceeded: %w", provider, context.DeadlineExceeded)
}

// GroundedWebSearch performs a Meta-native grounded web search via POST /responses.
// It is only supported when the client vendor is Meta; non-Meta calls fail closed
// before any HTTP is attempted.
func (c *OpenAICompatClient) GroundedWebSearch(ctx context.Context, query string) (*types.GroundedWebSearchResult, error) {
	if c == nil {
		return nil, fmt.Errorf("grounded search: client is nil")
	}
	if c.vendor != ProviderMeta {
		return nil, fmt.Errorf("grounded web search is only supported for provider %q (got %q)", ProviderMeta, c.vendor)
	}
	if err := c.admit(ctx, "", userTurn(query), nil); err != nil {
		return nil, err
	}
	c.clearGroundingSources()
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

	// The same builder the tool loop uses, so a grounded search is sent the
	// slot's configured effort (or a profile's) like every other request.
	reqBody := c.newResponsesRequest(ctx, []any{responsesInputText("user", trimmed)})
	// Always attach search. This call is a search even when the config
	// switch that grounds ordinary completions is off.
	if err := c.forceWebSearch(&reqBody, nil); err != nil {
		return nil, err
	}
	reqBody.PromptCacheKey = promptCacheKey(reqBody.Model, reqBody.Tools)
	model := reqBody.Model

	parsed, err := postGroundedSearch(ctx, responsesEndpoint{
		httpClient: c.httpClient, baseURL: c.baseURL, apiKey: c.apiKey, provider: c.vendor,
	}, reqBody, effectiveTimeout, c.throttle)
	if err != nil {
		return nil, err
	}
	// A grounded search is a billed model turn like any other; it does not go
	// through executeResponses (whose errors carry the body), so it reports
	// its own usage -- before admission, so a run that is not admitted is
	// still charged.
	reportUsage(ctx, model, c.vendor, usageOpSearch, parsed.Usage.metadata())
	if err := groundedTerminal(parsed, c.vendor, model, reqBody.MaxOutputTokens, c.apiKey); err != nil {
		return nil, err
	}
	result, sources, err := groundedResultFromReply(c.vendor, parsed)
	if err != nil {
		return nil, err
	}
	c.recordGroundingSources(sources)
	return result, nil
}

// postGroundedSearch sends one grounded-search request with bounded retry and
// returns the decoded reply. Errors never carry the response body or a vendor
// message: only the status and sanitized code/type tokens, because a search
// error surfaces to the model and a malicious structured error must not
// exfiltrate the key or inject text. Shared by Meta and OpenAI.
func postGroundedSearch(ctx context.Context, e responsesEndpoint, reqBody responsesRequest, effectiveTimeout time.Duration, throttle func()) (*responsesReply, error) {
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("%s grounded search: failed to marshal request: %w", e.provider, err)
	}
	if len(jsonData) > maxGroundedRequestBytes {
		return nil, fmt.Errorf("%s grounded search: serialized request exceeds %d bytes", e.provider, maxGroundedRequestBytes)
	}

	throttle()

	endpoint := e.url()

	httpClient := e.httpClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: effectiveTimeout, Transport: wireTransport}
	}

	// Bounded retry, mirroring executeResponses. A grounded search used to be
	// a single POST, so one transient 503 killed the whole search while the
	// sibling tool-loop path retried the same failure. The retry decision
	// uses only the status code — never the body — so the sanitized-error
	// posture below is unchanged: vendor text still never reaches an error.
	maxRetries := llmMaxRetries()
	var (
		body    []byte
		lastErr error
	)
	for attempt := 0; attempt <= maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(jsonData))
		if err != nil {
			return nil, fmt.Errorf("%s grounded search: failed to create request: %w", e.provider, err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+e.apiKey)

		resp, err := httpClient.Do(req)
		if err != nil {
			// Never expose API key or arbitrary provider text from a custom
			// RoundTripper. Preserve context cancellation/deadline identity so
			// callers can distinguish timeouts, but otherwise return a generic
			// transport failure without wrapping the raw error string.
			if ctxErr := groundedContextErr(e.provider, ctx, err); ctxErr != nil {
				return nil, ctxErr
			}
			lastErr = fmt.Errorf("%s grounded search: transport error", e.provider)
			logging.PerceptionWarn("[%s] grounded search attempt %d/%d transport failure; retrying",
				e.provider, attempt+1, maxRetries+1)
			if sleepErr := sleepCtx(ctx, retryDelay(nil, attempt)); sleepErr != nil {
				return nil, groundedSleepErr(e.provider, sleepErr)
			}
			continue
		}

		// Bound response body.
		body, err = io.ReadAll(io.LimitReader(resp.Body, maxGroundedResponseBytes+1))
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("%s grounded search: failed to read response: %w", e.provider, err)
		}
		if int64(len(body)) > maxGroundedResponseBytes {
			return nil, fmt.Errorf("%s grounded search: response too large (%d bytes)", e.provider, len(body))
		}

		if resp.StatusCode == http.StatusOK {
			lastErr = nil
			break
		}

		if resp.StatusCode == http.StatusTooManyRequests || isTransientHTTPStatus(resp.StatusCode) {
			wait := retryDelay(resp, attempt)
			logging.PerceptionWarn("[%s] grounded search attempt %d/%d got HTTP %d; retrying in %v",
				e.provider, attempt+1, maxRetries+1, resp.StatusCode, wait)
			lastErr = groundedStatusError(e.provider, resp.StatusCode, body, e.apiKey)
			if sleepErr := sleepCtx(ctx, wait); sleepErr != nil {
				return nil, groundedSleepErr(e.provider, sleepErr)
			}
			continue
		}
		return nil, groundedStatusError(e.provider, resp.StatusCode, body, e.apiKey)
	}
	if lastErr != nil {
		return nil, fmt.Errorf("%w (after %d attempts)", lastErr, maxRetries+1)
	}

	var parsed responsesReply
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("%s grounded search: failed to parse response: %w", e.provider, err)
	}
	if parsed.Error != nil {
		code := sanitizeGroundedErrorToken(parsed.Error.Code, e.apiKey, maxGroundedErrorCodeLen)
		typ := sanitizeGroundedErrorToken(parsed.Error.Type, e.apiKey, maxGroundedErrorTypeLen)
		// Any non-nil error object must fail, even without message. Never expose message.
		switch {
		case code != "" && typ != "":
			return nil, fmt.Errorf("%s grounded search: api error: code=%s type=%s", e.provider, code, typ)
		case code != "":
			return nil, fmt.Errorf("%s grounded search: api error: code=%s", e.provider, code)
		case typ != "":
			return nil, fmt.Errorf("%s grounded search: api error: type=%s", e.provider, typ)
		default:
			return nil, fmt.Errorf("%s grounded search: api error", e.provider)
		}
	}
	return &parsed, nil
}

// groundedTerminal admits a grounded-search run as an answer, or says why it
// is not one, before any result or source is published. Until 2026-10-30
// the Meta caller converted any reply with text, so an HTTP-200 run the
// provider marked incomplete (max_output_tokens) became a successful search
// whose prefix read as the whole answer. Shared by Meta and OpenAI, with the
// search transport's sanitized posture: no vendor message, refusal text or
// unsanitized token reaches the error.
//   - completed, or no status (a provider that reports none): admitted;
//   - an output-limit incomplete: the typed truncation error, so the broker
//     restates the request instead of accepting a cut answer;
//   - a refusal: the typed refused outcome, never rerouted as unavailability;
//   - failed, cancelled, any other incomplete reason, or a status that is not
//     terminal: an error naming only the sanitized status and reason.
func groundedTerminal(reply *responsesReply, provider Provider, model string, limit int, apiKey string) error {
	if reply == nil {
		return fmt.Errorf("%s grounded search: no reply", provider)
	}
	if _, refused := responsesRefusal(reply); refused {
		return &types.StreamError{Outcome: types.StreamRefused, Provider: string(provider), Model: model, Reason: "refusal", Detail: "grounded search declined by provider"}
	}
	status := strings.TrimSpace(reply.Status)
	switch status {
	case "", "completed":
		return nil
	case "incomplete":
		reason := ""
		if reply.IncompleteDetails != nil {
			reason = reply.IncompleteDetails.Reason
		}
		if types.LengthStop(reason) {
			produced := 0
			if reply.Usage != nil {
				produced = reply.Usage.outputTokens()
			}
			return outputTruncated(provider, model, "GroundedWebSearch", reason, responsesTextFromReply(reply), limit, produced)
		}
		return fmt.Errorf("%s grounded search: run incomplete (reason=%s)", provider, sanitizeGroundedErrorToken(reason, apiKey, maxGroundedErrorTypeLen))
	}
	return fmt.Errorf("%s grounded search: run %s, not completed", provider, sanitizeGroundedErrorToken(status, apiKey, maxGroundedErrorTypeLen))
}

// groundedResultFromReply turns a grounded-search reply into the result and
// its source URLs. Reasoning items are not read; web_search_call contributes
// retrieved pages, not answer text.
func groundedResultFromReply(provider Provider, reply *responsesReply) (*types.GroundedWebSearchResult, []string, error) {
	captured := responsesGroundingFromReply(reply)
	text := strings.TrimSpace(responsesAssistantText(reply))
	if text == "" {
		return nil, nil, fmt.Errorf("%s grounded search: no output_text in response", provider)
	}
	return &types.GroundedWebSearchResult{
		Text:      text,
		Citations: captured.Citations,
		Results:   captured.Results,
		Usage:     reply.Usage.groundedUsage(),
	}, captured.Sources, nil
}
