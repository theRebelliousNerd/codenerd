package perception

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"codenerd/internal/types"
)

// responsesStreamEvent is one Responses server-sent event, decoded once. The
// event's type is in the JSON body as well as the SSE event field; the body is
// what is read, since a proxy may strip the event line.
type responsesStreamEvent struct {
	Type string `json:"type"`
	// Delta is the text of response.output_text.delta and of
	// response.reasoning_summary_text.delta.
	Delta  string `json:"delta"`
	ItemID string `json:"item_id"`
	// Item is the item of response.output_item.added / .done.
	Item *responsesItem `json:"item"`
	// Response is the full response on response.completed, .incomplete and
	// .failed: the terminal evidence, carrying status and usage.
	Response *responsesReply `json:"response"`
	// Code and Message are an "error" event's fields.
	Code    string `json:"code"`
	Message string `json:"message"`
}

// readResponsesStream consumes a Responses event body, forwarding the visible
// answer to w and reasoning summaries to w's thoughts channel, and returns the
// terminal result (not yet published: the caller reports usage first, then
// calls w.Finish) together with the terminal response object, or nil when the
// stream never reached one.
//
// The terminal evidence is a response.completed, response.incomplete or
// response.failed event. A clean EOF before any of them is an incomplete
// stream, never a success. Commentary-phase message items (progress notes
// between tool calls or searches) are not the answer and are not forwarded.
func readResponsesStream(ctx context.Context, body io.Reader, p chatStreamProfile, w *types.StreamWriter) (types.StreamResult, *responsesReply) {
	r := newSSEReader(body)
	defer r.Close()

	end := streamEnd{provider: p.provider, model: p.model, limit: p.limit}
	var (
		text       strings.Builder
		terminal   *responsesReply
		commentary = map[string]bool{}
	)
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
			if data == "" || data == "[DONE]" {
				continue
			}
			var e responsesStreamEvent
			if err := json.Unmarshal([]byte(data), &e); err != nil {
				return protocolError(p.provider, p.model, "malformed stream event", err)
			}
			switch e.Type {
			case "error":
				msg := e.Message
				if e.Code != "" {
					msg = e.Code + ": " + msg
				}
				return providerStreamError(p.provider, p.model, msg)
			case "response.output_item.added":
				if e.Item != nil && e.Item.Type == "message" && e.Item.Phase == "commentary" {
					commentary[e.Item.ID] = true
				}
			case "response.output_text.delta":
				if commentary[e.ItemID] || e.Delta == "" {
					continue
				}
				text.WriteString(e.Delta)
				if err := w.Content(ctx, e.Delta); err != nil {
					return err
				}
			case "response.reasoning_summary_text.delta":
				if err := w.Thought(ctx, e.Delta); err != nil {
					return err
				}
			case "response.completed", "response.incomplete", "response.failed":
				if e.Response == nil {
					return protocolError(p.provider, p.model, e.Type+" event without a response", nil)
				}
				terminal = e.Response
				end.finish = responsesFinish(terminal)
				if terminal.Usage != nil {
					end.usage = terminal.Usage.metadata()
					end.usageSeen = true
				}
				return nil
			}
		}
	}()
	end.text = text.String()
	res := end.result(ctx, readErr, classifyResponsesFinish(ctx, terminal))
	return res, terminal
}

// responsesFinish is the stream's finish word: the incomplete reason when the
// run stopped early, else the status ("completed", "failed").
func responsesFinish(reply *responsesReply) string {
	if reply.Status == "incomplete" && reply.IncompleteDetails != nil && reply.IncompleteDetails.Reason != "" {
		return reply.IncompleteDetails.Reason
	}
	if reply.Status == "" || reply.Status == "completed" {
		// A run that ended by calling a tool is not a text answer.
		for _, item := range reply.Output {
			if item.Type == "function_call" {
				return "tool_calls"
			}
		}
		return "completed"
	}
	return reply.Status
}

// classifyResponsesFinish maps a Responses finish to the stream contract:
// completed is a text answer (and must have text), max_output_tokens is
// truncation, content_filter a safety stop, and failed or cancelled a
// provider failure carrying the vendor's message.
func classifyResponsesFinish(ctx context.Context, terminal *responsesReply) func(streamEnd) error {
	return func(e streamEnd) error {
		if types.LengthStop(e.finish) {
			return outputTruncated(e.provider, e.model, streamMethod, e.finish, e.text, e.limit, e.usage.OutputTokens)
		}
		switch e.finish {
		case "completed":
			return e.emptyStream(ctx)
		case "content_filter":
			return e.failure(types.StreamFiltered, "provider content filter stopped generation", nil)
		case "tool_calls":
			return e.failure(types.StreamUnsupported, "tool-call finish on a text-only stream", nil)
		case "failed", "cancelled":
			msg := e.finish
			if terminal != nil && terminal.Error != nil && terminal.Error.Message != "" {
				msg = terminal.Error.Message
			}
			return providerStreamError(e.provider, e.model, msg)
		default:
			return e.failure(types.StreamUnsupported, "unrecognized finish reason", nil)
		}
	}
}
