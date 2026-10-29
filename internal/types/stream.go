package types

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
)

// A streamed completion is two things on two channels: provisional visible
// text, and exactly one terminal StreamResult that says how generation ended.
//
// The content channel closing is NOT the end of the answer. A clean transport
// EOF, a parseable prefix and a provider's successful generation are three
// different observations; only the terminal result reports the third. A
// consumer drains content, then reads the terminal channel, and treats a
// terminal channel that closes without a value as an incomplete stream (see
// AwaitStreamResult). Producers publish through StreamWriter, which makes
// "exactly one terminal, after the last chunk" structural.

// ThoughtStreamer is the opt-in streaming surface for clients that can expose
// the model's reasoning summaries beside its visible output. Callers type-assert
// and fall back to LLMClient.CompleteWithStreaming when it is absent.
//
// Content and thoughts stream in arrival order on their own channels; the
// thoughts channel may close without data (thinking disabled, or the model
// produced none). The terminal channel has the same contract as
// CompleteWithStreaming's.
type ThoughtStreamer interface {
	CompleteWithStreamingAndThoughts(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (content <-chan string, thoughts <-chan string, done <-chan StreamResult)
}

// StreamOutcome classifies how a streamed generation ended.
type StreamOutcome string

const (
	// StreamComplete: the provider's protocol reported an acceptable end of
	// generation. It says nothing about whether the answer is right.
	StreamComplete StreamOutcome = "complete"
	// StreamTruncated: the output ceiling cut generation (*OutputTruncated).
	StreamTruncated StreamOutcome = "truncated"
	// StreamRefused: the model declined. Not an availability failure to route
	// around.
	StreamRefused StreamOutcome = "refused"
	// StreamFiltered: a provider safety or content filter stopped generation.
	StreamFiltered StreamOutcome = "filtered"
	// StreamCanceled: the caller's context ended the stream.
	StreamCanceled StreamOutcome = "canceled"
	// StreamUnsupported: a terminal shape this operation cannot accept, such as
	// a tool-call finish on a text-only stream or a finish reason the adapter
	// does not know. Unknown reasons never default to success.
	StreamUnsupported StreamOutcome = "unsupported"
	// StreamProtocolFailed: malformed or out-of-order events. A malformed event
	// is never silently skipped: the text it carried would be missing from a
	// fluent-looking answer.
	StreamProtocolFailed StreamOutcome = "protocol_failed"
	// StreamIncomplete: the stream ended without the provider's terminal
	// evidence (EOF before a finish reason, or a terminal channel closed with
	// no result).
	StreamIncomplete StreamOutcome = "incomplete"
	// StreamFailed: transport, HTTP, authentication or provider error.
	StreamFailed StreamOutcome = "failed"
)

// ErrStreamIncomplete matches (errors.Is) every stream that ended without
// terminal evidence.
var ErrStreamIncomplete = errors.New("stream ended without terminal evidence")

// StreamError reports a non-complete stream end other than truncation, which
// keeps its own *OutputTruncated type.
type StreamError struct {
	Outcome  StreamOutcome
	Provider string
	Model    string
	// Reason is the provider's own finish/stop reason, when it gave one.
	Reason string
	// Detail says what was observed, for diagnostics.
	Detail string
	// Err is the underlying cause, if any.
	Err error
}

// Error implements error.
func (e *StreamError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "stream %s", e.Outcome)
	if e.Provider != "" {
		fmt.Fprintf(&b, " (%s", e.Provider)
		if e.Model != "" {
			fmt.Fprintf(&b, " %s", e.Model)
		}
		b.WriteString(")")
	}
	if e.Reason != "" {
		fmt.Fprintf(&b, ": finish=%q", e.Reason)
	}
	if e.Detail != "" {
		fmt.Fprintf(&b, ": %s", e.Detail)
	}
	if e.Err != nil {
		fmt.Fprintf(&b, ": %v", e.Err)
	}
	return b.String()
}

// Unwrap returns the underlying cause.
func (e *StreamError) Unwrap() error { return e.Err }

// Is makes every incomplete StreamError match ErrStreamIncomplete.
func (e *StreamError) Is(target error) bool {
	return target == ErrStreamIncomplete && e.Outcome == StreamIncomplete
}

// StreamOutcomeOf classifies a stream's terminal error. nil is complete.
func StreamOutcomeOf(err error) StreamOutcome {
	if err == nil {
		return StreamComplete
	}
	if _, ok := AsOutputTruncated(err); ok {
		return StreamTruncated
	}
	var se *StreamError
	if errors.As(err, &se) {
		return se.Outcome
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return StreamCanceled
	}
	if errors.Is(err, ErrStreamIncomplete) {
		return StreamIncomplete
	}
	return StreamFailed
}

