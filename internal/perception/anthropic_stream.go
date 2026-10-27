package perception

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"codenerd/internal/config"
	"codenerd/internal/types"
)

// CompleteWithStreaming streams user-visible text. Thinking/signature deltas
// never enter the surface response. Native tool history uses the block-aware
// non-streaming methods, where signed thinking is preserved verbatim.
func (c *AnthropicClient) CompleteWithStreaming(ctx context.Context, system, user string, enableThinking bool) (<-chan string, <-chan error) {
	content := make(chan string, 100)
	errors := make(chan error, 1)
	go func() {
		defer close(content)
		defer close(errors)
		ctx, cancel := context.WithTimeout(ctx, c.httpClient.Timeout)
		defer cancel()
		if err := c.admitInput(system, userTurn(user), nil); err != nil {
			errors <- err
			return
		}
		req, err := c.request(ctx, system, []AnthropicMessage{{Role: "user", Content: user}}, nil)
		if err != nil {
			errors <- err
			return
		}
		if enableThinking && req.Thinking == nil {
			if config.AnthropicSupportsAdaptive(req.Model) {
				req.Thinking = &AnthropicThinking{Type: "adaptive"}
			} else {
				if req.MaxTokens <= 1024 {
					errors <- fmt.Errorf("extended thinking requires max_output_tokens greater than 1024")
					return
				}
				req.Thinking = &AnthropicThinking{Type: "enabled", BudgetTokens: 1024}
			}
		}
		req.Stream = true
		resp, err := c.sendMessages(ctx, req, anthropicCachePlan{system: c.systemCaching()})
		if err != nil {
			errors <- err
			return
		}
		defer resp.Body.Close()
		var usage anthropicUsage
		defer func() {
			trackUsageDetailed(ctx, req.Model, ProviderAnthropic, usage.promptTokens(), usage.OutputTokens, usage.CacheReadInputTokens, usage.CacheCreationInputTokens, usageOpChat)
		}()
		err = readAnthropicStream(ctx, resp.Body, req, &usage, func(text string) error {
			select {
			case content <- text:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
		if err != nil {
			errors <- err
		}
	}()
	return content, errors
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
		Type string `json:"type"`
	} `json:"error"`
	RequestID string `json:"request_id"`
}

func readAnthropicStream(ctx context.Context, reader io.Reader, req AnthropicRequest, usage *anthropicUsage, emit func(string) error) error {
	scanner, release := newPooledScanner(reader, 1024*1024)
	defer release()
	var data strings.Builder
	var text strings.Builder
	var started, stopped bool
	var stopReason string
	appendText := func(chunk string) error {
		if text.Len()+len(chunk) > anthropicResponseLimit {
			return fmt.Errorf("Anthropic streamed text exceeds response limit")
		}
		text.WriteString(chunk)
		if chunk != "" {
			return emit(chunk)
		}
		return nil
	}
	handle := func() error {
		if data.Len() == 0 {
			return nil
		}
		raw := data.String()
		data.Reset()
		// [DONE] is an OpenAI marker, not an Anthropic completion. Require
		// message_stop and a real stop reason even if a proxy emits it.
		if strings.TrimSpace(raw) == "[DONE]" {
			return nil
		}
		var evt anthropicStreamEvent
		if err := json.Unmarshal([]byte(raw), &evt); err != nil {
			return fmt.Errorf("malformed Anthropic streaming event: %w", err)
		}
		if evt.Error != nil {
			return &AnthropicAPIError{StatusCode: anthropicErrorStatus(evt.Error.Type), Type: evt.Error.Type, RequestID: evt.RequestID}
		}
		switch evt.Type {
		case "message_start":
			if started || evt.Message == nil {
				return fmt.Errorf("invalid Anthropic message_start event")
			}
			started = true
			*usage = evt.Message.Usage
		case "content_block_start":
			if !started || evt.ContentBlock == nil {
				return fmt.Errorf("invalid Anthropic content_block_start event")
			}
			switch evt.ContentBlock.Type {
			case "text":
				return appendText(evt.ContentBlock.Text)
			case "thinking", "redacted_thinking":
			default:
				return fmt.Errorf("unsupported Anthropic text-stream content block %q", evt.ContentBlock.Type)
			}
		case "content_block_delta":
			if !started || evt.Delta == nil {
				return fmt.Errorf("invalid Anthropic content_block_delta event")
			}
			if evt.Delta.Type == "text_delta" {
				return appendText(evt.Delta.Text)
			}
		case "message_delta":
			if !started {
				return fmt.Errorf("Anthropic message_delta before message_start")
			}
			if evt.Delta != nil && evt.Delta.StopReason != "" {
				stopReason = evt.Delta.StopReason
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
			}
		case "message_stop":
			if !started || stopReason == "" {
				return fmt.Errorf("incomplete Anthropic stream at message_stop")
			}
			stopped = true
			// Ping and future event types can be ignored without dropping known
			// message content. Unknown content blocks above fail explicitly.
		}
		return nil
	}
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return err
		}
		line := scanner.Text()
		if line == "" {
			if err := handle(); err != nil {
				return err
			}
			if stopped {
				break
			}
		} else if strings.HasPrefix(line, "data:") {
			if data.Len() != 0 {
				data.WriteByte('\n')
			}
			data.WriteString(strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
			if data.Len() > 1024*1024 {
				return fmt.Errorf("Anthropic SSE event exceeds 1 MiB")
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read Anthropic stream: %w", err)
	}
	if !stopped {
		if err := handle(); err != nil {
			return err
		}
	}
	if !stopped {
		return fmt.Errorf("Anthropic stream ended before message_stop: %w", io.ErrUnexpectedEOF)
	}
	if types.LengthStop(stopReason) {
		return outputTruncated(ProviderAnthropic, req.Model, "CompleteWithStreaming", stopReason, text.String(), req.MaxTokens, usage.OutputTokens)
	}
	if stopReason == "refusal" {
		return fmt.Errorf("Anthropic refused the request")
	}
	if stopReason != "end_turn" && stopReason != "stop_sequence" {
		return fmt.Errorf("Anthropic text stream ended with unexpected stop_reason %q", stopReason)
	}
	if text.Len() == 0 {
		return fmt.Errorf("no text completion returned by Anthropic stream")
	}
	return nil
}
