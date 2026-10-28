package perception

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"codenerd/internal/types"
)

// chatStreamChunk is one Chat Completions streaming event, decoded once for
// every OpenAI-shaped vendor. Only what the terminal contract and the visible
// surface need is decoded.
type chatStreamChunk struct {
	Choices []struct {
		Index int `json:"index"`
		Delta *struct {
			Content string `json:"content"`
			// ReasoningContent is the out-of-band thinking trace some vendors
			// (DashScope/Qwen, Z.AI) stream. It goes to the thoughts channel
			// when the consumer asked for one, never to content.
			ReasoningContent string            `json:"reasoning_content"`
			ToolCalls        []json.RawMessage `json:"tool_calls"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Usage *chatUsage `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

// chatStreamProfile names the stream being read, for typed errors and
// truncation reports.
type chatStreamProfile struct {
	provider Provider
	model    string
	// limit is the configured output ceiling, reported on truncation.
	limit int
}

// readChatStream consumes a Chat Completions server-sent-event body, forwarding
// visible deltas to w, and returns the stream's terminal result without
// publishing it: the caller reports the attempt's usage first and then calls
// w.Finish, so a broker settling on the terminal has already seen the report.
//
// The protocol's terminal evidence is a finish_reason on the choice. "[DONE]"
// alone is not evidence of a finished answer (a proxy can send it after a cut
// upstream), and a clean EOF without a finish reason is an incomplete stream,
// never a success. A malformed event is a protocol failure: skipping it would
// return a fluent answer with a hole in it. The include_usage tail may arrive
// after the finish chunk; its absence leaves the content complete and the
// usage partial.
func readChatStream(ctx context.Context, body io.Reader, p chatStreamProfile, w *types.StreamWriter) types.StreamResult {
	r := newSSEReader(body)
	defer r.Close()

	end := streamEnd{provider: p.provider, model: p.model, limit: p.limit}
	var text strings.Builder
	readErr := func() error {
		for {
			ev, err := r.Next()
			if errors.Is(err, io.EOF) {
				return nil
			}
			if err != nil {
				return transportError(ctx, p.provider, p.model, err)
			}
			data := strings.TrimSpace(ev.Data)
			if data == "" {
				continue
			}
			if data == "[DONE]" {
				return nil
			}
			var chunk chatStreamChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				return protocolError(p.provider, p.model, "malformed stream event", err)
			}
			if chunk.Error != nil {
				return providerStreamError(p.provider, p.model, chunk.Error.Message)
			}
			if chunk.Usage != nil {
				// include_usage snapshots are cumulative: the last one is the
				// attempt's bill.
				end.usage = chunk.Usage.metadata()
				end.usageSeen = true
			}
			for _, choice := range chunk.Choices {
				if choice.Index != 0 {
					// One candidate is requested; another index is an
					// identity change this text surface cannot accept.
					return protocolError(p.provider, p.model, "unexpected choice index in a single-candidate stream", nil)
				}
				if choice.Delta != nil {
					if err := w.Thought(ctx, choice.Delta.ReasoningContent); err != nil {
						return err
					}
					if delta := choice.Delta.Content; delta != "" {
						text.WriteString(delta)
						if err := w.Content(ctx, delta); err != nil {
							return err
						}
					}
				}
				if choice.FinishReason != nil && *choice.FinishReason != "" {
					end.finish = *choice.FinishReason
				}
			}
		}
	}()
	end.text = text.String()
	return end.result(ctx, readErr, classifyChatFinish(ctx))
}
