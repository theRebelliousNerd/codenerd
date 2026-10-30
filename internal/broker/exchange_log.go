package broker

import (
	"context"
	"strings"
	"sync"

	"codenerd/internal/sessionlog"
	"codenerd/internal/types"
)

// The session log hook. Every model call passes core.admit, observed and
// settle, so this is the one place every provider is covered: the HTTP
// clients through the capture on the call's context, the CLI engines through
// their exec recorder, the stream proxies through the text collected below.
//
// Two records come out of it. The exchange file is debug-only; the agent's
// transcript is not, and takes its assistant entries from the same settled
// result so an agent needs to do nothing for its answers to be recorded.

// streamText accumulates what a streamed call emitted, for the exchange's
// assembled response.
type streamText struct {
	mu       sync.Mutex
	text     strings.Builder
	thoughts strings.Builder
}

func (s *streamText) addText(chunk string) {
	s.mu.Lock()
	s.text.WriteString(chunk)
	s.mu.Unlock()
}

func (s *streamText) addThought(chunk string) {
	s.mu.Lock()
	s.thoughts.WriteString(chunk)
	s.mu.Unlock()
}

func (s *streamText) response(stop string) *sessionlog.Response {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := &sessionlog.Response{Text: sessionlog.Lines(s.text.String()), StopReason: stop, Streamed: true}
	if s.thoughts.Len() > 0 {
		r.Thoughts = sessionlog.Lines(s.thoughts.String())
	}
	return r
}

// tee forwards in to the returned channel, handing every chunk to add first.
// A nil channel stays nil. The goroutine ends when in closes; the consumer
// (types.RelayStream) always drains to the end.
func tee(in <-chan string, add func(string)) <-chan string {
	if in == nil {
		return nil
	}
	out := make(chan string)
	go func() {
		defer close(out)
		for chunk := range in {
			add(chunk)
			out <- chunk
		}
	}()
	return out
}

// recording reports whether anything will read this call's response: an
// exchange file is open, or the agent keeps a transcript.
func (o *callObserver) recording() bool {
	return o != nil && (o.call != nil || sessionlog.TranscriptFrom(o.ctx) != nil || sessionlog.HasLiveProgress(o.ctx))
}

// beginLog opens the exchange for an admitted call and attaches the capture to
// ctx. With logging off it returns ctx unchanged.
func (c *core) beginLog(ctx context.Context, receipt Receipt, req *Request) (context.Context, *sessionlog.Call) {
	return sessionlog.Begin(ctx, sessionlog.CallMeta{
		Provider:   receipt.Provider,
		Model:      receipt.Model,
		Method:     receipt.Method,
		Purpose:    string(receipt.Purpose),
		Phase:      string(receipt.Phase),
		Session:    receipt.Scope,
		Fallback:   string(receipt.Purpose),
		Credential: receipt.CredentialAlias,
	})
}

// logRefusal records a request the broker refused: it reached no provider, and
// the file says why.
func (c *core) logRefusal(ctx context.Context, receipt Receipt, req *Request) {
	_, call := c.beginLog(ctx, receipt, req)
	call.End(sessionlog.Result{
		Err:       &AdmissionError{Decision: receipt.Decision, Purpose: receipt.Purpose},
		Refused:   true,
		Admission: admissionOf(receipt),
		Logical:   logicalOf(req),
	})
}

// logSettled closes the exchange and appends the agent's transcript entry.
func (c *core) logSettled(receipt Receipt, req *Request, obs *callObserver, callErr error) {
	if !obs.recording() {
		return
	}
	resp := obs.responseValue()
	if call := obs.call; call != nil {
		u := &sessionlog.Usage{
			InputTokens:      int(receipt.Actual.InputTokens),
			CacheReadTokens:  int(receipt.Actual.CachedTokens),
			CacheWriteTokens: int(receipt.Actual.CacheWriteTokens),
			OutputTokens:     int(receipt.Actual.OutputTokens),
			ThinkingTokens:   int(receipt.Actual.ThinkingTokens),
			Status:           string(receipt.UsageStatus),
			Attempts:         receipt.Attempts,
		}
		sessionlog.PriceUsage(receipt.Model, u)
		call.End(sessionlog.Result{
			Response:  resp,
			Usage:     u,
			Admission: admissionOf(receipt),
			Err:       callErr,
			Logical:   logicalOf(req),
		})
	}
	tr := sessionlog.TranscriptFrom(obs.ctx)
	switch {
	case callErr != nil:
		tr.CallFailed(obs.ctx, receipt.Model, callErr)
	case resp != nil:
		tr.Assistant(obs.ctx, receipt.Model, resp)
	}
}

