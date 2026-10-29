package perception

import (
	"context"
	"errors"

	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// errFallbackNoClient is returned when a FallbackClient has neither a primary
// nor a secondary to call. Construction sites must avoid this by falling back
// to the main client; it survives here so a wiring gap fails loudly instead of
// panicking on a nil call.
var errFallbackNoClient = errors.New("fallback client has no primary or secondary client")

// FallbackClient tries its primary LLM client and, only when the primary call
// fails, retries once on its secondary. Either side may be nil: a nil primary
// goes straight to the secondary, and a nil secondary surfaces the primary's
// error unchanged.
//
// This exists because client construction can succeed while calls fail. The
// classification tier prefers the worker slot and only falls back to the main
// provider when construction fails — but an OpenRouter 403 (missing
// attestation), a revoked key, or a dead worker endpoint all surface at call
// time, after the tier was already chosen. Without call-time failover, every
// turn's intent understanding silently degrades to a non-LLM guess, policy
// derives the wrong next_action (observed live: /analyze_code for a /fix
// instruction), and the run reports success on work it never did.
//
// Refused, filtered, and truncated requests never fail over. Other primary
// errors can fail over, including context cancellation (the
// secondary then fails fast on the same context). Streaming fails over only
// when the primary errors before emitting its first chunk: once content has
// flowed to the consumer it cannot be un-emitted, so a late primary error is
// forwarded and the secondary is not consulted.
type FallbackClient struct {
	name      string
	primary   LLMClient
	secondary LLMClient
}

// NewFallbackClient builds a primary/secondary failover client. name labels
// log lines (e.g. "classification").
func NewFallbackClient(name string, primary, secondary LLMClient) *FallbackClient {
	return &FallbackClient{name: name, primary: primary, secondary: secondary}
}

var _ LLMClient = (*FallbackClient)(nil)

// Unwrap exposes the preferred path for broker chain walks: the primary
// when set, else the secondary. A nil side ends the walk (Walk treats a
// nil Unwrap as the chain end), which mirrors call behavior — a missing
// side never serves. Without this, Base/IsBrokered stop at the failover
// layer and concrete-engine checks downstream silently stop matching.
func (c *FallbackClient) Unwrap() LLMClient {
	if c.primary != nil {
		return c.primary
	}
	return c.secondary
}

// GetModel reports the primary's model, else the secondary's, for I/O
// tracing. Without this forwarding the scheduled wrapper logs MODEL: empty
// for every failover-protected call.
func (c *FallbackClient) GetModel() string {
	for _, client := range []LLMClient{c.primary, c.secondary} {
		if getter, ok := client.(interface{ GetModel() string }); ok {
			if model := getter.GetModel(); model != "" {
				return model
			}
		}
	}
	return ""
}

// ModelIdentity reports the primary's provider/model identity, else the
// secondary's. It satisfies types.ModelIdentifier for prompt-atom pinning.
func (c *FallbackClient) ModelIdentity() (provider, model string) {
	for _, client := range []LLMClient{c.primary, c.secondary} {
		if ident, ok := client.(types.ModelIdentifier); ok {
			if provider, model := ident.ModelIdentity(); model != "" {
				return provider, model
			}
		}
	}
	return "", ""
}

var _ types.ModelIdentifier = (*FallbackClient)(nil)

// Capability interfaces (ToolResultsProvider, CompleteWithSchema, thinking
// providers) are deliberately NOT forwarded. Forwarding would make the
// fallback client claim a capability whenever either side has it — but a
// secondary-only capability cannot serve the primary path, and callers that
// type-assert (the shard executor, the broker matrix) would take the
// multi-turn/native branch against a client that can only serve it after a
// primary failure. Observability forwarding above is safe because a model
// name is descriptive; a capability claim changes the caller's control flow.

// useSecondary reports whether a failed primary call should be retried on the
// secondary, logging the failover when it fires.
func (c *FallbackClient) useSecondary(op string, primaryErr error) bool {
	if c.secondary == nil || !failoverable(primaryErr) {
		return false
	}
	logging.Get(logging.CategoryPerception).Warn(
		"%s: primary %s failed (%v); failing over to secondary", c.name, op, primaryErr)
	return true
}

func (c *FallbackClient) Complete(ctx context.Context, prompt string) (string, error) {
	if c.primary != nil {
		resp, err := c.primary.Complete(ctx, prompt)
		if err == nil {
			return resp, nil
		}
		if !c.useSecondary("Complete", err) {
			return "", err
		}
	} else if c.secondary == nil {
		return "", errFallbackNoClient
	}
	return c.secondary.Complete(ctx, prompt)
}

func (c *FallbackClient) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	if c.primary != nil {
		resp, err := c.primary.CompleteWithSystem(ctx, systemPrompt, userPrompt)
		if err == nil {
			return resp, nil
		}
		if !c.useSecondary("CompleteWithSystem", err) {
			return "", err
		}
	} else if c.secondary == nil {
		return "", errFallbackNoClient
	}
	return c.secondary.CompleteWithSystem(ctx, systemPrompt, userPrompt)
}

func (c *FallbackClient) CompleteWithTools(ctx context.Context, systemPrompt, userPrompt string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	if c.primary != nil {
		resp, err := c.primary.CompleteWithTools(ctx, systemPrompt, userPrompt, tools)
		if err == nil {
			return resp, nil
		}
		if !c.useSecondary("CompleteWithTools", err) {
			return nil, err
		}
	} else if c.secondary == nil {
		return nil, errFallbackNoClient
	}
	return c.secondary.CompleteWithTools(ctx, systemPrompt, userPrompt, tools)
}

// CompleteWithStreaming relays the primary's stream and fails over to the
// secondary only while nothing has reached the consumer: a secondary answer is
// never spliced after primary content, because the result would read as one
// model's continuous answer. A refusal, a safety filter or a truncation is a
// verdict about the request, not an availability failure, so it is never
// routed around either.
func (c *FallbackClient) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (<-chan string, <-chan types.StreamResult) {
	w, out, done := types.NewStream()
	go func() {
		defer w.Close()
		if c.primary != nil {
			res, emitted := c.relayStream(ctx, w, c.primary, systemPrompt, userPrompt, enableThinking)
			if res.Err == nil || emitted || ctx.Err() != nil || !failoverable(res.Err) ||
				!c.useSecondary("CompleteWithStreaming", res.Err) {
				w.Finish(res)
				return
			}
		} else if c.secondary == nil {
			w.Fail(errFallbackNoClient)
			return
		}
		res, _ := c.relayStream(ctx, w, c.secondary, systemPrompt, userPrompt, enableThinking)
		w.Finish(res)
	}()
	return out, done
}

// relayStream forwards one client's stream into w and returns its terminal
// result and whether any content reached the consumer. A client returning nil
// channels ends incomplete, which permits failover instead of hanging.
func (c *FallbackClient) relayStream(ctx context.Context, w *types.StreamWriter, client LLMClient, systemPrompt, userPrompt string, enableThinking bool) (types.StreamResult, bool) {
	content, done := client.CompleteWithStreaming(ctx, systemPrompt, userPrompt, enableThinking)
	return types.RelayStream(ctx, w, content, nil, done)
}

// failoverable reports whether a provider failure may be retried on
// another model. Refused, filtered and truncated outcomes are answers about
// this request; another provider would only be asked to evade them.
func failoverable(err error) bool {
	switch types.StreamOutcomeOf(err) {
	case types.StreamRefused, types.StreamFiltered, types.StreamTruncated:
		return false
	}
	return true
}
