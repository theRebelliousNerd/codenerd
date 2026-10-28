package perception

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"codenerd/internal/config"
	"codenerd/internal/types"
)

// CompleteWithStreaming streams user-visible text. Thinking/signature deltas
// never enter the surface response. Native tool history uses the block-aware
// non-streaming methods, where signed thinking is preserved verbatim.
func (c *AnthropicClient) CompleteWithStreaming(ctx context.Context, system, user string, enableThinking bool) (<-chan string, <-chan types.StreamResult) {
	w, content, done := types.NewStream()
	go func() {
		defer w.Close()
		ctx, cancel := context.WithTimeout(ctx, c.httpClient.Timeout)
		defer cancel()
		// A streamed request declares no server tool, so none is counted.
		if err := c.admit(system, userTurn(user), nil, nil); err != nil {
			w.Fail(err)
			return
		}
		req, err := c.request(ctx, system, []AnthropicMessage{{Role: "user", Content: user}}, nil)
		if err != nil {
			w.Fail(err)
			return
		}
		if enableThinking && req.Thinking == nil {
			if config.AnthropicSupportsAdaptive(req.Model) {
				req.Thinking = &AnthropicThinking{Type: "adaptive"}
			} else {
				if req.MaxTokens <= 1024 {
					w.Fail(fmt.Errorf("extended thinking requires max_output_tokens greater than 1024"))
					return
				}
				req.Thinking = &AnthropicThinking{Type: "enabled", BudgetTokens: 1024}
			}
		}
		req.Stream = true
		resp, err := c.sendMessages(ctx, req, anthropicCachePlan{system: c.systemCaching()})
		if err != nil {
			w.Fail(err)
			return
		}
		defer resp.Body.Close()
		res := readAnthropicStream(ctx, resp.Body, req, w)
		// Reported before the terminal is published, so a broker settling on
		// the terminal has already seen this attempt's bill.
		reportUsage(ctx, req.Model, ProviderAnthropic, usageOpChat, res.Usage)
		w.Finish(res)
	}()
	return content, done
}

type anthropicStreamEvent struct {
	Type         string                 `json:"type"`
	Message      *AnthropicResponse     `json:"message"`
	ContentBlock *AnthropicContentBlock `json:"content_block"`
	Delta        *struct {
		Type       string `json:"type"`
		Text       string `json:"text"`
		StopReason string `json:"stop_reason"`
	} `json:"delta"`
	Usage *anthropicUsage `json:"usage"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
	RequestID string `json:"request_id"`
}

// readAnthropicStream consumes a Messages API event stream, forwarding text
// deltas to w, and returns the terminal result without publishing it.
//
// The protocol's terminal evidence is message_stop after a message_delta that
// carried a stop_reason. "[DONE]" is an OpenAI marker, not an Anthropic
// completion, and is ignored if a proxy emits it. Usage starts at
// message_start (input and cache fields) and message_delta's usage is
// cumulative for output; a stream cut before the final message_delta reports
// what it saw as partial.
func readAnthropicStream(ctx context.Context, body io.Reader, req AnthropicRequest, w *types.StreamWriter) types.StreamResult {
	r := newSSEReader(body)
	defer r.Close()

	var (
		usage    anthropicUsage
		text     strings.Builder
		started  bool
		stopped  bool
		finalUse bool
		stop     string
	)
	fail := func(detail string) error {
		return protocolError(ProviderAnthropic, req.Model, detail, nil)
	}
	appendText := func(chunk string) error {
		if text.Len()+len(chunk) > anthropicResponseLimit {
			return protocolError(ProviderAnthropic, req.Model, "streamed text exceeds response limit", nil)
		}
		text.WriteString(chunk)
		return w.Content(ctx, chunk)
	}
	handle := func(raw string) error {
		if strings.TrimSpace(raw) == "[DONE]" {
			return nil
		}
		var evt anthropicStreamEvent
		if err := json.Unmarshal([]byte(raw), &evt); err != nil {
			return protocolError(ProviderAnthropic, req.Model, "malformed streaming event", err)
		}
		if evt.Error != nil {
			return &AnthropicAPIError{StatusCode: anthropicErrorStatus(evt.Error.Type), Type: evt.Error.Type, Message: evt.Error.Message, RequestID: evt.RequestID}
		}
		switch evt.Type {
		case "message_start":
			if started || evt.Message == nil {
				return fail("invalid message_start event")
			}
			started = true
			usage = evt.Message.Usage
		case "content_block_start":
			if !started || evt.ContentBlock == nil {
				return fail("invalid content_block_start event")
			}
			switch evt.ContentBlock.Type {
			case "text":
				return appendText(evt.ContentBlock.Text)
			case "thinking", "redacted_thinking":
			default:
				return &types.StreamError{
					Outcome: types.StreamUnsupported, Provider: string(ProviderAnthropic), Model: req.Model,
					Detail: fmt.Sprintf("unsupported text-stream content block %q", evt.ContentBlock.Type),
				}
			}
		case "content_block_delta":
			if !started || evt.Delta == nil {
				return fail("invalid content_block_delta event")
			}
			if evt.Delta.Type == "text_delta" {
				return appendText(evt.Delta.Text)
			}
		case "message_delta":
			if !started {
				return fail("message_delta before message_start")
			}
			if evt.Delta != nil && evt.Delta.StopReason != "" {
				stop = evt.Delta.StopReason
			}
			if evt.Usage != nil {
				// message_delta usage is cumulative. Input/cache fields are
				// usually present only in message_start; preserve those fields.
				usage.OutputTokens = evt.Usage.OutputTokens
				if evt.Usage.InputTokens > 0 {
					usage.InputTokens = evt.Usage.InputTokens
				}
				if evt.Usage.CacheReadInputTokens > 0 {
					usage.CacheReadInputTokens = evt.Usage.CacheReadInputTokens
				}
				if evt.Usage.CacheCreationInputTokens > 0 {
					usage.CacheCreationInputTokens = evt.Usage.CacheCreationInputTokens
				}
				finalUse = true
			}
		case "message_stop":
			if !started || stop == "" {
				return fail("message_stop without a stop_reason")
			}
			stopped = true
			// Ping and future event types can be ignored without dropping known
			// message content. Unknown content blocks above fail explicitly.
		}
		return nil
	}

	readErr := func() error {
		for !stopped {
			ev, err := r.Next()
			if errors.Is(err, io.EOF) {
				return nil
			}
			if err != nil {
				return transportError(ctx, ProviderAnthropic, req.Model, err)
			}
			if err := handle(ev.Data); err != nil {
				return err
			}
		}
		return nil
	}()

	end := streamEnd{
		provider: ProviderAnthropic, model: req.Model, limit: req.MaxTokens,
		text: text.String(), usage: usage.usageMetadata(), usageSeen: finalUse,
	}
	if stopped {
		end.finish = stop
	}
	return end.result(ctx, readErr, func(e streamEnd) error {
		if types.LengthStop(e.finish) {
			return outputTruncated(ProviderAnthropic, req.Model, streamMethod, e.finish, e.text, req.MaxTokens, e.usage.OutputTokens)
		}
		switch e.finish {
		case "end_turn", "stop_sequence":
			return e.emptyStream(ctx)
		case "refusal":
			return e.failure(types.StreamRefused, "the model declined the request", nil)
		default:
			// tool_use and pause_turn are not text answers on this surface,
			// and an unknown reason never defaults to success.
			return e.failure(types.StreamUnsupported, "unexpected stop_reason on a text stream", nil)
		}
	})
}
