package typesafe

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
)

// Request limits from the API spec (errors.md).
const (
	maxBodyBytes = 32 << 20
	maxDepth     = 64
	userAgent    = "codenerd-typesafe"
)

// Options builds a Client. Model, BaseURL and APIKey are required: nothing
// here names a model or an endpoint.
type Options struct {
	Model       string
	BaseURL     string
	APIKey      string
	LongContext bool          // sends TypeSafe-Danger-Zone-Long-Context
	Timeout     time.Duration // one call, retries included; 0 leaves it to the caller's context
	Retry       *RetryPolicy  // nil is DefaultRetryPolicy
	HTTPClient  *http.Client  // nil is a fresh client
}

// Client talks to one TypeSafe endpoint. It is safe for concurrent use.
type Client struct {
	model, baseURL, key string
	longContext         bool
	timeout             time.Duration
	retry               RetryPolicy
	http                *http.Client
	sleep               func(context.Context, time.Duration) error
}

// NewClient validates the options. The key is held only for the header.
func NewClient(o Options) (*Client, error) {
	switch {
	case strings.TrimSpace(o.Model) == "":
		return nil, errors.New("typesafe: model is required")
	case strings.TrimSpace(o.BaseURL) == "":
		return nil, errors.New("typesafe: base URL is required")
	case o.APIKey == "":
		return nil, errors.New("typesafe: API key is required")
	case o.Timeout < 0:
		return nil, errors.New("typesafe: timeout is negative")
	}
	c := &Client{
		model:       o.Model,
		baseURL:     strings.TrimRight(o.BaseURL, "/"),
		key:         o.APIKey,
		longContext: o.LongContext,
		timeout:     o.Timeout,
		retry:       DefaultRetryPolicy,
		http:        o.HTTPClient,
		sleep:       sleepCtx,
	}
	if o.Retry != nil {
		c.retry = *o.Retry
	}
	if c.http == nil {
		c.http = &http.Client{}
	}
	return c, nil
}

// Model is the configured model name, an alias or a versioned ID. The ID that
// actually answered is Response.Model.
func (c *Client) Model() string { return c.model }

// Ask puts the questions about state. state is a string, an object or an
// array. Any failure is an error and no Response; answers are read with
// Response.Noul, Choice and Score, which fail on a missing or mistyped answer.
// An answered call is reported to the usage tracker on ctx (usage.go), under
// the operation WithOperation named.
func (c *Client) Ask(ctx context.Context, state any, questions map[string]Question) (*Response, error) {
	body, err := c.buildBody(state, questions)
	if err != nil {
		return nil, err
	}
	var resp Response
	if err := c.do(ctx, http.MethodPost, "/v1/systemone", body, &resp); err != nil {
		return nil, err
	}
	c.recordUsage(ctx, &resp)
	return &resp, nil
}

// Model describes one model or alias the account can name.
type Model struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	ReleaseDate string `json:"release_date"`
}

// ListModels returns the names the account may send as the model. It lists
// aliases; versioned IDs are accepted whether listed or not.
func (c *Client) ListModels(ctx context.Context) ([]Model, error) {
	var out struct {
		Models []Model `json:"models"`
	}
	if err := c.do(ctx, http.MethodGet, "/v1/models", nil, &out); err != nil {
		return nil, err
	}
	return out.Models, nil
}