// StreamResult is the one terminal value of a streamed completion.
type StreamResult struct {
	// StopReason is the provider's own word for how generation ended
	// ("end_turn", "stop", "STOP", "length", ...). Empty when the stream
	// failed before the provider said, or when the engine exposes none.
	StopReason string
	// Usage is the adapter's normalized report for the attempt that produced
	// this stream. It is response-carried evidence: the billed usage itself
	// reaches the tracker and broker through usage.Observer.
	Usage UsageMetadata
	// Err is nil exactly when Outcome() is StreamComplete. Truncation is an
	// *OutputTruncated; other non-complete ends are *StreamError, a context
	// error, or the provider/transport error itself.
	Err error
	// GroundingSources are the URLs this stream's own answer was grounded
	// on, as the provider reported them in the stream (search results,
	// citations). Request-owned: a client-wide "last sources" getter cannot
	// say which request produced its contents. Nil when the provider
	// reported none.
	GroundingSources []string
}

// Outcome classifies the result.
func (r StreamResult) Outcome() StreamOutcome { return StreamOutcomeOf(r.Err) }

// incompleteResult is what a consumer sees when the terminal channel closes
// without a value.
func incompleteResult() StreamResult {
	return StreamResult{Err: &StreamError{
		Outcome: StreamIncomplete,
		Detail:  "terminal channel closed without a result",
	}}
}

// StreamWriter is the producer side of a streamed completion. It owns the
// channels: content (and thoughts, when requested) carry provisional chunks,
// and Finish publishes the one terminal result after the last chunk and closes
// everything. Close without Finish closes the channels with no terminal, which
// every consumer reads as incomplete; defer it so a panic or an early return
// can never leave a consumer blocked or mistake an abandoned stream for a
// finished one.
type StreamWriter struct {
	content  chan string
	thoughts chan string
	done     chan StreamResult
	once     sync.Once
}

const streamBuffer = 64

// NewStream returns a writer and the consumer ends of a text stream.
func NewStream() (*StreamWriter, <-chan string, <-chan StreamResult) {
	w := &StreamWriter{
		content: make(chan string, streamBuffer),
		done:    make(chan StreamResult, 1),
	}
	return w, w.content, w.done
}

// NewThoughtStream is NewStream with a second provisional channel for the
// model's reasoning summaries, which never mix into the content channel.
func NewThoughtStream() (*StreamWriter, <-chan string, <-chan string, <-chan StreamResult) {
	w := &StreamWriter{
		content:  make(chan string, streamBuffer),
		thoughts: make(chan string, streamBuffer),
		done:     make(chan StreamResult, 1),
	}
	return w, w.content, w.thoughts, w.done
}

