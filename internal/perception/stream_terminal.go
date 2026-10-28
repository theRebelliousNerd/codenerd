package perception

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// streamMethod is the method name streamed completions report in typed
// truncation and stream errors.
const streamMethod = "CompleteWithStreaming"

// streamEnd is what one provider stream established when it stopped: the
// finish reason (empty when none arrived), the visible text forwarded, the
// normalized usage, and whether a usage block was seen at all.
type streamEnd struct {
	provider  Provider
	model     string
	limit     int
	finish    string
	text      string
	usage     types.UsageMetadata
	usageSeen bool
}

// streamFailure builds the typed error for a non-complete end.
func (e streamEnd) failure(outcome types.StreamOutcome, detail string, cause error) error {
	return &types.StreamError{
		Outcome:  outcome,
		Provider: string(e.provider),
		Model:    e.model,
		Reason:   e.finish,
		Detail:   detail,
		Err:      cause,
	}
}

// result turns the end of a stream into its terminal result. readErr is the
// error that stopped reading, if any (transport, protocol, provider error
// event, cancellation); classify maps a provider finish reason to nil
// (success) or the typed failure. Usage that was never reported, or reported
// by a stream that did not reach its terminal, is marked partial: the counts
// are lower bounds and the receipt must say so.
func (e streamEnd) result(ctx context.Context, readErr error, classify func(streamEnd) error) types.StreamResult {
	res := types.StreamResult{StopReason: e.finish, Usage: e.usage}
	switch {
	case ctx.Err() != nil:
		res.Err = ctx.Err()
	case readErr != nil:
		res.Err = readErr
	case e.finish == "":
		res.Err = e.failure(types.StreamIncomplete, "stream ended before the provider's finish reason", nil)
	default:
		res.Err = classify(e)
	}
	// Content completion and accounting completeness are separate facts. A
	// stream cut before the provider said how generation ended carries at
	// most a mid-stream usage snapshot; one whose final usage block never
	// arrived (an interrupted include_usage tail) has whole content and an
	// unknown bill. Either way the counts are lower bounds.
	if e.finish == "" || !e.usageSeen {
		res.Usage.Partial = true
	}
	return res
}

// emptyStream is the failure for a stream that finished normally having
// emitted no visible text: the same hollow success as an empty non-streaming
// completion, which would render blank in the chat surface. A caller that
// accepts empty output says so with WithAllowEmptyCompletion.
func (e streamEnd) emptyStream(ctx context.Context) error {
	if e.text != "" || allowEmptyCompletion(ctx) {
		return nil
	}
	return e.failure(types.StreamUnsupported,
		"stream produced no content; a reasoning model may have consumed the completion budget before emitting output", nil)
}

// classifyChatFinish maps a Chat Completions finish_reason. Only "stop" is a
// completed text answer; "length" is truncation whatever the prefix looks
// like; a content filter is a provider safety stop; a tool-call finish is not
// a text answer on this text-only surface; any other reason is unknown and is
// never defaulted to success.
func classifyChatFinish(ctx context.Context) func(streamEnd) error {
	return func(e streamEnd) error {
		if types.LengthStop(e.finish) {
			return outputTruncated(e.provider, e.model, streamMethod, e.finish, e.text, e.limit, e.usage.OutputTokens)
		}
		switch strings.ToLower(e.finish) {
		case "stop", "end_turn":
			return e.emptyStream(ctx)
		case "content_filter":
			return e.failure(types.StreamFiltered, "provider content filter stopped generation", nil)
		case "tool_calls", "function_call":
			return e.failure(types.StreamUnsupported, "tool-call finish on a text-only stream", nil)
		default:
			return e.failure(types.StreamUnsupported, "unrecognized finish reason", nil)
		}
	}
}

// logStreamEnd records how a stream ended, once, with its outcome class.
func logStreamEnd(vendor string, start time.Time, res types.StreamResult) {
	if res.Err != nil {
		logging.PerceptionError("[%s] CompleteWithStreaming: %s after %v: %v", vendor, res.Outcome(), time.Since(start), res.Err)
		return
	}
	logging.Perception("[%s] CompleteWithStreaming: completed in %v finish=%s output_tokens=%d partial_usage=%v",
		vendor, time.Since(start), res.StopReason, res.Usage.OutputTokens, res.Usage.Partial)
}

// protocolError wraps a malformed or illegal event.
func protocolError(provider Provider, model, detail string, cause error) error {
	return &types.StreamError{
		Outcome:  types.StreamProtocolFailed,
		Provider: string(provider),
		Model:    model,
		Detail:   detail,
		Err:      cause,
	}
}

// providerStreamError wraps an error event the provider sent mid-stream.
func providerStreamError(provider Provider, model, message string) error {
	return &types.StreamError{
		Outcome:  types.StreamFailed,
		Provider: string(provider),
		Model:    model,
		Detail:   fmt.Sprintf("provider error event: %s", message),
	}
}

// transportError wraps the error that ended reading the body. A cancellation
// is returned as itself so it classifies as canceled.
func transportError(ctx context.Context, provider Provider, model string, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	if errors.Is(err, errSSEOversize) {
		return protocolError(provider, model, "event exceeds the stream bound", err)
	}
	return &types.StreamError{
		Outcome:  types.StreamFailed,
		Provider: string(provider),
		Model:    model,
		Detail:   "transport error while reading the stream",
		Err:      err,
	}
}
