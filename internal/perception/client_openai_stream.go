package perception

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// CompleteWithStreaming streams a completion over Responses server-sent
// events. Visible answer text goes to the content channel. With
// enableThinking and a configured reasoning_summary, the model's reasoning
// summaries go to the thoughts channel of a consumer that has one.
func (c *OpenAIClient) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (<-chan string, <-chan types.StreamResult) {
	w, contentChan, doneChan := types.NewStream()
	go func() {
		defer w.Close()
		w.Finish(c.stream(ctx, systemPrompt, userPrompt, enableThinking, w))
	}()
	return contentChan, doneChan
}

// stream runs one streamed completion to its terminal result.
func (c *OpenAIClient) stream(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool, w *types.StreamWriter) types.StreamResult {
	if err := c.admit(ctx, systemPrompt, userTurn(userPrompt), nil); err != nil {
		return types.StreamResult{Err: err}
	}
	ctx, cancel := c.withDeadline(ctx)
	defer cancel()
	start := time.Now()
	c.clearGroundingSources()

	req := c.newRequest(ctx, responsesInputFromHistory(systemPrompt, userTurn(userPrompt), responsesInputOptions{Phase: true}))
	req.Stream = true
	if summary := strings.TrimSpace(c.options.ReasoningSummary); enableThinking && summary != "" && req.Reasoning != nil {
		req.Reasoning.Summary = summary
	}
	if isPiggybackPrompt(ctx, systemPrompt, userPrompt) {
		withStructuredOutput(&req)
	}
	if err := c.finalize(&req, false); err != nil {
		res := types.StreamResult{Err: err}
		logStreamEnd("openai", start, res)
		return res
	}

	resp, err := c.openStream(ctx, &req)
	if err != nil {
		res := types.StreamResult{Err: err}
		logStreamEnd("openai", start, res)
		return res
	}
	defer resp.Body.Close()

	res, terminal := readResponsesStream(ctx, resp.Body, chatStreamProfile{provider: ProviderOpenAI, model: req.Model, limit: c.maxOutputTokens}, w)
	if terminal != nil {
		sources := responsesGroundingFromReply(terminal).Sources
		c.recordGroundingSources(sources)
		res.GroundingSources = append([]string(nil), sources...)
		if len(sources) == 0 {
			res.GroundingSources = nil
		}
	}
	reportUsage(ctx, req.Model, ProviderOpenAI, usageOpChat, res.Usage)
	logStreamEnd("openai", start, res)
	return res
}

// openStream posts the streaming request and returns the open 200 response,
// retrying rate limits and transient failures before any event is read. A
// schema rejection drops structured output once, as executeStructured does.
func (c *OpenAIClient) openStream(ctx context.Context, req *responsesRequest) (*http.Response, error) {
	c.throttle()
	e := c.endpoint()
	maxRetries := llmMaxRetries()
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		body, err := json.Marshal(req)
		if err != nil {
			return nil, fmt.Errorf("marshal responses request: %w", err)
		}
		httpReq, err := e.newRequest(ctx, body, true)
		if err != nil {
			return nil, err
		}
		resp, err := c.httpClient.Do(httpReq)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			lastErr = fmt.Errorf("responses stream request failed: %w", err)
			if sleepErr := sleepCtx(ctx, retryDelay(nil, attempt)); sleepErr != nil {
				return nil, sleepErr
			}
			continue
		}
		if resp.StatusCode == http.StatusOK {
			return resp, nil
		}
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		bodyStr := strings.TrimSpace(string(raw))
		lastErr = fmt.Errorf("responses request failed with status %d: %s", resp.StatusCode, bodyStr)
		if resp.StatusCode == http.StatusBadRequest && req.Text != nil && req.Text.Format != nil && isSchemaRejection(bodyStr) {
			logging.PerceptionWarn("[openai] structured output rejected on stream; retrying without text.format")
			req.Text.Format = nil
			continue
		}
		if !retryableResponsesStatus(resp.StatusCode, bodyStr) {
			return nil, lastErr
		}
		wait := retryDelay(resp, attempt)
		logging.PerceptionWarn("[openai] stream setup attempt %d/%d got HTTP %d; retrying in %v", attempt+1, maxRetries+1, resp.StatusCode, wait)
		if sleepErr := sleepCtx(ctx, wait); sleepErr != nil {
			return nil, sleepErr
		}
	}
	return nil, fmt.Errorf("%w (after %d attempts)", lastErr, maxRetries+1)
}