func (c *Client) buildBody(state any, questions map[string]Question) ([]byte, error) {
	if len(questions) == 0 {
		return nil, errors.New("typesafe: no questions")
	}
	for name, q := range questions {
		if name == "" || q == nil {
			return nil, errors.New("typesafe: a question needs a name and a body")
		}
		if err := q.validate(); err != nil {
			return nil, fmt.Errorf("typesafe: question %q: %w", name, err)
		}
	}
	rawState, err := json.Marshal(state)
	if err != nil {
		return nil, fmt.Errorf("typesafe: state is not encodable (finite numbers only): %w", err)
	}
	if len(rawState) == 0 || !strings.ContainsRune(`"{[`, rune(rawState[0])) {
		return nil, errors.New("typesafe: state must be a string, an object or an array")
	}
	if d := jsonDepth(rawState); d > maxDepth {
		return nil, fmt.Errorf("typesafe: state nests %d levels, the API allows %d", d, maxDepth)
	}
	body, err := json.Marshal(struct {
		Model     string              `json:"model"`
		State     json.RawMessage     `json:"state"`
		Questions map[string]Question `json:"questions"`
	}{c.model, rawState, questions})
	if err != nil {
		return nil, fmt.Errorf("typesafe: questions are not encodable (finite numbers only): %w", err)
	}
	if len(body) > maxBodyBytes {
		return nil, fmt.Errorf("typesafe: request is %d bytes, the API allows %d", len(body), maxBodyBytes)
	}
	return body, nil
}

// jsonDepth is the deepest nesting of objects and arrays in valid JSON.
func jsonDepth(b []byte) int {
	depth, deepest := 0, 0
	inString := false
	for i := 0; i < len(b); i++ {
		switch ch := b[i]; {
		case inString && ch == '\\':
			i++
		case ch == '"':
			inString = !inString
		case inString:
		case ch == '{' || ch == '[':
			if depth++; depth > deepest {
				deepest = depth
			}
		case ch == '}' || ch == ']':
			depth--
		}
	}
	return deepest
}

// do sends one call, retrying a 429 or 529 per the policy, and decodes a 200
// into out. Anything else is an error.
func (c *Client) do(ctx context.Context, method, path string, body []byte, out any) error {
	if c.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}
	if body != nil && logging.IsDebugMode() {
		// The key lives only in the header, which is never logged.
		logging.Get(logging.CategoryAPI).Debug("typesafe: %s %s%s payload=%s", method, c.baseURL, path, body)
	}
	for retry := 0; ; retry++ {
		apiErr, err := c.once(ctx, method, path, body, out)
		if err != nil {
			return err
		}
		if apiErr == nil {
			return nil
		}
		if !apiErr.retryable() || retry >= c.retry.Max {
			return apiErr
		}
		wait := c.retry.wait(retry, apiErr)
		if wait > c.retry.MaxWait {
			return fmt.Errorf("%w (retry-after %s exceeds the %s limit)", apiErr, wait, c.retry.MaxWait)
		}
		if dl, ok := ctx.Deadline(); ok && time.Until(dl) < wait {
			return fmt.Errorf("%w (a %s wait would pass the deadline)", apiErr, wait)
		}
		logging.Get(logging.CategoryAPI).Warn("typesafe: HTTP %d, retry %d/%d in %s", apiErr.Status, retry+1, c.retry.Max, wait)
		if err := c.sleep(ctx, wait); err != nil {
			return fmt.Errorf("%w; stopped waiting to retry: %w", apiErr, err)
		}
	}
}

// once makes one attempt. A non-200 reply is the *APIError; a transport or
// decode failure is the error.
func (c *Client) once(ctx context.Context, method, path string, body []byte, out any) (*APIError, error) {
	var rd io.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, rd)
	if err != nil {
		return nil, fmt.Errorf("typesafe: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.key)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.longContext {
		req.Header.Set("TypeSafe-Danger-Zone-Long-Context", "true")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("typesafe: %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes+1))
	if err != nil {
		return nil, fmt.Errorf("typesafe: reading reply: %w", err)
	}
	if len(data) > maxBodyBytes {
		return nil, fmt.Errorf("typesafe: reply exceeds %d bytes", maxBodyBytes)
	}
	if resp.StatusCode != http.StatusOK {
		return &APIError{
			Status:     resp.StatusCode,
			Detail:     errorDetail(data),
			RetryAfter: parseRetryAfter(resp.Header, time.Now()),
		}, nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return nil, fmt.Errorf("typesafe: %s %s: unreadable reply: %w", method, path, err)
	}
	return nil, nil
}
