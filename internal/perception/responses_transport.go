package perception

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"codenerd/internal/logging"
)

// responsesEndpoint is one Responses API target: where to POST and as whom.
type responsesEndpoint struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
	provider   Provider
}

// url is the endpoint's /responses URL.
func (e responsesEndpoint) url() string {
	return strings.TrimSuffix(e.baseURL, "/") + responsesPath
}

// newRequest builds one POST of an already marshalled body.
func (e responsesEndpoint) newRequest(ctx context.Context, body []byte, stream bool) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.url(), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build responses request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.apiKey)
	if stream {
		req.Header.Set("Accept", "text/event-stream")
	}
	return req, nil
}

// retryableResponsesStatus reports a status worth sending again: rate
// limiting (429, including slow_down), a transient server failure (5xx,
// including server_is_overloaded), or a model_not_found the vendor has been
// observed to return spuriously for a model that exists.
func retryableResponsesStatus(code int, body string) bool {
	return code == http.StatusTooManyRequests || isTransientHTTPStatus(code) || isTransientModelNotFound(code, body)
}

// postResponses sends one non-streaming Responses request with bounded retry
// and returns the decoded reply. Retry-After is honored (retryDelay). Any
// other 4xx returns at once with the vendor's body: those name the offending
// field, and losing that turns a one-line fix into a guessing game. Usage is
// not reported here; the caller knows which bucket the request belongs to.
func postResponses(ctx context.Context, e responsesEndpoint, reqBody responsesRequest) (*responsesReply, error) {
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal responses request: %w", err)
	}

	maxRetries := llmMaxRetries()
	var (
		body    []byte
		lastErr error
	)
	for attempt := 0; attempt <= maxRetries; attempt++ {
		req, err := e.newRequest(ctx, jsonData, false)
		if err != nil {
			return nil, err
		}
		attemptStart := time.Now()
		resp, err := e.httpClient.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			lastErr = fmt.Errorf("responses request failed after %v: %w", time.Since(attemptStart).Round(time.Millisecond), err)
			logging.PerceptionWarn("[%s] responses attempt %d/%d failed after %v (%v); retrying",
				e.provider, attempt+1, maxRetries+1, time.Since(attemptStart).Round(time.Second), err)
			if sleepErr := sleepCtx(ctx, retryDelay(nil, attempt)); sleepErr != nil {
				return nil, sleepErr
			}
			continue
		}

		body, err = io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("read responses body: %w", err)
		}
		if resp.StatusCode == http.StatusOK {
			lastErr = nil
			break
		}

		bodyStr := strings.TrimSpace(string(body))
		lastErr = fmt.Errorf("responses request failed with status %d: %s", resp.StatusCode, bodyStr)
		if !retryableResponsesStatus(resp.StatusCode, bodyStr) {
			return nil, lastErr
		}
		wait := retryDelay(resp, attempt)
		logging.PerceptionWarn("[%s] responses attempt %d/%d got HTTP %d; retrying in %v",
			e.provider, attempt+1, maxRetries+1, resp.StatusCode, wait)
		if sleepErr := sleepCtx(ctx, wait); sleepErr != nil {
			return nil, sleepErr
		}
	}
	if lastErr != nil {
		return nil, fmt.Errorf("%w (after %d attempts)", lastErr, maxRetries+1)
	}

	var reply responsesReply
	if err := json.Unmarshal(body, &reply); err != nil {
		return nil, fmt.Errorf("decode responses body: %w", err)
	}
	if reply.Error != nil && strings.TrimSpace(reply.Error.Message) != "" {
		return nil, fmt.Errorf("responses error (%s): %s", reply.Error.Type, reply.Error.Message)
	}
	return &reply, nil
}

// logResponsesUsage writes the one debug line every Responses call leaves.
func logResponsesUsage(provider Provider, reply *responsesReply, start time.Time) {
	if reply == nil || reply.Usage == nil {
		return
	}
	logging.Get(logging.CategoryAPI).Debug(
		"%s responses: in=%d out=%d reasoning=%d cached=%d status=%s tier=%s in %v",
		provider, reply.Usage.InputTokens, reply.Usage.OutputTokens,
		reply.Usage.reasoningTokens(), reply.Usage.cachedTokens(),
		reply.Status, reply.ServiceTier, time.Since(start).Round(time.Millisecond))
}