// admission is what the broker believed before the request went out.
type admission struct {
	Estimated Count    `json:"estimated"`
	Decision  Decision `json:"decision"`
}

func admissionOf(r Receipt) any { return admission{Estimated: r.Estimated, Decision: r.Decision} }

// responseValue is the settled response: the value the call returned, or the
// stream it emitted assembled into its final form.
func (o *callObserver) responseValue() *sessionlog.Response {
	if o.stream != nil {
		return o.stream.response(o.streamStop)
	}
	return responseOf(o.out)
}

// responseOf converts what a call returned into the exchange's response.
func responseOf(out any) *sessionlog.Response {
	switch v := out.(type) {
	case string:
		return &sessionlog.Response{Text: sessionlog.Lines(v)}
	case *types.LLMToolResponse:
		if v == nil {
			return nil
		}
		r := &sessionlog.Response{Text: sessionlog.Lines(v.Text), StopReason: v.StopReason}
		if v.ThoughtSummary != "" {
			r.Thoughts = sessionlog.Lines(v.ThoughtSummary)
		}
		for _, tc := range v.ToolCalls {
			r.ToolCalls = append(r.ToolCalls, toolCallOf(tc))
		}
		return r
	case *types.GroundedWebSearchResult:
		if v == nil {
			return nil
		}
		r := &sessionlog.Response{Text: sessionlog.Lines(v.Text)}
		if tree, err := sessionlog.ToTree(map[string]any{"citations": v.Citations, "results": v.Results}); err == nil {
			r.Extra, _ = sessionlog.SplitLines(tree)
		}
		return r
	}
	return nil
}

func toolCallOf(tc types.ToolCall) sessionlog.ToolCall {
	out := sessionlog.ToolCall{ID: tc.ID, Name: tc.Name, ArgsError: tc.ArgsError}
	if tree, err := sessionlog.ToTree(tc.Input); err == nil {
		out.Input, out.LinesPaths = sessionlog.SplitLines(tree)
	}
	return out
}

// logicalOf is the request in the broker's terms. It is written only for
// calls that produced no wire capture.
func logicalOf(req *Request) *sessionlog.LogicalRequest {
	if req == nil {
		return nil
	}
	l := &sessionlog.LogicalRequest{}
	if req.System != "" {
		l.System = sessionlog.Lines(req.System)
	}
	if req.User != "" {
		l.User = sessionlog.Lines(req.User)
	}
	for _, m := range req.Messages {
		lm := sessionlog.LogicalMessage{Role: m.Role}
		if m.Text != "" {
			lm.Text = sessionlog.Lines(m.Text)
		}
		if len(m.ToolCalls) > 0 {
			var calls []sessionlog.ToolCall
			for _, tc := range m.ToolCalls {
				calls = append(calls, toolCallOf(tc))
			}
			lm.ToolCalls = calls
		}
		if len(m.ToolResults) > 0 {
			type result struct {
				ID      string   `json:"tool_use_id"`
				IsError bool     `json:"is_error,omitempty"`
				Content []string `json:"content"`
			}
			var rs []result
			for _, tr := range m.ToolResults {
				rs = append(rs, result{ID: tr.ToolUseID, IsError: tr.IsError, Content: sessionlog.Lines(tr.Content)})
			}
			lm.ToolResults = rs
		}
		l.Messages = append(l.Messages, lm)
	}
	if len(req.Tools) > 0 {
		if tree, err := sessionlog.ToTree(req.Tools); err == nil {
			l.Tools, l.LinesPaths = sessionlog.SplitLines(tree)
		}
	}
	return l
}