// Content delivers one visible chunk, or returns ctx's error if the consumer
// is gone first.
func (w *StreamWriter) Content(ctx context.Context, chunk string) error {
	if chunk == "" {
		return nil
	}
	select {
	case w.content <- chunk:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Thought delivers one reasoning chunk. A writer without a thoughts channel
// drops it: reasoning never enters the visible channel.
func (w *StreamWriter) Thought(ctx context.Context, chunk string) error {
	if w.thoughts == nil || chunk == "" {
		return nil
	}
	select {
	case w.thoughts <- chunk:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// HasThoughts reports whether the consumer asked for a thoughts channel.
func (w *StreamWriter) HasThoughts() bool { return w.thoughts != nil }

// Finish publishes res as the stream's terminal result and closes every
// channel. Only the first Finish or Close has any effect.
func (w *StreamWriter) Finish(res StreamResult) {
	w.once.Do(func() {
		w.closeChunks()
		w.done <- res
		close(w.done)
	})
}

// Fail is Finish with an error and no stop reason or usage.
func (w *StreamWriter) Fail(err error) { w.Finish(StreamResult{Err: err}) }

// Close closes every channel without a terminal result unless Finish already
// ran. Consumers then report the stream incomplete.
func (w *StreamWriter) Close() {
	w.once.Do(func() {
		w.closeChunks()
		close(w.done)
	})
}

func (w *StreamWriter) closeChunks() {
	close(w.content)
	if w.thoughts != nil {
		close(w.thoughts)
	}
}

// FailedStream returns a stream that ends at once with err and no content.
func FailedStream(err error) (<-chan string, <-chan StreamResult) {
	w, content, done := NewStream()
	w.Fail(err)
	return content, done
}

// FailedThoughtStream is FailedStream for the thoughts-capable surface.
func FailedThoughtStream(err error) (<-chan string, <-chan string, <-chan StreamResult) {
	w, content, thoughts, done := NewThoughtStream()
	w.Fail(err)
	return content, thoughts, done
}

// StreamWhole runs a whole (non-streaming) completion and publishes its text as
// one chunk followed by a complete terminal result. It is for engines that have
// no incremental transport: their completion call has already validated the
// provider's finish reason and reported usage, so the stream claims nothing
// more than that call established. A failed call publishes no content. The
// completion returns the URLs its answer was grounded on (nil for none), which
// travel on the terminal result.
func StreamWhole(ctx context.Context, complete func(context.Context) (string, []string, error)) (<-chan string, <-chan StreamResult) {
	w, content, done := NewStream()
	go func() {
		defer w.Close()
		text, sources, err := complete(ctx)
		if err != nil {
			w.Fail(err)
			return
		}
		if err := w.Content(ctx, text); err != nil {
			w.Fail(err)
			return
		}
		// The completion's own grounding rides on its terminal result, as a
		// real stream's does (StreamResult.GroundingSources).
		w.Finish(StreamResult{GroundingSources: sources})
	}()
	return content, done
}

// AwaitStreamResult receives the terminal result from done. A channel that
// closes without a value, or a nil channel, yields an incomplete result; a
// context that ends first yields its error.
func AwaitStreamResult(ctx context.Context, done <-chan StreamResult) StreamResult {
	if done == nil {
		return incompleteResult()
	}
	select {
	case res, ok := <-done:
		return StreamResultOf(res, ok)
	case <-ctx.Done():
		return StreamResult{Err: ctx.Err()}
	}
}

// StreamResultOf converts a receive from a terminal channel into the result:
// a closed channel (ok false) is an incomplete stream, never a success.
func StreamResultOf(res StreamResult, ok bool) StreamResult {
	if !ok {
		return incompleteResult()
	}
	return res
}

// CollectStream drains content, passing each chunk to onChunk (which may be
// nil), then returns the accumulated text and the terminal result.
func CollectStream(ctx context.Context, content <-chan string, done <-chan StreamResult, onChunk func(string)) (string, StreamResult) {
	var b strings.Builder
	for content != nil {
		select {
		case chunk, ok := <-content:
			if !ok {
				content = nil
				continue
			}
			b.WriteString(chunk)
			if onChunk != nil {
				onChunk(chunk)
			}
		case <-ctx.Done():
			return b.String(), StreamResult{Err: ctx.Err()}
		}
	}
	return b.String(), AwaitStreamResult(ctx, done)
}

// CaptureStream copies every chunk of src into b on its way through, for a
// wrapper that must keep the whole text (a trace, a response log). b is
// complete once the returned channel has closed; read it only after that. A
// nil src stays nil, which RelayStream treats as closed.
func CaptureStream(src <-chan string, b *strings.Builder) <-chan string {
	if src == nil {
		return nil
	}
	out := make(chan string)
	go func() {
		defer close(out)
		for chunk := range src {
			b.WriteString(chunk)
			out <- chunk
		}
	}()
	return out
}

// RelayStream forwards one stream into w: content to w's content channel and
// thoughts (when both sides have them) to w's thoughts channel, until both
// sources close; then it returns the source's terminal result without
// publishing it, so the wrapper can act on it (settle a receipt, release a
// slot, fail over) before calling w.Finish. emitted reports whether any
// non-empty chunk reached w. A nil source channel counts as closed.
//
// When ctx ends first, forwarding stops but the sources are still drained to
// their close and the source's terminal is still read: the producer is never
// left blocked on a send nobody receives, and the usage it observed before
// stopping is preserved on the returned result, whose error is then ctx's —
// this consumer did not receive the whole answer, whatever the source says.
func RelayStream(ctx context.Context, w *StreamWriter, content, thoughts <-chan string, done <-chan StreamResult) (res StreamResult, emitted bool) {
	var stopped error
	for content != nil || thoughts != nil {
		select {
		case chunk, ok := <-content:
			if !ok {
				content = nil
				continue
			}
			if stopped != nil {
				continue
			}
			if err := w.Content(ctx, chunk); err != nil {
				stopped = err
				continue
			}
			emitted = emitted || chunk != ""
		case chunk, ok := <-thoughts:
			if !ok {
				thoughts = nil
				continue
			}
			if stopped != nil {
				continue
			}
			if err := w.Thought(ctx, chunk); err != nil {
				stopped = err
				continue
			}
			emitted = emitted || (chunk != "" && w.HasThoughts())
		}
	}
	if done == nil {
		res = incompleteResult()
	} else {
		r, ok := <-done
		res = StreamResultOf(r, ok)
	}
	// A source that ended clean, or that merely closed without a terminal,
	// says less than the cancellation that stopped this relay; any other
	// failure the source reported is the more specific fact and is kept.
	if stopped != nil && (res.Err == nil || errors.Is(res.Err, ErrStreamIncomplete)) {
		res.Err = stopped
	}
	return res, emitted
}
